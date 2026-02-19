package security

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func TestSecurityMiddleware_EmitsTaxonomyEventsAndLegacyEntry(t *testing.T) {
	root := t.TempDir()
	auditPath := filepath.Join(root, "audit.log")
	audit, err := NewAuditLogger(auditPath)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer audit.Close()

	sb, err := NewSandbox(root, nil, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	mw := NewSecurityMiddleware(NewEnforcer(sb, slog.Default()), audit, slog.Default())

	target := filepath.Join(root, "x.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := mw.Gate("agent-1", types.PermAutonomous, ActionFileWrite, target, "1A", 1); err != nil {
		t.Fatalf("Gate: %v", err)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := splitLines(string(data))
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 log lines, got %d", len(lines))
	}

	var hasLegacy, hasPermission, hasAttempt bool
	for _, ln := range lines {
		var raw map[string]any
		if err := json.Unmarshal([]byte(ln), &raw); err != nil {
			t.Fatalf("unmarshal line %q: %v", ln, err)
		}
		if et, ok := raw["event_type"].(string); ok {
			switch et {
			case string(AuditPermissionChecked):
				hasPermission = true
			case string(AuditActionAttempted):
				hasAttempt = true
			}
		}
		if _, ok := raw["reason"]; ok {
			hasLegacy = true
		}
	}
	if !hasLegacy {
		t.Fatal("expected legacy AuditEntry record")
	}
	if !hasPermission {
		t.Fatal("expected permission_checked event")
	}
	if !hasAttempt {
		t.Fatal("expected action_attempted event")
	}
}

func splitLines(s string) []string {
	var out []string
	var b []rune
	for _, r := range s {
		if r == '\n' {
			if len(b) > 0 {
				out = append(out, string(b))
				b = b[:0]
			}
			continue
		}
		b = append(b, r)
	}
	if len(b) > 0 {
		out = append(out, string(b))
	}
	return out
}
