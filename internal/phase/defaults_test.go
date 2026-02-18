package phase_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/phase"
)

func TestDefaultConstraints_Count(t *testing.T) {
	constraints := phase.DefaultConstraints()
	if len(constraints) != 3 {
		t.Errorf("len(DefaultConstraints()) = %d, want 3", len(constraints))
	}
}

func TestDefaultConstraints_Immutable(t *testing.T) {
	first := phase.DefaultConstraints()
	first[0] = "mutated"

	second := phase.DefaultConstraints()
	if second[0] == "mutated" {
		t.Error("DefaultConstraints returned a mutable reference to the internal slice")
	}
}

func TestPermissionDescription_KnownTiers(t *testing.T) {
	tests := []struct {
		perm string
	}{
		{"readonly"},
		{"standard"},
		{"elevated"},
		{"autonomous"},
	}

	for _, tt := range tests {
		t.Run(tt.perm, func(t *testing.T) {
			desc := phase.PermissionDescription(tt.perm)
			if desc == "" {
				t.Errorf("PermissionDescription(%q) returned empty string", tt.perm)
			}
		})
	}
}

func TestPermissionDescription_Unknown(t *testing.T) {
	desc := phase.PermissionDescription("unknown")
	if desc != "" {
		t.Errorf("PermissionDescription(\"unknown\") = %q, want empty", desc)
	}
}
