package orchestrator

import (
	"strings"
	"testing"
)

func TestParseReviewResponseApprove(t *testing.T) {
	r := &ReviewOrchestrator{}
	raw := `some text
{"verdict":"APPROVE","summary":"looks good","confidence":0.93}
`
	resp, err := r.ParseReviewResponse(raw)
	if err != nil {
		t.Fatalf("ParseReviewResponse: %v", err)
	}
	if resp.Verdict != ReviewApprove {
		t.Fatalf("verdict = %q, want APPROVE", resp.Verdict)
	}
}

func TestParseReviewResponseRequestChanges(t *testing.T) {
	r := &ReviewOrchestrator{}
	raw := `{"verdict":"REQUEST_CHANGES","summary":"fix it","confidence":0.8,"issues":[{"file":"main.go","line":12,"severity":"major","description":"bug"}]}`
	resp, err := r.ParseReviewResponse(raw)
	if err != nil {
		t.Fatalf("ParseReviewResponse: %v", err)
	}
	if resp.Verdict != ReviewRequestChanges {
		t.Fatalf("verdict = %q, want REQUEST_CHANGES", resp.Verdict)
	}
	if len(resp.Issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(resp.Issues))
	}
}

func TestParseReviewResponseNoJSON(t *testing.T) {
	r := &ReviewOrchestrator{}
	if _, err := r.ParseReviewResponse("plain text"); err == nil {
		t.Fatal("expected parse error for missing JSON")
	}
}

func TestFormatFeedback(t *testing.T) {
	r := &ReviewOrchestrator{}
	resp := &ReviewResponse{
		Verdict: ReviewRequestChanges,
		Issues: []ReviewIssue{
			{File: "main.go", Line: 10, Severity: "major", Description: "fix nil check"},
		},
	}
	out := r.FormatFeedback(resp)
	if out == "" {
		t.Fatal("feedback should not be empty")
	}
	if !strings.Contains(out, "main.go:10") {
		t.Fatalf("feedback missing location: %q", out)
	}
}

func TestRequestReviewNoReviewer(t *testing.T) {
	r := &ReviewOrchestrator{}
	if _, err := r.RequestReview(t.Context(), ReviewRequest{}); err == nil {
		t.Fatal("expected ErrNoReviewer")
	}
}
