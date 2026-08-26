package infrastructure

import (
	"context"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	"sync"
	"time"
)

type TenantUsage struct {
	TenantID     string        `json:"tenant_id"`
	Executions   uint64        `json:"executions"`
	Instructions uint64        `json:"instructions"`
	CPUTime      time.Duration `json:"cpu_time"`
	OutputBytes  uint64        `json:"output_bytes"`
	LastRecorded time.Time     `json:"last_recorded"`
}
type MemoryMeter struct {
	mu    sync.RWMutex
	usage map[string]TenantUsage
}

func NewMemoryMeter() *MemoryMeter { return &MemoryMeter{} }
func (m *MemoryMeter) Record(ctx context.Context, e execution.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := TenantUsage{}
	u.TenantID = e.TenantID
	u.Executions = 1
	u.Instructions += e.Usage.Instructions
	u.CPUTime += e.Usage.CPUTime
	u.OutputBytes += uint64(e.Usage.OutputBytes)
	u.LastRecorded = time.Now().UTC()
	m.usage[e.TenantID] = u
	return nil
}
func (m *MemoryMeter) Get(tenant string) TenantUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return TenantUsage{}
}
