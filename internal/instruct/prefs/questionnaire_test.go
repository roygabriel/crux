package prefs

import (
	"strings"
	"testing"
)

func TestNewQuestionnaire_NilDefaults(t *testing.T) {
	t.Parallel()

	q := NewQuestionnaire(nil)
	result := q.Result()
	if result == nil {
		t.Fatal("Result() should not be nil for non-aborted questionnaire")
	}
	if result.Preset != PresetPragmatic {
		t.Errorf("Preset = %q, want %q", result.Preset, PresetPragmatic)
	}
}

func TestNewQuestionnaire_WithDefaults(t *testing.T) {
	t.Parallel()

	defaults := PresetDefaults(PresetStrict)
	q := NewQuestionnaire(defaults)
	result := q.Result()

	if result.Preset != PresetStrict {
		t.Errorf("Preset = %q, want %q", result.Preset, PresetStrict)
	}
	if result.Testing.CoverageTarget != 90 {
		t.Errorf("CoverageTarget = %d, want 90", result.Testing.CoverageTarget)
	}
}

func TestNewQuestionnaire_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	defaults := PresetDefaults(PresetStrict)
	original := defaults.Testing.CoverageTarget

	q := NewQuestionnaire(defaults)
	result := q.Result()
	result.Testing.CoverageTarget = 42

	if defaults.Testing.CoverageTarget != original {
		t.Errorf("input was mutated: CoverageTarget = %d, want %d",
			defaults.Testing.CoverageTarget, original)
	}
	_ = q
}

func TestQuestionnaireModel_ResultAllPresets(t *testing.T) {
	t.Parallel()

	presets := []PresetName{PresetStrict, PresetPragmatic, PresetStartup}

	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			t.Parallel()

			defaults := PresetDefaults(preset)
			q := NewQuestionnaire(defaults)
			result := q.Result()

			if result == nil {
				t.Fatal("Result() returned nil")
			}
			if result.Preset != preset {
				t.Errorf("Preset = %q, want %q", result.Preset, preset)
			}
			if result.Version != CurrentVersion {
				t.Errorf("Version = %q, want %q", result.Version, CurrentVersion)
			}

			// Validate the result is well-formed.
			if err := result.Validate(); err != nil {
				t.Errorf("Validate() error: %v", err)
			}
		})
	}
}

func TestQuestionnaireModel_AbortedReturnsNil(t *testing.T) {
	t.Parallel()

	q := NewQuestionnaire(nil)
	q.aborted = true

	if result := q.Result(); result != nil {
		t.Error("Result() should return nil when aborted")
	}
}

func TestQuestionnaireModel_Custom(t *testing.T) {
	t.Parallel()

	q := NewQuestionnaire(nil)
	if q.Custom() != "" {
		t.Errorf("Custom() = %q, want empty", q.Custom())
	}

	q.custom = "always use context.Context"
	if q.Custom() != "always use context.Context" {
		t.Errorf("Custom() = %q, want %q", q.Custom(), "always use context.Context")
	}
}

func TestRenderPreferencesSummary_ContainsAllCategories(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	summary := RenderPreferencesSummary(prefs, "")

	categories := []string{
		"Testing", "Error Handling", "Organization", "Naming",
		"Abstraction", "Documentation", "Formatting", "Dependencies",
		"Architecture",
	}

	for _, cat := range categories {
		if !strings.Contains(summary, cat) {
			t.Errorf("summary missing category %q", cat)
		}
	}
}

func TestRenderPreferencesSummary_IncludesPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset PresetName
		want   string
	}{
		{PresetStrict, "strict"},
		{PresetPragmatic, "pragmatic"},
		{PresetStartup, "startup"},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			summary := RenderPreferencesSummary(prefs, "")
			if !strings.Contains(summary, tt.want) {
				t.Errorf("summary does not contain preset %q", tt.want)
			}
		})
	}
}

func TestRenderPreferencesSummary_IncludesCustom(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	summary := RenderPreferencesSummary(prefs, "always use context.Context")
	if !strings.Contains(summary, "always use context.Context") {
		t.Error("summary does not contain custom text")
	}
	if !strings.Contains(summary, "Custom") {
		t.Error("summary does not contain Custom heading")
	}
}

func TestRenderPreferencesSummary_OmitsCustomWhenEmpty(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	summary := RenderPreferencesSummary(prefs, "")
	if strings.Contains(summary, "Custom") {
		t.Error("summary should not contain Custom heading when custom text is empty")
	}
}

func TestRenderPreferencesSummary_IncludesLanguage(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Language = "go"
	summary := RenderPreferencesSummary(prefs, "")
	if !strings.Contains(summary, "go") {
		t.Error("summary does not contain language")
	}
}

func TestRenderPreferencesSummary_IncludesValues(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetStrict)
	summary := RenderPreferencesSummary(prefs, "")

	values := []string{
		"tdd", "90%", "interfaces",
		"custom-types", "true",
		"clean-architecture", "snake_case",
		"abbreviated",
		"30",
		"120",
		"minimal",
		"layered", "inward",
	}

	for _, v := range values {
		if !strings.Contains(summary, v) {
			t.Errorf("summary missing value %q", v)
		}
	}
}

func TestValidateCoverageInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"0", false},
		{"50", false},
		{"100", false},
		{"-1", true},
		{"101", true},
		{"abc", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			err := validateCoverageInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCoverageInput(%q) error = %v, wantErr %v",
					tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePositiveInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"1", false},
		{"50", false},
		{"0", true},
		{"-1", true},
		{"abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			err := validatePositiveInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePositiveInt(%q) error = %v, wantErr %v",
					tt.input, err, tt.wantErr)
			}
		})
	}
}
