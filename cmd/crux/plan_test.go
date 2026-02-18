package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAPIKey_CruxEnvVar(t *testing.T) {
	t.Setenv("CRUX_ANTHROPIC_API_KEY", "crux-key-123")
	t.Setenv("ANTHROPIC_API_KEY", "")

	key := resolveAPIKey()
	if key != "crux-key-123" {
		t.Errorf("resolveAPIKey() = %q, want %q", key, "crux-key-123")
	}
}

func TestResolveAPIKey_AnthropicEnvVar(t *testing.T) {
	t.Setenv("CRUX_ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key-456")

	key := resolveAPIKey()
	if key != "anthropic-key-456" {
		t.Errorf("resolveAPIKey() = %q, want %q", key, "anthropic-key-456")
	}
}

func TestResolveAPIKey_Empty(t *testing.T) {
	t.Setenv("CRUX_ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	key := resolveAPIKey()
	if key != "" {
		t.Errorf("resolveAPIKey() = %q, want empty", key)
	}
}

func TestResolveAPIKey_CruxTakesPrecedence(t *testing.T) {
	t.Setenv("CRUX_ANTHROPIC_API_KEY", "crux-wins")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-loses")

	key := resolveAPIKey()
	if key != "crux-wins" {
		t.Errorf("resolveAPIKey() = %q, want %q (CRUX_ should take precedence)", key, "crux-wins")
	}
}

func TestPlanCmd_HasExpectedFlags(t *testing.T) {
	f := planCmd.Flags()

	fd := f.Lookup("from-description")
	if fd == nil {
		t.Fatal("--from-description flag not registered")
	}
	if fd.DefValue != "" {
		t.Errorf("--from-description default = %q, want empty", fd.DefValue)
	}

	vf := f.Lookup("validate")
	if vf == nil {
		t.Fatal("--validate flag not registered")
	}
	if vf.DefValue != "false" {
		t.Errorf("--validate default = %q, want %q", vf.DefValue, "false")
	}
}

func TestPlanCmd_UseAndShort(t *testing.T) {
	if planCmd.Use != "plan" {
		t.Errorf("Use = %q, want %q", planCmd.Use, "plan")
	}
	if planCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestRunPlanValidation_NoDirectory(t *testing.T) {
	dir := t.TempDir()
	origCfg := cfgFile
	defer func() { cfgFile = origCfg }()

	// Write a minimal config.
	cruxDir := filepath.Join(dir, ".crux")
	if err := os.MkdirAll(cruxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cruxDir, "config.yaml")
	cfg := `project:
  name: test-project
  root: ` + dir + `
  state_dir: ` + cruxDir + `
memory:
  sqlite_path: ` + filepath.Join(cruxDir, "memory.db") + `
  vector_dir: ` + filepath.Join(cruxDir, "vectors") + `
phases:
  spec_dir: docs/phases
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath

	// No docs/phases/ directory — should handle gracefully.
	if err := runPlanValidation(); err != nil {
		t.Fatalf("runPlanValidation should not error for missing dir: %v", err)
	}
}

func TestRunPlanValidation_ValidatesFiles(t *testing.T) {
	dir := t.TempDir()
	origCfg := cfgFile
	defer func() { cfgFile = origCfg }()

	cruxDir := filepath.Join(dir, ".crux")
	phasesDir := filepath.Join(dir, "docs", "phases")
	if err := os.MkdirAll(cruxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(phasesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(cruxDir, "config.yaml")
	cfg := `project:
  name: test-project
  root: ` + dir + `
  state_dir: ` + cruxDir + `
memory:
  sqlite_path: ` + filepath.Join(cruxDir, "memory.db") + `
  vector_dir: ` + filepath.Join(cruxDir, "vectors") + `
phases:
  spec_dir: docs/phases
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath

	// Write a valid spec file.
	validSpec := `# Phase 1A: Setup
## Status
Not started
## Depends On
None
## Tasks
### Prompt 1
- Create project scaffold
## Files
- New: main.go
## Exit Criteria
- ` + "`go build ./...`" + ` passes
`
	if err := os.WriteFile(filepath.Join(phasesDir, "PHASE1A.md"), []byte(validSpec), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write an invalid spec file (missing sections).
	invalidSpec := `# Phase 1B: Broken
## Status
Not started
`
	if err := os.WriteFile(filepath.Join(phasesDir, "PHASE1B.md"), []byte(invalidSpec), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should run without error (issues are printed, not returned as error).
	if err := runPlanValidation(); err != nil {
		t.Fatalf("runPlanValidation: %v", err)
	}
}

func TestRunPlan_MissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	origCfg := cfgFile
	defer func() { cfgFile = origCfg }()

	t.Setenv("CRUX_ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cruxDir := filepath.Join(dir, ".crux")
	if err := os.MkdirAll(cruxDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(cruxDir, "config.yaml")
	cfg := `project:
  name: test-project
  root: ` + dir + `
  state_dir: ` + cruxDir + `
memory:
  sqlite_path: ` + filepath.Join(cruxDir, "memory.db") + `
  vector_dir: ` + filepath.Join(cruxDir, "vectors") + `
phases:
  spec_dir: docs/phases
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath

	// runPlan should return nil (graceful degradation), not an error.
	if err := runPlan(planCmd, nil); err != nil {
		t.Fatalf("runPlan with missing API key should degrade gracefully: %v", err)
	}
}
