---
title: "Crux Documentation"
type: docs
---

# Crux

A single-binary Go orchestrator that coordinates AI coding agents across tmux sessions with persistent memory, phase-based workflow enforcement, and vector-searchable decision tracking.

Crux manages Claude Code, Codex CLI, Gemini CLI, or any configurable CLI tool -- assigning tasks from phase documents, enforcing verification gates between prompts, and maintaining a decision journal so the orchestrator never loses track of what agents are doing.

## Why Crux?

Multi-agent coding breaks in predictable ways:

- The orchestrator forgets what agents decided three prompts ago and issues contradictory instructions.
- Agents drift from scope because nothing enforces verification between steps.
- Sessions die and all context is lost -- the next session starts from scratch.
- Parallel agents silently edit the same files and produce merge conflicts.

Crux fixes these by automating the Plan, Prompt, Execute, Verify, Commit loop, turning a manual human workflow into an automated system that runs across multiple agents simultaneously.

## Getting Started

- [Installation]({{< relref "getting-started/installation" >}}) -- install Crux from source or binary
- [First Project]({{< relref "getting-started/first-project" >}}) -- walk through creating and running your first project

## Learn More

- [Architecture]({{< relref "concepts/architecture" >}}) -- how Crux works internally
- [Phase System]({{< relref "concepts/phase-system" >}}) -- the two-document phase model
- [Memory System]({{< relref "concepts/memory" >}}) -- three-layer persistent memory
- [Configuration Reference]({{< relref "reference/configuration" >}}) -- every config key explained
