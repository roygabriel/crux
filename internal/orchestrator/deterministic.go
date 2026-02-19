package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/runner"
	"github.com/roygabriel/crux/pkg/types"
)

var errNoDeterministicRunner = errors.New("no deterministic runner")

const (
	defaultDeterministicRunTimeout   = 7 * time.Minute
	defaultDeterministicIdleTimeout  = 90 * time.Second
	defaultStartupInflightGrace      = 20 * time.Second
	defaultFallbackDeterministicSpan = 10 * time.Minute
)

func (o *Orchestrator) dispatchDeterministic(
	ctx context.Context,
	inst *agent.AgentInstance,
	key promptKey,
	rendered string,
) error {
	if !o.deterministicEnabled() {
		return errNoDeterministicRunner
	}
	if o.runners == nil || o.cfg == nil || strings.TrimSpace(o.cfg.Project.Root) == "" {
		return errNoDeterministicRunner
	}
	if !isTaskExecutionRole(inst.Agent.Role) {
		return errNoDeterministicRunner
	}
	taskRunner, ok := o.runners.Get(inst.Agent.Plugin)
	if !ok {
		return errNoDeterministicRunner
	}
	if o.isDeterministicRunActive(inst.Agent.ID) {
		return fmt.Errorf("deterministic run already active for %s", inst.Agent.ID)
	}

	runID := fmt.Sprintf("%s-%d", inst.Agent.ID, time.Now().UnixNano())
	runTimeout := o.deterministicRunTimeout()
	idleTimeout := o.deterministicIdleTimeout()
	startupGrace := o.startupInflightGrace()
	now := time.Now().UTC()
	deadline := now.Add(runTimeout + startupGrace)
	if runTimeout <= 0 {
		deadline = now.Add(defaultFallbackDeterministicSpan + startupGrace)
	}
	envelopePath, err := o.writeTaskEnvelope(taskEnvelope{
		RunID:         runID,
		SessionID:     o.worldState.SessionID,
		AgentID:       inst.Agent.ID,
		Plugin:        inst.Agent.Plugin,
		PhaseID:       key.phaseID,
		PromptNum:     key.promptNum,
		PromptHash:    hashString(rendered),
		ExpectedFiles: o.currentExpectedFiles(),
		Timestamp:     time.Now().UTC(),
	})
	if err != nil {
		o.logger.Warn("failed to write deterministic task envelope",
			"agent_id", inst.Agent.ID,
			"run_id", runID,
			"error", err,
		)
		envelopePath = ""
	}
	if err := o.appendLedger(progressLedgerEntry{
		Timestamp:         now,
		RunID:             runID,
		Event:             "dispatched",
		AgentID:           inst.Agent.ID,
		Plugin:            inst.Agent.Plugin,
		PhaseID:           key.phaseID,
		PromptNum:         key.promptNum,
		TimeoutMS:         runTimeout.Milliseconds(),
		IdleTimeoutMS:     idleTimeout.Milliseconds(),
		TerminationReason: "in_progress",
	}); err != nil {
		o.logger.Warn("failed to append deterministic dispatch ledger entry",
			"agent_id", inst.Agent.ID,
			"run_id", runID,
			"error", err,
		)
	}

	state := deterministicRunState{
		RunID:         runID,
		Key:           key,
		StartedAt:     now,
		Deadline:      deadline,
		Timeout:       runTimeout,
		IdleTimeout:   idleTimeout,
		LastHeartbeat: now,
	}
	o.mu.Lock()
	if o.deterministicRuns == nil {
		o.deterministicRuns = make(map[types.AgentID]deterministicRunState)
	}
	if o.deterministicResults == nil {
		o.deterministicResults = make(chan deterministicRunResult, 32)
	}
	o.deterministicRuns[inst.Agent.ID] = state
	o.mu.Unlock()

	req := runner.Request{
		RunID:        runID,
		AgentID:      string(inst.Agent.ID),
		Prompt:       rendered,
		WorkDir:      o.cfg.Project.Root,
		EnvelopePath: envelopePath,
		Timeout:      runTimeout,
		IdleTimeout:  idleTimeout,
		RunnerMode:   "deterministic",
	}

	go func(agentID types.AgentID, st deterministicRunState) {
		result, runErr := taskRunner.Run(ctx, req)
		select {
		case o.deterministicResults <- deterministicRunResult{
			AgentID: agentID,
			State:   st,
			Result:  result,
			Err:     runErr,
		}:
		case <-ctx.Done():
		}
	}(inst.Agent.ID, state)

	return nil
}

