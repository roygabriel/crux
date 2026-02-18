package tui

import (
	"strings"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func testSnapshot() *AgentSnapshot {
	return &AgentSnapshot{
		ID:             "agent-1",
		Name:           "Agent One",
		Plugin:         "claude",
		Role:           types.RoleEngineer,
		Status:         types.StatusBusy,
		PromptDisplay:  "P3",
		Task:           "implement auth",
		CommandsPerMin: 12,
		FilesSession:   5,
		Permission:     "write",
		Decisions:      []string{"add tests — coverage was low", "refactor — reduce complexity"},
		WorkNotesInfo:  "Status: In progress\nNext: write integration tests",
	}
}

func TestDetailPanel_Open(t *testing.T) {
	var d DetailPanel
	snap := testSnapshot()
	d.Open(snap)

	if !d.IsVisible() {
		t.Error("expected visible after Open")
	}
	if d.agentID != "agent-1" {
		t.Errorf("agentID = %q, want %q", d.agentID, "agent-1")
	}
	if d.snapshot != snap {
		t.Error("snapshot not stored")
	}
	if d.IsInputMode() {
		t.Error("should not be in input mode after Open")
	}
}

func TestDetailPanel_Open_NilSnap(t *testing.T) {
	var d DetailPanel
	d.Open(nil)
	if d.IsVisible() {
		t.Error("should not be visible after Open(nil)")
	}
}

func TestDetailPanel_Close(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.Close()

	if d.IsVisible() {
		t.Error("expected not visible after Close")
	}
	if d.agentID != "" {
		t.Error("agentID should be empty after Close")
	}
	if d.snapshot != nil {
		t.Error("snapshot should be nil after Close")
	}
	if d.IsInputMode() {
		t.Error("should not be in input mode after Close")
	}
}

func TestDetailPanel_HandleKey_EscCloses(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())

	handled, cmd := d.HandleKey("esc")
	if !handled {
		t.Error("esc should be handled")
	}
	if cmd != nil {
		t.Error("esc should not produce a command")
	}
	if d.IsVisible() {
		t.Error("detail should close on esc")
	}
}

func TestDetailPanel_HandleKey_MEntersInputMode(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())

	handled, cmd := d.HandleKey("m")
	if !handled {
		t.Error("m should be handled")
	}
	if cmd != nil {
		t.Error("m should not produce a command")
	}
	if !d.IsInputMode() {
		t.Error("should be in input mode after m")
	}
}

func TestDetailPanel_HandleKey_SlashEntersInputMode(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())

	handled, _ := d.HandleKey("/")
	if !handled {
		t.Error("/ should be handled")
	}
	if !d.IsInputMode() {
		t.Error("should be in input mode after /")
	}
}

func TestDetailPanel_InputMode_CharAppend(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")

	d.HandleKey("h")
	d.HandleKey("i")

	if d.inputBuffer != "hi" {
		t.Errorf("inputBuffer = %q, want %q", d.inputBuffer, "hi")
	}
}

func TestDetailPanel_InputMode_Backspace(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("a")
	d.HandleKey("b")
	d.HandleKey("c")
	d.HandleKey("backspace")

	if d.inputBuffer != "ab" {
		t.Errorf("inputBuffer = %q, want %q", d.inputBuffer, "ab")
	}
}

func TestDetailPanel_InputMode_BackspaceEmpty(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("backspace")

	if d.inputBuffer != "" {
		t.Errorf("inputBuffer = %q, want empty", d.inputBuffer)
	}
}

func TestDetailPanel_InputMode_EnterSendsMessage(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("h")
	d.HandleKey("e")
	d.HandleKey("l")
	d.HandleKey("p")

	handled, cmd := d.HandleKey("enter")
	if !handled {
		t.Error("enter should be handled")
	}
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	if cmd.Type != CmdSendMessage {
		t.Errorf("cmd.Type = %v, want CmdSendMessage", cmd.Type)
	}
	if cmd.AgentID != "agent-1" {
		t.Errorf("cmd.AgentID = %q, want %q", cmd.AgentID, "agent-1")
	}
	if cmd.Text != "help" {
		t.Errorf("cmd.Text = %q, want %q", cmd.Text, "help")
	}
	if d.inputBuffer != "" {
		t.Error("inputBuffer should be cleared after send")
	}
	if d.sentMsg != "Sent" {
		t.Errorf("sentMsg = %q, want %q", d.sentMsg, "Sent")
	}
	if d.IsInputMode() {
		t.Error("should exit input mode after send")
	}
}

