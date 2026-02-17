package orchestrator_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/pkg/types"
)

func TestNewWorldState(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")

	if ws.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", ws.SessionID, "sess-1")
	}
	if ws.Agents == nil {
		t.Fatal("Agents map should be non-nil")
	}
	if ws.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be non-zero")
	}
}

func TestUpdateAgent(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	before := ws.UpdatedAt

	// Small sleep to ensure timestamp advances.
	time.Sleep(time.Millisecond)

	ws.UpdateAgent("agent-1", orchestrator.AgentState{
		Status:        types.StatusBusy,
		PromptDisplay: "Phase 1A P1",
		Task:          "implement foo",
	})

	// Read back via Compact to verify storage.
	raw := ws.Compact()
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Compact() returned invalid JSON: %v", err)
	}

	agents, ok := data["agents"].(map[string]any)
	if !ok {
		t.Fatal("agents field missing or wrong type")
	}
	a1, ok := agents["agent-1"].(map[string]any)
	if !ok {
		t.Fatal("agent-1 not found in compact output")
	}
	if a1["status"] != "busy" {
		t.Errorf("agent status = %q, want %q", a1["status"], "busy")
	}

	if !ws.UpdatedAt.After(before) {
		t.Error("UpdatedAt should be refreshed after UpdateAgent")
	}
}

func TestUpdateAgent_Overwrites(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")

	ws.UpdateAgent("agent-1", orchestrator.AgentState{
		Status: types.StatusBusy,
		Task:   "first task",
	})
	ws.UpdateAgent("agent-1", orchestrator.AgentState{
		Status: types.StatusIdle,
		Task:   "second task",
	})

	raw := ws.Compact()
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Compact() returned invalid JSON: %v", err)
	}

	agents := data["agents"].(map[string]any)
	a1 := agents["agent-1"].(map[string]any)
	if a1["status"] != "idle" {
		t.Errorf("agent status = %q, want %q after overwrite", a1["status"], "idle")
	}
}

func TestUpdatePhase(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	before := ws.UpdatedAt

	time.Sleep(time.Millisecond)

	ws.UpdatePhase("2A", "Config & Scaffolding")

	if ws.Phase != "2A" {
		t.Errorf("Phase = %q, want %q", ws.Phase, "2A")
	}
	if ws.PhaseName != "Config & Scaffolding" {
		t.Errorf("PhaseName = %q, want %q", ws.PhaseName, "Config & Scaffolding")
	}
	if !ws.UpdatedAt.After(before) {
		t.Error("UpdatedAt should be refreshed after UpdatePhase")
	}
}

func TestCompact_ValidJSON(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	ws.UpdatePhase("1A", "Foundation")
	ws.UpdateAgent("claude-1", orchestrator.AgentState{
		Status:        types.StatusBusy,
		PromptDisplay: "P1",
	})

	raw := ws.Compact()
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Compact() returned invalid JSON: %v\nraw: %s", err, raw)
	}

	if data["phase"] != "1A" {
		t.Errorf("phase = %v, want %q", data["phase"], "1A")
	}
	if data["phase_name"] != "Foundation" {
		t.Errorf("phase_name = %v, want %q", data["phase_name"], "Foundation")
	}
}

func TestCompact_UnderTokenLimit(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	ws.UpdatePhase("3B", "Memory Integration")
	ws.GatesPassed = []string{"go build", "go vet", "go test"}
	ws.GatesPending = []string{"lint", "coverage"}

	for i := 0; i < 5; i++ {
		id := types.AgentID("agent-" + string(rune('a'+i)))
		ws.UpdateAgent(id, orchestrator.AgentState{
			Status:        types.StatusBusy,
			PromptDisplay: "Phase 3B P2",
		})
	}

	raw := ws.Compact()
	// 300 tokens ≈ 1200 bytes for English text. Allow generous margin.
	if len(raw) > 1200 {
		t.Errorf("Compact() is %d bytes, want < 1200 (~300 tokens)", len(raw))
	}
}

func TestCompact_EmptyState(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")

	raw := ws.Compact()
	if raw == "" {
		t.Fatal("Compact() should never return empty string")
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Compact() returned invalid JSON: %v", err)
	}
}

func TestCompact_OmitsZeroFields(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	ws.UpdatePhase("1A", "Foundation")
	// DecisionsToday and OpenQuestions default to 0.

	raw := ws.Compact()
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("Compact() returned invalid JSON: %v", err)
	}

	if _, exists := data["decisions_today"]; exists {
		t.Error("decisions_today should be omitted when zero")
	}
	if _, exists := data["open_questions"]; exists {
		t.Error("open_questions should be omitted when zero")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 3 {
			case 0:
				ws.UpdateAgent(types.AgentID("agent-"+string(rune('a'+n%5))), orchestrator.AgentState{
					Status:        types.StatusBusy,
					PromptDisplay: "P1",
				})
			case 1:
				ws.UpdatePhase(types.PhaseID("1A"), "Foundation")
			case 2:
				_ = ws.Compact()
			}
		}(i)
	}
	wg.Wait()
}
