package instruct

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	claudeMaxBytes      = 15 * 1024
	claudeRuleFrontmatter = "---\ndescription: \"Crux session context: phase state and memory summaries\"\nglob: \"\"\nalwaysApply: true\n---\n"
)

type claudeAdapter struct{}

// CLI returns CLIClaude.
func (a *claudeAdapter) CLI() AgentCLI { return CLIClaude }

// TokenBudget returns the default Claude token budget.
func (a *claudeAdapter) TokenBudget() int { return BudgetClaude }

// ReloadMethod returns ReloadRestart for Claude.
func (a *claudeAdapter) ReloadMethod() ReloadMethod { return ReloadRestart }

// ReloadCommand returns the command to restart a Claude session.
func (a *claudeAdapter) ReloadCommand() string { return "exit\n" }

// PrepareFiles generates CLAUDE.md (primary) and .claude/rules/crux-session.md (rules).
func (a *claudeAdapter) PrepareFiles(rendered *RenderResult, projectRoot string, existingFiles map[string]string) ([]InstructionFile, error) {
	if rendered == nil {
		return nil, fmt.Errorf("rendered result is nil")
	}

	files := make([]InstructionFile, 0, 2)

	// Primary file: CLAUDE.md with marker preservation.
	primaryPath := filepath.Join(projectRoot, "CLAUDE.md")
	existing := existingFiles[primaryPath]
	primaryContent := InsertGenerated(existing, rendered.Content)

	files = append(files, InstructionFile{
		Path:    primaryPath,
		Content: primaryContent,
		Purpose: "primary",
	})

	// Rules file: .claude/rules/crux-session.md with phase + memory sections.
	rulesPath := filepath.Join(projectRoot, ".claude", "rules", "crux-session.md")
	rulesContent := a.buildRulesContent(rendered)

	files = append(files, InstructionFile{
		Path:    rulesPath,
		Content: rulesContent,
		Purpose: "rules",
	})

	return files, nil
}

// ValidateOutput checks for leaked template syntax and oversized content.
func (a *claudeAdapter) ValidateOutput(content string) error {
	if strings.Contains(content, "{{ .") || strings.Contains(content, "[[ .") {
		return fmt.Errorf("leaked template syntax detected in output")
	}
	if len(content) > claudeMaxBytes {
		return fmt.Errorf("content size %d exceeds Claude maximum of %d bytes", len(content), claudeMaxBytes)
	}
	return nil
}

// buildRulesContent extracts phase and memory sections and prepends YAML frontmatter.
func (a *claudeAdapter) buildRulesContent(rendered *RenderResult) string {
	var sb strings.Builder
	sb.WriteString(claudeRuleFrontmatter)

	for _, section := range rendered.Sections {
		if section.Name == "phase" || section.Name == "memory" {
			sb.WriteString(section.Content)
		}
	}

	return sb.String()
}
