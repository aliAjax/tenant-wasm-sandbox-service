package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	callback "github.com/acme/wasm-sandbox-executor/internal/callback/domain"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
)

type Webhook struct {
	client   *http.Client
	attempts int
	policy   callback.RetryPolicy
}

type StatusError struct{ StatusCode int }

func (e *StatusError) Error() string { return fmt.Sprintf("webhook status %d", e.StatusCode) }

func NewWebhook(timeout time.Duration, attempts int) *Webhook {
	if attempts < 1 {
		attempts = 1
	}
	return &Webhook{
		client:   &http.Client{Timeout: timeout},
		attempts: attempts,
		policy: callback.RetryPolicy{
			InitialDelay: 50 * time.Millisecond,
			MaximumDelay: 500 * time.Millisecond,
		},
	}
}

func retryableStatus(status int) bool { return status >= 500 && status <= 599 }

func releaseResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func (w *Webhook) Deliver(ctx context.Context, e execution.Execution) error {
	body, err := json.Marshal(struct {
		ID           string               `json:"id"`
		Status       execution.Status     `json:"status"`
		OutputDigest string               `json:"output_digest"`
		ErrorClass   execution.ErrorClass `json:"error_class"`
	}{e.ID, e.Status, e.OutputDigest, e.ErrorClass})
	if err != nil {
		return err
	}
	var last error
	for i := 0; i < w.attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.CallbackURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := w.client.Do(req)
		if err == nil {
			statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
			retryable := retryableStatus(resp.StatusCode)
			releaseResponse(resp)
			if statusOK {
				return nil
			}
			err = &StatusError{StatusCode: resp.StatusCode}
			if !retryable {
				return err
			}
		} else if resp != nil {
			releaseResponse(resp)
		}
		last = err
		if i+1 < w.attempts {
			if waitErr := w.policy.Wait(ctx, i+1); waitErr != nil {
				return fmt.Errorf("deliver webhook: %w", waitErr)
			}
		}
	}
	return fmt.Errorf("deliver webhook: %w", last)
}
