package instruct

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- Test mocks ---

type mockTmuxCmd struct {
	mu   sync.Mutex
	args [][]string
}

func (m *mockTmuxCmd) Run(_ context.Context, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.args = append(m.args, args)
	return "", nil
}

func (m *mockTmuxCmd) calls() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([][]string, len(m.args))
	copy(cp, m.args)
	return cp
}

type mockAuditLogger struct {
	mu      sync.Mutex
	entries []auditEntry
}

type auditEntry struct {
	agentID string
	action  string
	files   []string
}

func (m *mockAuditLogger) LogGeneration(agentID string, action string, files []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, auditEntry{agentID: agentID, action: action, files: files})
	return nil
}

type mockAgentConfig struct {
	agents map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}
}

func (m *mockAgentConfig) AgentIDs() []string {
	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids
}

func (m *mockAgentConfig) AgentRole(id string) RoleName {
	return m.agents[id].role
}

func (m *mockAgentConfig) AgentCLI(id string) AgentCLI {
	return m.agents[id].cli
}

func (m *mockAgentConfig) AgentPaneID(id string) string {
	return m.agents[id].paneID
}

// --- Test helpers ---

func testDistributor(t *testing.T, projectRoot string, agentCfgs map[string]struct {
	role   RoleName
	cli    AgentCLI
	paneID string
}) (*Distributor, *mockTmuxCmd, *mockAuditLogger) {
	t.Helper()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	agg := NewAggregator(AggregatorDeps{
		Config: AggregatorConfig{
			ProjectName: "test-project",
			Language:    "Go",
			RepoRoot:    projectRoot,
		},
		Version: "0.1.0",
	})

	tmuxCmd := &mockTmuxCmd{}
	audit := &mockAuditLogger{}
	agents := &mockAgentConfig{agents: agentCfgs}

	dist := NewDistributor(DistributorDeps{
		Aggregator:  agg,
		Renderer:    r,
		TmuxCmd:     tmuxCmd,
		Audit:       audit,
		Agents:      agents,
		ProjectRoot: projectRoot,
	})

	return dist, tmuxCmd, audit
}

// --- Tests ---

func TestGenerateForAgent_CreatesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, audit := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	})

	ctx := context.Background()
	if err := dist.GenerateForAgent(ctx, "eng-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	// Verify CLAUDE.md was written.
	primaryPath := filepath.Join(dir, "CLAUDE.md")
	data, err := os.ReadFile(primaryPath)
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if len(data) == 0 {
		t.Error("CLAUDE.md should not be empty")
	}
	if !strings.Contains(string(data), markerBeginPrefix) {
		t.Error("CLAUDE.md should contain BEGIN marker")
	}

	// Verify rules file was written.
	rulesPath := filepath.Join(dir, ".claude", "rules", "crux-session.md")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		t.Error("rules file should exist")
	}

	// Verify state was recorded.
	state := dist.State()
	if s, ok := state["eng-1"]; !ok {
		t.Error("state should exist for eng-1")
	} else {
		if s.Hash == "" {
			t.Error("hash should be set")
		}
		if s.TokenCount <= 0 {
			t.Error("token count should be positive")
		}
		if len(s.FilesWritten) != 2 {
			t.Errorf("expected 2 files written, got %d", len(s.FilesWritten))
		}
	}

	// Verify audit was called.
	if len(audit.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(audit.entries))
	}
	if audit.entries[0].action != "generate" {
		t.Errorf("audit action = %s, want generate", audit.entries[0].action)
	}
}

func TestGenerateForAgent_CodexSplit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"codex-1": {role: RoleSoftwareEngineer, cli: CLICodex, paneID: "%2"},
	})

	ctx := context.Background()
	if err := dist.GenerateForAgent(ctx, "codex-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	// Verify both files exist.
	agentsPath := filepath.Join(dir, "AGENTS.md")
	overridePath := filepath.Join(dir, "AGENTS.override.md")

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Error("AGENTS.md should exist")
	}
	if _, err := os.Stat(overridePath); os.IsNotExist(err) {
		t.Error("AGENTS.override.md should exist")
	}
}

func TestGenerateAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1":   {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
		"codex-1": {role: RoleSoftwareEngineer, cli: CLICodex, paneID: "%2"},
	})

	ctx := context.Background()
	if err := dist.GenerateAll(ctx); err != nil {
		t.Fatalf("GenerateAll() error: %v", err)
	}

	state := dist.State()
	if len(state) != 2 {
		t.Errorf("expected 2 states, got %d", len(state))
	}
}

func TestRegenerateIfStale_UnchangedContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, audit := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	})

	ctx := context.Background()

	// First generation.
	if err := dist.GenerateForAgent(ctx, "eng-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	// Regenerate — same context, so hash should match.
	changed, err := dist.RegenerateIfStale(ctx, "eng-1")
	if err != nil {
		t.Fatalf("RegenerateIfStale() error: %v", err)
	}
	if changed {
		t.Error("expected no change when context is identical")
	}

	// Only one audit entry from the initial generate.
	if len(audit.entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(audit.entries))
	}
}

