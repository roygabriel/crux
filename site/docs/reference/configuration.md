---
sidebar_position: 1
title: Configuration
description: Full YAML configuration reference for Crux. Covers project settings, agent definitions, memory backends, phase directories, security controls, context budgets, and generic plugins.
---

# Configuration Reference

Crux reads YAML configuration from `.crux/config.yaml`. Every field can be overridden with `CRUX_` prefixed environment variables.

## Project

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `project.name` | string | `"my-project"` | Human-readable project name |
| `project.root` | string | `"."` | Project root directory |
| `project.state_dir` | string | `".crux"` | Directory for Crux state files |

Environment overrides: `CRUX_PROJECT_NAME`, `CRUX_PROJECT_ROOT`, `CRUX_PROJECT_STATE_DIR`

## Agents

Each agent is a named entry under `agents:` with the following fields:

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `plugin` | string | Yes | Adapter name: `claude`, `codex`, `gemini`, `generic`, or a custom plugin name |
| `role` | string | Yes | Functional role: `orchestrator`, `project-manager`, `engineer`, `reviewer` |
| `permission` | string | Yes | Security tier: `readonly`, `standard`, `elevated`, `autonomous` |
| `model` | string | No | Optional model identifier passed to the plugin |

Example:

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
    model: opus
  engineer-2:
    plugin: codex
    role: engineer
    permission: standard
```

## Memory

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `memory.sqlite_path` | string | `".crux/memory.db"` | Path to SQLite database file |
| `memory.vector_dir` | string | `".crux/vectors"` | Directory for vector index persistence |
| `memory.embedding_provider` | string | `"chromem-default"` | Embedding backend: `chromem-default` or `ollama` |
| `memory.embedding_model` | string | `"nomic-embed-text"` | Model name when using the ollama provider |

Environment overrides: `CRUX_MEMORY_SQLITE_PATH`, `CRUX_MEMORY_VECTOR_DIR`, `CRUX_MEMORY_EMBEDDING_PROVIDER`, `CRUX_MEMORY_EMBEDDING_MODEL`

## Phases

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `phases.spec_dir` | string | `"docs/phases"` | Directory containing phase specification files |

Environment override: `CRUX_PHASES_SPEC_DIR`

## Security

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `security.audit_log` | string | `".crux/audit.log"` | Path to the structured JSON audit log |
| `security.max_cmds_per_min` | int | `60` | Maximum commands an agent may execute per minute |
| `security.max_files_per_session` | int | `100` | Maximum files an agent may modify per session |
| `security.allowed_paths` | []string | `[]` | Restrict file operations to these path prefixes |
| `security.denied_paths` | []string | `[]` | Block file operations on these paths |

Environment overrides: `CRUX_SECURITY_AUDIT_LOG`, `CRUX_SECURITY_MAX_CMDS_PER_MIN`, `CRUX_SECURITY_MAX_FILES_PER_SESSION`

## Context

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `context.total_budget` | int | 0 | Total token budget for all context sections |
| `context.world_state` | int | 0 | Token budget for world state injection |
| `context.decision_rag` | int | 0 | Token budget for decision RAG context |
| `context.summary` | int | 0 | Token budget for work notes summary |
| `context.reserve` | int | 0 | Token budget reserved for prompt structure |
| `context.ready_timeout` | string | `"20s"` | Wait for explicit ready detection before fallback dispatch readiness |

Environment override: `CRUX_CONTEXT_READY_TIMEOUT`

## Generic Plugins

Custom plugin adapters configured via regex patterns:

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | string | Yes | Display name |
| `binary` | string | Yes | Executable to launch |
| `args` | []string | No | Default CLI arguments |
| `ready_pattern` | string | Yes | Regex for ready state detection |
| `busy_pattern` | string | Yes | Regex for busy state detection |
| `error_pattern` | string | Yes | Regex with capture group for error message |
| `rate_limit_pattern` | string | No | Regex for rate-limit detection |
| `capabilities` | []string | No | Capability strings this plugin supports |

Example:

```yaml
generic_plugins:
  my-tool:
    name: "My Tool"
    binary: "my-tool"
    args: ["--interactive"]
    ready_pattern: "^> $"
    busy_pattern: "\\.\\.\\.$"
    error_pattern: "Error: (.+)"
```

## Full Example

```yaml
project:
  name: my-project
  root: .
  state_dir: .crux

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

memory:
  sqlite_path: .crux/memory.db
  vector_dir: .crux/vectors
  embedding_provider: ollama
  embedding_model: nomic-embed-text

phases:
  spec_dir: docs/phases

security:
  audit_log: .crux/audit.log
  max_cmds_per_min: 60
  max_files_per_session: 100
  allowed_paths: ["."]
  denied_paths: [".crux/secrets.env", ".git/"]
```
