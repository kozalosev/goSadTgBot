package base

import (
	log "log/slog"

	"slices"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/settings"
	"github.com/loctools/go-l10n/loc"
	"github.com/samber/lo"
)

const cmdTrTemplate = "commands.%s.description"

var (
	NoOpCustomizer     MessageCustomizer = func(msgConfig *tgbotapi.MessageConfig) {}
	MarkdownCustomizer MessageCustomizer = func(msgConfig *tgbotapi.MessageConfig) {
		msgConfig.ParseMode = tgbotapi.ModeMarkdown
	}
)

func ConvertHandlersToCommands(handlers []MessageHandler) []CommandHandler {
	var commands []CommandHandler
	for _, h := range handlers {
		if cmd, ok := h.(CommandHandler); ok {
			commands = append(commands, cmd)
		}
	}
	return commands
}

// NewReplier is a shortcut to reply with text in the user's language.
// Example:
//
//	reply := base.NewReplier(handler.appenv, reqenv, msg)
//	reply("messages.success")
func NewReplier(appenv *ApplicationEnv, reqenv *RequestEnv, msg *tgbotapi.Message) func(string) {
	return func(statusKey string) {
		appenv.Bot.Reply(msg, reqenv.Lang.Tr(statusKey))
	}
}

// IsGroupChat tells if the chat is a group or a supergroup.
func IsGroupChat(chat *tgbotapi.Chat) bool {
	return chat.IsGroup() || chat.IsSuperGroup()
}

func NewRequestEnv(langCtx *loc.Context, opts settings.UserOptions) *RequestEnv {
	return &RequestEnv{
		Lang:    langCtx,
		Options: opts,
	}
}

func (t CommandHandlerTrait) CanHandle(_ *RequestEnv, msg *tgbotapi.Message) bool {
	return slices.Contains(t.HandlerRefForTrait.GetCommands(), msg.Command())
}

func NewBotAPI(api *tgbotapi.BotAPI) *BotAPI {
	return &BotAPI{internal: api}
}

func (bot *BotAPI) GetName() string {
	return bot.internal.Self.UserName
}

func (bot *BotAPI) SetCommands(locpool *loc.Pool, langCodes []string, handlers []CommandHandler) {
	for _, langCode := range langCodes {
		lc := locpool.GetContext(langCode)
		for _, scope := range commandScopes {
			tgScope := tgbotapi.BotCommandScope{Type: string(scope)}
			commands := filterCommandsByScope(handlers, scope, lc)
			req := tgbotapi.NewSetMyCommandsWithScopeAndLanguage(tgScope, langCode, commands...)

			logger := log.With(
				logconst.FieldFunc, "setCommands",
				logconst.FieldCalledObject, "BotAPI",
				logconst.FieldCalledMethod, "Request",
			)
			if err := bot.Request(req); err != nil {
				logger.Error("Failed to update the bot commands",
					logconst.FieldError, err)
			} else {
				logger.Info("Commands were successfully updated!")
			}
		}
	}
}

func (bot *BotAPI) ReplyWithMessageCustomizer(msg *tgbotapi.Message, text string, customizer MessageCustomizer) {
	if len(text) == 0 {
		log.Error("Empty reply for the message",
			logconst.FieldMessage, msg.Text,
			logconst.FieldObject, "BotAPI",
			logconst.FieldMethod, "ReplyWithMessageCustomizer")
		return
	}

	reply := newReply(msg, text)
	customizer(&reply)
	if _, err := bot.internal.Send(reply); err != nil {
		log.Error("Failed to send a reply",
			logconst.FieldObject, "BotAPI",
			logconst.FieldMethod, "ReplyWithMessageCustomizer",
			logconst.FieldCalledObject, "internal",
			logconst.FieldCalledMethod, "Send",
			logconst.FieldError, err)
	}
}

func (bot *BotAPI) Reply(msg *tgbotapi.Message, text string) {
	bot.ReplyWithMessageCustomizer(msg, text, NoOpCustomizer)
}

func (bot *BotAPI) ReplyWithMarkdown(msg *tgbotapi.Message, text string) {
	bot.ReplyWithMessageCustomizer(msg, text, MarkdownCustomizer)
}

func (bot *BotAPI) ReplyWithKeyboard(msg *tgbotapi.Message, text string, options []string) {
	buttons := lo.Map(options, func(s string, _ int) tgbotapi.KeyboardButton {
		return tgbotapi.NewKeyboardButton(s)
	})
	rows := chunkBy(buttons, buttonsPerRow)
	keyboard := tgbotapi.NewOneTimeReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true

	bot.ReplyWithMessageCustomizer(msg, text, func(msgConfig *tgbotapi.MessageConfig) {
		msgConfig.ReplyMarkup = keyboard
	})
}

func (bot *BotAPI) ReplyWithInlineKeyboard(msg *tgbotapi.Message, text string, buttons []tgbotapi.InlineKeyboardButton) {
	rows := chunkBy(buttons, buttonsPerRow)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.ReplyWithMessageCustomizer(msg, text, func(msgConfig *tgbotapi.MessageConfig) {
		msgConfig.ReplyMarkup = keyboard
	})
}

// Request is a simple wrapper around [tgbotapi.BotAPI.Request].
func (bot *BotAPI) Request(c tgbotapi.Chattable) error {
	_, err := bot.internal.Request(c)
	return err
}

// Send is an even simpler wrapper around [tgbotapi.BotAPI.Send].
func (bot *BotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return bot.internal.Send(c)
}

// AnswerGuestQuery is a simple wrapper around [tgbotapi.BotAPI.AnswerGuestQuery].
func (bot *BotAPI) AnswerGuestQuery(config tgbotapi.AnswerGuestQueryConfig) (tgbotapi.SentGuestMessage, error) {
	return bot.internal.AnswerGuestQuery(config)
}

func (bot *BotAPI) GetStandardAPI() *tgbotapi.BotAPI {
	return bot.internal
}

// newReply creates a reply to the message. If the message is ephemeral (a command marked with
// [tgbotapi.BotCommand.IsEphemeral] was used in a group), the reply is ephemeral too and visible only to the sender.
// Such a reply must be sent within 15 seconds after the command.
func newReply(msg *tgbotapi.Message, text string) tgbotapi.MessageConfig {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	if msg.EphemeralMessageID != 0 && msg.From != nil {
		reply.ReplyParameters.EphemeralMessageID = msg.EphemeralMessageID
		reply.EphemeralMessageParameters = tgbotapi.EphemeralMessageParameters{ReceiverUserID: msg.From.ID}
	} else {
		reply.ReplyParameters.MessageID = msg.MessageID
	}
	return reply
}
