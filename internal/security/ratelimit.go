package security

import (
	"log/slog"
	"sync"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// RateLimiter enforces per-agent command rate limits and file modification caps.
type RateLimiter struct {
	commandsPerMin  int
	filesPerSession int
	logger          *slog.Logger
	nowFunc         func() time.Time
	mu              sync.Mutex
	commandLog      map[types.AgentID][]time.Time
	fileTracker     map[types.AgentID]map[string]struct{}
}

// NewRateLimiter creates a RateLimiter with the given limits. A limit of 0
// disables that check.
func NewRateLimiter(commandsPerMin, filesPerSession int, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RateLimiter{
		commandsPerMin:  commandsPerMin,
		filesPerSession: filesPerSession,
		logger:          logger,
		nowFunc:         time.Now,
		commandLog:      make(map[types.AgentID][]time.Time),
		fileTracker:     make(map[types.AgentID]map[string]struct{}),
	}
}

// CheckCommand returns ErrRateLimited if agentID has exceeded the commands-per-
// minute limit within a sliding 60-second window. A limit of 0 disables the check.
func (r *RateLimiter) CheckCommand(agentID types.AgentID) error {
	if r.commandsPerMin == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.nowFunc()
	cutoff := now.Add(-60 * time.Second)

	r.pruneCommandsLocked(agentID, cutoff)

	if len(r.commandLog[agentID]) >= r.commandsPerMin {
		r.logger.Warn("rate limit exceeded",
			"agent_id", agentID,
			"commands_last_min", len(r.commandLog[agentID]),
			"limit", r.commandsPerMin,
		)
		return types.ErrRateLimited
	}
	return nil
}

// RecordCommand appends a timestamp for agentID and prunes entries older than 60s.
func (r *RateLimiter) RecordCommand(agentID types.AgentID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.nowFunc()
	cutoff := now.Add(-60 * time.Second)
	r.pruneCommandsLocked(agentID, cutoff)
	r.commandLog[agentID] = append(r.commandLog[agentID], now)
}

// CheckFileModification returns ErrFileLimit if recording a new unique file for
// agentID would exceed the session file limit. Already-tracked paths are not
// counted again. A limit of 0 disables the check.
func (r *RateLimiter) CheckFileModification(agentID types.AgentID, path string) error {
	if r.filesPerSession == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	files := r.fileTracker[agentID]
	if files != nil {
		if _, ok := files[path]; ok {
			return nil
		}
	}

	count := len(files)
	if count >= r.filesPerSession {
		r.logger.Warn("file limit exceeded",
			"agent_id", agentID,
			"files_this_session", count,
			"limit", r.filesPerSession,
		)
		return types.ErrFileLimit
	}
	return nil
}

// RecordFileModification adds path to the tracked set for agentID.
func (r *RateLimiter) RecordFileModification(agentID types.AgentID, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.fileTracker[agentID] == nil {
		r.fileTracker[agentID] = make(map[string]struct{})
	}
	r.fileTracker[agentID][path] = struct{}{}
}

// ResetSession clears both command and file tracking for agentID.
func (r *RateLimiter) ResetSession(agentID types.AgentID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.commandLog, agentID)
	delete(r.fileTracker, agentID)
}

// Stats returns the number of commands in the last minute and unique files
// modified this session for agentID.
func (r *RateLimiter) Stats(agentID types.AgentID) (commandsLastMin, filesThisSession int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.nowFunc()
	cutoff := now.Add(-60 * time.Second)
	r.pruneCommandsLocked(agentID, cutoff)

	return len(r.commandLog[agentID]), len(r.fileTracker[agentID])
}

// pruneCommandsLocked removes command timestamps older than cutoff. Must be
// called with r.mu held.
func (r *RateLimiter) pruneCommandsLocked(agentID types.AgentID, cutoff time.Time) {
	entries := r.commandLog[agentID]
	i := 0
	for i < len(entries) && entries[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		r.commandLog[agentID] = entries[i:]
	}
}
