package tui

import (
	"strings"
	"testing"
	"time"
)

func TestLogsPanel_CircularBufferEvicts(t *testing.T) {
	p := NewLogsPanel(3)
	now := time.Now()

	for i := 0; i < 5; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "msg-" + string(rune('a'+i))})
	}

	entries := p.orderedEntries()
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	// Should have the last 3 entries: c, d, e.
	if entries[0].Message != "msg-c" {
		t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "msg-c")
	}
	if entries[2].Message != "msg-e" {
		t.Errorf("entries[2].Message = %q, want %q", entries[2].Message, "msg-e")
	}
}

func TestLogsPanel_OrderedEntriesAfterWrap(t *testing.T) {
	p := NewLogsPanel(4)
	now := time.Now()

	// Fill buffer completely, then add one more to wrap.
	for i := 0; i < 5; i++ {
		p.Append(LogEntry{Time: now.Add(time.Duration(i) * time.Second), Level: LogInfo, Message: "msg-" + string(rune('a'+i))})
	}

	entries := p.orderedEntries()
	if len(entries) != 4 {
		t.Fatalf("entry count = %d, want 4", len(entries))
	}

	// Should be b, c, d, e (oldest evicted was a).
	expected := []string{"msg-b", "msg-c", "msg-d", "msg-e"}
	for i, want := range expected {
		if entries[i].Message != want {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, want)
		}
	}
}

func TestLogsPanel_AutoScrollTracksBottom(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 5)
	now := time.Now()

	for i := 0; i < 20; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}

	if !p.autoScroll {
		t.Error("autoScroll should be true")
	}
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d, want 0", p.scrollPos)
	}
}

func TestLogsPanel_ScrollUpDisengagesAutoScroll(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 5)
	now := time.Now()

	for i := 0; i < 20; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}

	p.ScrollUp(3)
	if p.autoScroll {
		t.Error("autoScroll should be false after ScrollUp")
	}
	if p.scrollPos != 3 {
		t.Errorf("scrollPos = %d, want 3", p.scrollPos)
	}
}

func TestLogsPanel_ScrollDownReengagesAutoScroll(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 5)
	now := time.Now()

	for i := 0; i < 20; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}

	p.ScrollUp(5)
	p.ScrollDown(5)

	if !p.autoScroll {
		t.Error("autoScroll should be true after scrolling back to 0")
	}
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d, want 0", p.scrollPos)
	}
}

func TestLogsPanel_ViewEmpty(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 10)

	view := p.View()
	if !strings.Contains(view, "Waiting for logs...") {
		t.Errorf("empty View() = %q, want 'Waiting for logs...'", view)
	}
}

func TestLogsPanel_HandleKeySlash_EntersFilterMode(t *testing.T) {
	p := NewLogsPanel(10)

	handled, _ := p.HandleKey("/")
	if !handled {
		t.Error("'/' should be handled")
	}
	if !p.filterMode {
		t.Error("filterMode should be true after '/'")
	}
}

func TestLogsPanel_HandleKey_NormalScrollKeys(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 5)
	now := time.Now()
	for i := 0; i < 20; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}

	tests := []struct {
		key      string
		wantPos  int
		wantAuto bool
	}{
		{"k", 1, false},
		{"up", 1, false},
	}

	for _, tt := range tests {
		p2 := NewLogsPanel(100)
		p2.SetSize(80, 5)
		for i := 0; i < 20; i++ {
			p2.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
		}
		handled, _ := p2.HandleKey(tt.key)
		if !handled {
			t.Errorf("key %q should be handled", tt.key)
		}
		if p2.scrollPos != tt.wantPos {
			t.Errorf("key %q: scrollPos = %d, want %d", tt.key, p2.scrollPos, tt.wantPos)
		}
	}
}

func TestLogsPanel_HandleKey_PageScroll(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 5)
	now := time.Now()
	for i := 0; i < 30; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}

	handled, _ := p.HandleKey("pgup")
	if !handled {
		t.Error("pgup should be handled")
	}
	if p.scrollPos != 10 {
		t.Errorf("scrollPos = %d, want 10", p.scrollPos)
	}

	handled, _ = p.HandleKey("pgdown")
	if !handled {
		t.Error("pgdown should be handled")
	}
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d, want 0", p.scrollPos)
	}
}

func TestLogsPanel_HandleKey_UnhandledReturnsFalse(t *testing.T) {
	p := NewLogsPanel(10)

	handled, _ := p.HandleKey("z")
	if handled {
		t.Error("'z' should not be handled in normal mode")
	}
}

func TestLogsPanel_FilterMode_CharAppend(t *testing.T) {
	p := NewLogsPanel(10)
	p.HandleKey("/") // enter filter mode

	p.HandleKey("e")
	p.HandleKey("r")
	p.HandleKey("r")

	if p.filterInput != "err" {
		t.Errorf("filterInput = %q, want %q", p.filterInput, "err")
	}
}

