package phase

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// PromptContract is the structured representation of a single prompt
// within a PHASE*-PROMPT.md document.
type PromptContract struct {
	// PhaseID is the phase this prompt belongs to.
	PhaseID types.PhaseID `json:"phase_id"`
	// PromptNumber is the 1-based index of this prompt.
	PromptNumber int `json:"prompt_number"`
	// TotalPrompts is the total count of prompts in this phase.
	TotalPrompts int `json:"total_prompts"`
	// Title is the prompt heading text.
	Title string `json:"title"`
	// RequiredReading lists file paths to read before implementation.
	RequiredReading []string `json:"required_reading,omitempty"`
	// InterfaceContract is the raw Go code block defining the contract.
	InterfaceContract string `json:"interface_contract,omitempty"`
	// Task is the full prose task description.
	Task string `json:"task,omitempty"`
	// Items lists numbered task items extracted from the Task section.
	Items []string `json:"items,omitempty"`
	// Constraints lists implementation constraints.
	Constraints []string `json:"constraints,omitempty"`
	// Verification lists automated verification gates.
	Verification []Gate `json:"verification,omitempty"`
	// Acceptance lists acceptance criteria text.
	Acceptance []string `json:"acceptance,omitempty"`
}

// Prompt heading subsection names.
const (
	promptSubNone              = ""
	promptSubRequiredReading   = "Required Reading"
	promptSubInterfaceContract = "Interface Contract"
	promptSubTask              = "Task"
	promptSubVerification      = "Verification"
	promptSubAcceptance        = "Acceptance Criteria"
	promptSubConstraints       = "Constraints"
)

// promptHeadingRe matches "## Prompt N of M: Title".
var promptHeadingRe = regexp.MustCompile(`^##\s+Prompt\s+(\d+)\s+of\s+(\d+):\s*(.+)`)
var promptHeadingNoTotalRe = regexp.MustCompile(`^##\s+Prompt\s+(\d+)\s*:\s*(.+)`)
var phaseHeaderRe = regexp.MustCompile(`(?i)^phase\s+([a-z0-9]+)\b`)

// annotationRe matches trailing parenthetical annotations like " (repo layout)".
var annotationRe = regexp.MustCompile(`\s*\([^)]+\)\s*$`)

// numberedItemRe matches numbered list items like "1. Do something".
var numberedItemRe = regexp.MustCompile(`^\d+\.\s+(.+)`)

// ParsePromptDoc reads and parses a PHASE*-PROMPT.md file into a slice
// of PromptContract values, one per prompt in the document.
func ParsePromptDoc(path string) ([]PromptContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading prompt doc %s: %w", path, err)
	}
	return parsePromptDoc(string(data))
}

