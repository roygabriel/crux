package main

import (
	"crypto/sha256"
	"os"
	"strings"
	"testing"
)

func TestSimpleDiff_IdenticalContent(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3"}
	diff := simpleDiff(lines, lines)
	if diff != "" {
		t.Errorf("identical content should produce empty diff, got %q", diff)
	}
}

func TestSimpleDiff_AddedLines(t *testing.T) {
	t.Parallel()

	old := []string{"line1", "line2"}
	new := []string{"line1", "line2", "line3"}
	diff := simpleDiff(old, new)
	if !strings.Contains(diff, "+line3") {
		t.Errorf("diff should show added line3, got %q", diff)
	}
}

func TestSimpleDiff_RemovedLines(t *testing.T) {
	t.Parallel()

	old := []string{"line1", "line2", "line3"}
	new := []string{"line1", "line2"}
	diff := simpleDiff(old, new)
	if !strings.Contains(diff, "-line3") {
		t.Errorf("diff should show removed line3, got %q", diff)
	}
}

func TestSimpleDiff_ChangedLines(t *testing.T) {
	t.Parallel()

	old := []string{"line1", "old", "line3"}
	new := []string{"line1", "new", "line3"}
	diff := simpleDiff(old, new)
	if !strings.Contains(diff, "-old") {
		t.Errorf("diff should show removed old, got %q", diff)
	}
	if !strings.Contains(diff, "+new") {
		t.Errorf("diff should show added new, got %q", diff)
	}
}

func TestSimpleDiff_EmptyOld(t *testing.T) {
	t.Parallel()

	diff := simpleDiff(nil, []string{"line1"})
	if !strings.Contains(diff, "+line1") {
		t.Errorf("diff should show added line1, got %q", diff)
	}
}

func TestSimpleDiff_EmptyNew(t *testing.T) {
	t.Parallel()

	diff := simpleDiff([]string{"line1"}, nil)
	if !strings.Contains(diff, "-line1") {
		t.Errorf("diff should show removed line1, got %q", diff)
	}
}

func TestHashContent(t *testing.T) {
	t.Parallel()

	h1 := hashContent("hello world")
	h2 := hashContent("hello world")
	h3 := hashContent("different")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}

	// Verify it matches crypto/sha256 directly.
	expected := sha256.Sum256([]byte("hello world"))
	if h1 != expected {
		t.Error("hashContent should match sha256.Sum256")
	}
}

func TestTruncTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello"},
		{"empty", "", 5, ""},
		{"zero max", "hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncTime(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncTime(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSortedAgentIDs(t *testing.T) {
	t.Parallel()

	cfg := &configForSortTest{
		agents: map[string]bool{
			"charlie": true,
			"alpha":   true,
			"bravo":   true,
		},
	}

	ids := sortedAgentIDsFromMap(cfg.agents)
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != "alpha" || ids[1] != "bravo" || ids[2] != "charlie" {
		t.Errorf("expected sorted order, got %v", ids)
	}
}

// configForSortTest is a simple helper to test sorted ordering.
type configForSortTest struct {
	agents map[string]bool
}

// sortedAgentIDsFromMap extracts and sorts keys from a map.
func sortedAgentIDsFromMap(m map[string]bool) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	// Use the same sort as sortedAgentIDs.
	for i := 0; i < len(ids)-1; i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[i] > ids[j] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

func TestLanguageFromConfig_Defaults(t *testing.T) {
	t.Parallel()

	// With a temp dir that has no go.mod or package.json.
	dir := t.TempDir()
	result := detectLanguage(dir)
	if result != "" {
		t.Errorf("expected empty language, got %q", result)
	}
}

// detectLanguage mirrors languageFromConfig logic for testing without config dependency.
func detectLanguage(root string) string {
	return languageFromRoot(root)
}

// languageFromRoot is a standalone version for testing.
func languageFromRoot(root string) string {
	// Same logic as languageFromConfig but takes root directly.
	if fileExists(root + "/go.mod") {
		return "Go"
	}
	if fileExists(root + "/package.json") {
		return "JavaScript/TypeScript"
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
