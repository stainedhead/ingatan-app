// Package embed provides embedding provider adapters for ingatan.
package embed

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// compile-time interface check.
var _ memoryuc.Embedder = (*OpenAIEmbedder)(nil)

const defaultEmbeddingDimensions = 1536

// OpenAIEmbedder calls an OpenAI-compatible embeddings API to embed text.
type OpenAIEmbedder struct {
	client     *openai.Client
	model      string
	dimensions int
}

// NewOpenAIEmbedder creates an OpenAIEmbedder from the given EmbeddingConfig.
// If Dimensions is zero, it defaults to 1536.
// If BaseURL is non-empty, it overrides the default OpenAI API endpoint.
func NewOpenAIEmbedder(cfg config.EmbeddingConfig) *OpenAIEmbedder {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := openai.NewClient(opts...)

	dims := cfg.Dimensions
	if dims == 0 {
		dims = defaultEmbeddingDimensions
	}
	return &OpenAIEmbedder{client: client, model: cfg.Model, dimensions: dims}
}

// Embed converts a batch of texts into float32 embedding vectors using the
// configured OpenAI-compatible API. Returns nil, nil for an empty input slice.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.F[openai.EmbeddingNewParamsInputUnion](
			openai.EmbeddingNewParamsInputArrayOfStrings(texts),
		),
		Model: openai.F(openai.EmbeddingModel(e.model)),
	})
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.ErrCodeEmbeddingError,
			Message: fmt.Sprintf("embedding failed: %v", err),
		}
	}

	result := make([][]float32, len(resp.Data))
	for i, emb := range resp.Data {
		vec := make([]float32, len(emb.Embedding))
		for j, v := range emb.Embedding {
			vec[j] = float32(v)
		}
		result[i] = vec
	}
	return result, nil
}

// Dimensions returns the number of dimensions in each embedding vector.
func (e *OpenAIEmbedder) Dimensions() int { return e.dimensions }

// Model returns the embedding model name used by this embedder.
func (e *OpenAIEmbedder) Model() string { return e.model }
