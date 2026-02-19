package agent

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOutputTee_WriteRawToPaneAndStrippedToLog(t *testing.T) {
	var pane bytes.Buffer
	tee, err := NewOutputTee("engineer-1", t.TempDir(), &pane)
	if err != nil {
		t.Fatalf("NewOutputTee: %v", err)
	}
	defer tee.Close()

	raw := "\x1b[31mred\x1b[0m line\nnext"
	if _, err := tee.Write([]byte(raw)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if pane.String() != raw {
		t.Fatalf("pane output = %q, want %q", pane.String(), raw)
	}

	lines, err := tee.ReadSince(time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0].Content, "red line") {
		t.Fatalf("ANSI not stripped from first line: %q", lines[0].Content)
	}
}

func TestOutputTee_ReadSinceFilters(t *testing.T) {
	tee, err := NewOutputTee("engineer-1", t.TempDir(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("NewOutputTee: %v", err)
	}
	defer tee.Close()

	_, _ = tee.Write([]byte("first\n"))
	cutoff := time.Now().UTC()
	time.Sleep(20 * time.Millisecond)
	_, _ = tee.Write([]byte("second\n"))

	lines, err := tee.ReadSince(cutoff)
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected lines after cutoff")
	}
	if got := lines[len(lines)-1].Content; !strings.Contains(got, "second") {
		t.Fatalf("last content = %q, want second", got)
	}
}

func TestOutputTee_ConcurrentWrites(t *testing.T) {
	tee, err := NewOutputTee("engineer-1", t.TempDir(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("NewOutputTee: %v", err)
	}
	defer tee.Close()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				if _, err := tee.Write([]byte("line\n")); err != nil {
					t.Errorf("Write: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	lines, err := tee.ReadSince(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected logged lines from concurrent writes")
	}
}

func TestOutputTee_ClosePreventsWrites(t *testing.T) {
	tee, err := NewOutputTee("engineer-1", t.TempDir(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("NewOutputTee: %v", err)
	}
	if err := tee.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := tee.Write([]byte("x")); err == nil {
		t.Fatal("expected write error after close")
	}
}
