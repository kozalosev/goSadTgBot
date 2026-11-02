package selfdestruct

import (
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/stretchr/testify/assert"
)

func TestSendAndDelete(t *testing.T) {
	bot := &base.FakeBotAPI{}
	SendAndDelete(bot, tgbotapi.NewMessage(1, "text"), time.Millisecond)

	assert.Eventually(t, func() bool {
		requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
		return len(requests) == 2
	}, time.Second, time.Millisecond)
	requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
	assert.IsType(t, tgbotapi.MessageConfig{}, requests[0])
	assert.IsType(t, tgbotapi.DeleteMessageConfig{}, requests[1])
}

func TestReplaceInline(t *testing.T) {
	bot := &base.FakeBotAPI{}
	ReplaceInline(bot, base.FakeInlineMessageID, "·", time.Millisecond)

	assert.Eventually(t, func() bool {
		requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
		return len(requests) == 1
	}, time.Second, time.Millisecond)
	requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
	edit, ok := requests[0].(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Equal(t, base.FakeInlineMessageID, edit.InlineMessageID)
	assert.Equal(t, "·", edit.Text)
}

func TestIsExpectedDeletionFailure(t *testing.T) {
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message can't be deleted"}))
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message to delete not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: chat not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 429, Message: "Too Many Requests: retry after 5"}))
	assert.False(t, isExpectedDeletionFailure(errors.New("dial tcp: connection refused")))
}
