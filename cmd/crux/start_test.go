package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
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

func TestValidateGitRepo(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		wantErr string
	}{
		{
			name:    "not a git repo",
			setup:   func(t *testing.T, dir string) {},
			wantErr: "not a git repository",
		},
		{
			name: "git init but no commits",
			setup: func(t *testing.T, dir string) {
				cmd := exec.Command("git", "init", dir)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git init: %v\n%s", err, out)
				}
			},
			wantErr: "no commits",
		},
		{
			name: "valid repo with commit",
			setup: func(t *testing.T, dir string) {
				for _, args := range [][]string{
					{"git", "init", dir},
					{"git", "-C", dir, "config", "user.email", "test@test.com"},
					{"git", "-C", dir, "config", "user.name", "Test"},
					{"git", "-C", dir, "commit", "--allow-empty", "-m", "initial"},
				} {
					cmd := exec.Command(args[0], args[1:]...)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("%s: %v\n%s", args, err, out)
					}
				}
			},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)

			err := validateGitRepo(dir)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestDecideStartupCursor_UsesVerifiedCursor(t *testing.T) {
	order := []types.PhaseID{"1A", "1B"}
	progress := map[types.PhaseID]phase.PhaseProgress{
		"1A": {Prompts: []phase.PromptContract{{PromptNumber: 1}, {PromptNumber: 2}}},
		"1B": {Prompts: []phase.PromptContract{{PromptNumber: 1}}},
	}
	result := &orchestrator.ReconcileResult{
		VerifiedPhase:  "1A",
		VerifiedPrompt: 2,
	}

	phaseID, promptNum, fromScratch, reason := decideStartupCursor(nil, result, order, progress)
	if phaseID != "1A" || promptNum != 2 {
		t.Fatalf("cursor = %s:%d, want 1A:2", phaseID, promptNum)
	}
	if fromScratch {
		t.Fatalf("fromScratch = true, want false (reason=%s)", reason)
	}
}

func TestDecideStartupCursor_FallsBackOnReconcileError(t *testing.T) {
	order := []types.PhaseID{"1A", "1B"}
	progress := map[types.PhaseID]phase.PhaseProgress{
		"1A": {Prompts: []phase.PromptContract{{PromptNumber: 1}}},
		"1B": {Prompts: []phase.PromptContract{{PromptNumber: 1}}},
	}

	phaseID, promptNum, fromScratch, reason := decideStartupCursor(
		errors.New("reconcile failed"),
		nil,
		order,
		progress,
	)
	if phaseID != "1A" || promptNum != 1 {
		t.Fatalf("cursor = %s:%d, want 1A:1", phaseID, promptNum)
	}
	if !fromScratch {
		t.Fatal("fromScratch = false, want true")
	}
	if reason != "reconciliation_error" {
		t.Fatalf("reason = %q, want reconciliation_error", reason)
	}
}

func TestDecideStartupCursor_FallsBackOnInvalidVerifiedPrompt(t *testing.T) {
	order := []types.PhaseID{"1A", "1B"}
	progress := map[types.PhaseID]phase.PhaseProgress{
		"1A": {Prompts: []phase.PromptContract{{PromptNumber: 1}}},
		"1B": {Prompts: []phase.PromptContract{{PromptNumber: 1}}},
	}
	result := &orchestrator.ReconcileResult{
		VerifiedPhase:  "1A",
		VerifiedPrompt: 9,
	}

	phaseID, promptNum, fromScratch, reason := decideStartupCursor(nil, result, order, progress)
	if phaseID != "1A" || promptNum != 1 {
		t.Fatalf("cursor = %s:%d, want 1A:1", phaseID, promptNum)
	}
	if !fromScratch {
		t.Fatal("fromScratch = false, want true")
	}
	if reason != "invalid_verified_prompt" {
		t.Fatalf("reason = %q, want invalid_verified_prompt", reason)
	}
}
