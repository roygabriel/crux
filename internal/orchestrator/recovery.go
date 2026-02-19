package orchestrator

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// PromptAttemptState indicates where a prompt is in retry progression.
type PromptAttemptState int

const (
	AttemptFirst      PromptAttemptState = 1
	AttemptSecond     PromptAttemptState = 2
	AttemptThird      PromptAttemptState = 3
	AttemptQuarantine PromptAttemptState = 4
)

// RecoveryAction tells the orchestrator what to do next after a failure.
type RecoveryAction int

const (
	ActionRetry RecoveryAction = iota
	ActionReassign
	ActionQuarantine
)

// RetryContext includes failure details for the next retry prompt.
type RetryContext struct {
	AttemptNumber   int
	PreviousAgentID string
	FailureReason   string
	MissingFiles    []string
	GateFailures    []string
	StallDuration   time.Duration
	ProgressScore   float64
	Guidance        string
}

// QuarantineMetadata stores prompt state when terminally quarantined.
type QuarantineMetadata struct {
	PhaseID         types.PhaseID
	PromptNum       int
	TotalAttempts   int
	LastAgentID     string
	LastEvidence    *FilesystemEvidence
	LastFingerprint *ProgressFingerprint
	Reason          string
	QuarantinedAt   time.Time
	CompletedFiles  []string
	PartialProgress string
	Recommendation  string
}

// BackoffConfig configures retry delay strategy.
type BackoffConfig struct {
	InitialInterval time.Duration
	Coefficient     float64
	MaxInterval     time.Duration
	JitterFraction  float64
}

type promptAttemptTracker struct {
	attempts       int
	agentsTried    []string
	lastRetry      *RetryContext
	lastEvidence   *FilesystemEvidence
	lastFingerprint *ProgressFingerprint
}

// RecoveryManager tracks prompt retry state and quarantine outcomes.
type RecoveryManager struct {
	attempts    map[promptKey]*promptAttemptTracker
	quarantined map[promptKey]*QuarantineMetadata
	backoff     BackoffConfig
	logger      *slog.Logger
}

// NewRecoveryManager creates a recovery manager with sane defaults.
func NewRecoveryManager(cfg BackoffConfig, logger *slog.Logger) *RecoveryManager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.InitialInterval <= 0 {
		cfg.InitialInterval = 2 * time.Second
	}
	if cfg.Coefficient <= 0 {
		cfg.Coefficient = 2.0
	}
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = 30 * time.Second
	}
	if cfg.JitterFraction <= 0 {
		cfg.JitterFraction = 0.2
	}
	return &RecoveryManager{
		attempts:    make(map[promptKey]*promptAttemptTracker),
		quarantined: make(map[promptKey]*QuarantineMetadata),
		backoff:     cfg,
		logger:      logger,
	}
}

// RecordFailure records a prompt failure and computes the next action.
func (m *RecoveryManager) RecordFailure(
	key promptKey,
	agentID string,
	failureReason string,
	evidence *FilesystemEvidence,
	fingerprint *ProgressFingerprint,
	stallDuration time.Duration,
) RecoveryAction {
	tr := m.attempts[key]
	if tr == nil {
		tr = &promptAttemptTracker{}
		m.attempts[key] = tr
	}
	tr.attempts++
	tr.agentsTried = append(tr.agentsTried, agentID)
	tr.lastEvidence = evidence
	tr.lastFingerprint = fingerprint

	ctx := &RetryContext{
		AttemptNumber:   tr.attempts,
		PreviousAgentID: agentID,
		FailureReason:   failureReason,
		StallDuration:   stallDuration,
	}
	if evidence != nil {
		ctx.MissingFiles = append(ctx.MissingFiles, evidence.Missing...)
	}
	if fingerprint != nil {
		ctx.ProgressScore = fingerprint.ProgressScore
	}
	ctx.Guidance = m.buildGuidance(tr.attempts, ctx)
	tr.lastRetry = ctx

	switch tr.attempts {
	case 1:
		return ActionRetry
	case 2:
		return ActionReassign
	default:
		q := &QuarantineMetadata{
			PhaseID:         key.phaseID,
			PromptNum:       key.promptNum,
			TotalAttempts:   tr.attempts,
			LastAgentID:     agentID,
			LastEvidence:    evidence,
			LastFingerprint: fingerprint,
			Reason:          failureReason,
			QuarantinedAt:   time.Now().UTC(),
		}
		if evidence != nil {
			q.CompletedFiles = append(q.CompletedFiles, evidence.Found...)
			q.PartialProgress = evidence.Summary()
		}
		q.Recommendation = fmt.Sprintf(
			"Prompt %s-%d quarantined after %d failed attempts. Recommended action: verify phase spec file paths and rerun with resume.",
			key.phaseID, key.promptNum, tr.attempts,
		)
		m.quarantined[key] = q
		return ActionQuarantine
	}
}

// IsQuarantined reports whether this prompt key is terminally quarantined.
func (m *RecoveryManager) IsQuarantined(key promptKey) bool {
	_, ok := m.quarantined[key]
	return ok
}

// GetRetryContext returns the latest retry context for a prompt.
func (m *RecoveryManager) GetRetryContext(key promptKey) *RetryContext {
	if tr := m.attempts[key]; tr != nil && tr.lastRetry != nil {
		cp := *tr.lastRetry
		cp.MissingFiles = append([]string(nil), tr.lastRetry.MissingFiles...)
		cp.GateFailures = append([]string(nil), tr.lastRetry.GateFailures...)
		return &cp
	}
	return nil
}

// GetQuarantineMetadata returns metadata for a quarantined prompt.
func (m *RecoveryManager) GetQuarantineMetadata(key promptKey) *QuarantineMetadata {
	q := m.quarantined[key]
	if q == nil {
		return nil
	}
	cp := *q
	cp.CompletedFiles = append([]string(nil), q.CompletedFiles...)
	return &cp
}

// ClearPrompt clears retry/quarantine state for a prompt key.
func (m *RecoveryManager) ClearPrompt(key promptKey) {
	delete(m.attempts, key)
	delete(m.quarantined, key)
}

// NextBackoffDelay computes exponential backoff with jitter for this prompt.
func (m *RecoveryManager) NextBackoffDelay(key promptKey) time.Duration {
	tr := m.attempts[key]
	if tr == nil || tr.attempts <= 0 {
		return m.backoff.InitialInterval
	}
	pow := math.Pow(m.backoff.Coefficient, float64(maxInt(tr.attempts-1, 0)))
	delay := time.Duration(float64(m.backoff.InitialInterval) * pow)
	if delay > m.backoff.MaxInterval {
		delay = m.backoff.MaxInterval
	}
	jitter := (rand.Float64()*2 - 1) * m.backoff.JitterFraction
	delay = time.Duration(float64(delay) * (1 + jitter))
	if delay < 0 {
		delay = m.backoff.InitialInterval
	}
	return delay
}

func (m *RecoveryManager) buildGuidance(attempt int, ctx *RetryContext) string {
	missing := "none"
	if len(ctx.MissingFiles) > 0 {
		missing = strings.Join(ctx.MissingFiles, ", ")
	}
	switch attempt {
	case 1:
		return fmt.Sprintf("Previous attempt failed (%s). Missing files: %s. Create missing files before any other work.", ctx.FailureReason, missing)
	case 2:
		return fmt.Sprintf("Second attempt failed. Missing files: %s. Focus on creating each missing file and verify existence after each step.", missing)
	default:
		return fmt.Sprintf("Third attempt with stronger recovery. Previous agents failed to produce: %s. Start from clean minimal file creation order.", missing)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
