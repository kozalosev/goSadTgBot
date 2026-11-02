// Package metrics provides metrics for Prometheus.
package metrics

import (
	log "log/slog"
	"net/http"
	"reflect"

	"github.com/IBM/pgxpoolprometheus"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ChosenInlineResultCounter is a counter which is registered automatically for ChosenInlineResult.
// https://core.telegram.org/bots/inline#collecting-feedback
const ChosenInlineResultCounter = "inline_result_was_chosen"

type handlerName string

var handlerCounters = make(map[handlerName]prometheus.Counter)

// unknownLabel is used for the label values which are missing in the update.
const unknownLabel = "unknown"

var (
	updateKindCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "update_kind_total",
		Help: "Count of all updates received from Telegram, by update kind. The breakdown shows how much of the traffic no handler acts on",
	}, []string{"kind"})
	usedLanguageCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "used_language_total",
		Help: "Count of updates by the language_code of the sender's Telegram client, an anonymous proxy for the languages of the audience",
	}, []string{"language"})
	panicsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "panics_total",
		Help: "Count of panics recovered while handling updates, by update kind. Any nonzero rate is a bug",
	}, []string{"kind"})
	wizardCompletedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wizard_completed_total",
		Help: "Count of wizard forms which have got all their fields and invoked the action, by wizard. Read it against used_message_handler_* to see how many forms were abandoned",
	}, []string{"wizard"})
)

// The Init* functions below create the series for the known label values with zeros. A series of a vector is
// created lazily by its first increment otherwise, so the dashboards show "No data" instead of 0 and increase()
// doesn't see the first event, which matters for the alerts on rare events like panics.

// InitUpdateKindCounters creates the series of update_kind_total for the kinds.
func InitUpdateKindCounters(kinds ...string) {
	for _, kind := range kinds {
		updateKindCounter.WithLabelValues(kind)
	}
}

// InitPanicsCounters creates the series of panics_total for the kinds.
func InitPanicsCounters(kinds ...string) {
	for _, kind := range kinds {
		panicsCounter.WithLabelValues(kind)
	}
}

// InitWizardCompletedCounters creates the series of wizard_completed_total for the wizards.
func InitWizardCompletedCounters(wizards ...string) {
	for _, wizard := range wizards {
		wizardCompletedCounter.WithLabelValues(wizard)
	}
}

// IncUpdateCounters counts the update by its kind and the language of its sender.
func IncUpdateCounters(kind string, sender *tgbotapi.User) {
	updateKindCounter.WithLabelValues(kind).Inc()
	language := unknownLabel
	if sender != nil && len(sender.LanguageCode) > 0 {
		language = sender.LanguageCode
	}
	usedLanguageCounter.WithLabelValues(language).Inc()
}

// IncPanicsCounter counts a panic recovered while handling an update of the kind.
func IncPanicsCounter(kind string) {
	panicsCounter.WithLabelValues(kind).Inc()
}

// IncWizardCompletedCounter counts a wizard form which has invoked its action.
func IncWizardCompletedCounter(wizard string) {
	wizardCompletedCounter.WithLabelValues(wizard).Inc()
}

func init() {
	registerCounter(ChosenInlineResultCounter, "")
}

// RegisterMessageHandlerCounters following the scheme: "used_message_handler_%s".
func RegisterMessageHandlerCounters(handlers ...base.MessageHandler) {
	for _, h := range handlers {
		registerCounter(resolveHandlerName(h), "used_message_handler_")
	}
}

// IncMessageHandlerCounter increments the counter registered for a message handler.
func IncMessageHandlerCounter(handler base.MessageHandler) {
	Inc(resolveHandlerName(handler))
}

// RegisterInlineHandlerCounters following the scheme: "used_inline_handler_%s".
func RegisterInlineHandlerCounters(handlers ...base.InlineHandler) {
	for _, h := range handlers {
		registerCounter(resolveHandlerName(h), "used_inline_handler_")
	}
}

// IncInlineHandlerCounter increments the counter registered for an inline query handler.
func IncInlineHandlerCounter(handler base.InlineHandler) {
	Inc(resolveHandlerName(handler))
}

// RegisterGuestHandlerCounters following the scheme: "used_guest_handler_%s".
func RegisterGuestHandlerCounters(handlers ...base.GuestHandler) {
	for _, h := range handlers {
		registerCounter(resolveHandlerName(h), "used_guest_handler_")
	}
}

// IncGuestHandlerCounter increments the counter registered for a guest message handler.
func IncGuestHandlerCounter(handler base.GuestHandler) {
	Inc(resolveHandlerName(handler))
}

// RegisterCallbackHandlerCounters following the scheme: "used_callback_handler_%s".
func RegisterCallbackHandlerCounters(handlers ...base.CallbackHandler) {
	for _, h := range handlers {
		registerCounter(resolveHandlerName(h), "used_callback_handler_")
	}
}

// IncCallbackHandlerCounter increments the counter registered for a callback query handler.
func IncCallbackHandlerCounter(handler base.CallbackHandler) {
	Inc(resolveHandlerName(handler))
}

// Inc increments the counter registered as 'name'.
func Inc(name string) {
	counter, ok := handlerCounters[handlerName(name)]
	if ok {
		counter.Inc()
	} else {
		log.Warn("Counter is missing",
			logconst.FieldFunc, "Inc",
			logconst.FieldName, name)
	}
}

// RegisterMetricsForPgxPoolStat registers metrics for [pgxpool.Stat] with prefix "pgxpool_*".
func RegisterMetricsForPgxPoolStat(pool *pgxpool.Pool, dbName string) {
	collector := pgxpoolprometheus.NewCollector(pool, map[string]string{"db_name": dbName})
	prometheus.MustRegister(collector)
}

// AddHttpHandlerForMetrics adds a global route /metrics to the server by using [http.Handle].
func AddHttpHandlerForMetrics() {
	http.Handle("/metrics", promhttp.Handler())
}

func resolveHandlerName(handler any) string {
	t := reflect.TypeOf(handler)
	if t.Kind() == reflect.Pointer {
		return reflect.Indirect(reflect.ValueOf(handler)).Type().Name()
	}
	return t.Name()
}

func registerCounter(name, metricPrefix string) {
	handlerCounters[handlerName(name)] = promauto.NewCounter(prometheus.CounterOpts{
		Name: metricPrefix + name,
		Help: "Usage counter",
	})
}
