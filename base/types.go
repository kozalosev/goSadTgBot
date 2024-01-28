// Package base defines basic types and a wrapper around the original [tgbotapi.BotAPI] struct.
package base

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kozalosev/goSadTgBot/settings"
	"github.com/loctools/go-l10n/loc"
)

// MessageHandler is a handler for the [tgbotapi.Message] update type.
type MessageHandler interface {
	CanHandle(reqenv *RequestEnv, msg *tgbotapi.Message) bool
	Handle(reqenv *RequestEnv, msg *tgbotapi.Message)
}

type CommandScope string

const (
	CommandScopeDefault         CommandScope = "default"
	CommandScopeAllPrivateChats CommandScope = "all_private_chats"
	CommandScopeAllGroupChats   CommandScope = "all_group_chats"
	CommandScopeAllChatAdmins   CommandScope = "all_chat_administrators"
)

var commandScopes = []CommandScope{
	CommandScopeDefault,
	CommandScopeAllPrivateChats,
	CommandScopeAllGroupChats,
	CommandScopeAllChatAdmins,
}

// CommandHandler is a MessageHandler which is considered as a public command that will be registered automatically.
// https://core.telegram.org/bots/api#setmycommands
// Inject [CommandHandlerTrait] to get the default implementation of CanHandle().
type CommandHandler interface {
	MessageHandler

	GetCommands() []string
	GetScopes() []CommandScope
}

// InlineHandler is a handler for the [tgbotapi.InlineQuery] update type.
type InlineHandler interface {
	CanHandle(reqenv *RequestEnv, query *tgbotapi.InlineQuery) bool
	Handle(reqenv *RequestEnv, query *tgbotapi.InlineQuery)
}

// CallbackHandler is a handler for the [tgbotapi.CallbackQuery] update type.
type CallbackHandler interface {
	GetCallbackPrefix() string
	Handle(reqenv *RequestEnv, query *tgbotapi.CallbackQuery)
}

// MessageCustomizer is a function that can change the message before it will be sent to Telegram.
// See [BotAPI.ReplyWithMessageCustomizer] for more information.
type MessageCustomizer func(msgConfig *tgbotapi.MessageConfig)

// ExtendedBotAPI is a set of more convenient methods to work with Telegram Bot API.
type ExtendedBotAPI interface {
	// GetName returns the name of the bot got from the getMe() request.
	GetName() string
	// SetCommands for the bot. Use [ConvertHandlersToCommands] to pass [MessageHandler] as argument for this method.
	// Descriptions must be provided as keys for [loc.Context] in the format "commands.%s.description".
	// https://core.telegram.org/bots/api#setmycommands
	SetCommands(locpool *loc.Pool, langCodes []string, handlers []CommandHandler)
	// ReplyWithMessageCustomizer is the most common method to send text messages as a reply. Use this method if you want
	// to change several options like a message in Markdown with an inline keyboard.
	ReplyWithMessageCustomizer(*tgbotapi.Message, string, MessageCustomizer)
	// Reply with just a text message, without any customizations.
	Reply(msg *tgbotapi.Message, text string)
	ReplyWithMarkdown(msg *tgbotapi.Message, text string)
	// ReplyWithKeyboard uses a one time reply keyboard.
	// https://core.telegram.org/bots/api#replykeyboardmarkup
	ReplyWithKeyboard(msg *tgbotapi.Message, text string, options []string)
	// ReplyWithInlineKeyboard attaches an inline keyboard to the message.
	// https://core.telegram.org/bots/api#inlinekeyboardmarkup
	ReplyWithInlineKeyboard(msg *tgbotapi.Message, text string, buttons []tgbotapi.InlineKeyboardButton)
	// Request is the most common method that can be used to send any request to Telegram.
	Request(tgbotapi.Chattable) error
	// Send is like the Request method but returns the sent message back with non-empty ID field.
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	// GetStandardAPI lets you use all standard methods of the library.
	GetStandardAPI() *tgbotapi.BotAPI
}

type CommandHandlerTrait struct {
	HandlerRefForTrait CommandHandler
}

// BotAPI is a wrapper around the original [tgbotapi.BotAPI] struct.
// It implements the [ExtendedBotAPI] interface.
type BotAPI struct {
	internal *tgbotapi.BotAPI
}

// ApplicationEnv is a container for all application scoped resources.
type ApplicationEnv struct {
	// Bot is used when you need to send a request to Telegram Bot API.
	Bot ExtendedBotAPI
	// Database is a reference to a [sql.DB]-like object.
	Database *pgxpool.Pool
	// Ctx is a context of the application; It's state will be switched to Done when the application is received the SIGTERM signal.
	Ctx context.Context
}

// RequestEnv is a container for all request related common resources. It's passed to all kinds of handlers.
type RequestEnv struct {
	// Lang is a localization container. You can get a message in the user's language by key, using its [loc.Context.Tr] method.
	Lang *loc.Context
	// Options is a container for user options fetched from the database.
	Options settings.UserOptions
}
