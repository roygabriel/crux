---
title: "First Project"
weight: 2
---

# Your First Project

This walkthrough creates a project using the built-in HTTP API example, then starts the orchestrator.

## Initialize with an Example

```bash
mkdir my-project && cd my-project
crux init --example httpapi
```

This creates:

```
my-project/
├── .crux/
│   └── config.yaml          # Orchestrator configuration
├── docs/
│   └── phases/
│       ├── PHASE1A.md        # Phase spec: project skeleton
│       ├── PHASE1A-PROMPT.md # Prompt contract
│       ├── PHASE1B.md        # Phase spec: HTTP handlers
│       └── PHASE1B-PROMPT.md # Prompt contract
└── work-notes/               # Auto-generated per-phase notes
```

## Configure Agents

Edit `.crux/config.yaml` to define your agents:

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

memory:
  sqlite_path: .crux/memory.db
  vector_dir: .crux/vectors

phases:
  spec_dir: docs/phases

security:
  audit_log: .crux/audit.log
  max_cmds_per_min: 60
  max_files_per_session: 100
```

## Inspect Phases

```bash
# List all phases
crux phase list

# Show details for phase 1A
crux phase show 1A

# Validate all specs
crux phase validate
```

## Start Orchestration

```bash
crux start
```

Crux launches tmux sessions for each agent, assigns prompts from your phase docs, and begins the execution loop:

1. Poll agent panes for output
2. Parse completion markers through plugin adapters
3. Run verification gates (`go build`, `go test`, `go vet`)
4. Record decisions in the journal
5. Advance to the next prompt

## Monitor Progress

In another terminal:

```bash
# Live status
crux status

# View work notes
crux notes show 1A

# Search past decisions
crux decisions search "architecture choice"

# View audit log
crux audit list --since 1h
```

## Create Your Own Phases

```bash
crux phase create --id 2A --name "Database Layer" --depends-on 1A --prompts 3
```

This generates `PHASE2A.md` (spec) and `PHASE2A-PROMPT.md` (prompt contract) in your phases directory. Edit them to define scope, tasks, files, and exit criteria.
