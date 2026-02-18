package docgen

import (
	"strings"
	"testing"
)

func TestNewGenerator_Valid(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v, want nil", err)
	}
	if g.model != DefaultModel {
		t.Errorf("model = %q, want %q", g.model, DefaultModel)
	}
	if g.opts.Mode != GenerateModeBatch {
		t.Errorf("mode = %q, want %q", g.opts.Mode, GenerateModeBatch)
	}
	if g.budget == nil {
		t.Error("budget should not be nil")
	}
	if g.retryer == nil {
		t.Error("retryer should not be nil")
	}
	if g.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNewGenerator_EmptyAPIKey(t *testing.T) {
	t.Parallel()
	_, err := NewGenerator("", GenerateOptions{}, nil)
	if err == nil {
		t.Fatal("NewGenerator() expected error for empty API key, got nil")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("error = %q, want message containing 'API key is required'", err)
	}
}

func TestNewGenerator_CustomModel(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{
		Model: "claude-opus-4-20250514",
	}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	if g.model != "claude-opus-4-20250514" {
		t.Errorf("model = %q, want %q", g.model, "claude-opus-4-20250514")
	}
}

func TestNewGenerator_CustomOptions(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{
		Mode:      GenerateModeStream,
		MaxTokens: 16384,
		OutputDir: "docs/phases",
	}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	if g.opts.Mode != GenerateModeStream {
		t.Errorf("mode = %q, want %q", g.opts.Mode, GenerateModeStream)
	}
	if g.budget.MaxOutputTokens != 16384 {
		t.Errorf("MaxOutputTokens = %d, want 16384", g.budget.MaxOutputTokens)
	}
}

func TestNewGenerator_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	if g.budget.MaxOutputTokens != defaultMaxTokens {
		t.Errorf("MaxOutputTokens = %d, want %d", g.budget.MaxOutputTokens, defaultMaxTokens)
	}
}

func TestValidate_ValidRequests(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec, Description: "Phase 1 spec"},
		{PhaseID: "1", DocType: DocTypePrompt, Description: "Phase 1 prompts"},
		{PhaseID: "2", DocType: DocTypeSpec, Description: "Phase 2 spec"},
	}

	if err := g.Validate(requests); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_EmptySlice(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	err = g.Validate(nil)
	if err == nil {
		t.Fatal("Validate() expected error for empty slice, got nil")
	}
	if !strings.Contains(err.Error(), "no requests") {
		t.Errorf("error = %q, want message containing 'no requests'", err)
	}
}

func TestValidate_EmptyPhaseID(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	requests := []DocRequest{
		{PhaseID: "", DocType: DocTypeSpec},
	}
	err = g.Validate(requests)
	if err == nil {
		t.Fatal("Validate() expected error for empty PhaseID, got nil")
	}
	if !strings.Contains(err.Error(), "phase_id is required") {
		t.Errorf("error = %q, want message containing 'phase_id is required'", err)
	}
}

func TestValidate_InvalidDocType(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	requests := []DocRequest{
		{PhaseID: "1", DocType: "invalid"},
	}
	err = g.Validate(requests)
	if err == nil {
		t.Fatal("Validate() expected error for invalid DocType, got nil")
	}
	if !strings.Contains(err.Error(), "invalid doc_type") {
		t.Errorf("error = %q, want message containing 'invalid doc_type'", err)
	}
}

func TestValidate_Duplicates(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec},
		{PhaseID: "1", DocType: DocTypeSpec},
	}
	err = g.Validate(requests)
	if err == nil {
		t.Fatal("Validate() expected error for duplicate request, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want message containing 'duplicate'", err)
	}
}

func TestValidate_ContentExceedsBudget(t *testing.T) {
	t.Parallel()
	g, err := NewGenerator("test-key", GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	// Shrink the budget to make it easy to exceed.
	g.budget.MaxInputTokens = 100
	g.budget.ReserveTokens = 0
	g.budget.SystemPromptTokens = 0

	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec, Description: strings.Repeat("x", 500)},
	}
	err = g.Validate(requests)
	if err == nil {
		t.Fatal("Validate() expected error for content exceeding budget, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds token budget") {
		t.Errorf("error = %q, want message containing 'exceeds token budget'", err)
	}
}
