package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	moduleapp "github.com/acme/wasm-sandbox-executor/internal/module/application"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type missingModuleRepo struct{}

func (missingModuleRepo) Create(context.Context, moddomain.Module, []byte) error { return nil }
func (missingModuleRepo) Update(context.Context, moddomain.Module) error         { return nil }
func (missingModuleRepo) Get(context.Context, string) (moddomain.Module, error) {
	return moddomain.Module{}, moddomain.ErrNotFound
}
func (missingModuleRepo) Content(context.Context, string) ([]byte, error) {
	return nil, moddomain.ErrNotFound
}
func (missingModuleRepo) List(context.Context, string) ([]moddomain.Module, error) { return nil, nil }
func (missingModuleRepo) FindVersion(context.Context, string, string, string) (moddomain.Module, error) {
	return moddomain.Module{}, moddomain.ErrNotFound
}

type allowVerifier struct{}

func (allowVerifier) Verify(context.Context, string, []byte, []byte) error { return nil }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Unix(100, 0) }

type fixedIDs struct{}

func (fixedIDs) NewID(string) string { return "mod-score" }

func TestMissingModuleHTTPStatus(t *testing.T) {
	modules := moduleapp.New(missingModuleRepo{}, allowVerifier{}, fixedClock{}, fixedIDs{}, 1024)
	metrics := NewMetrics(func() int { return 0 }, func() int { return 0 })
	handler := NewRouter(Dependencies{Modules: modules, Metrics: metrics}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/v1/modules/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
}
