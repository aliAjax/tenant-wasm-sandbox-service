package infrastructure

import (
	"context"
	"errors"
	"fmt"
	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
	"sync"
	"sync/atomic"
	"time"
)

var ErrMiss = errors.New("cache miss")

type item struct {
	module runtime.CompiledModule
	entry  cache.Entry
}
type Memory struct {
	mu     sync.RWMutex
	items  map[cache.Key]item
	hits   atomic.Uint64
	misses atomic.Uint64
}

func NewMemory() *Memory { return &Memory{items: map[cache.Key]item{}} }
func (m *Memory) Get(ctx context.Context, key cache.Key) (runtime.CompiledModule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	snapshot, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		m.misses.Add(1)
		return nil, ErrMiss
	}
	if !snapshot.entry.ExpiresAt.IsZero() && time.Now().After(snapshot.entry.ExpiresAt) {
		m.mu.Lock()
		// Re-check under the write lock: another goroutine may have replaced or
		// removed the entry while we were waiting. Only evict the exact entry we
		// snapshotted so that a concurrent Put/Invalidate cannot lose data.
		current, present := m.items[key]
		if present && current.entry.ExpiresAt.Equal(snapshot.entry.ExpiresAt) {
			delete(m.items, key)
		}
		m.mu.Unlock()
		m.misses.Add(1)
		return nil, ErrMiss
	}
	m.mu.Lock()
	// Update access metadata under the write lock. Re-read the entry because a
	// concurrent Put may have replaced it; only mutate when it is still the one we
	// observed so the cache statistics and timestamps stay consistent.
	if current, present := m.items[key]; present && current.entry.ExpiresAt.Equal(snapshot.entry.ExpiresAt) {
		current.entry.Hits++
		current.entry.LastUsedAt = time.Now().UTC()
		m.items[key] = current
		snapshot = current
	}
	m.mu.Unlock()
	m.hits.Add(1)
	if snapshot.entry.Failure != "" {
		return nil, fmt.Errorf("cached compile failure: %s", snapshot.entry.Failure)
	}
	return snapshot.module, nil
}
func (m *Memory) Put(ctx context.Context, key cache.Key, module runtime.CompiledModule, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.items[key] = item{module: module, entry: cache.Entry{Key: key, CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(ttl)}}
	return nil
}
func (m *Memory) PutFailure(ctx context.Context, key cache.Key, failure error, ttl time.Duration) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.items[key] = item{entry: cache.Entry{Key: key, CreatedAt: now, LastUsedAt: now, Failure: failure.Error(), ExpiresAt: now.Add(ttl)}}
}
func (m *Memory) Invalidate(ctx context.Context, key cache.Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}
func (m *Memory) InvalidateTenant(ctx context.Context, tenant string) int {
	if ctx.Err() != nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for k := range m.items {
		if k.TenantID == tenant {
			delete(m.items, k)
			n++
		}
	}
	return n
}
func (m *Memory) Stats() cache.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := cache.Stats{Entries: len(m.items), Hits: m.hits.Load(), Misses: m.misses.Load()}
	for _, v := range m.items {
		if v.entry.Failure != "" {
			s.Failures++
		}
	}
	return s
}
