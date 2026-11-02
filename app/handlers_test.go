package app

import (
	"slices"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/loctools/go-l10n/loc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("placeholder result is unwrapped", func(t *testing.T) {
		result := tgbotapi.NewInlineQueryResultArticle("id", "title", "text")
		handler := &fakeGuestHandler{text: "foo", result: base.PlaceholderGuestResult{Result: result, After: time.Hour, Placeholder: "·"}}

		requests := answerGuestMessageForTest(t, []base.GuestHandler{handler}, newGuestMessage("foo"))
		assert.Len(t, requests, 1)
		answer, ok := requests[0].(tgbotapi.AnswerGuestQueryConfig)
		assert.True(t, ok)
		assert.Equal(t, result, answer.Result, "the wrapper must not be sent to Telegram")
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

func TestGoRecovered(t *testing.T) {
	before := gatheredPanics(t, tgbotapi.UpdateTypeMessage)

	var wg sync.WaitGroup
	goRecovered(&wg, tgbotapi.UpdateTypeMessage, func() { panic("boom") })
	wg.Wait()

	assert.Equal(t, before+1, gatheredPanics(t, tgbotapi.UpdateTypeMessage))
}

// gatheredPanics reads the value of panics_total for the kind from the default Prometheus registry.
func gatheredPanics(t *testing.T, kind string) float64 {
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "panics_total" {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, label := range m.GetLabel() {
				if label.GetName() == "kind" && label.GetValue() == kind {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestUpdateKind(t *testing.T) {
	assert.Equal(t, tgbotapi.UpdateTypeMessage, updateKind(&tgbotapi.Update{Message: &tgbotapi.Message{}}))
	assert.Equal(t, tgbotapi.UpdateTypeGuestMessage, updateKind(&tgbotapi.Update{GuestMessage: &tgbotapi.Message{}}))
	assert.Equal(t, tgbotapi.UpdateTypeEditedMessage, updateKind(&tgbotapi.Update{EditedMessage: &tgbotapi.Message{}}))
	assert.Equal(t, "other", updateKind(&tgbotapi.Update{}))
}
