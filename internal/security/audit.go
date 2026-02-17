package security

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// AuditEntry is a single audit log record written as JSON.
type AuditEntry struct {
	Timestamp  time.Time        `json:"timestamp"`
	AgentID    types.AgentID    `json:"agent_id"`
	Action     ActionType       `json:"action"`
	Target     string           `json:"target"`
	Permission types.Permission `json:"permission"`
	Allowed    bool             `json:"allowed"`
	Reason     string           `json:"reason,omitempty"`
	PhaseID    types.PhaseID    `json:"phase_id,omitempty"`
	PromptNum  int              `json:"prompt_num,omitempty"`
}

// AuditLogger writes append-only JSONL audit entries to a file.
type AuditLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewAuditLogger opens path for append-only JSONL writing. The file is
// created if it does not exist.
func NewAuditLogger(path string) (*AuditLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audit logger: open %s: %w", path, err)
	}
	return &AuditLogger{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// Log writes an audit entry as a single JSON line. If Timestamp is zero it
// is set to the current time. Log is safe for concurrent use.
func (a *AuditLogger) Log(entry AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	return a.enc.Encode(entry)
}

// Close closes the underlying file.
func (a *AuditLogger) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}
