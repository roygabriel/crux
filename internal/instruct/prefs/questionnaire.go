package prefs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

// QuestionnaireModel holds the state of the preference questionnaire.
// It uses charmbracelet/huh forms for interactive input.
type QuestionnaireModel struct {
	prefs    *Preferences
	custom   string
	aborted  bool
}

// NewQuestionnaire creates a QuestionnaireModel with the given defaults.
// If defaults is nil, pragmatic defaults are used.
func NewQuestionnaire(defaults *Preferences) QuestionnaireModel {
	if defaults == nil {
		defaults = PresetDefaults(PresetPragmatic)
	}
	// Deep copy so mutations don't affect the caller.
	p := *defaults
	return QuestionnaireModel{prefs: &p}
}

// Result returns the assembled Preferences after the questionnaire completes.
// Returns nil if the questionnaire was aborted.
func (q QuestionnaireModel) Result() *Preferences {
	if q.aborted {
		return nil
	}
	return q.prefs
}

// Custom returns any freeform custom preference text entered by the user.
func (q QuestionnaireModel) Custom() string {
	return q.custom
}

// Run executes the full interactive questionnaire flow.
// Returns an error if the user cancels or a form fails.
func (q *QuestionnaireModel) Run() error {
	// Step 1: Choose base preset.
	if err := q.runPresetSelection(); err != nil {
		q.aborted = true
		return fmt.Errorf("preset selection: %w", err)
	}

	// Step 2-5: Optional category customizations.
	customizers := []struct {
		title string
		fn    func() error
	}{
		{"Customize testing?", q.runTestingCustomization},
		{"Customize error handling?", q.runErrorHandlingCustomization},
		{"Customize code organization?", q.runOrganizationCustomization},
		{"Customize abstraction rules?", q.runAbstractionCustomization},
	}

	for _, c := range customizers {
		var customize bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(c.title).
					Description("Press Enter to skip with preset defaults").
					Affirmative("Yes").
					Negative("No").
					Value(&customize),
			),
		)
		if err := form.Run(); err != nil {
			q.aborted = true
			return fmt.Errorf("customize prompt: %w", err)
		}
		if customize {
			if err := c.fn(); err != nil {
				q.aborted = true
				return fmt.Errorf("customization: %w", err)
			}
		}
	}

	// Step 6: Freeform custom preferences.
	if err := q.runCustomInput(); err != nil {
		q.aborted = true
		return fmt.Errorf("custom input: %w", err)
	}

	// Step 7: Summary and confirmation.
	if err := q.runConfirmation(); err != nil {
		q.aborted = true
		return fmt.Errorf("confirmation: %w", err)
	}

	return nil
}

func (q *QuestionnaireModel) runPresetSelection() error {
	var preset string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose a base preset").
				Description("This sets defaults for all categories — you can customize individual settings next").
				Options(
					huh.NewOption("Strict — rigorous standards, TDD, 90% coverage, clean architecture", string(PresetStrict)),
					huh.NewOption("Pragmatic — balanced quality and velocity, 70% coverage", string(PresetPragmatic)),
					huh.NewOption("Startup — speed-first, minimal testing, flat structure", string(PresetStartup)),
				).
				Value(&preset),
		).Title("Engineering Preferences"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Apply preset defaults.
	defaults := PresetDefaults(PresetName(preset))
	// Preserve language from prior config if set.
	lang := q.prefs.Language
	*q.prefs = *defaults
	if lang != "" {
		q.prefs.Language = lang
	}

	return nil
}

func (q *QuestionnaireModel) runTestingCustomization() error {
	var style string
	coverageStr := strconv.Itoa(q.prefs.Testing.CoverageTarget)
	var mockApproach string
	tableDriven := q.prefs.Testing.TableDriven

	style = string(q.prefs.Testing.Style)
	mockApproach = q.prefs.Testing.MockApproach

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Testing style").
				Options(
					huh.NewOption("TDD — write tests before implementation", string(TestingTDD)),
					huh.NewOption("Test after — write tests after implementation", string(TestingTestAfter)),
					huh.NewOption("Minimal — test critical paths only", string(TestingMinimal)),
				).
				Value(&style),
			huh.NewInput().
				Title("Coverage target (0-100)").
				Description("Minimum test coverage percentage").
				Value(&coverageStr).
				Validate(validateCoverageInput),
			huh.NewSelect[string]().
				Title("Mock approach").
				Options(
					huh.NewOption("Interfaces — use interfaces for test doubles", "interfaces"),
					huh.NewOption("Generated — use generated mocks", "generated"),
					huh.NewOption("Minimal — inline fakes, minimal setup", "minimal"),
				).
				Value(&mockApproach),
			huh.NewConfirm().
				Title("Use table-driven tests?").
				Value(&tableDriven),
		).Title("Testing Preferences"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	q.prefs.Testing.Style = TestingStyle(style)
	if n, err := strconv.Atoi(coverageStr); err == nil {
		q.prefs.Testing.CoverageTarget = n
	}
	q.prefs.Testing.MockApproach = mockApproach
	q.prefs.Testing.TableDriven = tableDriven

	return nil
}

