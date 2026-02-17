package types

import "time"

// MessageType classifies the intent of a message between agents.
type MessageType string

const (
	// MessageTask assigns work to an agent.
	MessageTask MessageType = "task"
	// MessageStatus reports agent state changes.
	MessageStatus MessageType = "status"
	// MessageDecision records an orchestration decision.
	MessageDecision MessageType = "decision"
	// MessageError reports an error condition.
	MessageError MessageType = "error"
	// MessageAck acknowledges receipt of a prior message.
	MessageAck MessageType = "ack"
)

// String returns the string representation of a MessageType.
func (t MessageType) String() string { return string(t) }

// Priority controls message processing urgency.
type Priority string

const (
	// PriorityLow is for informational messages that can wait.
	PriorityLow Priority = "low"
	// PriorityNormal is the default priority for routine messages.
	PriorityNormal Priority = "normal"
	// PriorityHigh is for messages requiring prompt attention.
	PriorityHigh Priority = "high"
	// PriorityCritical is for messages requiring immediate attention.
	PriorityCritical Priority = "critical"
)

// String returns the string representation of a Priority.
func (p Priority) String() string { return string(p) }

// Message is the communication envelope exchanged between agents.
type Message struct {
	// ID uniquely identifies this message.
	ID string `json:"id"`
	// From is the sender agent.
	From AgentID `json:"from"`
	// To is the recipient agent.
	To AgentID `json:"to"`
	// Type classifies the message intent.
	Type MessageType `json:"type"`
	// Priority controls processing urgency.
	Priority Priority `json:"priority"`
	// Payload carries the message-specific data.
	Payload any `json:"payload"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
}
