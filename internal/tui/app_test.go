package tui

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roygabriel/crux/pkg/types"
)

func TestModel_Init_ReturnsCmd(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
}

func TestModel_KeyQ_Quits(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestModel_KeyTab_CyclesPanels(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	if m.activePanel != PanelSidebar {
		t.Fatalf("initial panel = %d, want PanelSidebar", m.activePanel)
	}

	// Sidebar → Content.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.activePanel != PanelContent {
		t.Errorf("after first tab: panel = %d, want PanelContent", m2.activePanel)
	}

	// Content → Logs.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.activePanel != PanelLogs {
		t.Errorf("after second tab: panel = %d, want PanelLogs", m3.activePanel)
	}

	// Logs → Sidebar.
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyTab})
	m4 := updated.(Model)
	if m4.activePanel != PanelSidebar {
		t.Errorf("after third tab: panel = %d, want PanelSidebar", m4.activePanel)
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)

	if m2.width != 120 {
		t.Errorf("width = %d, want 120", m2.width)
	}
	if m2.height != 40 {
		t.Errorf("height = %d, want 40", m2.height)
	}
	if !m2.ready {
		t.Error("ready should be true after WindowSizeMsg")
	}
}

func TestModel_StateUpdateMsg(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil, nil)

	state := StateUpdate{PhaseName: "build", Timestamp: time.Now()}
	updated, cmd := m.Update(StateUpdateMsg{State: state})
	m2 := updated.(Model)

	if m2.state.PhaseName != "build" {
		t.Errorf("state.PhaseName = %q, want %q", m2.state.PhaseName, "build")
	}
	if cmd == nil {
		t.Error("expected re-subscribe cmd after StateUpdateMsg")
	}
}

func TestModel_LogEntryMsg(t *testing.T) {
	lb := NewLogBridge(8)
	m := NewModel(NewStateBridge(1), lb, nil)

	entry := LogEntry{Time: time.Now(), Level: LogInfo, Message: "hello"}
	updated, cmd := m.Update(LogEntryMsg{Entry: entry})
	m2 := updated.(Model)

	if m2.logsPanel.count != 1 {
		t.Errorf("logsPanel.count = %d, want 1", m2.logsPanel.count)
	}
	if cmd == nil {
		t.Error("expected re-subscribe cmd after LogEntryMsg")
	}
}

func TestModel_View_NotReady(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	view := m.View()
	if !strings.Contains(view, "Initializing...") {
		t.Errorf("View() = %q, want 'Initializing...'", view)
	}
}

func TestModel_SidebarKeyRouting(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	m := NewModel(NewStateBridge(1), nil, bus)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})
	m.sidebar.SetFocused(true)
	m.activePanel = PanelSidebar

	// Press 's' to resume the stopped agent.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = updated.(Model)

	select {
	case cmd := <-bus.Receive():
		if cmd.Type != CmdResumeAgent {
			t.Errorf("Type = %v, want CmdResumeAgent", cmd.Type)
		}
		if cmd.AgentID != "a1" {
			t.Errorf("AgentID = %q, want %q", cmd.AgentID, "a1")
		}
	default:
		t.Fatal("expected command on bus after 's' on stopped agent")
	}
}

func TestModel_ContentPanelKeyRouting(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, PaneContent: "output here"},
	})
	m.activePanel = PanelContent
	m.contentPanel.SetFocused(true)
	m.contentPanel.SetAgent(m.sidebar.SelectedAgent())

	// Press 'd' to switch to details tab.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m2 := updated.(Model)

	if m2.contentPanel.ActiveTab() != TabDetails {
		t.Errorf("tab = %d, want TabDetails", m2.contentPanel.ActiveTab())
	}
}

func TestModel_LogPanelKeysAfterTab(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.logsPanel = NewLogsPanel(100)
	for i := 0; i < 20; i++ {
		m.logsPanel.Append(LogEntry{Time: time.Now(), Level: LogInfo, Message: "line"})
	}
	m.logsPanel.SetSize(80, 10)

	// Tab to Content, then to Logs.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.activePanel != PanelLogs {
		t.Fatalf("expected PanelLogs, got %d", m3.activePanel)
	}

	// Press 'k' to scroll up.
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m4 := updated.(Model)
	if m4.logsPanel.scrollPos == 0 {
		t.Error("expected scroll position to change after 'k' in logs panel")
	}
}

