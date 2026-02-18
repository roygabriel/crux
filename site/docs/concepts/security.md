---
sidebar_position: 4
title: Security Model
---

# Security Model

Crux enforces a layered security model to constrain agent actions. Every agent runs within a permission tier, a filesystem sandbox, and under rate limits.

## Permission Tiers

Four tiers control what each agent can do:

| Tier | File Write | Shell Commands | Network | Git Push |
|------|-----------|----------------|---------|----------|
| `readonly` | No | No | No | No |
| `standard` | Scoped paths | Allowlisted | No | No |
| `elevated` | Project root | Most commands | Localhost | Feature branches |
| `autonomous` | Project root | All non-destructive | Yes | Feature branches |

Assign a tier in your agent configuration:

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
```

## Filesystem Sandbox

The sandbox restricts file operations to configured paths:

- **Allowed paths**: by default, the project root
- **Denied paths**: sensitive files like `.crux/secrets.env`, `.git/`
- Path validation uses `filepath.Rel` to prevent directory traversal

```yaml
security:
  allowed_paths: ["."]
  denied_paths: [".crux/secrets.env", ".git/"]
```

## Audit Logging

Every agent action is logged to `.crux/audit.log` as structured JSON:

- Commands executed
- Files modified
- Messages sent and received
- Gate results (pass/fail)
- Security violations (blocked actions)

View the audit log:

```bash
crux audit list --since 1h
crux audit stats
```

## Rate Limiting

Per-agent caps prevent runaway execution:

```yaml
security:
  max_cmds_per_min: 60
  max_files_per_session: 100
```

When an agent exceeds the limit, further commands are blocked until the window resets. The blocking event is recorded in the audit log.

## Git Safety

All agent commits land on feature branches (`crux/<agent-id>/<task>`). Merging to main requires human review, so no agent can directly modify the main branch.
