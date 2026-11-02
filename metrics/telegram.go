package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"regexp"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// kinds of failed requests to Telegram Bot API
const (
	telegramErrorTimeout     = "timeout"
	telegramErrorConnect     = "connect"
	telegramErrorCanceled    = "canceled"
	telegramErrorNetwork     = "network"
	telegramErrorRateLimited = "rate_limited"
	telegramErrorServer      = "server"
	telegramErrorAPI         = "api"
)

const telegramOtherMethod = "other"

// telegramMethodRegexp extracts the method from the path of a request: "/bot<token>/sendMessage".
// The token must never get into a label, and file downloads mustn't produce a label per file.
var telegramMethodRegexp = regexp.MustCompile(`/bot[^/]+/(\w+)$`)

var (
	telegramRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "telegram_request_duration_seconds",
		Help: "Duration of requests to Telegram Bot API, by method and outcome (ok/error). The failed ones are measured too, so timeouts are not missing from the histogram",
	}, []string{"method", "outcome"})
	telegramRequestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_request_errors_total",
		Help: "Count of failed requests to Telegram Bot API, by method and kind (timeout/connect/canceled/network/rate_limited/server/api). rate_limited means Telegram asked the bot to slow down",
	}, []string{"method", "kind"})
)

// NewTelegramHTTPClient wraps the client to measure the requests to Telegram Bot API. Pass the result to
// [tgbotapi.WithHTTPClient] when you create the bot:
//
//	tgbotapi.NewBotAPIWithOptions(token, tgbotapi.WithHTTPClient(metrics.NewTelegramHTTPClient(&http.Client{})))
func NewTelegramHTTPClient(client tgbotapi.HTTPClient) tgbotapi.HTTPClient {
	return instrumentedTelegramClient{client: client}
}

type instrumentedTelegramClient struct {
	client tgbotapi.HTTPClient
}

func (c instrumentedTelegramClient) Do(req *http.Request) (*http.Response, error) {
	method := telegramMethod(req)
	start := time.Now()
	resp, err := c.client.Do(req)
	elapsed := time.Since(start).Seconds()

	if kind := classifyTelegramError(resp, err); kind != "" {
		telegramRequestErrors.WithLabelValues(method, kind).Inc()
		telegramRequestDuration.WithLabelValues(method, "error").Observe(elapsed)
	} else {
		telegramRequestDuration.WithLabelValues(method, "ok").Observe(elapsed)
	}
	return resp, err
}

func telegramMethod(req *http.Request) string {
	if req.URL == nil {
		return telegramOtherMethod
	}
	if m := telegramMethodRegexp.FindStringSubmatch(req.URL.Path); m != nil {
		return m[1]
	}
	return telegramOtherMethod
}

// classifyTelegramError returns the kind of the error, or an empty string if the request has succeeded.
// Telegram responds to the requests it refused with an HTTP status code of the error.
func classifyTelegramError(resp *http.Response, err error) string {
	if err != nil {
		var netErr net.Error
		var opErr *net.OpError
		switch {
		case errors.Is(err, context.Canceled):
			return telegramErrorCanceled
		case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
			return telegramErrorTimeout
		case errors.As(err, &opErr) && opErr.Op == "dial":
			return telegramErrorConnect
		default:
			return telegramErrorNetwork
		}
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return telegramErrorRateLimited
	case resp.StatusCode >= http.StatusInternalServerError:
		return telegramErrorServer
	case resp.StatusCode >= http.StatusBadRequest:
		return telegramErrorAPI
	default:
		return ""
	}
}
