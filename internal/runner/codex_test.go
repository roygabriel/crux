package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexExecRunnerRunSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := writeMockCodex(t, dir, mockCodexScript(0))
	r := NewCodexExecRunnerWithBinary(bin, slog.Default())

	req := Request{
		RunID:   "run-1",
		AgentID: "engineer-1",
		Prompt:  "implement feature",
		WorkDir: dir,
	}
	got, err := r.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if !strings.Contains(got.Output, "All done.") {
		t.Fatalf("Output = %q, want agent message", got.Output)
	}
	if len(got.Events) == 0 {
		t.Fatal("expected parsed events")
	}
	if strings.TrimSpace(got.RawOutput) == "" {
		t.Fatal("expected raw output")
	}
	if got.TerminationReason != "completed" {
		t.Fatalf("TerminationReason = %q, want completed", got.TerminationReason)
	}
}

func TestCodexExecRunnerRunFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := writeMockCodex(t, dir, mockCodexScript(7))
	r := NewCodexExecRunnerWithBinary(bin, slog.Default())

	req := Request{
		RunID:   "run-2",
		AgentID: "engineer-1",
		Prompt:  "implement feature",
		WorkDir: dir,
	}
	got, err := r.Run(context.Background(), req)
	if err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
	if got.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", got.ExitCode)
	}
	if !strings.Contains(err.Error(), "mock runner failure") {
		t.Fatalf("error = %q, want mock runner failure", err.Error())
	}
}

func TestCodexExecRunnerRunTimeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := writeMockCodex(t, dir, timeoutCodexScript())
	r := NewCodexExecRunnerWithBinary(bin, slog.Default())

	req := Request{
		RunID:       "run-timeout",
		AgentID:     "engineer-1",
		Prompt:      "do work",
		WorkDir:     dir,
		Timeout:     250 * time.Millisecond,
		IdleTimeout: 10 * time.Second,
	}
	got, err := r.Run(context.Background(), req)
	if err == nil {
		t.Fatal("Run() error = nil, want timeout")
	}
	if got.TerminationReason != "timeout" {
		t.Fatalf("TerminationReason = %q, want timeout", got.TerminationReason)
	}
}

func TestCodexExecRunnerIdleTimeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bin := writeMockCodex(t, dir, idleTimeoutCodexScript())
	r := NewCodexExecRunnerWithBinary(bin, slog.Default())

	req := Request{
		RunID:       "run-idle-timeout",
		AgentID:     "engineer-1",
		Prompt:      "do work",
		WorkDir:     dir,
		Timeout:     5 * time.Second,
		IdleTimeout: 300 * time.Millisecond,
	}
	got, err := r.Run(context.Background(), req)
	if err == nil {
		t.Fatal("Run() error = nil, want idle timeout")
	}
	if got.TerminationReason != "idle_timeout" {
		t.Fatalf("TerminationReason = %q, want idle_timeout", got.TerminationReason)
	}
}

func writeMockCodex(t *testing.T, dir, body string) string {
	t.Helper()
	name := "mock-codex"
	if runtime.GOOS == "windows" {
		name += ".bat"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write mock codex: %v", err)
	}
	return path
}

func mockCodexScript(exitCode int) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
echo '{"type":"thread.started","thread_id":"thread-1"}'
echo '{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"All done."}}'
if [ %d -ne 0 ]; then
  echo 'mock runner failure' 1>&2
  exit %d
fi
`, exitCode, exitCode)
}

func timeoutCodexScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
sleep 3
`
}

func idleTimeoutCodexScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
echo '{"type":"thread.started","thread_id":"thread-1"}'
sleep 3
`
}
