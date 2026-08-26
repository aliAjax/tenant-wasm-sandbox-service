package httpapi

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type Metrics struct {
	requests   atomic.Uint64
	errors     atomic.Uint64
	executions atomic.Uint64
	started    time.Time
	queueLen   func() int
	active     func() int
}

func NewMetrics(queueLen, active func() int) *Metrics {
	return &Metrics{started: time.Now(), queueLen: queueLen, active: active}
}
func (m *Metrics) Request()   { m.requests.Add(1) }
func (m *Metrics) Error()     { m.errors.Add(1) }
func (m *Metrics) Execution() { m.executions.Add(1) }
func (m *Metrics) Observe(status int) {
	m.requests.Add(1)
	if status == http.StatusInternalServerError {
		m.errors.Add(1)
	}
}
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP wasm_http_requests_total Total HTTP requests.\n# TYPE wasm_http_requests_total counter\nwasm_http_requests_total %d\n", m.requests.Load())
	_, _ = fmt.Fprintf(w, "# HELP wasm_http_errors_total Total HTTP errors.\n# TYPE wasm_http_errors_total counter\nwasm_http_errors_total %d\n", m.errors.Load())
	_, _ = fmt.Fprintf(w, "# HELP wasm_executions_submitted_total Submitted executions.\n# TYPE wasm_executions_submitted_total counter\nwasm_executions_submitted_total %d\n", m.executions.Load())
	_, _ = fmt.Fprintf(w, "# TYPE wasm_queue_depth gauge\nwasm_queue_depth %d\n", m.queueLen())
	_, _ = fmt.Fprintf(w, "# TYPE wasm_active_leases gauge\nwasm_active_leases %d\n", m.active())
	_, _ = fmt.Fprintf(w, "# TYPE wasm_process_uptime_seconds gauge\nwasm_process_uptime_seconds %.0f\n", time.Since(m.started).Seconds())
}
