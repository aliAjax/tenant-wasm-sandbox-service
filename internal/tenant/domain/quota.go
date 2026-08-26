package domain

import "time"

type Quota struct {
	TenantID        string        `json:"tenant_id"`
	MaxConcurrent   int           `json:"max_concurrent"`
	MaxCPUPerMinute time.Duration `json:"max_cpu_per_minute"`
	MaxMemoryPages  uint32        `json:"max_memory_pages"`
	Active          int           `json:"active"`
	CPUUsed         time.Duration `json:"cpu_used"`
	WindowStarted   time.Time     `json:"window_started"`
}
type Lease struct {
	ID          string
	TenantID    string
	MemoryPages uint32
	GrantedAt   time.Time
}

// Refresh rolls the per-minute accounting window forward when the supplied
// clock has advanced past the current window. It tolerates clock regressions
// (for example NTP step-backs) by treating a non-monotonic "now" as a signal
// that the window must be reset: once the recorded WindowStarted lies in the
// future relative to now, the counter is discarded and the window is rebuilt
// around now so stale budget from a rolled-back clock can never leak in.
func (q *Quota) Refresh(now time.Time) {
	if !q.WindowStarted.IsZero() && now.Before(q.WindowStarted) {
		q.CPUUsed = 0
		q.WindowStarted = now
		return
	}
	if now.Sub(q.WindowStarted) >= time.Minute {
		q.CPUUsed = 0
		q.WindowStarted = now
	}
}

// AddCPU accumulates consumed CPU time into the current window. Negative
// durations can arise from clock regressions or downstream execution-time
// reporting bugs; they are clamped to zero so the budget can never be driven
// below zero and a subsequent over-report cannot mint free quota.
func (q *Quota) AddCPU(cpu time.Duration) {
	if cpu <= 0 {
		return
	}
	q.CPUUsed += cpu
}
