package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchMockService extends mockMemoryService with controllable Search/Similar fns.
type searchMockService struct {
	mockMemoryService
	searchFn  func(ctx context.Context, req memoryuc.SearchRequest) (*memoryuc.SearchResponse, error)
	similarFn func(ctx context.Context, req memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error)
}

func (m *searchMockService) Search(ctx context.Context, req memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, req)
	}
	return &memoryuc.SearchResponse{}, nil
}

func (m *searchMockService) Similar(ctx context.Context, req memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
	if m.similarFn != nil {
		return m.similarFn(ctx, req)
	}
	return &memoryuc.SearchResponse{}, nil
}

func newSearchTestRouter(svc memoryuc.Service) http.Handler {
	return NewRouter(testSecret, testLookup, &mockSystemService{}, ServerOptions{}, NewSearchHandler(svc))
}

func TestSearchHandler_Search_OK(t *testing.T) {
	mem := testMemory()
	svc := &searchMockService{
		searchFn: func(_ context.Context, req memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "golang", req.Query)
			assert.Equal(t, memoryuc.SearchModeHybrid, req.Mode)
			return &memoryuc.SearchResponse{
				Results: []memoryuc.SearchResult{{Memory: mem, Score: 0.9}},
			}, nil
		},
	}

	body, _ := json.Marshal(map[string]any{"query": "golang"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/default/memories/search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validToken())
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp memoryuc.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, mem.ID, resp.Results[0].Memory.ID)
}

func TestSearchHandler_Search_MissingQuery(t *testing.T) {
	svc := &searchMockService{}

	body, _ := json.Marshal(map[string]any{"mode": "keyword"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/default/memories/search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validToken())
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSearchHandler_Search_ServiceError(t *testing.T) {
	svc := &searchMockService{
		searchFn: func(_ context.Context, _ memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "bad request")
		},
	}

	body, _ := json.Marshal(map[string]any{"query": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/default/memories/search", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validToken())
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSearchHandler_Similar_OK(t *testing.T) {
	mem := testMemory()
	svc := &searchMockService{
		similarFn: func(_ context.Context, req memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "mem-1", req.MemoryID)
			return &memoryuc.SearchResponse{
				Results: []memoryuc.SearchResult{{Memory: mem, Score: 0.8}},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/default/memories/mem-1/similar", nil)
	req.Header.Set("Authorization", "Bearer "+validToken())

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp memoryuc.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Results, 1)
}

func TestSearchHandler_Similar_NotFound(t *testing.T) {
	svc := &searchMockService{
		similarFn: func(_ context.Context, _ memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "memory not found")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores/default/memories/missing/similar", nil)
	req.Header.Set("Authorization", "Bearer "+validToken())

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSearchHandler_RequiresAuth(t *testing.T) {
	svc := &searchMockService{}

	body, _ := json.Marshal(map[string]any{"query": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stores/default/memories/search", bytes.NewReader(body))
	// No Authorization header

	rr := httptest.NewRecorder()
	newSearchTestRouter(svc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
