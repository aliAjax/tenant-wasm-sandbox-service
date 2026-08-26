package application

import (
	"context"
	"errors"
	queueapp "github.com/acme/wasm-sandbox-executor/internal/queue/application"
	queuedomain "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
	"log/slog"
	"sync"
)

type Runner interface {
	Run(context.Context, string) error
}
type Pool struct {
	queue     queuedomain.Queue
	scheduler *queueapp.Scheduler
	runner    Runner
	logger    *slog.Logger
	workers   int
	wg        sync.WaitGroup
}

func NewPool(q queuedomain.Queue, s *queueapp.Scheduler, r Runner, logger *slog.Logger, workers int) *Pool {
	if workers < 1 {
		workers = 1
	}
	return &Pool{queue: q, scheduler: s, runner: r, logger: logger, workers: workers}
}
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.loop(ctx, i)
	}
}
func (p *Pool) loop(ctx context.Context, index int) {
	defer p.wg.Done()
	for {
		job, err := p.queue.Dequeue(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, queuedomain.ErrClosed) {
				p.logger.Error("dequeue failed", "worker", index, "error", err)
			}
			return
		}
		if p.scheduler.Canceled(job.ExecutionID) {
			continue
		}
		if err = p.runner.Run(ctx, job.ExecutionID); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			p.logger.Warn("execution finished with error", "execution_id", job.ExecutionID, "error", err)
		}
	}
}
func (p *Pool) Wait() { p.wg.Wait() }
