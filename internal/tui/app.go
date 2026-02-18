package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Panel identifies which panel is currently focused.
type Panel int

const (
	// PanelAgents is the top panel showing agent status.
	PanelAgents Panel = iota
	// PanelLogs is the bottom panel showing the log stream.
	PanelLogs
)

var (
	focusedBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("141"))
	unfocusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	statusBarStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
)

// Model is the top-level bubbletea model for the TUI dashboard.
type Model struct {
	bridge      *StateBridge
	logBridge   *LogBridge
	commandBus  *CommandBus
	state       StateUpdate
	activePanel Panel
	agentsPanel AgentsPanel
	logsPanel   LogsPanel
	detailPanel DetailPanel
	helpOverlay HelpOverlay
	width       int
	height      int
	ready       bool
	startedAt   time.Time
}

// NewModel creates a new TUI model connected to the given state, log, and
// command bridges. The commandBus may be nil for read-only mode.
func NewModel(bridge *StateBridge, logBridge *LogBridge, commandBus *CommandBus) Model {
	return Model{
		bridge:      bridge,
		logBridge:   logBridge,
		commandBus:  commandBus,
		activePanel: PanelAgents,
		agentsPanel: NewAgentsPanel(),
		logsPanel:   NewLogsPanel(500),
		startedAt:   time.Now(),
	}
}

// Init implements tea.Model. It subscribes to state updates and log entries.
func (m Model) Init() tea.Cmd {
	return tea.Batch(WaitForUpdate(m.bridge), WaitForLogEntry(m.logBridge))
}

// Update implements tea.Model. It handles keyboard input, window resizing,
// state update messages, and log entry messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Priority 1: quit always works.
		switch key {
		case "q", "ctrl+c":
			if m.commandBus != nil {
				m.commandBus.Send(Command{Type: CmdShutdown})
			}
			return m, tea.Quit
		}

		// Priority 2: dismiss help overlay if visible.
		if m.helpOverlay.IsVisible() {
			m.helpOverlay.Dismiss()
			return m, nil
		}

		// Priority 3: toggle help.
		if key == "?" {
			m.helpOverlay.Toggle()
			return m, nil
		}

		// Priority 4: detail panel intercepts all other keys when visible.
		if m.detailPanel.IsVisible() {
			handled, cmd := m.detailPanel.HandleKey(key)
			if handled && cmd != nil && m.commandBus != nil {
				m.commandBus.Send(*cmd)
			}
			return m, nil
		}

		// Priority 5: filter mode intercepts all keys.
		if m.logsPanel.IsFilterMode() {
			m.logsPanel.HandleKey(key)
			return m, nil
		}

		// Priority 6: tab toggles panel focus.
		if key == "tab" {
			if m.activePanel == PanelAgents {
				m.activePanel = PanelLogs
			} else {
				m.activePanel = PanelAgents
			}
			m.agentsPanel.SetFocused(m.activePanel == PanelAgents)
			m.logsPanel.SetFocused(m.activePanel == PanelLogs)
			return m, nil
		}

		// Priority 7: panel-specific keys.
		if m.activePanel == PanelAgents {
			if key == "enter" {
				if agent := m.agentsPanel.SelectedAgent(); agent != nil {
					m.detailPanel.Open(agent)
					availableHeight := m.height - 1
					innerW := m.width - 4
					if innerW < 0 {
						innerW = 0
					}
					innerH := availableHeight - 4
					if innerH < 0 {
						innerH = 0
					}
					m.detailPanel.SetSize(innerW, innerH)
				}
				return m, nil
			}

			handled, cmd := m.agentsPanel.HandleKey(key)
			if handled {
				if cmd != nil && m.commandBus != nil {
					m.commandBus.Send(*cmd)
				}
				return m, nil
			}
		} else {
			m.logsPanel.HandleKey(key)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		m.helpOverlay.SetSize(m.width, m.height)

		availableHeight := m.height - 1 // reserve 1 line for status bar

		agentsHeight := availableHeight * 40 / 100
		if agentsHeight < 8 {
			agentsHeight = 8
		}
		logsHeight := availableHeight - agentsHeight

		innerW := m.width - 4
		if innerW < 0 {
			innerW = 0
		}
		agentsInnerH := agentsHeight - 4
		if agentsInnerH < 0 {
			agentsInnerH = 0
		}
		logsInnerH := logsHeight - 4
		if logsInnerH < 0 {
			logsInnerH = 0
		}
		m.agentsPanel.SetSize(innerW, agentsInnerH)
		m.logsPanel.SetSize(innerW, logsInnerH)

		detailInnerH := availableHeight - 4
		if detailInnerH < 0 {
			detailInnerH = 0
		}
		m.detailPanel.SetSize(innerW, detailInnerH)

	case StateUpdateMsg:
		m.state = msg.State
		m.agentsPanel.SetAgents(msg.State.Agents)

		// Refresh the detail panel if it's open.
		if m.detailPanel.IsVisible() {
			var found *AgentSnapshot
			for i := range msg.State.Agents {
				if msg.State.Agents[i].ID == m.detailPanel.agentID {
					found = &msg.State.Agents[i]
					break
				}
			}
			m.detailPanel.Update(found)
		}

		return m, WaitForUpdate(m.bridge)

	case LogEntryMsg:
		m.logsPanel.Append(msg.Entry)
		return m, WaitForLogEntry(m.logBridge)
	}

	return m, nil
}

