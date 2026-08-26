package domain

import (
	"context"
	"errors"
	"time"
)

var ErrClosed = errors.New("queue closed")

type QueueState string

const (
	QueueOpen     QueueState = "open"
	QueueDraining QueueState = "draining"
	QueueClosed   QueueState = "closed"
)

func (s QueueState) BeginClose(_ int) QueueState {
	if s == QueueOpen {
		return QueueClosed
	}
	return s
}

func (s QueueState) AfterDequeue(_ int) QueueState { return s }

func (s QueueState) IsTerminal() bool { return s != QueueOpen }

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
	State() QueueState
	Close() error
}
