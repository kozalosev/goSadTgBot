// Package selfdestruct removes the messages some time after they have been sent. The postponed actions are kept in
// a [Queue] and executed by a worker started by [Start]. With a [RedisQueue], they survive a restart of the
// application; without [Start], the package falls back to a [MemoryQueue] on the first use, and the messages that were
// not removed before a restart stay in the chats.
package selfdestruct

import (
	"context"
	"errors"
	log "log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/samber/lo"
)

const (
	// DefaultPollInterval is how often the worker started on the first use looks for the tasks whose time has come.
	DefaultPollInterval = time.Second

	tasksPerPoll          = 100
	maxMessagesPerRequest = 100 // the limit of deleteMessages
	maxConcurrentRequests = 8
)

var (
	globalSchedulerMu sync.Mutex
	globalScheduler   *scheduler
)

type scheduler struct {
	ctx   context.Context
	bot   base.ExtendedBotAPI
	queue Queue
}

// Start makes the package keep the tasks in the queue and starts the worker, which executes them every interval
// until ctx is done. Call it once at startup, before any message is scheduled.
func Start(ctx context.Context, bot base.ExtendedBotAPI, queue Queue, interval time.Duration) {
	s := &scheduler{ctx: ctx, bot: bot, queue: queue}
	globalSchedulerMu.Lock()
	globalScheduler = s
	globalSchedulerMu.Unlock()
	go s.run(interval)
}

// getScheduler returns the global scheduler or starts one with a [MemoryQueue] if [Start] hasn't been called.
func getScheduler(bot base.ExtendedBotAPI) *scheduler {
	globalSchedulerMu.Lock()
	defer globalSchedulerMu.Unlock()
	if globalScheduler == nil {
		log.Warn("selfdestruct.Start wasn't called: the tasks are kept in memory and will be lost on restart",
			logconst.FieldFunc, "selfdestruct.getScheduler")
		globalScheduler = &scheduler{ctx: context.Background(), bot: bot, queue: NewMemoryQueue()}
		go globalScheduler.run(DefaultPollInterval)
	}
	return globalScheduler
}

func (s *scheduler) run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			lastTickGauge.SetToCurrentTime()
			s.executeDue()
		}
	}
}

func (s *scheduler) executeDue() {
	defer recoverPanic("selfdestruct.executeDue")
	for {
		tasks, err := s.queue.TakeDue(s.ctx, time.Now(), tasksPerPoll)
		if err != nil {
			queueErrorsCounter.WithLabelValues(operationTakeDue).Inc()
			log.Error("Failed to take the due tasks from the queue",
				logconst.FieldFunc, "selfdestruct.executeDue",
				logconst.FieldCalledMethod, "TakeDue",
				logconst.FieldError, err)
		}
		batchSizeHistogram.Observe(float64(len(tasks)))
		if len(tasks) > 0 {
			start := time.Now()
			s.executeAll(tasks)
			batchDurationHistogram.Observe(time.Since(start).Seconds())
		}
		if err != nil || len(tasks) < tasksPerPoll {
			return
		}
	}
}

// executeAll executes the tasks concurrently, up to maxConcurrentRequests requests at once, and returns when all of
// them are done. The messages of a chat are deleted by one request and not concurrently, since the flood control of
// Telegram limits the requests per chat.
func (s *scheduler) executeAll(tasks []Task) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentRequests)
	goLimited := func(f func()) {
		semaphore <- struct{}{}
		wg.Go(func() {
			defer func() { <-semaphore }()
			defer recoverPanic("selfdestruct.executeAll")
			f()
		})
	}

	deletions, others := lo.FilterReject(tasks, func(task Task, _ int) bool { return task.Kind == KindDelete })
	for chatID, chatDeletions := range lo.GroupBy(deletions, func(task Task) int64 { return task.ChatID }) {
		goLimited(func() { s.deleteInChat(chatID, chatDeletions) })
	}
	for _, task := range others {
		goLimited(func() { s.execute(task) })
	}
	wg.Wait()
}

