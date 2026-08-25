package domain

import (
	"errors"
	"fmt"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
	StatusTimedOut  Status = "timed_out"
)

type ErrorClass string

const (
	ErrorNone        ErrorClass = "none"
	ErrorInvalid     ErrorClass = "invalid_request"
	ErrorResource    ErrorClass = "resource_exhausted"
	ErrorRuntime     ErrorClass = "runtime_error"
	ErrorCanceled    ErrorClass = "canceled"
	ErrorTimeout     ErrorClass = "timeout"
	ErrorUnavailable ErrorClass = "unavailable"
)

var ErrNotFound = errors.New("execution not found")

type InputKind string

const (
	InputJSON   InputKind = "json"
	InputBinary InputKind = "binary"
	InputStream InputKind = "stream"
)

type Budget struct {
	MaxInstructions uint64        `json:"max_instructions"`
	MaxMemoryPages  uint32        `json:"max_memory_pages"`
	MaxStackBytes   uint64        `json:"max_stack_bytes"`
	Timeout         time.Duration `json:"timeout"`
	MaxOutputBytes  int           `json:"max_output_bytes"`
}

func DefaultBudget() Budget {
	return Budget{MaxInstructions: 100000, MaxMemoryPages: 128, MaxStackBytes: 1 << 20, Timeout: 3 * time.Second, MaxOutputBytes: 64 << 10}
}
func (b Budget) Normalize(max Budget) Budget {
	if b.MaxInstructions == 0 {
		b.MaxInstructions = max.MaxInstructions
	}
	if b.MaxMemoryPages == 0 {
		b.MaxMemoryPages = max.MaxMemoryPages
	}
	if b.MaxStackBytes == 0 {
		b.MaxStackBytes = max.MaxStackBytes
	}
	if b.Timeout == 0 {
		b.Timeout = max.Timeout
	}
	if b.MaxOutputBytes == 0 {
		b.MaxOutputBytes = max.MaxOutputBytes
	}
	return b
}
func (b Budget) Validate(max Budget) error {
	if b.MaxInstructions > max.MaxInstructions {
		return fmt.Errorf("max instructions exceeds service limit")
	}
	if b.MaxMemoryPages > max.MaxMemoryPages {
		return fmt.Errorf("max memory pages exceeds service limit")
	}
	if b.MaxStackBytes > max.MaxStackBytes {
		return fmt.Errorf("max stack exceeds service limit")
	}
	if b.Timeout > max.Timeout {
		return fmt.Errorf("timeout exceeds service limit")
	}
	if b.MaxOutputBytes > max.MaxOutputBytes {
		return fmt.Errorf("output limit exceeds service limit")
	}
	return nil
}

type Invocation struct {
	Kind        InputKind         `json:"kind"`
	Payload     []byte            `json:"payload"`
	Arguments   []string          `json:"arguments"`
	Environment map[string]string `json:"environment"`
}
type Request struct {
	TenantID       string     `json:"tenant_id"`
	ModuleID       string     `json:"module_id"`
	Invocation     Invocation `json:"invocation"`
	Budget         Budget     `json:"budget"`
	CallbackURL    string     `json:"callback_url,omitempty"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
}
type Usage struct {
	Instructions uint64        `json:"instructions"`
	MemoryPages  uint32        `json:"memory_pages"`
	CPUTime      time.Duration `json:"cpu_time"`
	OutputBytes  int           `json:"output_bytes"`
}
type Execution struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ModuleID        string     `json:"module_id"`
	ModuleVersion   string     `json:"module_version"`
	ModuleDigest    string     `json:"module_digest"`
	RequestDigest   string     `json:"request_digest"`
	Status          Status     `json:"status"`
	Budget          Budget     `json:"budget"`
	Usage           Usage      `json:"usage"`
	Output          []byte     `json:"output,omitempty"`
	OutputDigest    string     `json:"output_digest,omitempty"`
	OutputTruncated bool       `json:"output_truncated"`
	ExitCode        int        `json:"exit_code"`
	ErrorClass      ErrorClass `json:"error_class"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CallbackURL     string     `json:"callback_url,omitempty"`
	NodeID          string     `json:"node_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

func (e Execution) Terminal() bool {
	switch e.Status {
	case StatusSucceeded, StatusFailed, StatusCanceled:
		return true
	}
	return false
}
