package httpapi

import (
	"encoding/json"
	"errors"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	module "github.com/acme/wasm-sandbox-executor/internal/module/domain"
	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	"net/http"
)

type errorBody struct {
	Error     errorDetail `json:"error"`
	RequestID string      `json:"request_id,omitempty"`
}
type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: err.Error()}, RequestID: requestID(r.Context())})
}
func classify(err error) (int, string) {
	switch {
	case err == module.ErrNotFound, err == execution.ErrNotFound:
		return http.StatusNotFound, "not_found"
	case errors.Is(err, module.ErrInvalidDigest), errors.Is(err, module.ErrIncompatibleABI), errors.Is(err, module.ErrInvalidTransition):
		return http.StatusUnprocessableEntity, "invalid_module"
	case errors.Is(err, resource.ErrQuotaExceeded):
		return http.StatusTooManyRequests, "quota_exceeded"
	default:
		return http.StatusBadRequest, "invalid_request"
	}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
