package prefs

import (
	"strings"
	"testing"
)

func TestCompile_AllPresetsProduceNonEmptyOutput(t *testing.T) {
	t.Parallel()

	presets := []PresetName{PresetStrict, PresetPragmatic, PresetStartup}

	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(preset)
			result := Compile(prefs)

			fields := map[string]string{
				"Testing":       result.Testing,
				"ErrorHandling": result.ErrorHandling,
				"Organization":  result.Organization,
				"Naming":        result.Naming,
				"Abstraction":   result.Abstraction,
				"Documentation": result.Documentation,
				"Formatting":    result.Formatting,
				"Dependencies":  result.Dependencies,
				"Architecture":  result.Architecture,
			}

			for name, text := range fields {
				if text == "" {
					t.Errorf("%s is empty for preset %q", name, preset)
				}
			}
		})
	}
}

func TestCompile_Deterministic(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetStrict)
	first := Compile(prefs)
	second := Compile(prefs)

	if first.Testing != second.Testing {
		t.Error("Testing output not deterministic")
	}
	if first.ErrorHandling != second.ErrorHandling {
		t.Error("ErrorHandling output not deterministic")
	}
	if first.Architecture != second.Architecture {
		t.Error("Architecture output not deterministic")
	}
}

func TestCompile_TestingKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict TDD",
			preset:   PresetStrict,
			keywords: []string{"BEFORE implementation", "table-driven", "90%", "interfaces"},
		},
		{
			name:     "pragmatic test-after",
			preset:   PresetPragmatic,
			keywords: []string{"after implementation", "table-driven", "70%", "interfaces"},
		},
		{
			name:     "startup minimal",
			preset:   PresetStartup,
			keywords: []string{"critical business logic", "40%", "simple and fast"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Testing, kw) {
					t.Errorf("Testing output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Testing)
				}
			}
		})
	}
}

func TestCompile_ErrorHandlingKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict custom-types",
			preset:   PresetStrict,
			keywords: []string{"sentinel errors", "custom error types", "ValidationError", "early returns"},
		},
		{
			name:     "pragmatic wrapping",
			preset:   PresetPragmatic,
			keywords: []string{"Wrap all errors", "fmt.Errorf", "early returns"},
		},
		{
			name:     "startup wrapping no wrap context",
			preset:   PresetStartup,
			keywords: []string{"Wrap all errors", "early returns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.ErrorHandling, kw) {
					t.Errorf("ErrorHandling output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.ErrorHandling)
				}
			}
		})
	}
}

func TestCompile_OrganizationKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict clean",
			preset:   PresetStrict,
			keywords: []string{"clean architecture", "snake_case", "internal/"},
		},
		{
			name:     "pragmatic flat",
			preset:   PresetPragmatic,
			keywords: []string{"flat", "snake_case", "internal/"},
		},
		{
			name:     "startup flat no internal",
			preset:   PresetStartup,
			keywords: []string{"flat", "snake_case"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Organization, kw) {
					t.Errorf("Organization output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Organization)
				}
			}
		})
	}
}

func TestCompile_AbstractionKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict 30 lines",
			preset:   PresetStrict,
			keywords: []string{"30 lines", "15 lines", "consumer"},
		},
		{
			name:     "pragmatic 50 lines",
			preset:   PresetPragmatic,
			keywords: []string{"50 lines", "25 lines", "consumer"},
		},
		{
			name:     "startup 80 lines",
			preset:   PresetStartup,
			keywords: []string{"80 lines", "40 lines", "provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Abstraction, kw) {
					t.Errorf("Abstraction output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Abstraction)
				}
			}
		})
	}
}

func TestCompile_DocumentationKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict godoc required",
			preset:   PresetStrict,
			keywords: []string{"Every exported", "godoc", "WHY", "README"},
		},
		{
			name:     "pragmatic optional godoc",
			preset:   PresetPragmatic,
			keywords: []string{"godoc", "WHY", "no per-package README"},
		},
		{
			name:     "startup minimal",
			preset:   PresetStartup,
			keywords: []string{"self-documenting", "no per-package README"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Documentation, kw) {
					t.Errorf("Documentation output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Documentation)
				}
			}
		})
	}
}

func TestCompile_FormattingKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict 120 chars",
			preset:   PresetStrict,
			keywords: []string{"120 characters", "gofmt", "golangci-lint", "exhaustive"},
		},
		{
			name:     "pragmatic 120 chars standard",
			preset:   PresetPragmatic,
			keywords: []string{"120 characters", "gofmt", "golangci-lint"},
		},
		{
			name:     "startup 140 chars minimal",
			preset:   PresetStartup,
			keywords: []string{"140 characters", "gofmt", "go vet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Formatting, kw) {
					t.Errorf("Formatting output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Formatting)
				}
			}
		})
	}
}

func TestCompile_DependenciesKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict minimal",
			preset:   PresetStrict,
			keywords: []string{"Minimize", "stdlib", "Do NOT vendor"},
		},
		{
			name:     "pragmatic",
			preset:   PresetPragmatic,
			keywords: []string{"well-maintained", "Do NOT vendor"},
		},
		{
			name:     "startup batteries",
			preset:   PresetStartup,
			keywords: []string{"freely", "Do NOT vendor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Dependencies, kw) {
					t.Errorf("Dependencies output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Dependencies)
				}
			}
		})
	}
}

