package domain

import (
	"context"
	"time"

	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
)

type DeliveryStatus string

const (
	DeliveryPending DeliveryStatus = "pending"
	DeliverySent    DeliveryStatus = "sent"
	DeliveryFailed  DeliveryStatus = "failed"
)

type Delivery struct {
	ExecutionID   string         `json:"execution_id"`
	URL           string         `json:"url"`
	Status        DeliveryStatus `json:"status"`
	Attempts      int            `json:"attempts"`
	LastError     string         `json:"last_error,omitempty"`
	NextAttemptAt time.Time      `json:"next_attempt_at,omitempty"`
}

type Sender interface {
	Deliver(context.Context, execution.Execution) error
}

type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaximumDelay time.Duration
}

func (p RetryPolicy) Delay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := p.InitialDelay
	for current := 1; current < attempt; current++ {
		delay *= 2
		if delay >= p.MaximumDelay {
			return p.MaximumDelay
		}
	}
	return delay
}
