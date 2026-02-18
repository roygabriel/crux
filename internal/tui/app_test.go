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
	// Execute the cmd and check it produces a QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestModel_KeyTab_TogglesPanel(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	if m.activePanel != PanelAgents {
		t.Fatalf("initial panel = %d, want PanelAgents", m.activePanel)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.activePanel != PanelLogs {
		t.Errorf("after tab: panel = %d, want PanelLogs", m2.activePanel)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.activePanel != PanelAgents {
		t.Errorf("after second tab: panel = %d, want PanelAgents", m3.activePanel)
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

func TestModel_AgentPanelKeyRouting(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	m := NewModel(NewStateBridge(1), nil, bus)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents

	// Press 'r' to resume the stopped agent — immediate command, no confirm.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
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
		t.Fatal("expected command on bus after 'r' on stopped agent")
	}
}

func TestModel_LogPanelKeysAfterTab(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.logsPanel = NewLogsPanel(100)
	for i := 0; i < 20; i++ {
		m.logsPanel.Append(LogEntry{Time: time.Now(), Level: LogInfo, Message: "line"})
	}
	m.logsPanel.SetSize(80, 10)

	// Tab to logs panel.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.activePanel != PanelLogs {
		t.Fatalf("expected PanelLogs after tab")
	}

	// Press 'k' (scroll up) should work in logs panel.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m3 := updated.(Model)
	if m3.logsPanel.scrollPos == 0 {
		t.Error("expected scroll position to change after 'k' in logs panel")
	}
}

func TestModel_QuitDuringConfirmation(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.activePanel = PanelAgents

	// Enter kill confirmation.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 := updated.(Model)
	if !m2.agentsPanel.IsConfirming() {
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
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})
	m.activePanel = PanelAgents

	// Press 'r' with nil commandBus — should not panic.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_ = updated.(Model)
}

func TestModel_EnterOpensDetailView(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Plugin: "claude", Status: types.StatusBusy, Task: "build"},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	if !m2.detailPanel.IsVisible() {
		t.Fatal("detail panel should be visible after enter")
	}
	if m2.detailPanel.agentID != "a1" {
		t.Errorf("detailPanel.agentID = %q, want %q", m2.detailPanel.agentID, "a1")
	}
}

func TestModel_EnterNoAgents_NoOp(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	if m2.detailPanel.IsVisible() {
		t.Error("detail panel should not open with no agents")
	}
}

func TestModel_EscInDetailReturnsToAgents(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	if !m2.detailPanel.IsVisible() {
		t.Fatal("detail should be visible")
	}

	// Esc closes it.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(Model)
	if m3.detailPanel.IsVisible() {
		t.Error("detail should close on esc")
	}
}

func TestModel_KeysInDetailDontLeakToAgentsPanel(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
		{ID: "a2", Status: types.StatusIdle},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail on first agent.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	cursorBefore := m2.agentsPanel.cursor

	// Press 'j' (would move cursor in agents panel).
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m3 := updated.(Model)

	if m3.agentsPanel.cursor != cursorBefore {
		t.Error("'j' in detail view should not move agents panel cursor")
	}
}

func TestModel_QuitInDetailView(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	// Press 'q' to quit.
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd in detail view")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestModel_StateUpdateRefreshesDetailView(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, Task: "old task"},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	// State update with new task.
	newState := StateUpdate{
		Agents: []AgentSnapshot{
			{ID: "a1", Status: types.StatusIdle, Task: "new task"},
		},
		Timestamp: time.Now(),
	}
	updated, _ = m2.Update(StateUpdateMsg{State: newState})
	m3 := updated.(Model)

	if !m3.detailPanel.IsVisible() {
		t.Fatal("detail should still be visible")
	}
	if m3.detailPanel.snapshot.Task != "new task" {
		t.Errorf("snapshot.Task = %q, want %q", m3.detailPanel.snapshot.Task, "new task")
	}
}

func TestModel_AgentRemovedClosesDetailView(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	// State update without the agent (killed).
	newState := StateUpdate{
		Agents:    []AgentSnapshot{},
		Timestamp: time.Now(),
	}
	updated, _ = m2.Update(StateUpdateMsg{State: newState})
	m3 := updated.(Model)

	if m3.detailPanel.IsVisible() {
		t.Error("detail should close when agent is removed")
	}
}

func TestModel_TabBlockedDuringDetailView(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	// Tab should not switch panels.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)

	if m3.activePanel != PanelAgents {
		t.Error("tab should not switch panels while detail view is open")
	}
}

func TestModel_DetailView_RendersFullScreen(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil, nil)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Plugin: "claude", Status: types.StatusBusy, Task: "build"},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40
	m.ready = true

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	view := m2.View()
	if !strings.Contains(view, "a1") {
		t.Error("detail view should render agent ID")
	}
	if !strings.Contains(view, "claude") {
		t.Error("detail view should render plugin")
	}
}

func TestModel_DetailView_SendMessage(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	m := NewModel(NewStateBridge(1), nil, bus)
	m.agentsPanel.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})
	m.agentsPanel.SetFocused(true)
	m.activePanel = PanelAgents
	m.width = 120
	m.height = 40

	// Open detail.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)

	// Enter input mode.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3 := updated.(Model)

	// Type message.
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m4 := updated.(Model)
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m5 := updated.(Model)

	// Send message.
	updated, _ = m5.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
