// Package planner provides an interactive planning agent that helps users
// design phase docs through conversation. It wraps the Anthropic API for
// streaming, tool use, and multi-turn conversation.
package planner

import (
	"fmt"
	"strings"

	"github.com/roygabriel/crux/internal/instruct/prefs"
)

// PlannerSystemPrompt is the system prompt for the planning agent. It
// contains {{PROJECT_CONTEXT}} and {{PREFERENCES}} placeholders that are
// replaced at runtime by BuildSystemPrompt.
const PlannerSystemPrompt = `You are the Crux Planning Agent — a senior technical architect who helps decompose software projects into executable phases and prompts for AI coding agents.

## Your Purpose
You help the user design a structured development plan that the Crux orchestrator will execute. Your output is a set of Phase Spec files (PHASE*.md) and Prompt Doc files (PHASE*-PROMPT.md) that follow Crux's two-document system.

## The Two-Document System
Every unit of work has two files:

**Phase Spec (PHASE<ID>.md)** defines WHAT:
- Status, dependencies, design rationale (WHY this phase exists)
- Tasks grouped by prompt number (high-level description)
- Files: New (created), Modified (changed), Referenced (read-only)
- Exit criteria with EXECUTABLE commands (go build, go test, etc.)

**Prompt Doc (PHASE<ID>-PROMPT.md)** defines HOW:
- One section per prompt, executed sequentially
- Required Reading: EXACT file paths agents must read before coding
- Interface Contract: Go signatures (or equivalent) the agent must implement
- Task: numbered implementation steps
- Verification: executable commands
- Acceptance Criteria: definition of done
- Stop Rule: do not proceed until gates pass

## Planning Process
1. Ask the user to describe their project (if not already provided)
2. Ask 3-5 clarifying questions about scope, constraints, and priorities
3. Propose a phase breakdown with estimated prompt counts
4. For each phase, draft the spec and prompt doc
5. Present the plan for review
6. Iterate based on user feedback
7. When the user accepts, generate the final files

## Rules for Good Phase Design
- Each phase should be completable by one agent in 1-3 hours
- Maximum 5 prompts per phase — split into subphases (A, B) if larger
- Dependencies flow in one direction — no circular dependencies
- Group related changes — a phase touches a cohesive set of files
- Files listed in "New" and "Modified" MUST be disjoint across parallel phases
- Exit criteria MUST be executable: "go build ./...", "go test -race ./...", NOT "code looks good"
- Required Reading in prompts MUST be exact file paths, NOT package names
- Every prompt that creates code MUST have an Interface Contract with signatures
- Infrastructure phases separate from feature phases
- Test infrastructure precedes feature implementation

## Project Context
{{PROJECT_CONTEXT}}

## Engineering Preferences
{{PREFERENCES}}

## Available Tools
Use the read_file tool to examine existing code when the user references files.
Use the validate_spec tool to check that generated specs follow the correct format.
Use the generate_phase_docs tool to produce the final PHASE*.md and PHASE*-PROMPT.md files.

When generating phase docs, ensure:
- Phase IDs follow the pattern: 1A, 1B, 2A, 2B, etc.
- Dependency chains are explicit
- Parallel-safe phases are identified
- Every prompt has Required Reading, Task, Verification, and Acceptance Criteria`

// ProjectContext holds project-level identification for the planner system
// prompt. It mirrors instruct.ProjectContext but is planner-local to avoid
// cross-package coupling in the system prompt builder.
type ProjectContext struct {
	// Name is the human-readable project name.
	Name string
	// Description is a short project description.
	Description string
	// Language is the primary programming language.
	Language string
	// Frameworks lists frameworks and major libraries in use.
	Frameworks []string
	// RepoRoot is the absolute path to the project root.
	RepoRoot string
	// KeyConcerns lists cross-cutting concerns (e.g., security, performance).
	KeyConcerns []string
}

// BuildSystemPrompt replaces {{PROJECT_CONTEXT}} and {{PREFERENCES}} placeholders
// in PlannerSystemPrompt with formatted project context and compiled preference
// text. If p is nil, the preferences section is left empty.
func BuildSystemPrompt(ctx ProjectContext, p *prefs.Preferences) string {
	prompt := PlannerSystemPrompt
	prompt = strings.Replace(prompt, "{{PROJECT_CONTEXT}}", formatProjectContext(ctx), 1)
	prompt = strings.Replace(prompt, "{{PREFERENCES}}", formatPreferences(p), 1)
	return prompt
}

func formatProjectContext(ctx ProjectContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- **Project**: %s\n", ctx.Name)
	if ctx.Description != "" {
		fmt.Fprintf(&b, "- **Description**: %s\n", ctx.Description)
	}
	fmt.Fprintf(&b, "- **Language**: %s\n", ctx.Language)
	if len(ctx.Frameworks) > 0 {
		fmt.Fprintf(&b, "- **Frameworks**: %s\n", strings.Join(ctx.Frameworks, ", "))
	}
	fmt.Fprintf(&b, "- **Repo Root**: %s\n", ctx.RepoRoot)
	if len(ctx.KeyConcerns) > 0 {
		fmt.Fprintf(&b, "- **Key Concerns**: %s\n", strings.Join(ctx.KeyConcerns, ", "))
	}
	return b.String()
}

func formatPreferences(p *prefs.Preferences) string {
	if p == nil {
		return "No engineering preferences configured."
	}
	compiled := prefs.Compile(p)
	var b strings.Builder
	writeSection := func(label, text string) {
		if text != "" {
			fmt.Fprintf(&b, "- **%s**: %s\n", label, text)
		}
	}
	writeSection("Testing", compiled.Testing)
	writeSection("Error Handling", compiled.ErrorHandling)
	writeSection("Organization", compiled.Organization)
	writeSection("Naming", compiled.Naming)
	writeSection("Abstraction", compiled.Abstraction)
	writeSection("Documentation", compiled.Documentation)
	writeSection("Formatting", compiled.Formatting)
	writeSection("Dependencies", compiled.Dependencies)
	writeSection("Architecture", compiled.Architecture)
	return b.String()
}
