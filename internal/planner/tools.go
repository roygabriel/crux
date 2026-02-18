package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// maxFileLines is the maximum number of lines read_file returns before truncation.
const maxFileLines = 500

// ToolHandler defines the interface for a planning agent tool.
type ToolHandler interface {
	// Name returns the tool name as registered with the Anthropic API.
	Name() string
	// Description returns a human-readable description of the tool.
	Description() string
	// InputSchema returns the JSON Schema for the tool's input.
	InputSchema() json.RawMessage
	// Execute runs the tool with the given JSON input and returns the result.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// RegisterTools creates all planning tools and registers them with the agent.
func RegisterTools(agent *Agent, projectRoot string) {
	handlers := []ToolHandler{
		&readFileTool{root: projectRoot},
		&validateSpecTool{},
		&generatePhaseDocsTool{root: projectRoot},
	}

	toolDefs := make([]anthropic.ToolUnionParam, len(handlers))
	for i, h := range handlers {
		schema := h.InputSchema()
		toolDefs[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        h.Name(),
				Description: anthropic.String(h.Description()),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: json.RawMessage(schema),
					Required:   extractPropertyNames(schema),
				},
			},
		}
	}

	agent.SetTools(toolDefs)
}

// extractPropertyNames parses a JSON object and returns its top-level keys.
func extractPropertyNames(schema json.RawMessage) []string {
	var props map[string]json.RawMessage
	if err := json.Unmarshal(schema, &props); err != nil {
		return nil
	}
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	return names
}

// ExecuteTool dispatches a tool call by name. It returns the result string and
// whether the execution resulted in an error.
func ExecuteTool(ctx context.Context, projectRoot string, name string, input json.RawMessage) (result string, isError bool) {
	handlers := map[string]ToolHandler{
		"read_file":            &readFileTool{root: projectRoot},
		"validate_spec":       &validateSpecTool{},
		"generate_phase_docs": &generatePhaseDocsTool{root: projectRoot},
	}

	handler, ok := handlers[name]
	if !ok {
		return fmt.Sprintf("unknown tool: %s", name), true
	}

	res, err := handler.Execute(ctx, input)
	if err != nil {
		return err.Error(), true
	}
	return res, false
}

// --- read_file tool ---

type readFileTool struct {
	root string
}

func (t *readFileTool) Name() string { return "read_file" }

func (t *readFileTool) Description() string {
	return "Read the contents of a file from the project. Use this to examine existing code, configs, or docs that the user references."
}

func (t *readFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"path": {"type": "string", "description": "Relative path from project root"}
	}`)
}

func (t *readFileTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := securePath(t.root, params.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxFileLines {
		truncated := strings.Join(lines[:maxFileLines], "\n")
		return fmt.Sprintf("%s\n\n[truncated: showing %d of %d lines]", truncated, maxFileLines, len(lines)), nil
	}

	return content, nil
}

// securePath resolves a relative path against root and ensures it does not
// escape the project directory.
func securePath(root, rel string) (string, error) {
	// Clean both paths to resolve any . or .. components.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving project root: %w", err)
	}

	joined := filepath.Join(absRoot, rel)
	resolved, err := filepath.EvalSymlinks(filepath.Dir(joined))
	if err != nil {
		// If the directory doesn't exist, fall back to Clean.
		resolved = filepath.Clean(joined)
	} else {
		resolved = filepath.Join(resolved, filepath.Base(joined))
	}

	if !strings.HasPrefix(resolved, absRoot) {
		return "", fmt.Errorf("path %q escapes project root", rel)
	}

	return resolved, nil
}

// --- validate_spec tool ---

type validateSpecTool struct{}

func (t *validateSpecTool) Name() string { return "validate_spec" }

func (t *validateSpecTool) Description() string {
	return "Validate that a phase spec or prompt doc follows the Crux format. Returns a list of issues."
}

func (t *validateSpecTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"content": {"type": "string", "description": "The markdown content to validate"},
		"doc_type": {"type": "string", "enum": ["spec", "prompt"], "description": "Type of document"}
	}`)
}

func (t *validateSpecTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Content string `json:"content"`
		DocType string `json:"doc_type"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	if params.DocType != "spec" && params.DocType != "prompt" {
		return "", fmt.Errorf("doc_type must be \"spec\" or \"prompt\"")
	}

	var issues []string
	if params.DocType == "spec" {
		issues = validateSpec(params.Content)
	} else {
		issues = validatePromptDoc(params.Content)
	}

	if len(issues) == 0 {
		return "Validation passed: no issues found.", nil
	}
	return fmt.Sprintf("Found %d issue(s):\n- %s", len(issues), strings.Join(issues, "\n- ")), nil
}

// validateSpec checks that a phase spec markdown has the required sections.
func validateSpec(content string) []string {
	var issues []string

	requiredSections := []string{
		"## Status",
		"## Depends On",
		"## Tasks",
		"## Files",
		"## Exit Criteria",
	}
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			issues = append(issues, fmt.Sprintf("missing section: %s", section))
		}
	}

	// Exit criteria should contain executable commands (backtick-quoted).
	if strings.Contains(content, "## Exit Criteria") {
		exitSection := extractSection(content, "## Exit Criteria")
		if !strings.Contains(exitSection, "`") {
			issues = append(issues, "exit criteria should contain executable commands in backticks")
		}
	}

	// Should have a phase header.
	if !strings.Contains(content, "# Phase ") {
		issues = append(issues, "missing phase header (expected '# Phase <ID>: <Title>')")
	}

	return issues
}

