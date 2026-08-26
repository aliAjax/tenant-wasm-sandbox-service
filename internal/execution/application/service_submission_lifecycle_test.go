package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type lifecycleRepo struct {
	mu          sync.Mutex
	executions  map[string]domain.Execution
	idempotency map[string]string
}

func newLifecycleRepo() *lifecycleRepo {
	return &lifecycleRepo{executions: map[string]domain.Execution{}, idempotency: map[string]string{}}
}

func (r *lifecycleRepo) Create(_ context.Context, e domain.Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executions[e.ID] = e
	return nil
}

func (r *lifecycleRepo) Update(_ context.Context, e domain.Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executions[e.ID] = e
	return nil
}

func (r *lifecycleRepo) Get(_ context.Context, id string) (domain.Execution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.executions[id]
	if !ok {
		return domain.Execution{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *lifecycleRepo) List(context.Context, string, int) ([]domain.Execution, error) {
	return nil, nil
}

func (r *lifecycleRepo) FindIdempotency(_ context.Context, tenant, key string) (domain.Execution, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.idempotency[tenant+":"+key]
	if !ok {
		return domain.Execution{}, false
	}
	e, ok := r.executions[id]
	return e, ok
}

func (r *lifecycleRepo) SaveIdempotency(_ context.Context, tenant, key, id string) {
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.idempotency[tenant+":"+key] = id
}

type lifecycleModules struct{ module moddomain.Module }

func (m lifecycleModules) Create(context.Context, moddomain.Module, []byte) error { return nil }
func (m lifecycleModules) Update(context.Context, moddomain.Module) error         { return nil }
func (m lifecycleModules) Get(context.Context, string) (moddomain.Module, error) {
	return m.module, nil
}
func (m lifecycleModules) Content(context.Context, string) ([]byte, error) { return nil, nil }
func (m lifecycleModules) List(context.Context, string) ([]moddomain.Module, error) {
	return nil, nil
}
func (m lifecycleModules) FindVersion(context.Context, string, string, string) (moddomain.Module, error) {
	return m.module, nil
}

type lifecycleScheduler struct {
	mu           sync.Mutex
	submitErrors []error
	cancelResult bool
}

func (s *lifecycleScheduler) Submit(context.Context, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.submitErrors) == 0 {
		return nil
	}
	err := s.submitErrors[0]
	s.submitErrors = s.submitErrors[1:]
	return err
}

func (s *lifecycleScheduler) Cancel(string) bool { return s.cancelResult }

type lifecycleIDs struct {
	mu sync.Mutex
	n  int
}

func (g *lifecycleIDs) NewID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return fmt.Sprintf("%s-%d", prefix, g.n)
}

type lifecycleClock struct{ now time.Time }

func (c lifecycleClock) Now() time.Time { return c.now }

func resetPayloadRegistry() {
	executionPayloadRegistry.Lock()
	defer executionPayloadRegistry.Unlock()
	executionPayloadRegistry.values = map[string][]byte{}
}

func newSubmissionService(repo *lifecycleRepo, scheduler *lifecycleScheduler) *Service {
	module := moddomain.Module{
		ID:       "mod-1",
		TenantID: "tenant-1",
		Version:  "1.0.0",
		Digest:   "sha256:module",
		State:    moddomain.StatePublished,
	}
	return New(repo, lifecycleModules{module: module}, nil, nil, nil, scheduler, nil, nil,
		&lifecycleIDs{}, lifecycleClock{now: time.Unix(100, 0)}, Options{MaxBudget: domain.DefaultBudget()})
}

func submissionRequest() domain.Request {
	return domain.Request{
		TenantID:       "tenant-1",
		ModuleID:       "mod-1",
		IdempotencyKey: "retry-key",
		Invocation: domain.Invocation{
			Kind:    domain.InputJSON,
			Payload: []byte(`{"job":"compile"}`),
		},
	}
}

func TestSubmitFailureDoesNotPoisonIdempotency(t *testing.T) {
	resetPayloadRegistry()
	repo := newLifecycleRepo()
	scheduler := &lifecycleScheduler{submitErrors: []error{errors.New("queue unavailable"), nil}}
	service := newSubmissionService(repo, scheduler)

	failed, err := service.Submit(context.Background(), submissionRequest())
	if err == nil {
		t.Fatal("first submission unexpectedly succeeded")
	}
	retried, err := service.Submit(context.Background(), submissionRequest())
	if err != nil {
		t.Fatalf("retry submission failed: %v", err)
	}
	if retried.ID == failed.ID {
		t.Fatalf("retry reused failed execution %q", failed.ID)
	}
	if retried.Status != domain.StatusQueued {
		t.Fatalf("retry status = %s, want queued", retried.Status)
	}
	replayed, err := service.Submit(context.Background(), submissionRequest())
	if err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	if replayed.ID != retried.ID {
		t.Fatalf("replay created %q, want successful execution %q", replayed.ID, retried.ID)
	}
}

func TestSubmitFailureReleasesPayload(t *testing.T) {
	resetPayloadRegistry()
	repo := newLifecycleRepo()
	scheduler := &lifecycleScheduler{submitErrors: []error{errors.New("queue unavailable")}}
	service := newSubmissionService(repo, scheduler)

	failed, err := service.Submit(context.Background(), submissionRequest())
	if err == nil {
		t.Fatal("submission unexpectedly succeeded")
	}
	if payload := executionPayloadRegistry.take(failed.ID); len(payload) != 0 {
		t.Fatalf("failed execution retained %d payload bytes", len(payload))
	}
}

func TestCancelRejectedBySchedulerKeepsQueuedState(t *testing.T) {
	repo := newLifecycleRepo()
	repo.executions["exe-queued"] = domain.Execution{ID: "exe-queued", Status: domain.StatusQueued}
	scheduler := &lifecycleScheduler{cancelResult: false}
	service := newSubmissionService(repo, scheduler)

	got, err := service.Cancel(context.Background(), "exe-queued")
	if err != nil {
		t.Fatalf("cancel returned error: %v", err)
	}
	if got.Status != domain.StatusQueued {
		t.Fatalf("status = %s, want queued", got.Status)
	}
	persisted, err := repo.Get(context.Background(), "exe-queued")
	if err != nil {
		t.Fatalf("get queued execution: %v", err)
	}
	if persisted.Status != domain.StatusQueued || persisted.FinishedAt != nil {
		t.Fatalf("rejected cancel mutated execution: %+v", persisted)
	}
}
