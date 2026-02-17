package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	chromem "github.com/philippgille/chromem-go"

	"github.com/roygabriel/crux/internal/config"
)

// ollamaRequest is the JSON body sent to the Ollama /api/embed endpoint.
type ollamaRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// ollamaResponse is the JSON body returned from the Ollama /api/embed endpoint.
type ollamaResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// OllamaEmbedder returns an EmbeddingFunc that calls the Ollama /api/embed endpoint.
func OllamaEmbedder(baseURL, model string) chromem.EmbeddingFunc {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return func(ctx context.Context, text string) ([]float32, error) {
		reqBody := ollamaRequest{Model: model, Input: text}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshaling ollama request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/embed", strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, fmt.Errorf("creating ollama request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("calling ollama embed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("ollama embed returned %d: %s", resp.StatusCode, string(b))
		}

		var result ollamaResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decoding ollama response: %w", err)
		}

		if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
			return nil, fmt.Errorf("ollama returned empty embeddings")
		}

		return result.Embeddings[0], nil
	}
}

// NewEmbeddingFunc creates an EmbeddingFunc based on the memory configuration.
// Supported providers: "ollama", "chromem-default" (or empty for default).
func NewEmbeddingFunc(cfg config.MemoryConfig) (chromem.EmbeddingFunc, error) {
	switch cfg.EmbeddingProvider {
	case "ollama":
		return OllamaEmbedder("", cfg.EmbeddingModel), nil
	case "chromem-default", "":
		return chromem.NewEmbeddingFuncDefault(), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %q", cfg.EmbeddingProvider)
	}
}
