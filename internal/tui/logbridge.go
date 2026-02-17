package tui

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LogEntryMsg wraps a LogEntry as a bubbletea message.
type LogEntryMsg struct {
	Entry LogEntry
}

// LogBridge implements slog.Handler and routes log records to a channel for
// consumption by the TUI logs panel.
type LogBridge struct {
	ch    chan LogEntry
	attrs []slog.Attr
	group string
}

// NewLogBridge creates a LogBridge with the given channel buffer size.
func NewLogBridge(bufSize int) *LogBridge {
	if bufSize < 1 {
		bufSize = 1
	}
	return &LogBridge{
		ch: make(chan LogEntry, bufSize),
	}
}

// Subscribe returns a receive-only channel that delivers log entries.
func (lb *LogBridge) Subscribe() <-chan LogEntry {
	return lb.ch
}

// Send directly pushes a LogEntry to the channel without going through slog.
// It is non-blocking: entries are dropped if the buffer is full.
func (lb *LogBridge) Send(entry LogEntry) {
	select {
	case lb.ch <- entry:
	default:
	}
}

// Enabled implements slog.Handler. All levels are enabled.
func (lb *LogBridge) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle implements slog.Handler. It converts a slog.Record to a LogEntry
// and sends it non-blocking to the channel.
func (lb *LogBridge) Handle(_ context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   slogLevelToLogLevel(r.Level),
		Message: r.Message,
	}

	// Extract "agent_id" attribute as Source.
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "agent_id" {
			entry.Source = a.Value.String()
			return false
		}
		return true
	})

	// Also check pre-set attrs from WithAttrs.
	if entry.Source == "" {
		for _, a := range lb.attrs {
			if a.Key == "agent_id" {
				entry.Source = a.Value.String()
				break
			}
		}
	}

	select {
	case lb.ch <- entry:
	default:
	}
	return nil
}

// WithAttrs implements slog.Handler. Returns a new LogBridge sharing the same channel.
func (lb *LogBridge) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(lb.attrs)+len(attrs))
	copy(combined, lb.attrs)
	copy(combined[len(lb.attrs):], attrs)
	return &LogBridge{
		ch:    lb.ch,
		attrs: combined,
		group: lb.group,
	}
}

// WithGroup implements slog.Handler. Returns a new LogBridge sharing the same channel.
func (lb *LogBridge) WithGroup(name string) slog.Handler {
	g := lb.group
	if g != "" {
		g += "." + name
	} else {
		g = name
	}
	return &LogBridge{
		ch:    lb.ch,
		attrs: lb.attrs,
		group: g,
	}
}

// slogLevelToLogLevel maps slog levels to TUI LogLevel values.
func slogLevelToLogLevel(level slog.Level) LogLevel {
	switch {
	case level >= slog.LevelError:
		return LogError
	case level >= slog.LevelWarn:
		return LogWarn
	default:
		return LogInfo
	}
}

// AuditToLogEntry converts audit fields into a LogEntry suitable for the TUI.
// Allowed actions map to LogOK; denied actions map to LogWarn with a "DENIED:" prefix.
func AuditToLogEntry(timestamp time.Time, agentID, action, target string, allowed bool, reason string) LogEntry {
	level := LogOK
	msg := fmt.Sprintf("%s → %s", action, target)
	if !allowed {
		level = LogWarn
		msg = fmt.Sprintf("DENIED: %s → %s", action, target)
		if reason != "" {
			msg += " (" + reason + ")"
		}
	}
	return LogEntry{
		Time:    timestamp,
		Level:   level,
		Message: msg,
		Source:  agentID,
	}
}

// WaitForLogEntry returns a tea.Cmd that blocks on the log bridge channel and
// delivers the next LogEntry as a LogEntryMsg.
func WaitForLogEntry(lb *LogBridge) tea.Cmd {
	if lb == nil {
		return nil
	}
	return func() tea.Msg {
		entry := <-lb.Subscribe()
		return LogEntryMsg{Entry: entry}
	}
}
