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
