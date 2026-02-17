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
		var e AuditEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSON in audit log: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}
