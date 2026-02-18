package prefs

import (
	"fmt"
	"strings"

	"github.com/roygabriel/crux/internal/instruct"
)

// categories enumerates the valid category names for CompileCategory.
var categories = map[string]func(*Preferences) string{
	"testing":        compileTesting,
	"error_handling": compileErrorHandling,
	"organization":   compileOrganization,
	"naming":         compileNaming,
	"abstraction":    compileAbstraction,
	"documentation":  compileDocumentation,
	"formatting":     compileFormatting,
	"dependencies":   compileDependencies,
	"architecture":   compileArchitecture,
}

// Compile transforms structured preferences into concrete instruction text
// for every category. The output is deterministic for a given input.
func Compile(prefs *Preferences) *instruct.PreferenceInstructions {
	return &instruct.PreferenceInstructions{
		Testing:       compileTesting(prefs),
		ErrorHandling: compileErrorHandling(prefs),
		Organization:  compileOrganization(prefs),
		Naming:        compileNaming(prefs),
		Abstraction:   compileAbstraction(prefs),
		Documentation: compileDocumentation(prefs),
		Formatting:    compileFormatting(prefs),
		Dependencies:  compileDependencies(prefs),
		Architecture:  compileArchitecture(prefs),
	}
}

// CompileCategory transforms a single preference category into instruction
// text. Returns an empty string for unknown category names.
func CompileCategory(prefs *Preferences, category string) string {
	fn, ok := categories[category]
	if !ok {
		return ""
	}
	return fn(prefs)
}

func compileTesting(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.Testing.Style {
	case TestingTDD:
		b.WriteString("Write tests BEFORE implementation. Every public function must have a corresponding test. ")
	case TestingTestAfter:
		b.WriteString("Write tests after implementation. All public functions should have tests. ")
	case TestingMinimal:
		b.WriteString("Test critical business logic paths. Focus on functions that handle money, auth, and data integrity. ")
	default:
		b.WriteString("Write tests for public functions. ")
	}

	if prefs.Testing.TableDriven {
		switch prefs.Testing.Style {
		case TestingTDD:
			b.WriteString("Use table-driven tests with at least: happy path, two error paths, nil/zero-value inputs, and boundary conditions. ")
		default:
			b.WriteString("Use table-driven tests. Cover happy path and primary error paths. ")
		}
	} else {
		b.WriteString("Keep tests simple and fast. ")
	}

	b.WriteString(fmt.Sprintf("Target %d%% coverage. ", prefs.Testing.CoverageTarget))

	switch prefs.Testing.MockApproach {
	case "interfaces":
		b.WriteString("Use interfaces for test doubles — no mocking frameworks.")
	case "generated":
		b.WriteString("Use generated mocks from interfaces for test doubles.")
	case "minimal":
		b.WriteString("Keep test setup minimal — inline fakes over shared fixtures.")
	default:
		b.WriteString("Use interfaces for dependency injection in tests.")
	}

	return b.String()
}

func compileErrorHandling(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.ErrorHandling.Style {
	case ErrorCustom:
		b.WriteString("Define sentinel errors at package level: `var ErrNotFound = errors.New(\"not found\")`. ")
		b.WriteString("Create custom error types for errors that carry data: `type ValidationError struct { Field, Message string }`. ")
	case ErrorSentinel:
		b.WriteString("Define sentinel errors at package level: `var ErrNotFound = errors.New(\"not found\")`. ")
		b.WriteString("Use `errors.Is` and `errors.As` for error checks — never compare error strings. ")
	case ErrorWrapping:
		b.WriteString("Wrap all errors with context using `fmt.Errorf(\"operation: %w\", err)`. ")
	default:
		b.WriteString("Handle all errors explicitly. ")
	}

	if prefs.ErrorHandling.WrapContext {
		if prefs.ErrorHandling.Style != ErrorWrapping {
			b.WriteString("Always wrap errors with context: `fmt.Errorf(\"loading config: %w\", err)`. ")
		}
	}

	if prefs.ErrorHandling.EarlyReturn {
		b.WriteString("Use early returns — never nest error handling. ")
	}

	b.WriteString("Check errors on every fallible call — never use `_`.")

	return b.String()
}

