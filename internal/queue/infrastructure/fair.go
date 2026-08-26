package infrastructure

import (
	"context"
	"sync"

	queue "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
)

type FairQueue struct {
	mu       sync.Mutex
	notify   chan struct{}
	state    queue.QueueState
	byTenant map[string][]queue.Job
	tenants  []string
	cursor   int
}

func NewFairQueue() *FairQueue {
	return &FairQueue{notify: make(chan struct{}, 1), state: queue.QueueOpen, byTenant: map[string][]queue.Job{}}
}

func (q *FairQueue) Enqueue(ctx context.Context, job queue.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state != queue.QueueOpen {
		return queue.ErrClosed
	}
	if len(q.byTenant[job.TenantID]) == 0 {
		q.tenants = append(q.tenants, job.TenantID)
	}
	q.byTenant[job.TenantID] = append(q.byTenant[job.TenantID], job)
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return nil
}

func (q *FairQueue) Dequeue(ctx context.Context) (queue.Job, error) {
	for {
		q.mu.Lock()
		if len(q.tenants) > 0 {
			if q.cursor >= len(q.tenants) {
				q.cursor = 0
			}
			tenantID := q.tenants[q.cursor]
			jobs := q.byTenant[tenantID]
			job := jobs[0]
			jobs = jobs[1:]
			if len(jobs) == 0 {
				delete(q.byTenant, tenantID)
				q.tenants = append(q.tenants[:q.cursor], q.tenants[q.cursor+1:]...)
			} else {
				q.byTenant[tenantID] = jobs
				q.cursor = (q.cursor + 1) % len(q.tenants)
			}
			q.state = q.state.AfterDequeue(len(q.tenants))
			q.mu.Unlock()
			return job, nil
		}
		if q.state.IsTerminal() {
			q.mu.Unlock()
			return queue.Job{}, queue.ErrClosed
		}
		q.mu.Unlock()
		<-q.notify
	}
}

func (q *FairQueue) State() queue.QueueState {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.state
}

func (q *FairQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	count := 0
	for _, jobs := range q.byTenant {
		count += len(jobs)
	}
	return count
}

func (q *FairQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state == queue.QueueOpen {
		q.state = q.state.BeginClose(len(q.tenants))
		select {
		case q.notify <- struct{}{}:
		default:
		}
	}
	return nil
}
