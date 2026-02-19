package orchestrator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// --- Mock AgentLister ---

type mockAgentLister struct {
	instances    []*agent.AgentInstance
	getErr       error
	updateCalled bool
	updateID     types.AgentID
	updateStatus types.AgentStatus
}

func (m *mockAgentLister) List() []*agent.AgentInstance {
	return m.instances
}

func (m *mockAgentLister) Get(id types.AgentID) (*agent.AgentInstance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, inst := range m.instances {
		if inst.Agent.ID == id {
			return inst, nil
		}
	}
	return nil, errors.New("agent not found")
}

func (m *mockAgentLister) UpdateStatus(id types.AgentID, status types.AgentStatus) error {
	m.updateCalled = true
	m.updateID = id
	m.updateStatus = status
	// Also update the instance in memory so subsequent reads reflect it.
	for _, inst := range m.instances {
		if inst.Agent.ID == id {
			inst.Agent.Status = status
		}
	}
	return nil
}

// --- Mock PromptProvider ---

type mockPromptProvider struct {
	phase  *phase.PhaseSpec
	prompt *phase.PromptContract
}

func (m *mockPromptProvider) CurrentPhase() *phase.PhaseSpec {
	return m.phase
}

func (m *mockPromptProvider) CurrentPrompt() *phase.PromptContract {
	return m.prompt
}

// --- Mock Plugin ---

type mockPlugin struct {
	name string
	caps []plugin.Capability
}

func (m *mockPlugin) Name() string                                             { return m.name }
func (m *mockPlugin) LaunchCmd(_ plugin.AgentConfig) (string, []string, error) { return "", nil, nil }
func (m *mockPlugin) DetectReady(_ string) bool                                { return false }
func (m *mockPlugin) DetectBusy(_ string) bool                                 { return false }
func (m *mockPlugin) DetectError(_ string) (string, bool)                      { return "", false }
func (m *mockPlugin) DetectRateLimit(_ string) (time.Duration, bool)           { return 0, false }
func (m *mockPlugin) DetectPrompt(_ string) (plugin.PromptResponse, bool) {
	return plugin.PromptResponse{}, false
}
func (m *mockPlugin) FormatMessage(_ types.Message) string { return "" }
func (m *mockPlugin) ParseOutput(_ string) (plugin.AgentOutput, error) {
	return plugin.AgentOutput{}, nil
}
func (m *mockPlugin) Capabilities() []plugin.Capability { return m.caps }

// --- Helper ---

func makeInstance(id string, status types.AgentStatus, caps []plugin.Capability) *agent.AgentInstance {
	return &agent.AgentInstance{
		Agent: types.Agent{
			ID:     types.AgentID(id),
			Status: status,
			Plugin: "mock",
			Role:   types.RoleEngineer,
		},
		Plugin:     &mockPlugin{name: "mock", caps: caps},
		LaunchedAt: time.Now(),
	}
}

// --- Tests ---

