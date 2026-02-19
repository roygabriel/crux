package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

var (
	// ErrNoReviewer indicates no idle reviewer is configured.
	ErrNoReviewer = errors.New("no reviewer agent available")
	jsonBlockRe   = regexp.MustCompile(`\{[\s\S]*\}`)
)

// ReviewRequest is sent to reviewer agents after verification passes.
type ReviewRequest struct {
	PhaseID        types.PhaseID
	PromptNum      int
	EngineerAgent  string
	Diff           string
	DiffStat       string
	FilesChanged   []string
	Spec           *phase.PhaseSpec
	AcceptCriteria []string
	GateResults    []phase.GateResult
}

// ReviewVerdict is the review decision.
type ReviewVerdict string

const (
	ReviewApprove        ReviewVerdict = "APPROVE"
	ReviewRequestChanges ReviewVerdict = "REQUEST_CHANGES"
)

// ReviewIssue describes a specific review concern.
type ReviewIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// ReviewResponse is parsed from reviewer output.
type ReviewResponse struct {
	Verdict    ReviewVerdict `json:"verdict"`
	Issues     []ReviewIssue `json:"issues,omitempty"`
	Summary    string        `json:"summary"`
	Confidence float64       `json:"confidence"`
}

// ReviewOrchestrator coordinates reviewer dispatch and response parsing.
type ReviewOrchestrator struct {
	messenger *agent.Messenger
	registry  *agent.Registry
	root      string
}

// NewReviewOrchestrator creates a reviewer coordinator.
func NewReviewOrchestrator(messenger *agent.Messenger, registry *agent.Registry, root string) *ReviewOrchestrator {
	return &ReviewOrchestrator{
		messenger: messenger,
		registry:  registry,
		root:      root,
	}
}

// RequestReview sends a structured review prompt to an idle reviewer.
func (r *ReviewOrchestrator) RequestReview(ctx context.Context, req ReviewRequest) (types.AgentID, error) {
	if r.registry == nil || r.messenger == nil {
		return "", ErrNoReviewer
	}
	reviewerID := types.AgentID("")
	for _, inst := range r.registry.List() {
		role := types.NormalizeAgentRole(inst.Agent.Role)
		if role != types.RoleCodeReviewer {
			continue
		}
		if inst.Agent.Status != types.StatusIdle {
			continue
		}
		if inst.Agent.ID == types.AgentID(req.EngineerAgent) {
			continue
		}
		reviewerID = inst.Agent.ID
		break
	}
	if reviewerID == "" {
		return "", ErrNoReviewer
	}

	msg := types.Message{
		From:     "orchestrator",
		To:       reviewerID,
		Type:     types.MessageTask,
		Priority: types.PriorityHigh,
		Payload:  formatReviewRequest(req),
	}
	if err := r.registry.UpdateStatus(reviewerID, types.StatusBusy); err != nil {
		return "", err
	}
	if err := r.messenger.Send(ctx, reviewerID, msg); err != nil {
		_ = r.registry.UpdateStatus(reviewerID, types.StatusIdle)
		return "", err
	}
	return reviewerID, nil
}

// ParseReviewResponse extracts a JSON review response from raw output.
func (r *ReviewOrchestrator) ParseReviewResponse(raw string) (*ReviewResponse, error) {
	m := jsonBlockRe.FindString(raw)
	if strings.TrimSpace(m) == "" {
		return nil, fmt.Errorf("review response has no JSON block")
	}
	var resp ReviewResponse
	if err := json.Unmarshal([]byte(m), &resp); err != nil {
		return nil, fmt.Errorf("parse review JSON: %w", err)
	}
	switch resp.Verdict {
	case ReviewApprove:
	case ReviewRequestChanges:
		if len(resp.Issues) == 0 {
			return nil, fmt.Errorf("REQUEST_CHANGES requires at least one issue")
		}
	default:
		return nil, fmt.Errorf("invalid review verdict %q", resp.Verdict)
	}
	return &resp, nil
}

// FormatFeedback converts reviewer issues into engineer-facing feedback text.
func (r *ReviewOrchestrator) FormatFeedback(resp *ReviewResponse) string {
	if resp == nil {
		return "## Reviewer Feedback\nNo response payload provided."
	}
	if len(resp.Issues) == 0 {
		return "## Reviewer Feedback\nNo blocking issues reported."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Reviewer Feedback — Changes Required\n\nThe code reviewer found %d issues:\n", len(resp.Issues))
	for _, issue := range resp.Issues {
		loc := issue.File
		if issue.Line > 0 {
			loc = fmt.Sprintf("%s:%d", issue.File, issue.Line)
		}
		fmt.Fprintf(&b, "\n### %s — %s\n%s\n", loc, issue.Severity, issue.Description)
	}
	b.WriteString("\nPlease address all issues and ensure gates still pass.\n")
	return b.String()
}

func (r *ReviewOrchestrator) snapshotReviewDiff(ctx context.Context) (diffStat string, diff string, files []string) {
	diffStat = gitOut(ctx, r.root, "diff", "--stat")
	diff = gitOut(ctx, r.root, "diff", "--no-color")
	namesRaw := gitOut(ctx, r.root, "diff", "--name-only")
	for _, line := range strings.Split(namesRaw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return diffStat, diff, files
}

func formatReviewRequest(req ReviewRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Code Review Request — Phase %s Prompt %d\n\n", req.PhaseID, req.PromptNum)
	b.WriteString("## Changes\n")
	if req.DiffStat == "" {
		b.WriteString("(no diff stat)\n")
	} else {
		b.WriteString(req.DiffStat + "\n")
	}
	b.WriteString("\n## Files Changed\n")
	if len(req.FilesChanged) == 0 {
		b.WriteString("- (none)\n")
	} else {
		for _, f := range req.FilesChanged {
			b.WriteString("- " + f + "\n")
		}
	}
	if len(req.AcceptCriteria) > 0 {
		b.WriteString("\n## Acceptance Criteria\n")
		for _, c := range req.AcceptCriteria {
			b.WriteString("- " + c + "\n")
		}
	}
	b.WriteString("\n## Gate Results\n")
	for _, g := range req.GateResults {
		status := "PASS"
		if !g.Passed {
			status = "FAIL"
		}
		cmd := g.Gate.Command
		if cmd == "" {
			cmd = g.Gate.Expected
		}
		b.WriteString("- " + status + ": " + cmd + "\n")
	}
	b.WriteString("\n## Full Diff\n```diff\n")
	b.WriteString(req.Diff)
	b.WriteString("\n```\n\n## Instructions\n")
	b.WriteString("Review these changes against the acceptance criteria above.\n")
	b.WriteString("Respond with EXACTLY one JSON block.\n")
	return b.String()
}

func gitOut(ctx context.Context, root string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
