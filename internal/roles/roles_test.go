package roles_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/roles"
)

func TestDefinition_KnownRoles(t *testing.T) {
	tests := []struct {
		role    string
		keyword string
	}{
		{"engineer", "implementation"},
		{"orchestrator", "coordination"},
		{"reviewer", "review"},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			def := roles.Definition(tt.role)
			if def == "" {
				t.Errorf("Definition(%q) returned empty string", tt.role)
			}
			if len(def) < 50 {
				t.Errorf("Definition(%q) too short: %d chars", tt.role, len(def))
			}
		})
	}
}

func TestDefinition_Unknown(t *testing.T) {
	def := roles.Definition("nonexistent")
	if def != "" {
		t.Errorf("Definition(\"nonexistent\") = %q, want empty", def)
	}
}

func TestDefinition_Empty(t *testing.T) {
	def := roles.Definition("")
	if def != "" {
		t.Errorf("Definition(\"\") = %q, want empty", def)
	}
}
