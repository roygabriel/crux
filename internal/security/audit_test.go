package security

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

func TestAuditLogger_WritesJSONL(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.log")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	entry := AuditEntry{
		AgentID:    "agent-1",
		Action:     ActionFileRead,
		Target:     "src/main.go",
		Permission: types.PermStandard,
		Allowed:    true,
		Reason:     "sandbox ok",
	}
	if err := al.Log(entry); err != nil {
		t.Fatal(err)
	}

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var decoded AuditEntry
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.AgentID != "agent-1" {
		t.Errorf("agent_id = %q, want %q", decoded.AgentID, "agent-1")
	}
	if decoded.Action != ActionFileRead {
		t.Errorf("action = %q, want %q", decoded.Action, ActionFileRead)
	}
}

func TestAuditLogger_AppendsMultiple(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.log")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	for i := 0; i < 3; i++ {
		if err := al.Log(AuditEntry{
			AgentID: types.AgentID("agent"),
			Action:  ActionShellExec,
			Target:  "go build",
			Allowed: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestAuditLogger_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.log")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = al.Log(AuditEntry{
				AgentID: "agent-x",
				Action:  ActionFileWrite,
				Target:  "file.go",
				Allowed: true,
			})
		}()
	}
	wg.Wait()

	lines := readLines(t, path)
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d: corrupted JSON: %v", i, err)
		}
	}
}

func TestAuditLogger_SetsTimestamp(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.log")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	before := time.Now().UTC()
	if err := al.Log(AuditEntry{AgentID: "a", Action: ActionFileRead}); err != nil {
		t.Fatal(err)
	}

	lines := readLines(t, path)
	var entry AuditEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if entry.Timestamp.Before(before) {
		t.Error("timestamp is before test start")
	}
}

func TestAuditLogger_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewAuditLogger("/nonexistent/dir/audit.log")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}
