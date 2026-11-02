package selfdestruct

import (
	"context"
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func newSchedulerForTest(bot base.ExtendedBotAPI) *scheduler {
	return &scheduler{ctx: context.Background(), bot: bot, queue: NewMemoryQueue()}
}

func TestExecuteAll_DeletesByOneRequestPerChat(t *testing.T) {
	bot := &base.FakeBotAPI{}
	newSchedulerForTest(bot).executeAll([]Task{
		{Kind: KindDelete, ChatID: 1, MessageID: 1},
		{Kind: KindReplaceInline, InlineMessageID: "inline", Text: "·"},
		{Kind: KindDelete, ChatID: 2, MessageID: 3},
		{Kind: KindDelete, ChatID: 1, MessageID: 2},
	})

	assert.ElementsMatch(t, []tgbotapi.Chattable{
		tgbotapi.NewDeleteMessages(1, []int{1, 2}),
		tgbotapi.NewDeleteMessage(2, 3),
		tgbotapi.EditMessageTextConfig{BaseEdit: tgbotapi.BaseEdit{InlineMessageID: "inline"}, Text: "·"},
	}, requests(bot))
}

func TestExecuteAll_DeletesTheMessagesOfATaskTogether(t *testing.T) {
	bot := &base.FakeBotAPI{}
	newSchedulerForTest(bot).executeAll([]Task{
		{Kind: KindDelete, ChatID: 1, MessageID: 1, WithMessageIDs: []int{2}},
		{Kind: KindDelete, ChatID: 1, MessageID: 2}, // the same message is deleted once
	})

	assert.Equal(t, []tgbotapi.Chattable{tgbotapi.NewDeleteMessages(1, []int{1, 2})}, requests(bot))
}

func TestExecuteAll_DeletesOneByOneIfRefused(t *testing.T) {
	bot := &base.FakeBotAPI{RequestError: func(c tgbotapi.Chattable) error {
		if _, ok := c.(tgbotapi.DeleteMessagesConfig); ok {
			return &tgbotapi.Error{Code: 400, Message: "Bad Request: message can't be deleted"}
		}
		return nil
	}}
	newSchedulerForTest(bot).executeAll([]Task{
		{Kind: KindDelete, ChatID: 1, MessageID: 1},
		{Kind: KindDelete, ChatID: 1, MessageID: 2},
	})

	assert.Equal(t, []tgbotapi.Chattable{
		tgbotapi.NewDeleteMessages(1, []int{1, 2}),
		tgbotapi.NewDeleteMessage(1, 1),
		tgbotapi.NewDeleteMessage(1, 2),
	}, requests(bot))
}

func TestExecuteAll_PostponesByFloodControl(t *testing.T) {
	bot := &base.FakeBotAPI{RequestError: func(tgbotapi.Chattable) error {
		return &tgbotapi.Error{
			Code:               429,
			Message:            "Too Many Requests: retry after 5",
			ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 5},
		}
	}}
	s := newSchedulerForTest(bot)
	tasks := []Task{
		{Kind: KindDelete, ChatID: 1, MessageID: 1},
		{Kind: KindDelete, ChatID: 1, MessageID: 2},
		{Kind: KindReplaceInline, InlineMessageID: "inline", Text: "·"},
	}
	s.executeAll(tasks)

	ctx := context.Background()
	due, err := s.queue.TakeDue(ctx, time.Now(), 10)
	require.NoError(t, err)
	assert.Empty(t, due, "the tasks must wait for retry_after")

	due, err = s.queue.TakeDue(ctx, time.Now().Add(6*time.Second), 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, tasks, due)
	assert.Len(t, requests(bot), 2, "no deletions one by one after the flood control")
}

func TestExecuteAll_CountsOutcomes(t *testing.T) {
	refused := &tgbotapi.Error{Code: 400, Message: "Bad Request: message can't be deleted"}
	bot := &base.FakeBotAPI{RequestError: func(c tgbotapi.Chattable) error {
		switch req := c.(type) {
		case tgbotapi.DeleteMessageConfig:
			if req.MessageID == 2 {
				return refused
			}
		case tgbotapi.EditMessageTextConfig:
			return errors.New("dial tcp: connection refused")
		}
		return nil
	}}
	counter := func(kind TaskKind, outcome string) float64 {
		return testutil.ToFloat64(messagesCounter.WithLabelValues(string(kind), outcome))
	}
	done, refusedBefore, failed := counter(KindDelete, outcomeDone), counter(KindDelete, outcomeRefused), counter(KindReplaceInline, outcomeFailed)

	newSchedulerForTest(bot).executeAll([]Task{
		{Kind: KindDelete, ChatID: 1, MessageID: 1},
		{Kind: KindDelete, ChatID: 2, MessageID: 2},
		{Kind: KindReplaceInline, InlineMessageID: "inline", Text: "·"},
	})

	assert.Equal(t, done+1, counter(KindDelete, outcomeDone))
	assert.Equal(t, refusedBefore+1, counter(KindDelete, outcomeRefused))
	assert.Equal(t, failed+1, counter(KindReplaceInline, outcomeFailed))
}

func TestExecuteAll_RecoversFromPanic(t *testing.T) {
	bot := &base.FakeBotAPI{RequestError: func(tgbotapi.Chattable) error { panic("boom") }}
	panics := func() float64 { return panicsOfWorker(t) }
	before := panics()

	assert.NotPanics(t, func() {
		newSchedulerForTest(bot).executeAll([]Task{
			{Kind: KindDelete, ChatID: 1, MessageID: 1},
			{Kind: KindDelete, ChatID: 2, MessageID: 2},
		})
	})
	assert.Equal(t, before+2, panics())
}

// panicsOfWorker reads panics_total{kind="self_destruction"} from the default registry, where the metrics package
// registers it.
func panicsOfWorker(t *testing.T) float64 {
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "panics_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "kind" && label.GetValue() == panicKind {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	t.Fatal("panics_total of the worker isn't registered")
	return 0
}

func TestIsExpectedDeletionFailure(t *testing.T) {
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message can't be deleted"}))
	assert.True(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: message to delete not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 400, Message: "Bad Request: chat not found"}))
	assert.False(t, isExpectedDeletionFailure(&tgbotapi.Error{Code: 429, Message: "Too Many Requests: retry after 5"}))
	assert.False(t, isExpectedDeletionFailure(errors.New("dial tcp: connection refused")))
}
