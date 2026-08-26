package application

import (
	"context"
	queue "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
	"sync"
	"time"
)

type Scheduler struct {
	queue    queue.Queue
	mu       sync.Mutex
	canceled map[string]bool
}

func NewScheduler(q queue.Queue) *Scheduler { return &Scheduler{queue: q, canceled: map[string]bool{}} }
func (s *Scheduler) Submit(ctx context.Context, id, tenant string) error {
	return s.queue.Enqueue(ctx, queue.Job{ExecutionID: id, TenantID: tenant, EnqueuedAt: time.Now().UTC()})
}
func (s *Scheduler) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled[id] = true
	return true
}
func (s *Scheduler) Canceled(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.canceled[id] {
		delete(s.canceled, id)
		return true
	}
	return false
}
