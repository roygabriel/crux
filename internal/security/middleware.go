package security

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// SecurityMiddleware combines permission enforcement with audit logging.
type SecurityMiddleware struct {
	enforcer    *Enforcer
	audit       *AuditLogger
	logger      *slog.Logger
	rateLimiter *RateLimiter
	gitGuard    *GitGuard
	scanner     *SecretsScanner
	secretsMgr  *SecretsManager
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

// SetGitGuard configures the git branch safety guard. When set, Gate()
// enforces branch protection and feature-branch requirements.
func (m *SecurityMiddleware) SetGitGuard(g *GitGuard) {
	m.gitGuard = g
}

// SetSecretsScanner configures the secrets scanner. When set, Gate() blocks
// file reads of known secrets files that contain detected secrets.
func (m *SecurityMiddleware) SetSecretsScanner(s *SecretsScanner) {
	m.scanner = s
}

// SetSecretsManager configures the secrets manager. When set, Gate() redacts
// secret values from audit log targets.
func (m *SecurityMiddleware) SetSecretsManager(mgr *SecretsManager) {
	m.secretsMgr = mgr
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

	// Pre-check: git safety.
	if m.gitGuard != nil {
		if action == ActionGitPush {
			if err := m.gitGuard.ValidateNotProtected(target); err != nil {
				m.logDenial(agentID, action, target, perm, "protected_branch", phaseID, promptNum)
				return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrPermissionDenied)
			}
			if err := m.gitGuard.PrePushCheck(context.Background(), target); err != nil {
				m.logDenial(agentID, action, target, perm, "push_not_feature_branch", phaseID, promptNum)
				return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrPermissionDenied)
			}
		}
		if action == ActionGitCommit {
			branch, err := m.gitGuard.currentBranch(context.Background())
			if err == nil && !strings.HasPrefix(branch, "crux/") {
				m.logDenial(agentID, action, target, perm, "commit_not_feature_branch", phaseID, promptNum)
				return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrPermissionDenied)
			}
		}
	}

	// Pre-check: secrets file read denial.
	if m.scanner != nil && action == ActionFileRead && isKnownSecretsFile(target) {
		findings, err := m.scanner.ScanFile(target)
		if err == nil && len(findings) > 0 {
			m.logDenial(agentID, action, target, perm, "secrets_file_denied", phaseID, promptNum)
			return fmt.Errorf("action %s denied for %s: %w", action, agentID, types.ErrPermissionDenied)
		}
	}

	// Enforcer check.
	result := m.enforcer.Check(perm, action, target)
	interactionID := NewInteractionID(agentID, string(action), target, phaseID, promptNum)

	// New structured taxonomy event: permission check.
	_ = EmitPermissionChecked(context.Background(), m.audit, AuditEvent{
		ID:            NewEventID(),
		InteractionID: interactionID,
		Action:        string(action),
		Target:        target,
		AgentID:       string(agentID),
		PhaseID:       string(phaseID),
		PromptNum:     promptNum,
		Allowed:       result.Allowed,
		Timestamp:     time.Now().UTC(),
		Metadata: map[string]string{
			"permission": string(perm),
		},
	})

	// Redact secrets from audit target.
	auditTarget := target
	if m.secretsMgr != nil {
		auditTarget = m.secretsMgr.Redact(target)
	}

	entry := AuditEntry{
		AgentID:    agentID,
		Action:     action,
		Target:     auditTarget,
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

	// New structured taxonomy event: action attempted after allow.
	_ = EmitActionAttempted(context.Background(), m.audit, AuditEvent{
		ID:            NewEventID(),
		InteractionID: interactionID,
		Action:        string(action),
		Target:        target,
		AgentID:       string(agentID),
		PhaseID:       string(phaseID),
		PromptNum:     promptNum,
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
	})

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
	parsed, ok := ParseActionType(action)
	if !ok {
		return m.Gate(agentID, perm, ActionType(action), target, phaseID, promptNum)
	}
	return m.Gate(agentID, perm, parsed, target, phaseID, promptNum)
}

// GateMessage adapts the full Gate method to the messenger's MessageGate
// interface. It normalizes the provided action to message_send and uses
// fail-open+alert behavior for unknown control-plane action names.
func (m *SecurityMiddleware) GateMessage(
	agentID types.AgentID,
	perm types.Permission,
	action, target string,
) error {
	parsed, ok := ParseActionType(action)
	if !ok {
		m.logger.Warn("unknown control-plane action; allowing dispatch via fail-open policy",
			"agent_id", agentID,
			"raw_action", action,
			"target", target,
		)
		m.emitControlPlaneFailOpenAudit(agentID, perm, action, target, "unknown_action_type")
		return m.GateDispatch(agentID, perm, target)
	}
	if parsed != ActionMessageSend {
		m.logger.Warn("unexpected non-message action for control-plane dispatch; coercing to message_send",
			"agent_id", agentID,
			"raw_action", action,
			"parsed_action", parsed,
			"target", target,
		)
		m.emitControlPlaneFailOpenAudit(agentID, perm, action, target, "non_message_action")
		return m.GateDispatch(agentID, perm, target)
	}
	return m.GateDispatch(agentID, perm, target)
}

// GateDispatch authorizes control-plane dispatch to an agent pane.
func (m *SecurityMiddleware) GateDispatch(
	agentID types.AgentID,
	perm types.Permission,
	target string,
) error {
	return m.Gate(agentID, perm, ActionMessageSend, target, "", 0)
}

// EmitEffectConfirmed records an effect_confirmed taxonomy event.
func (m *SecurityMiddleware) EmitEffectConfirmed(
	agentID types.AgentID,
	action, target string,
	phaseID types.PhaseID,
	promptNum int,
	metadata map[string]string,
) error {
	interactionID := NewInteractionID(agentID, action, target, phaseID, promptNum)
	return EmitEffectConfirmed(context.Background(), m.audit, AuditEvent{
		ID:            NewEventID(),
		InteractionID: interactionID,
		Action:        action,
		Target:        target,
		AgentID:       string(agentID),
		PhaseID:       string(phaseID),
		PromptNum:     promptNum,
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
		Metadata:      metadata,
	})
}

func (m *SecurityMiddleware) emitControlPlaneFailOpenAudit(
	agentID types.AgentID,
	perm types.Permission,
	rawAction, target, reason string,
) {
	if m.audit == nil {
		return
	}

	metadata := map[string]string{
		"policy":        "fail_open_control_plane",
		"reason":        reason,
		"raw_action":    rawAction,
		"normalized_as": string(ActionMessageSend),
	}
	interactionID := NewInteractionID(agentID, rawAction, target, "", 0)
	_ = EmitPermissionChecked(context.Background(), m.audit, AuditEvent{
		ID:            NewEventID(),
		InteractionID: interactionID,
		Action:        string(ActionMessageSend),
		Target:        target,
		AgentID:       string(agentID),
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
		Metadata:      metadata,
	})
	_ = EmitActionAttempted(context.Background(), m.audit, AuditEvent{
		ID:            NewEventID(),
		InteractionID: interactionID,
		Action:        string(ActionMessageSend),
		Target:        target,
		AgentID:       string(agentID),
		Allowed:       true,
		Timestamp:     time.Now().UTC(),
		Metadata:      metadata,
	})
	entry := AuditEntry{
		AgentID:    agentID,
		Action:     ActionMessageSend,
		Target:     target,
		Permission: perm,
		Allowed:    true,
		Reason:     "fail-open control-plane action normalization",
	}
	if err := m.audit.Log(entry); err != nil {
		m.logger.Warn("audit log write failed", "error", err, "agent_id", agentID)
	}
}
