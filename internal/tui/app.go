package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/roygabriel/crux/internal/ui/chrome"
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

// Model is the top-level bubbletea model for the TUI dashboard.
type Model struct {
	bridge       *StateBridge
	logBridge    *LogBridge
	commandBus   *CommandBus
	theme        chrome.Theme
	state        StateUpdate
	activePanel  Panel
	sidebar      SidebarPanel
	contentPanel ContentPanel
	logsPanel    LogsPanel
	helpOverlay  HelpOverlay
	confirmForce bool
	compact      bool
	width        int
	height       int
	ready        bool
	startedAt    time.Time
}

// NewModel creates a new TUI model connected to the given state, log, and
// command bridges. The commandBus may be nil for read-only mode.
func NewModel(bridge *StateBridge, logBridge *LogBridge, commandBus *CommandBus) Model {
	sidebar := NewSidebarPanel()
	sidebar.SetFocused(true)
	return Model{
		bridge:       bridge,
		logBridge:    logBridge,
		commandBus:   commandBus,
		theme:        chrome.NewTheme(),
		activePanel:  PanelSidebar,
		sidebar:      sidebar,
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

		// Priority 4: force-advance confirmation intercept.
		if m.confirmForce {
			switch key {
			case "y":
				m.confirmForce = false
				if m.commandBus != nil && m.state.Phase != "" {
					m.commandBus.Send(Command{Type: CmdForceAdvance, PhaseID: m.state.Phase})
				}
			case "n", "esc":
				m.confirmForce = false
			default:
				// Swallow all keys while confirming to prevent accidental actions.
			}
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
		if key == "shift+tab" || key == "backtab" {
			m.cyclePanelBackward()
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
			if key == "a" && m.state.Phase != "" {
				m.confirmForce = true
				return m, nil
			}
			handled, cmd := m.contentPanel.HandleKey(key)
			if handled {
				if cmd != nil && m.commandBus != nil {
					if cmd.Type == CmdForceAdvance && cmd.PhaseID == "" {
						cmd.PhaseID = m.state.Phase
					}
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

// cyclePanelBackward moves focus in reverse order.
func (m *Model) cyclePanelBackward() {
	switch m.activePanel {
	case PanelSidebar:
		m.activePanel = PanelLogs
	case PanelContent:
		m.activePanel = PanelSidebar
	case PanelLogs:
		m.activePanel = PanelContent
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
	m.compact = m.width < 100 || m.height < 30

	availableHeight := m.height - 2 // header + footer legend
	if availableHeight < 3 {
		availableHeight = 3
	}

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
	if m.compact {
		contentHeight = availableHeight * 55 / 100
	}
	if contentHeight < 5 {
		contentHeight = 5
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
	if m.width <= 0 {
		return 0
	}
	if m.compact {
		w := m.width * 30 / 100
		if w < sidebarMinWidth {
			w = sidebarMinWidth
		}
		if w > m.width {
			w = m.width
		}
		return w
	}
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

	availableHeight := m.height - 2
	if availableHeight < 3 {
		availableHeight = 3
	}

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

	// Render panels.
	sidebarView := m.renderPanel("Agents", "fleet", m.sidebar.View(), sidebarInnerW, sidebarInnerH, m.activePanel == PanelSidebar)
	contentView := m.renderPanel("Workspace", m.contentPanel.ActiveTab().String(), m.contentPanel.View(), rightInnerW, contentInnerH, m.activePanel == PanelContent)
	logsView := m.renderPanel("Event Log", m.logsPanel.ModeLabel(), m.logsPanel.View(), rightInnerW, logsInnerH, m.activePanel == PanelLogs)

	// Compose layout: sidebar | (content / logs).
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, contentView, logsView)
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightColumn)

	return m.renderStatusBar() + "\n" + main + "\n" + m.renderLegend()
}

// renderStatusBar returns the bottom status bar with phase, agent summary, and session duration.
func (m Model) renderStatusBar() string {
	left := "Phase: n/a"
	if m.state.PhaseName != "" {
		left = fmt.Sprintf("Phase: %s", m.state.PhaseName)
		if m.state.Progress != "" {
			left += fmt.Sprintf(" [%s]", m.state.Progress)
		}
	}
	if m.state.GatesPassed > 0 || m.state.GatesPending > 0 {
		left += fmt.Sprintf("  Gates %d/%d", m.state.GatesPassed, m.state.GatesPassed+m.state.GatesPending)
	}

	center := agentSummary(m.state.Agents)
	if m.confirmForce {
		center = "Confirm force-advance current phase? y/n"
	}

	dur := time.Since(m.startedAt).Truncate(time.Second)
	minutes := int(dur.Minutes())
	seconds := int(dur.Seconds()) % 60
	right := fmt.Sprintf("Session: %dm%02ds", minutes, seconds)

	return m.theme.RenderHeaderBar(m.width, left, center, right)
}

func (m Model) renderLegend() string {
	if m.confirmForce {
		return m.theme.RenderLegend(m.width, "confirm", []chrome.LegendItem{
			{Key: "y", Action: "force-advance"},
			{Key: "n/esc", Action: "cancel"},
		})
	}

	items := []chrome.LegendItem{
		{Key: "q", Action: "quit"},
		{Key: "?", Action: "help"},
		{Key: "tab/shift+tab", Action: "focus"},
	}

	switch m.activePanel {
	case PanelSidebar:
		items = append(items,
			chrome.LegendItem{Key: "j/k", Action: "move"},
			chrome.LegendItem{Key: "s", Action: "pause/resume"},
			chrome.LegendItem{Key: "x", Action: "kill"},
		)
	case PanelContent:
		items = append(items,
			chrome.LegendItem{Key: "o d n", Action: "tabs"},
			chrome.LegendItem{Key: "i", Action: "message"},
			chrome.LegendItem{Key: "a", Action: "force phase"},
		)
	case PanelLogs:
		items = append(items,
			chrome.LegendItem{Key: "j/k", Action: "scroll"},
			chrome.LegendItem{Key: "/", Action: "filter"},
			chrome.LegendItem{Key: "g/G", Action: "oldest/newest"},
		)
	}

	scope := "panel"
	switch m.activePanel {
	case PanelSidebar:
		scope = "agents"
	case PanelContent:
		scope = "workspace"
	case PanelLogs:
		scope = "logs"
	}
	return m.theme.RenderLegend(m.width, scope, items)
}

func (m Model) renderPanel(title, meta, body string, innerW, innerH int, focused bool) string {
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	header := m.theme.PanelTitle.Render(title)
	if meta != "" {
		header += " " + m.theme.PanelMeta.Render("["+meta+"]")
	}
	content := header + "\n" + body
	return m.theme.PanelStyle(focused).Width(innerW).Height(innerH).Render(content)
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

func (t ContentTab) String() string {
	switch t {
	case TabOutput:
		return "output"
	case TabDetails:
		return "details"
	case TabDecisions:
		return "notes"
	default:
		return "unknown"
	}
}
