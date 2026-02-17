package security

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
)

// SecretsManager loads secret key-value pairs from a file and provides
// redaction of secret values in arbitrary text.
type SecretsManager struct {
	secrets map[string]string
	mu      sync.RWMutex
	path    string
	logger  *slog.Logger
}

// NewSecretsManager creates a SecretsManager that loads secrets from path.
func NewSecretsManager(path string, logger *slog.Logger) *SecretsManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &SecretsManager{
		secrets: make(map[string]string),
		path:    path,
		logger:  logger,
	}
}

// Load reads the secrets file (KEY=VALUE format). Lines starting with # and
// empty lines are skipped. A missing file is not an error — Load returns nil.
func (m *SecretsManager) Load() error {
	f, err := os.Open(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open secrets file: %w", err)
	}
	defer f.Close()

	m.mu.Lock()
	defer m.mu.Unlock()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 1 {
			continue
		}
		key := line[:idx]
		value := line[idx+1:]
		if value != "" {
			m.secrets[key] = value
		}
	}
	return scanner.Err()
}

// Redact replaces all loaded secret values in text with [REDACTED]. Values are
// replaced longest-first to handle overlapping substrings correctly.
func (m *SecretsManager) Redact(text string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.secrets) == 0 {
		return text
	}

	// Sort values longest-first so longer matches are replaced before shorter
	// substrings.
	values := make([]string, 0, len(m.secrets))
	for _, v := range m.secrets {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})

	for _, v := range values {
		text = strings.ReplaceAll(text, v, "[REDACTED]")
	}
	return text
}

// Get returns the value for a secret key, or "" if not found.
func (m *SecretsManager) Get(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.secrets[name]
}

// Names returns a sorted list of all loaded secret key names.
func (m *SecretsManager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
