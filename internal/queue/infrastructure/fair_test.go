package infrastructure

import (
	"context"
	"testing"

	queue "github.com/acme/wasm-sandbox-executor/internal/queue/domain"
)

func TestFairQueueRotatesTenants(t *testing.T) {
	q := NewFairQueue()
	jobs := []queue.Job{{ExecutionID: "a1", TenantID: "a"}, {ExecutionID: "a2", TenantID: "a"}, {ExecutionID: "b1", TenantID: "b"}, {ExecutionID: "b2", TenantID: "b"}}
	for _, job := range jobs {
		if err := q.Enqueue(context.Background(), job); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"a1", "b1", "a2", "b2"}
	for i, id := range want {
		got, err := q.Dequeue(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got.ExecutionID != id {
			t.Fatalf("position %d got %s want %s", i, got.ExecutionID, id)
		}
	}
}

func TestFairQueueRejectsAfterClose(t *testing.T) {
	q := NewFairQueue()
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(context.Background(), queue.Job{TenantID: "a"}); err != queue.ErrClosed {
		t.Fatalf("got %v", err)
	}
}
