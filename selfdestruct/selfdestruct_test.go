package selfdestruct

import (
	"context"
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startForTest starts a worker with a queue in memory, which polls often enough for the tests.
func startForTest(t *testing.T, bot base.ExtendedBotAPI) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	Start(ctx, bot, NewMemoryQueue(), time.Millisecond)
}

func requests(bot *base.FakeBotAPI) []tgbotapi.Chattable {
	reqs, _ := bot.GetOutput().([]tgbotapi.Chattable)
	return reqs
}

func TestSendAndDelete(t *testing.T) {
	bot := &base.FakeBotAPI{}
	startForTest(t, bot)
	SendAndDelete(bot, tgbotapi.NewMessage(1, "text"), time.Millisecond)

	assert.Eventually(t, func() bool { return len(requests(bot)) == 2 }, time.Second, time.Millisecond)
	assert.IsType(t, tgbotapi.MessageConfig{}, requests(bot)[0])
	assert.IsType(t, tgbotapi.DeleteMessageConfig{}, requests(bot)[1])
}

func TestReplaceInline(t *testing.T) {
	bot := &base.FakeBotAPI{}
	startForTest(t, bot)
	ReplaceInline(bot, base.FakeInlineMessageID, "·", time.Millisecond)

	assert.Eventually(t, func() bool { return len(requests(bot)) == 1 }, time.Second, time.Millisecond)
	edit, ok := requests(bot)[0].(tgbotapi.EditMessageTextConfig)
	require.True(t, ok)
	assert.Equal(t, base.FakeInlineMessageID, edit.InlineMessageID)
	assert.Equal(t, "·", edit.Text)
}

func TestCancelDeletion(t *testing.T) {
	bot := &base.FakeBotAPI{}
	startForTest(t, bot)
	Schedule(bot, Task{Kind: KindDelete, ChatID: 1, MessageID: 2, ReplyToMessageID: 3}, 20*time.Millisecond)

	task, ok := CancelDeletion(bot, 1, 2)
	require.True(t, ok)
	assert.Equal(t, 3, task.ReplyToMessageID)
	assert.Never(t, func() bool { return len(requests(bot)) > 0 }, 60*time.Millisecond, 5*time.Millisecond)

	_, ok = CancelDeletion(bot, 1, 2)
	assert.False(t, ok, "the deletion has already been cancelled")
}

func TestIsExpectedDeletionFailure(t *testing.T) {
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message can't be deleted"}))
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message to delete not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: chat not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 429, Message: "Too Many Requests: retry after 5"}))
	assert.False(t, isExpectedDeletionFailure(errors.New("dial tcp: connection refused")))
}
