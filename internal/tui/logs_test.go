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
