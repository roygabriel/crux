package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/pkg/types"
)

// PhaseValidator checks if phases can run in parallel.
type PhaseValidator interface {
	ValidateParallelism(phases []types.PhaseID) error
}

// GitDiffer returns file paths changed in the working tree.
type GitDiffer interface {
	DiffNames(ctx context.Context) ([]string, error)
}

// WorkNotesAppender appends session log entries to phase work notes.
type WorkNotesAppender interface {
	AppendSession(phaseID string, entry worknotes.SessionLogEntry) error
}

// DecisionRecorder records decisions to the journal.
type DecisionRecorder interface {
	Record(ctx context.Context, d types.Decision) error
}

// ConflictEvent represents a detected file conflict between two agents.
type ConflictEvent struct {
	// AgentA is the first agent in the conflict.
	AgentA types.AgentID `json:"agent_a"`
	// AgentB is the second agent in the conflict.
	AgentB types.AgentID `json:"agent_b"`
	// PhaseA is the phase assigned to AgentA.
	PhaseA types.PhaseID `json:"phase_a"`
	// PhaseB is the phase assigned to AgentB.
	PhaseB types.PhaseID `json:"phase_b"`
	// ConflictingFiles lists the files both agents are touching.
	ConflictingFiles []string `json:"conflicting_files"`
	// DetectedAt is when the conflict was detected.
	DetectedAt time.Time `json:"detected_at"`
}

// trackedAssignment records an agent's tracked file ownership.
type trackedAssignment struct {
	phaseID types.PhaseID
	files   []string
}

// ConflictDetector monitors for file conflicts between parallel agents.
type ConflictDetector struct {
	validator  PhaseValidator
	differ     GitDiffer
	agents     AgentLister
	worldState *WorldState
	workNotes  WorkNotesAppender
	journal    DecisionRecorder
	logger     *slog.Logger
	interval   time.Duration
	mu         sync.RWMutex
	tracked    map[types.AgentID]trackedAssignment
}

// NewConflictDetector creates a ConflictDetector with the given dependencies.
func NewConflictDetector(
	validator PhaseValidator,
	differ GitDiffer,
	agents AgentLister,
	worldState *WorldState,
	workNotes WorkNotesAppender,
	journal DecisionRecorder,
	interval time.Duration,
	logger *slog.Logger,
) *ConflictDetector {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ConflictDetector{
		validator:  validator,
		differ:     differ,
		agents:     agents,
		worldState: worldState,
		workNotes:  workNotes,
		journal:    journal,
		logger:     logger,
		interval:   interval,
		tracked:    make(map[types.AgentID]trackedAssignment),
	}
}

// CheckBeforeAssign validates that two phases can run in parallel.
// Returns a descriptive error naming conflicting files on failure.
func (cd *ConflictDetector) CheckBeforeAssign(phaseA, phaseB types.PhaseID) error {
	return cd.validator.ValidateParallelism([]types.PhaseID{phaseA, phaseB})
}

// TrackAssignment registers an agent's file ownership for conflict monitoring.
func (cd *ConflictDetector) TrackAssignment(agentID types.AgentID, phaseID types.PhaseID, files []string) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.tracked[agentID] = trackedAssignment{
		phaseID: phaseID,
		files:   files,
	}
}

// UntrackAssignment removes tracking for a completed agent.
func (cd *ConflictDetector) UntrackAssignment(agentID types.AgentID) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	delete(cd.tracked, agentID)
}

