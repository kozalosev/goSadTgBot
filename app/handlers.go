package app

import (
	"fmt"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/go-redis/redis/v8"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/metrics"
	"github.com/kozalosev/goSadTgBot/wizard"
	log "log/slog"
	"strings"
	"sync"
)

const (
	defaultMessageTr          = "commands.default.message"
	defaultMessageOnCommandTr = "commands.default.message.on.command"
)

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
		form.FixDataTypes()
		form.ProcessNextField(reqenv, msg)
		return
	}
	if err != redis.Nil {
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

func processCallbackQuery(appParams *Params, query *tgbotapi.CallbackQuery) {
	lang, opts := appParams.Settings.FetchUserOptions(query.From.ID, query.From.LanguageCode)
	lc := appParams.LangPool.GetContext(string(lang))
	reqenv := base.NewRequestEnv(lc, opts)

	splitData := strings.SplitN(query.Data, ":", 2)
	if len(splitData) < 2 {
		log.Warn("Unexpected callback",
			logconst.FieldFunc, "processCallbackQuery",
			logconst.FieldQuery, fmt.Sprintf("%+v", query))
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
