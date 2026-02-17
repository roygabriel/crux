package phase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/pkg/types"
)

// PhaseProgress tracks the execution state of a single phase.
type PhaseProgress struct {
	// Spec is the parsed phase specification.
	Spec *PhaseSpec `json:"spec"`
	// Prompts lists the prompt contracts for this phase.
	Prompts []PromptContract `json:"prompts,omitempty"`
	// CompletedPrompts is the number of prompts that have passed verification.
	CompletedPrompts int `json:"completed_prompts"`
	// GatesPassed is the count of exit criteria gates that have passed.
	GatesPassed int `json:"gates_passed"`
	// GatesTotal is the total count of exit criteria gates.
	GatesTotal int `json:"gates_total"`
}

// Engine orchestrates phase-by-phase, prompt-by-prompt workflow execution.
// It loads phase specs and prompt docs, maintains a topologically sorted
// execution order, and enforces verification gates before advancing.
type Engine struct {
	specDir    string
	gateRunner *GateRunner
	journal    *journal.Journal
	logger     *slog.Logger

	// Loaded state.
	specs     map[types.PhaseID]*PhaseSpec
	prompts   map[types.PhaseID][]PromptContract
	topoOrder []types.PhaseID
	progress  map[types.PhaseID]*PhaseProgress
}

// NewEngine creates an Engine that reads specs from specDir.
// The journal parameter is optional — pass nil if decision recording is not needed.
func NewEngine(specDir string, gateRunner *GateRunner, j *journal.Journal, logger *slog.Logger) (*Engine, error) {
	if specDir == "" {
		return nil, fmt.Errorf("specDir is required")
	}
	if gateRunner == nil {
		return nil, fmt.Errorf("gateRunner is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		specDir:    specDir,
		gateRunner: gateRunner,
		journal:    j,
		logger:     logger,
		specs:      make(map[types.PhaseID]*PhaseSpec),
		prompts:    make(map[types.PhaseID][]PromptContract),
		progress:   make(map[types.PhaseID]*PhaseProgress),
	}, nil
}

// LoadAll discovers and parses all PHASE*.md specs and their corresponding
// PHASE*-PROMPT.md docs from the spec directory. It builds the dependency
// graph and computes topological execution order.
func (e *Engine) LoadAll() error {
	// Glob for spec files, excluding prompt docs.
	pattern := filepath.Join(e.specDir, "PHASE*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing specs: %w", err)
	}

	for _, path := range matches {
		base := filepath.Base(path)
		if strings.Contains(base, "-PROMPT") {
			continue
		}

		spec, err := ParseSpec(path)
		if err != nil {
			return fmt.Errorf("parsing spec %s: %w", base, err)
		}
		e.specs[spec.ID] = spec

		// Try to load corresponding prompt doc.
		promptPath := strings.TrimSuffix(path, ".md") + "-PROMPT.md"
		prompts, err := ParsePromptDoc(promptPath)
		if err != nil {
			// Missing prompt doc is not fatal.
			e.logger.Debug("no prompt doc found", "phase", spec.ID, "path", promptPath)
		} else {
			e.prompts[spec.ID] = prompts
		}

		e.progress[spec.ID] = &PhaseProgress{
			Spec:       spec,
			Prompts:    e.prompts[spec.ID],
			GatesTotal: len(spec.ExitCriteria),
		}
	}

	// Validate dependencies and compute topological order.
	order, err := e.topoSort()
	if err != nil {
		return err
	}
	e.topoOrder = order

	return nil
}

// CurrentPhase returns the first phase in topological order whose
// dependencies are all complete and that is not itself complete.
// Returns nil if all phases are complete.
func (e *Engine) CurrentPhase() *PhaseSpec {
	for _, id := range e.topoOrder {
		prog := e.progress[id]
		if e.isPhaseComplete(id) {
			continue
		}
		if e.depsComplete(id) {
			return prog.Spec
		}
	}
	return nil
}

// CurrentPrompt returns the next unfinished prompt contract for the
// current phase. Returns nil if no phase is active or all prompts are done.
func (e *Engine) CurrentPrompt() *PromptContract {
	spec := e.CurrentPhase()
	if spec == nil {
		return nil
	}
	prog := e.progress[spec.ID]
	prompts := e.prompts[spec.ID]
	if prog.CompletedPrompts >= len(prompts) {
		return nil
	}
	return &prompts[prog.CompletedPrompts]
}

