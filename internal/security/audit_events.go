package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// AuditEventType classifies an audit event in the effect-verification taxonomy.
type AuditEventType string

const (
	// AuditPermissionChecked indicates authorization was evaluated.
	AuditPermissionChecked AuditEventType = "permission_checked"
	// AuditActionAttempted indicates a tool action was initiated.
	AuditActionAttempted AuditEventType = "action_attempted"
	// AuditEffectConfirmed indicates an effect was verified in the environment.
	AuditEffectConfirmed AuditEventType = "effect_confirmed"
)

// AuditEvent is a structured taxonomy event written to the audit stream.
type AuditEvent struct {
	ID            string            `json:"id"`
	InteractionID string            `json:"interaction_id"`
	Type          AuditEventType    `json:"event_type"`
	Action        string            `json:"action"`
	Target        string            `json:"target"`
	AgentID       string            `json:"agent_id"`
	PhaseID       string            `json:"phase_id,omitempty"`
	PromptNum     int               `json:"prompt_num,omitempty"`
	Allowed       bool              `json:"allowed"`
	Timestamp     time.Time         `json:"timestamp"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// NewInteractionID builds a stable correlation ID for permission/attempt/effect.
func NewInteractionID(agentID types.AgentID, action, target string, phaseID types.PhaseID, promptNum int) string {
	key := fmt.Sprintf("%s|%s|%s|%s|%d", agentID, action, target, phaseID, promptNum)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// NewEventID builds a unique event ID for this event emission.
func NewEventID() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:8])
}

// EmitPermissionChecked emits a permission_checked event.
func EmitPermissionChecked(_ context.Context, logger *AuditLogger, event AuditEvent) error {
	if logger == nil {
		return nil
	}
	event.Type = AuditPermissionChecked
	return logger.LogEvent(event)
}

// EmitActionAttempted emits an action_attempted event.
func EmitActionAttempted(_ context.Context, logger *AuditLogger, event AuditEvent) error {
	if logger == nil {
		return nil
	}
	event.Type = AuditActionAttempted
	return logger.LogEvent(event)
}

// EmitEffectConfirmed emits an effect_confirmed event.
func EmitEffectConfirmed(_ context.Context, logger *AuditLogger, event AuditEvent) error {
	if logger == nil {
		return nil
	}
	event.Type = AuditEffectConfirmed
	return logger.LogEvent(event)
}
