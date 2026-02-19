package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/phase"
)

// FilesystemEvidence captures expected vs observed file effects.
type FilesystemEvidence struct {
	Expected    []string
	Found       []string
	Missing     []string
	Unexpected  []string
	GitEvidence []string
	Timestamp   time.Time
	Duration    time.Duration
}

// ReconcileFiles checks whether expected files exist and captures git evidence.
func ReconcileFiles(ctx context.Context, root string, spec phase.PhaseSpec, sinceCommit string) (*FilesystemEvidence, error) {
	start := time.Now()
	if root == "" {
		return nil, fmt.Errorf("reconcile files: root must not be empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("reconcile files: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("reconcile files: root %q is not a directory", root)
	}

	expected := make([]string, 0, len(spec.FilesNew)+len(spec.FilesModified))
	expected = append(expected, spec.FilesNew...)
	expected = append(expected, spec.FilesModified...)

	e := &FilesystemEvidence{
		Expected:  dedupeSorted(expected),
		Timestamp: time.Now().UTC(),
	}

	for _, rel := range e.Expected {
		full := filepath.Join(root, rel)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			e.Found = append(e.Found, rel)
		} else {
			e.Missing = append(e.Missing, rel)
		}
	}
	e.Found = dedupeSorted(e.Found)
	e.Missing = dedupeSorted(e.Missing)

	changed, err := gitDiffNames(ctx, root, sinceCommit)
	if err != nil {
		return nil, err
	}
	e.GitEvidence = dedupeSorted(changed)

	expectedSet := make(map[string]struct{}, len(e.Expected))
	for _, f := range e.Expected {
		expectedSet[f] = struct{}{}
	}
	for _, f := range e.GitEvidence {
		if _, ok := expectedSet[f]; !ok {
			e.Unexpected = append(e.Unexpected, f)
		}
	}
	e.Unexpected = dedupeSorted(e.Unexpected)
	e.Duration = time.Since(start)
	return e, nil
}

// IsComplete returns true when all expected files exist.
func (e *FilesystemEvidence) IsComplete() bool {
	if e == nil {
		return false
	}
	return len(e.Missing) == 0
}

// Summary returns a concise human-readable evidence summary.
func (e *FilesystemEvidence) Summary() string {
	if e == nil {
		return "no filesystem evidence"
	}
	total := len(e.Expected)
	if total == 0 {
		if len(e.GitEvidence) == 0 {
			return "No expected files declared. Git shows 0 files changed."
		}
		return fmt.Sprintf("No expected files declared. Git shows %d files changed.", len(e.GitEvidence))
	}
	if len(e.Missing) == 0 {
		return fmt.Sprintf("All %d expected files exist. Git shows %d files changed.", total, len(e.GitEvidence))
	}
	return fmt.Sprintf(
		"Missing %d of %d expected files: %s. Git shows %d files changed since assignment.",
		len(e.Missing), total, strings.Join(e.Missing, ", "), len(e.GitEvidence),
	)
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func gitDiffNames(ctx context.Context, root, sinceCommit string) ([]string, error) {
	var args []string
	if strings.TrimSpace(sinceCommit) == "" {
		args = []string{"diff", "--name-only"}
	} else {
		args = []string{"diff", "--name-only", sinceCommit}
	}
	diffOutput, err := runGitList(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	untrackedOutput, err := runGitList(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(diffOutput+"\n"+untrackedOutput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func runGitList(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("reconcile files: git %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
