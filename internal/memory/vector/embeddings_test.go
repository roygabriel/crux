package vector_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/vector"
)

func TestNewEmbeddingFuncOllama(t *testing.T) {
	t.Parallel()
	cfg := config.MemoryConfig{
		EmbeddingProvider: "ollama",
		EmbeddingModel:    "nomic-embed-text",
	}
	ef, err := vector.NewEmbeddingFunc(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingFunc() error: %v", err)
	}
	if ef == nil {
		t.Fatal("expected non-nil EmbeddingFunc for ollama provider")
	}
}

func TestNewEmbeddingFuncChromemDefault(t *testing.T) {
	t.Parallel()
	cfg := config.MemoryConfig{
		EmbeddingProvider: "chromem-default",
	}
	ef, err := vector.NewEmbeddingFunc(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingFunc() error: %v", err)
	}
	if ef == nil {
		t.Fatal("expected non-nil EmbeddingFunc for chromem-default provider")
	}
}

func TestNewEmbeddingFuncEmpty(t *testing.T) {
	t.Parallel()
	cfg := config.MemoryConfig{
		EmbeddingProvider: "",
	}
	ef, err := vector.NewEmbeddingFunc(cfg)
	if err != nil {
		t.Fatalf("NewEmbeddingFunc() error: %v", err)
	}
	if ef == nil {
		t.Fatal("expected non-nil EmbeddingFunc for empty provider (defaults to chromem-default)")
	}
}

func TestNewEmbeddingFuncUnknown(t *testing.T) {
	t.Parallel()
	cfg := config.MemoryConfig{
		EmbeddingProvider: "nonexistent",
	}
	_, err := vector.NewEmbeddingFunc(cfg)
	if err == nil {
		t.Fatal("expected error for unknown embedding provider, got nil")
	}
}

func TestOllamaEmbedderReturnsFunc(t *testing.T) {
	t.Parallel()
	ef := vector.OllamaEmbedder("http://localhost:11434", "nomic-embed-text")
	if ef == nil {
		t.Fatal("expected non-nil EmbeddingFunc from OllamaEmbedder")
	}
}
