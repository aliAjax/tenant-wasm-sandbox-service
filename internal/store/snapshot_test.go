package store

import (
	"context"
	"sync"
	"testing"
	"time"

	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

func makeModule() moddomain.Module {
	return moddomain.Module{
		ID:           "mod-1",
		TenantID:     "tenant-1",
		Name:         "demo",
		Version:      "1.0.0",
		Capabilities: []moddomain.Capability{{Name: "net", Access: "read"}},
		EnvAllowlist: []string{"PATH"},
		ImportList:   []string{"env"},
		State:        moddomain.StateDraft,
		CreatedAt:    time.Unix(1, 0).UTC(),
	}
}

func TestStoreModuleMutationDoesNotPersist(t *testing.T) {
	m, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Create(context.Background(), makeModule(), []byte("body")); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(context.Background(), "mod-1")
	if err != nil {
		t.Fatal(err)
	}
	got.Capabilities[0].Name = "mutated"
	got.EnvAllowlist[0] = "SECRET"
	got.ImportList[0] = "fs"
	again, err := m.Get(context.Background(), "mod-1")
	if err != nil {
		t.Fatal(err)
	}
	if again.Capabilities[0].Name != "net" || again.EnvAllowlist[0] != "PATH" || again.ImportList[0] != "env" {
		t.Fatalf("store was mutated through returned module: %#v", again)
	}
}

func TestStoreModuleListIsSnapshot(t *testing.T) {
	m, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Create(context.Background(), makeModule(), []byte("body")); err != nil {
		t.Fatal(err)
	}
	items, err := m.List(context.Background(), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	items[0].Capabilities[0].Access = "write"
	items[0].ImportList = append(items[0].ImportList, "clock")
	again, err := m.List(context.Background(), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Capabilities[0].Access != "read" || len(again[0].ImportList) != 1 {
		t.Fatalf("list returned shared backing storage: %#v", again[0])
	}
}

func TestStoreExecutionGetIsSnapshot(t *testing.T) {
	m, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	repo := ExecutionRepository{Store: m}
	e := execdomain.Execution{ID: "exec-1", TenantID: "tenant-1", Status: execdomain.StatusSucceeded, Output: []byte("result")}
	if err := repo.Create(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), "exec-1")
	if err != nil {
		t.Fatal(err)
	}
	got.Output[0] = 'X'
	again, err := repo.Get(context.Background(), "exec-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Output) != "result" {
		t.Fatalf("execution output was mutated through returned value: %q", again.Output)
	}
}

func TestStoreConcurrentListRaceFree(t *testing.T) {
	m, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Create(context.Background(), makeModule(), []byte("body")); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 500; i++ {
			items, listErr := m.List(context.Background(), "tenant-1")
			if listErr != nil || len(items) == 0 {
				continue
			}
			items[0].Capabilities[0].Name = "mutated"
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 500; i++ {
			items, listErr := m.List(context.Background(), "tenant-1")
			if listErr == nil && len(items) > 0 {
				_ = items[0].Capabilities[0].Name
			}
		}
	}()
	close(start)
	wg.Wait()
	got, err := m.Get(context.Background(), "mod-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Capabilities[0].Name != "net" {
		t.Fatalf("concurrent list mutated internal store: %q", got.Capabilities[0].Name)
	}
}
