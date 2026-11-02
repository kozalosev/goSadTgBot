package selfdestruct

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kozalosev/goSadTgBot/base"
)

// TaskKind is what to do with a message when its time has come.
type TaskKind string

const (
	// KindDelete deletes the message ChatID/MessageID.
	KindDelete TaskKind = "delete"
	// KindReplaceInline replaces the text of the inline message InlineMessageID with Text.
	KindReplaceInline TaskKind = "replace_inline"
)

// Task is a postponed action with a message, stored in a [Queue].
type Task struct {
	Kind            TaskKind       `json:"kind"`
	ChatID          base.ChatID    `json:"chatID,omitempty"`
	MessageID       base.MessageID `json:"messageID,omitempty"`
	InlineMessageID string         `json:"inlineMessageID,omitempty"`
	Text            string         `json:"text,omitempty"`
	// ReplyToMessageID is the message the deleted message replies to, returned by [CancelDeletion]: Telegram doesn't
	// send the replies of the replies, so whoever cancels the deletion of a reply may need it back.
	ReplyToMessageID base.MessageID `json:"replyToMessageID,omitempty"`
	// WithMessageIDs are the other messages of the chat deleted together with MessageID, by the same request.
	WithMessageIDs []base.MessageID `json:"withMessageIDs,omitempty"`
	// Warning, if set, is shown in place of the messages of the bot before the deletion.
	Warning *Warning `json:"warning,omitempty"`
}

// Warning tells the members of the chat that the bot is about to delete the messages: when the time of a deletion
// comes, the text of one message of the bot is replaced with Text, and all the messages of the task are deleted
// the grace period later. Telegram edits one message per request, so a single message is edited, e.g. the last one
// the users see.
type Warning struct {
	Text      string `json:"text"`
	ParseMode string `json:"parseMode,omitempty"`
	// MessageID is the message of the bot to edit, among the deleted ones.
	MessageID base.MessageID `json:"messageID"`
	Grace     time.Duration  `json:"grace"`
}

// ID identifies the task in the queue: there is only one task for a message, and it can be found by the message.
func (task Task) ID() string {
	if task.Kind == KindReplaceInline {
		return "inline:" + task.InlineMessageID
	}
	return deletionTaskID(task.ChatID, task.MessageID)
}

// messageIDs returns all the messages a deletion task deletes.
func (task Task) messageIDs() []base.MessageID {
	return append([]base.MessageID{task.MessageID}, task.WithMessageIDs...)
}

func deletionTaskID(chatID base.ChatID, messageID base.MessageID) string {
	return fmt.Sprintf("delete:%d:%d", chatID, messageID)
}

// Queue stores the tasks until their time comes. [NewMemoryQueue] keeps them in memory, so they are lost on restart;
// [ConnectToRedis] keeps them in Redis.
type Queue interface {
	// Add schedules the task. It replaces a task with the same [Task.ID].
	Add(ctx context.Context, task Task, at time.Time) error
	// Take removes the task with the identifier from the queue and returns it. ok is false if there is no such task.
	Take(ctx context.Context, id string) (task Task, ok bool, err error)
	// TakeDue removes up to limit tasks whose time has come by now and returns them, the earliest first.
	TakeDue(ctx context.Context, now time.Time, limit int) ([]Task, error)
	// Len returns the number of tasks in the queue, due or not.
	Len(ctx context.Context) (int, error)
}

// MemoryQueue is a [Queue] in memory: a min-heap by time, so the due tasks are taken from its top without looking
// through the others, and an index by the identifiers to find a task in the heap.
type MemoryQueue struct {
	mu       sync.Mutex
	schedule schedule
	byID     map[string]*scheduledTask
}

type scheduledTask struct {
	task  Task
	at    time.Time
	index int // the position in the heap, maintained by schedule
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{byID: make(map[string]*scheduledTask)}
}

func (q *MemoryQueue) Add(_ context.Context, task Task, at time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if scheduled, ok := q.byID[task.ID()]; ok {
		scheduled.task, scheduled.at = task, at
		heap.Fix(&q.schedule, scheduled.index)
		return nil
	}
	scheduled := &scheduledTask{task: task, at: at}
	heap.Push(&q.schedule, scheduled)
	q.byID[task.ID()] = scheduled
	return nil
}

func (q *MemoryQueue) Take(_ context.Context, id string) (Task, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	scheduled, ok := q.byID[id]
	if !ok {
		return Task{}, false, nil
	}
	heap.Remove(&q.schedule, scheduled.index)
	delete(q.byID, id)
	return scheduled.task, true, nil
}

func (q *MemoryQueue) TakeDue(_ context.Context, now time.Time, limit int) ([]Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var tasks []Task
	for len(tasks) < limit && len(q.schedule) > 0 && !q.schedule[0].at.After(now) {
		scheduled := heap.Pop(&q.schedule).(*scheduledTask)
		delete(q.byID, scheduled.task.ID())
		tasks = append(tasks, scheduled.task)
	}
	return tasks, nil
}

func (q *MemoryQueue) Len(context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.schedule), nil
}

// schedule implements [heap.Interface]: the earliest task is at the top.
type schedule []*scheduledTask

func (s schedule) Len() int           { return len(s) }
func (s schedule) Less(i, j int) bool { return s[i].at.Before(s[j].at) }

func (s schedule) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
	s[i].index, s[j].index = i, j
}

func (s *schedule) Push(x any) {
	scheduled := x.(*scheduledTask)
	scheduled.index = len(*s)
	*s = append(*s, scheduled)
}

func (s *schedule) Pop() any {
	old := *s
	last := old[len(old)-1]
	old[len(old)-1] = nil
	*s = old[:len(old)-1]
	return last
}
