package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/session"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "less than a minute",
			duration: 30 * time.Second,
			want:     "just now",
		},
		{
			name:     "exactly zero",
			duration: 0,
			want:     "just now",
		},
		{
			name:     "minutes only",
			duration: 5 * time.Minute,
			want:     "5m ago",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			want:     "1m ago",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 14*time.Minute,
			want:     "2h14m ago",
		},
		{
			name:     "one hour even",
			duration: 1 * time.Hour,
			want:     "1h0m ago",
		},
		{
			name:     "days and hours",
			duration: 26 * time.Hour,
			want:     "1d 2h ago",
		},
		{
			name:     "exactly one day",
			duration: 24 * time.Hour,
			want:     "1d 0h ago",
		},
		{
			name:     "45 minutes",
			duration: 45 * time.Minute,
			want:     "45m ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestPhaseIndicator(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"complete", "\u2713"},
		{"in-progress", "\u25c9"},
		{"not-started", "\u25cb"},
		{"unknown", "\u25cb"},
		{"", "\u25cb"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := phaseIndicator(tt.status)
			if got != tt.want {
				t.Errorf("phaseIndicator(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"running", "\u25cf"},
		{"active", "\u25cf"},
		{"busy", "\u25cf"},
		{"idle", "\u25cb"},
		{"ready", "\u25cb"},
		{"stopped", "\u2715"},
		{"error", "\u2715"},
		{"failed", "\u2715"},
		{"unknown", "\u25cb"},
		{"", "\u25cb"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusIndicator(tt.status)
			if got != tt.want {
				t.Errorf("statusIndicator(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestPadOrTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"exact length", "hello", 5, "hello"},
		{"pad short string", "hi", 5, "hi   "},
		{"truncate long string", "hello world", 5, "hello"},
		{"empty string", "", 3, "   "},
		{"zero width", "abc", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padOrTruncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("padOrTruncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestStatusOutputJSON(t *testing.T) {
	output := StatusOutput{
		Session: SessionInfo{
			ID:        "abc12345",
			StartedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			Duration:  "2h14m ago",
		},
		Phase:          "2A",
		PromptProgress: 2,
		Agents:         map[string]session.AgentState{},
		PhaseProgress: []PhaseProgressLine{
			{ID: "1A", Name: "Config Layer", Status: "complete", Completed: 4, Total: 4},
			{ID: "2A", Name: "Agent Plugin", Status: "in-progress", Completed: 2, Total: 4},
			{ID: "2B", Name: "Phase Engine", Status: "not-started", Completed: 0, Total: 3},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal StatusOutput: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	requiredKeys := []string{"session", "phase", "prompt_progress", "agents", "phase_progress"}
	for _, key := range requiredKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required JSON key %q", key)
		}
	}

	sessionMap, ok := parsed["session"].(map[string]interface{})
	if !ok {
		t.Fatal("session is not a JSON object")
	}
	sessionKeys := []string{"id", "started_at", "duration"}
	for _, key := range sessionKeys {
		if _, ok := sessionMap[key]; !ok {
			t.Errorf("missing session key %q", key)
		}
	}

	if parsed["phase"] != "2A" {
		t.Errorf("phase = %v, want %q", parsed["phase"], "2A")
	}

	progress, ok := parsed["phase_progress"].([]interface{})
	if !ok {
		t.Fatal("phase_progress is not a JSON array")
	}
	if len(progress) != 3 {
		t.Errorf("phase_progress length = %d, want 3", len(progress))
	}
}

func TestPhaseNameFromLines(t *testing.T) {
	lines := []PhaseProgressLine{
		{ID: "1A", Name: "Config Layer"},
		{ID: "2A", Name: "Agent Plugin"},
	}

	if got := phaseNameFromLines(lines, "2A"); got != "Agent Plugin" {
		t.Errorf("phaseNameFromLines for 2A = %q, want %q", got, "Agent Plugin")
	}

	if got := phaseNameFromLines(lines, "9Z"); got != "" {
		t.Errorf("phaseNameFromLines for 9Z = %q, want empty", got)
	}

	if got := phaseNameFromLines(nil, "1A"); got != "" {
		t.Errorf("phaseNameFromLines for nil = %q, want empty", got)
	}
}
