package application

import (
	"fmt"
	"time"

	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	tenant "github.com/acme/wasm-sandbox-executor/internal/tenant/domain"
)

type Service struct {
	resources *resource.Manager
	revisions RevisionStore
}

type UpdateQuota struct {
	MaxConcurrent   int
	MaxCPUPerMinute time.Duration
	MaxMemoryPages  uint32
}

func NewService(resources *resource.Manager) *Service {
	return NewServiceWithRevisionStore(resources, NewMemoryRevisionStore())
}

func NewServiceWithRevisionStore(resources *resource.Manager, revisions RevisionStore) *Service {
	if revisions == nil {
		revisions = NewMemoryRevisionStore()
	}
	return &Service{resources: resources, revisions: revisions}
}

func (s *Service) Get(tenantID string) (tenant.Quota, error) {
	if tenantID == "" {
		return tenant.Quota{}, fmt.Errorf("tenant id is required")
	}
	return s.resources.Get(tenantID), nil
}

func (s *Service) Update(tenantID string, update UpdateQuota) (tenant.Quota, error) {
	if tenantID == "" {
		return tenant.Quota{}, fmt.Errorf("tenant id is required")
	}
	if err := validateUpdate(update); err != nil {
		return tenant.Quota{}, err
	}
	return s.resources.Set(tenantID, update.MaxConcurrent, update.MaxCPUPerMinute, update.MaxMemoryPages), nil
}

func validateUpdate(update UpdateQuota) error {
	if update.MaxConcurrent < 1 {
		return fmt.Errorf("max concurrent must be positive")
	}
	if update.MaxCPUPerMinute <= 0 {
		return fmt.Errorf("CPU budget must be positive")
	}
	if update.MaxMemoryPages < 1 {
		return fmt.Errorf("memory budget must be positive")
	}
	return nil
}
