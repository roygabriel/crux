package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// Status indicator styles.
var (
	statusDotIdle        = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	statusDotBusy        = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	statusDotError       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	statusDotRateLimited = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // cyan
	statusDotStopped     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	statusDotDefault     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))  // white
	headerStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	selectedRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("236")) // dark gray bg
	confirmStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")) // yellow bold
)

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
			padOrTruncate(truncateRole(a.Role), colRole),
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

// styledStatusDot returns a colored bullet character for the given status.
func styledStatusDot(status types.AgentStatus) string {
	switch status {
	case types.StatusIdle:
		return statusDotIdle.Render("\u25cf")
	case types.StatusBusy:
		return statusDotBusy.Render("\u25cf")
	case types.StatusError:
		return statusDotError.Render("\u25cf")
	case types.StatusRateLimited:
		return statusDotRateLimited.Render("\u25cf")
	case types.StatusStopped:
		return statusDotStopped.Render("\u25cf")
	default:
		return statusDotDefault.Render("\u25cf")
	}
}

// truncateRole shortens long role names for display.
func truncateRole(role types.AgentRole) string {
	s := string(role)
	if len(s) > colRole-2 {
		return s[:colRole-3] + "."
	}
	return s
}

// padOrTruncate pads s with spaces to width, or truncates with "..." if too long.
func padOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > width {
		if width <= 3 {
			return string(runes[:width])
		}
		return string(runes[:width-3]) + "..."
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}