func compileOrganization(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.Organization.Style {
	case OrgClean:
		b.WriteString("Use clean architecture: separate domain, application, and infrastructure layers. ")
		b.WriteString("Domain types must not import infrastructure packages. ")
	case OrgHexagonal:
		b.WriteString("Use hexagonal architecture: ports (interfaces) in the domain, adapters at the edges. ")
		b.WriteString("All external dependencies connect through adapter implementations. ")
	case OrgFlat:
		b.WriteString("Use a flat package layout. Group by feature, not by layer. ")
		b.WriteString("Avoid deep nesting — prefer fewer, well-named packages. ")
	default:
		b.WriteString("Organize code into clear, cohesive packages. ")
	}

	switch prefs.Organization.FileNaming {
	case "snake_case":
		b.WriteString("Name files in snake_case: `user_store.go`, `auth_handler.go`. ")
	case "camelCase":
		b.WriteString("Name files in camelCase: `userStore.go`, `authHandler.go`. ")
	}

	if prefs.Organization.InternalPkg {
		b.WriteString("Use `internal/` packages to hide implementation details from external consumers.")
	} else {
		b.WriteString("Keep package structure simple — avoid `internal/` unless the project is a library.")
	}

	return b.String()
}

func compileNaming(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.Naming.ReceiverStyle {
	case "single-letter":
		b.WriteString("Use single-letter method receivers: `func (s *Store) Get(...)`. ")
	case "abbreviated":
		b.WriteString("Use abbreviated method receivers: `func (st *Store) Get(...)`, `func (cfg *Config) Validate(...)`. ")
	default:
		b.WriteString("Use concise method receiver names. ")
	}

	if prefs.Naming.InterfaceEr {
		b.WriteString("Name interfaces by behavior with `-er` suffix: `Reader`, `Sender`, `Validator`. ")
	} else {
		b.WriteString("Name interfaces descriptively: `Store`, `Engine`, `Plugin`. ")
	}

	if prefs.Naming.Abbreviations != "" {
		b.WriteString(fmt.Sprintf("Use standard abbreviations: %s. ", prefs.Naming.Abbreviations))
	}

	b.WriteString("Exported names should be clear without package qualifier — avoid stutter like `config.ConfigStore`.")

	return b.String()
}

func compileAbstraction(prefs *Preferences) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Functions must not exceed %d lines. ", prefs.Abstraction.MaxFunctionLines))
	b.WriteString(fmt.Sprintf("If a function exceeds %d lines, consider extracting a helper. ", prefs.Abstraction.ExtractThreshold))
	b.WriteString("Prefer composition over deep nesting. ")

	switch prefs.Abstraction.GenericsPolicy {
	case "prefer":
		b.WriteString("Use generics where they reduce duplication across types. ")
	case "avoid":
		b.WriteString("Avoid generics — use concrete types and interfaces instead. ")
	case "when-obvious":
		b.WriteString("Use generics only when the benefit is obvious (e.g., container types, `slices` helpers). ")
	}

	switch prefs.Abstraction.InterfaceOwnership {
	case "consumer":
		b.WriteString("Extract interfaces at the consumer side — define them where they're used, not where they're implemented.")
	case "provider":
		b.WriteString("Define interfaces alongside their implementations — co-locate the contract with the provider.")
	default:
		b.WriteString("Place interfaces where they make the dependency graph clearest.")
	}

	return b.String()
}

