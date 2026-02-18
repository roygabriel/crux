package phase

import (
	"strings"
	"text/template"
)

// PromptData holds all data needed to render a prompt template.
type PromptData struct {
	// Role is the agent role executing this prompt.
	Role string `json:"role"`
	// Permission is the agent's permission tier.
	Permission string `json:"permission"`
	// PhaseID is the phase identifier.
	PhaseID string `json:"phase_id"`
	// PhaseName is the human-readable phase title.
	PhaseName string `json:"phase_name"`
	// Title is the prompt heading text.
	Title string `json:"title"`
	// PromptNumber is the 1-based prompt index.
	PromptNumber int `json:"prompt_number"`
	// TotalPrompts is the total count of prompts in this phase.
	TotalPrompts int `json:"total_prompts"`
	// RequiredReading lists file paths to read before implementation.
	RequiredReading []string `json:"required_reading,omitempty"`
	// InterfaceContract is the raw Go code block, may be empty.
	InterfaceContract string `json:"interface_contract,omitempty"`
	// Task is the full prose task description.
	Task string `json:"task,omitempty"`
	// Items lists numbered task items extracted from the task section.
	Items []string `json:"items,omitempty"`
	// Constraints lists implementation constraints.
	Constraints []string `json:"constraints,omitempty"`
	// Acceptance lists acceptance criteria text.
	Acceptance []string `json:"acceptance,omitempty"`
	// Verification lists command strings for verification gates.
	Verification []string `json:"verification,omitempty"`
	// WorkNotes is the pre-rendered work notes context block.
	WorkNotes string `json:"work_notes,omitempty"`
	// Decisions is the pre-rendered prior decisions context block.
	Decisions string `json:"decisions,omitempty"`
	// BankSummary is the pre-rendered memory bank summary.
	BankSummary string `json:"bank_summary,omitempty"`
	// RoleDefinition is the embedded markdown for the agent's role.
	RoleDefinition string `json:"role_definition,omitempty"`
}

// defaultConstraints are always injected into every prompt.
var defaultConstraints = []string{
	"Do not modify files outside the scope of this prompt.",
	"Update work notes after completing the task.",
	"Run all verification commands before considering the task complete.",
}

// BuildPromptData assembles a PromptData from a contract, spec, and context strings.
// It is a pure function with no I/O.
func BuildPromptData(contract PromptContract, spec PhaseSpec, workNotes, decisions, bankSummary, role, permission, roleDefinition string) PromptData {
	// Extract command strings from verification gates.
	var verification []string
	for _, g := range contract.Verification {
		if g.Command != "" {
			verification = append(verification, g.Command)
		}
	}

	// Merge contract constraints with defaults, deduplicating.
	seen := make(map[string]bool, len(contract.Constraints)+len(defaultConstraints))
	var constraints []string
	for _, c := range contract.Constraints {
		if !seen[c] {
			seen[c] = true
			constraints = append(constraints, c)
		}
	}
	for _, c := range defaultConstraints {
		if !seen[c] {
			seen[c] = true
			constraints = append(constraints, c)
		}
	}

	return PromptData{
		Role:              role,
		Permission:        permission,
		PhaseID:           string(spec.ID),
		PhaseName:         spec.Name,
		Title:             contract.Title,
		PromptNumber:      contract.PromptNumber,
		TotalPrompts:      contract.TotalPrompts,
		RequiredReading:   contract.RequiredReading,
		InterfaceContract: contract.InterfaceContract,
		Task:              contract.Task,
		Items:             contract.Items,
		Constraints:       constraints,
		Acceptance:        contract.Acceptance,
		Verification:      verification,
		WorkNotes:         workNotes,
		Decisions:         decisions,
		BankSummary:       bankSummary,
		RoleDefinition:    roleDefinition,
	}
}

// promptTemplate is the compiled text/template for rendering prompts.
var promptTemplate = template.Must(template.New("prompt").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
}).Parse(promptTemplateText))

// RenderPrompt renders a PromptData into the final prompt string.
func RenderPrompt(data PromptData) (string, error) {
	var buf strings.Builder
	if err := promptTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const promptTemplateText = `## Role

You are a **{{.Role}}** with **{{.Permission}}** permissions.
{{if .RoleDefinition}}
### Role Definition

{{.RoleDefinition}}
{{end}}
## Phase {{.PhaseID}}: {{.PhaseName}} — Prompt {{.PromptNumber}} of {{.TotalPrompts}}

### {{.Title}}
{{if .RequiredReading}}
### Required Reading
{{range .RequiredReading}}- {{.}}
{{end}}{{end}}{{if .WorkNotes}}
### Work Notes

{{.WorkNotes}}
{{end}}{{if .Decisions}}
### Prior Decisions

{{.Decisions}}
{{end}}{{if .BankSummary}}
### Memory Bank Summary

{{.BankSummary}}
{{end}}{{if .InterfaceContract}}
### Interface Contract

` + "```go" + `
{{.InterfaceContract}}
` + "```" + `
{{end}}{{if .Task}}
### Task

{{.Task}}
{{end}}{{if .Items}}
### Task Items
{{range $i, $item := .Items}}{{add $i 1}}. {{$item}}
{{end}}{{end}}{{if .Constraints}}
### Constraints
{{range .Constraints}}- {{.}}
{{end}}{{end}}{{if .Verification}}
### Verification
{{range .Verification}}` + "```bash" + `
{{.}}
` + "```" + `
{{end}}{{end}}{{if .Acceptance}}
### Acceptance Criteria
{{range .Acceptance}}- {{.}}
{{end}}{{end}}
### Stop Rule

Do NOT proceed to the next prompt. Stop after completing this prompt's task and passing all verification gates.

### Session Management

After completing the task, update your work notes with what was done, any decisions made, and what comes next.
`
