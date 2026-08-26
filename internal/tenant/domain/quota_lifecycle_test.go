package domain

import (
	"testing"
	"time"
)

func TestQuotaRefreshHandlesFutureWindow(t *testing.T) {
	now := time.Now().UTC()
	q := Quota{CPUUsed: time.Second, WindowStarted: now.Add(time.Hour)}
	q.Refresh(now)
	if q.CPUUsed != 0 || !q.WindowStarted.Equal(now) {
		t.Fatalf("future window was not reset: %#v", q)
	}
}
