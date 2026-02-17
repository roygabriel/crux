package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_Init_ReturnsCmd(t *testing.T) {
	bridge := NewStateBridge(1)
	m := NewModel(bridge, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
}

func TestModel_KeyQ_Quits(t *testing.T) {
	m := NewModel(NewStateBridge(1), nil)
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
	m := NewModel(NewStateBridge(1), nil)
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
	m := NewModel(NewStateBridge(1), nil)

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
	m := NewModel(bridge, nil)

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
	m := NewModel(NewStateBridge(1), lb)

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
	m := NewModel(NewStateBridge(1), nil)
	view := m.View()
	if !strings.Contains(view, "Initializing...") {
		t.Errorf("View() = %q, want 'Initializing...'", view)
	}
}
