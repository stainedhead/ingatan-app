package integration_test

import (
	"net/http"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_KeywordSearch(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Save two memories with distinct keywords.
	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Golang Tutorial",
		"content": "Go is a statically typed compiled language designed at Google. Goroutines enable concurrency.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Python Tutorial",
		"content": "Python is a dynamically typed interpreted language popular for data science and machine learning.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Search for "goroutines" — should match only the Go memory.
	resp = ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories/search", tok, map[string]any{
		"query": "goroutines concurrency",
		"mode":  "keyword",
		"top_k": 10,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	results, _ := result["Results"].([]any)
	require.NotEmpty(t, results, "keyword search should return results")

	// The first result should reference the Go memory via the nested Memory object.
	first, _ := results[0].(map[string]any)
	mem, _ := first["Memory"].(map[string]any)
	title := firstString(mem, "Title", "title")
	assert.Contains(t, title, "Golang", "first result should be the Go tutorial")
}

func TestSearch_HybridSearchWithoutEmbedder(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Hybrid Test",
		"content": "Unique keyword elephantine for hybrid search test verification.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Hybrid search without embedder — should fall back to keyword-only and not error.
	resp = ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories/search", tok, map[string]any{
		"query": "elephantine",
		"mode":  "hybrid",
		"top_k": 10,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	results, _ := result["Results"].([]any)
	assert.NotEmpty(t, results, "hybrid search should return keyword results even without embedder")
}

func TestSearch_NoResults(t *testing.T) {
	ts := newTestServer(t)
	tok := ts.token(t, "user-1", "User One", domain.InstanceRoleUser)

	// Save a memory so the store is initialized.
	resp := ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories", tok, map[string]any{
		"title":   "Something",
		"content": "Some content that exists.",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Search for a nonexistent keyword.
	resp = ts.do(t, http.MethodPost, "/api/v1/stores/user-1/memories/search", tok, map[string]any{
		"query": "xyzzy-nonexistent-keyword-12345",
		"mode":  "keyword",
		"top_k": 10,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	results, _ := result["Results"].([]any)
	assert.Empty(t, results, "search for nonexistent keyword should return empty results")
}
