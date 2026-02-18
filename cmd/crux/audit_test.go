package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/security"
	"github.com/roygabriel/crux/pkg/types"
)

func TestParseAuditLog_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	entries := []security.AuditEntry{
		{
			Timestamp: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			AgentID:   "eng-1",
			Action:    "file_write",
			Target:    "/src/main.go",
			Allowed:   true,
		},
		{
			Timestamp: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
			AgentID:   "eng-2",
			Action:    "exec",
			Target:    "rm -rf /",
			Allowed:   false,
			Reason:    "destructive command",
		},
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		enc.Encode(e)
	}
	f.Close()

	parsed, err := parseAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed))
	}
	if string(parsed[0].AgentID) != "eng-1" {
		t.Errorf("entry 0 agent = %q, want %q", parsed[0].AgentID, "eng-1")
	}
	if parsed[1].Allowed {
		t.Error("entry 1 should be denied")
	}
}

func TestParseAuditLog_CorruptLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	content := `{"timestamp":"2025-06-15T10:00:00Z","agent_id":"eng-1","action":"file_write","target":"/src/main.go","permission":"standard","allowed":true}
this is not json
{"timestamp":"2025-06-15T11:00:00Z","agent_id":"eng-2","action":"exec","target":"ls","permission":"standard","allowed":true}
`
	os.WriteFile(path, []byte(content), 0o644)

	parsed, err := parseAuditLog(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 entries (corrupt skipped), got %d", len(parsed))
	}
}

func TestParseAuditLog_MissingFile(t *testing.T) {
	parsed, err := parseAuditLog("/nonexistent/audit.log")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil entries, got %d", len(parsed))
	}
}

func TestFilterAuditEntries(t *testing.T) {
	entries := []security.AuditEntry{
		{AgentID: "eng-1", Allowed: true},
		{AgentID: "eng-2", Allowed: false},
		{AgentID: "eng-1", Allowed: false},
		{AgentID: "eng-2", Allowed: true},
	}

	tests := []struct {
		name       string
		agent      string
		deniedOnly bool
		wantCount  int
	}{
		{"no filter", "", false, 4},
		{"agent filter", "eng-1", false, 2},
		{"denied only", "", true, 2},
		{"agent + denied", "eng-1", true, 1},
		{"unknown agent", "eng-99", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterAuditEntries(entries, tt.agent, tt.deniedOnly)
			if len(result) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestComputeAuditStats(t *testing.T) {
	entries := []security.AuditEntry{
		{AgentID: "eng-1", Action: "file_write", Allowed: true},
		{AgentID: "eng-1", Action: "exec", Allowed: true},
		{AgentID: "eng-2", Action: "file_write", Allowed: false},
		{AgentID: "eng-2", Action: "exec", Allowed: true},
	}

	stats := computeAuditStats(entries)

	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.Allowed != 3 {
		t.Errorf("Allowed = %d, want 3", stats.Allowed)
	}
	if stats.Denied != 1 {
		t.Errorf("Denied = %d, want 1", stats.Denied)
	}
	if stats.ByAgent["eng-1"] != 2 {
		t.Errorf("ByAgent[eng-1] = %d, want 2", stats.ByAgent["eng-1"])
	}
	if stats.ByAgent["eng-2"] != 2 {
		t.Errorf("ByAgent[eng-2] = %d, want 2", stats.ByAgent["eng-2"])
	}
	if stats.ByAction["file_write"] != 2 {
		t.Errorf("ByAction[file_write] = %d, want 2", stats.ByAction["file_write"])
	}
}

func TestComputeAuditStats_Empty(t *testing.T) {
	stats := computeAuditStats(nil)
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1h", 1 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"invalid", 0, true},
		{"xd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Verify AuditEntry types used match security package.
func TestAuditEntryTypes(t *testing.T) {
	entry := security.AuditEntry{
		Timestamp:  time.Now(),
		AgentID:    types.AgentID("test"),
		Action:     security.ActionFileWrite,
		Target:     "/test",
		Permission: types.PermStandard,
		Allowed:    true,
	}
	if string(entry.AgentID) != "test" {
		t.Error("AgentID mismatch")
	}
}
