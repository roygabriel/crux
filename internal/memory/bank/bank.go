// Package bank provides file-based memory storage using markdown files.
// Each file follows the Cline Memory Bank pattern with titled sections.
// Files are human-readable, git-tracked, and designed for LLM context injection.
package bank

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors for bank operations.
var (
	// ErrFileNotFound indicates the requested memory bank file does not exist.
	ErrFileNotFound = errors.New("memory bank file not found")
	// ErrSectionNotFound indicates the requested section header was not found in the file.
	ErrSectionNotFound = errors.New("section not found")
)

// knownFiles is the set of valid memory bank filenames for validation.
var knownFiles map[string]bool

func init() {
	knownFiles = make(map[string]bool, len(templateFiles))
	for name := range templateFiles {
		knownFiles[name] = true
	}
}

// Bank manages a directory of markdown memory bank files.
type Bank struct {
	rootDir string
	logger  *slog.Logger
}

// NewBank creates a new Bank rooted at the given directory.
func NewBank(rootDir string, logger *slog.Logger) *Bank {
	return &Bank{
		rootDir: rootDir,
		logger:  logger,
	}
}

// Init creates the memory bank directory and writes template files for any
// that do not already exist. It is safe to call multiple times (idempotent).
func (b *Bank) Init() error {
	if err := os.MkdirAll(b.rootDir, 0o755); err != nil {
		return fmt.Errorf("creating memory bank directory: %w", err)
	}

	for _, name := range TemplateFilenames() {
		path := filepath.Join(b.rootDir, name)
		if _, err := os.Stat(path); err == nil {
			b.logger.Debug("memory bank file exists, skipping", "file", name)
			continue
		}

		content := templateFiles[name]
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing template %s: %w", name, err)
		}
		b.logger.Info("created memory bank file", "file", name)
	}

	return nil
}

// Read returns the contents of a memory bank file.
// Returns ErrFileNotFound if the filename is not in the known set or the file does not exist.
func (b *Bank) Read(filename string) (string, error) {
	if !knownFiles[filename] {
		return "", fmt.Errorf("%w: %s", ErrFileNotFound, filename)
	}

	data, err := os.ReadFile(filepath.Join(b.rootDir, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrFileNotFound, filename)
		}
		return "", fmt.Errorf("reading %s: %w", filename, err)
	}

	return string(data), nil
}

// ReadAll returns the contents of all memory bank files keyed by filename.
func (b *Bank) ReadAll() (map[string]string, error) {
	result := make(map[string]string, len(templateFiles))
	for _, name := range TemplateFilenames() {
		content, err := b.Read(name)
		if err != nil {
			return nil, fmt.Errorf("reading all: %w", err)
		}
		result[name] = content
	}
	return result, nil
}

// Update overwrites the entire content of a memory bank file.
// Returns ErrFileNotFound if the filename is not in the known set.
func (b *Bank) Update(filename, content string) error {
	if !knownFiles[filename] {
		return fmt.Errorf("%w: %s", ErrFileNotFound, filename)
	}

	path := filepath.Join(b.rootDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("updating %s: %w", filename, err)
	}

	return nil
}

// AppendSection inserts content into a specific section of a memory bank file.
// It finds the line starting with "## <section>" and inserts the new content
// before the next "## " heading or at end of file.
// Returns ErrFileNotFound if the filename is unknown, ErrSectionNotFound if
// the section heading is not present.
func (b *Bank) AppendSection(filename, section, content string) error {
	if !knownFiles[filename] {
		return fmt.Errorf("%w: %s", ErrFileNotFound, filename)
	}

	existing, err := b.Read(filename)
	if err != nil {
		return err
	}

	header := "## " + section
	lines := strings.Split(existing, "\n")

	sectionIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			sectionIdx = i
			break
		}
	}
	if sectionIdx < 0 {
		return fmt.Errorf("%w: %s", ErrSectionNotFound, section)
	}

	// Find the next section heading or EOF.
	insertIdx := len(lines)
	for i := sectionIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			insertIdx = i
			break
		}
	}

	// Build new content: everything before insert point, new content, everything after.
	newLines := make([]string, 0, len(lines)+2)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, content)
	newLines = append(newLines, lines[insertIdx:]...)

	return b.Update(filename, strings.Join(newLines, "\n"))
}

// Summary reads all memory bank files and produces a combined, token-efficient
// summary suitable for LLM context injection.
func (b *Bank) Summary() (string, error) {
	all, err := b.ReadAll()
	if err != nil {
		return "", fmt.Errorf("building summary: %w", err)
	}

	var sb strings.Builder
	for _, name := range TemplateFilenames() {
		content := all[name]
		sb.WriteString("--- ")
		sb.WriteString(name)
		sb.WriteString(" ---\n")
		sb.WriteString(strings.TrimSpace(content))
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}
