package planner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_Success(t *testing.T) {
	root := t.TempDir()
	content := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &readFileTool{root: root}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != content {
		t.Errorf("result = %q, want %q", result, content)
	}
}

func TestReadFile_PathTraversal(t *testing.T) {
	root := t.TempDir()
	// Create a file inside root so the tool is functional.
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &readFileTool{root: root}

	tests := []struct {
		name string
		path string
	}{
		{"dot-dot slash", "../../../etc/passwd"},
		{"absolute path", "/etc/passwd"},
		{"dot-dot in middle", "subdir/../../etc/passwd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"`+tc.path+`"}`))
			if err == nil {
				t.Error("expected error for path traversal, got nil")
			}
			if err != nil && !strings.Contains(err.Error(), "escapes project root") && !strings.Contains(err.Error(), "reading file") {
				// Absolute paths on some systems might fail at read time
				// rather than at securePath, which is also acceptable.
				t.Logf("error (acceptable): %v", err)
			}
		})
	}
}

func TestReadFile_Truncation(t *testing.T) {
	root := t.TempDir()

	// Create a file with more than maxFileLines lines.
	var lines []string
	for i := 0; i < maxFileLines+100; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &readFileTool{root: root}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"big.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result, "[truncated:") {
		t.Error("expected truncation notice in result")
	}
	if !strings.Contains(result, "500 of 600") {
		t.Errorf("expected '500 of 600' in truncation notice, got: %s", result[len(result)-60:])
	}
}

