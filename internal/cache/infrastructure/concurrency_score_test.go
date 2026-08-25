package infrastructure

import (
	"context"
	"sync"
	"testing"
	"time"

	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
)

func TestConcurrentCacheGetIsRaceFree(t *testing.T) {
	m := NewMemory()
	key := cache.Key{TenantID: "tenant-a", RuntimeVersion: "v1", ModuleDigest: "m"}
	m.PutFailure(context.Background(), key, context.Canceled, time.Minute)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				_, _ = m.Get(context.Background(), key)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestConcurrentTenantInvalidationIsRaceFree(t *testing.T) {
	m := NewMemory()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(seed int) {
			defer wg.Done()
			<-start
			for j := 0; j < 80; j++ {
				key := cache.Key{TenantID: "tenant-a", RuntimeVersion: "v1", ModuleDigest: string(rune(seed*1000 + j))}
				m.PutFailure(context.Background(), key, context.Canceled, time.Minute)
			}
		}(i)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 80; j++ {
				m.InvalidateTenant(context.Background(), "tenant-a")
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestConcurrentCacheStatsIsRaceFree(t *testing.T) {
	m := NewMemory()
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for j := 0; j < 500; j++ {
			key := cache.Key{TenantID: "tenant-a", RuntimeVersion: "v1", ModuleDigest: string(rune(j))}
			m.PutFailure(context.Background(), key, context.Canceled, time.Minute)
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for j := 0; j < 500; j++ {
			_ = m.Stats()
		}
	}()
	close(start)
	wg.Wait()
}