func TestDetailPanel_InputMode_EnterEmptyNoOp(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")

	handled, cmd := d.HandleKey("enter")
	if !handled {
		t.Error("enter should be handled even on empty")
	}
	if cmd != nil {
		t.Error("empty enter should not produce a command")
	}
}

func TestDetailPanel_InputMode_EscCancels(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("a")
	d.HandleKey("b")

	handled, cmd := d.HandleKey("esc")
	if !handled {
		t.Error("esc should be handled")
	}
	if cmd != nil {
		t.Error("esc should not produce a command")
	}
	if d.inputBuffer != "" {
		t.Error("inputBuffer should be cleared after esc")
	}
	if d.IsInputMode() {
		t.Error("should not be in input mode after esc")
	}
	// Panel should still be visible (esc in input mode just cancels input).
	if !d.IsVisible() {
		t.Error("panel should still be visible after input esc")
	}
}

func TestDetailPanel_Update_NilClosesPanel(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.Update(nil)

	if d.IsVisible() {
		t.Error("expected panel to close when updated with nil")
	}
}

func TestDetailPanel_Update_RefreshesSnapshot(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())

	newSnap := &AgentSnapshot{
		ID:     "agent-1",
		Status: types.StatusIdle,
		Task:   "new task",
	}
	d.Update(newSnap)

	if d.snapshot.Task != "new task" {
		t.Errorf("snapshot.Task = %q, want %q", d.snapshot.Task, "new task")
	}
	if d.snapshot.Status != types.StatusIdle {
		t.Errorf("snapshot.Status = %q, want %q", d.snapshot.Status, types.StatusIdle)
	}
}

func TestDetailPanel_View_ContainsAllSections(t *testing.T) {
	var d DetailPanel
	d.SetSize(80, 30)
	d.Open(testSnapshot())

	view := d.View()

	checks := []string{
		"agent-1",
		"claude",
		"engineer",
		"write",
		"busy",
		"implement auth",
		"Recent Decisions:",
		"add tests",
		"refactor",
		"Work Notes:",
		"In progress",
		"write integration tests",
		"Send Message",
	}
	for _, s := range checks {
		if !strings.Contains(view, s) {
			t.Errorf("View() missing %q", s)
		}
	}
}

func TestDetailPanel_View_SentFlash(t *testing.T) {
	var d DetailPanel
	d.SetSize(80, 30)
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("x")
	d.HandleKey("enter")

	view := d.View()
	if !strings.Contains(view, "Sent") {
		t.Error("View() should show 'Sent' flash after message send")
	}
}

func TestDetailPanel_View_EmptyDecisions(t *testing.T) {
	var d DetailPanel
	d.SetSize(80, 30)
	snap := testSnapshot()
	snap.Decisions = nil
	d.Open(snap)

	view := d.View()
	if !strings.Contains(view, "(none)") {
		t.Error("View() should show '(none)' for empty decisions")
	}
}

func TestDetailPanel_View_EmptyWorkNotes(t *testing.T) {
	var d DetailPanel
	d.SetSize(80, 30)
	snap := testSnapshot()
	snap.WorkNotesInfo = ""
	d.Open(snap)

	view := d.View()
	if !strings.Contains(view, "(none)") {
		t.Error("View() should show '(none)' for empty work notes")
	}
}

func TestDetailPanel_View_NilSnapshot(t *testing.T) {
	var d DetailPanel
	view := d.View()
	if view != "" {
		t.Errorf("View() on closed panel = %q, want empty", view)
	}
}

func TestDetailPanel_HandleKey_SwallowsUnknown(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())

	handled, cmd := d.HandleKey("z")
	if !handled {
		t.Error("unknown key should still be handled (swallowed)")
	}
	if cmd != nil {
		t.Error("unknown key should not produce a command")
	}
}

func TestDetailPanel_InputMode_MultiRuneKeyIgnored(t *testing.T) {
	var d DetailPanel
	d.Open(testSnapshot())
	d.HandleKey("m")
	d.HandleKey("tab")

	if d.inputBuffer != "" {
		t.Errorf("inputBuffer = %q, want empty (multi-rune keys should be ignored)", d.inputBuffer)
	}
}
