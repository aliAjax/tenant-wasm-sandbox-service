package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
	cacheinfra "github.com/acme/wasm-sandbox-executor/internal/cache/infrastructure"
	"github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
)

type Service struct {
	repo      domain.Repository
	modules   moddomain.Repository
	runtime   runtime.Runtime
	cache     cache.Cache
	resources *resource.Manager
	scheduler domain.Scheduler
	meter     domain.Meter
	callback  domain.Callback
	ids       IDGenerator
	clock     Clock
	maxBudget domain.Budget
	nodeID    string
	cacheTTL  time.Duration
	mu        sync.Mutex
	cancels   map[string]context.CancelFunc
}
type IDGenerator interface{ NewID(string) string }
type Clock interface{ Now() time.Time }
type Options struct {
	MaxBudget domain.Budget
	NodeID    string
	CacheTTL  time.Duration
}

func New(repo domain.Repository, modules moddomain.Repository, rt runtime.Runtime, c cache.Cache, resources *resource.Manager, scheduler domain.Scheduler, meter domain.Meter, callback domain.Callback, ids IDGenerator, clock Clock, opts Options) *Service {
	if opts.CacheTTL == 0 {
		opts.CacheTTL = 15 * time.Minute
	}
	return &Service{repo: repo, modules: modules, runtime: rt, cache: c, resources: resources, scheduler: scheduler, meter: meter, callback: callback, ids: ids, clock: clock, maxBudget: opts.MaxBudget, nodeID: opts.NodeID, cacheTTL: opts.CacheTTL, cancels: map[string]context.CancelFunc{}}
}

func (s *Service) Submit(ctx context.Context, in domain.Request) (domain.Execution, error) {
	if in.TenantID == "" || in.ModuleID == "" {
		return domain.Execution{}, fmt.Errorf("tenant_id and module_id are required")
	}
	if in.IdempotencyKey != "" {
		if old, ok := s.repo.FindIdempotency(ctx, in.TenantID, in.IdempotencyKey); ok {
			return old, nil
		}
	}
	m, err := s.modules.Get(ctx, in.ModuleID)
	if err != nil {
		return domain.Execution{}, fmt.Errorf("get module: %w", err)
	}
	if m.TenantID != in.TenantID {
		return domain.Execution{}, fmt.Errorf("module is not owned by tenant")
	}
	if !m.IsRunnable() {
		return domain.Execution{}, fmt.Errorf("module is not published")
	}
	if err = validateInvocation(m, in.Invocation); err != nil {
		return domain.Execution{}, err
	}
	in.Budget = in.Budget.Normalize(s.maxBudget)
	if err = in.Budget.Validate(s.maxBudget); err != nil {
		return domain.Execution{}, err
	}
	digest := requestDigest(in)
	e := domain.Execution{ID: s.ids.NewID("exe"), TenantID: in.TenantID, ModuleID: m.ID, ModuleVersion: m.Version, ModuleDigest: m.Digest, RequestDigest: digest, Status: domain.StatusQueued, Budget: in.Budget, ErrorClass: domain.ErrorNone, CallbackURL: in.CallbackURL, CreatedAt: s.clock.Now().UTC()}
	if err = s.repo.Create(ctx, e); err != nil {
		return domain.Execution{}, fmt.Errorf("create execution: %w", err)
	}
	s.repo.SaveIdempotency(ctx, in.TenantID, in.IdempotencyKey, e.ID)
	executionPayloadRegistry.put(e.ID, in.Invocation.Payload)
	if err = s.scheduler.Submit(ctx, e.ID, e.TenantID); err != nil {
		e.Status = domain.StatusFailed
		e.ErrorClass = domain.ErrorUnavailable
		e.ErrorMessage = err.Error()
		_ = s.repo.Update(ctx, e)
		return e, fmt.Errorf("schedule execution: %w", err)
	}
	return e, nil
}

