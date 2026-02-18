package instruct

import (
	"fmt"
	"path/filepath"
)

const copilotMaxBytes = 20 * 1024

type copilotAdapter struct{}

// CLI returns CLICopilot.
func (a *copilotAdapter) CLI() AgentCLI { return CLICopilot }

// TokenBudget returns the default Copilot token budget.
func (a *copilotAdapter) TokenBudget() int { return BudgetCopilot }

// ReloadMethod returns ReloadRestart for Copilot.
func (a *copilotAdapter) ReloadMethod() ReloadMethod { return ReloadRestart }

// ReloadCommand returns the command to restart a Copilot session.
func (a *copilotAdapter) ReloadCommand() string { return "exit\n" }

// PrepareFiles generates a single COPILOT.md file with marker preservation.
func (a *copilotAdapter) PrepareFiles(rendered *RenderResult, projectRoot string, existingFiles map[string]string) ([]InstructionFile, error) {
	if rendered == nil {
		return nil, fmt.Errorf("rendered result is nil")
	}

	primaryPath := filepath.Join(projectRoot, "COPILOT.md")
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

// ValidateOutput returns an error if content exceeds the Copilot size limit.
func (a *copilotAdapter) ValidateOutput(content string) error {
	if len(content) > copilotMaxBytes {
		return fmt.Errorf("content size %d exceeds Copilot maximum of %d bytes", len(content), copilotMaxBytes)
	}
	return nil
}
