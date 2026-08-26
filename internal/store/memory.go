package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type snapshot struct {
	Modules     map[string]moddomain.Module     `json:"modules"`
	Contents    map[string][]byte               `json:"contents"`
	Executions  map[string]execdomain.Execution `json:"executions"`
	Idempotency map[string]string               `json:"idempotency"`
}

type Memory struct {
	mu   sync.RWMutex
	path string
	data snapshot
}

func New(path string) (*Memory, error) {
	m := &Memory{path: path, data: snapshot{Modules: map[string]moddomain.Module{}, Contents: map[string][]byte{}, Executions: map[string]execdomain.Execution{}, Idempotency: map[string]string{}}}
	if path == "" {
		return m, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if len(b) > 0 {
		if err = json.Unmarshal(b, &m.data); err != nil {
			return nil, fmt.Errorf("decode state: %w", err)
		}
	}
	m.normalize()
	return m, nil
}

func (m *Memory) normalize() {
	if m.data.Modules == nil {
		m.data.Modules = map[string]moddomain.Module{}
	}
	if m.data.Contents == nil {
		m.data.Contents = map[string][]byte{}
	}
	if m.data.Executions == nil {
		m.data.Executions = map[string]execdomain.Execution{}
	}
	if m.data.Idempotency == nil {
		m.data.Idempotency = map[string]string{}
	}
}

func (m *Memory) persistLocked() error {
	if m.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	b, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := m.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err = os.Rename(tmp, m.path); err != nil {
		return fmt.Errorf("commit state: %w", err)
	}
	return nil
}

func (m *Memory) Create(ctx context.Context, mod moddomain.Module, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data.Modules[mod.ID]; ok {
		return fmt.Errorf("module id already exists")
	}
	for _, v := range m.data.Modules {
		if v.TenantID == mod.TenantID && v.Name == mod.Name && v.Version == mod.Version {
			return fmt.Errorf("module version already exists")
		}
	}
	m.data.Modules[mod.ID] = mod
	m.data.Contents[mod.ID] = append([]byte(nil), content...)
	return m.persistLocked()
}
func (m *Memory) Update(ctx context.Context, mod moddomain.Module) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data.Modules[mod.ID]; !ok {
		return moddomain.ErrNotFound
	}
	m.data.Modules[mod.ID] = mod
	return m.persistLocked()
}
func (m *Memory) Get(ctx context.Context, id string) (moddomain.Module, error) {
	if err := ctx.Err(); err != nil {
		return moddomain.Module{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data.Modules[id]
	if !ok {
		return moddomain.Module{}, moddomain.ErrNotFound
	}
	return v, nil
}
func (m *Memory) Content(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data.Contents[id]
	if !ok {
		return nil, moddomain.ErrNotFound
	}
	return append([]byte(nil), v...), nil
}
func (m *Memory) List(ctx context.Context, tenant string) ([]moddomain.Module, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]moddomain.Module, 0)
	for _, v := range m.data.Modules {
		if tenant == "" || v.TenantID == tenant {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (m *Memory) FindVersion(ctx context.Context, tenant, name, version string) (moddomain.Module, error) {
	mods, err := m.List(ctx, tenant)
	if err != nil {
		return moddomain.Module{}, err
	}
	for _, v := range mods {
		if v.Name == name && v.Version == version {
			return v, nil
		}
	}
	return moddomain.Module{}, moddomain.ErrNotFound
}

func (m *Memory) CreateExecution(ctx context.Context, e execdomain.Execution) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data.Executions[e.ID]; ok {
		return fmt.Errorf("execution exists")
	}
	m.data.Executions[e.ID] = e
	return m.persistLocked()
}
func (m *Memory) UpdateExecution(ctx context.Context, e execdomain.Execution) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data.Executions[e.ID]; !ok {
		return execdomain.ErrNotFound
	}
	m.data.Executions[e.ID] = e
	return m.persistLocked()
}

type ExecutionRepository struct{ Store *Memory }

func (r ExecutionRepository) Create(ctx context.Context, e execdomain.Execution) error {
	return r.Store.CreateExecution(ctx, e)
}
func (r ExecutionRepository) Update(ctx context.Context, e execdomain.Execution) error {
	return r.Store.UpdateExecution(ctx, e)
}
func (r ExecutionRepository) Get(ctx context.Context, id string) (execdomain.Execution, error) {
	if err := ctx.Err(); err != nil {
		return execdomain.Execution{}, err
	}
	r.Store.mu.RLock()
	defer r.Store.mu.RUnlock()
	v, ok := r.Store.data.Executions[id]
	if !ok {
		return execdomain.Execution{}, execdomain.ErrNotFound
	}
	return v, nil
}
func (r ExecutionRepository) List(ctx context.Context, tenant string, limit int) ([]execdomain.Execution, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.Store.mu.RLock()
	defer r.Store.mu.RUnlock()
	out := make([]execdomain.Execution, 0)
	for _, v := range r.Store.data.Executions {
		if tenant == "" || v.TenantID == tenant {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r ExecutionRepository) FindIdempotency(ctx context.Context, tenant, key string) (execdomain.Execution, bool) {
	if ctx.Err() != nil {
		return execdomain.Execution{}, false
	}
	r.Store.mu.RLock()
	defer r.Store.mu.RUnlock()
	id, ok := r.Store.data.Idempotency[tenant+":"+key]
	if !ok {
		return execdomain.Execution{}, false
	}
	v, ok := r.Store.data.Executions[id]
	return v, ok
}
func (r ExecutionRepository) SaveIdempotency(ctx context.Context, tenant, key, id string) {
	if ctx.Err() != nil || key == "" {
		return
	}
	r.Store.mu.Lock()
	defer r.Store.mu.Unlock()
	r.Store.data.Idempotency[tenant+":"+key] = id
	_ = r.Store.persistLocked()
}

func (m *Memory) RecoverInterrupted(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, e := range m.data.Executions {
		if e.Status == execdomain.StatusRunning || e.Status == execdomain.StatusQueued {
			e.Status = execdomain.StatusFailed
			e.ErrorClass = execdomain.ErrorUnavailable
			e.ErrorMessage = "execution interrupted by service restart"
			m.data.Executions[id] = e
			n++
		}
	}
	return n, m.persistLocked()
}
