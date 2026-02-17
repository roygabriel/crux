package security

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// protectedBranches are branch names that agents may not push to or commit on.
var protectedBranches = map[string]bool{
	"main":       true,
	"master":     true,
	"develop":    true,
	"production": true,
}

// protectedPrefixes are branch name prefixes that agents may not push to.
var protectedPrefixes = []string{"release/"}

// GitGuard enforces git branch safety rules for agent operations.
type GitGuard struct {
	projectRoot string
	logger      *slog.Logger
}

// NewGitGuard creates a GitGuard rooted at projectRoot.
func NewGitGuard(projectRoot string, logger *slog.Logger) *GitGuard {
	if logger == nil {
		logger = slog.Default()
	}
	return &GitGuard{
		projectRoot: projectRoot,
		logger:      logger,
	}
}

// EnsureFeatureBranch creates or checks out the agent's feature branch
// (crux/<agentID>/work). Returns the branch name.
func (g *GitGuard) EnsureFeatureBranch(ctx context.Context, agentID types.AgentID) (string, error) {
	branch := fmt.Sprintf("crux/%s/work", agentID)

	// Check if branch already exists.
	out, err := g.runGit(ctx, "branch", "--list", branch)
	if err != nil {
		return "", fmt.Errorf("list branches: %w", err)
	}

	if strings.TrimSpace(out) != "" {
		// Branch exists — check it out.
		if _, err := g.runGit(ctx, "checkout", branch); err != nil {
			return "", fmt.Errorf("checkout %s: %w", branch, err)
		}
		return branch, nil
	}

	// Create and check out new branch.
	if _, err := g.runGit(ctx, "checkout", "-b", branch); err != nil {
		return "", fmt.Errorf("create branch %s: %w", branch, err)
	}
	return branch, nil
}

// ValidateNotProtected returns an error if branch is a protected branch name.
func (g *GitGuard) ValidateNotProtected(branch string) error {
	if protectedBranches[branch] {
		return fmt.Errorf("branch %s is protected: %w", branch, types.ErrPermissionDenied)
	}
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(branch, prefix) {
			return fmt.Errorf("branch %s matches protected prefix %s: %w", branch, prefix, types.ErrPermissionDenied)
		}
	}
	return nil
}

// SafeCommit validates the current branch has a crux/ prefix, stages the given
// files individually, and commits with the provided message. Returns the commit
// hash.
func (g *GitGuard) SafeCommit(ctx context.Context, agentID types.AgentID, message string, files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files to commit: %w", types.ErrPermissionDenied)
	}

	branch, err := g.currentBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	if !strings.HasPrefix(branch, "crux/") {
		return "", fmt.Errorf("commit on non-feature branch %s: %w", branch, types.ErrPermissionDenied)
	}

	for _, f := range files {
		if _, err := g.runGit(ctx, "add", f); err != nil {
			return "", fmt.Errorf("stage %s: %w", f, err)
		}
	}

	if _, err := g.runGit(ctx, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	hash, err := g.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(hash), nil
}

// PrePushCheck verifies the branch has a crux/ prefix and is not protected.
func (g *GitGuard) PrePushCheck(ctx context.Context, branch string) error {
	if !strings.HasPrefix(branch, "crux/") {
		return fmt.Errorf("push requires crux/ branch prefix, got %s: %w", branch, types.ErrPermissionDenied)
	}
	return g.ValidateNotProtected(branch)
}

// currentBranch returns the name of the current HEAD branch.
func (g *GitGuard) currentBranch(ctx context.Context) (string, error) {
	out, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runGit executes a git command in the project root directory.
func (g *GitGuard) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}
