// Package vector provides a persistent vector index for semantic similarity search,
// wrapping the chromem-go embedded vector database.
package vector

import (
	"context"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
)

// SearchResult holds a single match from a semantic similarity search.
type SearchResult struct {
	// ID is the document identifier.
	ID string
	// Content is the original text content.
	Content string
	// Similarity is the cosine similarity score in [-1, 1].
	Similarity float32
	// Metadata is the key-value metadata attached to the document.
	Metadata map[string]string
}

// VectorIndex wraps a chromem-go collection for semantic search operations.
type VectorIndex struct {
	collection *chromem.Collection
}

// NewVectorIndex creates or loads a persisted chromem-go vector index at the given
// directory. It uses a single "decisions" collection with the provided embedding function.
func NewVectorIndex(persistDir string, embeddingFunc chromem.EmbeddingFunc) (*VectorIndex, error) {
	db, err := chromem.NewPersistentDB(persistDir, false)
	if err != nil {
		return nil, fmt.Errorf("creating vector db: %w", err)
	}

	col, err := db.GetOrCreateCollection("decisions", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("creating decisions collection: %w", err)
	}

	return &VectorIndex{collection: col}, nil
}

// Add inserts a document into the vector index. The content is embedded automatically.
func (vi *VectorIndex) Add(ctx context.Context, id, content string, metadata map[string]string) error {
	doc := chromem.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}
	if err := vi.collection.AddDocument(ctx, doc); err != nil {
		return fmt.Errorf("adding document %s: %w", id, err)
	}
	return nil
}

// Search performs a semantic similarity search, returning the top n matches.
// Returns an empty slice if the collection has no documents or n <= 0.
func (vi *VectorIndex) Search(ctx context.Context, query string, n int, filter map[string]string) ([]SearchResult, error) {
	if n <= 0 || vi.collection.Count() == 0 {
		return nil, nil
	}

	// Clamp n to collection size to avoid chromem-go error.
	if n > vi.collection.Count() {
		n = vi.collection.Count()
	}

	results, err := vi.collection.Query(ctx, query, n, filter, nil)
	if err != nil {
		return nil, fmt.Errorf("searching vector index: %w", err)
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			ID:         r.ID,
			Content:    r.Content,
			Similarity: r.Similarity,
			Metadata:   r.Metadata,
		}
	}
	return out, nil
}

// Delete removes a document from the vector index by ID.
func (vi *VectorIndex) Delete(ctx context.Context, id string) error {
	if err := vi.collection.Delete(ctx, nil, nil, id); err != nil {
		return fmt.Errorf("deleting document %s: %w", id, err)
	}
	return nil
}

// Count returns the number of documents in the index.
func (vi *VectorIndex) Count() int {
	return vi.collection.Count()
}
