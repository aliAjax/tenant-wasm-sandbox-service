package infrastructure

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
)

func callbackExecution(url string) execution.Execution {
	return execution.Execution{ID: "exe-1", CallbackURL: url, Status: execution.StatusSucceeded}
}

func TestWebhookDoesNotRetryPermanentStatus(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "invalid callback", http.StatusBadRequest)
	}))
	defer server.Close()

	err := NewWebhook(time.Second, 3).Deliver(context.Background(), callbackExecution(server.URL))
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected permanent status error, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("permanent response was retried %d times", got)
	}
}

type trackedBody struct {
	reader  *strings.Reader
	closed  bool
	drained bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if errors.Is(err, io.EOF) {
		b.drained = true
	}
	return n, err
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type sequenceTransport struct {
	bodies []*trackedBody
	calls  int
	err    error
}

func (t *sequenceTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if t.err != nil {
		return nil, t.err
	}
	status := http.StatusServiceUnavailable
	if t.calls > 0 {
		status = http.StatusNoContent
	}
	body := &trackedBody{reader: strings.NewReader("retry response")}
	t.bodies = append(t.bodies, body)
	t.calls++
	return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}, nil
}

func TestWebhookDrainsAndClosesResponseBeforeRetry(t *testing.T) {
	transport := &sequenceTransport{}
	webhook := NewWebhook(time.Second, 2)
	webhook.client.Transport = transport
	webhook.policy.InitialDelay = time.Millisecond
	if err := webhook.Deliver(context.Background(), callbackExecution("http://callback.invalid")); err != nil {
		t.Fatal(err)
	}
	if len(transport.bodies) != 2 || !transport.bodies[0].drained || !transport.bodies[0].closed {
		t.Fatalf("first response not released: %#v", transport.bodies)
	}
}

func TestWebhookCancellationInterruptsBackoff(t *testing.T) {
	webhook := NewWebhook(time.Second, 3)
	webhook.client.Transport = &sequenceTransport{}
	webhook.policy.InitialDelay = 2 * time.Second
	webhook.policy.MaximumDelay = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	started := time.Now()
	err := webhook.Deliver(ctx, callbackExecution("http://callback.invalid"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("cancellation waited for retry timer: %s", elapsed)
	}
}

func TestWebhookPreservesTransportErrorChain(t *testing.T) {
	sentinel := errors.New("dial refused")
	webhook := NewWebhook(time.Second, 1)
	webhook.client.Transport = &sequenceTransport{err: sentinel}
	err := webhook.Deliver(context.Background(), callbackExecution("http://callback.invalid"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("transport cause was lost: %v", err)
	}
}
