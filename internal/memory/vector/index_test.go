package vector_test

import (
	"context"
	"crypto/sha256"
	"math"
	"path/filepath"
	"testing"

	chromem "github.com/philippgille/chromem-go"

	"github.com/roygabriel/crux/internal/memory/vector"
)

// testEmbedder returns a deterministic embedding function for testing.
// It hashes the input text to produce a normalized 32-dimensional vector,
// avoiding any external API calls.
func testEmbedder() chromem.EmbeddingFunc {
	return func(_ context.Context, text string) ([]float32, error) {
		h := sha256.Sum256([]byte(text))
		v := make([]float32, 32)
		var norm float64
		for i := range v {
			v[i] = float32(int8(h[i]))
			norm += float64(v[i]) * float64(v[i])
		}
		norm = math.Sqrt(norm)
		for i := range v {
			v[i] /= float32(norm)
		}
		return v, nil
	}
}

func newTestIndex(t *testing.T) *vector.VectorIndex {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "vectors")
	vi, err := vector.NewVectorIndex(dir, testEmbedder())
	if err != nil {
		t.Fatalf("NewVectorIndex() error: %v", err)
	}
	return vi
}

func TestNewVectorIndexCreatesDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "subdir", "vectors")
	vi, err := vector.NewVectorIndex(dir, testEmbedder())
	if err != nil {
		t.Fatalf("NewVectorIndex() error: %v", err)
	}
	if vi == nil {
		t.Fatal("expected non-nil VectorIndex")
	}
}

func TestAddAndSearch(t *testing.T) {
	t.Parallel()
	vi := newTestIndex(t)
	ctx := context.Background()

	err := vi.Add(ctx, "doc-1", "the quick brown fox jumps over the lazy dog", map[string]string{"source": "test"})
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	results, err := vi.Search(ctx, "quick fox", 1, nil)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "doc-1" {
		t.Errorf("result ID = %q, want %q", results[0].ID, "doc-1")
	}
	if results[0].Content != "the quick brown fox jumps over the lazy dog" {
		t.Errorf("result Content = %q, want original content", results[0].Content)
	}
	if results[0].Metadata["source"] != "test" {
		t.Errorf("result Metadata[source] = %q, want %q", results[0].Metadata["source"], "test")
	}
}

func TestSearchEmptyIndex(t *testing.T) {
	t.Parallel()
	vi := newTestIndex(t)
	ctx := context.Background()

	results, err := vi.Search(ctx, "anything", 5, nil)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty index, got %d", len(results))
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	vi := newTestIndex(t)
	ctx := context.Background()

	vi.Add(ctx, "doc-1", "first document", nil)
	vi.Add(ctx, "doc-2", "second document", nil)
	if vi.Count() != 2 {
		t.Fatalf("expected 2 docs, got %d", vi.Count())
	}

	if err := vi.Delete(ctx, "doc-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if vi.Count() != 1 {
		t.Errorf("expected 1 doc after delete, got %d", vi.Count())
	}
}

func TestCount(t *testing.T) {
	t.Parallel()
	vi := newTestIndex(t)
	ctx := context.Background()

	if vi.Count() != 0 {
		t.Errorf("expected 0 count on new index, got %d", vi.Count())
	}

	vi.Add(ctx, "doc-1", "hello world", nil)
	if vi.Count() != 1 {
		t.Errorf("expected 1 after add, got %d", vi.Count())
	}

	vi.Add(ctx, "doc-2", "goodbye world", nil)
	if vi.Count() != 2 {
		t.Errorf("expected 2 after second add, got %d", vi.Count())
	}
}

// testEmbedderDim is a helper used by the test to verify the deterministic embedder
// produces valid normalized vectors.
func TestDeterministicEmbedder(t *testing.T) {
	t.Parallel()
	ef := testEmbedder()
	ctx := context.Background()

	v1, err := ef(ctx, "hello")
	if err != nil {
		t.Fatalf("embedding error: %v", err)
	}
	v2, err := ef(ctx, "hello")
	if err != nil {
		t.Fatalf("embedding error: %v", err)
	}

	// Same input should produce same output.
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("embedder not deterministic at index %d: %f != %f", i, v1[i], v2[i])
		}
	}

	// Vector should be normalized (length ≈ 1).
	var norm float64
	for _, x := range v1 {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.001 {
		t.Errorf("expected unit vector, got norm %f", norm)
	}
}