func TestLogsPanel_FilterMode_Backspace(t *testing.T) {
	p := NewLogsPanel(10)
	p.HandleKey("/")
	p.HandleKey("a")
	p.HandleKey("b")
	p.HandleKey("backspace")

	if p.filterInput != "a" {
		t.Errorf("filterInput = %q, want %q", p.filterInput, "a")
	}
}

func TestLogsPanel_FilterMode_BackspaceEmpty(t *testing.T) {
	p := NewLogsPanel(10)
	p.HandleKey("/")
	p.HandleKey("backspace") // no panic on empty

	if p.filterInput != "" {
		t.Errorf("filterInput = %q, want empty", p.filterInput)
	}
}

func TestLogsPanel_FilterMode_EnterApplies(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 10)
	now := time.Now()
	for i := 0; i < 10; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "line"})
	}
	p.ScrollUp(5)

	p.HandleKey("/")
	p.HandleKey("e")
	p.HandleKey("r")
	p.HandleKey("r")
	p.HandleKey("enter")

	if p.filterMode {
		t.Error("filterMode should be false after enter")
	}
	if p.filterActive != "err" {
		t.Errorf("filterActive = %q, want %q", p.filterActive, "err")
	}
	if p.scrollPos != 0 {
		t.Errorf("scrollPos = %d, want 0 after enter", p.scrollPos)
	}
	if !p.autoScroll {
		t.Error("autoScroll should be true after enter")
	}
}

func TestLogsPanel_FilterMode_EscClears(t *testing.T) {
	p := NewLogsPanel(10)
	p.HandleKey("/")
	p.HandleKey("x")
	p.HandleKey("esc")

	if p.filterMode {
		t.Error("filterMode should be false after esc")
	}
	if p.filterInput != "" {
		t.Errorf("filterInput = %q, want empty", p.filterInput)
	}
	if p.filterActive != "" {
		t.Errorf("filterActive = %q, want empty", p.filterActive)
	}
}

func TestFilteredEntries_ByLevel(t *testing.T) {
	entries := []LogEntry{
		{Level: LogInfo, Message: "info msg"},
		{Level: LogError, Message: "err msg"},
		{Level: LogWarn, Message: "warn msg"},
	}

	result := filteredEntries(entries, "error")
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Message != "err msg" {
		t.Errorf("Message = %q, want %q", result[0].Message, "err msg")
	}
}

func TestFilteredEntries_BySource(t *testing.T) {
	entries := []LogEntry{
		{Level: LogInfo, Source: "agent-1", Message: "hello"},
		{Level: LogInfo, Source: "agent-2", Message: "world"},
	}

	result := filteredEntries(entries, "agent-1")
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Source != "agent-1" {
		t.Errorf("Source = %q, want %q", result[0].Source, "agent-1")
	}
}

func TestFilteredEntries_ByMessage_CaseInsensitive(t *testing.T) {
	entries := []LogEntry{
		{Level: LogInfo, Message: "Hello World"},
		{Level: LogInfo, Message: "goodbye"},
	}

	result := filteredEntries(entries, "hello")
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Message != "Hello World" {
		t.Errorf("Message = %q, want %q", result[0].Message, "Hello World")
	}
}

func TestFilteredEntries_NoMatch(t *testing.T) {
	entries := []LogEntry{
		{Level: LogInfo, Message: "hello"},
	}

	result := filteredEntries(entries, "zzz")
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestLogsPanel_ViewWithFilterBadge(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 10)
	now := time.Now()
	p.Append(LogEntry{Time: now, Level: LogInfo, Message: "test"})
	p.filterActive = "err"

	view := p.View()
	if !strings.Contains(view, "filter: err") {
		t.Errorf("View should contain filter badge, got %q", view)
	}
}

func TestLogsPanel_ViewWithFilterInput(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 10)
	now := time.Now()
	p.Append(LogEntry{Time: now, Level: LogInfo, Message: "test"})
	p.filterMode = true
	p.filterInput = "warn"

	view := p.View()
	if !strings.Contains(view, "Filter:") {
		t.Errorf("View should contain filter input, got %q", view)
	}
}

func TestLogsPanel_ViewFilteredReducesEntries(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 20)
	now := time.Now()
	for i := 0; i < 10; i++ {
		p.Append(LogEntry{Time: now, Level: LogInfo, Message: "info line"})
	}
	p.Append(LogEntry{Time: now, Level: LogError, Message: "error line"})

	p.filterActive = "error"
	view := p.View()
	if !strings.Contains(view, "error line") {
		t.Error("filtered view should contain error line")
	}
	if strings.Contains(view, "info line") {
		t.Error("filtered view should not contain info lines")
	}
}

func TestLogsPanel_SlashPreservesExistingFilter(t *testing.T) {
	p := NewLogsPanel(10)
	p.filterActive = "existing"

	p.HandleKey("/")
	if p.filterInput != "existing" {
		t.Errorf("filterInput = %q, want %q", p.filterInput, "existing")
	}
}
