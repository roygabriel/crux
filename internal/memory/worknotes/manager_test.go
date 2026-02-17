package worknotes_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/memory/worknotes"
)

func newTestManager(t *testing.T) (*worknotes.Manager, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "work-notes")
	m := worknotes.NewManager(dir, slog.Default())
	return m, dir
}

func TestInitCreatesFile(t *testing.T) {
	t.Parallel()
	m, dir := newTestManager(t)

	if err := m.Init("3B", "Semantic Search"); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	path := filepath.Join(dir, "PHASE3B.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Phase 3B - Semantic Search") {
		t.Error("expected phase header in generated file")
	}
	if !strings.Contains(content, "- [x] Not started") {
		t.Error("expected default status 'Not started' checked")
	}
}

func TestInitIdempotent(t *testing.T) {
	t.Parallel()
	m, dir := newTestManager(t)

	m.Init("3B", "Semantic Search")

	// Modify the file.
	path := filepath.Join(dir, "PHASE3B.md")
	original, _ := os.ReadFile(path)

	// Second init should not overwrite.
	if err := m.Init("3B", "Different Name"); err != nil {
		t.Fatalf("second Init() error: %v", err)
	}

	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Error("Init() overwrote existing file")
	}
}

func TestReadParsesAllSections(t *testing.T) {
	t.Parallel()
	m, dir := newTestManager(t)

	content := `# Phase 3B - Semantic Search

## Status
- [ ] Not started
- [x] In progress
- [ ] Blocked
- [ ] Complete

## Decisions
- Use chromem-go: Pure Go, no CGO dependency
- Use FTS5 fallback: Handles case when vector index unavailable

## Assumptions
- Ollama may not be available
- SQLite is always present

## Open Questions
- [x] Which embedding model to use?
- [ ] Should we support custom embedding dimensions?

## Session Log

### 2026-02-17 10:00
- What changed: Added vector index wrapper
- Why: Needed for semantic search
- Blockers: None
- Next: Integrate with journal

### 2026-02-17 14:00
- What changed: Integrated with journal
- Why: Complete semantic search pipeline
- Blockers: None
- Next: Work notes manager

## Prompt Progress
- [x] Prompt 1
- [x] Prompt 2
- [ ] Prompt 3

## Commits
- abc1234 - feat(vector): add index wrapper
- def5678 - feat(journal): integrate vector search
`
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "PHASE3B.md"), []byte(content), 0o644)

	notes, err := m.Read("3B")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if notes.PhaseID != "3B" {
		t.Errorf("PhaseID = %q, want %q", notes.PhaseID, "3B")
	}
	if notes.PhaseName != "Semantic Search" {
		t.Errorf("PhaseName = %q, want %q", notes.PhaseName, "Semantic Search")
	}
	if notes.Status != "In progress" {
		t.Errorf("Status = %q, want %q", notes.Status, "In progress")
	}

	if len(notes.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(notes.Decisions))
	}
	if notes.Decisions[0].Decision != "Use chromem-go" {
		t.Errorf("Decision[0] = %q, want %q", notes.Decisions[0].Decision, "Use chromem-go")
	}
	if notes.Decisions[0].Rationale != "Pure Go, no CGO dependency" {
		t.Errorf("Rationale[0] = %q", notes.Decisions[0].Rationale)
	}

	if len(notes.Assumptions) != 2 {
		t.Fatalf("expected 2 assumptions, got %d", len(notes.Assumptions))
	}

	if len(notes.OpenQuestions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(notes.OpenQuestions))
	}
	if !notes.OpenQuestions[0].Resolved {
		t.Error("expected first question resolved")
	}
	if notes.OpenQuestions[1].Resolved {
		t.Error("expected second question unresolved")
	}

	if len(notes.SessionLog) != 2 {
		t.Fatalf("expected 2 session entries, got %d", len(notes.SessionLog))
	}
	if notes.SessionLog[0].Timestamp != "2026-02-17 10:00" {
		t.Errorf("SessionLog[0].Timestamp = %q", notes.SessionLog[0].Timestamp)
	}
	if notes.SessionLog[0].Changed != "Added vector index wrapper" {
		t.Errorf("SessionLog[0].Changed = %q", notes.SessionLog[0].Changed)
	}

	if len(notes.PromptProgress) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(notes.PromptProgress))
	}
	if !notes.PromptProgress[0].Complete {
		t.Error("expected Prompt 1 complete")
	}
	if notes.PromptProgress[2].Complete {
		t.Error("expected Prompt 3 incomplete")
	}

	if len(notes.Commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(notes.Commits))
	}
	if notes.Commits[0].Hash != "abc1234" {
		t.Errorf("Commit[0].Hash = %q", notes.Commits[0].Hash)
	}
}

func TestAppendDecision(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)
	m.Init("3B", "Semantic Search")

	if err := m.AppendDecision("3B", "Use chromem-go", "Pure Go, no CGO"); err != nil {
		t.Fatalf("AppendDecision() error: %v", err)
	}

	notes, err := m.Read("3B")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(notes.Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(notes.Decisions))
	}
	if notes.Decisions[0].Decision != "Use chromem-go" {
		t.Errorf("Decision = %q", notes.Decisions[0].Decision)
	}
	if notes.Decisions[0].Rationale != "Pure Go, no CGO" {
		t.Errorf("Rationale = %q", notes.Decisions[0].Rationale)
	}
}

