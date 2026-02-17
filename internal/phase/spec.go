// Package phase implements parsing, validation, and execution of phase
// specifications and prompt contracts. It provides the workflow enforcement
// layer that drives prompt-by-prompt progress through project phases.
package phase

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// GateType classifies a verification gate as either automated or human-approval.
type GateType string

const (
	// GateAutomated indicates a gate that can be verified by running a command.
	GateAutomated GateType = "automated"
	// GateHumanApproval indicates a gate requiring manual verification.
	GateHumanApproval GateType = "human-approval"
)

// Gate represents a single verification check in an exit criteria list.
type Gate struct {
	// Command is the shell command to execute (empty for human-approval gates).
	Command string `json:"command,omitempty"`
	// Expected is the expected outcome description.
	Expected string `json:"expected"`
	// Type classifies whether this gate is automated or requires human approval.
	Type GateType `json:"type"`
}

// TaskGroup groups related task items under a numbered prompt.
type TaskGroup struct {
	// PromptNumber is the 1-based prompt number this group belongs to.
	PromptNumber int `json:"prompt_number"`
	// Description is the heading text for this task group.
	Description string `json:"description,omitempty"`
	// Items lists the individual task bullet points.
	Items []string `json:"items"`
}

// PhaseSpec is the structured representation of a PHASE*.md specification file.
type PhaseSpec struct {
	// ID uniquely identifies this phase (e.g., "1A", "2A").
	ID types.PhaseID `json:"id"`
	// Name is the human-readable title.
	Name string `json:"name"`
	// Status is the current lifecycle state.
	Status types.PhaseStatus `json:"status"`
	// DependsOn lists phase IDs that must complete before this phase can start.
	DependsOn []types.PhaseID `json:"depends_on,omitempty"`
	// Rationale explains the design reasoning for this phase.
	Rationale string `json:"rationale,omitempty"`
	// Tasks groups task items by prompt number.
	Tasks []TaskGroup `json:"tasks,omitempty"`
	// FilesNew lists files created by this phase.
	FilesNew []string `json:"files_new,omitempty"`
	// FilesModified lists files modified by this phase.
	FilesModified []string `json:"files_modified,omitempty"`
	// FilesRef lists files referenced (read-only) by this phase.
	FilesRef []string `json:"files_ref,omitempty"`
	// ExitCriteria lists verification gates.
	ExitCriteria []Gate `json:"exit_criteria,omitempty"`
	// ProgressNotes accumulates free-form progress text.
	ProgressNotes []string `json:"progress_notes,omitempty"`
}

// Section names for the spec parser state machine.
const (
	specSectionNone          = ""
	specSectionStatus        = "Status"
	specSectionDependsOn     = "Depends On"
	specSectionRationale     = "Design Rationale"
	specSectionTasks         = "Tasks"
	specSectionFiles         = "Files"
	specSectionExitCriteria  = "Exit Criteria"
	specSectionProgressNotes = "Progress Notes"
)

// File subsection names.
const (
	specSubNone       = ""
	specSubNew        = "New"
	specSubModified   = "Modified"
	specSubReferenced = "Referenced"
)

// backtickCmd extracts backtick-quoted commands from exit criteria lines.
var backtickCmd = regexp.MustCompile("`([^`]+)`")

// promptHeading matches "### Prompt N" headings under the Tasks section.
var promptHeading = regexp.MustCompile(`^###\s+Prompt\s+(\d+)`)

// ParseSpec reads and parses a phase specification markdown file into a PhaseSpec.
func ParseSpec(path string) (*PhaseSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec %s: %w", path, err)
	}
	return parseSpec(string(data))
}