func (o *Orchestrator) drainDeterministicResults(ctx context.Context) {
	o.monitorDeterministicRuns(ctx)
	for {
		select {
		case done := <-o.deterministicResults:
			o.handleDeterministicResult(ctx, done)
		default:
			return
		}
	}
}

func (o *Orchestrator) handleDeterministicResult(ctx context.Context, done deterministicRunResult) {
	active, ok := o.getDeterministicRun(done.AgentID)
	if !ok || active.RunID != done.State.RunID {
		o.logger.Warn("late deterministic result ignored",
			"agent_id", done.AgentID,
			"run_id", done.State.RunID,
		)
		_ = o.appendLedger(progressLedgerEntry{
			Timestamp:         time.Now().UTC(),
			RunID:             done.State.RunID,
			Event:             "late_result_ignored",
			AgentID:           done.AgentID,
			PhaseID:           done.State.Key.phaseID,
			PromptNum:         done.State.Key.promptNum,
			TerminationReason: done.Result.TerminationReason,
		})
		return
	}
	_ = o.clearDeterministicRun(done.AgentID, done.State.RunID)

	inst, err := o.registry.Get(done.AgentID)
	if err != nil {
		o.logger.Warn("deterministic result dropped; agent not found",
			"agent_id", done.AgentID,
			"run_id", done.State.RunID,
			"error", err,
		)
		return
	}

	if err := o.registry.UpdateStatus(done.AgentID, types.StatusIdle); err != nil {
		o.logger.Warn("failed to set agent idle after deterministic run",
			"agent_id", done.AgentID,
			"error", err,
		)
	}
	o.prevStatus[done.AgentID] = types.StatusIdle

	if done.Err != nil || done.Result.ExitCode != 0 {
		runErr := done.Err
		if runErr == nil {
			runErr = fmt.Errorf("exit code %d", done.Result.ExitCode)
		}
		reason := strings.TrimSpace(done.Result.TerminationReason)
		if reason == "" {
			reason = "error"
		}
		_ = o.appendLedger(progressLedgerEntry{
			Timestamp:         time.Now().UTC(),
			RunID:             done.State.RunID,
			Event:             "failed",
			AgentID:           done.AgentID,
			Plugin:            inst.Agent.Plugin,
			PhaseID:           done.State.Key.phaseID,
			PromptNum:         done.State.Key.promptNum,
			DurationMS:        done.Result.Duration.Milliseconds(),
			ExitCode:          done.Result.ExitCode,
			TerminationReason: reason,
			TimeoutMS:         done.State.Timeout.Milliseconds(),
			IdleTimeoutMS:     done.State.IdleTimeout.Milliseconds(),
			Error:             errorText(runErr),
		})
		o.onPromptFailure(ctx, done.AgentID, done.State.Key.phaseID, done.State.Key.promptNum, "deterministic runner "+reason, nil, 0)
		o.clearDispatchAck(done.AgentID)
		o.clearDispatchTracking(done.AgentID)
		o.revertAssignment(done.AgentID, fmt.Errorf("deterministic runner: %w", runErr))
		return
	}

	_ = o.appendLedger(progressLedgerEntry{
		Timestamp:         time.Now().UTC(),
		RunID:             done.State.RunID,
		Event:             "completed",
		AgentID:           done.AgentID,
		Plugin:            inst.Agent.Plugin,
		PhaseID:           done.State.Key.phaseID,
		PromptNum:         done.State.Key.promptNum,
		DurationMS:        done.Result.Duration.Milliseconds(),
		ExitCode:          done.Result.ExitCode,
		TerminationReason: "completed",
		TimeoutMS:         done.State.Timeout.Milliseconds(),
		IdleTimeoutMS:     done.State.IdleTimeout.Milliseconds(),
	})

	spec := o.engine.CurrentPhase()
	prompt := o.engine.CurrentPrompt()
	if spec == nil || prompt == nil || spec.ID != done.State.Key.phaseID || prompt.PromptNumber != done.State.Key.promptNum {
		o.logger.Warn("stale deterministic result ignored",
			"agent_id", done.AgentID,
			"run_id", done.State.RunID,
			"result_phase", done.State.Key.phaseID,
			"result_prompt", done.State.Key.promptNum,
		)
		existing, _ := o.worldState.GetAgent(done.AgentID)
		o.worldState.UpdateAgent(done.AgentID, AgentState{
			Status:        types.StatusIdle,
			PromptDisplay: existing.PromptDisplay,
			Task:          existing.Task,
			PhaseID:       existing.PhaseID,
			AssignedAt:    existing.AssignedAt,
			LastActive:    time.Now().UTC(),
		})
		return
	}

	raw := strings.TrimSpace(done.Result.Output)
	if raw == "" {
		raw = strings.TrimSpace(done.Result.RawOutput)
	}
	if raw != "" {
		o.mu.Lock()
		o.paneContent[done.AgentID] = raw
		if o.firstContentAt[done.AgentID].IsZero() {
			o.firstContentAt[done.AgentID] = time.Now().UTC()
		}
		o.mu.Unlock()
	}

	o.handleCompletion(ctx, inst, raw)
}

