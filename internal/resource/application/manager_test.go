package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerEnforcesConcurrentQuota(t *testing.T) {
	m := NewManager(1, time.Second, 10)
	if _, err := m.Acquire(context.Background(), "tenant", "one", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Acquire(context.Background(), "tenant", "two", 2); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("got %v", err)
	}
	m.Release("one", time.Millisecond)
	if _, err := m.Acquire(context.Background(), "tenant", "two", 2); err != nil {
		t.Fatal(err)
	}
}

func TestManagerEnforcesMemoryQuota(t *testing.T) {
	m := NewManager(1, time.Second, 3)
	if _, err := m.Acquire(context.Background(), "tenant", "one", 4); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("got %v", err)
	}
}