func parseSpec(text string) (*PhaseSpec, error) {
	spec := &PhaseSpec{}
	lines := strings.Split(text, "\n")

	currentSection := specSectionNone
	currentSubSection := specSubNone
	var rationaleLines []string
	var currentTaskGroup *TaskGroup

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// H1: Phase header.
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			rest := strings.TrimPrefix(trimmed, "# ")
			if idx := strings.Index(rest, ": "); idx >= 0 {
				// Extract ID from "Phase 1A: Title" → "1A"
				prefix := rest[:idx]
				spec.Name = rest[idx+2:]
				spec.ID = types.PhaseID(strings.TrimPrefix(prefix, "Phase "))
			}
			continue
		}

		// H2: Section header.
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			// Flush rationale.
			if currentSection == specSectionRationale && len(rationaleLines) > 0 {
				spec.Rationale = strings.TrimSpace(strings.Join(rationaleLines, "\n"))
			}
			// Flush pending task group.
			if currentTaskGroup != nil {
				spec.Tasks = append(spec.Tasks, *currentTaskGroup)
				currentTaskGroup = nil
			}

			section := strings.TrimPrefix(trimmed, "## ")
			currentSection = section
			currentSubSection = specSubNone
			continue
		}

		// H3: Subsection header.
		if strings.HasPrefix(trimmed, "### ") {
			sub := strings.TrimPrefix(trimmed, "### ")

			if currentSection == specSectionTasks {
				// Flush previous task group.
				if currentTaskGroup != nil {
					spec.Tasks = append(spec.Tasks, *currentTaskGroup)
				}
				m := promptHeading.FindStringSubmatch(trimmed)
				if m != nil {
					num := parseNum(m[1])
					currentTaskGroup = &TaskGroup{PromptNumber: num}
				} else {
					currentTaskGroup = &TaskGroup{Description: sub}
				}
				continue
			}

			if currentSection == specSectionFiles {
				switch {
				case strings.HasPrefix(sub, "New"):
					currentSubSection = specSubNew
				case strings.HasPrefix(sub, "Modified"):
					currentSubSection = specSubModified
				case strings.HasPrefix(sub, "Referenced"):
					currentSubSection = specSubReferenced
				default:
					currentSubSection = specSubNone
				}
				continue
			}
		}

		// Parse content based on current section.
		switch currentSection {
		case specSectionStatus:
			if trimmed != "" {
				spec.Status = types.PhaseStatus(strings.ToLower(trimmed))
			}

		case specSectionDependsOn:
			parseDependsOn(trimmed, spec)

		case specSectionRationale:
			rationaleLines = append(rationaleLines, line)

		case specSectionTasks:
			if currentTaskGroup != nil && strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimPrefix(trimmed, "- ")
				currentTaskGroup.Items = append(currentTaskGroup.Items, item)
			}

		case specSectionFiles:
			parseFileLine(trimmed, currentSubSection, spec)

		case specSectionExitCriteria:
			parseExitCriteriaLine(trimmed, spec)

		case specSectionProgressNotes:
			if trimmed != "" {
				spec.ProgressNotes = append(spec.ProgressNotes, trimmed)
			}
		}
	}

	// Flush trailing state.
	if currentSection == specSectionRationale && len(rationaleLines) > 0 {
		spec.Rationale = strings.TrimSpace(strings.Join(rationaleLines, "\n"))
	}
	if currentTaskGroup != nil {
		spec.Tasks = append(spec.Tasks, *currentTaskGroup)
	}

	if len(spec.ExitCriteria) == 0 {
		slog.Warn("phase spec has no exit criteria", "id", spec.ID)
	}

	return spec, nil
}

func parseDependsOn(line string, spec *PhaseSpec) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.EqualFold(trimmed, "none") {
		return
	}
	// Handle bullet lists like "- Phase 1B" or plain "Phase 1B".
	trimmed = strings.TrimPrefix(trimmed, "- ")
	trimmed = strings.TrimPrefix(trimmed, "Phase ")
	if trimmed != "" {
		spec.DependsOn = append(spec.DependsOn, types.PhaseID(trimmed))
	}
}

func parseFileLine(line string, sub string, spec *PhaseSpec) {
	if !strings.HasPrefix(line, "- ") {
		// Handle bare "None" lines.
		if strings.EqualFold(strings.TrimSpace(line), "none") {
			return
		}
		return
	}
	file := strings.TrimPrefix(line, "- ")
	if strings.EqualFold(file, "none") {
		return
	}
	switch sub {
	case specSubNew:
		spec.FilesNew = append(spec.FilesNew, file)
	case specSubModified:
		spec.FilesModified = append(spec.FilesModified, file)
	case specSubReferenced:
		spec.FilesRef = append(spec.FilesRef, file)
	}
}

func parseExitCriteriaLine(line string, spec *PhaseSpec) {
	// Match checkbox lines: "- [ ] ..." or "- [x] ..."
	if !strings.HasPrefix(line, "- [ ] ") && !strings.HasPrefix(line, "- [x] ") {
		return
	}
	content := strings.TrimPrefix(line, "- [ ] ")
	content = strings.TrimPrefix(content, "- [x] ")

	gate := Gate{}
	matches := backtickCmd.FindStringSubmatch(content)
	if matches != nil {
		gate.Command = matches[1]
		gate.Type = GateAutomated
		// Extract expected from the rest of the line after the backtick command.
		rest := backtickCmd.ReplaceAllString(content, "")
		rest = strings.TrimSpace(rest)
		if rest != "" {
			gate.Expected = rest
		} else {
			gate.Expected = "exit 0"
		}
	} else {
		gate.Type = GateHumanApproval
		gate.Expected = content
	}

	spec.ExitCriteria = append(spec.ExitCriteria, gate)
}

func parseNum(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
