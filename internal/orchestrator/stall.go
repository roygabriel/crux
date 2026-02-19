package orchestrator

import (
	"log/slog"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/agent"
)

// StallDetector detects semantic stagnation from fingerprint history.
type StallDetector struct {
	history   []ProgressFingerprint
	threshold int
	logger    *slog.Logger

	logReader      outputLogReader
	lastLogCheck   time.Time
	lastLogLine    string
	repeatLogLines int
}

type outputLogReader interface {
	ReadSince(since time.Time) ([]agent.LogLine, error)
}

// NewStallDetector creates a detector with a consecutive-unchanged threshold.
func NewStallDetector(threshold int, logger *slog.Logger) *StallDetector {
	if threshold <= 0 {
		threshold = 5
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &StallDetector{
		threshold: threshold,
		logger:    logger,
	}
}

// SetLogReader configures a structured log source for supplementary stall checks.
func (d *StallDetector) SetLogReader(reader outputLogReader) {
	d.logReader = reader
}

// Record appends fp and returns true when semantic progress is stalled.
func (d *StallDetector) Record(fp ProgressFingerprint) (stalled bool) {
	d.history = append(d.history, fp)
	maxKeep := d.threshold * 2
	if len(d.history) > maxKeep {
		d.history = d.history[len(d.history)-maxKeep:]
	}

	if len(d.history) < d.threshold {
		return false
	}
	start := len(d.history) - d.threshold
	base := d.history[start]
	for i := start + 1; i < len(d.history); i++ {
		if !base.SameAs(&d.history[i]) {
			return d.outputLooksStalled()
		}
	}
	return true
}

// Reset clears the fingerprint history.
func (d *StallDetector) Reset() {
	d.history = nil
	d.lastLogCheck = time.Time{}
	d.lastLogLine = ""
	d.repeatLogLines = 0
}

// StallDuration reports how long the current stalled window spans.
func (d *StallDetector) StallDuration() time.Duration {
	if len(d.history) < d.threshold {
		return 0
	}
	start := len(d.history) - d.threshold
	base := d.history[start]
	for i := start + 1; i < len(d.history); i++ {
		if !base.SameAs(&d.history[i]) {
			return 0
		}
	}
	return time.Since(base.Timestamp)
}

// History returns a copy of the detector history.
func (d *StallDetector) History() []ProgressFingerprint {
	out := make([]ProgressFingerprint, len(d.history))
	copy(out, d.history)
	return out
}

func (d *StallDetector) outputLooksStalled() bool {
	if d.logReader == nil {
		return false
	}
	since := d.lastLogCheck
	if since.IsZero() {
		since = time.Now().Add(-30 * time.Second)
	}
	lines, err := d.logReader.ReadSince(since)
	d.lastLogCheck = time.Now().UTC()
	if err != nil {
		return false
	}
	for _, line := range lines {
		text := strings.TrimSpace(line.Content)
		if text == "" {
			continue
		}
		if text == d.lastLogLine {
			d.repeatLogLines++
		} else {
			d.lastLogLine = text
			d.repeatLogLines = 1
		}
	}
	return d.repeatLogLines >= d.threshold
}
