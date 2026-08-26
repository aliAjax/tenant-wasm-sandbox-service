package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAcquireRechecksContextAfterLockWait(t *testing.T) {
	m := NewManager(2, time.Minute, 10)
	m.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := m.Acquire(ctx, "tenant", "lease", 1); result <- err }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	m.mu.Unlock()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation after lock wait, got %v", err)
	}
	if m.Active() != 0 {
		t.Fatal("canceled acquire created a lease")
	}
}

func TestAcquireRejectsDuplicateLeaseID(t *testing.T) {
	m := NewManager(3, time.Minute, 10)
	if _, err := m.Acquire(context.Background(), "a", "same", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Acquire(context.Background(), "b", "same", 1); !errors.Is(err, ErrLeaseExists) {
		t.Fatalf("expected duplicate lease error, got %v", err)
	}
	if q := m.Get("b"); q.Active != 0 {
		t.Fatalf("duplicate changed tenant active count: %d", q.Active)
	}
}

func TestReleaseIgnoresNegativeCPU(t *testing.T) {
	m := NewManager(1, time.Minute, 10)
	if _, err := m.Acquire(context.Background(), "tenant", "lease", 1); err != nil {
		t.Fatal(err)
	}
	m.Release("lease", -time.Second)
	if q := m.Get("tenant"); q.CPUUsed != 0 {
		t.Fatalf("negative CPU polluted quota: %s", q.CPUUsed)
	}
}

func TestQuotaWindowRecoversFromClockRollback(t *testing.T) {
	m := NewManager(1, time.Second, 10)
	future := time.Now().UTC().Add(time.Hour)
	m.now = func() time.Time { return future }
	if _, err := m.Acquire(context.Background(), "tenant", "one", 1); err != nil {
		t.Fatal(err)
	}
	m.Release("one", time.Second)
	m.now = func() time.Time { return future.Add(-2 * time.Hour) }
	if _, err := m.Acquire(context.Background(), "tenant", "two", 1); err != nil {
		t.Fatalf("clock rollback left quota stuck: %v", err)
	}
}
