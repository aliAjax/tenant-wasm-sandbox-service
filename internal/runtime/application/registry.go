package application

import (
	"context"
	"fmt"
	"sort"
	"sync"

	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
)

type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]runtime.Runtime
}

func NewRegistry(runtimes ...runtime.Runtime) (*Registry, error) {
	r := &Registry{runtimes: map[string]runtime.Runtime{}}
	for _, candidate := range runtimes {
		if err := r.Register(candidate); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(candidate runtime.Runtime) error {
	if candidate == nil {
		return fmt.Errorf("runtime is nil")
	}
	version := candidate.Version()
	if version == "" {
		return fmt.Errorf("runtime version is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runtimes[version]; exists {
		return fmt.Errorf("runtime %s already registered", version)
	}
	r.runtimes[version] = candidate
	return nil
}

func (r *Registry) Get(version string) (runtime.Runtime, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	candidate, ok := r.runtimes[version]
	if !ok {
		return nil, fmt.Errorf("runtime %s not registered", version)
	}
	return candidate, nil
}

func (r *Registry) Versions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := make([]string, 0, len(r.runtimes))
	for version := range r.runtimes {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}

func (r *Registry) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for version, candidate := range r.runtimes {
		if err := candidate.Close(ctx); err != nil {
			return fmt.Errorf("close runtime %s: %w", version, err)
		}
	}
	return nil
}
