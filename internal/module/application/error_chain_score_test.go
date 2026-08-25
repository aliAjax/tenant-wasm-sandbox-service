package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type scoreRepo struct {
	module domain.Module
}

func (r scoreRepo) Create(context.Context, domain.Module, []byte) error { return nil }
func (r scoreRepo) Update(context.Context, domain.Module) error         { return nil }
func (r scoreRepo) Get(context.Context, string) (domain.Module, error) {
	if r.module.ID == "" {
		return domain.Module{}, domain.ErrNotFound
	}
	return r.module, nil
}
func (r scoreRepo) Content(context.Context, string) ([]byte, error) {
	return nil, domain.ErrNotFound
}
func (r scoreRepo) List(context.Context, string) ([]domain.Module, error) { return nil, nil }
func (r scoreRepo) FindVersion(context.Context, string, string, string) (domain.Module, error) {
	return domain.Module{}, domain.ErrNotFound
}

type scoreVerifier struct{ err error }

func (v scoreVerifier) Verify(context.Context, string, []byte, []byte) error { return v.err }

type scoreClock struct{}

func (scoreClock) Now() time.Time { return time.Unix(100, 0) }

type scoreIDs struct{}

func (scoreIDs) NewID(string) string { return "mod-score" }

func TestRegisterPreservesSignatureCause(t *testing.T) {
	cause := errors.New("signing service unavailable")
	service := New(scoreRepo{}, scoreVerifier{err: cause}, scoreClock{}, scoreIDs{}, 1024)
	_, err := service.Register(context.Background(), domain.RegisterRequest{TenantID: "tenant-a", Content: []byte("module")}, nil)
	if !errors.Is(err, cause) {
		t.Fatalf("signature cause was not preserved: %v", err)
	}
}

func TestContentPreservesNotFoundChain(t *testing.T) {
	service := New(scoreRepo{}, scoreVerifier{}, scoreClock{}, scoreIDs{}, 1024)
	_, err := service.Content(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("content lookup lost ErrNotFound: %v", err)
	}
}

func TestPublishPreservesInvalidTransitionChain(t *testing.T) {
	service := New(scoreRepo{module: domain.Module{ID: "mod-1", State: domain.StatePublished}}, scoreVerifier{}, scoreClock{}, scoreIDs{}, 1024)
	_, err := service.Publish(context.Background(), "mod-1")
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("publish transition lost ErrInvalidTransition: %v", err)
	}
}
