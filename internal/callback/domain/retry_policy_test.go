package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryPolicyWaitHonorsCanceledContext(t *testing.T) {
	policy := RetryPolicy{InitialDelay: time.Second, MaximumDelay: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := policy.Wait(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("wait ignored cancellation for %s", elapsed)
	}
}
