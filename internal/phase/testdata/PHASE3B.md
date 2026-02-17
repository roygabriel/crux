# Phase 3B: Vector Search, Embeddings, Work Notes Manager

## Status
Planned

## Depends On
Phase 3A

## Design Rationale
Add semantic search via chromem-go for the decision journal. Build the embedding pipeline (Ollama + hugot fallback). Create the automated work notes manager that generates and updates per-phase work notes in the exact format from the LLM Development Guide Chapter 4.

## Exit Criteria
- [ ] `go build ./...` exits 0 (no CGO)
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] Vector search returns semantically relevant decisions
- [ ] Work notes render matching the Chapter 4 template
- [ ] Embedding pipeline works with Ollama or falls back to hugot

## Progress Notes
