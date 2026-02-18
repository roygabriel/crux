---
sidebar_position: 100
title: Troubleshooting
description: Solutions for common Crux issues. Covers tmux not found, missing phase specs, unresponsive agents, rate limiting, SQLite database locks, and empty vector search results.
---

# Troubleshooting

Common issues and their solutions.

---

## "tmux not found" or "tmux: command not found"

**Symptom**: Crux fails to start with an error about tmux not being found.

**Cause**: tmux is not installed or not in your PATH.

**Fix**: Install tmux with your system package manager:

```bash
# Debian / Ubuntu
sudo apt install tmux

# macOS
brew install tmux

# Arch Linux
sudo pacman -S tmux

# Fedora
sudo dnf install tmux
```

Verify with `tmux -V`.

---

## "no phase specs found"

**Symptom**: Commands like `crux phase list` or `crux start` report no phase specs.

**Cause**: The project has not been initialized, or the `phases.spec_dir` config points to a directory without `PHASE*.md` files.

**Fix**: Run `crux init` to create the project structure, or verify that your `docs/phases/` directory contains phase spec files:

```bash
crux init
# or
crux init --example httpapi
```

Check your config:

```bash
grep spec_dir .crux/config.yaml
ls docs/phases/PHASE*.md
```

---

## "agent not responding" or agent appears stuck

**Symptom**: The orchestrator reports an agent as unresponsive, or the agent pane shows output but the orchestrator does not detect it.

**Cause**: The plugin's detection regex does not match the agent's actual output format. This often happens with updated CLI versions that change their prompt format.

**Fix**:

1. Attach to the agent's tmux session to see its actual output:
   ```bash
   tmux attach -t crux-engineer-1
   ```

2. Check what the agent is showing and compare with the plugin's detection patterns.

3. For custom plugins, update the regex patterns in your config:
   ```yaml
   generic_plugins:
     my-tool:
       ready_pattern: "^> $"  # Must match the actual prompt
   ```

---

## "rate limited" or commands being blocked

**Symptom**: Agent commands are blocked with rate limit messages in the audit log.

**Cause**: The agent is exceeding the configured commands-per-minute or files-per-session limit.

**Fix**: Increase the limits in `.crux/config.yaml`:

```yaml
security:
  max_cmds_per_min: 120      # default: 60
  max_files_per_session: 200  # default: 100
```

Check the audit log for details:

```bash
crux audit list --since 1h
```

---

## "database is locked" or SQLite errors

**Symptom**: Errors mentioning database lock or SQLite busy when running multiple commands.

**Cause**: Multiple processes attempting to write to the SQLite database simultaneously. SQLite uses WAL mode, but concurrent write contention can still occur.

**Fix**:

1. Make sure only one `crux start` instance is running at a time.
2. If the issue persists after stopping all instances, the lock file may be stale:
   ```bash
   ls -la .crux/memory.db*
   ```
3. Stop all crux processes and remove the WAL/SHM files (the data is safe):
   ```bash
   rm -f .crux/memory.db-wal .crux/memory.db-shm
   ```

---

## "vector search returns no results"

**Symptom**: `crux decisions search` returns empty results even though decisions have been recorded.

**Cause**: The embedding provider is not configured or not running. Without embeddings, vector search has no indexed data.

**Fix**:

1. Check your embedding config:
   ```yaml
   memory:
     embedding_provider: chromem-default  # or "ollama"
     embedding_model: nomic-embed-text
   ```

2. If using `ollama`, make sure the Ollama server is running and the model is pulled:
   ```bash
   ollama list
   ollama pull nomic-embed-text
   ```

3. The `chromem-default` provider requires no setup and works out of the box.

4. Try a full-text search instead (uses FTS5, always available):
   ```bash
   crux decisions search "your query"
   ```
