package security

import (
	"fmt"
	"log/slog"

	"github.com/roygabriel/crux/pkg/types"
)

// SecurityMiddleware combines permission enforcement with audit logging.
type SecurityMiddleware struct {
	enforcer *Enforcer
	audit    *AuditLogger
	logger   *slog.Logger
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
	return nil
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
