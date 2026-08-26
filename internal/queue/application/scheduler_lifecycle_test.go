package application

import (
	"context"
	"errors"
	"testing"

	queuedomain "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
)

type schedulerQueue struct{ enqueueErr error }

func (q *schedulerQueue) Enqueue(context.Context, queuedomain.Job) error { return q.enqueueErr }
func (q *schedulerQueue) Dequeue(context.Context) (queuedomain.Job, error) {
	return queuedomain.Job{}, queuedomain.ErrClosed
}
func (q *schedulerQueue) Len() int                      { return 0 }
func (q *schedulerQueue) State() queuedomain.QueueState { return queuedomain.QueueOpen }
func (q *schedulerQueue) Close() error                  { return nil }

func TestSchedulerFailedSubmitCannotBeCanceled(t *testing.T) {
	scheduler := NewScheduler(&schedulerQueue{enqueueErr: errors.New("queue unavailable")})
	if err := scheduler.Submit(context.Background(), "exe-failed", "tenant-1"); err == nil {
		t.Fatal("submission unexpectedly succeeded")
	}
	if scheduler.Cancel("exe-failed") {
		t.Fatal("failed submission remained cancelable")
	}
}

func TestSchedulerUnknownCancelReturnsFalse(t *testing.T) {
	scheduler := NewScheduler(&schedulerQueue{})
	if scheduler.Cancel("missing-execution") {
		t.Fatal("unknown execution was reported as canceled")
	}
}

func TestSchedulerCanceledMarkerSurvivesDuplicateCheck(t *testing.T) {
	scheduler := NewScheduler(&schedulerQueue{})
	if err := scheduler.Submit(context.Background(), "exe-canceled", "tenant-1"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !scheduler.Cancel("exe-canceled") {
		t.Fatal("known execution was not canceled")
	}
	if scheduler.Claim("exe-canceled") {
		t.Fatal("canceled delivery was claimed")
	}
	if scheduler.Claim("exe-canceled") {
		t.Fatal("duplicate canceled delivery was claimed")
	}
}