func TestModel_QuitDuringConfirmation(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.activePanel = PanelSidebar

	// Enter kill confirmation.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 := updated.(Model)
	if !m2.sidebar.IsConfirming() {
		t.Fatal("expected confirming state after 'x'")
	}

	// Press 'q' should still quit.
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd even during confirmation")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestModel_NilCommandBus(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})
	m.activePanel = PanelSidebar

	// Press 's' with nil commandBus — should not panic.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = updated.(Model)
}

func TestModel_SidebarSelectionUpdatesContent(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, PaneContent: "content-a1"},
		{ID: "a2", Status: types.StatusIdle, PaneContent: "content-a2"},
	})
	m.activePanel = PanelSidebar
	m.sidebar.SetFocused(true)
	m.syncContentWithSidebar()

	if m.contentPanel.agentID() != "a1" {
		t.Errorf("content agent = %q, want a1", m.contentPanel.agentID())
	}

	// Move cursor down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := updated.(Model)

	if m2.contentPanel.agentID() != "a2" {
		t.Errorf("content agent = %q after j, want a2", m2.contentPanel.agentID())
	}
}

func TestModel_StateUpdateRefreshesContent(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, Task: "old task"},
	})
	m.syncContentWithSidebar()

	// State update with new task.
	newState := StateUpdate{
		Agents: []AgentSnapshot{
			{ID: "a1", Status: types.StatusIdle, Task: "new task"},
		},
		Timestamp: time.Now(),
	}
	updated, _ := m.Update(StateUpdateMsg{State: newState})
	m2 := updated.(Model)

	if m2.contentPanel.agent == nil {
		t.Fatal("content panel agent should not be nil")
	}
	if m2.contentPanel.agent.Task != "new task" {
		t.Errorf("agent.Task = %q, want %q", m2.contentPanel.agent.Task, "new task")
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)
	if !m2.helpOverlay.IsVisible() {
		t.Error("help should be visible after '?'")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m3 := updated.(Model)
	if m3.helpOverlay.IsVisible() {
		t.Error("help should be hidden after second '?'")
	}
}

func TestModel_AnyKeyDismissesHelp(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m3 := updated.(Model)
	if m3.helpOverlay.IsVisible() {
		t.Error("help should be dismissed after pressing 'a'")
	}
}

func TestModel_HelpBlocksOtherKeys(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.activePanel = PanelSidebar

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)

	// Press tab while help is visible — should dismiss help, not switch panels.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.helpOverlay.IsVisible() {
		t.Error("help should be dismissed")
	}
	if m3.activePanel != PanelSidebar {
		t.Error("tab should not switch panels when help was visible")
	}
}

func TestModel_LogPanelFilterModeKeyRouting(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.logsPanel = NewLogsPanel(100)
	now := time.Now()
	for i := 0; i < 10; i++ {
		m.logsPanel.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}
	m.logsPanel.SetSize(80, 10)

	// Switch to Logs panel.
	m.activePanel = PanelLogs
	m.logsPanel.SetFocused(true)

	// Enter filter mode.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	if !m2.logsPanel.IsFilterMode() {
		t.Fatal("filter mode should be active")
	}

	// Type a character.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m3 := updated.(Model)
	if m3.logsPanel.filterInput != "e" {
		t.Errorf("filterInput = %q, want %q", m3.logsPanel.filterInput, "e")
	}
}

func TestModel_TabBlockedDuringFilterMode(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.logsPanel = NewLogsPanel(100)
	m.logsPanel.SetSize(80, 10)
	m.activePanel = PanelLogs
	m.logsPanel.filterMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.activePanel != PanelLogs {
		t.Error("tab should not switch panels during filter mode")
	}
}

func TestModel_TabBlockedDuringInputMode(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.contentPanel.SetAgent(m.sidebar.SelectedAgent())
	m.activePanel = PanelContent
	m.contentPanel.inputMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.activePanel != PanelContent {
		t.Error("tab should not switch panels during input mode")
	}
}

func TestModel_StatusBarInView(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.state.PhaseName = "build"
	m.state.Progress = "3/5"
	m.state.Agents = []AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	}

	view := m.View()
	if !strings.Contains(view, "Session:") {
		t.Error("View should contain status bar with session duration")
	}
}

