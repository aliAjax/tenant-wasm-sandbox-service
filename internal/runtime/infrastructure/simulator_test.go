package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	execution "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	module "github.com/acme/wasm-sandbox-executor/internal/module/domain"
	runtime "github.com/acme/wasm-sandbox-executor/internal/runtime/domain"
)

func compileProgram(t *testing.T, simulator *Simulator, program Program) runtime.CompiledModule {
	t.Helper()
	content, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := simulator.Compile(context.Background(), content, module.Digest(content))
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestSimulatorEnforcesResourceBudgets(t *testing.T) {
	simulator := NewSimulator("sim/test")
	compiled := compileProgram(t, simulator, Program{Operation: "echo", InstructionCost: 200, MemoryPages: 4, StackBytes: 8192})
	base := execution.Budget{MaxInstructions: 1000, MaxMemoryPages: 10, MaxStackBytes: 16384, Timeout: time.Second, MaxOutputBytes: 100}
	tests := []struct {
		name   string
		budget execution.Budget
		want   error
	}{
		{"instructions", func() execution.Budget { v := base; v.MaxInstructions = 100; return v }(), runtime.ErrInstructionLimit},
		{"memory", func() execution.Budget { v := base; v.MaxMemoryPages = 2; return v }(), runtime.ErrMemoryLimit},
		{"stack", func() execution.Budget { v := base; v.MaxStackBytes = 1024; return v }(), runtime.ErrStackLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simulator.Execute(context.Background(), compiled, "run", execution.Invocation{Payload: []byte("input")}, tt.budget)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v want %v", err, tt.want)
			}
		})
	}
}

func TestSimulatorTruncatesOutput(t *testing.T) {
	simulator := NewSimulator("sim/test")
	compiled := compileProgram(t, simulator, Program{Operation: "upper", InstructionCost: 10, MemoryPages: 1, StackBytes: 1, OutputRepeat: 10})
	result, err := simulator.Execute(context.Background(), compiled, "run", execution.Invocation{Payload: []byte("abc")}, execution.Budget{MaxInstructions: 100, MaxMemoryPages: 10, MaxStackBytes: 100, Timeout: time.Second, MaxOutputBytes: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("expected truncated result")
	}
	if got, want := string(result.Output), "ABCABCA"; got != want {
		t.Fatalf("output=%q want=%q", got, want)
	}
}

func TestSimulatorHonorsCancellation(t *testing.T) {
	simulator := NewSimulator("sim/test")
	compiled := compileProgram(t, simulator, Program{Operation: "echo", InstructionCost: 10, MemoryPages: 1, StackBytes: 1, DelayMillis: 500})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := simulator.Execute(ctx, compiled, "run", execution.Invocation{}, execution.Budget{MaxInstructions: 100, MaxMemoryPages: 10, MaxStackBytes: 100, Timeout: time.Second, MaxOutputBytes: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want canceled", err)
	}
}
