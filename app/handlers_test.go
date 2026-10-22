package app

import (
	"slices"
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/loctools/go-l10n/loc"
	"github.com/stretchr/testify/assert"
)

const testGuestQueryID = "guest-query"

type fakeGuestHandler struct {
	text   string
	result tgbotapi.InlineQueryResult
	called bool
}

func (h *fakeGuestHandler) CanHandle(_ *base.RequestEnv, msg *tgbotapi.Message) bool {
	return msg.Text == h.text
}

func (h *fakeGuestHandler) Handle(*base.RequestEnv, *tgbotapi.Message) tgbotapi.InlineQueryResult {
	h.called = true
	return h.result
}

func TestAnswerGuestMessage(t *testing.T) {
	result := tgbotapi.NewInlineQueryResultArticle("id", "title", "text")
	defaultResult := tgbotapi.NewInlineQueryResultArticle(defaultGuestResultID, defaultMessageTr, defaultMessageTr)

	t.Run("handled", func(t *testing.T) {
		first := &fakeGuestHandler{text: "foo", result: result}
		second := &fakeGuestHandler{text: "foo"}
		requests := answerGuestMessageForTest(t, []base.GuestHandler{first, second}, newGuestMessage("foo"))

		assert.True(t, first.called)
		assert.False(t, second.called)
		assert.Equal(t, []tgbotapi.Chattable{tgbotapi.NewAnswerGuestQuery(testGuestQueryID, result)}, requests)
	})
	t.Run("nil result", func(t *testing.T) {
		handler := &fakeGuestHandler{text: "foo"}
		requests := answerGuestMessageForTest(t, []base.GuestHandler{handler}, newGuestMessage("foo"))

		assert.True(t, handler.called)
		assert.Equal(t, []tgbotapi.Chattable{tgbotapi.NewAnswerGuestQuery(testGuestQueryID, defaultResult)}, requests)
	})
	t.Run("no handler", func(t *testing.T) {
		handler := &fakeGuestHandler{text: "foo", result: result}
		requests := answerGuestMessageForTest(t, []base.GuestHandler{handler}, newGuestMessage("bar"))

		assert.False(t, handler.called)
		assert.Equal(t, []tgbotapi.Chattable{tgbotapi.NewAnswerGuestQuery(testGuestQueryID, defaultResult)}, requests)
	})
	t.Run("no query ID", func(t *testing.T) {
		handler := &fakeGuestHandler{text: "foo", result: result}
		msg := newGuestMessage("foo")
		msg.GuestQueryID = ""
		requests := answerGuestMessageForTest(t, []base.GuestHandler{handler}, msg)

		assert.False(t, handler.called)
		assert.Empty(t, requests)
	})
}

type fakeInlineHandler struct{}

func (fakeInlineHandler) CanHandle(*base.RequestEnv, *tgbotapi.InlineQuery) bool { return true }
func (fakeInlineHandler) Handle(*base.RequestEnv, *tgbotapi.InlineQuery)         {}

func TestAllowedUpdates(t *testing.T) {
	always := []string{tgbotapi.UpdateTypeMessage, tgbotapi.UpdateTypeCallbackQuery}
	inline := []string{tgbotapi.UpdateTypeInlineQuery, tgbotapi.UpdateTypeChosenInlineResult}
	guest := []string{tgbotapi.UpdateTypeGuestMessage}

	assert.Equal(t, always, AllowedUpdates(&Params{}))
	assert.Equal(t, slices.Concat(always, inline), AllowedUpdates(&Params{
		InlineHandlers: []base.InlineHandler{fakeInlineHandler{}},
	}))
	assert.Equal(t, slices.Concat(always, guest), AllowedUpdates(&Params{
		GuestHandlers: []base.GuestHandler{&fakeGuestHandler{}},
	}))
	assert.Equal(t, slices.Concat(always, inline, guest), AllowedUpdates(&Params{
		InlineHandlers: []base.InlineHandler{fakeInlineHandler{}},
		GuestHandlers:  []base.GuestHandler{&fakeGuestHandler{}},
	}))
}

func newGuestMessage(text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Text:         text,
		From:         &tgbotapi.User{ID: 1},
		GuestQueryID: testGuestQueryID,
	}
}

func answerGuestMessageForTest(t *testing.T, handlers []base.GuestHandler, msg *tgbotapi.Message) []tgbotapi.Chattable {
	t.Helper()
	bot := &base.FakeBotAPI{}
	appenv := &base.ApplicationEnv{Bot: bot}
	reqenv := base.NewRequestEnv(loc.NewPool("en").GetContext("en"), nil)

	answerGuestMessage(appenv, reqenv, handlers, msg)

	requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
	return requests
}
