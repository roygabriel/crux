package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// phaseASpec is a minimal phase spec with human-approval gates only.
const phaseASpec = `# Phase A: Foundation

## Status

planned

## Depends On

None

## Exit Criteria

- [ ] All foundation code compiles
- [ ] Unit tests pass
`

// phaseAPrompt contains 2 prompts for Phase A with human-approval verification.
const phaseAPrompt = `# Phase A Prompts

## Prompt 1: Setup project structure

### Task

Create the basic project structure with config and types.

### Verification

- [ ] Project structure created
- [ ] Config loads correctly

## Prompt 2: Add core types

### Task

Define the core domain types.

### Verification

- [ ] Types compile
- [ ] Tests pass
`

// phaseBSpec depends on Phase A.
const phaseBSpec = `# Phase B: Integration

## Status

planned

## Depends On

- A

## Exit Criteria

- [ ] Integration tests pass
`

// phaseBPrompt contains 1 prompt for Phase B.
const phaseBPrompt = `# Phase B Prompts

## Prompt 1: Wire integration

### Task

Wire the integration layer.

### Verification

- [ ] Integration compiles
`

// SetupTwoPhaseProject writes a 2-phase project (A: 2 prompts, B: 1 prompt
// depending on A) into the given directory. It creates the required
// subdirectory structure under dir.
func SetupTwoPhaseProject(t *testing.T, dir string) {
	t.Helper()

	phasesDir := filepath.Join(dir, "phases")
	mustMkdir(t, phasesDir)
	mustWrite(t, filepath.Join(phasesDir, "PHASEA.md"), phaseASpec)
	mustWrite(t, filepath.Join(phasesDir, "PHASEA-PROMPT.md"), phaseAPrompt)
	mustWrite(t, filepath.Join(phasesDir, "PHASEB.md"), phaseBSpec)
	mustWrite(t, filepath.Join(phasesDir, "PHASEB-PROMPT.md"), phaseBPrompt)
}

// SetupDirs creates the standard state directories under dir.
func SetupDirs(t *testing.T, dir string) {
	t.Helper()
	for _, sub := range []string{"sessions", "notes", "phases"} {
		mustMkdir(t, filepath.Join(dir, sub))
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