// MonitorRuntime starts a background goroutine that periodically checks for
// file conflicts between tracked agents. It returns a channel that emits
// ConflictEvent values. The goroutine stops when ctx is cancelled and the
// channel is closed on exit.
func (cd *ConflictDetector) MonitorRuntime(ctx context.Context) <-chan ConflictEvent {
	ch := make(chan ConflictEvent, 8)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(cd.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events := cd.detectConflicts(ctx)
				for _, ev := range events {
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch
}

// detectConflicts checks git diff output against tracked assignments.
func (cd *ConflictDetector) detectConflicts(ctx context.Context) []ConflictEvent {
	changed, err := cd.differ.DiffNames(ctx)
	if err != nil {
		cd.logger.Warn("git diff failed during conflict detection", "error", err)
		return nil
	}
	if len(changed) == 0 {
		return nil
	}

	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// Build a map of changed file -> which agents claim it.
	type ownerInfo struct {
		agentID types.AgentID
		phaseID types.PhaseID
	}

	fileOwners := make(map[string][]ownerInfo)
	changedSet := make(map[string]bool, len(changed))
	for _, f := range changed {
		changedSet[f] = true
	}

	for agentID, tracked := range cd.tracked {
		for _, f := range tracked.files {
			if changedSet[f] {
				fileOwners[f] = append(fileOwners[f], ownerInfo{
					agentID: agentID,
					phaseID: tracked.phaseID,
				})
			}
		}
	}

	// Find files with multiple owners.
	type conflictPair struct {
		a, b ownerInfo
	}
	pairFiles := make(map[conflictPair][]string)

	for file, owners := range fileOwners {
		if len(owners) < 2 {
			continue
		}
		for i := 0; i < len(owners); i++ {
			for j := i + 1; j < len(owners); j++ {
				pair := conflictPair{a: owners[i], b: owners[j]}
				pairFiles[pair] = append(pairFiles[pair], file)
			}
		}
	}

	now := time.Now().UTC()
	var events []ConflictEvent
	for pair, files := range pairFiles {
		events = append(events, ConflictEvent{
			AgentA:           pair.a.agentID,
			AgentB:           pair.b.agentID,
			PhaseA:           pair.a.phaseID,
			PhaseB:           pair.b.phaseID,
			ConflictingFiles: files,
			DetectedAt:       now,
		})
	}
	return events
}

// HandleConflict resolves a file conflict by halting the later-assigned agent.
func (cd *ConflictDetector) HandleConflict(ctx context.Context, event ConflictEvent) error {
	// Determine which agent was assigned later.
	stateA, _ := cd.worldState.GetAgent(event.AgentA)
	stateB, _ := cd.worldState.GetAgent(event.AgentB)

	laterAgent := event.AgentA
	earlierAgent := event.AgentB
	laterPhase := event.PhaseA
	earlierPhase := event.PhaseB
	if stateB.AssignedAt.After(stateA.AssignedAt) {
		laterAgent = event.AgentB
		earlierAgent = event.AgentA
		laterPhase = event.PhaseB
		earlierPhase = event.PhaseA
	}

	// Halt the later agent.
	if err := cd.agents.UpdateStatus(laterAgent, types.StatusError); err != nil {
		return fmt.Errorf("halt agent %s: %w", laterAgent, err)
	}

	// Update world state.
	cd.worldState.UpdateAgent(laterAgent, AgentState{
		Status:       types.StatusError,
		LastDecision: fmt.Sprintf("halted: file conflict with %s", earlierAgent),
		LastActive:   time.Now().UTC(),
		PhaseID:      laterPhase,
		AssignedAt:   stateA.AssignedAt,
	})

	filesStr := strings.Join(event.ConflictingFiles, ", ")

	// Log to work notes for both phases.
	entry := worknotes.SessionLogEntry{
		Timestamp: time.Now().UTC().Format("2006-01-02 15:04"),
		Changed:   fmt.Sprintf("File conflict detected on: %s", filesStr),
		Why:       fmt.Sprintf("Agents %s and %s both modifying same files", event.AgentA, event.AgentB),
		Blockers:  fmt.Sprintf("Agent %s halted", laterAgent),
		Next:      "Resolve conflict before continuing",
	}
	if err := cd.workNotes.AppendSession(string(laterPhase), entry); err != nil {
		cd.logger.Warn("failed to log conflict to work notes", "phase", laterPhase, "error", err)
	}
	if err := cd.workNotes.AppendSession(string(earlierPhase), entry); err != nil {
		cd.logger.Warn("failed to log conflict to work notes", "phase", earlierPhase, "error", err)
	}

	// Record decision in journal.
	decision := types.Decision{
		Timestamp: time.Now().UTC(),
		PhaseID:   laterPhase,
		AgentID:   laterAgent,
		Context:   fmt.Sprintf("File conflict on %s with %s", filesStr, earlierAgent),
		Rationale: "Later-assigned agent halted to prevent file corruption",
		Action:    fmt.Sprintf("Halted %s due to file conflict on %s with %s", laterAgent, filesStr, earlierAgent),
	}
	if err := cd.journal.Record(ctx, decision); err != nil {
		cd.logger.Warn("failed to record conflict decision", "error", err)
	}

	cd.logger.Warn("file conflict resolved",
		"halted", laterAgent,
		"kept", earlierAgent,
		"files", filesStr,
	)
	return nil
}

// noopRecorder is a no-op DecisionRecorder for when journal is nil.
type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ types.Decision) error { return nil }

// execGitDiffer implements GitDiffer using os/exec.
type execGitDiffer struct {
	root string
}

// NewExecGitDiffer creates a GitDiffer that runs git diff in the given root directory.
func NewExecGitDiffer(root string) GitDiffer {
	return &execGitDiffer{root: root}
}

// DiffNames returns file paths changed in the working tree.
func (d *execGitDiffer) DiffNames(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	cmd.Dir = d.root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff: %s: %w", stderr.String(), err)
	}

	var files []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
