package selfdestruct

import (
	"context"
	log "log/slog"
	"runtime/debug"
	"time"

	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// panicKind is the kind of panics_total for the panics of the worker.
const panicKind = "self_destruction"

// The outcomes of the tasks in self_destruction_total.
const (
	outcomeDone    = "done"    // the message is deleted or replaced
	outcomeRefused = "refused" // Telegram refused to delete it for a reason the bot can't help
	outcomeFailed  = "failed"  // any other error: the task is lost
)

// The operations with the queue in self_destruction_queue_errors_total.
const (
	operationAdd     = "add"
	operationTake    = "take"
	operationTakeDue = "take_due"
)

var (
	messagesCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "self_destruction_total",
		Help: "Count of the messages the self-destruction worker has finished with, by kind (delete/replace_inline) and outcome: done, refused when Telegram refused to delete the message for a reason the bot can't help (no rights, already deleted, too old), failed for any other error. A message deleted by deleteMessages is done even if Telegram skipped it as missing. A postponed task isn't finished and is counted by self_destruction_retries_total",
	}, []string{"kind", "outcome"})
	retriesCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "self_destruction_retries_total",
		Help: "Count of the tasks postponed because the flood control of Telegram asked to wait (429 with retry_after), by kind. A task may be postponed many times and still end as done",
	}, []string{"kind"})
	warningsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "self_destruction_warnings_total",
		Help: "Count of the messages of the bot replaced with the warning before the deletion, by outcome: done, or failed if the message couldn't be edited (it's deleted anyway)",
	}, []string{"outcome"})
	bulkFallbackCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "self_destruction_bulk_fallback_total",
		Help: "Count of the deleteMessages requests Telegram refused, so that the messages of the chat were deleted one by one",
	})
	queueErrorsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "self_destruction_queue_errors_total",
		Help: "Count of the failed operations with the queue of the self-destruction (add/take/take_due). With a Redis queue, it means Redis is unavailable: the messages scheduled meanwhile stay in the chats",
	}, []string{"operation"})
	batchSizeHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "self_destruction_batch_size",
		Help:    "How many tasks one batch of the worker took. Batches that reach self_destruction_batch_limit mean the queue isn't drained by one batch per poll",
		Buckets: []float64{0, 1, 5, 10, 25, 50, 100},
	})
	batchDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "self_destruction_batch_duration_seconds",
		Help:    "How long the requests of one non-empty batch of the worker took. Long batches of a few tasks mean Telegram is slow",
		Buckets: prometheus.DefBuckets,
	})
	lastTickGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "self_destruction_last_tick_timestamp_seconds",
		Help: "When the worker last began a poll of the queue, as a Unix timestamp. Alert when time() minus this passes a few poll intervals: the worker is stuck or dead, while self_destruction_pending, read from the queue on scrape, may still look fine",
	})
	batchLimitGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "self_destruction_batch_limit",
		Help: "The maximum size of a batch of the worker, so that a graph can tell a full batch from a small one",
	})
	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "self_destruction_pending",
		Help: "Number of tasks waiting in the queue of the self-destruction, read from the queue on scrape. A number that only grows means the worker isn't draining the queue",
	}, pendingTasks)
)

func init() {
	for _, kind := range []TaskKind{KindDelete, KindReplaceInline} {
		for _, outcome := range []string{outcomeDone, outcomeRefused, outcomeFailed} {
			messagesCounter.WithLabelValues(string(kind), outcome)
		}
		retriesCounter.WithLabelValues(string(kind))
	}
	for _, outcome := range []string{outcomeDone, outcomeFailed} {
		warningsCounter.WithLabelValues(outcome)
	}
	for _, operation := range []string{operationAdd, operationTake, operationTakeDue} {
		queueErrorsCounter.WithLabelValues(operation)
	}
	metrics.InitPanicsCounters(panicKind)
	batchLimitGauge.Set(tasksPerPoll)
}

func countMessages(kind TaskKind, outcome string, n int) {
	messagesCounter.WithLabelValues(string(kind), outcome).Add(float64(n))
}

// pendingTasks returns the length of the queue of the global scheduler, or 0 if there is none yet.
func pendingTasks() float64 {
	globalSchedulerMu.Lock()
	s := globalScheduler
	globalSchedulerMu.Unlock()
	if s == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()
	n, err := s.queue.Len(ctx)
	if err != nil {
		log.Error("Failed to get the length of the queue",
			logconst.FieldFunc, "selfdestruct.pendingTasks",
			logconst.FieldCalledMethod, "Len",
			logconst.FieldError, err)
	}
	return float64(n)
}

// recoverPanic counts and logs a panic of the worker instead of crashing the application. Call it deferred.
func recoverPanic(funcName string) {
	if r := recover(); r != nil {
		metrics.IncPanicsCounter(panicKind)
		log.Error("Recovered from a panic of the self-destruction worker",
			logconst.FieldFunc, funcName,
			"panic", r,
			"stack", string(debug.Stack()))
	}
}
