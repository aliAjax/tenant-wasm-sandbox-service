package application

import (
	"context"
	"errors"
	"testing"
	"time"

	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	tenant "github.com/acme/wasm-sandbox-executor/internal/tenant/domain"
)

func newRevisionService() (*Service, *resource.Manager) {
	resources := resource.NewManager(2, time.Minute, 64)
	return NewService(resources), resources
}

func revisionUpdate() UpdateQuota {
	return UpdateQuota{MaxConcurrent: 7, MaxCPUPerMinute: 3 * time.Minute, MaxMemoryPages: 512}
}

func TestTypedNilRevisionStoreFallsBack(t *testing.T) {
	resources := resource.NewManager(2, time.Minute, 64)
	var revisions *MemoryRevisionStore
	service := NewServiceWithRevisionStore(resources, revisions)
	if _, err := service.ProposeRevision(context.Background(), "rev-typed-nil", "tenant-a", revisionUpdate()); err != nil {
		t.Fatalf("typed-nil store should fall back to memory store: %v", err)
	}
}

func TestApplyMissingRevisionReturnsError(t *testing.T) {
	service, _ := newRevisionService()
	if _, err := service.ApplyRevision(context.Background(), "missing"); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("ApplyRevision error = %v, want ErrRevisionNotFound", err)
	}
}

func TestProposeRevisionHonorsCanceledContext(t *testing.T) {
	service, _ := newRevisionService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ProposeRevision(ctx, "rev-canceled", "tenant-a", revisionUpdate()); !errors.Is(err, context.Canceled) {
		t.Fatalf("ProposeRevision error = %v, want context.Canceled", err)
	}
	if _, err := service.Revision(context.Background(), "rev-canceled"); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("canceled proposal was persisted: %v", err)
	}
}

func TestRevisionReadHonorsCanceledContext(t *testing.T) {
	service, _ := newRevisionService()
	if _, err := service.ProposeRevision(context.Background(), "rev-read", "tenant-a", revisionUpdate()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Revision(ctx, "rev-read"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Revision error = %v, want context.Canceled", err)
	}
}

func TestRevisionSnapshotDoesNotMutateStore(t *testing.T) {
	service, _ := newRevisionService()
	if _, err := service.ProposeRevision(context.Background(), "rev-snapshot", "tenant-a", revisionUpdate()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Revision(context.Background(), "rev-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Events[0].Kind = "corrupted"
	again, err := service.Revision(context.Background(), "rev-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if again.Events[0].Kind != string(tenant.RevisionProposed) {
		t.Fatalf("stored event mutated through snapshot: %q", again.Events[0].Kind)
	}
}

func TestRevisionCannotApplyAfterRollback(t *testing.T) {
	service, _ := newRevisionService()
	if _, err := service.ProposeRevision(context.Background(), "rev-terminal", "tenant-a", revisionUpdate()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyRevision(context.Background(), "rev-terminal"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RollbackRevision(context.Background(), "rev-terminal"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyRevision(context.Background(), "rev-terminal"); !errors.Is(err, tenant.ErrRevisionTransition) {
		t.Fatalf("second ApplyRevision error = %v, want transition error", err)
	}
}

func TestRevisionRejectsRollbackBeforeApply(t *testing.T) {
	service, _ := newRevisionService()
	if _, err := service.ProposeRevision(context.Background(), "rev-proposed", "tenant-a", revisionUpdate()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RollbackRevision(context.Background(), "rev-proposed"); !errors.Is(err, tenant.ErrRevisionTransition) {
		t.Fatalf("RollbackRevision error = %v, want transition error", err)
	}
}

func TestRollbackRestoresPreviousQuota(t *testing.T) {
	service, resources := newRevisionService()
	resources.Set("tenant-a", 3, 2*time.Minute, 96)
	if _, err := service.ProposeRevision(context.Background(), "rev-restore", "tenant-a", revisionUpdate()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyRevision(context.Background(), "rev-restore"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RollbackRevision(context.Background(), "rev-restore"); err != nil {
		t.Fatal(err)
	}
	quota := resources.Get("tenant-a")
	if quota.MaxConcurrent != 3 || quota.MaxCPUPerMinute != 2*time.Minute || quota.MaxMemoryPages != 96 {
		t.Fatalf("rollback quota = %+v, want original limits", quota)
	}
}
