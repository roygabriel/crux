package tui

import (
	"fmt"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// SidebarPanel renders a compact agent list in the left sidebar with cursor
// selection, pause/resume/kill actions, and confirmation prompts.
type SidebarPanel struct {
	agents        []AgentSnapshot
	focused       bool
	width         int
	height        int
	cursor        int
	confirming    bool
	confirmAction Command
	confirmPrompt string
}

// NewSidebarPanel creates an empty sidebar panel.
func NewSidebarPanel() SidebarPanel {
	return SidebarPanel{}
}

// SetAgents updates the agent list and clamps the cursor if the list shrank.
func (p *SidebarPanel) SetAgents(agents []AgentSnapshot) {
	p.agents = agents
	if len(agents) == 0 {
		p.cursor = 0
	} else if p.cursor >= len(agents) {
		p.cursor = len(agents) - 1
	}
}

// SetSize sets the panel rendering dimensions.
func (p *SidebarPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// SetFocused sets whether this panel is visually focused.
func (p *SidebarPanel) SetFocused(focused bool) {
	p.focused = focused
}

// SelectedAgent returns the agent at the cursor position, or nil if the list
// is empty.
func (p *SidebarPanel) SelectedAgent() *AgentSnapshot {
	if len(p.agents) == 0 {
		return nil
	}
	return &p.agents[p.cursor]
}

// IsConfirming returns true if a confirmation prompt is active.
func (p *SidebarPanel) IsConfirming() bool {
	return p.confirming
}

// HandleKey processes a key press when the sidebar is focused. It returns
// whether the key was handled and an optional command to send.
func (p *SidebarPanel) HandleKey(key string) (handled bool, cmd *Command) {
	if p.confirming {
		return p.handleConfirmKey(key)
	}
	return p.handleNormalKey(key)
}

// handleConfirmKey processes keys during confirmation mode.
func (p *SidebarPanel) handleConfirmKey(key string) (bool, *Command) {
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
		return true, nil
	}
}

// handleNormalKey processes keys in normal (non-confirming) mode.
func (p *SidebarPanel) handleNormalKey(key string) (bool, *Command) {
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

	default:
		return false, nil
	}
}

// View renders the sidebar agent list as a string.
func (p *SidebarPanel) View() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("AGENTS"))
	b.WriteByte('\n')

	nameWidth := p.width - 4 // 2 for dot+space, 2 for padding
	if nameWidth < 4 {
		nameWidth = 4
	}

	for i, a := range p.agents {
		dot := styledStatusDot(a.Status)
		name := padOrTruncate(string(a.ID), nameWidth)
		row := fmt.Sprintf(" %s %s", dot, name)

		if i == p.cursor && p.focused {
			row = selectedRowStyle.Render(row)
		}

		b.WriteString(row)
		b.WriteByte('\n')
	}

	if len(p.agents) == 0 {
		b.WriteString(" (no agents)")
		b.WriteByte('\n')
	}

	// Confirmation prompt.
	if p.confirming && p.confirmPrompt != "" {
		b.WriteByte('\n')
		b.WriteString(confirmStyle.Render(p.confirmPrompt))
	}

	return b.String()
}
