package application

import (
	"context"
	"errors"
	"testing"

	worker "github.com/acme/wasm-sandbox-executor/internal/worker/domain"
)

func TestDrainCanceledContextLeavesNodeReady(t *testing.T) {
	registry := NewNodeRegistry("node-state", 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := registry.Drain(ctx, "node-state"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Drain error = %v, want context.Canceled", err)
	}
	if state := registry.State().State; state != worker.NodeReady {
		t.Fatalf("state = %s, want ready", state)
	}
}

func TestDrainRejectsIsolatedNode(t *testing.T) {
	registry := NewNodeRegistry("node-state", 4)
	if _, err := registry.Isolate(context.Background(), "node-state", "maintenance"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Drain(context.Background(), "node-state"); !errors.Is(err, worker.ErrNodeUnavailable) {
		t.Fatalf("Drain error = %v, want ErrNodeUnavailable", err)
	}
	if state := registry.State().State; state != worker.NodeIsolated {
		t.Fatalf("state = %s, want isolated", state)
	}
}

func TestReadyFalseDuringDrain(t *testing.T) {
	registry := NewNodeRegistry("node-state", 4)
	if _, err := registry.Drain(context.Background(), "node-state"); err != nil {
		t.Fatal(err)
	}
	if registry.Ready() {
		t.Fatal("draining node reported ready")
	}
}
