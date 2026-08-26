package infrastructure

import (
	"context"
	"testing"
	"time"

	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
)

func TestCacheTenantPartitions(t *testing.T) {
	m := NewMemory()
	keys := []cache.Key{{TenantID: "a", RuntimeVersion: "v1", ModuleDigest: "same"}, {TenantID: "b", RuntimeVersion: "v1", ModuleDigest: "same"}}
	for _, key := range keys {
		m.PutFailure(context.Background(), key, context.Canceled, time.Minute)
	}
	if got := m.Stats().Entries; got != 2 {
		t.Fatalf("entries=%d", got)
	}
	if got := m.InvalidateTenant(context.Background(), "a"); got != 1 {
		t.Fatalf("invalidated=%d", got)
	}
	if got := m.Stats().Entries; got != 1 {
		t.Fatalf("remaining=%d", got)
	}
}

func TestCacheExpiresEntries(t *testing.T) {
	m := NewMemory()
	key := cache.Key{TenantID: "a", RuntimeVersion: "v1", ModuleDigest: "digest"}
	m.PutFailure(context.Background(), key, context.Canceled, -time.Second)
	if _, err := m.Get(context.Background(), key); err != ErrMiss {
		t.Fatalf("got %v want miss", err)
	}
}