func (s *Service) Run(parent context.Context, id string) error {
	e, err := s.repo.Get(parent, id)
	if err != nil {
		return err
	}
	if e.Terminal() {
		return nil
	}
	m, err := s.modules.Get(parent, e.ModuleID)
	if err != nil {
		return s.fail(parent, &e, domain.ErrorInvalid, err)
	}
	lease, err := s.resources.Acquire(parent, e.TenantID, e.ID, e.Budget.MaxMemoryPages)
	if err != nil {
		return s.fail(parent, &e, domain.ErrorResource, err)
	}
	ctx, cancel := context.WithTimeout(parent, e.Budget.Timeout)
	s.mu.Lock()
	s.cancels[id] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancels, id)
		s.mu.Unlock()
		s.resources.Release(lease.ID, e.Usage.CPUTime)
	}()
	now := s.clock.Now().UTC()
	e.Status = domain.StatusRunning
	e.StartedAt = &now
	e.NodeID = s.nodeID
	if err = s.repo.Update(ctx, e); err != nil {
		return err
	}
	content, err := s.modules.Content(ctx, m.ID)
	if err != nil {
		return s.fail(context.Background(), &e, domain.ErrorInvalid, err)
	}
	key := cache.Key{TenantID: e.TenantID, RuntimeVersion: s.runtime.Version(), ModuleDigest: m.Digest}
	compiled, err := s.cache.Get(ctx, key)
	if errors.Is(err, cacheinfra.ErrMiss) {
		compiled, err = s.runtime.Compile(ctx, content, m.Digest)
		if err != nil {
			s.cache.PutFailure(context.Background(), key, err, time.Minute)
			return s.fail(context.Background(), &e, domain.ErrorRuntime, err)
		}
		if err = s.cache.Put(ctx, key, compiled, s.cacheTTL); err != nil {
			return s.fail(context.Background(), &e, domain.ErrorRuntime, err)
		}
	} else if err != nil {
		return s.fail(context.Background(), &e, domain.ErrorRuntime, err)
	}
	invocation := domain.Invocation{Kind: domain.InputJSON, Payload: executionPayloadRegistry.take(id)}
	result, runErr := s.runtime.Execute(ctx, compiled, m.Entry, invocation, e.Budget)
	e.Usage = result.Usage
	e.ExitCode = result.ExitCode
	e.Output = result.Output
	e.OutputTruncated = result.Truncated
	e.OutputDigest = digestBytes(result.Output)
	finished := s.clock.Now().UTC()
	e.FinishedAt = &finished
	if runErr != nil {
		switch {
		case errors.Is(runErr, context.Canceled):
			e.Status = domain.StatusCanceled
			e.ErrorClass = domain.ErrorCanceled
		case errors.Is(runErr, context.DeadlineExceeded):
			e.Status = domain.StatusTimedOut
			e.ErrorClass = domain.ErrorTimeout
		case errors.Is(runErr, runtime.ErrInstructionLimit), errors.Is(runErr, runtime.ErrMemoryLimit), errors.Is(runErr, runtime.ErrStackLimit):
			e.Status = domain.StatusFailed
			e.ErrorClass = domain.ErrorResource
		default:
			e.Status = domain.StatusFailed
			e.ErrorClass = domain.ErrorRuntime
		}
		e.ErrorMessage = runErr.Error()
	} else {
		e.Status = domain.StatusSucceeded
		e.ErrorClass = domain.ErrorNone
	}
	if err = s.repo.Update(context.Background(), e); err != nil {
		return err
	}
	if s.meter != nil {
		_ = s.meter.Record(context.Background(), e)
	}
	if s.callback != nil && e.CallbackURL != "" {
		go func(done domain.Execution) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.callback.Deliver(ctx, done)
		}(e)
	}
	return runErr
}

func (s *Service) RememberPayload(id string, payload []byte) {
	executionPayloadRegistry.put(id, payload)
}
func (s *Service) Get(ctx context.Context, id string) (domain.Execution, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, tenant string, limit int) ([]domain.Execution, error) {
	return s.repo.List(ctx, tenant, limit)
}
func (s *Service) Cancel(ctx context.Context, id string) (domain.Execution, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Execution{}, err
	}
	if e.Terminal() {
		return e, nil
	}
	s.mu.Lock()
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	} else {
		s.scheduler.Cancel(id)
		now := s.clock.Now().UTC()
		e.Status = domain.StatusCanceled
		e.ErrorClass = domain.ErrorCanceled
		e.ErrorMessage = "canceled before execution"
		e.FinishedAt = &now
		if err = s.repo.Update(ctx, e); err != nil {
			return domain.Execution{}, err
		}
	}
	return s.repo.Get(ctx, id)
}
func (s *Service) fail(ctx context.Context, e *domain.Execution, class domain.ErrorClass, cause error) error {
	now := s.clock.Now().UTC()
	e.Status = domain.StatusFailed
	e.ErrorClass = class
	e.ErrorMessage = cause.Error()
	e.FinishedAt = &now
	if err := s.repo.Update(ctx, *e); err != nil {
		return fmt.Errorf("persist failure: %w", err)
	}
	return cause
}

func validateInvocation(m moddomain.Module, in domain.Invocation) error {
	if in.Kind == "" {
		in.Kind = domain.InputJSON
	}
	switch in.Kind {
	case domain.InputJSON, domain.InputBinary, domain.InputStream:
	default:
		return fmt.Errorf("unsupported input kind")
	}
	allowed := map[string]bool{}
	for _, v := range m.EnvAllowlist {
		allowed[v] = true
	}
	for k := range in.Environment {
		if !allowed[k] {
			return fmt.Errorf("environment variable %q is not allowed", k)
		}
	}
	return nil
}
func requestDigest(in domain.Request) string {
	safe := struct {
		TenantID, ModuleID string
		Kind               domain.InputKind
		PayloadDigest      string
		Budget             domain.Budget
	}{in.TenantID, in.ModuleID, in.Invocation.Kind, digestBytes(in.Invocation.Payload), in.Budget}
	b, _ := json.Marshal(safe)
	return digestBytes(b)
}
func digestBytes(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

type payloads struct {
	sync.Mutex
	values map[string][]byte
}

var executionPayloadRegistry = &payloads{values: map[string][]byte{}}

func (p *payloads) put(id string, b []byte) {
	p.Lock()
	defer p.Unlock()
	p.values[id] = append([]byte(nil), b...)
}
func (p *payloads) take(id string) []byte {
	p.Lock()
	defer p.Unlock()
	b := p.values[id]
	delete(p.values, id)
	return b
}
