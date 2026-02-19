package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

// ProgressFingerprint captures semantic progress for a prompt attempt.
type ProgressFingerprint struct {
	PhaseID         types.PhaseID
	PromptNum       int
	Timestamp       time.Time
	AttemptNum      int
	FilesExistCount int
	FilesExpected   int
	GitDiffHash     string
	TestPassCount   int
	TestTotalCount  int
	LastGateHash    string
	ProgressScore   float64
}

// ComputeFingerprint builds a semantic fingerprint from filesystem/git/gate state.
func ComputeFingerprint(
	ctx context.Context,
	root string,
	spec phase.PhaseSpec,
	promptNum int,
	attemptNum int,
	gateResults []phase.GateResult,
) (*ProgressFingerprint, error) {
	expected := append([]string(nil), spec.FilesNew...)
	expected = append(expected, spec.FilesModified...)
	expected = dedupeSorted(expected)

	existCount := 0
	for _, rel := range expected {
		full := filepath.Join(root, rel)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			existCount++
		}
	}

	gitStat, err := gitDiffStat(ctx, root)
	if err != nil {
		return nil, err
	}
	gateJSON, _ := json.Marshal(gateResults)

	pass := 0
	for _, r := range gateResults {
		if r.Passed {
			pass++
		}
	}
	fp := &ProgressFingerprint{
		PhaseID:         spec.ID,
		PromptNum:       promptNum,
		Timestamp:       time.Now().UTC(),
		AttemptNum:      attemptNum,
		FilesExistCount: existCount,
		FilesExpected:   len(expected),
		GitDiffHash:     hashString(gitStat),
		TestPassCount:   pass,
		TestTotalCount:  len(gateResults),
		LastGateHash:    hashString(string(gateJSON)),
	}
	fp.ProgressScore = fp.Score()
	return fp, nil
}

// Score computes progress score from filesystem/gate/git signals.
func (f *ProgressFingerprint) Score() float64 {
	if f == nil {
		return 0
	}
	filesComponent := 0.4
	if f.FilesExpected > 0 {
		filesComponent = (float64(f.FilesExistCount) / float64(f.FilesExpected)) * 0.4
	}

	testsComponent := 0.4
	if f.TestTotalCount > 0 {
		testsComponent = (float64(f.TestPassCount) / float64(f.TestTotalCount)) * 0.4
	}

	gitComponent := 0.0
	if f.GitDiffHash != hashString("") {
		gitComponent = 0.2
	}
	score := filesComponent + testsComponent + gitComponent
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// SameAs compares semantic progress signals while ignoring timestamp/attempt.
func (f *ProgressFingerprint) SameAs(other *ProgressFingerprint) bool {
	if f == nil || other == nil {
		return false
	}
	return f.FilesExistCount == other.FilesExistCount &&
		f.GitDiffHash == other.GitDiffHash &&
		f.TestPassCount == other.TestPassCount &&
		f.LastGateHash == other.LastGateHash
}

func gitDiffStat(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("compute fingerprint: git diff --stat failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
