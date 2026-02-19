package tui

import (
	"strings"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func contentSnapshot() *AgentSnapshot {
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
		PaneContent:    "line 1\nline 2\nline 3\nline 4\nline 5",
	}
}

func TestContentPanel_TabSwitching(t *testing.T) {
	tests := []struct {
		key     string
		wantTab ContentTab
	}{
		{"o", TabOutput},
		{"d", TabDetails},
		{"n", TabDecisions},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			p := NewContentPanel()
			p.SetAgent(contentSnapshot())

			handled, _ := p.HandleKey(tt.key)
			if !handled {
				t.Errorf("key %q should be handled", tt.key)
			}
			if p.ActiveTab() != tt.wantTab {
				t.Errorf("ActiveTab() = %d, want %d", p.ActiveTab(), tt.wantTab)
			}
		})
	}
}

func TestContentPanel_TabSwitchResetsScroll(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 5)
	snap := contentSnapshot()
	snap.PaneContent = strings.Join(make([]string, 30), "\nline")
	p.SetAgent(snap)

	// Scroll down.
	p.HandleKey("j")
	p.HandleKey("j")
	if p.scrollPos == 0 {
		t.Fatal("expected non-zero scroll after j")
	}

	// Switch tab resets.
	p.HandleKey("d")
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d after tab switch, want 0", p.scrollPos)
	}
}

func TestContentPanel_ScrollDownUp(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 5)
	snap := contentSnapshot()
	snap.PaneContent = strings.Join(make([]string, 20), "\nline")
	p.SetAgent(snap)

	p.HandleKey("j")
	if p.scrollPos != 1 {
		t.Errorf("scrollPos = %d after j, want 1", p.scrollPos)
	}

	p.HandleKey("k")
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d after k, want 0", p.scrollPos)
	}

	// Clamp at 0.
	p.HandleKey("k")
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d after extra k, want 0", p.scrollPos)
	}
}

func TestContentPanel_PageScroll(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 5)
	snap := contentSnapshot()
	snap.PaneContent = strings.Join(make([]string, 50), "\nline")
	p.SetAgent(snap)

	p.HandleKey("pgdown")
	if p.scrollPos != 10 {
		t.Errorf("scrollPos = %d after pgdown, want 10", p.scrollPos)
	}

	p.HandleKey("pgup")
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d after pgup, want 0", p.scrollPos)
	}
}

func TestContentPanel_SetAgentResetsScroll(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 5)
	p.SetAgent(contentSnapshot())
	p.HandleKey("j")

	// Change to different agent.
	newSnap := contentSnapshot()
	newSnap.ID = "agent-2"
	p.SetAgent(newSnap)

	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d after agent change, want 0", p.scrollPos)
	}
}

func TestContentPanel_SetAgentSameKeepsScroll(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 5)
	snap := contentSnapshot()
	snap.PaneContent = strings.Join(make([]string, 20), "\nline")
	p.SetAgent(snap)
	p.HandleKey("j")
	pos := p.scrollPos

	// Update same agent.
	updated := contentSnapshot()
	updated.PaneContent = strings.Join(make([]string, 20), "\nline")
	p.SetAgent(updated)

	if p.scrollPos != pos {
		t.Errorf("scrollPos changed from %d to %d for same agent", pos, p.scrollPos)
	}
}

func TestContentPanel_NoAgent(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)

	view := p.View()
	if !strings.Contains(view, "Select an agent") {
		t.Errorf("View() = %q, want to contain 'Select an agent'", view)
	}
}

func TestContentPanel_OutputTab(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	p.SetAgent(contentSnapshot())
	p.HandleKey("o")

	view := p.View()
	if !strings.Contains(view, "line 1") {
		t.Error("output tab should show pane content")
	}
}

func TestContentPanel_OutputTabEmptyContent(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	snap := contentSnapshot()
	snap.PaneContent = ""
	p.SetAgent(snap)
	p.HandleKey("o")

	view := p.View()
	if !strings.Contains(view, "Waiting for output") {
		t.Error("output tab should show waiting message when no content")
	}
}

func TestContentPanel_DetailsTab(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	p.SetAgent(contentSnapshot())
	p.HandleKey("d")

	view := p.View()
	checks := []string{"agent-1", "claude", "engineer", "write", "implement auth"}
	for _, s := range checks {
		if !strings.Contains(view, s) {
			t.Errorf("details tab missing %q", s)
		}
	}
}

func TestContentPanel_DecisionsTab(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 30)
	p.SetAgent(contentSnapshot())
	p.HandleKey("n")

	view := p.View()
	checks := []string{"Recent Decisions", "add tests", "Work Notes", "In progress"}
	for _, s := range checks {
		if !strings.Contains(view, s) {
			t.Errorf("decisions tab missing %q", s)
		}
	}
}

