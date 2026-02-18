package instruct

import (
	"fmt"
	"path/filepath"
	"strings"
)

const codexMaxBytes = 32768 // 32 KiB hard limit; Codex silently truncates beyond this.

// codexStableSections are section names whose content changes infrequently.
var codexStableSections = map[string]bool{
	"identity":         true,
	"project":          true,
	"responsibilities": true,
	"constraints":      true,
	"preferences":      true,
}

type codexAdapter struct{}

// CLI returns CLICodex.
func (a *codexAdapter) CLI() AgentCLI { return CLICodex }

// TokenBudget returns the default Codex token budget.
func (a *codexAdapter) TokenBudget() int { return BudgetCodex }

// ReloadMethod returns ReloadNewSession for Codex.
func (a *codexAdapter) ReloadMethod() ReloadMethod { return ReloadNewSession }

// ReloadCommand returns the command to start a new Codex session.
func (a *codexAdapter) ReloadCommand() string { return "/new\n" }

// PrepareFiles generates AGENTS.md (stable sections) and AGENTS.override.md (volatile sections).
func (a *codexAdapter) PrepareFiles(rendered *RenderResult, projectRoot string, existingFiles map[string]string) ([]InstructionFile, error) {
	if rendered == nil {
		return nil, fmt.Errorf("rendered result is nil")
	}

	var stable, volatile strings.Builder

	for _, section := range rendered.Sections {
		if codexStableSections[section.Name] {
			stable.WriteString(section.Content)
		} else {
			volatile.WriteString(section.Content)
		}
	}

	files := []InstructionFile{
		{
			Path:    filepath.Join(projectRoot, "AGENTS.md"),
			Content: stable.String(),
			Purpose: "primary",
		},
		{
			Path:    filepath.Join(projectRoot, "AGENTS.override.md"),
			Content: volatile.String(),
			Purpose: "override",
		},
	}

	return files, nil
}

// ValidateOutput returns an error if content exceeds the Codex 32 KiB limit.
func (a *codexAdapter) ValidateOutput(content string) error {
	if len(content) > codexMaxBytes {
		return fmt.Errorf("content size %d exceeds Codex maximum of %d bytes", len(content), codexMaxBytes)
	}
	return nil
}
