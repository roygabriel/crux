// Package security implements filesystem sandboxing, permission enforcement,
// and audit logging for agent actions.
package security

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// FileOp represents a filesystem operation type.
type FileOp string

const (
	// OpRead is a file read operation.
	OpRead FileOp = "read"
	// OpWrite is a file write operation.
	OpWrite FileOp = "write"
	// OpDelete is a file delete operation.
	OpDelete FileOp = "delete"
	// OpExecute is a file execute operation.
	OpExecute FileOp = "execute"
)

var (
	// ErrPathOutsideProject indicates the path escapes the project root.
	ErrPathOutsideProject = errors.New("path is outside project root")
	// ErrPathDenied indicates the path matches a denied pattern.
	ErrPathDenied = errors.New("path is denied")
	// ErrPathNotAllowed indicates the path is not in the allowed paths list.
	ErrPathNotAllowed = errors.New("path is not in allowed paths")
)

// Sandbox enforces filesystem access boundaries for agent operations.
type Sandbox struct {
	projectRoot  string   // absolute, symlink-resolved
	allowedPaths []string // absolute paths, empty = all under root
	deniedPaths  []string // patterns for filepath.Match + prefix match
	logger       *slog.Logger
}

// NewSandbox creates a Sandbox rooted at projectRoot. Allowed and denied paths
// are resolved relative to root if not absolute. Denied paths with a trailing
// "/" are treated as directory prefixes; others as exact file matches or glob
// patterns.
func NewSandbox(projectRoot string, allowed, denied []string, logger *slog.Logger) (*Sandbox, error) {
	if logger == nil {
		logger = slog.Default()
	}

	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("sandbox: abs root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("sandbox: resolve root: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("sandbox: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("sandbox: root %q is not a directory", resolved)
	}

	resolvedAllowed := make([]string, 0, len(allowed))
	for _, p := range allowed {
		resolvedAllowed = append(resolvedAllowed, resolvePath(resolved, p))
	}

	resolvedDenied := make([]string, 0, len(denied))
	for _, p := range denied {
		resolvedDenied = append(resolvedDenied, p)
	}

	return &Sandbox{
		projectRoot:  resolved,
		allowedPaths: resolvedAllowed,
		deniedPaths:  resolvedDenied,
		logger:       logger,
	}, nil
}

// Check validates that path is allowed for the given operation.
func (s *Sandbox) Check(path string, op FileOp) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("sandbox check: abs: %w", err)
	}
	absPath = filepath.Clean(absPath)

	resolved, err := resolveWithParent(absPath)
	if err != nil {
		// If we can't resolve, use the cleaned absolute path.
		// This is safe because the containment check below will still
		// catch paths outside the project root.
		resolved = absPath
	}

	// Containment check.
	rel, err := filepath.Rel(s.projectRoot, resolved)
	if err != nil {
		return ErrPathOutsideProject
	}
	if strings.HasPrefix(rel, "..") {
		return ErrPathOutsideProject
	}

	// Denied check.
	for _, denied := range s.deniedPaths {
		if matchDenied(rel, denied) {
			return ErrPathDenied
		}
	}

	// Allowed check.
	if len(s.allowedPaths) > 0 {
		found := false
		for _, allowed := range s.allowedPaths {
			if strings.HasPrefix(resolved, allowed) {
				found = true
				break
			}
		}
		if !found {
			return ErrPathNotAllowed
		}
	}

	return nil
}

// CheckPaths validates all paths for the given operation, returning the
// first error encountered.
func (s *Sandbox) CheckPaths(paths []string, op FileOp) error {
	for _, p := range paths {
		if err := s.Check(p, op); err != nil {
			return err
		}
	}
	return nil
}

// resolveWithParent resolves symlinks for a path. If the path doesn't exist,
// it resolves the parent directory and appends the base name.
func resolveWithParent(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	// File doesn't exist — resolve parent and append base.
	parent := filepath.Dir(path)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(path)), nil
}

// matchDenied checks if relPath matches a denied pattern. Patterns with
// trailing "/" are directory prefixes; others use filepath.Match or exact match.
func matchDenied(relPath, denied string) bool {
	if strings.HasSuffix(denied, "/") {
		prefix := strings.TrimSuffix(denied, "/")
		if relPath == prefix || strings.HasPrefix(relPath, denied) {
			return true
		}
		return false
	}

	if relPath == denied {
		return true
	}

	matched, err := filepath.Match(denied, relPath)
	if err == nil && matched {
		return true
	}

	return false
}

// resolvePath resolves p relative to root if it is not absolute.
func resolvePath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}
