package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	cache "github.com/acme/wasm-sandbox-executor/internal/cache/domain"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/application"
	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	module "github.com/acme/wasm-sandbox-executor/internal/module/application"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
	resource "github.com/acme/wasm-sandbox-executor/internal/resource/application"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
	worker "github.com/acme/wasm-sandbox-executor/internal/worker/application"
)

type Router struct {
	modules    *module.Service
	executions *execution.Service
	resources  *resource.Manager
	cache      cache.Cache
	nodes      *worker.NodeRegistry
	runtime    runtime.Runtime
	metrics    *Metrics
	started    time.Time
	maxModule  int64
}
type Dependencies struct {
	Modules        *module.Service
	Executions     *execution.Service
	Resources      *resource.Manager
	Cache          cache.Cache
	Nodes          *worker.NodeRegistry
	Runtime        runtime.Runtime
	Metrics        *Metrics
	MaxModuleBytes int64
}

func NewRouter(d Dependencies) *Router {
	return &Router{modules: d.Modules, executions: d.Executions, resources: d.Resources, cache: d.Cache, nodes: d.Nodes, runtime: d.Runtime, metrics: d.Metrics, started: time.Now().UTC(), maxModule: d.MaxModuleBytes}
}
func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", r.consoleRedirect)
	mux.Handle("GET /console", r.consoleHandler())
	mux.Handle("GET /console/", r.consoleHandler())
	mux.HandleFunc("GET /healthz", r.health)
	mux.HandleFunc("GET /readyz", r.ready)
	mux.Handle("GET /metrics", r.metrics)
	mux.HandleFunc("POST /v1/modules", r.registerModule)
	mux.HandleFunc("GET /v1/modules", r.listModules)
	mux.HandleFunc("GET /v1/modules/{id}", r.getModule)
	mux.HandleFunc("POST /v1/modules/{id}/publish", r.publishModule)
	mux.HandleFunc("POST /v1/modules/{id}/withdraw", r.withdrawModule)
	mux.HandleFunc("POST /v1/executions", r.submitExecution)
	mux.HandleFunc("GET /v1/executions", r.listExecutions)
	mux.HandleFunc("GET /v1/executions/{id}", r.getExecution)
	mux.HandleFunc("POST /v1/executions/{id}/cancel", r.cancelExecution)
	mux.HandleFunc("GET /v1/tenants/{id}/quota", r.getQuota)
	mux.HandleFunc("PUT /v1/tenants/{id}/quota", r.setQuota)
	mux.HandleFunc("GET /v1/cache", r.cacheStats)
	mux.HandleFunc("POST /v1/cache/recalculate", r.recalculateCache)
	mux.HandleFunc("GET /v1/nodes", r.listNodes)
	mux.HandleFunc("POST /v1/nodes/{id}/drain", r.drainNode)
	mux.HandleFunc("POST /v1/nodes/{id}/isolate", r.isolateNode)
	mux.HandleFunc("GET /v1/sandbox/health", r.sandboxHealth)
	return mux
}

