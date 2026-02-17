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
	state       StateUpdate
	activePanel Panel
	agentsPanel AgentsPanel
	width       int
	height      int
	ready       bool
}

// NewModel creates a new TUI model connected to the given state bridge.
func NewModel(bridge *StateBridge) Model {
	return Model{
		bridge:      bridge,
		activePanel: PanelAgents,
		agentsPanel: NewAgentsPanel(),
	}
}

// Init implements tea.Model. It subscribes to state updates.
func (m Model) Init() tea.Cmd {
	return WaitForUpdate(m.bridge)
}

// Update implements tea.Model. It handles keyboard input, window resizing,
// and state update messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.activePanel == PanelAgents {
				m.activePanel = PanelLogs
			} else {
				m.activePanel = PanelAgents
			}
			m.agentsPanel.SetFocused(m.activePanel == PanelAgents)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		agentsHeight := m.height * 40 / 100
		if agentsHeight < 8 {
			agentsHeight = 8
		}
		innerW := m.width - 4
		if innerW < 0 {
			innerW = 0
		}
		agentsInnerH := agentsHeight - 4
		if agentsInnerH < 0 {
			agentsInnerH = 0
		}
		m.agentsPanel.SetSize(innerW, agentsInnerH)

	case StateUpdateMsg:
		m.state = msg.State
		m.agentsPanel.SetAgents(msg.State.Agents)
		return m, WaitForUpdate(m.bridge)
	}

	return m, nil
}

// View implements tea.Model. It renders a two-panel layout with agent status
// on top and logs on the bottom.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
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
	logsContent := renderLogPlaceholder(innerW, logsInnerH)

	top := agentsBorder.Width(innerW).Height(agentsInnerH).Render(agentsContent)
	bottom := logsBorder.Width(innerW).Height(logsInnerH).Render(logsContent)

	return fmt.Sprintf("%s\n%s", top, bottom)
}

func renderLogPlaceholder(w, h int) string {
	return "Logs"
}
