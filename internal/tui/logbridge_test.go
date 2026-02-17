package tui

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestLogBridge_Handle_ConvertsRecord(t *testing.T) {
	lb := NewLogBridge(8)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	r.AddAttrs(slog.String("agent_id", "eng-1"))

	if err := lb.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	select {
	case entry := <-lb.Subscribe():
		if entry.Message != "test message" {
			t.Errorf("Message = %q, want %q", entry.Message, "test message")
		}
		if entry.Source != "eng-1" {
			t.Errorf("Source = %q, want %q", entry.Source, "eng-1")
		}
		if entry.Level != LogInfo {
			t.Errorf("Level = %d, want LogInfo(%d)", entry.Level, LogInfo)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestLogBridge_NonBlockingOnFull(t *testing.T) {
	lb := NewLogBridge(1)

	// Fill the buffer.
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "first", 0)
	_ = lb.Handle(context.Background(), r)

	// This should not block.
	done := make(chan struct{})
	go func() {
		r2 := slog.NewRecord(time.Now(), slog.LevelInfo, "second", 0)
		_ = lb.Handle(context.Background(), r2)
		close(done)
	}()

	select {
	case <-done:
		// OK, did not block.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Handle blocked on full buffer")
	}
}

func TestAuditToLogEntry_Allowed(t *testing.T) {
	entry := AuditToLogEntry(time.Now(), "eng-1", "file_write", "/src/foo.go", true, "")
	if entry.Level != LogOK {
		t.Errorf("Level = %d, want LogOK(%d)", entry.Level, LogOK)
	}
	if entry.Source != "eng-1" {
		t.Errorf("Source = %q, want %q", entry.Source, "eng-1")
	}
}

func TestAuditToLogEntry_Denied(t *testing.T) {
	entry := AuditToLogEntry(time.Now(), "eng-1", "shell_exec", "rm -rf /", false, "path denied")

	if entry.Level != LogWarn {
		t.Errorf("Level = %d, want LogWarn(%d)", entry.Level, LogWarn)
	}
	if entry.Message == "" {
		t.Fatal("Message should not be empty")
	}
	if len(entry.Message) < 7 || entry.Message[:7] != "DENIED:" {
		t.Errorf("Message = %q, want DENIED: prefix", entry.Message)
	}
}

func TestSlogLevelToLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		want  LogLevel
	}{
		{"debug", slog.LevelDebug, LogInfo},
		{"info", slog.LevelInfo, LogInfo},
		{"warn", slog.LevelWarn, LogWarn},
		{"error", slog.LevelError, LogError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slogLevelToLogLevel(tt.level)
			if got != tt.want {
				t.Errorf("slogLevelToLogLevel(%v) = %d, want %d", tt.level, got, tt.want)
			}
		})
	}
}

func TestLogBridge_Enabled(t *testing.T) {
	lb := NewLogBridge(1)
	if !lb.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled should return true for all levels")
	}
}

func TestWaitForLogEntry_NilBridge(t *testing.T) {
	cmd := WaitForLogEntry(nil)
	if cmd != nil {
		t.Error("WaitForLogEntry(nil) should return nil")
	}
}
