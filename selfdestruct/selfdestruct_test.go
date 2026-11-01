package selfdestruct

import (
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
