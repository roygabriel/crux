package security

import (
	"fmt"
	"log/slog"

	"github.com/roygabriel/crux/pkg/types"
)

// SecurityMiddleware combines permission enforcement with audit logging.
type SecurityMiddleware struct {
	enforcer    *Enforcer
	audit       *AuditLogger
	logger      *slog.Logger
	rateLimiter *RateLimiter
}

// NewSecurityMiddleware creates a SecurityMiddleware with the given enforcer
// and audit logger.
func NewSecurityMiddleware(enforcer *Enforcer, audit *AuditLogger, logger *slog.Logger) *SecurityMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &SecurityMiddleware{
		enforcer: enforcer,
		audit:    audit,
		logger:   logger,
	}
}

// SetRateLimiter configures the per-agent rate limiter. When set, Gate()
// enforces command-per-minute and file-per-session limits.
func (m *SecurityMiddleware) SetRateLimiter(rl *RateLimiter) {
	m.rateLimiter = rl
}

// Gate checks whether agentID with perm may perform action on target. It logs
// both allowed and denied actions to the audit log. Returns nil if allowed,
// or an error wrapping types.ErrPermissionDenied if denied.
func (m *SecurityMiddleware) Gate(
	agentID types.AgentID,
	perm types.Permission,
	action ActionType,
	target string,
	phaseID types.PhaseID,
	promptNum int,
) error {
	// Pre-check: rate limiter.
	if m.rateLimiter != nil {
		if action == ActionShellExec {
			if err := m.rateLimiter.CheckCommand(agentID); err != nil {
				m.logDenial(agentID, action, target, perm, "rate_limited", phaseID, promptNum)
				return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrRateLimited)
			}
		}
		if action == ActionFileWrite {
			if err := m.rateLimiter.CheckFileModification(agentID, target); err != nil {
				m.logDenial(agentID, action, target, perm, "file_limit", phaseID, promptNum)
				return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrFileLimit)
			}
		}
	}

	// Enforcer check.
	result := m.enforcer.Check(perm, action, target)

	entry := AuditEntry{
		AgentID:    agentID,
		Action:     action,
		Target:     target,
		Permission: perm,
		Allowed:    result.Allowed,
		Reason:     result.Reason,
		PhaseID:    phaseID,
		PromptNum:  promptNum,
	}

	if err := m.audit.Log(entry); err != nil {
		m.logger.Warn("audit log write failed", "error", err, "agent_id", agentID)
	}

	if !result.Allowed {
		return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrPermissionDenied)
	}

	// Post-record: rate limiter.
	if m.rateLimiter != nil {
		if action == ActionShellExec {
			m.rateLimiter.RecordCommand(agentID)
		}
		if action == ActionFileWrite {
			m.rateLimiter.RecordFileModification(agentID, target)
		}
	}

	return nil
}

// logDenial writes an audit entry for a denial from rate limiter, git guard,
// or secrets scanner.
func (m *SecurityMiddleware) logDenial(
	agentID types.AgentID,
	action ActionType,
	target string,
	perm types.Permission,
	reason string,
	phaseID types.PhaseID,
	promptNum int,
) {
	entry := AuditEntry{
		AgentID:    agentID,
		Action:     action,
		Target:     target,
		Permission: perm,
		Allowed:    false,
		Reason:     reason,
		PhaseID:    phaseID,
		PromptNum:  promptNum,
	}
	if err := m.audit.Log(entry); err != nil {
		m.logger.Warn("audit log write failed", "error", err, "agent_id", agentID)
	}
}

// GateString adapts the full Gate method for consumers that use string-typed
// actions (e.g., the orchestrator's SecurityGate interface).
func (m *SecurityMiddleware) GateString(
	agentID types.AgentID,
	perm types.Permission,
	action, target string,
	phaseID types.PhaseID,
	promptNum int,
) error {
	return m.Gate(agentID, perm, ActionType(action), target, phaseID, promptNum)
}

// GateMessage adapts the full Gate method to the messenger's MessageGate
// interface. It uses a fixed ActionType and empty phase context.
func (m *SecurityMiddleware) GateMessage(
	agentID types.AgentID,
	perm types.Permission,
	action, target string,
) error {
	return m.Gate(agentID, perm, ActionType(action), target, "", 0)
}
