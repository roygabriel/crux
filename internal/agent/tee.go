package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// LogLine is a timestamped line captured from agent output.
type LogLine struct {
	Timestamp time.Time
	Content   string
	AgentID   string
}

// OutputTee writes agent output to pane and structured log.
type OutputTee struct {
	agentID   string
	logDir    string
	logPath   string
	logFile   *os.File
	writer    io.Writer
	stripANSI func([]byte) []byte
	mu        sync.Mutex
}

// NewOutputTee creates an output tee that logs to {logDir}/{agentID}.log.
func NewOutputTee(agentID string, logDir string, paneWriter io.Writer) (*OutputTee, error) {
	if strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("output tee: agentID must not be empty")
	}
	if strings.TrimSpace(logDir) == "" {
		return nil, fmt.Errorf("output tee: logDir must not be empty")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("output tee: mkdir %s: %w", logDir, err)
	}
	logPath := filepath.Join(logDir, agentID+".log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("output tee: open %s: %w", logPath, err)
	}
	if paneWriter == nil {
		paneWriter = io.Discard
	}
	return &OutputTee{
		agentID: agentID,
		logDir:  logDir,
		logPath: logPath,
		logFile: f,
		writer:  paneWriter,
		stripANSI: func(p []byte) []byte {
			return []byte(ansiStripRe.ReplaceAllString(string(p), ""))
		},
	}, nil
}

// Write writes raw output to pane and stripped/timestamped lines to log.
func (t *OutputTee) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.logFile == nil {
		return 0, fmt.Errorf("output tee closed")
	}
	if t.writer != nil {
		if _, err := t.writer.Write(p); err != nil {
			return 0, err
		}
	}
	clean := string(t.stripANSI(p))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if _, err := fmt.Fprintf(t.logFile, "[%s] %s\n", now, line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Close closes the tee log file.
func (t *OutputTee) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.logFile == nil {
		return nil
	}
	err := t.logFile.Close()
	t.logFile = nil
	t.writer = nil
	return err
}

// LogPath returns the log file path.
func (t *OutputTee) LogPath() string {
	return t.logPath
}

// ReadSince returns log lines written after since.
func (t *OutputTee) ReadSince(since time.Time) ([]LogLine, error) {
	t.mu.Lock()
	path := t.logPath
	t.mu.Unlock()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []LogLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "[") {
			continue
		}
		end := strings.Index(line, "] ")
		if end < 0 {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, line[1:end])
		if err != nil {
			ts, err = time.Parse(time.RFC3339, line[1:end])
		}
		if err != nil {
			continue
		}
		if ts.Before(since) || ts.Equal(since) {
			continue
		}
		out = append(out, LogLine{
			Timestamp: ts,
			Content:   line[end+2:],
			AgentID:   t.agentID,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
