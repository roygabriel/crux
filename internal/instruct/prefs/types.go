// Package prefs defines structured engineering preferences and provides
// YAML persistence for the crux instruction engine. Preferences are
// organized into nine categories that map 1:1 to the PreferenceInstructions
// output type in the instruct package.
package prefs

import (
	"errors"
	"fmt"
)

// CurrentVersion is the schema version for the preferences file.
const CurrentVersion = "1.0"

// Sentinel errors for preference validation.
var (
	// ErrUnsupportedVersion indicates the preferences file version is not supported.
	ErrUnsupportedVersion = errors.New("unsupported preferences version")
	// ErrInvalidPreset indicates an unrecognized preset name.
	ErrInvalidPreset = errors.New("invalid preset name")
)

// PresetName identifies a built-in preference preset.
type PresetName string

const (
	// PresetStrict enforces rigorous engineering standards.
	PresetStrict PresetName = "strict"
	// PresetPragmatic balances quality with velocity.
	PresetPragmatic PresetName = "pragmatic"
	// PresetStartup prioritizes speed and iteration.
	PresetStartup PresetName = "startup"
)

// TestingStyle describes the testing methodology.
type TestingStyle string

const (
	// TestingTDD requires tests before implementation.
	TestingTDD TestingStyle = "tdd"
	// TestingTestAfter writes tests after implementation.
	TestingTestAfter TestingStyle = "test-after"
	// TestingMinimal tests only critical paths.
	TestingMinimal TestingStyle = "minimal"
)

// ErrorStyle describes the error handling approach.
type ErrorStyle string

const (
	// ErrorSentinel uses package-level sentinel errors.
	ErrorSentinel ErrorStyle = "sentinel"
	// ErrorWrapping wraps errors with context.
	ErrorWrapping ErrorStyle = "wrapping"
	// ErrorCustom uses custom error types.
	ErrorCustom ErrorStyle = "custom-types"
)

// OrgStyle describes the code organization approach.
type OrgStyle string

const (
	// OrgFlat uses a flat package layout.
	OrgFlat OrgStyle = "flat"
	// OrgClean uses clean architecture layers.
	OrgClean OrgStyle = "clean-architecture"
	// OrgHexagonal uses hexagonal architecture.
	OrgHexagonal OrgStyle = "hexagonal"
)

// Preferences holds all structured engineering preferences.
type Preferences struct {
	// Version is the schema version.
	Version string `yaml:"version"`
	// Preset is the base preset these preferences derive from.
	Preset PresetName `yaml:"preset"`
	// Language is the primary programming language (e.g., "go", "python").
	Language string `yaml:"language"`

	// Testing holds testing methodology preferences.
	Testing TestingPrefs `yaml:"testing"`
	// ErrorHandling holds error handling preferences.
	ErrorHandling ErrorHandlingPrefs `yaml:"error_handling"`
	// Organization holds code organization preferences.
	Organization OrganizationPrefs `yaml:"organization"`
	// Naming holds naming convention preferences.
	Naming NamingPrefs `yaml:"naming"`
	// Abstraction holds abstraction and DRY preferences.
	Abstraction AbstractionPrefs `yaml:"abstraction"`
	// Documentation holds documentation standards.
	Documentation DocumentationPrefs `yaml:"documentation"`
	// Formatting holds formatting rule preferences.
	Formatting FormattingPrefs `yaml:"formatting"`
	// Dependencies holds dependency management preferences.
	Dependencies DependencyPrefs `yaml:"dependencies"`
	// Architecture holds architectural preferences.
	Architecture ArchitecturePrefs `yaml:"architecture"`
}

// TestingPrefs configures testing methodology.
type TestingPrefs struct {
	// Style is the testing methodology (tdd, test-after, minimal).
	Style TestingStyle `yaml:"style"`
	// CoverageTarget is the minimum coverage percentage.
	CoverageTarget int `yaml:"coverage_target"`
	// MockApproach describes how test doubles are created.
	MockApproach string `yaml:"mock_approach"`
	// TableDriven enables table-driven test patterns.
	TableDriven bool `yaml:"table_driven"`
}