func parsePromptDoc(text string) ([]PromptContract, error) {
	lines := strings.Split(text, "\n")

	var phaseID types.PhaseID
	var prompts []PromptContract
	var current *PromptContract
	currentSub := promptSubNone

	inCodeBlock := false
	codeBlockLang := ""
	var codeBlockLines []string
	var taskLines []string

	flushCurrent := func() {
		if current == nil {
			return
		}
		// Flush pending code block.
		if inCodeBlock {
			flushCodeBlock(current, currentSub, codeBlockLang, codeBlockLines)
			inCodeBlock = false
			codeBlockLines = nil
		}
		// Flush task prose.
		if len(taskLines) > 0 {
			current.Task = strings.TrimSpace(strings.Join(taskLines, "\n"))
			taskLines = nil
		}
		prompts = append(prompts, *current)
		current = nil
		currentSub = promptSubNone
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block fence handling.
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// Closing fence.
				flushCodeBlock(current, currentSub, codeBlockLang, codeBlockLines)
				inCodeBlock = false
				codeBlockLines = nil
				codeBlockLang = ""
				continue
			}
			// Opening fence.
			inCodeBlock = true
			codeBlockLang = strings.TrimPrefix(trimmed, "```")
			codeBlockLines = nil
			continue
		}
		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		// H1: Phase header.
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			rest := strings.TrimPrefix(trimmed, "# ")
			// Match case-insensitive forms like:
			// - "Phase 1A Implementation Prompts"
			// - "PHASE 1A PROMPTS: ..."
			if m := phaseHeaderRe.FindStringSubmatch(rest); m != nil {
				phaseID = types.PhaseID(strings.ToUpper(m[1]))
			}
			continue
		}

		// Separator.
		if trimmed == "---" {
			flushCurrent()
			continue
		}

		// H2: Prompt heading.
		if m := promptHeadingRe.FindStringSubmatch(trimmed); m != nil {
			flushCurrent()
			n := parseNum(m[1])
			total := parseNum(m[2])
			current = &PromptContract{
				PhaseID:      phaseID,
				PromptNumber: n,
				TotalPrompts: total,
				Title:        strings.TrimSpace(m[3]),
			}
			currentSub = promptSubNone
			continue
		}
		if m := promptHeadingNoTotalRe.FindStringSubmatch(trimmed); m != nil {
			flushCurrent()
			n := parseNum(m[1])
			current = &PromptContract{
				PhaseID:      phaseID,
				PromptNumber: n,
				TotalPrompts: 0,
				Title:        strings.TrimSpace(m[2]),
			}
			currentSub = promptSubNone
			continue
		}

		// H3: Subsection.
		if strings.HasPrefix(trimmed, "### ") {
			// Flush task lines when leaving Task section.
			if currentSub == promptSubTask && current != nil && len(taskLines) > 0 {
				current.Task = strings.TrimSpace(strings.Join(taskLines, "\n"))
				taskLines = nil
			}

			sub := strings.TrimPrefix(trimmed, "### ")
			switch {
			case strings.HasPrefix(sub, "Required Reading"):
				currentSub = promptSubRequiredReading
			case strings.HasPrefix(sub, "Interface Contract"):
				currentSub = promptSubInterfaceContract
			case strings.HasPrefix(sub, "Task"):
				currentSub = promptSubTask
			case strings.HasPrefix(sub, "Verification"):
				currentSub = promptSubVerification
			case strings.HasPrefix(sub, "Acceptance"):
				currentSub = promptSubAcceptance
			case strings.HasPrefix(sub, "Constraints"):
				currentSub = promptSubConstraints
			default:
				currentSub = promptSubNone
			}
			continue
		}

		if current == nil {
			continue
		}

		// Dispatch content by subsection.
		switch currentSub {
		case promptSubRequiredReading:
			parseRequiredReadingLine(trimmed, current)
		case promptSubTask:
			taskLines = append(taskLines, line)
			if m := numberedItemRe.FindStringSubmatch(trimmed); m != nil {
				current.Items = append(current.Items, m[1])
			}
		case promptSubAcceptance:
			if strings.HasPrefix(trimmed, "- ") {
				current.Acceptance = append(current.Acceptance, strings.TrimPrefix(trimmed, "- "))
			}
		case promptSubConstraints:
			if strings.HasPrefix(trimmed, "- ") {
				current.Constraints = append(current.Constraints, strings.TrimPrefix(trimmed, "- "))
			}
		}
	}

	// Flush final prompt.
	flushCurrent()

	// Backfill totals when headings omitted "of M".
	if len(prompts) > 0 {
		total := len(prompts)
		for i := range prompts {
			if prompts[i].TotalPrompts == 0 {
				prompts[i].TotalPrompts = total
			}
		}
	}

	return prompts, nil
}

func flushCodeBlock(current *PromptContract, sub string, lang string, lines []string) {
	if current == nil {
		return
	}
	content := strings.Join(lines, "\n")

	switch sub {
	case promptSubInterfaceContract:
		current.InterfaceContract = content
	case promptSubVerification:
		for _, l := range lines {
			cmd := strings.TrimSpace(l)
			if cmd == "" {
				continue
			}
			current.Verification = append(current.Verification, Gate{
				Command:  cmd,
				Expected: "exit 0",
				Type:     GateAutomated,
			})
		}
	}
}

func parseRequiredReadingLine(line string, pc *PromptContract) {
	if !strings.HasPrefix(line, "- ") {
		return
	}
	path := strings.TrimPrefix(line, "- ")
	// Strip trailing parenthetical annotations like " (repo layout)".
	path = annotationRe.ReplaceAllString(path, "")
	path = strings.TrimSpace(path)
	if path != "" {
		pc.RequiredReading = append(pc.RequiredReading, path)
	}
}
