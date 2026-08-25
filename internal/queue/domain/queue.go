package domain

import (
	"context"
	"errors"
	"time"
)

var ErrClosed = errors.New("queue closed")

type Job struct {
	ExecutionID string    `json:"execution_id"`
	TenantID    string    `json:"tenant_id"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	Attempts    int       `json:"attempts"`
}
type Queue interface {
	Enqueue(context.Context, Job) error
	Dequeue(context.Context) (Job, error)
	Len() int
	Close() error
}
