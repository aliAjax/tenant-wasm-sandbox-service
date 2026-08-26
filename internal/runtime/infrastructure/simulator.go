package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
)

type Simulator struct {
	version string
	active  atomic.Int64
	closed  atomic.Bool
}
type compiled struct {
	digest  string
	version string
	program Program
}

func (c compiled) Digest() string         { return c.digest }
func (c compiled) RuntimeVersion() string { return c.version }

type Program struct {
	Operation       string `json:"operation"`
	InstructionCost uint64 `json:"instruction_cost"`
	MemoryPages     uint32 `json:"memory_pages"`
	StackBytes      uint64 `json:"stack_bytes"`
	DelayMillis     int    `json:"delay_millis"`
	OutputRepeat    int    `json:"output_repeat"`
	ExitCode        int    `json:"exit_code"`
}

func NewSimulator(version string) *Simulator {
	if version == "" {
		version = "sim/v1"
	}
	return &Simulator{version: version}
}
func (s *Simulator) Version() string { return s.version }
func (s *Simulator) Compile(ctx context.Context, content []byte, digest string) (runtime.CompiledModule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed.Load() {
		return nil, fmt.Errorf("runtime closed")
	}
	actual := sha256.Sum256(content)
	actualDigest := "sha256:" + hex.EncodeToString(actual[:])
	if digest != "" && digest != actualDigest {
		return nil, fmt.Errorf("compile digest mismatch")
	}
	var p Program
	if err := json.Unmarshal(content, &p); err != nil {
		return nil, fmt.Errorf("decode simulated wasm manifest: %w", err)
	}
	if p.Operation == "" {
		p.Operation = "echo"
	}
	if p.InstructionCost == 0 {
		p.InstructionCost = 100
	}
	if p.MemoryPages == 0 {
		p.MemoryPages = 1
	}
	if p.StackBytes == 0 {
		p.StackBytes = 4096
	}
	if p.OutputRepeat < 1 {
		p.OutputRepeat = 1
	}
	switch p.Operation {
	case "echo", "upper", "sum", "hash", "reverse", "fail":
	default:
		return nil, fmt.Errorf("unsupported simulated operation %q", p.Operation)
	}
	return compiled{digest: actualDigest, version: s.version, program: p}, nil
}

func (s *Simulator) Execute(ctx context.Context, module runtime.CompiledModule, entry string, in execution.Invocation, budget execution.Budget) (runtime.Result, error) {
	if s.closed.Load() {
		return runtime.Result{}, fmt.Errorf("runtime closed")
	}
	m := module.(compiled)
	if entry == "" {
		return runtime.Result{}, fmt.Errorf("entry is required")
	}
	p := m.program
	if p.InstructionCost > budget.MaxInstructions {
		return runtime.Result{Usage: execution.Usage{Instructions: p.InstructionCost}}, runtime.ErrInstructionLimit
	}
	if p.MemoryPages > budget.MaxMemoryPages {
		return runtime.Result{Usage: execution.Usage{MemoryPages: p.MemoryPages}}, runtime.ErrMemoryLimit
	}
	if p.StackBytes > budget.MaxStackBytes {
		return runtime.Result{}, runtime.ErrStackLimit
	}
	s.active.Add(1)
	defer func() {
		if ctx.Err() == nil {
			s.active.Add(-1)
		}
	}()
	started := time.Now()
	if p.DelayMillis > 0 {
		timer := time.NewTimer(time.Duration(p.DelayMillis) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return runtime.Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	var out []byte
	var err error
	switch p.Operation {
	case "echo":
		out = append([]byte(nil), in.Payload...)
	case "upper":
		out = []byte(strings.ToUpper(string(in.Payload)))
	case "reverse":
		r := []rune(string(in.Payload))
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		out = []byte(string(r))
	case "hash":
		sum := sha256.Sum256(in.Payload)
		out = []byte(hex.EncodeToString(sum[:]))
	case "sum":
		out, err = sumJSON(in.Payload)
	case "fail":
		return runtime.Result{ExitCode: max(p.ExitCode, 1), Usage: execution.Usage{Instructions: p.InstructionCost, MemoryPages: p.MemoryPages, CPUTime: time.Since(started)}}, runtime.ErrTrap
	}
	if err != nil {
		return runtime.Result{}, fmt.Errorf("execute operation: %w", err)
	}
	out = bytes.Repeat(out, p.OutputRepeat)
	truncated := false
	if len(out) > budget.MaxOutputBytes {
		out = out[:budget.MaxOutputBytes]
		truncated = true
	}
	return runtime.Result{Output: out, ExitCode: p.ExitCode, Truncated: truncated, Usage: execution.Usage{Instructions: p.InstructionCost, MemoryPages: p.MemoryPages, CPUTime: time.Since(started), OutputBytes: len(out)}}, nil
}
func sumJSON(payload []byte) ([]byte, error) {
	var values []float64
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, err
	}
	var total float64
	for _, v := range values {
		total += v
	}
	return []byte(strconv.FormatFloat(total, 'f', -1, 64)), nil
}
func (s *Simulator) Close(context.Context) error {
	s.closed.Store(true)
	s.active.Store(0)
	return nil
}
func (s *Simulator) Active() int { return int(s.active.Load()) }
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
