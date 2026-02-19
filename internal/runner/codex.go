package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultCodexBinary = "codex"
)

// CodexExecRunner executes prompts using "codex exec --json -" and parses
// newline-delimited JSON events into a deterministic result.
type CodexExecRunner struct {
	binary string
	logger *slog.Logger
}

// NewCodexExecRunner constructs a Codex deterministic runner using the default
// codex binary from PATH.
func NewCodexExecRunner(logger *slog.Logger) *CodexExecRunner {
	return NewCodexExecRunnerWithBinary(defaultCodexBinary, logger)
}

// NewCodexExecRunnerWithBinary constructs a Codex deterministic runner with
// an explicit binary path (primarily for tests).
func NewCodexExecRunnerWithBinary(binary string, logger *slog.Logger) *CodexExecRunner {
	if strings.TrimSpace(binary) == "" {
		binary = defaultCodexBinary
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CodexExecRunner{
		binary: binary,
		logger: logger,
	}
}

// Name returns the deterministic runner name.
func (r *CodexExecRunner) Name() string {
	return "codex-exec"
}

// Run executes the request prompt via Codex in non-interactive JSON mode.
func (r *CodexExecRunner) Run(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.WorkDir) == "" {
		return Result{}, fmt.Errorf("codex runner: workdir is required")
	}

	if req.RunnerMode == "" {
		req.RunnerMode = "deterministic"
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	timeoutCancel := func() {}
	if req.Timeout > 0 {
		runCtx, timeoutCancel = context.WithTimeout(runCtx, req.Timeout)
	}
	defer timeoutCancel()

	started := time.Now().UTC()
	cmd := exec.CommandContext(runCtx, r.binary, "exec", "--json", "-")
	cmd.Dir = req.WorkDir
	cmd.Stdin = strings.NewReader(req.Prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("codex runner: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("codex runner: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("codex runner: start: %w", err)
	}

	var rawLines []string
	var events []Event
	var agentMessages []string
	var linesMu sync.Mutex
	lastEventUnix := started.UnixNano()
	terminationReason := "completed"
	var termMu sync.Mutex
	setTermination := func(v string) {
		termMu.Lock()
		terminationReason = v
		termMu.Unlock()
	}
	getTermination := func() string {
		termMu.Lock()
		defer termMu.Unlock()
		return terminationReason
	}
	idleCancelled := atomic.Bool{}

	var wg sync.WaitGroup
	readStdout := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			atomic.StoreInt64(&lastEventUnix, time.Now().UTC().UnixNano())
			linesMu.Lock()
			rawLines = append(rawLines, line)
			linesMu.Unlock()

			var payload map[string]any
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				continue
			}

			evtType, _ := payload["type"].(string)
			evtTS := time.Now().UTC()
			if rawTS, ok := payload["timestamp"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339Nano, rawTS); err == nil {
					evtTS = parsed.UTC()
				}
			}

			linesMu.Lock()
			events = append(events, Event{
				Type:      evtType,
				Timestamp: evtTS,
				Raw:       line,
			})

			if evtType == "item.completed" {
				item, _ := payload["item"].(map[string]any)
				if itemType, _ := item["type"].(string); itemType == "agent_message" {
					if text, _ := item["text"].(string); text != "" {
						agentMessages = append(agentMessages, text)
					}
				}
			}
			linesMu.Unlock()
		}
	}

	var stderrBuf bytes.Buffer
	readStderr := func(reader io.Reader) {
		defer wg.Done()
		_, _ = io.Copy(&stderrBuf, reader)
	}

	wg.Add(2)
	go readStdout(stdout)
	go readStderr(stderr)

	if req.IdleTimeout > 0 {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					last := time.Unix(0, atomic.LoadInt64(&lastEventUnix))
					if time.Since(last) >= req.IdleTimeout {
						if idleCancelled.CompareAndSwap(false, true) {
							setTermination("idle_timeout")
							cancel()
						}
						return
					}
				}
			}
		}()
	}

	waitErr := cmd.Wait()
	wg.Wait()

	if waitErr != nil {
		switch {
		case idleCancelled.Load():
			setTermination("idle_timeout")
		case req.Timeout > 0 && errors.Is(runCtx.Err(), context.DeadlineExceeded):
			setTermination("timeout")
		case errors.Is(runCtx.Err(), context.Canceled):
			setTermination("cancelled")
		default:
			setTermination("error")
		}
	}

	finished := time.Now().UTC()
	lastEventAt := time.Unix(0, atomic.LoadInt64(&lastEventUnix)).UTC()

	result := Result{
		StartedAt:         started,
		FinishedAt:        finished,
		Duration:          finished.Sub(started),
		RawOutput:         strings.TrimSpace(strings.Join(rawLines, "\n")),
		Output:            strings.TrimSpace(strings.Join(agentMessages, "\n")),
		Events:            append([]Event(nil), events...),
		LastEventAt:       lastEventAt,
		TerminationReason: getTermination(),
		ExitCode:          commandExitCode(waitErr),
	}
	if strings.TrimSpace(result.Output) == "" {
		result.Output = strings.TrimSpace(result.RawOutput)
	}

	if waitErr != nil {
		combined := strings.TrimSpace(stderrBuf.String())
		if combined == "" {
			combined = strings.TrimSpace(result.RawOutput)
		}
		if combined != "" {
			waitErr = fmt.Errorf("%w: %s", waitErr, combined)
		}
		r.logger.Warn("deterministic codex run failed",
			"agent_id", req.AgentID,
			"run_id", req.RunID,
			"exit_code", result.ExitCode,
			"termination_reason", result.TerminationReason,
			"error", waitErr,
		)
		return result, waitErr
	}

	r.logger.Info("deterministic codex run completed",
		"agent_id", req.AgentID,
		"run_id", req.RunID,
		"duration", result.Duration.Round(time.Second),
		"events", len(result.Events),
	)
	return result, nil
}

func commandExitCode(runErr error) int {
	if runErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
