package scaffold

import (
	"io/fs"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	data := DefaultConfig()
	if len(data) == 0 {
		t.Fatal("DefaultConfig() returned empty bytes")
	}

	content := string(data)
	if got := content; len(got) == 0 {
		t.Fatal("DefaultConfig() content is empty")
	}

	// Verify it contains expected YAML keys.
	for _, want := range []string{"project:", "agents:", "memory:", "phases:", "security:"} {
		if !contains(content, want) {
			t.Errorf("DefaultConfig() missing expected key %q", want)
		}
	}
}

func TestTemplatesFS(t *testing.T) {
	t.Parallel()

	fsys, err := TemplatesFS()
	if err != nil {
		t.Fatalf("TemplatesFS() error: %v", err)
	}

	wantFiles := []string{
		"phase-spec.md",
		"phase-prompt.md",
		"work-notes.md",
	}

	for _, name := range wantFiles {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Errorf("TemplatesFS() missing file %q: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("TemplatesFS() file %q is empty", name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