func TestCompile_ArchitectureKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict layered inward",
			preset:   PresetStrict,
			keywords: []string{"layered", "handlers", "inward", "dependency injection"},
		},
		{
			name:     "pragmatic flat inward",
			preset:   PresetPragmatic,
			keywords: []string{"flat", "inward", "dependency injection"},
		},
		{
			name:     "startup flat downward",
			preset:   PresetStartup,
			keywords: []string{"flat", "downward", "circular"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Architecture, kw) {
					t.Errorf("Architecture output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Architecture)
				}
			}
		})
	}
}

func TestCompileCategory_AllCategories(t *testing.T) {
	t.Parallel()

	allCategories := []string{
		"testing", "error_handling", "organization", "naming",
		"abstraction", "documentation", "formatting", "dependencies",
		"architecture",
	}

	prefs := PresetDefaults(PresetPragmatic)

	for _, cat := range allCategories {
		t.Run(cat, func(t *testing.T) {
			t.Parallel()

			result := CompileCategory(prefs, cat)
			if result == "" {
				t.Errorf("CompileCategory(%q) returned empty string", cat)
			}
		})
	}
}

func TestCompileCategory_UnknownReturnsEmpty(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	result := CompileCategory(prefs, "nonexistent")
	if result != "" {
		t.Errorf("CompileCategory(\"nonexistent\") = %q, want empty", result)
	}
}

func TestCompileCategory_MatchesCompile(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetStrict)
	full := Compile(prefs)

	categoryToField := map[string]string{
		"testing":        full.Testing,
		"error_handling": full.ErrorHandling,
		"organization":   full.Organization,
		"naming":         full.Naming,
		"abstraction":    full.Abstraction,
		"documentation":  full.Documentation,
		"formatting":     full.Formatting,
		"dependencies":   full.Dependencies,
		"architecture":   full.Architecture,
	}

	for cat, want := range categoryToField {
		t.Run(cat, func(t *testing.T) {
			t.Parallel()

			got := CompileCategory(prefs, cat)
			if got != want {
				t.Errorf("CompileCategory(%q) does not match Compile() output\ngot:  %s\nwant: %s",
					cat, got, want)
			}
		})
	}
}

func TestCompile_NamingKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		preset   PresetName
		keywords []string
	}{
		{
			name:     "strict abbreviated",
			preset:   PresetStrict,
			keywords: []string{"abbreviated", "-er", "ID not Id"},
		},
		{
			name:     "pragmatic single-letter",
			preset:   PresetPragmatic,
			keywords: []string{"single-letter", "-er", "ID not Id"},
		},
		{
			name:     "startup single-letter no er",
			preset:   PresetStartup,
			keywords: []string{"single-letter", "descriptively"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefs := PresetDefaults(tt.preset)
			result := Compile(prefs)

			for _, kw := range tt.keywords {
				if !strings.Contains(result.Naming, kw) {
					t.Errorf("Naming output missing keyword %q for preset %q\nGot: %s",
						kw, tt.preset, result.Naming)
				}
			}
		})
	}
}

func TestCompile_SentinelErrorStyle(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.ErrorHandling.Style = ErrorSentinel

	result := Compile(prefs)
	keywords := []string{"sentinel errors", "errors.Is", "errors.As"}
	for _, kw := range keywords {
		if !strings.Contains(result.ErrorHandling, kw) {
			t.Errorf("ErrorHandling with sentinel style missing keyword %q\nGot: %s",
				kw, result.ErrorHandling)
		}
	}
}

func TestCompile_VendoringEnabled(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Dependencies.Vendoring = true

	result := Compile(prefs)
	if !strings.Contains(result.Dependencies, "Vendor all dependencies") {
		t.Errorf("Dependencies with vendoring=true missing vendor instruction\nGot: %s",
			result.Dependencies)
	}
}

func TestCompile_ApprovedList(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Dependencies.ApprovedList = []string{"chi", "cobra", "yaml.v3"}

	result := Compile(prefs)
	if !strings.Contains(result.Dependencies, "chi") {
		t.Errorf("Dependencies with approved list missing entries\nGot: %s",
			result.Dependencies)
	}
}

func TestCompile_HexagonalOrganization(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Organization.Style = OrgHexagonal

	result := Compile(prefs)
	if !strings.Contains(result.Organization, "hexagonal") {
		t.Errorf("Organization with hexagonal style missing keyword\nGot: %s",
			result.Organization)
	}
}

func TestCompile_HexagonalArchitecture(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Architecture.LayerStructure = "hexagonal"

	result := Compile(prefs)
	if !strings.Contains(result.Architecture, "hexagonal") {
		t.Errorf("Architecture with hexagonal layer missing keyword\nGot: %s",
			result.Architecture)
	}
}

func TestCompile_GenericsPrefer(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Abstraction.GenericsPolicy = "prefer"

	result := Compile(prefs)
	if !strings.Contains(result.Abstraction, "generics") {
		t.Errorf("Abstraction with generics=prefer missing keyword\nGot: %s",
			result.Abstraction)
	}
}

func TestCompile_CamelCaseFileNaming(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Organization.FileNaming = "camelCase"

	result := Compile(prefs)
	if !strings.Contains(result.Organization, "camelCase") {
		t.Errorf("Organization with camelCase file naming missing keyword\nGot: %s",
			result.Organization)
	}
}

func TestCompile_ThoroughCommentStyle(t *testing.T) {
	t.Parallel()

	prefs := PresetDefaults(PresetPragmatic)
	prefs.Documentation.CommentStyle = "thorough"

	result := Compile(prefs)
	if !strings.Contains(result.Documentation, "parameters") {
		t.Errorf("Documentation with thorough style missing keyword\nGot: %s",
			result.Documentation)
	}
}
