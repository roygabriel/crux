package security

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

func TestEmitPermissionChecked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer logger.Close()

	ev := AuditEvent{
		ID:            NewEventID(),
		InteractionID: "int-1",
		Action:        "file_write",
		Target:        "main.go",
		AgentID:       "engineer-1",
		PhaseID:       "1A",
		PromptNum:     1,
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
	}
	if err := EmitPermissionChecked(context.Background(), logger, ev); err != nil {
		t.Fatalf("EmitPermissionChecked: %v", err)
	}

	lines := readAuditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}
	if got := lines[0]["event_type"]; got != string(AuditPermissionChecked) {
		t.Fatalf("event_type = %v, want %q", got, AuditPermissionChecked)
	}
	if got := lines[0]["interaction_id"]; got != "int-1" {
		t.Fatalf("interaction_id = %v, want int-1", got)
	}
}

func TestEmitEffectConfirmed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer logger.Close()

	ev := AuditEvent{
		ID:            NewEventID(),
		InteractionID: "int-2",
		Action:        "file_write",
		Target:        "go.mod",
		AgentID:       "engineer-1",
		PhaseID:       "1A",
		PromptNum:     1,
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
		Metadata:      map[string]string{"exists": "true"},
	}
	if err := EmitEffectConfirmed(context.Background(), logger, ev); err != nil {
		t.Fatalf("EmitEffectConfirmed: %v", err)
	}

	lines := readAuditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}
	if got := lines[0]["event_type"]; got != string(AuditEffectConfirmed) {
		t.Fatalf("event_type = %v, want %q", got, AuditEffectConfirmed)
	}
	meta, ok := lines[0]["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata should be object, got %T", lines[0]["metadata"])
	}
	if got := meta["exists"]; got != "true" {
		t.Fatalf("metadata.exists = %v, want true", got)
	}
}

func TestInteractionIDCorrelation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer logger.Close()

	interactionID := NewInteractionID("engineer-1", "file_write", "go.mod", "1A", 1)
	base := AuditEvent{
		InteractionID: interactionID,
		Action:        "file_write",
		Target:        "go.mod",
		AgentID:       "engineer-1",
		PhaseID:       "1A",
		PromptNum:     1,
		Allowed:       true,
	}

	p := base
	p.ID = NewEventID()
	if err := EmitPermissionChecked(context.Background(), logger, p); err != nil {
		t.Fatalf("EmitPermissionChecked: %v", err)
	}

	e := base
	e.ID = NewEventID()
	if err := EmitEffectConfirmed(context.Background(), logger, e); err != nil {
		t.Fatalf("EmitEffectConfirmed: %v", err)
	}

	lines := readAuditLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}
	if lines[0]["interaction_id"] != interactionID || lines[1]["interaction_id"] != interactionID {
		t.Fatalf("interaction IDs mismatch: %v", lines)
	}
}

func TestEventRequiredFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer logger.Close()

	ev := AuditEvent{
		ID:            NewEventID(),
		InteractionID: NewInteractionID("engineer-1", "shell_exec", "go test ./...", "1A", 1),
		Action:        "shell_exec",
		Target:        "go test ./...",
		AgentID:       "engineer-1",
		PhaseID:       "1A",
		PromptNum:     1,
		Allowed:       true,
	}
	if err := EmitActionAttempted(context.Background(), logger, ev); err != nil {
		t.Fatalf("EmitActionAttempted: %v", err)
	}
	lines := readAuditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}
	got := lines[0]
	required := []string{"id", "interaction_id", "event_type", "action", "target", "agent_id", "phase_id", "prompt_num", "allowed", "timestamp"}
	for _, k := range required {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing required field %q", k)
		}
	}
}

func readAuditLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []map[string]any
	for _, line := range splitNonEmpty(string(data)) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func splitNonEmpty(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func TestNewInteractionIDStable(t *testing.T) {
	a := NewInteractionID("agent-1", "file_write", "x.go", "1A", 1)
	b := NewInteractionID(types.AgentID("agent-1"), "file_write", "x.go", "1A", 1)
	if a != b {
		t.Fatalf("interaction IDs should match: %q != %q", a, b)
	}
}
