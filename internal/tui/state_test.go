package tui

import (
	"testing"
	"time"
)

func TestStateBridge_PushSubscribe(t *testing.T) {
	b := NewStateBridge(1)
	update := StateUpdate{PhaseName: "test-phase"}

	b.Push(update)

	select {
	case got := <-b.Subscribe():
		if got.PhaseName != "test-phase" {
			t.Errorf("PhaseName = %q, want %q", got.PhaseName, "test-phase")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for update")
	}
}

func TestStateBridge_PushDrainsOldOnFull(t *testing.T) {
	b := NewStateBridge(1)

	b.Push(StateUpdate{PhaseName: "old"})
	b.Push(StateUpdate{PhaseName: "new"})

	select {
	case got := <-b.Subscribe():
		if got.PhaseName != "new" {
			t.Errorf("PhaseName = %q, want %q", got.PhaseName, "new")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for update")
	}
}

func TestNewStateBridge_ZeroClampsToOne(t *testing.T) {
	b := NewStateBridge(0)

	b.Push(StateUpdate{PhaseName: "clamped"})

	select {
	case got := <-b.Subscribe():
		if got.PhaseName != "clamped" {
			t.Errorf("PhaseName = %q, want %q", got.PhaseName, "clamped")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for update")
	}
}

func TestWaitForUpdate_NilBridge(t *testing.T) {
	cmd := WaitForUpdate(nil)
	if cmd != nil {
		t.Error("WaitForUpdate(nil) should return nil")
	}
}