func compileDocumentation(prefs *Preferences) string {
	var b strings.Builder

	if prefs.Documentation.GodocRequired {
		b.WriteString("Every exported type, function, and method must have a godoc comment. ")
		b.WriteString("Start godoc comments with the identifier name: `// Store provides persistence for...`. ")
	} else {
		b.WriteString("Add godoc comments to exported types and non-obvious functions. Skip trivial getters and setters. ")
	}

	switch prefs.Documentation.CommentStyle {
	case "why-not-what":
		b.WriteString("Comments explain WHY, not WHAT. Do NOT restate the code in English. ")
	case "thorough":
		b.WriteString("Document all public APIs, parameters, return values, and edge cases. ")
	case "minimal":
		b.WriteString("Minimize comments. Code should be self-documenting — rename before adding a comment. ")
	}

	if prefs.Documentation.ReadmeRequired {
		b.WriteString("Each package must have a README.md describing its purpose, usage, and key types.")
	} else {
		b.WriteString("Package-level doc comments are sufficient — no per-package README required.")
	}

	return b.String()
}

func compileFormatting(prefs *Preferences) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Maximum line length: %d characters. ", prefs.Formatting.LineLengthLimit))

	if len(prefs.Formatting.ImportOrder) > 0 {
		b.WriteString(fmt.Sprintf("Group imports in order: %s. Separate groups with a blank line. ",
			strings.Join(prefs.Formatting.ImportOrder, ", ")))
	}

	b.WriteString("Run `gofmt` on all files — no exceptions. ")

	switch prefs.Formatting.LinterConfig {
	case "strict":
		b.WriteString("Enable all linter checks: `golangci-lint run` with exhaustive, gocritic, and revive. Zero warnings allowed.")
	case "standard":
		b.WriteString("Run `golangci-lint run` with default checks. Fix all errors; warnings are advisory.")
	case "minimal":
		b.WriteString("Run `go vet` at minimum. Use linters selectively — focus on correctness over style.")
	default:
		b.WriteString("Run `go vet` on all packages before committing.")
	}

	return b.String()
}

func compileDependencies(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.Dependencies.Philosophy {
	case "minimal":
		b.WriteString("Minimize external dependencies. Prefer stdlib solutions. ")
		b.WriteString("Every new dependency must justify itself — write a brief rationale in work notes before adding. ")
	case "pragmatic":
		b.WriteString("Use well-maintained dependencies when they save significant effort. ")
		b.WriteString("Prefer libraries with stable APIs and active maintenance. ")
	case "batteries-included":
		b.WriteString("Use dependencies freely to move fast. Prefer feature-complete libraries over rolling your own. ")
	default:
		b.WriteString("Evaluate dependencies on merit — stability, maintenance, and API quality. ")
	}

	if len(prefs.Dependencies.ApprovedList) > 0 {
		b.WriteString(fmt.Sprintf("Pre-approved dependencies: %s. ",
			strings.Join(prefs.Dependencies.ApprovedList, ", ")))
	}

	if prefs.Dependencies.Vendoring {
		b.WriteString("Vendor all dependencies with `go mod vendor`. Commit the vendor directory.")
	} else {
		b.WriteString("Do NOT vendor — use Go module proxy for reproducible builds.")
	}

	return b.String()
}

func compileArchitecture(prefs *Preferences) string {
	var b strings.Builder

	switch prefs.Architecture.LayerStructure {
	case "layered":
		b.WriteString("Use layered architecture: handlers → services → repositories. ")
		b.WriteString("Each layer has a clear responsibility and well-defined interfaces. ")
	case "hexagonal":
		b.WriteString("Use hexagonal architecture: domain core with port interfaces, adapters at boundaries. ")
		b.WriteString("The domain must never import adapter packages. ")
	case "flat":
		b.WriteString("Use a flat structure — avoid unnecessary layers. ")
		b.WriteString("If a service has one implementation, skip the interface until you need a second. ")
	default:
		b.WriteString("Structure the project to keep the dependency graph clear and acyclic. ")
	}

	switch prefs.Architecture.DepDirection {
	case "inward":
		b.WriteString("Dependencies point inward: outer layers depend on inner layers, never the reverse. ")
		b.WriteString("Use dependency injection to invert control at layer boundaries.")
	case "downward":
		b.WriteString("Dependencies flow downward: higher-level packages call lower-level ones. ")
		b.WriteString("Avoid circular dependencies — if package A imports B, B must not import A.")
	default:
		b.WriteString("Keep the dependency graph acyclic. Use interfaces to break cycles when needed.")
	}

	return b.String()
}