func TestAppendSession(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)
	m.Init("3B", "Semantic Search")

	entry := worknotes.SessionLogEntry{
		Timestamp: "2026-02-17 10:00",
		Changed:   "Added vector wrapper",
		Why:       "Needed for semantic search",
		Blockers:  "None",
		Next:      "Integrate with journal",
	}
	if err := m.AppendSession("3B", entry); err != nil {
		t.Fatalf("AppendSession() error: %v", err)
	}

	notes, err := m.Read("3B")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(notes.SessionLog) != 1 {
		t.Fatalf("expected 1 session entry, got %d", len(notes.SessionLog))
	}
	if notes.SessionLog[0].Timestamp != "2026-02-17 10:00" {
		t.Errorf("Timestamp = %q", notes.SessionLog[0].Timestamp)
	}
	if notes.SessionLog[0].Changed != "Added vector wrapper" {
		t.Errorf("Changed = %q", notes.SessionLog[0].Changed)
	}
}

func TestUpdatePromptProgress(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)
	m.Init("3B", "Semantic Search")

	// Add prompt 1 as complete.
	if err := m.UpdatePromptProgress("3B", 1, true); err != nil {
		t.Fatalf("UpdatePromptProgress() error: %v", err)
	}

	notes, _ := m.Read("3B")
	found := false
	for _, p := range notes.PromptProgress {
		if p.Number == 1 && p.Complete {
			found = true
		}
	}
	if !found {
		t.Error("expected Prompt 1 to be marked complete")
	}

	// Update prompt 1 to incomplete.
	if err := m.UpdatePromptProgress("3B", 1, false); err != nil {
		t.Fatalf("UpdatePromptProgress() error: %v", err)
	}
	notes, _ = m.Read("3B")
	for _, p := range notes.PromptProgress {
		if p.Number == 1 && p.Complete {
			t.Error("expected Prompt 1 to be marked incomplete after update")
		}
	}
}

func TestUpdateStatus(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)
	m.Init("3B", "Semantic Search")

	if err := m.UpdateStatus("3B", "In progress"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	notes, _ := m.Read("3B")
	if notes.Status != "In progress" {
		t.Errorf("Status = %q, want %q", notes.Status, "In progress")
	}

	// Verify render has correct checkbox state.
	rendered := m.Render(notes)
	if !strings.Contains(rendered, "- [x] In progress") {
		t.Error("expected 'In progress' to be checked")
	}
	if !strings.Contains(rendered, "- [ ] Not started") {
		t.Error("expected 'Not started' to be unchecked")
	}
}

func TestRenderMatchesTemplate(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)
	m.Init("3B", "Semantic Search")

	notes, _ := m.Read("3B")
	rendered := m.Render(notes)

	if !strings.Contains(rendered, "# Phase 3B - Semantic Search") {
		t.Error("missing phase header")
	}
	if !strings.Contains(rendered, "## Status") {
		t.Error("missing Status section")
	}
	if !strings.Contains(rendered, "## Decisions") {
		t.Error("missing Decisions section")
	}
	if !strings.Contains(rendered, "## Assumptions") {
		t.Error("missing Assumptions section")
	}
	if !strings.Contains(rendered, "## Open Questions") {
		t.Error("missing Open Questions section")
	}
	if !strings.Contains(rendered, "## Session Log") {
		t.Error("missing Session Log section")
	}
	if !strings.Contains(rendered, "## Prompt Progress") {
		t.Error("missing Prompt Progress section")
	}
	if !strings.Contains(rendered, "## Commits") {
		t.Error("missing Commits section")
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	m, _ := newTestManager(t)

	// Init.
	m.Init("3B", "Semantic Search")

	// Read initial.
	notes, err := m.Read("3B")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if notes.PhaseID != "3B" {
		t.Fatalf("initial PhaseID = %q", notes.PhaseID)
	}

	// Append a decision.
	m.AppendDecision("3B", "Use chromem-go", "Pure Go embedding")

	// Read again — verify decision persisted.
	notes, err = m.Read("3B")
	if err != nil {
		t.Fatalf("Read() after append error: %v", err)
	}
	if len(notes.Decisions) != 1 {
		t.Fatalf("expected 1 decision after append, got %d", len(notes.Decisions))
	}

	// Append another decision.
	m.AppendDecision("3B", "FTS5 fallback", "Offline support")

	// Add session.
	m.AppendSession("3B", worknotes.SessionLogEntry{
		Timestamp: "2026-02-17 10:00",
		Changed:   "Vector index",
		Why:       "Semantic search",
		Blockers:  "None",
		Next:      "Journal integration",
	})

	// Update status.
	m.UpdateStatus("3B", "In progress")

	// Update prompt progress.
	m.UpdatePromptProgress("3B", 1, true)
	m.UpdatePromptProgress("3B", 2, false)

	// Final read — verify everything.
	notes, err = m.Read("3B")
	if err != nil {
		t.Fatalf("final Read() error: %v", err)
	}
	if len(notes.Decisions) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(notes.Decisions))
	}
	if len(notes.SessionLog) != 1 {
		t.Errorf("expected 1 session entry, got %d", len(notes.SessionLog))
	}
	if notes.Status != "In progress" {
		t.Errorf("Status = %q, want %q", notes.Status, "In progress")
	}

	completedPrompts := 0
	for _, p := range notes.PromptProgress {
		if p.Complete {
			completedPrompts++
		}
	}
	if completedPrompts != 1 {
		t.Errorf("expected 1 completed prompt, got %d", completedPrompts)
	}

	// Render and verify it's valid markdown.
	rendered := m.Render(notes)
	if !strings.Contains(rendered, "- [x] In progress") {
		t.Error("rendered missing checked In progress")
	}
	if !strings.Contains(rendered, "- [x] Prompt 1") {
		t.Error("rendered missing checked Prompt 1")
	}
	if !strings.Contains(rendered, "- [ ] Prompt 2") {
		t.Error("rendered missing unchecked Prompt 2")
	}
}
