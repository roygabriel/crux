package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
)

// writeTestStartConfig writes a minimal config with a claude agent for start tests.
func writeTestStartConfig(t *testing.T, projectRoot string) (*config.Config, *instruct.Distributor) {
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

func TestEnsureInstructionFiles_GeneratesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := writeTestStartConfig(t, dir)

	ctx := context.Background()
	log := setupLogger()

	generated, refreshed, err := ensureInstructionFiles(ctx, dist, cfg, log)
	if err != nil {
		t.Fatalf("ensureInstructionFiles: %v", err)
	}
	if generated == 0 {
		t.Error("expected generated > 0 when files are missing")
	}
	if refreshed != 0 {
		t.Errorf("expected refreshed = 0, got %d", refreshed)
	}

	// Verify files were actually written.
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		t.Errorf("CLAUDE.md not generated: %v", err)
	}
}

func TestEnsureInstructionFiles_SkipsWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	cfg, dist := writeTestStartConfig(t, dir)

	ctx := context.Background()
	log := setupLogger()

	// First call generates.
	if _, _, err := ensureInstructionFiles(ctx, dist, cfg, log); err != nil {
		t.Fatalf("first ensureInstructionFiles: %v", err)
	}

	// Build a fresh distributor (no in-memory state carried over).
	dist2 := buildDistributor(cfg, log)

	// Second call should find files up to date.
	generated, refreshed, err := ensureInstructionFiles(ctx, dist2, cfg, log)
	if err != nil {
		t.Fatalf("second ensureInstructionFiles: %v", err)
	}
	if generated != 0 {
		t.Errorf("expected generated = 0, got %d", generated)
	}
	if refreshed != 0 {
		t.Errorf("expected refreshed = 0, got %d", refreshed)
	}
}

func TestInstructionFilesMissing_DetectsAbsent(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := writeTestStartConfig(t, dir)

	if !instructionFilesMissing(cfg) {
		t.Error("expected instructionFilesMissing to return true when no files exist")
	}

	// Generate the files.
	log := setupLogger()
	dist := buildDistributor(cfg, log)
	if err := dist.GenerateAll(context.Background()); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	if instructionFilesMissing(cfg) {
		t.Error("expected instructionFilesMissing to return false after generation")
	}
}
