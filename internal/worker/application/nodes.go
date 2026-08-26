package application

import (
	"context"
	"fmt"
	worker "github.com/acme/wasm-sandbox-executor/internal/worker/domain"
	"sync"
	"time"
)

type NodeRegistry struct {
	mu      sync.RWMutex
	nodes   map[string]worker.Node
	localID string
}

func NewNodeRegistry(localID string, capacity int) *NodeRegistry {
	now := time.Now().UTC()
	return &NodeRegistry{nodes: map[string]worker.Node{localID: {ID: localID, State: worker.NodeReady, Capacity: capacity, StartedAt: now, UpdatedAt: now}}, localID: localID}
}
func (r *NodeRegistry) Get(id string) (worker.Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.nodes[id]
	if !ok {
		return worker.Node{}, fmt.Errorf("node %s not found", id)
	}
	return v, nil
}
func (r *NodeRegistry) List() []worker.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]worker.Node, 0, len(r.nodes))
	for _, v := range r.nodes {
		out = append(out, v)
	}
	return out
}
func (r *NodeRegistry) Drain(ctx context.Context, id string) (worker.Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.nodes[id]
	if !ok {
		return worker.Node{}, fmt.Errorf("node %s not found", id)
	}
	if v.State == worker.NodeStopped {
		return worker.Node{}, worker.ErrNodeUnavailable
	}
	v.State = worker.NodeDraining
	v.Reason = "operator requested drain"
	v.UpdatedAt = time.Now().UTC()
	r.nodes[id] = v
	return v, nil
}
func (r *NodeRegistry) Isolate(ctx context.Context, id, reason string) (worker.Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.nodes[id]
	if !ok {
		return worker.Node{}, fmt.Errorf("node %s not found", id)
	}
	v.State = worker.NodeIsolated
	v.Reason = reason
	v.UpdatedAt = time.Now().UTC()
	r.nodes[id] = v
	return v, nil
}
func (r *NodeRegistry) Ready() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := r.nodes[r.localID].State
	return state != worker.NodeIsolated && state != worker.NodeStopped
}
func (r *NodeRegistry) State() worker.Node {
	return func() worker.Node { r.mu.RLock(); defer r.mu.RUnlock(); return r.nodes[r.localID] }()
}
func (r *NodeRegistry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.nodes[r.localID]
	v.State = worker.NodeStopped
	v.UpdatedAt = time.Now().UTC()
	r.nodes[r.localID] = v
}