func TestAssignNext_IdleAgent(t *testing.T) {
	idleAgent := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{idleAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if err != nil {
		t.Fatalf("AssignNext() error = %v", err)
	}
	if agentID != "claude-1" {
		t.Errorf("AssignNext() agentID = %q, want %q", agentID, "claude-1")
	}
	if !lister.updateCalled {
		t.Fatal("expected UpdateStatus to be called")
	}
	if lister.updateID != "claude-1" {
		t.Errorf("UpdateStatus called with ID %q, want %q", lister.updateID, "claude-1")
	}
	if lister.updateStatus != types.StatusBusy {
		t.Errorf("UpdateStatus called with status %q, want %q", lister.updateStatus, types.StatusBusy)
	}

	// Verify world state was updated.
	compact := ws.Compact()
	if !containsStr(compact, "claude-1") {
		t.Error("world state compact should contain assigned agent")
	}
}

func TestAssignNext_SkipsBusy(t *testing.T) {
	busyAgent := makeInstance("claude-1", types.StatusBusy, []plugin.Capability{plugin.CapCodeGen})
	idleAgent := makeInstance("codex-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{busyAgent, idleAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if err != nil {
		t.Fatalf("AssignNext() error = %v", err)
	}
	if agentID != "codex-1" {
		t.Errorf("AssignNext() agentID = %q, want %q", agentID, "codex-1")
	}
	if lister.updateID != "codex-1" {
		t.Errorf("should have assigned idle agent codex-1, got %q", lister.updateID)
	}
}

func TestAssignNext_NoIdle(t *testing.T) {
	busyAgent := makeInstance("claude-1", types.StatusBusy, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{busyAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if !errors.Is(err, orchestrator.ErrNoAvailableAgent) {
		t.Errorf("AssignNext() error = %v, want ErrNoAvailableAgent", err)
	}
	if agentID != "" {
		t.Errorf("AssignNext() agentID = %q, want empty", agentID)
	}
}

func TestAssignNext_SkipsNonExecutionRoles(t *testing.T) {
	orchestratorAgent := makeInstance("orchestrator-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	orchestratorAgent.Agent.Role = types.RoleOrchestrator
	reviewerAgent := makeInstance("reviewer-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	reviewerAgent.Agent.Role = types.RoleReviewer
	engineerAgent := makeInstance("engineer-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	engineerAgent.Agent.Role = types.RoleEngineer

	lister := &mockAgentLister{instances: []*agent.AgentInstance{orchestratorAgent, reviewerAgent, engineerAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if err != nil {
		t.Fatalf("AssignNext() error = %v", err)
	}
	if agentID != "engineer-1" {
		t.Fatalf("AssignNext() agentID = %q, want engineer-1", agentID)
	}
}

func TestAssignNext_NoExecutionRolesAvailable(t *testing.T) {
	orchestratorAgent := makeInstance("orchestrator-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	orchestratorAgent.Agent.Role = types.RoleOrchestrator
	reviewerAgent := makeInstance("reviewer-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	reviewerAgent.Agent.Role = types.RoleReviewer

	lister := &mockAgentLister{instances: []*agent.AgentInstance{orchestratorAgent, reviewerAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if !errors.Is(err, orchestrator.ErrNoAvailableAgent) {
		t.Fatalf("AssignNext() error = %v, want ErrNoAvailableAgent", err)
	}
	if agentID != "" {
		t.Fatalf("AssignNext() agentID = %q, want empty", agentID)
	}
}

func TestAssignNext_ReadyGateFiltersIdleAgents(t *testing.T) {
	agent1 := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	agent2 := makeInstance("codex-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{agent1, agent2}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)
	assigner.SetReadyGate(func(id types.AgentID) bool {
		return id == "codex-1"
	})

	agentID, err := assigner.AssignNext(context.Background())
	if err != nil {
		t.Fatalf("AssignNext() error = %v", err)
	}
	if agentID != "codex-1" {
		t.Fatalf("AssignNext() agentID = %q, want codex-1", agentID)
	}
}

func TestAssignNext_ReadyGateNoReadyAgents(t *testing.T) {
	agent1 := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{agent1}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)
	assigner.SetReadyGate(func(types.AgentID) bool { return false })

	agentID, err := assigner.AssignNext(context.Background())
	if !errors.Is(err, orchestrator.ErrNoAvailableAgent) {
		t.Fatalf("AssignNext() error = %v, want ErrNoAvailableAgent", err)
	}
	if agentID != "" {
		t.Fatalf("AssignNext() agentID = %q, want empty", agentID)
	}
}

func TestAssignNext_NoPrompt(t *testing.T) {
	idleAgent := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{idleAgent}}
	provider := &mockPromptProvider{phase: nil, prompt: nil}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	agentID, err := assigner.AssignNext(context.Background())
	if err != nil {
		t.Fatalf("AssignNext() should return nil when no prompt, got %v", err)
	}
	if agentID != "" {
		t.Errorf("AssignNext() agentID = %q, want empty", agentID)
	}
	if lister.updateCalled {
		t.Error("UpdateStatus should not be called when no prompt")
	}
}

func TestAssignNext_PromptGateBlocksPrompt(t *testing.T) {
	idleAgent := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{idleAgent}}
	provider := &mockPromptProvider{
		phase:  &phase.PhaseSpec{ID: "1A", Name: "Foundation"},
		prompt: &phase.PromptContract{PromptNumber: 1, Task: "create types"},
	}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)
	assigner.SetPromptGate(func(phaseID types.PhaseID, promptNum int) bool {
		return false
	})

	agentID, err := assigner.AssignNext(context.Background())
	if !errors.Is(err, orchestrator.ErrNoAvailableAgent) {
		t.Fatalf("AssignNext() error = %v, want ErrNoAvailableAgent", err)
	}
	if agentID != "" {
		t.Fatalf("AssignNext() agentID = %q, want empty", agentID)
	}
	if lister.updateCalled {
		t.Fatal("UpdateStatus should not be called when prompt gate blocks")
	}
}

func TestAssignToAgent(t *testing.T) {
	idleAgent := makeInstance("claude-1", types.StatusIdle, []plugin.Capability{plugin.CapCodeGen})
	lister := &mockAgentLister{instances: []*agent.AgentInstance{idleAgent}}
	provider := &mockPromptProvider{}
	ws := orchestrator.NewWorldState("sess-1")
	assigner := orchestrator.NewAssigner(lister, provider, ws, nil)

	err := assigner.AssignToAgent(context.Background(), "claude-1", "2A", 3)
	if err != nil {
		t.Fatalf("AssignToAgent() error = %v", err)
	}
	if lister.updateStatus != types.StatusBusy {
		t.Errorf("agent status = %q, want %q", lister.updateStatus, types.StatusBusy)
	}

	compact := ws.Compact()
	if !containsStr(compact, "claude-1") {
		t.Error("world state compact should contain assigned agent")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