// View implements tea.Model. It renders a two-panel layout with agent status
// on top and logs on the bottom, or a full-screen detail overlay.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Full-screen help overlay (no status bar).
	if m.helpOverlay.IsVisible() {
		return m.helpOverlay.View()
	}

	availableHeight := m.height - 1 // reserve 1 line for status bar

	// Full-screen detail overlay when visible.
	if m.detailPanel.IsVisible() {
		innerW := m.width - 4
		if innerW < 0 {
			innerW = 0
		}
		innerH := availableHeight - 4
		if innerH < 0 {
			innerH = 0
		}
		content := m.detailPanel.View()
		return focusedBorder.Width(innerW).Height(innerH).Render(content) + "\n" + m.renderStatusBar()
	}

	agentsHeight := availableHeight * 40 / 100
	if agentsHeight < 8 {
		agentsHeight = 8
	}
	logsHeight := availableHeight - agentsHeight

	// Inner content dimensions account for border (2 chars each side).
	innerW := m.width - 4
	if innerW < 0 {
		innerW = 0
	}
	agentsInnerH := agentsHeight - 4
	if agentsInnerH < 0 {
		agentsInnerH = 0
	}
	logsInnerH := logsHeight - 4
	if logsInnerH < 0 {
		logsInnerH = 0
	}

	agentsBorder := unfocusedBorder
	logsBorder := unfocusedBorder
	if m.activePanel == PanelAgents {
		agentsBorder = focusedBorder
	} else {
		logsBorder = focusedBorder
	}

	agentsContent := m.agentsPanel.View()
	logsContent := m.logsPanel.View()

	top := agentsBorder.Width(innerW).Height(agentsInnerH).Render(agentsContent)
	bottom := logsBorder.Width(innerW).Height(logsInnerH).Render(logsContent)

	return fmt.Sprintf("%s\n%s\n%s", top, bottom, m.renderStatusBar())
}

// renderStatusBar returns the bottom status bar with phase, agent summary, and session duration.
func (m Model) renderStatusBar() string {
	// Left: phase info.
	left := ""
	if m.state.PhaseName != "" {
		left = fmt.Sprintf(" %s", m.state.PhaseName)
		if m.state.Progress != "" {
			left += fmt.Sprintf(" [%s]", m.state.Progress)
		}
	}

	// Center: agent summary.
	center := agentSummary(m.state.Agents)

	// Right: session duration.
	dur := time.Since(m.startedAt).Truncate(time.Second)
	minutes := int(dur.Minutes())
	seconds := int(dur.Seconds()) % 60
	right := fmt.Sprintf("Session: %dm%02ds ", minutes, seconds)

	// Three-column layout.
	leftWidth := m.width / 3
	centerWidth := m.width / 3
	rightWidth := m.width - leftWidth - centerWidth

	bar := padOrTruncate(left, leftWidth) +
		padOrTruncate(center, centerWidth) +
		padOrTruncate(right, rightWidth)

	return statusBarStyle.Render(bar)
}

// agentSummary returns a compact string like "3 agents: 1 busy, 1 idle, 1 rate-limited".
func agentSummary(agents []AgentSnapshot) string {
	if len(agents) == 0 {
		return "0 agents"
	}

	counts := make(map[string]int)
	for _, a := range agents {
		counts[string(a.Status)]++
	}

	var parts []string
	// Ordered status keys for deterministic output.
	for _, status := range []string{"busy", "idle", "stopped", "error", "rate-limited"} {
		if c, ok := counts[status]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", c, status))
		}
	}

	return fmt.Sprintf("%d agents: %s", len(agents), joinParts(parts))
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
