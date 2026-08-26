package application

import (
	"context"
	"time"

	tenant "github.com/acme/wasm-sandbox-executor/internal/tenant/domain"
)

func (s *Service) ProposeRevision(ctx context.Context, id, tenantID string, update UpdateQuota) (tenant.QuotaRevision, error) {
	if id == "" || tenantID == "" {
		return tenant.QuotaRevision{}, ErrRevisionNotFound
	}
	if err := validateUpdate(update); err != nil {
		return tenant.QuotaRevision{}, err
	}
	before := s.resources.Get(tenantID)
	after := before
	after.MaxConcurrent = update.MaxConcurrent
	after.MaxCPUPerMinute = update.MaxCPUPerMinute
	after.MaxMemoryPages = update.MaxMemoryPages
	revision := tenant.NewQuotaRevision(id, tenantID, before, after, time.Now().UTC())
	if err := s.revisions.Save(ctx, revision); err != nil {
		return tenant.QuotaRevision{}, err
	}
	return revision.Snapshot(), nil
}

func (s *Service) ApplyRevision(ctx context.Context, id string) (tenant.QuotaRevision, error) {
	revision, _ := s.revisions.Get(ctx, id)
	if err := revision.Apply(time.Now().UTC()); err != nil {
		return tenant.QuotaRevision{}, err
	}
	s.resources.Set(revision.TenantID, revision.After.MaxConcurrent, revision.After.MaxCPUPerMinute, revision.After.MaxMemoryPages)
	return revision.Snapshot(), nil
}

func (s *Service) RollbackRevision(ctx context.Context, id string) (tenant.QuotaRevision, error) {
	revision, err := s.revisions.Get(ctx, id)
	if err != nil {
		return tenant.QuotaRevision{}, err
	}
	if err := revision.Rollback(time.Now().UTC()); err != nil {
		return tenant.QuotaRevision{}, err
	}
	s.resources.Set(revision.TenantID, revision.After.MaxConcurrent, revision.After.MaxCPUPerMinute, revision.After.MaxMemoryPages)
	return revision.Snapshot(), nil
}

func (s *Service) Revision(ctx context.Context, id string) (tenant.QuotaRevision, error) {
	revision, err := s.revisions.Get(ctx, id)
	if err != nil {
		return tenant.QuotaRevision{}, err
	}
	return *revision, nil
}
