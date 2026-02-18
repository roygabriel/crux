package testutil

import (
	"context"
	"fmt"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/pkg/types"
)

// NoopDecisionRecorder satisfies phase.DecisionRecorder with no-op behavior.
type NoopDecisionRecorder struct{}

// Record is a no-op.
func (NoopDecisionRecorder) Record(_ context.Context, _ types.Decision) error { return nil }

// NoopDecisionSearcher satisfies phase.DecisionSearcher with no-op behavior.
type NoopDecisionSearcher struct{}

// SemanticSearch returns nil results.
func (NoopDecisionSearcher) SemanticSearch(_ context.Context, _ string, _ int) ([]types.Decision, error) {
	return nil, nil
}

// NoopWorkNotes satisfies phase.WorkNotesManager with no-op behavior.
type NoopWorkNotes struct{}

// Read returns an empty WorkNotes.
func (NoopWorkNotes) Read(_ string) (*worknotes.WorkNotes, error) {
	return &worknotes.WorkNotes{}, nil
}

// Init is a no-op.
func (NoopWorkNotes) Init(_, _ string) error { return nil }

// AppendDecision is a no-op.
func (NoopWorkNotes) AppendDecision(_, _, _ string) error { return nil }

// AppendSession is a no-op.
func (NoopWorkNotes) AppendSession(_ string, _ worknotes.SessionLogEntry) error { return nil }

// UpdatePromptProgress is a no-op.
func (NoopWorkNotes) UpdatePromptProgress(_ string, _ int, _ bool) error { return nil }

// UpdateStatus is a no-op.
func (NoopWorkNotes) UpdateStatus(_, _ string) error { return nil }

// Render returns an empty string.
func (NoopWorkNotes) Render(_ *worknotes.WorkNotes) string { return "" }

// NoopBankSummarizer satisfies phase.BankSummarizer with no-op behavior.
type NoopBankSummarizer struct{}

// Summary returns an empty string.
func (NoopBankSummarizer) Summary() (string, error) { return "", nil }

// NoopSecurityGate satisfies orchestrator.SecurityGate and always allows.
type NoopSecurityGate struct{}

// Gate always returns nil (allowed).
func (NoopSecurityGate) Gate(_ types.AgentID, _ types.Permission, _, _ string, _ types.PhaseID, _ int) error {
	return nil
}

// DenyingSecurityGate satisfies orchestrator.SecurityGate and always denies.
type DenyingSecurityGate struct {
	Err error
}

// NewDenyingSecurityGate creates a DenyingSecurityGate with the given error.
// If err is nil, a default error is used.
func NewDenyingSecurityGate(err error) *DenyingSecurityGate {
	if err == nil {
		err = fmt.Errorf("security gate denied: %w", types.ErrPermissionDenied)
	}
	return &DenyingSecurityGate{Err: err}
}

// Gate always returns the configured error (denied).
func (d *DenyingSecurityGate) Gate(_ types.AgentID, _ types.Permission, _, _ string, _ types.PhaseID, _ int) error {
	return d.Err
}
