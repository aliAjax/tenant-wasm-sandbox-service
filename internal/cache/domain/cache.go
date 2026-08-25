package domain

import (
	"context"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
	"time"
)

type Key struct {
	TenantID       string
	RuntimeVersion string
	ModuleDigest   string
}
type Entry struct {
	Key        Key       `json:"key"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Hits       uint64    `json:"hits"`
	Failure    string    `json:"failure,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Cache interface {
	Get(context.Context, Key) (runtime.CompiledModule, error)
	Put(context.Context, Key, runtime.CompiledModule, time.Duration) error
	PutFailure(context.Context, Key, error, time.Duration)
	Invalidate(context.Context, Key) error
	InvalidateTenant(context.Context, string) int
	Stats() Stats
}
type Stats struct {
	Entries  int    `json:"entries"`
	Hits     uint64 `json:"hits"`
	Misses   uint64 `json:"misses"`
	Failures int    `json:"failures"`
}
