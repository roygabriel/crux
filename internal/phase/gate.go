package phase

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// GateResult captures the outcome of running a single verification gate.
type GateResult struct {
	// Gate is the gate that was executed.
	Gate Gate `json:"gate"`
	// Passed indicates whether the gate succeeded.
	Passed bool `json:"passed"`
	// Output is the combined stdout/stderr from the command.
	Output string `json:"output,omitempty"`
	// Duration is how long the gate took to execute.
	Duration time.Duration `json:"duration"`
	// Error is set for gate failures or infrastructure problems.
	Error error `json:"error,omitempty"`
}

// GateRunner executes verification gates as shell commands.
type GateRunner struct {
	workDir string
	timeout time.Duration
	logger  *slog.Logger
}

// NewGateRunner creates a GateRunner that executes commands in workDir
// with the given default timeout per gate.
func NewGateRunner(workDir string, timeout time.Duration, logger *slog.Logger) *GateRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &GateRunner{
		workDir: workDir,
		timeout: timeout,
		logger:  logger,
	}
}

// Run executes a single gate and returns the result.
// Human-approval gates always pass. Automated gates run the command
// and check the exit code and optional output pattern.
func (g *GateRunner) Run(ctx context.Context, gate Gate) (*GateResult, error) {
	if gate.Type == GateHumanApproval {
		return &GateResult{Gate: gate, Passed: true}, nil
	}

	if gate.Command == "" {
		return nil, fmt.Errorf("gate has empty command: %w", types.ErrGateFailed)
	}

	argv := strings.Fields(gate.Command)
	if len(argv) == 0 {
		return nil, fmt.Errorf("gate command parsed to empty argv: %w", types.ErrGateFailed)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = g.workDir

	out, err := cmd.CombinedOutput()
	duration := time.Since(start)
	output := string(out)

	result := &GateResult{
		Gate:     gate,
		Output:   output,
		Duration: duration,
	}

	if ctx.Err() != nil {
		result.Passed = false
		result.Error = fmt.Errorf("gate %q timed out: %w", gate.Command, types.ErrGateFailed)
		g.logger.Warn("gate timed out", "command", gate.Command, "timeout", g.timeout)
		return result, nil
	}

	if err != nil {
		result.Passed = false
		result.Error = fmt.Errorf("gate %q failed: %w", gate.Command, types.ErrGateFailed)
		g.logger.Info("gate failed", "command", gate.Command, "error", err)
		return result, nil
	}

	// Check output pattern if Expected is not just "exit 0" or empty.
	expected := strings.TrimSpace(gate.Expected)
	if expected != "" && expected != "exit 0" {
		if !strings.Contains(output, expected) {
			result.Passed = false
			result.Error = fmt.Errorf("gate %q output does not contain %q: %w", gate.Command, expected, types.ErrGateFailed)
			return result, nil
		}
	}

	result.Passed = true
	return result, nil
}

// RunAll executes gates sequentially, stopping at the first failure.
// It returns all results up to and including the first failure.
func (g *GateRunner) RunAll(ctx context.Context, gates []Gate) ([]GateResult, error) {
	var results []GateResult

	for _, gate := range gates {
		result, err := g.Run(ctx, gate)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
		if !result.Passed {
			return results, nil
		}
	}

	return results, nil
}
