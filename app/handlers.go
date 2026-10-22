package app

import (
	"errors"
	log "log/slog"
	"strings"
	"sync"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/metrics"
	"github.com/kozalosev/goSadTgBot/wizard"
)

const (
	defaultMessageTr          = "commands.default.message"
	defaultMessageOnCommandTr = "commands.default.message.on.command"

	defaultGuestResultID = "default"
)

// AllowedUpdates returns the list of update types processed by [HandleUpdate] for which there are handlers. Pass it to
// [tgbotapi.UpdateConfig.AllowedUpdates] when you use long polling.
//   - message and callback_query are always included since they are used by the wizard forms.
//   - inline_query and chosen_inline_result are included only if there are [Params.InlineHandlers].
//   - guest_message is included only if there are [Params.GuestHandlers].
//
// A warning is logged if there are handlers for a mode that is not enabled for the bot via @BotFather.
func AllowedUpdates(appParams *Params) []string {
	updateTypes := []string{
		tgbotapi.UpdateTypeMessage,
		tgbotapi.UpdateTypeCallbackQuery,
	}
	var self tgbotapi.User
	if appParams.API != nil {
		self = appParams.API.GetStandardAPI().Self
	}
	if len(appParams.InlineHandlers) > 0 {
		warnIfModeDisabled(appParams, self.SupportsInlineQueries, "inline")
		updateTypes = append(updateTypes, tgbotapi.UpdateTypeInlineQuery, tgbotapi.UpdateTypeChosenInlineResult)
	}
	if len(appParams.GuestHandlers) > 0 {
		warnIfModeDisabled(appParams, self.SupportsGuestQueries, "guest")
		updateTypes = append(updateTypes, tgbotapi.UpdateTypeGuestMessage)
	}
	return updateTypes
}

func warnIfModeDisabled(appParams *Params, enabled bool, mode string) {
	if appParams.API != nil && !enabled {
		log.Warn("Handlers are registered, but the mode is disabled for the bot",
			logconst.FieldFunc, "AllowedUpdates",
			"mode", mode)
	}
}

// HandleUpdate is the main router function for processing of [tgbotapi.Update].
func HandleUpdate(appParams *Params, wg *sync.WaitGroup, upd *tgbotapi.Update) {
	if upd.InlineQuery != nil {
		wg.Add(1)
		go func(query tgbotapi.InlineQuery) {
			defer wg.Done()
			processInline(appParams, &query)
		}(*upd.InlineQuery) // copy by value
	} else if upd.ChosenInlineResult != nil {
		metrics.Inc(metrics.ChosenInlineResultCounter)
	} else if upd.Message != nil {
		wg.Add(1)
		go func(msg tgbotapi.Message) {
			defer wg.Done()
			processMessage(appParams, &msg)
		}(*upd.Message) // copy by value
	} else if upd.CallbackQuery != nil {
		wg.Add(1)
		go func(query tgbotapi.CallbackQuery) {
			defer wg.Done()
			processCallbackQuery(appParams, &query)
		}(*upd.CallbackQuery) // copy by value
	} else if upd.GuestMessage != nil {
		wg.Add(1)
		go func(msg tgbotapi.Message) {
			defer wg.Done()
			processGuestMessage(appParams, &msg)
		}(*upd.GuestMessage) // copy by value
	}
}

func processMessage(appParams *Params, msg *tgbotapi.Message) {
	lang, opts := appParams.Settings.FetchUserOptions(msg.From.ID, msg.From.LanguageCode)
	lc := appParams.LangPool.GetContext(string(lang))
	reqenv := base.NewRequestEnv(lc, opts)
	appenv := NewAppEnv(appParams)

	// for commands and other handlers
	for _, handler := range appParams.MessageHandlers {
		if handler.CanHandle(reqenv, msg) {
			metrics.IncMessageHandlerCounter(handler)
			handler.Handle(reqenv, msg)
			return
		}
	}

	// If no handler was chosen, check if this is a parameter for some previously created form.
	var form wizard.Form
	err := appParams.StateStorage.GetCurrentState(msg.From.ID, &form)
	if err == nil {
		resources := wizard.NewEnv(appenv, appParams.StateStorage)
		form.PopulateRestored(msg, resources)
		form.ProcessNextField(reqenv, msg)
		return
	}
	if !errors.Is(err, wizard.ErrNoState) {
		log.Error("Failed to get a current state",
			logconst.FieldFunc, "processMessage",
			logconst.FieldError, err)
		return
	}

	// fallback/default handler
	var defMsgTr string
	if msg.IsCommand() {
		defMsgTr = defaultMessageOnCommandTr
	} else {
		defMsgTr = defaultMessageTr
	}
	appenv.Bot.Reply(msg, reqenv.Lang.Tr(defMsgTr))
}

