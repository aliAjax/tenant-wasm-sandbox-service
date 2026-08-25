package application

import (
	"context"
	"fmt"
	"time"

	"github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type Service struct {
	repo     domain.Repository
	verifier domain.SignatureVerifier
	clock    domain.Clock
	ids      domain.IDGenerator
	maxSize  int64
}

func New(repo domain.Repository, verifier domain.SignatureVerifier, clock domain.Clock, ids domain.IDGenerator, maxSize int64) *Service {
	return &Service{repo: repo, verifier: verifier, clock: clock, ids: ids, maxSize: maxSize}
}
func (s *Service) Register(ctx context.Context, in domain.RegisterRequest, signature []byte) (domain.Module, error) {
	if err := s.verifier.Verify(ctx, in.TenantID, in.Content, signature); err != nil {
		return domain.Module{}, fmt.Errorf("verify module signature: %w", err)
	}
	m, err := domain.New(s.ids.NewID("mod"), in, s.maxSize, s.clock.Now())
	if err != nil {
		return domain.Module{}, err
	}
	if err = s.repo.Create(ctx, m, in.Content); err != nil {
		return domain.Module{}, fmt.Errorf("store module: %w", err)
	}
	return m, nil
}
func (s *Service) Publish(ctx context.Context, id string) (domain.Module, error) {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Module{}, err
	}
	m, err = m.Publish(s.clock.Now())
	if err != nil {
		return domain.Module{}, err
	}
	if err = s.repo.Update(ctx, m); err != nil {
		return domain.Module{}, fmt.Errorf("publish module: %w", err)
	}
	return m, nil
}
func (s *Service) Withdraw(ctx context.Context, id string) (domain.Module, error) {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Module{}, err
	}
	m, err = m.Withdraw(s.clock.Now())
	if err != nil {
		return domain.Module{}, err
	}
	if err = s.repo.Update(ctx, m); err != nil {
		return domain.Module{}, fmt.Errorf("withdraw module: %w", err)
	}
	return m, nil
}
func (s *Service) Get(ctx context.Context, id string) (domain.Module, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, tenant string) ([]domain.Module, error) {
	return s.repo.List(ctx, tenant)
}
func (s *Service) Content(ctx context.Context, id string) ([]byte, error) {
	return s.repo.Content(ctx, id)
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