func (o *Orchestrator) isDeterministicRunActive(agentID types.AgentID) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.deterministicRuns[agentID]
	return ok
}

func (o *Orchestrator) isPromptDeterministicRunActive(key promptKey) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, run := range o.deterministicRuns {
		if run.Key == key {
			return true
		}
	}
	return false
}

func (o *Orchestrator) monitorDeterministicRuns(ctx context.Context) {
	o.mu.Lock()
	type stale struct {
		agentID types.AgentID
		state   deterministicRunState
	}
	var staleRuns []stale
	now := time.Now().UTC()
	for agentID, st := range o.deterministicRuns {
		if st.Deadline.IsZero() {
			continue
		}
		if now.After(st.Deadline) {
			staleRuns = append(staleRuns, stale{agentID: agentID, state: st})
		}
	}
	o.mu.Unlock()

	for _, s := range staleRuns {
		o.forceDeterministicTimeout(ctx, s.agentID, s.state)
	}
}

func (o *Orchestrator) forceDeterministicTimeout(ctx context.Context, agentID types.AgentID, st deterministicRunState) {
	if !o.clearDeterministicRun(agentID, st.RunID) {
		return
	}
	if o.registry == nil {
		return
	}
	inst, err := o.registry.Get(agentID)
	if err != nil {
		return
	}
	_ = o.registry.UpdateStatus(agentID, types.StatusIdle)
	o.prevStatus[agentID] = types.StatusIdle

	runErr := fmt.Errorf("deterministic in-flight watchdog timeout after %s", time.Since(st.StartedAt).Round(time.Second))
	_ = o.appendLedger(progressLedgerEntry{
		Timestamp:         time.Now().UTC(),
		RunID:             st.RunID,
		Event:             "failed",
		AgentID:           agentID,
		Plugin:            inst.Agent.Plugin,
		PhaseID:           st.Key.phaseID,
		PromptNum:         st.Key.promptNum,
		DurationMS:        time.Since(st.StartedAt).Milliseconds(),
		ExitCode:          124,
		TerminationReason: "orchestrator_inflight_timeout",
		TimeoutMS:         st.Timeout.Milliseconds(),
		IdleTimeoutMS:     st.IdleTimeout.Milliseconds(),
		Error:             runErr.Error(),
	})
	o.onPromptFailure(ctx, agentID, st.Key.phaseID, st.Key.promptNum, "deterministic in-flight timeout", nil, 0)
	o.clearDispatchAck(agentID)
	o.clearDispatchTracking(agentID)
	o.revertAssignment(agentID, runErr)
}

