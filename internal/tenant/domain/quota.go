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

func (q *Quota) Refresh(now time.Time) {
	if now.Sub(q.WindowStarted) >= time.Minute {
		q.CPUUsed = 0
		q.WindowStarted = now
	}
}

func (q *Quota) AddCPU(cpu time.Duration) { q.CPUUsed += cpu }
