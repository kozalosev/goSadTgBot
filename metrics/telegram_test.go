package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

type fakeHTTPClient struct {
	resp *http.Response
	err  error
}

func (c fakeHTTPClient) Do(*http.Request) (*http.Response, error) {
	return c.resp, c.err
}

func newTelegramRequest(path string) *http.Request {
	return &http.Request{URL: &url.URL{Scheme: "https", Host: "api.telegram.org", Path: path}}
}

func TestTelegramMethod(t *testing.T) {
	assert.Equal(t, "sendMessage", telegramMethod(newTelegramRequest("/bot123:secret/sendMessage")))
	assert.Equal(t, "getUpdates", telegramMethod(newTelegramRequest("/custom/prefix/bot123:secret/getUpdates")))
	assert.Equal(t, telegramOtherMethod, telegramMethod(newTelegramRequest("/file/bot123:secret/photos/file_1.jpg")))
	assert.Equal(t, telegramOtherMethod, telegramMethod(&http.Request{}))
}

func TestClassifyTelegramError(t *testing.T) {
	status := func(code int) *http.Response {
		return &http.Response{StatusCode: code}
	}
	assert.Empty(t, classifyTelegramError(status(http.StatusOK), nil))
	assert.Equal(t, telegramErrorAPI, classifyTelegramError(status(http.StatusBadRequest), nil))
	assert.Equal(t, telegramErrorRateLimited, classifyTelegramError(status(http.StatusTooManyRequests), nil))
	assert.Equal(t, telegramErrorServer, classifyTelegramError(status(http.StatusBadGateway), nil))
	assert.Equal(t, telegramErrorCanceled, classifyTelegramError(nil, context.Canceled))
	assert.Equal(t, telegramErrorTimeout, classifyTelegramError(nil, &url.Error{Op: "Post", Err: context.DeadlineExceeded}))
	assert.Equal(t, telegramErrorConnect, classifyTelegramError(nil, &net.OpError{Op: "dial", Err: errors.New("refused")}))
	assert.Equal(t, telegramErrorNetwork, classifyTelegramError(nil, errors.New("EOF")))
}

func TestInstrumentedTelegramClient(t *testing.T) {
	errorsBefore := testutil.ToFloat64(telegramRequestErrors.WithLabelValues("sendSticker", telegramErrorRateLimited))

	ok := NewTelegramHTTPClient(fakeHTTPClient{resp: &http.Response{StatusCode: http.StatusOK}})
	_, err := ok.Do(newTelegramRequest("/bot123:secret/sendSticker"))
	assert.NoError(t, err)

	limited := NewTelegramHTTPClient(fakeHTTPClient{resp: &http.Response{StatusCode: http.StatusTooManyRequests}})
	_, err = limited.Do(newTelegramRequest("/bot123:secret/sendSticker"))
	assert.NoError(t, err)

	assert.Equal(t, errorsBefore+1, testutil.ToFloat64(telegramRequestErrors.WithLabelValues("sendSticker", telegramErrorRateLimited)))
	// both outcomes are observed: one series for "ok" and one for "error"
	assert.Equal(t, 2, testutil.CollectAndCount(telegramRequestDuration, "telegram_request_duration_seconds"))
}