func (r *Router) health(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "uptime": time.Since(r.started).String()})
}
func (r *Router) ready(w http.ResponseWriter, req *http.Request) {
	if !r.nodes.Ready() {
		writeError(w, req, http.StatusServiceUnavailable, "not_ready", fmt.Errorf("local node is not accepting work"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "node": r.nodes.State().ID})
}
func (r *Router) registerModule(w http.ResponseWriter, req *http.Request) {
	var body struct {
		TenantID        string                 `json:"tenant_id"`
		Name            string                 `json:"name"`
		Version         string                 `json:"version"`
		Digest          string                 `json:"digest"`
		Entry           string                 `json:"entry"`
		ABI             string                 `json:"abi"`
		Runtime         string                 `json:"runtime"`
		Capabilities    []moddomain.Capability `json:"capabilities"`
		EnvAllowlist    []string               `json:"env_allowlist"`
		ImportAllowlist []string               `json:"import_allowlist"`
		ObjectKey       string                 `json:"object_key"`
		Content         []byte                 `json:"content"`
		Signature       []byte                 `json:"signature"`
	}
	if err := decode(req, &body); err != nil {
		writeError(w, req, http.StatusBadRequest, "invalid_json", err)
		return
	}
	m, err := r.modules.Register(req.Context(), moddomain.RegisterRequest{TenantID: body.TenantID, Name: body.Name, Version: body.Version, Digest: body.Digest, Entry: body.Entry, ABI: body.ABI, Runtime: body.Runtime, Capabilities: body.Capabilities, EnvAllowlist: body.EnvAllowlist, ImportList: body.ImportAllowlist, ObjectKey: body.ObjectKey, Content: body.Content}, body.Signature)
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}
func (r *Router) listModules(w http.ResponseWriter, req *http.Request) {
	items, err := r.modules.List(req.Context(), req.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, req, http.StatusInternalServerError, "storage_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}
func (r *Router) getModule(w http.ResponseWriter, req *http.Request) {
	m, err := r.modules.Get(req.Context(), req.PathValue("id"))
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (r *Router) publishModule(w http.ResponseWriter, req *http.Request) {
	m, err := r.modules.Publish(req.Context(), req.PathValue("id"))
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (r *Router) withdrawModule(w http.ResponseWriter, req *http.Request) {
	m, err := r.modules.Withdraw(req.Context(), req.PathValue("id"))
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	r.cache.InvalidateTenant(req.Context(), m.TenantID)
	writeJSON(w, http.StatusOK, m)
}
func (r *Router) submitExecution(w http.ResponseWriter, req *http.Request) {
	if !r.nodes.Ready() {
		writeError(w, req, http.StatusServiceUnavailable, "node_draining", fmt.Errorf("node is not accepting executions"))
		return
	}
	var in execdomain.Request
	if err := decode(req, &in); err != nil {
		writeError(w, req, http.StatusBadRequest, "invalid_json", err)
		return
	}
	e, err := r.executions.Submit(req.Context(), in)
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	r.metrics.Execution()
	writeJSON(w, http.StatusAccepted, e)
}
func (r *Router) getExecution(w http.ResponseWriter, req *http.Request) {
	e, err := r.executions.Get(req.Context(), req.PathValue("id"))
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}
func (r *Router) listExecutions(w http.ResponseWriter, req *http.Request) {
	limit := 50
	if raw := req.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	items, err := r.executions.List(req.Context(), req.URL.Query().Get("tenant_id"), limit)
	if err != nil {
		writeError(w, req, http.StatusInternalServerError, "storage_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}
func (r *Router) cancelExecution(w http.ResponseWriter, req *http.Request) {
	e, err := r.executions.Cancel(req.Context(), req.PathValue("id"))
	if err != nil {
		status, code := classify(err)
		writeError(w, req, status, code, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}
func (r *Router) getQuota(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, r.resources.Get(req.PathValue("id")))
}
func (r *Router) setQuota(w http.ResponseWriter, req *http.Request) {
	var in struct {
		MaxConcurrent   int           `json:"max_concurrent"`
		MaxCPUPerMinute time.Duration `json:"max_cpu_per_minute"`
		MaxMemoryPages  uint32        `json:"max_memory_pages"`
	}
	if err := decode(req, &in); err != nil {
		writeError(w, req, http.StatusBadRequest, "invalid_json", err)
		return
	}
	if in.MaxConcurrent < 1 || in.MaxCPUPerMinute <= 0 || in.MaxMemoryPages < 1 {
		writeError(w, req, http.StatusUnprocessableEntity, "invalid_quota", fmt.Errorf("all quota values must be positive"))
		return
	}
	writeJSON(w, http.StatusOK, r.resources.Set(req.PathValue("id"), in.MaxConcurrent, in.MaxCPUPerMinute, in.MaxMemoryPages))
}
func (r *Router) cacheStats(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, r.cache.Stats())
}
func (r *Router) recalculateCache(w http.ResponseWriter, req *http.Request) {
	var in struct {
		TenantID string `json:"tenant_id"`
	}
	if err := decode(req, &in); err != nil {
		writeError(w, req, http.StatusBadRequest, "invalid_json", err)
		return
	}
	if in.TenantID == "" {
		writeError(w, req, http.StatusUnprocessableEntity, "tenant_required", fmt.Errorf("tenant_id required"))
		return
	}
	n := r.cache.InvalidateTenant(req.Context(), in.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{"invalidated": n, "tenant_id": in.TenantID})
}
func (r *Router) listNodes(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": r.nodes.List()})
}
func (r *Router) drainNode(w http.ResponseWriter, req *http.Request) {
	node, err := r.nodes.Drain(req.Context(), req.PathValue("id"))
	if err != nil {
		writeError(w, req, http.StatusBadRequest, "drain_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}
func (r *Router) isolateNode(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := decode(req, &in); err != nil {
		writeError(w, req, http.StatusBadRequest, "invalid_json", err)
		return
	}
	node, err := r.nodes.Isolate(req.Context(), req.PathValue("id"), in.Reason)
	if err != nil {
		writeError(w, req, http.StatusBadRequest, "isolation_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}
func (r *Router) sandboxHealth(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, runtime.Health{Version: r.runtime.Version(), Ready: r.nodes.Ready(), Active: r.resources.Active(), LastCheck: time.Now().UTC()})
}
func decode(req *http.Request, dst any) error {
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON value")
	}
	return nil
}
