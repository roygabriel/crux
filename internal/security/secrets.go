package security

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Finding represents a secret detected in content or a file.
type Finding struct {
	Pattern string `json:"pattern"`
	Match   string `json:"match"`
	Line    int    `json:"line"`
	File    string `json:"file,omitempty"`
}

// secretPattern pairs a human-readable name with a compiled regex.
type secretPattern struct {
	Name string
	Re   *regexp.Regexp
}

// SecretsScanner detects secrets and sensitive values in text and files.
type SecretsScanner struct {
	patterns []secretPattern
	logger   *slog.Logger
}

// NewSecretsScanner creates a SecretsScanner with the default detection patterns.
func NewSecretsScanner(logger *slog.Logger) *SecretsScanner {
	if logger == nil {
		logger = slog.Default()
	}
	s := &SecretsScanner{logger: logger}
	s.patterns = defaultPatterns()
	return s
}

// AddPattern registers a custom detection pattern. Returns an error if the
// regex is invalid.
func (s *SecretsScanner) AddPattern(name, regex string) error {
	re, err := regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("compile pattern %q: %w", name, err)
	}
	s.patterns = append(s.patterns, secretPattern{Name: name, Re: re})
	return nil
}

// ScanContent scans text line-by-line and returns all findings.
func (s *SecretsScanner) ScanContent(content string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, p := range s.patterns {
			if loc := p.Re.FindStringIndex(line); loc != nil {
				match := line[loc[0]:loc[1]]
				findings = append(findings, Finding{
					Pattern: p.Name,
					Match:   truncateMatch(match),
					Line:    i + 1,
				})
			}
		}
	}
	return findings
}

// ScanFile reads a file and scans its content. The File field is set on each
// finding.
func (s *SecretsScanner) ScanFile(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scan file %s: %w", path, err)
	}
	findings := s.ScanContent(string(data))
	for i := range findings {
		findings[i].File = path
	}
	return findings, nil
}

// ScanDirectory walks root and scans all non-binary, non-excluded files.
func (s *SecretsScanner) ScanDirectory(root string, exclude []string) ([]Finding, error) {
	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}

	var allFindings []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludeSet[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isBinaryFile(path) {
			return nil
		}
		findings, scanErr := s.ScanFile(path)
		if scanErr != nil {
			s.logger.Warn("scan file failed", "path", path, "error", scanErr)
			return nil
		}
		allFindings = append(allFindings, findings...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan directory %s: %w", root, err)
	}
	return allFindings, nil
}

// truncateMatch truncates a match to 16 characters plus "..." if longer.
func truncateMatch(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16] + "..."
}

// isBinaryFile checks if a file appears to be binary by reading the first 512
// bytes and looking for null bytes.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// isKnownSecretsFile checks if a file path is a known secrets file.
func isKnownSecretsFile(path string) bool {
	base := filepath.Base(path)
	known := map[string]bool{
		".env":             true,
		".env.local":       true,
		".env.production":  true,
		"secrets.env":      true,
		"credentials.json": true,
	}
	return known[base]
}

// defaultPatterns returns the built-in secret detection patterns.
func defaultPatterns() []secretPattern {
	return []secretPattern{
		{Name: "AWS Access Key", Re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{Name: "OpenAI API Key", Re: regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)},
		{Name: "GitHub PAT", Re: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`)},
		{Name: "GitHub OAuth", Re: regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`)},
		{Name: "Generic Secret", Re: regexp.MustCompile(`(?i)(password|token|api_key|apikey|secret|credential)\s*[=:]\s*["']?[^\s"']{8,}`)},
		{Name: "PEM Private Key", Re: regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE KEY-----`)},
		{Name: "Env Value", Re: regexp.MustCompile(`^[A-Z_]{2,}=\S{8,}`)},
	}
}

