package tui

import (
	"log/slog"

	"github.com/roygabriel/crux/pkg/types"
)

// CommandType classifies the kind of operator command.
type CommandType int

const (
	// CmdPauseAgent pauses a running agent.
	CmdPauseAgent CommandType = iota
	// CmdResumeAgent resumes a paused agent.
	CmdResumeAgent
	// CmdKillAgent terminates an agent permanently.
	CmdKillAgent
	// CmdForceAdvance skips the current prompt in a phase.
	CmdForceAdvance
	// CmdSendMessage sends a text message to an agent.
	CmdSendMessage
)

// String returns a human-readable label for a CommandType.
func (ct CommandType) String() string {
	switch ct {
	case CmdPauseAgent:
		return "pause"
	case CmdResumeAgent:
		return "resume"
	case CmdKillAgent:
		return "kill"
	case CmdForceAdvance:
		return "force-advance"
	case CmdSendMessage:
		return "send-message"
	default:
		return "unknown"
	}
}

// Command is an operator-issued instruction routed from the TUI to the
// orchestrator via the CommandBus.
type Command struct {
	// Type is the kind of command.
	Type CommandType `json:"type"`
	// AgentID is the target agent for agent-scoped commands.
	AgentID types.AgentID `json:"agent_id,omitempty"`
	// PhaseID is the target phase for phase-scoped commands.
	PhaseID types.PhaseID `json:"phase_id,omitempty"`
	// Text carries free-form content (e.g. message payload).
	Text string `json:"text,omitempty"`
}

// CommandBus is a non-blocking channel for routing operator commands from the
// TUI to the orchestrator. Commands that arrive when the buffer is full are
// dropped with a warning log.
type CommandBus struct {
	ch     chan Command
	logger *slog.Logger
}

// NewCommandBus creates a CommandBus with the given buffer size. A size less
// than 1 is clamped to 1.
func NewCommandBus(bufSize int, logger *slog.Logger) *CommandBus {
	if bufSize < 1 {
		bufSize = 1
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CommandBus{
		ch:     make(chan Command, bufSize),
		logger: logger,
	}
}

// Send enqueues a command. If the buffer is full the command is dropped and a
// warning is logged. Send never blocks.
func (cb *CommandBus) Send(cmd Command) {
	select {
	case cb.ch <- cmd:
	default:
		cb.logger.Warn("command bus full, dropping command",
			"type", cmd.Type.String(),
			"agent_id", cmd.AgentID,
		)
	}
}

// Receive returns a receive-only channel that delivers commands.
func (cb *CommandBus) Receive() <-chan Command {
	return cb.ch
}
