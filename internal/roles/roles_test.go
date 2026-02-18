package roles_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/instruct"
	"github.com/roygabriel/crux/internal/roles"
)

func TestDefinition_AllRoles(t *testing.T) {
	t.Parallel()

	names := []string{
		"planner",
		"project-manager",
		"software-engineer",
		"systems-engineer",
		"code-reviewer",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			def := roles.Definition(name)
			if def == "" {
				t.Errorf("Definition(%q) returned empty string", name)
			}
			if len(def) < 200 {
				t.Errorf("Definition(%q) too short: %d chars", name, len(def))
			}
		})
	}
}

func TestDefinition_Unknown(t *testing.T) {
	t.Parallel()

	def := roles.Definition("nonexistent")
	if def != "" {
		t.Errorf("Definition(\"nonexistent\") = %q, want empty", def)
	}
}

func TestDefinition_Empty(t *testing.T) {
	t.Parallel()

	def := roles.Definition("")
	if def != "" {
		t.Errorf("Definition(\"\") = %q, want empty", def)
	}
}

func TestBuildRoleContext_AllRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		roleName     instruct.RoleName
		wantTitle    string
		wantPlanning bool
		wantReview   bool
	}{
		{instruct.RolePlanner, "Planner", true, false},
		{instruct.RoleProjectManager, "Project Manager", false, true},
		{instruct.RoleSoftwareEngineer, "Software Engineer", false, false},
		{instruct.RoleSystemsEngineer, "Systems Engineer", false, false},
		{instruct.RoleCodeReviewer, "Code Reviewer", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.roleName), func(t *testing.T) {
			t.Parallel()

			rc, err := roles.BuildRoleContext(tt.roleName)
			if err != nil {
				t.Fatalf("BuildRoleContext(%q) error: %v", tt.roleName, err)
			}

			if rc.Name != tt.roleName {
				t.Errorf("Name = %q, want %q", rc.Name, tt.roleName)
			}
			if rc.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", rc.Title, tt.wantTitle)
			}
			if rc.Identity == "" {
				t.Error("Identity is empty")
			}
			if len(rc.Responsibilities) == 0 {
				t.Error("Responsibilities is empty")
			}
			if len(rc.Constraints) == 0 {
				t.Error("Constraints is empty")
			}
			if len(rc.Communication) == 0 {
				t.Error("Communication is empty")
			}

			if tt.wantPlanning && len(rc.PlanningRules) == 0 {
				t.Error("PlanningRules is empty, expected non-empty")
			}
			if !tt.wantPlanning && len(rc.PlanningRules) != 0 {
				t.Errorf("PlanningRules should be empty, got %d items", len(rc.PlanningRules))
			}

			if tt.wantReview && len(rc.ReviewFocus) == 0 {
				t.Error("ReviewFocus is empty, expected non-empty")
			}
			if !tt.wantReview && len(rc.ReviewFocus) != 0 {
				t.Errorf("ReviewFocus should be empty, got %d items", len(rc.ReviewFocus))
			}
		})
	}
}

func TestBuildRoleContext_UnknownRole(t *testing.T) {
	t.Parallel()

	_, err := roles.BuildRoleContext("nonexistent")
	if err == nil {
		t.Fatal("BuildRoleContext(\"nonexistent\") expected error, got nil")
	}
}

func TestNormalizeRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"engineer", "software-engineer"},
		{"reviewer", "code-reviewer"},
		{"orchestrator", "orchestrator"},
		{"planner", "planner"},
		{"project-manager", "project-manager"},
		{"software-engineer", "software-engineer"},
		{"systems-engineer", "systems-engineer"},
		{"code-reviewer", "code-reviewer"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := roles.NormalizeRole(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeRole(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildRoleContext_BackwardCompat(t *testing.T) {
	t.Parallel()

	// "engineer" should resolve to software-engineer.
	rc, err := roles.BuildRoleContext("engineer")
	if err != nil {
		t.Fatalf("BuildRoleContext(\"engineer\") error: %v", err)
	}
	if rc.Name != instruct.RoleSoftwareEngineer {
		t.Errorf("Name = %q, want %q", rc.Name, instruct.RoleSoftwareEngineer)
	}
	if rc.Title != "Software Engineer" {
		t.Errorf("Title = %q, want %q", rc.Title, "Software Engineer")
	}

	// "reviewer" should resolve to code-reviewer.
	rc, err = roles.BuildRoleContext("reviewer")
	if err != nil {
		t.Fatalf("BuildRoleContext(\"reviewer\") error: %v", err)
	}
	if rc.Name != instruct.RoleCodeReviewer {
		t.Errorf("Name = %q, want %q", rc.Name, instruct.RoleCodeReviewer)
	}
	if rc.Title != "Code Reviewer" {
		t.Errorf("Title = %q, want %q", rc.Title, "Code Reviewer")
	}
}

func TestParseRoleContext_IdentityContent(t *testing.T) {
	t.Parallel()

	content := roles.Definition("planner")
	rc := roles.ParseRoleContext(instruct.RolePlanner, content)

	if rc.Identity == "" {
		t.Fatal("Identity is empty")
	}
	if !containsStr(rc.Identity, "You are") {
		t.Errorf("Identity does not contain 'You are': %q", rc.Identity)
	}
}

func TestParseRoleContext_EmptyContent(t *testing.T) {
	t.Parallel()

	rc := roles.ParseRoleContext("empty", "")

	if rc.Name != "empty" {
		t.Errorf("Name = %q, want %q", rc.Name, "empty")
	}
	if rc.Title != "" {
		t.Errorf("Title = %q, want empty", rc.Title)
	}
	if rc.Identity != "" {
		t.Errorf("Identity = %q, want empty", rc.Identity)
	}
	if len(rc.Responsibilities) != 0 {
		t.Errorf("Responsibilities should be empty, got %d", len(rc.Responsibilities))
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
