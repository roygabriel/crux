package bank

// templateProjectBrief is the starter content for projectBrief.md.
const templateProjectBrief = `# Project Brief

## Overview
[High-level description of what this project does and why it exists]

## Goals
[Primary objectives and success criteria]

## Scope
[What is included and excluded from this project]

## Timeline
[Key milestones and target dates]
`

// templateProductContext is the starter content for productContext.md.
const templateProductContext = `# Product Context

## Problem Statement
[What problem does this product solve]

## Target Users
[Who uses this product and what are their needs]

## User Experience
[How the product should work from the user's perspective]

## Competitive Landscape
[Alternative solutions and differentiators]
`

// templateSystemPatterns is the starter content for systemPatterns.md.
const templateSystemPatterns = `# System Patterns

## Architecture
[High-level architectural decisions and patterns]

## Design Patterns
[Recurring design patterns used in the codebase]

## Conventions
[Naming conventions, file organization, and coding standards]

## Error Handling
[How errors are propagated and reported]
`

// templateTechContext is the starter content for techContext.md.
const templateTechContext = `# Tech Context

## Language & Runtime
[Programming language, version, and runtime environment]

## Dependencies
[Key external libraries and their purposes]

## Build System
[How the project is built, tested, and deployed]

## Infrastructure
[Hosting, storage, and external services]
`

// templateActiveContext is the starter content for activeContext.md.
const templateActiveContext = `# Active Context

## Current Focus
[What is actively being worked on right now]

## Recent Changes
[What changed recently that affects current work]

## Open Questions
[Unresolved decisions or blockers]

## Next Steps
[Immediate next actions to take]
`

// templateProgress is the starter content for progress.md.
const templateProgress = `# Progress

## Completed
[Phases, features, or milestones that are done]

## In Progress
[What is currently being worked on]

## Upcoming
[What comes next in the plan]

## Known Issues
[Bugs, tech debt, or problems to address]
`

// templateFiles maps each memory bank filename to its template content.
var templateFiles = map[string]string{
	"projectBrief.md":   templateProjectBrief,
	"productContext.md":  templateProductContext,
	"systemPatterns.md":  templateSystemPatterns,
	"techContext.md":     templateTechContext,
	"activeContext.md":   templateActiveContext,
	"progress.md":        templateProgress,
}

// TemplateFilenames returns the ordered list of memory bank filenames
// for deterministic iteration.
func TemplateFilenames() []string {
	return []string{
		"projectBrief.md",
		"productContext.md",
		"systemPatterns.md",
		"techContext.md",
		"activeContext.md",
		"progress.md",
	}
}