func (o *Orchestrator) getDeterministicRun(agentID types.AgentID) (deterministicRunState, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	st, ok := o.deterministicRuns[agentID]
	return st, ok
}

func (o *Orchestrator) clearDeterministicRun(agentID types.AgentID, runID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	st, ok := o.deterministicRuns[agentID]
	if !ok {
		return false
	}
	if runID != "" && st.RunID != runID {
		return false
	}
	delete(o.deterministicRuns, agentID)
	return true
}

func (o *Orchestrator) deterministicEnabled() bool {
	if o.cfg == nil {
		return true
	}
	if !o.cfg.Execution.DeterministicEnabled &&
		strings.TrimSpace(o.cfg.Execution.RunTimeout) == "" &&
		strings.TrimSpace(o.cfg.Execution.IdleTimeout) == "" &&
		strings.TrimSpace(o.cfg.Execution.StartupInflightGrace) == "" {
		// Backward compatibility for tests/configs that instantiate Config
		// literals without loading defaults.
		return true
	}
	return o.cfg.Execution.DeterministicEnabled
}

func (o *Orchestrator) deterministicRunTimeout() time.Duration {
	if o.cfg == nil {
		return defaultDeterministicRunTimeout
	}
	return parsePositiveDuration(o.cfg.Execution.RunTimeout, defaultDeterministicRunTimeout)
}

func (o *Orchestrator) deterministicIdleTimeout() time.Duration {
	if o.cfg == nil {
		return defaultDeterministicIdleTimeout
	}
	return parsePositiveDuration(o.cfg.Execution.IdleTimeout, defaultDeterministicIdleTimeout)
}

func (o *Orchestrator) startupInflightGrace() time.Duration {
	if o.cfg == nil {
		return defaultStartupInflightGrace
	}
	return parsePositiveDuration(o.cfg.Execution.StartupInflightGrace, defaultStartupInflightGrace)
}

func parsePositiveDuration(raw string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func (o *Orchestrator) currentExpectedFiles() []string {
	if o.engine == nil {
		return nil
	}
	spec := o.engine.CurrentPhase()
	if spec == nil {
		return nil
	}
	files := make([]string, 0, len(spec.FilesNew)+len(spec.FilesModified))
	files = append(files, spec.FilesNew...)
	files = append(files, spec.FilesModified...)
	return files
}

func (o *Orchestrator) writeTaskEnvelope(env taskEnvelope) (string, error) {
	if strings.TrimSpace(o.envelopeDir) == "" {
		return "", nil
	}
	if err := os.MkdirAll(o.envelopeDir, 0o755); err != nil {
		return "", fmt.Errorf("create envelope dir: %w", err)
	}
	envPath := filepath.Join(o.envelopeDir, env.RunID+".json")
	env.EnvelopePath = envPath
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	if err := os.WriteFile(envPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write envelope: %w", err)
	}
	return envPath, nil
}

func (o *Orchestrator) appendLedger(entry progressLedgerEntry) error {
	if strings.TrimSpace(o.ledgerPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(o.ledgerPath), 0o755); err != nil {
		return fmt.Errorf("create ledger dir: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal ledger entry: %w", err)
	}
	f, err := os.OpenFile(o.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append ledger: %w", err)
	}
	return nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
