package application

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	queueapp "github.com/acme/wasm-sandbox-executor/internal/queue/application"
	queuedomain "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
)

type duplicateDeliveryQueue struct {
	mu   sync.Mutex
	jobs []queuedomain.Job
}

func (q *duplicateDeliveryQueue) Enqueue(context.Context, queuedomain.Job) error { return nil }
func (q *duplicateDeliveryQueue) Dequeue(context.Context) (queuedomain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return queuedomain.Job{}, queuedomain.ErrClosed
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, nil
}
func (q *duplicateDeliveryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}
func (q *duplicateDeliveryQueue) State() queuedomain.QueueState { return queuedomain.QueueOpen }
func (q *duplicateDeliveryQueue) Close() error                  { return nil }

type countingRunner struct {
	mu   sync.Mutex
	runs int
	ids  []string
}

func (r *countingRunner) Run(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs++
	r.ids = append(r.ids, id)
	return nil
}

func TestPoolDuplicateDeliveryRunsAtMostOnce(t *testing.T) {
	job := queuedomain.Job{ExecutionID: "exe-duplicate", TenantID: "tenant-1"}
	queue := &duplicateDeliveryQueue{jobs: []queuedomain.Job{job, job}}
	scheduler := queueapp.NewScheduler(queue)
	if err := scheduler.Submit(context.Background(), job.ExecutionID, job.TenantID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	runner := &countingRunner{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool := NewPool(queue, scheduler, runner, logger, 1)

	pool.Start(context.Background())
	pool.Wait()

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.runs != 1 {
		t.Fatalf("runner called %d times for duplicate delivery, want 1", runner.runs)
	}
}
