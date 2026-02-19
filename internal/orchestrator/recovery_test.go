package orchestrator

import (
	"log/slog"
	"testing"
	"time"
)

func TestRecoveryManager_RecordFailureProgression(t *testing.T) {
	m := NewRecoveryManager(BackoffConfig{}, slog.Default())
	key := makePromptKey("1A", 1)

	if a := m.RecordFailure(key, "engineer-1", "semantic_stall", nil, nil, 0); a != ActionRetry {
		t.Fatalf("attempt1 action = %v, want retry", a)
	}
	if a := m.RecordFailure(key, "engineer-1", "semantic_stall", nil, nil, 0); a != ActionReassign {
		t.Fatalf("attempt2 action = %v, want reassign", a)
	}
	if a := m.RecordFailure(key, "engineer-1", "semantic_stall", nil, nil, 0); a != ActionQuarantine {
		t.Fatalf("attempt3 action = %v, want quarantine", a)
	}
	if !m.IsQuarantined(key) {
		t.Fatal("prompt should be quarantined")
	}
}

func TestRecoveryManagerRetryContextGuidance(t *testing.T) {
	m := NewRecoveryManager(BackoffConfig{}, slog.Default())
	key := makePromptKey("2B", 2)
	e := &FilesystemEvidence{Missing: []string{"go.mod", "main.go"}}
	m.RecordFailure(key, "engineer-1", "gate_failure", e, nil, 0)
	ctx := m.GetRetryContext(key)
	if ctx == nil {
		t.Fatal("expected retry context")
	}
	if ctx.Guidance == "" {
		t.Fatal("expected guidance")
	}
}

func TestRecoveryManagerBackoffIncreases(t *testing.T) {
	m := NewRecoveryManager(BackoffConfig{
		InitialInterval: time.Second,
		Coefficient:     2,
		MaxInterval:     30 * time.Second,
		JitterFraction:  0,
	}, slog.Default())
	key := makePromptKey("1A", 1)
	m.RecordFailure(key, "engineer-1", "x", nil, nil, 0)
	d1 := m.NextBackoffDelay(key)
	m.RecordFailure(key, "engineer-1", "x", nil, nil, 0)
	d2 := m.NextBackoffDelay(key)
	if d2 <= d1 {
		t.Fatalf("expected d2 > d1, got %v <= %v", d2, d1)
	}
}

func TestRecoveryManagerQuarantineRecommendation(t *testing.T) {
	m := NewRecoveryManager(BackoffConfig{}, slog.Default())
	key := makePromptKey("3A", 4)
	m.RecordFailure(key, "engineer-1", "x", nil, nil, 0)
	m.RecordFailure(key, "engineer-1", "x", nil, nil, 0)
	m.RecordFailure(key, "engineer-1", "x", nil, nil, 0)
	q := m.GetQuarantineMetadata(key)
	if q == nil {
		t.Fatal("expected quarantine metadata")
	}
	if q.Recommendation == "" {
		t.Fatal("expected recommendation")
	}
}