func TestModel_StatusBarShowsDuration(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.startedAt = time.Now().Add(-14*time.Minute - 32*time.Second)

	bar := m.renderStatusBar()
	if !strings.Contains(bar, "Session: 14m32s") {
		t.Errorf("status bar = %q, want to contain 'Session: 14m32s'", bar)
	}
}

func TestModel_StatusBarShowsPhaseName(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.state.PhaseName = "build"
	m.state.Progress = "2/4"

	bar := m.renderStatusBar()
	if !strings.Contains(bar, "build") {
		t.Errorf("status bar = %q, want to contain 'build'", bar)
	}
	if !strings.Contains(bar, "2/4") {
		t.Errorf("status bar = %q, want to contain '2/4'", bar)
	}
}

func TestModel_QuitSendsCmdShutdown(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	m := NewModel(NewStateBridge(1), nil, bus)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}

	select {
	case c := <-bus.Receive():
		if c.Type != CmdShutdown {
			t.Errorf("Type = %v, want CmdShutdown", c.Type)
		}
	default:
		t.Fatal("expected CmdShutdown on bus")
	}
}

func TestModel_QuitNilBusNoPanic(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestModel_WindowSizeSetsHelpOverlaySize(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)

	if m2.helpOverlay.width != 120 {
		t.Errorf("helpOverlay.width = %d, want 120", m2.helpOverlay.width)
	}
	if m2.helpOverlay.height != 40 {
		t.Errorf("helpOverlay.height = %d, want 40", m2.helpOverlay.height)
	}
}

func TestModel_ContentPanel_SendMessage(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	m := NewModel(NewStateBridge(1), nil, bus)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.syncContentWithSidebar()
	m.activePanel = PanelContent
	m.contentPanel.SetFocused(true)

	// Enter input mode.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m2 := updated.(Model)

	// Type message.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m4 := updated.(Model)

	// Send message.
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated.(Model)

	select {
	case cmd := <-bus.Receive():
		if cmd.Type != CmdSendMessage {
			t.Errorf("Type = %v, want CmdSendMessage", cmd.Type)
		}
		if cmd.AgentID != "a1" {
			t.Errorf("AgentID = %q, want %q", cmd.AgentID, "a1")
		}
		if cmd.Text != "go" {
			t.Errorf("Text = %q, want %q", cmd.Text, "go")
		}
	default:
		t.Fatal("expected command on bus after message send")
	}
}

func TestModel_ThreePanelView(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, PaneContent: "hello world"},
	})
	m.syncContentWithSidebar()

	view := m.View()
	if !strings.Contains(view, "Agents") {
		t.Error("view should contain sidebar header")
	}
	if !strings.Contains(view, "a1") {
		t.Error("view should contain agent ID in sidebar")
	}
	if !strings.Contains(view, "Session:") {
		t.Error("view should contain status bar")
	}
}

func TestModel_ConfirmationInterceptsKeys(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.sidebar.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
		{ID: "a2", Status: types.StatusIdle},
	})
	m.activePanel = PanelSidebar
	m.sidebar.SetFocused(true)

	// Enter kill confirmation.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 := updated.(Model)
	if !m2.sidebar.IsConfirming() {
		t.Fatal("expected confirming state")
	}

	// Tab should be intercepted (no panel switch).
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.activePanel != PanelSidebar {
		t.Error("tab should not switch panels during confirmation")
	}
}

func TestModel_SidebarWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		wantMin   int
		wantMax   int
	}{
		{"narrow terminal", 60, sidebarMinWidth, sidebarMinWidth},
		{"wide terminal", 160, sidebarMinWidth, sidebarMaxWidth},
		{"medium terminal", 100, sidebarMinWidth, sidebarMaxWidth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(NewStateBridge(1), nil, nil)
			m.width = tt.termWidth
			w := m.sidebarWidth()
			if w < tt.wantMin || w > tt.wantMax {
				t.Errorf("sidebarWidth() = %d, want [%d, %d]", w, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestModel_SidebarWidth_UsesCurrentDimensionsForCompact(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.width = 90
	m.height = 40
	// Do not call recalcSizes; sidebarWidth should still use compact math.
	if got := m.sidebarWidth(); got != 27 {
		t.Fatalf("sidebarWidth() = %d, want 27", got)
	}
}