func (q *QuestionnaireModel) runErrorHandlingCustomization() error {
	var style string
	earlyReturn := q.prefs.ErrorHandling.EarlyReturn
	wrapContext := q.prefs.ErrorHandling.WrapContext

	style = string(q.prefs.ErrorHandling.Style)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Error handling style").
				Options(
					huh.NewOption("Custom types — sentinel errors + typed errors", string(ErrorCustom)),
					huh.NewOption("Wrapping — wrap all errors with fmt.Errorf", string(ErrorWrapping)),
					huh.NewOption("Sentinel — package-level sentinel errors", string(ErrorSentinel)),
				).
				Value(&style),
			huh.NewConfirm().
				Title("Use early returns on error?").
				Value(&earlyReturn),
			huh.NewConfirm().
				Title("Always wrap errors with context?").
				Value(&wrapContext),
		).Title("Error Handling Preferences"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	q.prefs.ErrorHandling.Style = ErrorStyle(style)
	q.prefs.ErrorHandling.EarlyReturn = earlyReturn
	q.prefs.ErrorHandling.WrapContext = wrapContext

	return nil
}

func (q *QuestionnaireModel) runOrganizationCustomization() error {
	var style string
	var fileNaming string
	internalPkg := q.prefs.Organization.InternalPkg

	style = string(q.prefs.Organization.Style)
	fileNaming = q.prefs.Organization.FileNaming

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Code organization style").
				Options(
					huh.NewOption("Flat — group by feature, minimal nesting", string(OrgFlat)),
					huh.NewOption("Clean architecture — domain, application, infrastructure layers", string(OrgClean)),
					huh.NewOption("Hexagonal — ports and adapters", string(OrgHexagonal)),
				).
				Value(&style),
			huh.NewSelect[string]().
				Title("File naming convention").
				Options(
					huh.NewOption("snake_case — user_store.go", "snake_case"),
					huh.NewOption("camelCase — userStore.go", "camelCase"),
				).
				Value(&fileNaming),
			huh.NewConfirm().
				Title("Use internal/ packages?").
				Description("Hides implementation from external consumers").
				Value(&internalPkg),
		).Title("Code Organization Preferences"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	q.prefs.Organization.Style = OrgStyle(style)
	q.prefs.Organization.FileNaming = fileNaming
	q.prefs.Organization.InternalPkg = internalPkg

	return nil
}

func (q *QuestionnaireModel) runAbstractionCustomization() error {
	maxLinesStr := strconv.Itoa(q.prefs.Abstraction.MaxFunctionLines)
	var genericsPolicy string

	genericsPolicy = q.prefs.Abstraction.GenericsPolicy

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Max function lines").
				Description("Maximum lines per function before requiring extraction").
				Value(&maxLinesStr).
				Validate(validatePositiveInt),
			huh.NewSelect[string]().
				Title("Generics policy").
				Options(
					huh.NewOption("When obvious — use for containers, slices helpers", "when-obvious"),
					huh.NewOption("Prefer — use generics to reduce duplication", "prefer"),
					huh.NewOption("Avoid — stick to concrete types and interfaces", "avoid"),
				).
				Value(&genericsPolicy),
		).Title("Abstraction Preferences"),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if n, err := strconv.Atoi(maxLinesStr); err == nil {
		q.prefs.Abstraction.MaxFunctionLines = n
		// Set extract threshold to half the max lines.
		q.prefs.Abstraction.ExtractThreshold = n / 2
	}
	q.prefs.Abstraction.GenericsPolicy = genericsPolicy

	return nil
}

func (q *QuestionnaireModel) runCustomInput() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Any additional preferences?").
				Description("Freeform text for project-specific rules (leave empty to skip)").
				CharLimit(500).
				Value(&q.custom),
		),
	)

	return form.Run()
}

func (q *QuestionnaireModel) runConfirmation() error {
	summary := RenderPreferencesSummary(q.prefs, q.custom)
	fmt.Println("\n" + summary)

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save these preferences?").
				Affirmative("Yes, save").
				Negative("No, cancel").
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !confirmed {
		return fmt.Errorf("preferences cancelled by user")
	}

	return nil
}

