---
title: "Multi-Agent Workflows"
weight: 2
---

# Multi-Agent Workflows

Crux supports running multiple agents in parallel across separate tmux sessions. This guide covers how to configure and manage parallel execution safely.

## Defining Multiple Agents

Add multiple agents to your config, each with its own plugin, role, and permission:

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
  engineer-2:
    plugin: codex
    role: engineer
    permission: standard
  reviewer-1:
    plugin: claude
    role: reviewer
    permission: readonly
```

## Parallel Phase Requirements

For two phases to run in parallel, their file sets must be disjoint:

- `FilesNew` lists must not overlap
- `FilesModified` lists must not overlap
- `FilesRef` (read-only) can overlap

The phase engine validates this automatically. If conflicts are detected, the later-assigned agent is halted.

## Git Branch Strategy

Each agent works on a separate feature branch:

```
main
├── crux/engineer-1/phase-2a-prompt-1
├── crux/engineer-2/phase-2b-prompt-1
└── crux/reviewer-1/review-2a
```

Merging to main always requires human review.

## Agent Roles

Three built-in roles define agent behavior:

### Engineer

Implementation-focused. Writes code scoped to the current prompt, runs verification, updates work notes.

### Orchestrator

Coordination-focused. Assigns prompts, monitors world state, resolves conflicts, advances phases. Does not write code.

### Reviewer

Review-focused. Checks code quality, test coverage, error handling, and security. Provides structured feedback but does not rewrite code.

## Monitoring Parallel Work

```bash
# See all agent statuses
crux status

# View work notes for each phase
crux notes list

# Check for conflicts
crux phase validate
```

## Handling Conflicts

If the engine detects a file conflict between parallel agents:

1. The later-assigned agent is halted
2. The conflict is recorded in the decision journal
3. The orchestrator reassigns work or serializes the phases

To manually resolve, force-advance the blocking phase:

```bash
crux phase advance 2A --yes
```
