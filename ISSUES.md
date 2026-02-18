# Known Issues

## 1. Init doc-building flow

Init should offer to launch an agent in a tmux pane to help populate memory bank
docs (projectBrief.md, systemPatterns.md, etc.) interactively. Currently writes
empty template stubs.

## 2. TUI layout sizing

Three-panel layout has sizing issues: no text wrapping (just truncation), border
math can go negative on small terminals, content panel tabs don't reflow.

## 3. TUI log panel startup leak

Log entries display as formatted `HH:MM:SS [LEVEL] (source) message` but slog
JSON output leaks through during startup before the LogBridge is installed as
default handler. Need to ensure LogBridge is set before any logging occurs, or
buffer early entries.
