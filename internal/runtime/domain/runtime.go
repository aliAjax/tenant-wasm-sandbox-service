package domain

import (
	"context"
	"errors"
	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	"time"
)

var (
	ErrInstructionLimit = errors.New("instruction budget exceeded")
	ErrMemoryLimit      = errors.New("memory page budget exceeded")
	ErrStackLimit       = errors.New("stack budget exceeded")
	ErrOutputLimit      = errors.New("output budget exceeded")
	ErrTrap             = errors.New("runtime trap")
)

type CompiledModule interface {
	Digest() string
	RuntimeVersion() string
}
type Result struct {
	Output    []byte
	ExitCode  int
	Usage     execution.Usage
	Truncated bool
}
type Runtime interface {
	Version() string
	Compile(context.Context, []byte, string) (CompiledModule, error)
	Execute(context.Context, CompiledModule, string, execution.Invocation, execution.Budget) (Result, error)
	Close(context.Context) error
}
type Health struct {
	Version   string    `json:"version"`
	Ready     bool      `json:"ready"`
	Active    int       `json:"active"`
	LastCheck time.Time `json:"last_check"`
}