func TestRegenerateIfStale_DetectsChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	agg := NewAggregator(AggregatorDeps{
		Config: AggregatorConfig{
			ProjectName: "test-project",
			Language:    "Go",
			RepoRoot:    dir,
		},
		Version: "0.1.0",
	})

	audit := &mockAuditLogger{}
	agents := &mockAgentConfig{agents: map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	}}

	dist := NewDistributor(DistributorDeps{
		Aggregator:  agg,
		Renderer:    r,
		Audit:       audit,
		Agents:      agents,
		ProjectRoot: dir,
	})

	ctx := context.Background()

	// First generation.
	if err := dist.GenerateForAgent(ctx, "eng-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	// Tamper with the stored hash to simulate a context change.
	dist.mu.Lock()
	dist.state["eng-1"].Hash = "stale-hash"
	dist.mu.Unlock()

	changed, err := dist.RegenerateIfStale(ctx, "eng-1")
	if err != nil {
		t.Fatalf("RegenerateIfStale() error: %v", err)
	}
	if !changed {
		t.Error("expected change when hash differs")
	}

	// Should have 2 audit entries: generate + regenerate.
	if len(audit.entries) != 2 {
		t.Errorf("expected 2 audit entries, got %d", len(audit.entries))
	}
	if audit.entries[1].action != "regenerate" {
		t.Errorf("second audit action = %s, want regenerate", audit.entries[1].action)
	}
}

func TestReloadAgent_GeminiSendsKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, tmuxCmd, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"gemini-1": {role: RoleSoftwareEngineer, cli: CLIGemini, paneID: "%5"},
	})

	ctx := context.Background()
	if err := dist.ReloadAgent(ctx, "gemini-1"); err != nil {
		t.Fatalf("ReloadAgent() error: %v", err)
	}

	calls := tmuxCmd.calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tmux call, got %d", len(calls))
	}

	args := calls[0]
	if args[0] != "send-keys" {
		t.Errorf("expected send-keys command, got %s", args[0])
	}
	if args[2] != "%5" {
		t.Errorf("expected pane ID %%5, got %s", args[2])
	}
	// The reload command should be /memory refresh\n
	if !strings.Contains(args[3], "/memory refresh") {
		t.Errorf("expected /memory refresh command, got %s", args[3])
	}
}

func TestReloadAgent_ClaudeSkips(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, tmuxCmd, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"claude-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	})

	ctx := context.Background()
	if err := dist.ReloadAgent(ctx, "claude-1"); err != nil {
		t.Fatalf("ReloadAgent() error: %v", err)
	}

	// Claude uses ReloadRestart, so no tmux command should be sent.
	calls := tmuxCmd.calls()
	if len(calls) != 0 {
		t.Errorf("expected no tmux calls for Claude, got %d", len(calls))
	}
}

func TestReloadAgent_CodexSendsNewSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, tmuxCmd, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"codex-1": {role: RoleSoftwareEngineer, cli: CLICodex, paneID: "%3"},
	})

	ctx := context.Background()
	if err := dist.ReloadAgent(ctx, "codex-1"); err != nil {
		t.Fatalf("ReloadAgent() error: %v", err)
	}

	calls := tmuxCmd.calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tmux call, got %d", len(calls))
	}
	if !strings.Contains(calls[0][3], "/new") {
		t.Errorf("expected /new command, got %s", calls[0][3])
	}
}

func TestHashComparisonWorks(t *testing.T) {
	t.Parallel()

	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	h3 := contentHash("hello world!")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestFilePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	})

	ctx := context.Background()
	if err := dist.GenerateForAgent(ctx, "eng-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	primaryPath := filepath.Join(dir, "CLAUDE.md")
	info, err := os.Stat(primaryPath)
	if err != nil {
		t.Fatalf("stat CLAUDE.md: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o644 {
		t.Errorf("file permissions = %o, want 644", perm)
	}
}

func TestNeedsReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"claude-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
		"gemini-1": {role: RoleSoftwareEngineer, cli: CLIGemini, paneID: "%2"},
		"codex-1":  {role: RoleSoftwareEngineer, cli: CLICodex, paneID: "%3"},
	})

	if dist.NeedsReload("claude-1") {
		t.Error("Claude should not need mid-session reload")
	}
	if !dist.NeedsReload("gemini-1") {
		t.Error("Gemini should need mid-session reload")
	}
	if !dist.NeedsReload("codex-1") {
		t.Error("Codex should need mid-session reload")
	}
}

func TestDirectoryCreation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dist, _, _ := testDistributor(t, dir, map[string]struct {
		role   RoleName
		cli    AgentCLI
		paneID string
	}{
		"eng-1": {role: RoleSoftwareEngineer, cli: CLIClaude, paneID: "%1"},
	})

	ctx := context.Background()
	if err := dist.GenerateForAgent(ctx, "eng-1"); err != nil {
		t.Fatalf("GenerateForAgent() error: %v", err)
	}

	// .claude/rules/ should have been created automatically.
	rulesDir := filepath.Join(dir, ".claude", "rules")
	info, err := os.Stat(rulesDir)
	if err != nil {
		t.Fatalf("rules dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("rules path should be a directory")
	}
}
