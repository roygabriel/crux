package security

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func newTestMiddleware(t *testing.T) (*SecurityMiddleware, string, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")

	auditPath := filepath.Join(t.TempDir(), "audit.log")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	enforcer := NewEnforcer(sb, nil)
	audit, err := NewAuditLogger(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { audit.Close() })

	mw := NewSecurityMiddleware(enforcer, audit, nil)
	return mw, root, auditPath
}

func TestGate_AllowedAction(t *testing.T) {
	t.Parallel()
	mw, root, auditPath := newTestMiddleware(t)

	err := mw.Gate("agent-1", types.PermElevated, ActionFileWrite,
		filepath.Join(root, "src", "main.go"), "1A", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if !entries[0].Allowed {
		t.Error("audit entry should show allowed=true")
	}
}

func TestGate_DeniedAction(t *testing.T) {
	t.Parallel()
	mw, root, auditPath := newTestMiddleware(t)

	err := mw.Gate("agent-2", types.PermReadonly, ActionFileWrite,
		filepath.Join(root, "src", "main.go"), "1A", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Allowed {
		t.Error("audit entry should show allowed=false")
	}
}

func TestGate_ShellAllowed(t *testing.T) {
	t.Parallel()
	mw, _, auditPath := newTestMiddleware(t)

	err := mw.Gate("agent-1", types.PermStandard, ActionShellExec,
		"go build ./...", "1A", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 || !entries[0].Allowed {
		t.Error("expected allowed audit entry")
	}
}

func TestGate_ShellDenied(t *testing.T) {
	t.Parallel()
	mw, _, auditPath := newTestMiddleware(t)

	err := mw.Gate("agent-1", types.PermStandard, ActionShellExec,
		"curl http://evil.com", "1A", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 || entries[0].Allowed {
		t.Error("expected denied audit entry")
	}
}

func TestGate_FailClosed(t *testing.T) {
	t.Parallel()
	mw, root, _ := newTestMiddleware(t)

	// Path outside sandbox should fail closed.
	err := mw.Gate("agent-1", types.PermStandard, ActionFileWrite,
		filepath.Join(root, "..", "..", "etc", "passwd"), "1A", 1)
	if err == nil {
		t.Fatal("expected error for sandbox violation")
	}
}

func TestGate_AuditFailureDoesNotBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")

	// Create audit logger, then close it so writes fail.
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	audit, err := NewAuditLogger(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	audit.Close()

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	enforcer := NewEnforcer(sb, nil)
	mw := NewSecurityMiddleware(enforcer, audit, nil)

	// File read should be allowed even though audit log fails.
	err = mw.Gate("agent-1", types.PermStandard, ActionFileRead,
		filepath.Join(root, "src", "main.go"), "", 0)
	if err != nil {
		t.Errorf("expected nil (audit failure should not block), got %v", err)
	}
}

func TestGate_PopulatesPhasePrompt(t *testing.T) {
	t.Parallel()
	mw, root, auditPath := newTestMiddleware(t)

	err := mw.Gate("agent-3", types.PermElevated, ActionFileRead,
		filepath.Join(root, "src", "main.go"), "2B", 3)
	if err != nil {
		t.Fatal(err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].PhaseID != "2B" {
		t.Errorf("phase_id = %q, want %q", entries[0].PhaseID, "2B")
	}
	if entries[0].PromptNum != 3 {
		t.Errorf("prompt_num = %d, want 3", entries[0].PromptNum)
	}
}

func TestGate_RateLimitedShellExec(t *testing.T) {
	t.Parallel()
	mw, _, auditPath := newTestMiddleware(t)

	rl := NewRateLimiter(1, 0, nil)
	mw.SetRateLimiter(rl)

	// First shell_exec should succeed.
	err := mw.Gate("agent-1", types.PermStandard, ActionShellExec,
		"go build ./...", "1A", 1)
	if err != nil {
		t.Fatalf("first exec: %v", err)
	}

	// Second shell_exec should be rate-limited.
	err = mw.Gate("agent-1", types.PermStandard, ActionShellExec,
		"go test ./...", "1A", 2)
	if !errors.Is(err, types.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	// Should have 2 entries: allowed + denied.
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}
	if entries[1].Allowed {
		t.Error("second entry should be denied")
	}
	if entries[1].Reason != "rate_limited" {
		t.Errorf("reason = %q, want %q", entries[1].Reason, "rate_limited")
	}
}

func TestGate_RateLimitedFileWrite(t *testing.T) {
	t.Parallel()
	mw, root, auditPath := newTestMiddleware(t)

	rl := NewRateLimiter(0, 1, nil)
	mw.SetRateLimiter(rl)

	target1 := filepath.Join(root, "src", "main.go")
	err := mw.Gate("agent-1", types.PermElevated, ActionFileWrite,
		target1, "1A", 1)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Create a second file so sandbox passes.
	writeFile(t, filepath.Join(root, "src", "other.go"), "package main")
	target2 := filepath.Join(root, "src", "other.go")

	err = mw.Gate("agent-1", types.PermElevated, ActionFileWrite,
		target2, "1A", 2)
	if !errors.Is(err, types.ErrFileLimit) {
		t.Errorf("expected ErrFileLimit, got %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}
	if entries[1].Reason != "file_limit" {
		t.Errorf("reason = %q, want %q", entries[1].Reason, "file_limit")
	}
}

func TestGate_NoRateLimiterPassthrough(t *testing.T) {
	t.Parallel()
	mw, _, _ := newTestMiddleware(t)

	// No rate limiter set — should work as before.
	for i := 0; i < 5; i++ {
		err := mw.Gate("agent-1", types.PermStandard, ActionShellExec,
			"go build ./...", "1A", i+1)
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
	}
}

func TestGateDispatch_AllowsControlPlaneDispatch(t *testing.T) {
	t.Parallel()
	mw, _, auditPath := newTestMiddleware(t)

	if err := mw.GateDispatch("agent-1", types.PermStandard, "task"); err != nil {
		t.Fatalf("GateDispatch: %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 legacy audit entry, got %d", len(entries))
	}
	if entries[0].Action != ActionMessageSend {
		t.Fatalf("action = %q, want %q", entries[0].Action, ActionMessageSend)
	}
	if !entries[0].Allowed {
		t.Fatal("expected allowed control-plane dispatch")
	}
}

func TestGateMessage_UnknownAction_FailOpenWithAuditSignal(t *testing.T) {
	t.Parallel()
	mw, _, auditPath := newTestMiddleware(t)

	if err := mw.GateMessage("agent-1", types.PermStandard, "message.send.v2", "task"); err != nil {
		t.Fatalf("GateMessage: %v", err)
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 legacy audit entries, got %d", len(entries))
	}
	foundFailOpen := false
	for _, e := range entries {
		if e.Reason == "fail-open control-plane action normalization" && e.Allowed {
			foundFailOpen = true
			break
		}
	}
	if !foundFailOpen {
		t.Fatal("expected fail-open legacy audit marker")
	}

	events := readAuditEventMaps(t, auditPath)
	foundPolicy := false
	for _, ev := range events {
		md, ok := ev["metadata"].(map[string]any)
		if !ok {
			continue
		}
		if md["policy"] == "fail_open_control_plane" && md["reason"] == "unknown_action_type" {
			foundPolicy = true
			break
		}
	}
	if !foundPolicy {
		t.Fatal("expected fail_open_control_plane metadata in audit events")
	}
}

func TestGateString_UnknownAction_FailClosed(t *testing.T) {
	t.Parallel()
	mw, _, _ := newTestMiddleware(t)

	err := mw.GateString("agent-1", types.PermAutonomous, "definitely_unknown_action", "task", "", 0)
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func readAuditEntries(t *testing.T, path string) []AuditEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("invalid JSON in audit log: %v", err)
		}
		if _, isEvent := raw["event_type"]; isEvent {
			continue
		}
		var e AuditEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSON in audit log: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func readAuditEventMaps(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var out []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("invalid JSON in audit log: %v", err)
		}
		if _, isEvent := raw["event_type"]; isEvent {
			out = append(out, raw)
		}
	}
	return out
}
