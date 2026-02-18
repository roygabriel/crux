package instruct

import (
	"fmt"
	"path/filepath"
	"strings"
)

type geminiAdapter struct{}

// CLI returns CLIGemini.
func (a *geminiAdapter) CLI() AgentCLI { return CLIGemini }

// TokenBudget returns the default Gemini token budget.
func (a *geminiAdapter) TokenBudget() int { return BudgetGemini }

// ReloadMethod returns ReloadMemoryRefresh for Gemini.
func (a *geminiAdapter) ReloadMethod() ReloadMethod { return ReloadMemoryRefresh }

// ReloadCommand returns the command to trigger a Gemini memory refresh.
func (a *geminiAdapter) ReloadCommand() string { return "/memory refresh\n" }

// PrepareFiles generates a single GEMINI.md file with marker preservation.
func (a *geminiAdapter) PrepareFiles(rendered *RenderResult, projectRoot string, existingFiles map[string]string) ([]InstructionFile, error) {
	if rendered == nil {
		return nil, fmt.Errorf("rendered result is nil")
	}

	primaryPath := filepath.Join(projectRoot, "GEMINI.md")
	existing := existingFiles[primaryPath]
	content := InsertGenerated(existing, rendered.Content)

	return []InstructionFile{
		{
			Path:    primaryPath,
			Content: content,
			Purpose: "primary",
		},
	}, nil
}

// ValidateOutput returns a *ValidationWarning if @./ references are detected,
// which can cause unintended file inclusion in Gemini CLI.
func (a *geminiAdapter) ValidateOutput(content string) error {
	if strings.Contains(content, "@./") {
		return &ValidationWarning{
			Message: "content contains @./ file references which may trigger unintended Gemini file inclusion",
		}
	}
	return nil
}
