package plugin

import (
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// Capability describes a functional capability that an agent plugin supports.
type Capability string

const (
	// CapCodeGen indicates the plugin can generate code.
	CapCodeGen Capability = "code-gen"
	// CapFileEdit indicates the plugin can edit existing files.
	CapFileEdit Capability = "file-edit"
	// CapShellExec indicates the plugin can execute shell commands.
	CapShellExec Capability = "shell-exec"
	// CapWebSearch indicates the plugin can search the web.
	CapWebSearch Capability = "web-search"
)

// AgentPlugin is the contract every CLI adapter implements to participate in
// the orchestration system. Each method maps to a specific lifecycle concern:
// launching the agent process, detecting its state from pane output, and
// formatting messages for communication.
type AgentPlugin interface {
	// Name returns the unique identifier for this plugin (e.g. "claude", "codex").
	Name() string

	// LaunchCmd returns the binary and arguments needed to start the agent
	// process with the given configuration.
	LaunchCmd(cfg AgentConfig) (bin string, args []string, err error)

	// DetectReady inspects pane content and returns true if the agent is
	// idle and ready to accept input.
	DetectReady(paneContent string) bool

	// DetectBusy inspects pane content and returns true if the agent is
	// currently processing a task.
	DetectBusy(paneContent string) bool

	// DetectError inspects pane content for error conditions. If an error
	// is detected, it returns the error message and true.
	DetectError(paneContent string) (errMsg string, isError bool)

	// DetectRateLimit inspects pane content for rate-limiting signals.
	// If a rate limit is detected, it returns the suggested retry delay
	// and true.
	DetectRateLimit(paneContent string) (retryAfter time.Duration, isLimited bool)

	// FormatMessage converts a Message into a string suitable for sending
	// to the agent via tmux send-keys.
	FormatMessage(msg types.Message) string

	// ParseOutput extracts structured information from raw pane content
	// captured after the agent finishes processing.
	ParseOutput(paneContent string) (AgentOutput, error)

	// Capabilities returns the set of capabilities this plugin supports.
	Capabilities() []Capability
}

// AgentConfig holds the runtime configuration passed to an agent plugin when
// launching a new agent instance.
type AgentConfig struct {
	// ID is the unique identifier for this agent instance.
	ID types.AgentID `json:"id"`
	// WorkDir is the working directory the agent process should use.
	WorkDir string `json:"work_dir"`
	// Permission is the security tier granted to this agent.
	Permission types.Permission `json:"permission"`
	// ExtraArgs are additional CLI arguments appended to the launch command.
	ExtraArgs []string `json:"extra_args,omitempty"`
	// Environment holds additional environment variables for the agent process.
	Environment map[string]string `json:"environment,omitempty"`
}

// AgentOutput is the structured result of parsing raw pane content from an
// agent after it finishes processing a task.
type AgentOutput struct {
	// Raw is the unprocessed pane content.
	Raw string `json:"raw"`
	// FilesChanged lists file paths that the agent modified.
	FilesChanged []string `json:"files_changed,omitempty"`
	// Decisions records reasoning steps the agent made during execution.
	Decisions []OutputDecision `json:"decisions,omitempty"`
	// Errors collects error messages encountered during execution.
	Errors []string `json:"errors,omitempty"`
	// IsComplete indicates whether the agent finished its task.
	IsComplete bool `json:"is_complete"`
}

// OutputDecision captures a single reasoning step extracted from agent output.
type OutputDecision struct {
	// Decision is a short description of what was decided.
	Decision string `json:"decision"`
	// Rationale explains why the decision was made.
	Rationale string `json:"rationale"`
}
