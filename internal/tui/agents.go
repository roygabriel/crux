package tui

import (
	"fmt"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// Column minimum widths for the agent table.
const (
	colAgent  = 15
	colPlugin = 10
	colRole   = 12
	colStatus = 14
	colPrompt = 9
	colCmdM   = 6
	colFiles  = 6
)

// fixedColumnsWidth is the sum of all fixed-width columns.
var fixedColumnsWidth = colAgent + colPlugin + colRole + colStatus + colPrompt + colCmdM + colFiles

// AgentsPanel renders agent status as a table with cursor selection.
type AgentsPanel struct {
	agents        []AgentSnapshot
	focused       bool
	width         int
	height        int
	cursor        int
	confirming    bool
	confirmAction Command
	confirmPrompt string
}

// NewAgentsPanel creates an empty agents panel.
func NewAgentsPanel() AgentsPanel {
	return AgentsPanel{}
}

// SetAgents updates the agent list and clamps the cursor if the list shrank.
func (p *AgentsPanel) SetAgents(agents []AgentSnapshot) {
	p.agents = agents
	if len(agents) == 0 {
		p.cursor = 0
	} else if p.cursor >= len(agents) {
		p.cursor = len(agents) - 1
	}
}

// SetSize sets the panel rendering dimensions.
func (p *AgentsPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// SetFocused sets whether this panel is visually focused.
func (p *AgentsPanel) SetFocused(focused bool) {
	p.focused = focused
}

// SelectedAgent returns the agent at the cursor position, or nil if the list
// is empty.
func (p *AgentsPanel) SelectedAgent() *AgentSnapshot {
	if len(p.agents) == 0 {
		return nil
	}
	return &p.agents[p.cursor]
}

// IsConfirming returns true if a confirmation prompt is active.
func (p *AgentsPanel) IsConfirming() bool {
	return p.confirming
}

// HandleKey processes a key press when the agents panel is focused. It returns
// whether the key was handled and an optional command to send.
func (p *AgentsPanel) HandleKey(key string) (handled bool, cmd *Command) {
	if p.confirming {
		return p.handleConfirmKey(key)
	}
	return p.handleNormalKey(key)
}

// handleConfirmKey processes keys during confirmation mode.
func (p *AgentsPanel) handleConfirmKey(key string) (bool, *Command) {
	switch key {
	case "y":
		cmd := p.confirmAction
		p.confirming = false
		p.confirmPrompt = ""
		return true, &cmd
	case "n", "esc":
		p.confirming = false
		p.confirmPrompt = ""
		return true, nil
	default:
		// Swallow all other keys during confirmation.
		return true, nil
	}
}

// handleNormalKey processes keys in normal (non-confirming) mode.
func (p *AgentsPanel) handleNormalKey(key string) (bool, *Command) {
	switch key {
	case "j", "down":
		if len(p.agents) > 0 && p.cursor < len(p.agents)-1 {
			p.cursor++
		}
		return true, nil

	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
		return true, nil

	case "p":
		agent := p.SelectedAgent()
		if agent == nil || agent.Status == types.StatusStopped {
			return true, nil
		}
		p.confirming = true
		p.confirmAction = Command{Type: CmdPauseAgent, AgentID: agent.ID}
		p.confirmPrompt = fmt.Sprintf("Pause %s? [y/n]", agent.ID)
		return true, nil

	case "r":
		agent := p.SelectedAgent()
		if agent == nil || agent.Status != types.StatusStopped {
			return true, nil
		}
		cmd := Command{Type: CmdResumeAgent, AgentID: agent.ID}
		return true, &cmd

	case "x":
		agent := p.SelectedAgent()
		if agent == nil {
			return true, nil
		}
		p.confirming = true
		p.confirmAction = Command{Type: CmdKillAgent, AgentID: agent.ID}
		p.confirmPrompt = fmt.Sprintf("Kill %s? This cannot be undone. [y/n]", agent.ID)
		return true, nil

	case "enter":
		return true, nil

	default:
		return false, nil
	}
}

// View renders the agent table as a string.
func (p *AgentsPanel) View() string {
	taskWidth := p.width - fixedColumnsWidth
	if taskWidth < 10 {
		taskWidth = 10
	}

	var b strings.Builder

	// Header row.
	header := fmt.Sprintf("%s%s%s%s%s%s%s%s",
		headerStyle.Render(padOrTruncate("AGENT", colAgent)),
		headerStyle.Render(padOrTruncate("PLUGIN", colPlugin)),
		headerStyle.Render(padOrTruncate("ROLE", colRole)),
		headerStyle.Render(padOrTruncate("STATUS", colStatus)),
		headerStyle.Render(padOrTruncate("PROMPT", colPrompt)),
		headerStyle.Render(padOrTruncate("TASK", taskWidth)),
		headerStyle.Render(padOrTruncate("CMD/m", colCmdM)),
		headerStyle.Render(padOrTruncate("FILES", colFiles)),
	)
	b.WriteString(header)
	b.WriteByte('\n')

	// Agent rows.
	for i, a := range p.agents {
		dot := styledStatusDot(a.Status)
		statusLabel := fmt.Sprintf("%s %s", dot, a.Status)

		prompt := a.PromptDisplay
		if prompt == "" {
			prompt = "\u2014"
		}
		task := a.Task
		if task == "" {
			task = "\u2014"
		}

		row := fmt.Sprintf("%s%s%s%s%s%s%s%s",
			padOrTruncate(string(a.ID), colAgent),
			padOrTruncate(a.Plugin, colPlugin),
			padOrTruncate(truncateRole(a.Role, colRole-2), colRole),
			padOrTruncate(statusLabel, colStatus),
			padOrTruncate(prompt, colPrompt),
			padOrTruncate(task, taskWidth),
			padOrTruncate(fmt.Sprintf("%d", a.CommandsPerMin), colCmdM),
			padOrTruncate(fmt.Sprintf("%d", a.FilesSession), colFiles),
		)

		if i == p.cursor && p.focused {
			row = selectedRowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteByte('\n')
	}

	// Confirmation prompt.
	if p.confirming && p.confirmPrompt != "" {
		b.WriteString(confirmStyle.Render(p.confirmPrompt))
		b.WriteByte('\n')
	}

	return b.String()
}

