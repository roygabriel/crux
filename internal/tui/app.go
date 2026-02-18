package tui

import (
	"fmt"

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
	width       int
	height      int
	ready       bool
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

		// Global keys: always handled regardless of panel or confirmation.
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

		// Detail panel intercepts all other keys when visible.
		if m.detailPanel.IsVisible() {
			handled, cmd := m.detailPanel.HandleKey(key)
			if handled && cmd != nil && m.commandBus != nil {
				m.commandBus.Send(*cmd)
			}
			return m, nil
		}

		// Tab toggles panel focus (only when detail not visible).
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

		// Panel-specific keys.
		if m.activePanel == PanelAgents {
			// Enter opens the detail view for the selected agent.
			if key == "enter" {
				if agent := m.agentsPanel.SelectedAgent(); agent != nil {
					m.detailPanel.Open(agent)
					innerW := m.width - 4
					if innerW < 0 {
						innerW = 0
					}
					innerH := m.height - 4
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
			switch key {
			case "up", "k":
				m.logsPanel.ScrollUp(1)
			case "down", "j":
				m.logsPanel.ScrollDown(1)
			case "pgup":
				m.logsPanel.ScrollUp(10)
			case "pgdown":
				m.logsPanel.ScrollDown(10)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		agentsHeight := m.height * 40 / 100
		if agentsHeight < 8 {
			agentsHeight = 8
		}
		logsHeight := m.height - agentsHeight

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

		detailInnerH := m.height - 4
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

	// Full-screen detail overlay when visible.
	if m.detailPanel.IsVisible() {
		innerW := m.width - 4
		if innerW < 0 {
			innerW = 0
		}
		innerH := m.height - 4
		if innerH < 0 {
			innerH = 0
		}
		content := m.detailPanel.View()
		return focusedBorder.Width(innerW).Height(innerH).Render(content)
	}

	agentsHeight := m.height * 40 / 100
	if agentsHeight < 8 {
		agentsHeight = 8
	}
	logsHeight := m.height - agentsHeight

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

	return fmt.Sprintf("%s\n%s", top, bottom)
}