// validatePromptDoc checks that a prompt doc markdown has required sections
// for each prompt.
func validatePromptDoc(content string) []string {
	var issues []string

	// Should have at least one prompt heading.
	if !strings.Contains(content, "## Prompt ") {
		issues = append(issues, "missing prompt heading (expected '## Prompt N of M: Title')")
		return issues
	}

	// Split into prompt sections for per-prompt validation.
	prompts := splitPromptSections(content)

	for _, p := range prompts {
		heading := extractPromptHeading(p)

		requiredSubs := []string{
			"### Required Reading",
			"### Task",
			"### Verification",
			"### Acceptance Criteria",
		}
		for _, sub := range requiredSubs {
			if !strings.Contains(p, sub) {
				issues = append(issues, fmt.Sprintf("%s: missing section %s", heading, sub))
			}
		}

		// If the prompt creates code, it should have an interface contract.
		if strings.Contains(p, "### Task") {
			taskSection := extractSection(p, "### Task")
			looksLikeCode := strings.Contains(taskSection, "Create ") ||
				strings.Contains(taskSection, "Implement ") ||
				strings.Contains(taskSection, ".go")
			if looksLikeCode && !strings.Contains(p, "### Interface Contract") {
				issues = append(issues, fmt.Sprintf("%s: code-producing prompt should have an Interface Contract section", heading))
			}
		}
	}

	return issues
}

// splitPromptSections splits a prompt doc into individual prompt sections.
func splitPromptSections(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Prompt ") {
			if len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n"))
			}
			current = []string{line}
		} else if len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}
	return sections
}

// extractPromptHeading returns the first line of a prompt section as a label.
func extractPromptHeading(section string) string {
	lines := strings.SplitN(section, "\n", 2)
	return strings.TrimSpace(lines[0])
}

// extractSection returns the content between a heading and the next heading
// of equal or higher level.
func extractSection(content, heading string) string {
	idx := strings.Index(content, heading)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(heading):]

	// Determine heading level.
	level := 0
	for _, c := range heading {
		if c == '#' {
			level++
		} else {
			break
		}
	}

	// Find the next heading of equal or higher level.
	lines := strings.Split(rest, "\n")
	var sectionLines []string
	for _, line := range lines[1:] { // skip heading line itself
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= level {
			isHeading := true
			for i := 0; i < level; i++ {
				if trimmed[i] != '#' {
					isHeading = false
					break
				}
			}
			if isHeading && (len(trimmed) == level || trimmed[level] == ' ') {
				break
			}
		}
		sectionLines = append(sectionLines, line)
	}
	return strings.Join(sectionLines, "\n")
}

// ValidateSpecContent validates a phase spec markdown and returns a list of issues.
// An empty slice means the content is valid.
func ValidateSpecContent(content string) []string {
	return validateSpec(content)
}

// ValidatePromptContent validates a prompt doc markdown and returns a list of issues.
// An empty slice means the content is valid.
func ValidatePromptContent(content string) []string {
	return validatePromptDoc(content)
}

// --- generate_phase_docs tool ---

type generatePhaseDocsTool struct {
	root string
}

func (t *generatePhaseDocsTool) Name() string { return "generate_phase_docs" }

func (t *generatePhaseDocsTool) Description() string {
	return "Generate phase spec and prompt doc files. Call this when the user has approved the plan."
}

func (t *generatePhaseDocsTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"phases": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"id": {"type": "string"},
					"spec_content": {"type": "string"},
					"prompt_content": {"type": "string"}
				},
				"required": ["id", "spec_content", "prompt_content"]
			}
		}
	}`)
}

type phaseInput struct {
	ID            string `json:"id"`
	SpecContent   string `json:"spec_content"`
	PromptContent string `json:"prompt_content"`
}

func (t *generatePhaseDocsTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Phases []phaseInput `json:"phases"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if len(params.Phases) == 0 {
		return "", fmt.Errorf("at least one phase is required")
	}

	phasesDir := filepath.Join(t.root, "docs", "phases")
	if err := os.MkdirAll(phasesDir, 0o755); err != nil {
		return "", fmt.Errorf("creating phases directory: %w", err)
	}

	var written []string
	var validationWarnings []string

	for _, p := range params.Phases {
		if p.ID == "" {
			return "", fmt.Errorf("phase id is required")
		}

		// Validate before writing.
		if issues := validateSpec(p.SpecContent); len(issues) > 0 {
			validationWarnings = append(validationWarnings,
				fmt.Sprintf("PHASE%s.md: %s", p.ID, strings.Join(issues, "; ")))
		}
		if issues := validatePromptDoc(p.PromptContent); len(issues) > 0 {
			validationWarnings = append(validationWarnings,
				fmt.Sprintf("PHASE%s-PROMPT.md: %s", p.ID, strings.Join(issues, "; ")))
		}

		specPath := filepath.Join(phasesDir, fmt.Sprintf("PHASE%s.md", p.ID))
		promptPath := filepath.Join(phasesDir, fmt.Sprintf("PHASE%s-PROMPT.md", p.ID))

		if err := os.WriteFile(specPath, []byte(p.SpecContent), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", specPath, err)
		}
		written = append(written, specPath)

		if err := os.WriteFile(promptPath, []byte(p.PromptContent), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", promptPath, err)
		}
		written = append(written, promptPath)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Generated %d phase(s), %d file(s) written:\n", len(params.Phases), len(written))
	for _, f := range written {
		// Show relative path from project root.
		rel, err := filepath.Rel(t.root, f)
		if err != nil {
			rel = f
		}
		fmt.Fprintf(&result, "- %s\n", rel)
	}
	if len(validationWarnings) > 0 {
		fmt.Fprintf(&result, "\nValidation warnings:\n")
		for _, w := range validationWarnings {
			fmt.Fprintf(&result, "- %s\n", w)
		}
	}

	return result.String(), nil
}
