package adapter

import (
	"time"

	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
)

type AuditRecord struct {
	ExecutionID   string               `json:"execution_id"`
	TenantID      string               `json:"tenant_id"`
	ModuleID      string               `json:"module_id"`
	ModuleVersion string               `json:"module_version"`
	ModuleDigest  string               `json:"module_digest"`
	RequestDigest string               `json:"request_digest"`
	OutputDigest  string               `json:"output_digest"`
	Status        execution.Status     `json:"status"`
	ErrorClass    execution.ErrorClass `json:"error_class"`
	Usage         execution.Usage      `json:"usage"`
	NodeID        string               `json:"node_id"`
	CreatedAt     time.Time            `json:"created_at"`
	FinishedAt    *time.Time           `json:"finished_at,omitempty"`
}

func ToAuditRecord(e execution.Execution) AuditRecord {
	return AuditRecord{
		ExecutionID:   e.ID,
		TenantID:      e.TenantID,
		ModuleID:      e.ModuleID,
		ModuleVersion: e.ModuleVersion,
		ModuleDigest:  e.ModuleDigest,
		RequestDigest: e.RequestDigest,
		OutputDigest:  e.OutputDigest,
		Status:        e.Status,
		ErrorClass:    e.ErrorClass,
		Usage:         e.Usage,
		NodeID:        e.NodeID,
		CreatedAt:     e.CreatedAt,
		FinishedAt:    e.FinishedAt,
	}
}
