package protocol_test

import (
	"testing"
	"time"

	"github.com/roygabriel/crux/pkg/protocol"
	"github.com/roygabriel/crux/pkg/types"
)

func sampleMessage() types.Message {
	return types.Message{
		ID:        "msg-1",
		From:      "orchestrator",
		To:        "engineer-1",
		Type:      types.MessageTask,
		Priority:  types.PriorityNormal,
		Payload:   "implement the feature",
		Timestamp: time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC),
	}
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	data, err := protocol.Marshal(sampleMessage())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Marshal returned empty bytes")
	}

	// Verify key fields are present in JSON output.
	s := string(data)
	for _, want := range []string{`"id":"msg-1"`, `"type":"task"`, `"from":"orchestrator"`} {
		if !contains(s, want) {
			t.Errorf("JSON missing %s: %s", want, s)
		}
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	raw := `{"id":"msg-1","from":"orch","to":"eng-1","type":"task","priority":"normal","payload":"do stuff","timestamp":"2026-02-17T12:00:00Z"}`

	msg, err := protocol.Unmarshal([]byte(raw))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if msg.ID != "msg-1" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-1")
	}
	if msg.From != "orch" {
		t.Errorf("From = %q, want %q", msg.From, "orch")
	}
	if msg.To != "eng-1" {
		t.Errorf("To = %q, want %q", msg.To, "eng-1")
	}
	if msg.Type != types.MessageTask {
		t.Errorf("Type = %q, want %q", msg.Type, types.MessageTask)
	}
	if msg.Priority != types.PriorityNormal {
		t.Errorf("Priority = %q, want %q", msg.Priority, types.PriorityNormal)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	original := types.Message{
		ID:        "msg-42",
		From:      "orchestrator",
		To:        "engineer-1",
		Type:      types.MessageTask,
		Priority:  types.PriorityHigh,
		Payload:   map[string]any{"task": "build the thing", "phase": "2a"},
		Timestamp: time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC),
	}

	data, err := protocol.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := protocol.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID = %q, want %q", got.ID, original.ID)
	}
	if got.From != original.From {
		t.Errorf("From = %q, want %q", got.From, original.From)
	}
	if got.To != original.To {
		t.Errorf("To = %q, want %q", got.To, original.To)
	}
	if got.Type != original.Type {
		t.Errorf("Type = %q, want %q", got.Type, original.Type)
	}
	if got.Priority != original.Priority {
		t.Errorf("Priority = %q, want %q", got.Priority, original.Priority)
	}
	if !got.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, original.Timestamp)
	}
}

func TestUnmarshalInvalid(t *testing.T) {
	t.Parallel()

	_, err := protocol.Unmarshal([]byte("not json"))
	if err == nil {
		t.Fatal("Unmarshal: expected error for invalid JSON")
	}
}

func TestRoundTripNilPayload(t *testing.T) {
	t.Parallel()

	original := types.Message{
		ID:        "msg-nil",
		From:      "orch",
		To:        "eng-1",
		Type:      types.MessageAck,
		Timestamp: time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC),
	}

	data, err := protocol.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := protocol.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != original.ID {
		t.Errorf("ID = %q, want %q", got.ID, original.ID)
	}
	if got.Payload != nil {
		t.Errorf("Payload = %v, want nil", got.Payload)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