func TestContentPanel_DecisionsTabEmpty(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 30)
	snap := contentSnapshot()
	snap.Decisions = nil
	snap.WorkNotesInfo = ""
	p.SetAgent(snap)
	p.HandleKey("n")

	view := p.View()
	if strings.Count(view, "(none)") < 2 {
		t.Error("expected '(none)' for both empty decisions and work notes")
	}
}

func TestContentPanel_MessageInput(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	p.SetAgent(contentSnapshot())

	// Enter input mode.
	handled, _ := p.HandleKey("i")
	if !handled {
		t.Fatal("i should be handled")
	}
	if !p.IsInputMode() {
		t.Fatal("should be in input mode")
	}

	// Type characters.
	p.HandleKey("h")
	p.HandleKey("i")
	if p.inputBuffer != "hi" {
		t.Errorf("inputBuffer = %q, want %q", p.inputBuffer, "hi")
	}

	// Backspace.
	p.HandleKey("backspace")
	if p.inputBuffer != "h" {
		t.Errorf("inputBuffer = %q after backspace, want %q", p.inputBuffer, "h")
	}

	// Add more and send.
	p.HandleKey("e")
	p.HandleKey("l")
	p.HandleKey("p")
	handled, cmd := p.HandleKey("enter")
	if !handled || cmd == nil {
		t.Fatal("enter should return command")
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
	if p.inputBuffer != "" {
		t.Error("inputBuffer should be cleared after send")
	}
	if p.sentMsg != "Sent" {
		t.Errorf("sentMsg = %q, want %q", p.sentMsg, "Sent")
	}
	if p.IsInputMode() {
		t.Error("should exit input mode after send")
	}
}

func TestContentPanel_MessageInputCancel(t *testing.T) {
	p := NewContentPanel()
	p.SetAgent(contentSnapshot())
	p.HandleKey("i")
	p.HandleKey("a")
	p.HandleKey("b")

	handled, cmd := p.HandleKey("esc")
	if !handled {
		t.Fatal("esc should be handled")
	}
	if cmd != nil {
		t.Error("esc should not return a command")
	}
	if p.inputBuffer != "" {
		t.Error("inputBuffer should be cleared after esc")
	}
	if p.IsInputMode() {
		t.Error("should exit input mode after esc")
	}
}

func TestContentPanel_MessageInputEmptyNoOp(t *testing.T) {
	p := NewContentPanel()
	p.SetAgent(contentSnapshot())
	p.HandleKey("i")

	handled, cmd := p.HandleKey("enter")
	if !handled {
		t.Fatal("enter should be handled")
	}
	if cmd != nil {
		t.Error("empty enter should not produce a command")
	}
}

func TestContentPanel_MessageNoAgentNoOp(t *testing.T) {
	p := NewContentPanel()
	handled, _ := p.HandleKey("i")
	if !handled {
		t.Fatal("i should be handled even without agent")
	}
	if p.IsInputMode() {
		t.Error("should not enter input mode without agent")
	}
}

func TestContentPanel_UnhandledKeys(t *testing.T) {
	p := NewContentPanel()
	p.SetAgent(contentSnapshot())

	handled, cmd := p.HandleKey("z")
	if handled {
		t.Error("z should not be handled")
	}
	if cmd != nil {
		t.Error("unhandled key should not produce a command")
	}
}

func TestContentPanel_MultiRuneKeyIgnored(t *testing.T) {
	p := NewContentPanel()
	p.SetAgent(contentSnapshot())
	p.HandleKey("i")
	p.HandleKey("tab")

	if p.inputBuffer != "" {
		t.Errorf("inputBuffer = %q, want empty (multi-rune keys should be ignored)", p.inputBuffer)
	}
}

func TestContentPanel_ViewTabBar(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	p.SetAgent(contentSnapshot())

	view := p.View()
	if !strings.Contains(view, "Output") {
		t.Error("tab bar should contain 'Output'")
	}
	if !strings.Contains(view, "Details") {
		t.Error("tab bar should contain 'Details'")
	}
	if !strings.Contains(view, "Notes") {
		t.Error("tab bar should contain 'Decisions'")
	}
}

func TestContentPanel_ViewInputLine(t *testing.T) {
	p := NewContentPanel()
	p.SetSize(80, 20)
	p.SetAgent(contentSnapshot())

	view := p.View()
	if !strings.Contains(view, "press i to type") {
		t.Error("view should contain input hint")
	}
}

func TestContentPanel_BackspaceEmpty(t *testing.T) {
	p := NewContentPanel()
	p.SetAgent(contentSnapshot())
	p.HandleKey("i")
	p.HandleKey("backspace")

	if p.inputBuffer != "" {
		t.Errorf("inputBuffer = %q, want empty", p.inputBuffer)
	}
}

func TestContentPanel_AgentID(t *testing.T) {
	p := NewContentPanel()
	if p.agentID() != "" {
		t.Error("agentID should be empty with no agent")
	}
	p.SetAgent(contentSnapshot())
	if p.agentID() != "agent-1" {
		t.Errorf("agentID = %q, want %q", p.agentID(), "agent-1")
	}
}
