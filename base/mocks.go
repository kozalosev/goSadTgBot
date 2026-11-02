package base

import (
	"slices"
	"sync"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/loctools/go-l10n/loc"
)

type callType byte

const (
	message callType = iota
	request
)

// FakeInlineMessageID is returned by [FakeBotAPI.AnswerGuestQuery].
const FakeInlineMessageID = "fake-inline-message-id"

// FakeBotAPI is a mock for the [BotAPI] struct. It's safe for concurrent use.
// Use the GetOutput() method to get either the text of the sent message, or the request itself.
type FakeBotAPI struct {
	// DeletionAllowed is returned by CanDeleteMessages.
	DeletionAllowed bool
	// RequestError, if set, returns the error for a request made by Request or Send. The request is recorded anyway.
	RequestError func(c tgbotapi.Chattable) error

	mu           sync.Mutex
	sentMessages []string
	sentRequests []tgbotapi.Chattable
	callType     callType
}

func (bot *FakeBotAPI) GetName() string                                   { return "TestMockBotAPI" }
func (bot *FakeBotAPI) SetCommands(*loc.Pool, []string, []CommandHandler) {}
func (bot *FakeBotAPI) ReplyWithMessageCustomizer(_ *tgbotapi.Message, text string, _ MessageCustomizer) {
	bot.reply(text)
}
func (bot *FakeBotAPI) Reply(_ *tgbotapi.Message, text string)             { bot.reply(text) }
func (bot *FakeBotAPI) ReplyWithMarkdown(_ *tgbotapi.Message, text string) { bot.reply(text) }
func (bot *FakeBotAPI) ReplyWithKeyboard(_ *tgbotapi.Message, text string, _ []string) {
	bot.reply(text)
}
func (bot *FakeBotAPI) ReplyWithInlineKeyboard(_ *tgbotapi.Message, text string, _ []tgbotapi.InlineKeyboardButton) {
	bot.reply(text)
}

func (bot *FakeBotAPI) reply(text string) {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	bot.callType = message
	bot.sentMessages = append(bot.sentMessages, text)
}

func (bot *FakeBotAPI) record(c tgbotapi.Chattable) error {
	bot.mu.Lock()
	bot.callType = request
	bot.sentRequests = append(bot.sentRequests, c)
	bot.mu.Unlock()
	if bot.RequestError != nil {
		return bot.RequestError(c)
	}
	return nil
}

func (bot *FakeBotAPI) Request(c tgbotapi.Chattable) error {
	return bot.record(c)
}

func (bot *FakeBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, bot.record(c)
}

func (bot *FakeBotAPI) AnswerGuestQuery(c tgbotapi.AnswerGuestQueryConfig) (tgbotapi.SentGuestMessage, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	bot.callType = request
	bot.sentRequests = append(bot.sentRequests, c)
	return tgbotapi.SentGuestMessage{InlineMessageID: FakeInlineMessageID}, nil
}

func (bot *FakeBotAPI) CanDeleteMessages(int64) bool {
	return bot.DeletionAllowed
}

func (bot *FakeBotAPI) GetStandardAPI() *tgbotapi.BotAPI {
	return nil
}

// GetOutput returns either a string after usage of Reply*() methods or a [tgbotapi.Chattable] after Request().
// It returns a copy, so the slice doesn't change if the bot is used concurrently.
func (bot *FakeBotAPI) GetOutput() any {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	switch bot.callType {
	case message:
		return slices.Clone(bot.sentMessages)
	case request:
		return slices.Clone(bot.sentRequests)
	default:
		return nil
	}
}

// ClearOutput deletes all data from internal buffers.
func (bot *FakeBotAPI) ClearOutput() {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	bot.sentMessages = []string{}
	bot.sentRequests = []tgbotapi.Chattable{}
}
