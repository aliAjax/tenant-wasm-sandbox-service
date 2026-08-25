package store

import (
	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

// cloneModule returns a deep copy of m so callers cannot mutate the snapshot
// stored inside Memory through the slice headers or pointer fields that a
// struct-value copy would share. Every Module handed in or out of the store
// must pass through here.
func cloneModule(m moddomain.Module) moddomain.Module {
	cp := m
	if m.Capabilities != nil {
		cp.Capabilities = make([]moddomain.Capability, len(m.Capabilities))
		copy(cp.Capabilities, m.Capabilities)
	}
	if m.EnvAllowlist != nil {
		cp.EnvAllowlist = append([]string(nil), m.EnvAllowlist...)
	}
	if m.ImportList != nil {
		cp.ImportList = append([]string(nil), m.ImportList...)
	}
	if m.PublishedAt != nil {
		t := *m.PublishedAt
		cp.PublishedAt = &t
	}
	if m.WithdrawnAt != nil {
		t := *m.WithdrawnAt
		cp.WithdrawnAt = &t
	}
	return cp
}

// cloneExecution returns a deep copy of e for the same reason as cloneModule.
func cloneExecution(e execdomain.Execution) execdomain.Execution {
	cp := e
	if e.Output != nil {
		cp.Output = append([]byte(nil), e.Output...)
	}
	if e.StartedAt != nil {
		t := *e.StartedAt
		cp.StartedAt = &t
	}
	if e.FinishedAt != nil {
		t := *e.FinishedAt
		cp.FinishedAt = &t
	}
	return cp
}
