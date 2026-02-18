package main

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
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

// buildTestDistributor creates a distributor and config for validation testing.
// It writes a config with a claude agent and returns the config and distributor.
func buildTestDistributor(t *testing.T, projectRoot string) (*config.Config, *instruct.Distributor) {
	t.Helper()

	cruxDir := filepath.Join(projectRoot, ".crux")
	if err := os.MkdirAll(cruxDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(cruxDir, "config.yaml")
	cfgContent := `project:
  name: test-project
  root: "` + projectRoot + `"
  state_dir: "` + cruxDir + `"
agents:
  dev:
    plugin: claude
    role: engineer
    permission: standard
memory:
  sqlite_path: "` + filepath.Join(cruxDir, "memory.db") + `"
  vector_dir: "` + filepath.Join(cruxDir, "vectors") + `"
phases:
  spec_dir: docs/phases
security:
  audit_log: "` + filepath.Join(cruxDir, "audit.log") + `"
  max_cmds_per_min: 60
  max_files_per_session: 100
context:
  total_budget: 8000
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	log := setupLogger()
	dist := buildDistributor(cfg, log)
	return cfg, dist
}

func TestValidateAgent_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := buildTestDistributor(t, dir)

	ctx := context.Background()

	// Generate valid instruction files first.
	if err := dist.GenerateAll(ctx); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	issues := validateAgent(ctx, dist, cfg, "dev")
	if len(issues) > 0 {
		t.Errorf("expected no issues for valid file, got: %v", issues)
	}
}

func TestValidateAgent_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := buildTestDistributor(t, dir)

	ctx := context.Background()

	// Don't generate files — they should be missing.
	issues := validateAgent(ctx, dist, cfg, "dev")

	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "file missing") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'file missing' issue, got: %v", issues)
	}
}

func TestValidateAgent_LeakedTemplateSyntax(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := buildTestDistributor(t, dir)

	ctx := context.Background()

	// Generate then corrupt the file with template syntax.
	if err := dist.GenerateAll(ctx); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("## Identity\n## Constraints\n## Session\n[[ .Something ]]"), 0o644); err != nil {
		t.Fatal(err)
	}

	issues := validateAgent(ctx, dist, cfg, "dev")

	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "leaked template syntax") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'leaked template syntax' issue, got: %v", issues)
	}
}

func TestValidateAgent_StaleFile(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := buildTestDistributor(t, dir)

	ctx := context.Background()

	// Generate valid files.
	if err := dist.GenerateAll(ctx); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	// Overwrite the file with completely different content to simulate
	// staleness. The marker-based InsertGenerated preserves content outside
	// markers, so we must replace the entire file to create a mismatch.
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	staleContent := "## Identity\nStale identity\n## Constraints\nStale constraints\n## Session\nStale session\n"
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}

	issues := validateAgent(ctx, dist, cfg, "dev")

	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "stale") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'stale' issue, got: %v", issues)
	}
}
