package testutil_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/roygabriel/crux/internal/testutil"
)

func TestMockCommander_ScriptProgression(t *testing.T) {
	cmd := testutil.NewMockCommander("default-content")
	cmd.AddScript("%1", []testutil.ResponseStep{
		{Content: "step-0"},
		{Content: "step-1"},
		{Content: "step-2"},
	})

	ctx := context.Background()

	for i, want := range []string{"step-0", "step-1", "step-2"} {
		got, err := cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if got != want {
			t.Errorf("step %d: got %q, want %q", i, got, want)
		}
	}

	// Exhausted script returns default.
	got, err := cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")
	if err != nil {
		t.Fatalf("default step: unexpected error: %v", err)
	}
	if got != "default-content" {
		t.Errorf("default step: got %q, want %q", got, "default-content")
	}
}

func TestMockCommander_SplitWindow(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	ctx := context.Background()

	got, err := cmd.Run(ctx, "split-window", "-t", "session", "-P", "-F", "#{pane_id}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "%1" {
		t.Errorf("got %q, want %%1", got)
	}
}

func TestMockCommander_RecordsCalls(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	ctx := context.Background()

	_, _ = cmd.Run(ctx, "has-session", "-t", "test")
	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")
	_, _ = cmd.Run(ctx, "send-keys", "-t", "%1", "hello", "Enter")

	calls := cmd.Calls()
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3", len(calls))
	}
	if calls[0].Args[0] != "has-session" {
		t.Errorf("call 0: got %q, want has-session", calls[0].Args[0])
	}
	if calls[1].Args[0] != "capture-pane" {
		t.Errorf("call 1: got %q, want capture-pane", calls[1].Args[0])
	}
	if calls[2].Args[0] != "send-keys" {
		t.Errorf("call 2: got %q, want send-keys", calls[2].Args[0])
	}
}

func TestMockCommander_ErrorStep(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	wantErr := fmt.Errorf("capture failed")
	cmd.SetError("%2", wantErr)

	ctx := context.Background()
	_, err := cmd.Run(ctx, "capture-pane", "-t", "%2", "-p")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != wantErr.Error() {
		t.Errorf("got error %q, want %q", err, wantErr)
	}
}

func TestMockCommander_CallCount(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	ctx := context.Background()

	if cmd.CallCount() != 0 {
		t.Fatalf("initial call count: got %d, want 0", cmd.CallCount())
	}

	_, _ = cmd.Run(ctx, "has-session")
	_, _ = cmd.Run(ctx, "list-panes")

	if cmd.CallCount() != 2 {
		t.Errorf("got %d, want 2", cmd.CallCount())
	}
}

func TestMockCommander_CallsForSubcommand(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	ctx := context.Background()

	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")
	_, _ = cmd.Run(ctx, "send-keys", "-t", "%1", "hello")
	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%2", "-p")

	captures := cmd.CallsForSubcommand("capture-pane")
	if len(captures) != 2 {
		t.Errorf("got %d capture-pane calls, want 2", len(captures))
	}

	sends := cmd.CallsForSubcommand("send-keys")
	if len(sends) != 1 {
		t.Errorf("got %d send-keys calls, want 1", len(sends))
	}
}

func TestMockCommander_Reset(t *testing.T) {
	cmd := testutil.NewMockCommander("default")
	cmd.AddScript("%1", []testutil.ResponseStep{{Content: "a"}})
	ctx := context.Background()
	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")

	cmd.Reset()

	if cmd.CallCount() != 0 {
		t.Errorf("after reset: got %d calls, want 0", cmd.CallCount())
	}

	// Script should be gone — returns default.
	got, _ := cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")
	if got != "default" {
		t.Errorf("after reset: got %q, want %q", got, "default")
	}
}

func TestMockCommander_Verify(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	cmd.AddScript("%1", []testutil.ResponseStep{{Content: "a"}, {Content: "b"}})
	ctx := context.Background()

	// Consume only one step.
	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")

	if err := cmd.Verify(); err == nil {
		t.Error("expected verify error for unconsumed steps")
	}

	// Consume the second step.
	_, _ = cmd.Run(ctx, "capture-pane", "-t", "%1", "-p")

	if err := cmd.Verify(); err != nil {
		t.Errorf("unexpected verify error: %v", err)
	}
}

func TestMockCommander_UnknownPaneReturnsDefault(t *testing.T) {
	cmd := testutil.NewMockCommander("fallback")
	ctx := context.Background()

	got, err := cmd.Run(ctx, "capture-pane", "-t", "%99", "-p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestMockCommander_EmptyArgs(t *testing.T) {
	cmd := testutil.NewMockCommander("")
	ctx := context.Background()

	got, err := cmd.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
