package infrastructure

import (
	"context"
	"errors"
	"fmt"
	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
	"sync"
	"time"
)

var ErrMiss = errors.New("cache miss")

type item struct {
	module runtime.CompiledModule
	entry  cache.Entry
}
type Memory struct {
	mu     sync.Mutex
	items  map[cache.Key]item
	hits   uint64
	misses uint64
}

func NewMemory() *Memory { return &Memory{items: map[cache.Key]item{}} }
func (m *Memory) Get(ctx context.Context, key cache.Key) (runtime.CompiledModule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	v, ok := m.items[key]
	m.mu.Unlock()
	if !ok {
		m.misses++
		return nil, ErrMiss
	}
	if !v.entry.ExpiresAt.IsZero() && time.Now().After(v.entry.ExpiresAt) {
		delete(m.items, key)
		m.misses++
		return nil, ErrMiss
	}
	v.entry.Hits++
	v.entry.LastUsedAt = time.Now().UTC()
	m.items[key] = v
	m.hits++
	if v.entry.Failure != "" {
		return nil, fmt.Errorf("cached compile failure: %s", v.entry.Failure)
	}
	return v.module, nil
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
	keys := make([]cache.Key, 0)
	for k := range m.items {
		if k.TenantID == tenant {
			keys = append(keys, k)
		}
	}
	m.mu.Unlock()
	n := 0
	for _, k := range keys {
		delete(m.items, k)
		n++
	}
	return n
}
func (m *Memory) Stats() cache.Stats {
	m.mu.Lock()
	items := m.items
	m.mu.Unlock()
	s := cache.Stats{Entries: len(items), Hits: m.hits, Misses: m.misses}
	for _, v := range items {
		if v.entry.Failure != "" {
			s.Failures++
		}
	}
	return s
}
