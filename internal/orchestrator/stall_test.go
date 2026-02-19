package orchestrator

import (
	"testing"
	"time"
)

func TestStallDetectorThreshold(t *testing.T) {
	d := NewStallDetector(5, nil)
	fp := ProgressFingerprint{
		Timestamp:       time.Now().UTC(),
		FilesExistCount: 1,
		GitDiffHash:     "x",
		TestPassCount:   1,
		LastGateHash:    "g",
	}
	for i := 0; i < 4; i++ {
		fp.Timestamp = time.Now().UTC()
		if d.Record(fp) {
			t.Fatalf("stalled unexpectedly at i=%d", i)
		}
	}
	fp.Timestamp = time.Now().UTC()
	if !d.Record(fp) {
		t.Fatal("expected stalled at 5th identical fingerprint")
	}
}

func TestStallDetectorResetsOnDifferentFingerprint(t *testing.T) {
	d := NewStallDetector(5, nil)
	base := ProgressFingerprint{Timestamp: time.Now().UTC(), FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	for i := 0; i < 4; i++ {
		base.Timestamp = time.Now().UTC()
		d.Record(base)
	}
	diff := base
	diff.GitDiffHash = "y"
	diff.Timestamp = time.Now().UTC()
	if d.Record(diff) {
		t.Fatal("should not be stalled with changed fingerprint")
	}
	for i := 0; i < 4; i++ {
		base.Timestamp = time.Now().UTC()
		if d.Record(base) {
			t.Fatal("should not be stalled after reset sequence")
		}
	}
}

func TestStallDetectorReset(t *testing.T) {
	d := NewStallDetector(3, nil)
	fp := ProgressFingerprint{Timestamp: time.Now().UTC(), FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	d.Record(fp)
	d.Record(fp)
	d.Reset()
	if got := len(d.History()); got != 0 {
		t.Fatalf("history len = %d, want 0", got)
	}
}

func TestStallDetectorDuration(t *testing.T) {
	d := NewStallDetector(3, nil)
	now := time.Now().UTC()
	fp := ProgressFingerprint{FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	fp.Timestamp = now.Add(-3 * time.Second)
	d.Record(fp)
	fp.Timestamp = now.Add(-2 * time.Second)
	d.Record(fp)
	fp.Timestamp = now.Add(-1 * time.Second)
	d.Record(fp)
	if dur := d.StallDuration(); dur <= 0 {
		t.Fatalf("stall duration = %v, want > 0", dur)
	}
}

func TestStallDetectorHistoryCopy(t *testing.T) {
	d := NewStallDetector(2, nil)
	fp := ProgressFingerprint{Timestamp: time.Now().UTC(), FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	d.Record(fp)
	h := d.History()
	h[0].GitDiffHash = "mutated"
	h2 := d.History()
	if h2[0].GitDiffHash != "x" {
		t.Fatal("history should return copy")
	}
}
