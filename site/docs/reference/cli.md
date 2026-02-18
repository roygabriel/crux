---
sidebar_position: 3
title: CLI Reference
---

# CLI Reference

Complete list of Crux commands and their usage.

## `crux init`

Initialize a new project with Crux configuration and scaffolding.

```bash
crux init                          # Interactive wizard (when in a terminal)
crux init -y                       # Non-interactive, use defaults
crux init --example httpapi        # Seed with named example project
```

Creates `.crux/config.yaml`, `docs/phases/`, and `work-notes/` directories.

## `crux start`

Start the orchestration loop.

```bash
crux start
```

Launches tmux sessions for each configured agent, assigns prompts from phase docs, and begins the continuous poll-verify-advance loop.

## `crux status`

Show the current world state.

```bash
crux status
```

Displays agent statuses, current phases, prompt progress, gate results, and open questions.

## `crux phase`

Phase management commands.

```bash
crux phase create --id 2a          # Generate phase spec + prompt doc
crux phase list                    # List all phases with status
crux phase show 2a                 # Show details for a phase
crux phase validate                # Verify all specs have exit criteria
crux phase advance 2a              # Force-advance a phase (human override)
```

### `crux phase create`

| Flag | Description |
|------|-------------|
| `--id` | Phase identifier (e.g., `2A`, `3B`) |
| `--name` | Human-readable phase name |
| `--depends-on` | Comma-separated list of dependency phase IDs |
| `--prompts` | Number of prompts to scaffold |

### `crux phase validate`

Checks:
- Every spec has exit criteria
- Every prompt has verification commands
- Parallel phases have no file conflicts

## `crux decisions`

Search and export the decision journal.

```bash
crux decisions search "query"      # Semantic search across decision journal
crux decisions list --since 24h    # List recent decisions
crux decisions export              # Export all decisions as JSON
```

## `crux notes`

View and manage per-phase work notes.

```bash
crux notes show 2a                 # Display work notes for a phase
crux notes list                    # List all work notes
```

## `crux replay`

Replay a session transcript.

```bash
crux replay <session-id>           # Replay a session transcript
```

## `crux audit`

View the structured audit log.

```bash
crux audit list --since 24h        # View audit log with filters
crux audit stats                   # Summary statistics
```

| Flag | Description |
|------|-------------|
| `--since` | Time window (e.g., `1h`, `24h`, `7d`) |
| `--agent` | Filter by agent name |
| `--type` | Filter by event type |

## `crux completion`

Generate shell completions.

```bash
crux completion bash               # Bash completions
crux completion zsh                # Zsh completions
crux completion fish               # Fish completions
crux completion powershell         # PowerShell completions
```

See [Installation: Shell Completions](/docs/getting-started/installation#shell-completions) for permanent installation instructions.

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `.crux/config.yaml`) |
| `--verbose` | Enable verbose logging |
| `--version` | Print version information |
| `--help` | Show help for any command |
