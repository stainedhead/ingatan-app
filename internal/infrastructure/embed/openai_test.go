package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
)

func mockEmbeddingServer(t *testing.T, embeddings [][]float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		data := make([]map[string]any, len(embeddings))
		for i, emb := range embeddings {
			data[i] = map[string]any{
				"object":    "embedding",
				"index":     i,
				"embedding": emb,
			}
		}
		resp := map[string]any{
			"object": "list",
			"data":   data,
			"model":  "text-embedding-3-small",
			"usage":  map[string]any{"prompt_tokens": 10, "total_tokens": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
}

func TestOpenAIEmbedder_Embed(t *testing.T) {
	rawVecs := [][]float64{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	srv := mockEmbeddingServer(t, rawVecs)
	defer srv.Close()

	cfg := config.EmbeddingConfig{
		APIKey:     "test-key",
		Model:      "text-embedding-3-small",
		Dimensions: 3,
		BaseURL:    srv.URL,
	}
	embedder := NewOpenAIEmbedder(cfg)

	vecs, err := embedder.Embed(context.Background(), []string{"hello", "world"})

	require.NoError(t, err)
	require.Len(t, vecs, 2)
	require.Len(t, vecs[0], 3)
	require.Len(t, vecs[1], 3)
	assert.InDelta(t, 0.1, float64(vecs[0][0]), 0.001)
	assert.InDelta(t, 0.2, float64(vecs[0][1]), 0.001)
	assert.InDelta(t, 0.3, float64(vecs[0][2]), 0.001)
	assert.InDelta(t, 0.4, float64(vecs[1][0]), 0.001)
	assert.InDelta(t, 0.5, float64(vecs[1][1]), 0.001)
	assert.InDelta(t, 0.6, float64(vecs[1][2]), 0.001)
}

func TestOpenAIEmbedder_EmptyInput(t *testing.T) {
	cfg := config.EmbeddingConfig{
		APIKey:     "key",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
	embedder := NewOpenAIEmbedder(cfg)

	vecs, err := embedder.Embed(context.Background(), nil)

	assert.NoError(t, err)
	assert.Nil(t, vecs)
}

func TestOpenAIEmbedder_EmptySlice(t *testing.T) {
	cfg := config.EmbeddingConfig{
		APIKey:     "key",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
	embedder := NewOpenAIEmbedder(cfg)

	vecs, err := embedder.Embed(context.Background(), []string{})

	assert.NoError(t, err)
	assert.Nil(t, vecs)
}

func TestOpenAIEmbedder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]any{
			"error": map[string]any{
				"message": "Incorrect API key provided",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode error response: %v", err)
		}
	}))
	defer srv.Close()

	cfg := config.EmbeddingConfig{
		APIKey:     "bad-key",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		BaseURL:    srv.URL,
	}
	embedder := NewOpenAIEmbedder(cfg)

	vecs, err := embedder.Embed(context.Background(), []string{"hello"})

	require.Error(t, err)
	assert.Nil(t, vecs)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeEmbeddingError, appErr.Code)
}

func TestOpenAIEmbedder_Dimensions(t *testing.T) {
	cfg := config.EmbeddingConfig{
		APIKey:     "key",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
	embedder := NewOpenAIEmbedder(cfg)
	assert.Equal(t, 1536, embedder.Dimensions())
}

func TestOpenAIEmbedder_DimensionsDefault(t *testing.T) {
	cfg := config.EmbeddingConfig{
		APIKey: "key",
		Model:  "text-embedding-3-small",
		// Dimensions is zero — should default to 1536
	}
	embedder := NewOpenAIEmbedder(cfg)
	assert.Equal(t, defaultEmbeddingDimensions, embedder.Dimensions())
}

func TestOpenAIEmbedder_Model(t *testing.T) {
	cfg := config.EmbeddingConfig{
		APIKey:     "key",
		Model:      "text-embedding-3-large",
		Dimensions: 3072,
	}
	embedder := NewOpenAIEmbedder(cfg)
	assert.Equal(t, "text-embedding-3-large", embedder.Model())
}
