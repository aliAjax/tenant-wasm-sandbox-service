package application

import (
	"context"
	"errors"
	"fmt"
	tenant "github.com/acme/wasm-sandbox-executor/internal/tenant/domain"
	"sync"
	"time"
)

var ErrQuotaExceeded = errors.New("tenant quota exceeded")

type Manager struct {
	mu           sync.Mutex
	quotas       map[string]tenant.Quota
	leases       map[string]tenant.Lease
	defaultQuota tenant.Quota
}

func NewManager(maxConcurrent int, maxCPU time.Duration, maxMemory uint32) *Manager {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Manager{quotas: map[string]tenant.Quota{}, leases: map[string]tenant.Lease{}, defaultQuota: tenant.Quota{MaxConcurrent: maxConcurrent, MaxCPUPerMinute: maxCPU, MaxMemoryPages: maxMemory}}
}
func (m *Manager) quotaLocked(id string) tenant.Quota {
	q, ok := m.quotas[id]
	if !ok {
		q = m.defaultQuota
		q.TenantID = id
		q.WindowStarted = time.Now().UTC()
	}
	if time.Since(q.WindowStarted) >= time.Minute {
		q.CPUUsed = 0
		q.WindowStarted = time.Now().UTC()
	}
	return q
}
func (m *Manager) Acquire(ctx context.Context, id, leaseID string, pages uint32) (tenant.Lease, error) {
	if err := ctx.Err(); err != nil {
		return tenant.Lease{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.quotaLocked(id)
	if q.Active >= q.MaxConcurrent {
		return tenant.Lease{}, fmt.Errorf("%w: concurrent slots %d/%d", ErrQuotaExceeded, q.Active, q.MaxConcurrent)
	}
	if pages > q.MaxMemoryPages {
		return tenant.Lease{}, fmt.Errorf("%w: memory pages %d/%d", ErrQuotaExceeded, pages, q.MaxMemoryPages)
	}
	if q.CPUUsed >= q.MaxCPUPerMinute {
		return tenant.Lease{}, fmt.Errorf("%w: CPU minute budget consumed", ErrQuotaExceeded)
	}
	q.Active++
	m.quotas[id] = q
	l := tenant.Lease{ID: leaseID, TenantID: id, MemoryPages: pages, GrantedAt: time.Now().UTC()}
	m.leases[leaseID] = l
	return l, nil
}
func (m *Manager) Release(leaseID string, cpu time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.leases[leaseID]
	if !ok {
		return
	}
	q := m.quotaLocked(l.TenantID)
	if q.Active > 0 {
		q.Active--
	}
	q.CPUUsed += cpu
	m.quotas[l.TenantID] = q
	delete(m.leases, leaseID)
}
func (m *Manager) Get(id string) tenant.Quota {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.quotaLocked(id)
	m.quotas[id] = q
	return q
}
func (m *Manager) Set(id string, maxConcurrent int, maxCPU time.Duration, maxMemory uint32) tenant.Quota {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.quotaLocked(id)
	q.MaxConcurrent = maxConcurrent
	q.MaxCPUPerMinute = maxCPU
	q.MaxMemoryPages = maxMemory
	m.quotas[id] = q
	return q
}
func (m *Manager) Active() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.leases) }
