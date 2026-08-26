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
	s.objects[key] = append([]byte(nil), value...)
	return nil
}
func (s *MemoryObjectStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
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
