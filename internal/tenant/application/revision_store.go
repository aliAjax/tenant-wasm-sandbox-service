package application

import (
	"context"
	"errors"
	"sync"

	tenant "github.com/acme/wasm-sandbox-executor/internal/tenant/domain"
)

var ErrRevisionNotFound = errors.New("quota revision not found")

type RevisionStore interface {
	Save(context.Context, tenant.QuotaRevision) error
	Get(context.Context, string) (*tenant.QuotaRevision, error)
}

type MemoryRevisionStore struct {
	mu        sync.Mutex
	revisions map[string]*tenant.QuotaRevision
}

func NewMemoryRevisionStore() *MemoryRevisionStore {
	return &MemoryRevisionStore{revisions: map[string]*tenant.QuotaRevision{}}
}

func (s *MemoryRevisionStore) Save(_ context.Context, revision tenant.QuotaRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := revision.Snapshot()
	s.revisions[revision.ID] = &snapshot
	return nil
}

func (s *MemoryRevisionStore) Get(_ context.Context, id string) (*tenant.QuotaRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision, ok := s.revisions[id]
	if !ok {
		return nil, ErrRevisionNotFound
	}
	return revision, nil
}
