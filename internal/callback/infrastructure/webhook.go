package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	"net/http"
	"time"
)

type Webhook struct {
	client   *http.Client
	attempts int
}

func NewWebhook(timeout time.Duration, attempts int) *Webhook {
	if attempts < 1 {
		attempts = 1
	}
	return &Webhook{client: &http.Client{Timeout: timeout}, attempts: attempts}
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
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			err = fmt.Errorf("webhook status %d", resp.StatusCode)
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * 50 * time.Millisecond):
		}
	}
	return fmt.Errorf("deliver webhook: %w", last)
}
