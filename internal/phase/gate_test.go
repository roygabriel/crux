package phase_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func newTestRunner(t *testing.T) *phase.GateRunner {
	t.Helper()
	return phase.NewGateRunner(t.TempDir(), 5*time.Second, slog.Default())
}

func TestGateRunner_PassingGate(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "true", Expected: "exit 0", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Passed {
		t.Error("expected gate to pass")
	}
}

func TestGateRunner_FailingGate(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "false", Expected: "exit 0", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail")
	}
	if !errors.Is(result.Error, types.ErrGateFailed) {
		t.Errorf("result.Error = %v, want ErrGateFailed", result.Error)
	}
}

func TestGateRunner_OutputPattern(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "echo hello", Expected: "hello", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Passed {
		t.Error("expected gate to pass when output contains pattern")
	}
}

func TestGateRunner_OutputNoMatch(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "echo hello", Expected: "world", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail when output does not contain pattern")
	}
}

func TestGateRunner_Timeout(t *testing.T) {
	runner := phase.NewGateRunner(t.TempDir(), 100*time.Millisecond, slog.Default())
	gate := phase.Gate{Command: "sleep 60", Expected: "exit 0", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Passed {
		t.Error("expected gate to fail on timeout")
	}
}

func TestGateRunner_HumanApproval(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Expected: "manual check", Type: phase.GateHumanApproval}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Passed {
		t.Error("human-approval gates should always pass")
	}
}

func TestGateRunner_RunAllStopsOnFailure(t *testing.T) {
	runner := newTestRunner(t)
	gates := []phase.Gate{
		{Command: "true", Expected: "exit 0", Type: phase.GateAutomated},
		{Command: "false", Expected: "exit 0", Type: phase.GateAutomated},
		{Command: "true", Expected: "exit 0", Type: phase.GateAutomated},
	}

	results, err := runner.RunAll(context.Background(), gates)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2 (stop at first failure)", len(results))
	}
	if results[0].Passed != true {
		t.Error("first gate should pass")
	}
	if results[1].Passed != false {
		t.Error("second gate should fail")
	}
}

func TestGateRunner_EmptyCommand(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "", Type: phase.GateAutomated}

	_, err := runner.Run(context.Background(), gate)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestGateRunner_RunAllAllPass(t *testing.T) {
	runner := newTestRunner(t)
	gates := []phase.Gate{
		{Command: "true", Expected: "exit 0", Type: phase.GateAutomated},
		{Command: "true", Expected: "exit 0", Type: phase.GateAutomated},
	}

	results, err := runner.RunAll(context.Background(), gates)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("results[%d].Passed = false, want true", i)
		}
	}
}

func TestGateRunner_Duration(t *testing.T) {
	runner := newTestRunner(t)
	gate := phase.Gate{Command: "true", Expected: "exit 0", Type: phase.GateAutomated}

	result, err := runner.Run(context.Background(), gate)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}
