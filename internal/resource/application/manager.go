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
var ErrLeaseExists = errors.New("resource lease already exists")

type Manager struct {
	mu           sync.Mutex
	quotas       map[string]tenant.Quota
	leases       map[string]tenant.Lease
	defaultQuota tenant.Quota
	now          func() time.Time
}

func NewManager(maxConcurrent int, maxCPU time.Duration, maxMemory uint32) *Manager {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Manager{quotas: map[string]tenant.Quota{}, leases: map[string]tenant.Lease{}, defaultQuota: tenant.Quota{MaxConcurrent: maxConcurrent, MaxCPUPerMinute: maxCPU, MaxMemoryPages: maxMemory}, now: time.Now}
}
func (m *Manager) quotaLocked(id string) tenant.Quota {
	q, ok := m.quotas[id]
	if !ok {
		q = m.defaultQuota
		q.TenantID = id
		q.WindowStarted = m.now().UTC()
	}
	q.Refresh(m.now().UTC())
	return q
}
func (m *Manager) Acquire(ctx context.Context, id, leaseID string, pages uint32) (tenant.Lease, error) {
	if err := ctx.Err(); err != nil {
		return tenant.Lease{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// The initial ctx.Err() check above runs before we hold the lock. If the
	// caller was canceled while blocked waiting for the lock (for example by a
	// concurrent Release, quota refresh, or tenant update holding the mutex),
	// honoring the request now would mint a lease for a dead request. Re-check
	// once we actually hold the lock so a cancellation during the wait still
	// aborts the acquisition.
	if err := ctx.Err(); err != nil {
		return tenant.Lease{}, err
	}
	if err := m.reserveLeaseID(leaseID); err != nil {
		return tenant.Lease{}, err
	}
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
	l := tenant.Lease{ID: leaseID, TenantID: id, MemoryPages: pages, GrantedAt: m.now().UTC()}
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
	q.AddCPU(cpu)
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

// reserveLeaseID rejects a lease id that is already tracked. Reusing an id
// while the previous lease is still live would silently overwrite the old
// lease map entry while leaving its contribution to the tenant's Active
// counter in place, double-charging the quota and losing track of the
// original holder. Rejecting the duplicate keeps the bookkeeping paired.
func (m *Manager) reserveLeaseID(leaseID string) error {
	if _, ok := m.leases[leaseID]; ok {
		return fmt.Errorf("%w: lease %s", ErrLeaseExists, leaseID)
	}
	return nil
}