func TestReadFile_EmptyPath(t *testing.T) {
	tool := &readFileTool{root: t.TempDir()}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":""}`))
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestReadFile_NonexistentFile(t *testing.T) {
	tool := &readFileTool{root: t.TempDir()}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"no-such-file.txt"}`))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- validate_spec tests ---

const validSpec = `# Phase 1A: Basic Setup

## Status
Pending

## Depends On
None

## Design Rationale
Initial project scaffolding.

## Tasks

### Prompt 1
- Create project structure
- Initialize go module

## Files

### New
- go.mod
- main.go

### Modified
None

### Referenced
None

## Exit Criteria
- [ ] ` + "`go build ./...`" + ` exits 0
- [ ] ` + "`go vet ./...`" + ` exits 0
`

func TestValidateSpec_Valid(t *testing.T) {
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  validSpec,
		"doc_type": "spec",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "no issues found") {
		t.Errorf("expected valid spec, got: %s", result)
	}
}

func TestValidateSpec_MissingSections(t *testing.T) {
	tests := []struct {
		name    string
		content string
		missing string
	}{
		{
			name:    "missing status",
			content: "# Phase 1A: Test\n## Depends On\nNone\n## Tasks\n## Files\n## Exit Criteria\n- [ ] `go build ./...`\n",
			missing: "Status",
		},
		{
			name:    "missing depends on",
			content: "# Phase 1A: Test\n## Status\nPending\n## Tasks\n## Files\n## Exit Criteria\n- [ ] `go build ./...`\n",
			missing: "Depends On",
		},
		{
			name:    "missing tasks",
			content: "# Phase 1A: Test\n## Status\nPending\n## Depends On\nNone\n## Files\n## Exit Criteria\n- [ ] `go build ./...`\n",
			missing: "Tasks",
		},
		{
			name:    "missing files",
			content: "# Phase 1A: Test\n## Status\nPending\n## Depends On\nNone\n## Tasks\n## Exit Criteria\n- [ ] `go build ./...`\n",
			missing: "Files",
		},
		{
			name:    "missing exit criteria",
			content: "# Phase 1A: Test\n## Status\nPending\n## Depends On\nNone\n## Tasks\n## Files\n",
			missing: "Exit Criteria",
		},
	}

	tool := &validateSpecTool{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{
				"content":  tc.content,
				"doc_type": "spec",
			})
			result, err := tool.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !strings.Contains(result, tc.missing) {
				t.Errorf("expected issue mentioning %q, got: %s", tc.missing, result)
			}
		})
	}
}

func TestValidateSpec_NoExecutableExitCriteria(t *testing.T) {
	content := "# Phase 1A: Test\n## Status\nPending\n## Depends On\nNone\n## Tasks\n## Files\n## Exit Criteria\n- [ ] Code looks good\n"
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  content,
		"doc_type": "spec",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "executable commands") {
		t.Errorf("expected warning about executable commands, got: %s", result)
	}
}

func TestValidateSpec_MissingPhaseHeader(t *testing.T) {
	content := "## Status\nPending\n## Depends On\nNone\n## Tasks\n## Files\n## Exit Criteria\n- [ ] `go build`\n"
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  content,
		"doc_type": "spec",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "phase header") {
		t.Errorf("expected issue about phase header, got: %s", result)
	}
}

// --- validate prompt doc tests ---

const validPromptDoc = `# Phase 1A Implementation Prompts

## Prompt 1 of 2: Setup

### Required Reading
- go.mod
- main.go

### Interface Contract
` + "```go" + `
func New() *App
` + "```" + `

### Task
1. Create the app struct.

### Verification
` + "```bash" + `
go build ./...
` + "```" + `

### Acceptance Criteria
- App struct exists
- Builds clean

---

## Prompt 2 of 2: Tests

### Required Reading
- main.go

### Task
1. Write tests for main.

### Verification
` + "```bash" + `
go test ./...
` + "```" + `

### Acceptance Criteria
- All tests pass
`

func TestValidatePromptDoc_Valid(t *testing.T) {
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  validPromptDoc,
		"doc_type": "prompt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "no issues found") {
		t.Errorf("expected valid prompt doc, got: %s", result)
	}
}

func TestValidatePromptDoc_MissingSections(t *testing.T) {
	// Prompt doc with no Required Reading.
	content := `# Phase 1A Implementation Prompts

## Prompt 1 of 1: Setup

### Task
1. Do something.

### Verification
` + "```bash" + `
go build ./...
` + "```" + `

### Acceptance Criteria
- Done
`

	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  content,
		"doc_type": "prompt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "Required Reading") {
		t.Errorf("expected issue about Required Reading, got: %s", result)
	}
}

func TestValidatePromptDoc_NoPromptHeading(t *testing.T) {
	content := "# Phase 1A\n\nSome content without prompt headings.\n"
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  content,
		"doc_type": "prompt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "missing prompt heading") {
		t.Errorf("expected issue about missing prompt heading, got: %s", result)
	}
}

func TestValidateSpec_InvalidDocType(t *testing.T) {
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  "test",
		"doc_type": "invalid",
	})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid doc_type")
	}
}

func TestValidateSpec_EmptyContent(t *testing.T) {
	tool := &validateSpecTool{}
	input, _ := json.Marshal(map[string]string{
		"content":  "",
		"doc_type": "spec",
	})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty content")
	}
}

// --- generate_phase_docs tests ---

func TestGeneratePhaseDocs_WritesFiles(t *testing.T) {
	root := t.TempDir()
	tool := &generatePhaseDocsTool{root: root}

	phases := []phaseInput{
		{
			ID:            "1A",
			SpecContent:   validSpec,
			PromptContent: validPromptDoc,
		},
	}
	input, _ := json.Marshal(map[string]any{"phases": phases})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Check files were written.
	specPath := filepath.Join(root, "docs", "phases", "PHASE1A.md")
	promptPath := filepath.Join(root, "docs", "phases", "PHASE1A-PROMPT.md")

	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("spec file not created: %v", err)
	}
	if _, err := os.Stat(promptPath); err != nil {
		t.Errorf("prompt file not created: %v", err)
	}

	// Verify content.
	specData, _ := os.ReadFile(specPath)
	if string(specData) != validSpec {
		t.Error("spec content mismatch")
	}

	promptData, _ := os.ReadFile(promptPath)
	if string(promptData) != validPromptDoc {
		t.Error("prompt content mismatch")
	}

	// Check result mentions files.
	if !strings.Contains(result, "PHASE1A.md") {
		t.Errorf("result should mention PHASE1A.md, got: %s", result)
	}
	if !strings.Contains(result, "PHASE1A-PROMPT.md") {
		t.Errorf("result should mention PHASE1A-PROMPT.md, got: %s", result)
	}
	if !strings.Contains(result, "1 phase(s)") {
		t.Errorf("result should mention phase count, got: %s", result)
	}
}

func TestGeneratePhaseDocs_MultiplePhases(t *testing.T) {
	root := t.TempDir()
	tool := &generatePhaseDocsTool{root: root}

	phases := []phaseInput{
		{ID: "1A", SpecContent: validSpec, PromptContent: validPromptDoc},
		{ID: "1B", SpecContent: validSpec, PromptContent: validPromptDoc},
	}
	input, _ := json.Marshal(map[string]any{"phases": phases})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result, "2 phase(s)") {
		t.Errorf("expected 2 phases, got: %s", result)
	}
	if !strings.Contains(result, "4 file(s)") {
		t.Errorf("expected 4 files, got: %s", result)
	}

	// All four files should exist.
	for _, name := range []string{"PHASE1A.md", "PHASE1A-PROMPT.md", "PHASE1B.md", "PHASE1B-PROMPT.md"} {
		path := filepath.Join(root, "docs", "phases", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

func TestGeneratePhaseDocs_ValidationWarnings(t *testing.T) {
	root := t.TempDir()
	tool := &generatePhaseDocsTool{root: root}

	// Invalid spec: missing required sections.
	phases := []phaseInput{
		{
			ID:            "2A",
			SpecContent:   "# Phase 2A: Bad\n\n## Status\nPending\n",
			PromptContent: validPromptDoc,
		},
	}
	input, _ := json.Marshal(map[string]any{"phases": phases})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Files should still be written (warnings, not errors).
	specPath := filepath.Join(root, "docs", "phases", "PHASE2A.md")
	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("spec file should be written despite warnings: %v", err)
	}

	if !strings.Contains(result, "Validation warnings") {
		t.Errorf("expected validation warnings, got: %s", result)
	}
}

func TestGeneratePhaseDocs_EmptyPhases(t *testing.T) {
	tool := &generatePhaseDocsTool{root: t.TempDir()}
	input, _ := json.Marshal(map[string]any{"phases": []phaseInput{}})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty phases")
	}
}

func TestGeneratePhaseDocs_MissingID(t *testing.T) {
	tool := &generatePhaseDocsTool{root: t.TempDir()}
	phases := []phaseInput{{ID: "", SpecContent: "x", PromptContent: "y"}}
	input, _ := json.Marshal(map[string]any{"phases": phases})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing phase ID")
	}
}

func TestGeneratePhaseDocs_CreatesDirectory(t *testing.T) {
	root := t.TempDir()
	// Ensure docs/phases doesn't exist yet.
	phasesDir := filepath.Join(root, "docs", "phases")
	if _, err := os.Stat(phasesDir); err == nil {
		t.Fatal("docs/phases should not exist before test")
	}

	tool := &generatePhaseDocsTool{root: root}
	phases := []phaseInput{
		{ID: "1A", SpecContent: validSpec, PromptContent: validPromptDoc},
	}
	input, _ := json.Marshal(map[string]any{"phases": phases})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(phasesDir); err != nil {
		t.Errorf("docs/phases directory should be created: %v", err)
	}
}

// --- ExecuteTool tests ---

func TestExecuteTool_UnknownTool(t *testing.T) {
	result, isError := ExecuteTool(context.Background(), t.TempDir(), "nonexistent", nil)
	if !isError {
		t.Error("expected isError for unknown tool")
	}
	if !strings.Contains(result, "unknown tool") {
		t.Errorf("result = %q, want 'unknown tool'", result)
	}
}

func TestExecuteTool_DispatchesCorrectly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, isError := ExecuteTool(context.Background(), root, "read_file", json.RawMessage(`{"path":"test.txt"}`))
	if isError {
		t.Fatalf("unexpected error: %s", result)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

// --- RegisterTools test ---

func TestRegisterTools_RegistersFourTools(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	RegisterTools(agent, t.TempDir())

	if len(agent.sdkBackend.tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(agent.sdkBackend.tools))
	}

	names := make(map[string]bool)
	for _, tool := range agent.sdkBackend.tools {
		if tool.OfTool != nil {
			names[tool.OfTool.Name] = true
		}
	}

	for _, expected := range []string{"read_file", "validate_spec", "generate_phase_docs", "generate_single_phase"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

func TestRegisterTools_SetsRequired(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	RegisterTools(agent, t.TempDir())

	for _, tool := range agent.sdkBackend.tools {
		if tool.OfTool == nil {
			continue
		}
		if len(tool.OfTool.InputSchema.Required) == 0 {
			t.Errorf("tool %q: Required should not be empty", tool.OfTool.Name)
		}
	}
}

func TestExtractPropertyNames(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   int
	}{
		{
			name:   "two properties",
			schema: `{"a": {"type":"string"}, "b": {"type":"number"}}`,
			want:   2,
		},
		{
			name:   "single property",
			schema: `{"path": {"type":"string"}}`,
			want:   1,
		},
		{
			name:   "invalid json",
			schema: `not json`,
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := extractPropertyNames(json.RawMessage(tc.schema))
			if len(names) != tc.want {
				t.Errorf("extractPropertyNames returned %d names, want %d", len(names), tc.want)
			}
		})
	}
}

// --- securePath tests ---

func TestSecurePath_Valid(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := securePath(root, "file.txt")
	if err != nil {
		t.Fatalf("securePath: %v", err)
	}
	if !strings.HasPrefix(resolved, root) {
		t.Errorf("resolved path %q should be under root %q", resolved, root)
	}
}

func TestSecurePath_Escaping(t *testing.T) {
	root := t.TempDir()

	_, err := securePath(root, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

// --- generate_single_phase tests ---

func TestGenerateSinglePhase_WritesFiles(t *testing.T) {
	root := t.TempDir()
	tool := &generateSinglePhaseTool{root: root}

	input, _ := json.Marshal(phaseInput{
		ID:            "1A",
		SpecContent:   validSpec,
		PromptContent: validPromptDoc,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	specPath := filepath.Join(root, "docs", "phases", "PHASE1A.md")
	promptPath := filepath.Join(root, "docs", "phases", "PHASE1A-PROMPT.md")

	specData, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("spec file not created: %v", err)
	}
	if string(specData) != validSpec {
		t.Error("spec content mismatch")
	}

	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("prompt file not created: %v", err)
	}
	if string(promptData) != validPromptDoc {
		t.Error("prompt content mismatch")
	}

	if !strings.Contains(result, "PHASE1A.md") {
		t.Errorf("result should mention PHASE1A.md, got: %s", result)
	}
	if !strings.Contains(result, "PHASE1A-PROMPT.md") {
		t.Errorf("result should mention PHASE1A-PROMPT.md, got: %s", result)
	}
}

func TestGenerateSinglePhase_ValidationWarnings(t *testing.T) {
	root := t.TempDir()
	tool := &generateSinglePhaseTool{root: root}

	input, _ := json.Marshal(phaseInput{
		ID:            "2A",
		SpecContent:   "# Phase 2A: Bad\n\n## Status\nPending\n",
		PromptContent: validPromptDoc,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Files should still be written.
	specPath := filepath.Join(root, "docs", "phases", "PHASE2A.md")
	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("spec file should be written despite warnings: %v", err)
	}

	if !strings.Contains(result, "Validation warnings") {
		t.Errorf("expected validation warnings, got: %s", result)
	}
}

func TestGenerateSinglePhase_EmptyID(t *testing.T) {
	tool := &generateSinglePhaseTool{root: t.TempDir()}
	input, _ := json.Marshal(phaseInput{ID: "", SpecContent: "x", PromptContent: "y"})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty phase ID")
	}
}

func TestGenerateSinglePhase_CreatesDirectory(t *testing.T) {
	root := t.TempDir()
	phasesDir := filepath.Join(root, "docs", "phases")
	if _, err := os.Stat(phasesDir); err == nil {
		t.Fatal("docs/phases should not exist before test")
	}

	tool := &generateSinglePhaseTool{root: root}
	input, _ := json.Marshal(phaseInput{ID: "1A", SpecContent: validSpec, PromptContent: validPromptDoc})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(phasesDir); err != nil {
		t.Errorf("docs/phases directory should be created: %v", err)
	}
}

func TestExecuteTool_DispatchesSinglePhase(t *testing.T) {
	root := t.TempDir()
	input, _ := json.Marshal(phaseInput{ID: "3A", SpecContent: validSpec, PromptContent: validPromptDoc})

	result, isError := ExecuteTool(context.Background(), root, "generate_single_phase", input)
	if isError {
		t.Fatalf("unexpected error: %s", result)
	}
	if !strings.Contains(result, "Phase 3A written") {
		t.Errorf("result should confirm phase 3A, got: %s", result)
	}

	specPath := filepath.Join(root, "docs", "phases", "PHASE3A.md")
	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("spec file should exist: %v", err)
	}
}

func TestSecurePath_NestedValid(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.go"), []byte("pkg"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := securePath(root, "sub/dir/file.go")
	if err != nil {
		t.Fatalf("securePath: %v", err)
	}
	if !strings.HasPrefix(resolved, root) {
		t.Errorf("resolved path %q should be under root %q", resolved, root)
	}
}
