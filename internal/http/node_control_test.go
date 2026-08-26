package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	workerapp "github.com/acme/wasm-sandbox-executor/internal/worker/application"
	worker "github.com/acme/wasm-sandbox-executor/internal/worker/domain"
)

func nodeControlHandler(nodes *workerapp.NodeRegistry) http.Handler {
	metrics := NewMetrics(func() int { return 0 }, func() int { return 0 })
	return NewRouter(Dependencies{Nodes: nodes, Metrics: metrics}).Handler()
}

func TestDrainNodeErrorResponseIsSingleJSON(t *testing.T) {
	handler := nodeControlHandler(workerapp.NewNodeRegistry("node-http", 4))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/nodes/missing/drain", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	decoder := json.NewDecoder(recorder.Body)
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	var second json.RawMessage
	if err := decoder.Decode(&second); !errors.Is(err, io.EOF) {
		t.Fatalf("response contains a second JSON value: %s", second)
	}
}

func TestIsolateNodeHonorsRequestCancellation(t *testing.T) {
	nodes := workerapp.NewNodeRegistry("node-http", 4)
	handler := nodeControlHandler(nodes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/v1/nodes/node-http/isolate", bytes.NewBufferString(`{"reason":"maintenance"}`)).WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if state := nodes.State().State; state != worker.NodeReady {
		t.Fatalf("state = %s, want ready", state)
	}
}