// ErrorHandlingPrefs configures error handling patterns.
type ErrorHandlingPrefs struct {
	// Style is the error handling approach (sentinel, wrapping, custom-types).
	Style ErrorStyle `yaml:"style"`
	// EarlyReturn prefers early returns on error.
	EarlyReturn bool `yaml:"early_return"`
	// WrapContext always wraps errors with fmt.Errorf context.
	WrapContext bool `yaml:"wrap_context"`
}

// OrganizationPrefs configures code organization.
type OrganizationPrefs struct {
	// Style is the organizational approach (flat, clean-architecture, hexagonal).
	Style OrgStyle `yaml:"style"`
	// FileNaming is the file naming convention (snake_case, camelCase).
	FileNaming string `yaml:"file_naming"`
	// InternalPkg enables use of internal/ packages.
	InternalPkg bool `yaml:"internal_pkg"`
}

// NamingPrefs configures naming conventions.
type NamingPrefs struct {
	// ReceiverStyle is the method receiver naming style (single-letter, abbreviated).
	ReceiverStyle string `yaml:"receiver_style"`
	// InterfaceEr enables -er suffix for interfaces.
	InterfaceEr bool `yaml:"interface_er"`
	// Abbreviations lists preferred abbreviation forms (e.g., "ID not Id").
	Abbreviations string `yaml:"abbreviations"`
}

// AbstractionPrefs configures abstraction and complexity limits.
type AbstractionPrefs struct {
	// MaxFunctionLines is the maximum lines per function.
	MaxFunctionLines int `yaml:"max_function_lines"`
	// ExtractThreshold is the line count before extracting a helper.
	ExtractThreshold int `yaml:"extract_threshold"`
	// GenericsPolicy controls generics usage (prefer, avoid, when-obvious).
	GenericsPolicy string `yaml:"generics_policy"`
	// InterfaceOwnership controls where interfaces are defined (consumer, provider).
	InterfaceOwnership string `yaml:"interface_ownership"`
}

// DocumentationPrefs configures documentation standards.
type DocumentationPrefs struct {
	// GodocRequired requires godoc comments on all public functions.
	GodocRequired bool `yaml:"godoc_required"`
	// CommentStyle is the commenting approach (why-not-what, thorough, minimal).
	CommentStyle string `yaml:"comment_style"`
	// ReadmeRequired requires per-package README files.
	ReadmeRequired bool `yaml:"readme_required"`
}

// FormattingPrefs configures code formatting rules.
type FormattingPrefs struct {
	// LineLengthLimit is the maximum line length in characters.
	LineLengthLimit int `yaml:"line_length_limit"`
	// ImportOrder defines import group ordering.
	ImportOrder []string `yaml:"import_order"`
	// LinterConfig is the linter strictness level (strict, standard, minimal).
	LinterConfig string `yaml:"linter_config"`
}

// DependencyPrefs configures dependency management.
type DependencyPrefs struct {
	// Philosophy is the dependency selection approach (minimal, pragmatic, batteries-included).
	Philosophy string `yaml:"philosophy"`
	// ApprovedList is an optional list of pre-approved dependencies.
	ApprovedList []string `yaml:"approved_list,omitempty"`
	// Vendoring enables dependency vendoring.
	Vendoring bool `yaml:"vendoring"`
}

// ArchitecturePrefs configures architectural patterns.
type ArchitecturePrefs struct {
	// LayerStructure is the project layering approach (flat, layered, hexagonal).
	LayerStructure string `yaml:"layer_structure"`
	// DepDirection is the dependency direction rule (inward, downward).
	DepDirection string `yaml:"dep_direction"`
}

// IsZero returns true when the Preferences struct has not been populated.
func (p *Preferences) IsZero() bool {
	return p.Preset == "" && p.Version == ""
}

// Validate checks that the preferences are well-formed.
func (p *Preferences) Validate() error {
	if p.Version != CurrentVersion {
		return fmt.Errorf("%w: got %q, want %q", ErrUnsupportedVersion, p.Version, CurrentVersion)
	}

	validPresets := map[PresetName]bool{
		PresetStrict:    true,
		PresetPragmatic: true,
		PresetStartup:   true,
	}
	if !validPresets[p.Preset] {
		return fmt.Errorf("%w: %q", ErrInvalidPreset, p.Preset)
	}

	if p.Testing.CoverageTarget < 0 || p.Testing.CoverageTarget > 100 {
		return fmt.Errorf("coverage_target must be between 0 and 100, got %d", p.Testing.CoverageTarget)
	}

	return nil
}
