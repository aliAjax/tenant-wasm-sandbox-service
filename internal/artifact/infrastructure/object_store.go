package infrastructure

import (
	"context"
	"errors"
	"sync"
)

var ErrObjectNotFound = errors.New("object not found")

type MemoryObjectStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemoryObjectStore() *MemoryObjectStore {
	return &MemoryObjectStore{objects: map[string][]byte{}}
}
func (s *MemoryObjectStore) Put(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Store an immutable copy so later mutation of the caller's buffer cannot
	// rewrite the artifact bytes observed by concurrent Get callers.
	s.objects[key] = append([]byte(nil), value...)
	return nil
}
func (s *MemoryObjectStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	v, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrObjectNotFound
	}
	// Return a private copy so callers cannot mutate the stored snapshot.
	return append([]byte(nil), v...), nil
}
func (s *MemoryObjectStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