// Advance runs the verification gates for the current prompt. If all
// pass, it increments the completed prompt counter. If any fail, it
// returns an error wrapping types.ErrGateFailed.
func (e *Engine) Advance(ctx context.Context) error {
	spec := e.CurrentPhase()
	if spec == nil {
		return fmt.Errorf("no active phase to advance")
	}

	prompt := e.CurrentPrompt()
	if prompt == nil {
		return fmt.Errorf("no current prompt for phase %s", spec.ID)
	}

	gates := prompt.Verification
	if len(gates) == 0 {
		// No gates means auto-pass.
		e.advancePrompt(spec.ID)
		return nil
	}

	results, err := e.gateRunner.RunAll(ctx, gates)
	if err != nil {
		return fmt.Errorf("running gates for phase %s prompt %d: %w", spec.ID, prompt.PromptNumber, err)
	}

	for _, r := range results {
		if !r.Passed {
			return fmt.Errorf("gate %q failed for phase %s prompt %d: %w",
				r.Gate.Command, spec.ID, prompt.PromptNumber, types.ErrGateFailed)
		}
	}

	e.advancePrompt(spec.ID)
	return nil
}

// ForceAdvance skips gate verification and advances the prompt counter
// for the given phase. This is the human override escape hatch.
func (e *Engine) ForceAdvance(_ context.Context, phaseID types.PhaseID) error {
	prog, ok := e.progress[phaseID]
	if !ok {
		return fmt.Errorf("unknown phase %s: %w", phaseID, types.ErrNotFound)
	}
	prompts := e.prompts[phaseID]
	if prog.CompletedPrompts >= len(prompts) {
		return fmt.Errorf("phase %s has no more prompts to advance", phaseID)
	}
	e.advancePrompt(phaseID)
	return nil
}

// ValidateParallelism checks whether the given phases can execute in
// parallel by detecting overlapping file sets (FilesNew ∪ FilesModified).
func (e *Engine) ValidateParallelism(phases []types.PhaseID) error {
	type fileOwner struct {
		file    string
		phaseID types.PhaseID
	}

	seen := make(map[string]types.PhaseID)
	var conflicts []string

	for _, id := range phases {
		spec, ok := e.specs[id]
		if !ok {
			return fmt.Errorf("unknown phase %s: %w", id, types.ErrNotFound)
		}
		files := make([]string, 0, len(spec.FilesNew)+len(spec.FilesModified))
		files = append(files, spec.FilesNew...)
		files = append(files, spec.FilesModified...)

		for _, f := range files {
			if owner, exists := seen[f]; exists {
				conflicts = append(conflicts, fmt.Sprintf("%s: phases %s and %s", f, owner, id))
			} else {
				seen[f] = id
			}
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("file conflicts detected: %s", strings.Join(conflicts, "; "))
	}
	return nil
}

// PhaseOrder returns the topologically sorted phase IDs.
func (e *Engine) PhaseOrder() []types.PhaseID { return e.topoOrder }

// Progress returns the current progress map for all loaded phases.
func (e *Engine) Progress() map[types.PhaseID]PhaseProgress {
	result := make(map[types.PhaseID]PhaseProgress, len(e.progress))
	for id, prog := range e.progress {
		result[id] = *prog
	}
	return result
}

func (e *Engine) advancePrompt(phaseID types.PhaseID) {
	prog := e.progress[phaseID]
	prog.CompletedPrompts++
	e.logger.Info("advanced prompt",
		"phase", phaseID,
		"completed", prog.CompletedPrompts,
		"total", len(e.prompts[phaseID]),
	)
}

func (e *Engine) isPhaseComplete(id types.PhaseID) bool {
	prog := e.progress[id]
	prompts := e.prompts[id]
	if len(prompts) == 0 {
		// Phases without prompts are considered complete when status says so.
		return prog.Spec.Status == types.PhaseComplete
	}
	return prog.CompletedPrompts >= len(prompts)
}

func (e *Engine) depsComplete(id types.PhaseID) bool {
	spec := e.specs[id]
	for _, dep := range spec.DependsOn {
		if !e.isPhaseComplete(dep) {
			return false
		}
	}
	return true
}

// topoSort performs Kahn's algorithm for topological sorting with cycle detection.
func (e *Engine) topoSort() ([]types.PhaseID, error) {
	// Validate all dependencies exist.
	for id, spec := range e.specs {
		for _, dep := range spec.DependsOn {
			if _, ok := e.specs[dep]; !ok {
				return nil, fmt.Errorf("phase %s depends on unknown phase %s", id, dep)
			}
		}
	}

	// Build in-degree map.
	inDegree := make(map[types.PhaseID]int)
	dependents := make(map[types.PhaseID][]types.PhaseID)

	for id := range e.specs {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
	}
	for id, spec := range e.specs {
		for _, dep := range spec.DependsOn {
			inDegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	// Seed queue with zero in-degree nodes, sorted for determinism.
	var queue []types.PhaseID
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return string(queue[i]) < string(queue[j])
	})

	var order []types.PhaseID
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		deps := dependents[node]
		sort.Slice(deps, func(i, j int) bool {
			return string(deps[i]) < string(deps[j])
		})
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(order) != len(e.specs) {
		return nil, errors.New("dependency cycle detected in phase specifications")
	}

	return order, nil
}
