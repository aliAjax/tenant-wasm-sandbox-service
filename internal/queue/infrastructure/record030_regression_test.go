package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	queue "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
)

func TestFairQueueEnqueueCanceledWhileWaitingForLock(t *testing.T) {
	q := NewFairQueue()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan error, 1)

	q.mu.Lock()
	go func() {
		close(started)
		result <- q.Enqueue(ctx, queue.Job{ExecutionID: "late", TenantID: "tenant-a"})
	}()
	<-started
	cancel()
	q.mu.Unlock()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("enqueue error = %v, want context canceled", err)
	}
	if got := q.Len(); got != 0 {
		t.Fatalf("queue length = %d, canceled enqueue added a job", got)
	}
}

func TestFairQueueDequeueCanceledWhileWaitingForLock(t *testing.T) {
	q := NewFairQueue()
	if err := q.Enqueue(context.Background(), queue.Job{ExecutionID: "keep", TenantID: "tenant-a"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	type dequeueResult struct {
		job queue.Job
		err error
	}
	result := make(chan dequeueResult, 1)

	q.mu.Lock()
	go func() {
		close(started)
		job, err := q.Dequeue(ctx)
		result <- dequeueResult{job: job, err: err}
	}()
	<-started
	cancel()
	q.mu.Unlock()

	got := <-result
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("dequeue returned job %q and error %v, want context canceled", got.job.ExecutionID, got.err)
	}
	if got := q.Len(); got != 1 {
		t.Fatalf("queue length = %d, canceled dequeue consumed the job", got)
	}
}

func TestFairQueueDequeueCancellationWhileIdle(t *testing.T) {
	q := NewFairQueue()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := q.Dequeue(ctx)
		result <- err
	}()

	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dequeue error = %v, want context canceled", err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = q.Close()
		<-result
		t.Fatal("idle dequeue did not return after context cancellation")
	}
}

func TestFairQueueCloseEntersDrainingWithPendingJobs(t *testing.T) {
	q := NewFairQueue()
	if err := q.Enqueue(context.Background(), queue.Job{ExecutionID: "pending", TenantID: "tenant-a"}); err != nil {
		t.Fatal(err)
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if got := q.State(); got != queue.QueueDraining {
		t.Fatalf("state after close with pending job = %q, want %q", got, queue.QueueDraining)
	}
	job, err := q.Dequeue(context.Background())
	if err != nil || job.ExecutionID != "pending" {
		t.Fatalf("drain returned job %q and error %v", job.ExecutionID, err)
	}
	if got := q.State(); got != queue.QueueClosed {
		t.Fatalf("state after drain = %q, want %q", got, queue.QueueClosed)
	}
}

func TestFairQueueCloseWakesAllWaiters(t *testing.T) {
	q := NewFairQueue()
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		select {
		case _, open := <-q.notify:
			if open {
				t.Fatalf("notification receive %d observed an open channel; close must broadcast", i+1)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("notification receive %d blocked; not all waiters were released", i+1)
		}
	}
}
