package instruct

import (
	"errors"
	"fmt"
)

// ErrUnknownCLI is returned when no adapter exists for a given AgentCLI.
var ErrUnknownCLI = errors.New("unknown agent CLI")

// InstructionFile represents a single file to be written by the distributor.
type InstructionFile struct {
	// Path is the absolute file path.
	Path string `json:"path"`
	// Content is the full file content to write.
	Content string `json:"content"`
	// Purpose describes the file role: "primary", "rules", or "override".
	Purpose string `json:"purpose"`
}

// ReloadMethod describes how an agent CLI picks up instruction changes.
type ReloadMethod string

const (
	// ReloadRestart requires the agent session to be restarted.
	ReloadRestart ReloadMethod = "restart"
	// ReloadNewSession starts a new conversation within the same process.
	ReloadNewSession ReloadMethod = "new_session"
	// ReloadMemoryRefresh triggers an in-session memory refresh command.
	ReloadMemoryRefresh ReloadMethod = "memory_refresh"
	// ReloadNone means the agent has no reload mechanism.
	ReloadNone ReloadMethod = "none"
)

// ValidationWarning is a non-fatal validation issue with generated output.
type ValidationWarning struct {
	Message string
}

// Error implements the error interface.
func (w *ValidationWarning) Error() string {
	return w.Message
}

// AgentAdapter translates RenderResult into CLI-native instruction files.
// Adapters are pure: they do not perform filesystem I/O. The existingFiles
// parameter in PrepareFiles provides current on-disk content, keyed by path.
type AgentAdapter interface {
	// CLI returns the AgentCLI this adapter handles.
	CLI() AgentCLI
	// TokenBudget returns the default token budget for this CLI.
	TokenBudget() int
	// PrepareFiles translates rendered output into instruction files.
	// existingFiles maps file paths to their current on-disk content.
	PrepareFiles(rendered *RenderResult, projectRoot string, existingFiles map[string]string) ([]InstructionFile, error)
	// ReloadMethod returns how this CLI picks up instruction changes.
	ReloadMethod() ReloadMethod
	// ReloadCommand returns the command string to trigger a reload.
	ReloadCommand() string
	// ValidateOutput checks generated content for issues. Returns nil if
	// valid, *ValidationWarning for non-fatal issues, or an error for
	// fatal problems.
	ValidateOutput(content string) error
}

// AdapterForCLI returns the AgentAdapter for the given CLI.
func AdapterForCLI(cli AgentCLI) (AgentAdapter, error) {
	switch cli {
	case CLIClaude:
		return &claudeAdapter{}, nil
	case CLICodex:
		return &codexAdapter{}, nil
	case CLIGemini:
		return &geminiAdapter{}, nil
	case CLICopilot:
		return &copilotAdapter{}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownCLI, cli)
	}
}