// deleteInChat deletes the messages of one chat by a single request (or a few, if there are more of them than one
// request takes). If Telegram refuses it, the messages are deleted one by one: the documentation doesn't tell what
// happens if only some of them can't be deleted, and the failures of separate deletions are logged according to their
// reasons. If the flood control asks to wait, all the tasks of the chat are postponed: deleting a message again is
// harmless.
func (s *scheduler) deleteInChat(chatID int64, tasks []Task) {
	messageIDs := lo.Uniq(lo.FlatMap(tasks, func(task Task, _ int) []int { return task.messageIDs() }))
	if len(messageIDs) == 1 {
		s.deleteMessage(chatID, messageIDs[0], tasks...)
		return
	}
	for _, chunk := range lo.Chunk(messageIDs, maxMessagesPerRequest) {
		err := s.bot.Request(tgbotapi.NewDeleteMessages(chatID, chunk))
		if err == nil {
			countMessages(KindDelete, outcomeDone, len(chunk))
			continue
		}
		if s.retryLater(err, tasks...) {
			return
		}
		bulkFallbackCounter.Inc()
		log.Info("Couldn't delete the messages at once, deleting them one by one",
			logconst.FieldFunc, "selfdestruct.deleteInChat",
			logconst.FieldError, err)
		for _, messageID := range chunk {
			s.deleteMessage(chatID, messageID, Task{Kind: KindDelete, ChatID: chatID, MessageID: messageID})
		}
	}
}

// deleteMessage deletes one message; the tasks are postponed if the flood control asks to wait.
func (s *scheduler) deleteMessage(chatID int64, messageID int, tasks ...Task) {
	err := s.bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID))
	if err == nil {
		countMessages(KindDelete, outcomeDone, 1)
		return
	}
	if s.retryLater(err, tasks...) {
		return
	}
	level, outcome := log.LevelError, outcomeFailed
	if isExpectedDeletionFailure(err) {
		level, outcome = log.LevelDebug, outcomeRefused
	}
	countMessages(KindDelete, outcome, 1)
	log.Log(context.Background(), level, "Couldn't delete a message",
		logconst.FieldFunc, "selfdestruct.deleteMessage",
		logconst.FieldError, err)
}

// retryLater puts the tasks back into the queue if the flood control of Telegram has asked to wait.
func (s *scheduler) retryLater(err error, tasks ...Task) bool {
	var apiErr *tgbotapi.Error
	if !errors.As(err, &apiErr) || apiErr.RetryAfter <= 0 {
		return false
	}
	at := time.Now().Add(time.Duration(apiErr.RetryAfter) * time.Second)
	for _, task := range tasks {
		retriesCounter.WithLabelValues(string(task.Kind)).Inc()
		if err := s.queue.Add(s.ctx, task, at); err != nil {
			queueErrorsCounter.WithLabelValues(operationAdd).Inc()
			log.Error("Failed to postpone a task",
				logconst.FieldFunc, "selfdestruct.retryLater",
				logconst.FieldCalledMethod, "Add",
				logconst.FieldError, err)
		}
	}
	log.Warn("The tasks are postponed by the flood control of Telegram",
		logconst.FieldFunc, "selfdestruct.retryLater",
		"retry_after", apiErr.RetryAfter,
		"tasks", len(tasks))
	return true
}

