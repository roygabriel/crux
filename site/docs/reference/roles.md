---
sidebar_position: 2
title: Roles
description: Built-in agent role definitions in Crux. Engineer, Orchestrator, and Reviewer roles with their rules and behaviors for multi-agent coordination.
---

# Role Definitions

Crux ships with three built-in roles that define agent behavior. Roles are assigned in the agent configuration and injected into each agent's prompt.

## Engineer

Implementation-focused role for writing code.

| Rule | Description |
|------|-------------|
| Scope | Write code strictly scoped to the current prompt |
| Tests | Table-driven tests with descriptive names |
| Errors | Wrap errors with context (`fmt.Errorf("op: %w", err)`) |
| No panics | Return errors instead of panicking |
| Interfaces | Extract interfaces from external dependencies for testability |
| Exports | Minimize exported API surface |
| Context | Use `context.Context` as first parameter for I/O functions |
| Verification | Run all verification commands before stopping |
| Work notes | Update work notes after completing a prompt |

## Orchestrator

Coordination-focused role for managing agent workflows.

| Rule | Description |
|------|-------------|
| Assignment | Assign prompts to idle agents, never reuse busy agents |
| Monitoring | Monitor world state continuously |
| Conflicts | Resolve file and resource conflicts between agents |
| Advancement | Advance phases only after all prompts and gates pass |
| Delegation | Do not implement code directly |
| Journal | Track all decisions in the decision journal |
| Permissions | Respect permission tiers for each agent |
| Error handling | Handle error states (rate limits, hangs, failures) |
| Audit | Maintain audit trail for all actions |
| Summaries | Summarize progress at phase boundaries |

## Reviewer

Code review-focused role for quality assurance.

| Rule | Description |
|------|-------------|
| Quality | Check naming, structure, duplication, and conventions |
| Coverage | Verify test coverage meets threshold, flag untested error paths |
| Error handling | Review error wrapping, no silent failures |
| Security | Check input validation, injection risks, no hardcoded credentials |
| Feedback | Structured feedback with file paths and line numbers |
| Severity | Classify findings as blocking, warning, or nit |
| Approval | Approve only when blocking issues are resolved and gates pass |
| No rewrites | Describe problems, do not rewrite code |
| Scope | Flag scope creep beyond the current prompt |

## Assigning Roles

Roles are assigned per agent in `.crux/config.yaml`:

```yaml
agents:
  orchestrator:
    plugin: claude
    role: orchestrator
    permission: elevated
  engineer-1:
    plugin: claude
    role: engineer
    permission: standard
  reviewer-1:
    plugin: claude
    role: reviewer
    permission: readonly
```