// RenderPreferencesSummary produces a formatted text summary of preferences.
func RenderPreferencesSummary(p *Preferences, custom string) string {
	var b strings.Builder

	b.WriteString("=== Engineering Preferences Summary ===\n\n")
	b.WriteString(fmt.Sprintf("  Preset:            %s\n", p.Preset))
	if p.Language != "" {
		b.WriteString(fmt.Sprintf("  Language:          %s\n", p.Language))
	}
	b.WriteString("\n")

	b.WriteString("  Testing\n")
	b.WriteString(fmt.Sprintf("    Style:           %s\n", p.Testing.Style))
	b.WriteString(fmt.Sprintf("    Coverage:        %d%%\n", p.Testing.CoverageTarget))
	b.WriteString(fmt.Sprintf("    Mock approach:   %s\n", p.Testing.MockApproach))
	b.WriteString(fmt.Sprintf("    Table-driven:    %v\n", p.Testing.TableDriven))
	b.WriteString("\n")

	b.WriteString("  Error Handling\n")
	b.WriteString(fmt.Sprintf("    Style:           %s\n", p.ErrorHandling.Style))
	b.WriteString(fmt.Sprintf("    Early return:    %v\n", p.ErrorHandling.EarlyReturn))
	b.WriteString(fmt.Sprintf("    Wrap context:    %v\n", p.ErrorHandling.WrapContext))
	b.WriteString("\n")

	b.WriteString("  Organization\n")
	b.WriteString(fmt.Sprintf("    Style:           %s\n", p.Organization.Style))
	b.WriteString(fmt.Sprintf("    File naming:     %s\n", p.Organization.FileNaming))
	b.WriteString(fmt.Sprintf("    Internal pkg:    %v\n", p.Organization.InternalPkg))
	b.WriteString("\n")

	b.WriteString("  Naming\n")
	b.WriteString(fmt.Sprintf("    Receiver style:  %s\n", p.Naming.ReceiverStyle))
	b.WriteString(fmt.Sprintf("    Interface -er:   %v\n", p.Naming.InterfaceEr))
	if p.Naming.Abbreviations != "" {
		b.WriteString(fmt.Sprintf("    Abbreviations:   %s\n", p.Naming.Abbreviations))
	}
	b.WriteString("\n")

	b.WriteString("  Abstraction\n")
	b.WriteString(fmt.Sprintf("    Max func lines:  %d\n", p.Abstraction.MaxFunctionLines))
	b.WriteString(fmt.Sprintf("    Extract at:      %d lines\n", p.Abstraction.ExtractThreshold))
	b.WriteString(fmt.Sprintf("    Generics:        %s\n", p.Abstraction.GenericsPolicy))
	b.WriteString(fmt.Sprintf("    Interface owner: %s\n", p.Abstraction.InterfaceOwnership))
	b.WriteString("\n")

	b.WriteString("  Documentation\n")
	b.WriteString(fmt.Sprintf("    Godoc required:  %v\n", p.Documentation.GodocRequired))
	b.WriteString(fmt.Sprintf("    Comment style:   %s\n", p.Documentation.CommentStyle))
	b.WriteString(fmt.Sprintf("    README required: %v\n", p.Documentation.ReadmeRequired))
	b.WriteString("\n")

	b.WriteString("  Formatting\n")
	b.WriteString(fmt.Sprintf("    Line length:     %d\n", p.Formatting.LineLengthLimit))
	b.WriteString(fmt.Sprintf("    Import order:    %s\n", strings.Join(p.Formatting.ImportOrder, ", ")))
	b.WriteString(fmt.Sprintf("    Linter config:   %s\n", p.Formatting.LinterConfig))
	b.WriteString("\n")

	b.WriteString("  Dependencies\n")
	b.WriteString(fmt.Sprintf("    Philosophy:      %s\n", p.Dependencies.Philosophy))
	b.WriteString(fmt.Sprintf("    Vendoring:       %v\n", p.Dependencies.Vendoring))
	b.WriteString("\n")

	b.WriteString("  Architecture\n")
	b.WriteString(fmt.Sprintf("    Layer structure: %s\n", p.Architecture.LayerStructure))
	b.WriteString(fmt.Sprintf("    Dep direction:   %s\n", p.Architecture.DepDirection))

	if custom != "" {
		b.WriteString("\n  Custom\n")
		b.WriteString(fmt.Sprintf("    %s\n", custom))
	}

	return b.String()
}

// validateCoverageInput validates that a string is a valid coverage percentage.
func validateCoverageInput(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n < 0 || n > 100 {
		return fmt.Errorf("must be between 0 and 100")
	}
	return nil
}

// validatePositiveInt validates that a string is a positive integer.
func validatePositiveInt(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n <= 0 {
		return fmt.Errorf("must be positive")
	}
	return nil
}