// execute executes a task of any kind but a deletion, which is made by deleteInChat.
func (s *scheduler) execute(task Task) {
	switch task.Kind {
	case KindReplaceInline:
		edit := tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{InlineMessageID: task.InlineMessageID},
			Text:     task.Text,
		}
		err := s.bot.Request(edit)
		if err == nil {
			countMessages(task.Kind, outcomeDone, 1)
			return
		}
		if s.retryLater(err, task) {
			return
		}
		countMessages(task.Kind, outcomeFailed, 1)
		log.Error("Failed to replace an inline message with the placeholder",
			logconst.FieldFunc, "selfdestruct.execute",
			"inline_message_id", task.InlineMessageID,
			logconst.FieldError, err)
	default:
		countMessages(task.Kind, outcomeFailed, 1)
		log.Error("Unknown kind of a task",
			logconst.FieldFunc, "selfdestruct.execute",
			logconst.FieldValue, task.Kind)
	}
}

// Schedule puts the task into the queue to be executed after the delay. It replaces a task for the same message.
func Schedule(bot base.ExtendedBotAPI, task Task, after time.Duration) {
	s := getScheduler(bot)
	if err := s.queue.Add(s.ctx, task, time.Now().Add(after)); err != nil {
		queueErrorsCounter.WithLabelValues(operationAdd).Inc()
		log.Error("Failed to schedule a task",
			logconst.FieldFunc, "selfdestruct.Schedule",
			logconst.FieldCalledMethod, "Add",
			logconst.FieldError, err)
	}
}

// CancelDeletion removes the scheduled deletion of the message from the queue and returns it. ok is false if the
// message wasn't scheduled for deletion or its time has already come.
func CancelDeletion(bot base.ExtendedBotAPI, chatID int64, messageID int) (task Task, ok bool) {
	s := getScheduler(bot)
	task, ok, err := s.queue.Take(s.ctx, deletionTaskID(chatID, messageID))
	if err != nil {
		queueErrorsCounter.WithLabelValues(operationTake).Inc()
		log.Error("Failed to cancel a deletion",
			logconst.FieldFunc, "selfdestruct.CancelDeletion",
			logconst.FieldCalledMethod, "Take",
			logconst.FieldError, err)
	}
	return task, ok
}

// SendAndDelete sends the message and deletes it after the delay. It returns after sending.
func SendAndDelete(bot base.ExtendedBotAPI, msgToSend tgbotapi.MessageConfig, after time.Duration) {
	sentMsg, err := bot.Send(msgToSend)
	if err != nil {
		log.Error("Failed to call ExtendedBotAPI.Send",
			logconst.FieldFunc, "SendAndDelete",
			logconst.FieldCalledObject, "ExtendedBotAPI",
			logconst.FieldCalledMethod, "Send",
			logconst.FieldError, err)
		return
	}
	DeleteAfter(bot, msgToSend.ChatID, sentMsg.MessageID, after)
}

// DeleteAfter deletes a message after the delay. A message of another user can be deleted only if the bot has the right
// to delete messages in the group, so Telegram's refusal is expected and logged at the debug level only; other failures
// are errors.
func DeleteAfter(bot base.ExtendedBotAPI, chatID int64, messageID int, after time.Duration) {
	Schedule(bot, Task{Kind: KindDelete, ChatID: chatID, MessageID: messageID}, after)
}

// isExpectedDeletionFailure tells if Telegram has refused to delete a message for a reason the bot can't help:
// it has no right to delete messages of other users in the chat (or the message is older than 48 hours), or the
// message has already been deleted by its author or an admin.
func isExpectedDeletionFailure(err error) bool {
	var apiErr *tgbotapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusBadRequest {
		return false
	}
	return strings.Contains(apiErr.Message, "message can't be deleted") ||
		strings.Contains(apiErr.Message, "message to delete not found")
}

// ReplaceInline replaces the text of an inline message with the placeholder after the delay. It returns immediately.
// An inline message, including the answer to a guest query, can't be deleted by the bot: Telegram refuses it
// since the bot isn't a member of the chat, even if the chat and the message are decoded from the identifier.
func ReplaceInline(bot base.ExtendedBotAPI, inlineMessageID, placeholder string, after time.Duration) {
	Schedule(bot, Task{Kind: KindReplaceInline, InlineMessageID: inlineMessageID, Text: placeholder}, after)
}
