package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request-id"

var requestSequence atomic.Uint64

func requestID(ctx context.Context) string { v, _ := ctx.Value(requestIDKey).(string); return v }

type Middleware struct {
	logger  *slog.Logger
	apiKey  string
	timeout time.Duration
	maxBody int64
	limiter *limiter
	metrics *Metrics
}

func NewMiddleware(logger *slog.Logger, apiKey string, timeout time.Duration, maxBody int64, rate int, metrics *Metrics) *Middleware {
	return &Middleware{logger: logger, apiKey: apiKey, timeout: timeout, maxBody: maxBody, limiter: newLimiter(rate), metrics: metrics}
}
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	h := m.recover(next)
	h = m.timeoutRequest(h)
	h = m.limitBody(h)
	h = m.authenticate(h)
	h = m.rateLimit(h)
	h = m.logging(h)
	h = m.identify(h)
	return h
}
func (m *Middleware) identify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("req-%d-%d", time.Now().UnixMilli(), requestSequence.Add(1))
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, e := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, e
}
func (m *Middleware) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		m.metrics.Request()
		next.ServeHTTP(sw, r)
		if sw.status >= 400 {
			m.metrics.Error()
		}
		m.logger.Info("http request", "request_id", requestID(r.Context()), "method", r.Method, "path", r.URL.Path, "status", sw.status, "bytes", sw.bytes, "duration", time.Since(started))
	})
}
func (m *Middleware) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/console") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(m.apiKey)) != 1 {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", fmt.Errorf("valid bearer token required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, m.maxBody)
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) timeoutRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (m *Middleware) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				m.logger.Error("panic recovered", "request_id", requestID(r.Context()), "panic", recovered, "stack", string(debug.Stack()))
				writeError(w, r, http.StatusInternalServerError, "internal_error", fmt.Errorf("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.limiter.Allow() {
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", fmt.Errorf("request rate exceeded"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type limiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64
	last     time.Time
}

func newLimiter(rate int) *limiter {
	if rate < 1 {
		rate = 1
	}
	return &limiter{tokens: float64(rate), capacity: float64(rate), rate: float64(rate), last: time.Now()}
}
func (l *limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
