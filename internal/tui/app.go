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
	// PanelSidebar is the left panel showing the agent list.
	PanelSidebar Panel = iota
	// PanelContent is the right top panel showing output/details/decisions.
	PanelContent
	// PanelLogs is the right bottom panel showing the log stream.
	PanelLogs
)

// Sidebar width constraints.
const (
	sidebarMinWidth = 20
	sidebarMaxWidth = 35
	sidebarPct      = 25
)

var (
	focusedBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("141"))
	unfocusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	statusBarStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
)

// Model is the top-level bubbletea model for the TUI dashboard.
type Model struct {
	bridge       *StateBridge
	logBridge    *LogBridge
	commandBus   *CommandBus
	state        StateUpdate
	activePanel  Panel
	sidebar      SidebarPanel
	contentPanel ContentPanel
	logsPanel    LogsPanel
	helpOverlay  HelpOverlay
	width        int
	height       int
	ready        bool
	startedAt    time.Time
}

// NewModel creates a new TUI model connected to the given state, log, and
// command bridges. The commandBus may be nil for read-only mode.
func NewModel(bridge *StateBridge, logBridge *LogBridge, commandBus *CommandBus) Model {
	return Model{
		bridge:       bridge,
		logBridge:    logBridge,
		commandBus:   commandBus,
		activePanel:  PanelSidebar,
		sidebar:      NewSidebarPanel(),
		contentPanel: NewContentPanel(),
		logsPanel:    NewLogsPanel(500),
		startedAt:    time.Now(),
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

		// Priority 4: content panel input mode intercepts all keys.
		if m.contentPanel.IsInputMode() {
			handled, cmd := m.contentPanel.HandleKey(key)
			if handled && cmd != nil && m.commandBus != nil {
				m.commandBus.Send(*cmd)
			}
			return m, nil
		}

		// Priority 5: sidebar confirmation mode intercepts all keys.
		if m.sidebar.IsConfirming() {
			handled, cmd := m.sidebar.HandleKey(key)
			if handled && cmd != nil && m.commandBus != nil {
				m.commandBus.Send(*cmd)
			}
			return m, nil
		}

		// Priority 6: filter mode intercepts all keys.
		if m.logsPanel.IsFilterMode() {
			m.logsPanel.HandleKey(key)
			return m, nil
		}

		// Priority 7: tab cycles panel focus.
		if key == "tab" {
			m.cyclePanel()
			return m, nil
		}

		// Priority 8: panel-specific keys.
		switch m.activePanel {
		case PanelSidebar:
			handled, cmd := m.sidebar.HandleKey(key)
			if handled {
				if cmd != nil && m.commandBus != nil {
					m.commandBus.Send(*cmd)
				}
				// Update content panel when sidebar selection changes.
				m.syncContentWithSidebar()
				return m, nil
			}

		case PanelContent:
			handled, cmd := m.contentPanel.HandleKey(key)
			if handled {
				if cmd != nil && m.commandBus != nil {
					m.commandBus.Send(*cmd)
				}
				return m, nil
			}

		case PanelLogs:
			m.logsPanel.HandleKey(key)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalcSizes()

	case StateUpdateMsg:
		m.state = msg.State
		m.sidebar.SetAgents(msg.State.Agents)

		// Refresh content panel with the selected agent's latest data.
		m.syncContentWithSidebar()

		return m, WaitForUpdate(m.bridge)

	case LogEntryMsg:
		m.logsPanel.Append(msg.Entry)
		return m, WaitForLogEntry(m.logBridge)
	}

	return m, nil
}

// cyclePanel advances focus: Sidebar → Content → Logs → Sidebar.
func (m *Model) cyclePanel() {
	switch m.activePanel {
	case PanelSidebar:
		m.activePanel = PanelContent
	case PanelContent:
		m.activePanel = PanelLogs
	case PanelLogs:
		m.activePanel = PanelSidebar
	}
	m.sidebar.SetFocused(m.activePanel == PanelSidebar)
	m.contentPanel.SetFocused(m.activePanel == PanelContent)
	m.logsPanel.SetFocused(m.activePanel == PanelLogs)
}

// syncContentWithSidebar updates the content panel with the currently selected
// agent from the sidebar.
func (m *Model) syncContentWithSidebar() {
	m.contentPanel.SetAgent(m.sidebar.SelectedAgent())
}

// recalcSizes recomputes panel dimensions after a resize.
func (m *Model) recalcSizes() {
	m.helpOverlay.SetSize(m.width, m.height)

	availableHeight := m.height - 1 // status bar

	sidebarW := m.sidebarWidth()
	rightW := m.width - sidebarW

	// Sidebar inner dimensions (border takes 4 chars width, 4 chars height).
	sidebarInnerW := sidebarW - 4
	if sidebarInnerW < 0 {
		sidebarInnerW = 0
	}
	sidebarInnerH := availableHeight - 4
	if sidebarInnerH < 0 {
		sidebarInnerH = 0
	}
	m.sidebar.SetSize(sidebarInnerW, sidebarInnerH)

	// Right side split: 60% content, 40% logs.
	contentHeight := availableHeight * 60 / 100
	if contentHeight < 6 {
		contentHeight = 6
	}
	logsHeight := availableHeight - contentHeight

	rightInnerW := rightW - 4
	if rightInnerW < 0 {
		rightInnerW = 0
	}
	contentInnerH := contentHeight - 4
	if contentInnerH < 0 {
		contentInnerH = 0
	}
	logsInnerH := logsHeight - 4
	if logsInnerH < 0 {
		logsInnerH = 0
	}

	m.contentPanel.SetSize(rightInnerW, contentInnerH)
	m.logsPanel.SetSize(rightInnerW, logsInnerH)
}

// sidebarWidth returns the sidebar width clamped to min/max.
func (m Model) sidebarWidth() int {
	w := m.width * sidebarPct / 100
	if w < sidebarMinWidth {
		w = sidebarMinWidth
	}
	if w > sidebarMaxWidth {
		w = sidebarMaxWidth
	}
	if w > m.width {
		w = m.width
	}
	return w
}

// View implements tea.Model. It renders a three-panel lazydocker-style layout.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.helpOverlay.IsVisible() {
		return m.helpOverlay.View()
	}

	availableHeight := m.height - 1

	sidebarW := m.sidebarWidth()
	rightW := m.width - sidebarW

	// Inner content dimensions (borders take 4 chars).
	sidebarInnerW := sidebarW - 4
	if sidebarInnerW < 0 {
		sidebarInnerW = 0
	}
	sidebarInnerH := availableHeight - 4
	if sidebarInnerH < 0 {
		sidebarInnerH = 0
	}

	contentHeight := availableHeight * 60 / 100
	if contentHeight < 6 {
		contentHeight = 6
	}
	logsHeight := availableHeight - contentHeight

	rightInnerW := rightW - 4
	if rightInnerW < 0 {
		rightInnerW = 0
	}
	contentInnerH := contentHeight - 4
	if contentInnerH < 0 {
		contentInnerH = 0
	}
	logsInnerH := logsHeight - 4
	if logsInnerH < 0 {
		logsInnerH = 0
	}

	// Border styles per focus state.
	sidebarBorder := unfocusedBorder
	contentBorder := unfocusedBorder
	logsBorder := unfocusedBorder
	switch m.activePanel {
	case PanelSidebar:
		sidebarBorder = focusedBorder
	case PanelContent:
		contentBorder = focusedBorder
	case PanelLogs:
		logsBorder = focusedBorder
	}

	// Render panels.
	sidebarView := sidebarBorder.Width(sidebarInnerW).Height(sidebarInnerH).Render(m.sidebar.View())
	contentView := contentBorder.Width(rightInnerW).Height(contentInnerH).Render(m.contentPanel.View())
	logsView := logsBorder.Width(rightInnerW).Height(logsInnerH).Render(m.logsPanel.View())

	// Compose layout: sidebar | (content / logs).
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, contentView, logsView)
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightColumn)

	return main + "\n" + m.renderStatusBar()
}

// renderStatusBar returns the bottom status bar with phase, agent summary, and session duration.
func (m Model) renderStatusBar() string {
	left := ""
	if m.state.PhaseName != "" {
		left = fmt.Sprintf(" %s", m.state.PhaseName)
		if m.state.Progress != "" {
			left += fmt.Sprintf(" [%s]", m.state.Progress)
		}
	}

	center := agentSummary(m.state.Agents)

	dur := time.Since(m.startedAt).Truncate(time.Second)
	minutes := int(dur.Minutes())
	seconds := int(dur.Seconds()) % 60
	right := fmt.Sprintf("Session: %dm%02ds ", minutes, seconds)

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