func processInline(appParams *Params, query *tgbotapi.InlineQuery) {
	lang, opts := appParams.Settings.FetchUserOptions(query.From.ID, query.From.LanguageCode)
	lc := appParams.LangPool.GetContext(string(lang))
	reqenv := base.NewRequestEnv(lc, opts)

	for _, handler := range appParams.InlineHandlers {
		if handler.CanHandle(reqenv, query) {
			metrics.IncInlineHandlerCounter(handler)
			handler.Handle(reqenv, query)
			return
		}
	}
}

func processGuestMessage(appParams *Params, msg *tgbotapi.Message) {
	user := msg.From
	if user == nil {
		user = msg.GuestBotCallerUser
	}
	if user == nil {
		log.Warn("Guest message without a user",
			logconst.FieldFunc, "processGuestMessage",
			logconst.FieldMessage, msg)
		return
	}

	lang, opts := appParams.Settings.FetchUserOptions(user.ID, user.LanguageCode)
	lc := appParams.LangPool.GetContext(string(lang))
	reqenv := base.NewRequestEnv(lc, opts)
	answerGuestMessage(NewAppEnv(appParams), reqenv, appParams.GuestHandlers, msg)
}

// answerGuestMessage answers the guest query exactly once: with the result of the first suitable handler, or with
// the default message if there is no such handler or it has returned nil.
// Unlike processMessage, it never touches the state storage since the message was sent in a chat with other people.
func answerGuestMessage(appenv *base.ApplicationEnv, reqenv *base.RequestEnv, handlers []base.GuestHandler, msg *tgbotapi.Message) {
	if msg.GuestQueryID == "" {
		log.Warn("Guest message without a query ID",
			logconst.FieldFunc, "answerGuestMessage",
			logconst.FieldMessage, msg)
		return
	}

	var result tgbotapi.InlineQueryResult
	for _, handler := range handlers {
		if handler.CanHandle(reqenv, msg) {
			metrics.IncGuestHandlerCounter(handler)
			result = handler.Handle(reqenv, msg)
			break
		}
	}
	if result == nil {
		text := reqenv.Lang.Tr(defaultMessageTr)
		result = tgbotapi.NewInlineQueryResultArticle(defaultGuestResultID, text, text)
	}

	if err := appenv.Bot.Request(tgbotapi.NewAnswerGuestQuery(msg.GuestQueryID, result)); err != nil {
		log.Error("Failed to answer the guest query",
			logconst.FieldFunc, "answerGuestMessage",
			logconst.FieldCalledMethod, "AnswerGuestQuery",
			logconst.FieldError, err)
	}
}

func processCallbackQuery(appParams *Params, query *tgbotapi.CallbackQuery) {
	lang, opts := appParams.Settings.FetchUserOptions(query.From.ID, query.From.LanguageCode)
	lc := appParams.LangPool.GetContext(string(lang))
	reqenv := base.NewRequestEnv(lc, opts)

	splitData := strings.SplitN(query.Data, ":", 2)
	if len(splitData) < 2 {
		log.Warn("Unexpected callback",
			logconst.FieldFunc, "processCallbackQuery",
			logconst.FieldQuery, query)
		return
	}
	prefix := splitData[0] + ":"

	// special case for the wizard callback, otherwise check other [base.CallbackHandler]s
	if prefix == wizard.CallbackDataFieldPrefix {
		resources := wizard.NewEnv(NewAppEnv(appParams), appParams.StateStorage)
		wizard.CallbackQueryHandler(reqenv, query, resources)
	} else {
		for _, handler := range appParams.CallbackHandlers {
			if prefix == handler.GetCallbackPrefix() {
				handler.Handle(reqenv, query)
				return
			}
		}
	}
}
