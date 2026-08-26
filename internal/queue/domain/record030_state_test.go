package domain

import "testing"

func TestQueueStateTransitionsDuringDrain(t *testing.T) {
	if got := QueueOpen.BeginClose(2); got != QueueDraining {
		t.Fatalf("begin close with pending jobs = %q, want %q", got, QueueDraining)
	}
	if QueueDraining.IsTerminal() {
		t.Fatal("draining queue was treated as terminal")
	}
	if got := QueueDraining.AfterDequeue(1); got != QueueDraining {
		t.Fatalf("state with one pending job = %q, want %q", got, QueueDraining)
	}
	if got := QueueDraining.AfterDequeue(0); got != QueueClosed {
		t.Fatalf("state after final dequeue = %q, want %q", got, QueueClosed)
	}
}
