package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIngestService is a test double for memoryuc.Service used by ingest handler tests.
type mockIngestService struct {
	saveURLFn  func(ctx context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error)
	saveFileFn func(ctx context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error)
}

func (m *mockIngestService) Save(_ context.Context, _ memoryuc.SaveRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockIngestService) Get(_ context.Context, _, _ string, _ *domain.Principal) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockIngestService) Update(_ context.Context, _ memoryuc.UpdateRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockIngestService) Delete(_ context.Context, _, _ string, _ *domain.Principal) error {
	return nil
}

func (m *mockIngestService) List(_ context.Context, _ memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
	return &memoryuc.ListResponse{}, nil
}

func (m *mockIngestService) Search(_ context.Context, _ memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockIngestService) Similar(_ context.Context, _ memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockIngestService) SaveURL(ctx context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error) {
	return m.saveURLFn(ctx, req)
}

func (m *mockIngestService) SaveFile(ctx context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error) {
	return m.saveFileFn(ctx, req)
}

// newIngestTestRouter wires an IngestHandler onto a fresh Chi router with JWT auth.
func newIngestTestRouter(svc memoryuc.Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewIngestHandler(svc)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(memHandlerTestSecret, memHandlerTestLookup))
		h.Register(r)
	})
	return r
}

func TestIngestHandler_SaveURL_Success(t *testing.T) {
	mem := testMemory()
	mem.Source = domain.MemorySourceURL
	mem.SourceURL = "https://example.com/page"

	svc := &mockIngestService{
		saveURLFn: func(_ context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "https://example.com/page", req.URL)
			return mem, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"url":  "https://example.com/page",
		"tags": []string{"web"},
	})
	rr := httptest.NewRecorder()
	newIngestTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories/url", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "mem-1", got.ID)
}

func TestIngestHandler_SaveURL_MissingURL(t *testing.T) {
	svc := &mockIngestService{}

	body, _ := json.Marshal(map[string]any{})
	rr := httptest.NewRecorder()
	newIngestTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories/url", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestIngestHandler_SaveFile_Success(t *testing.T) {
	mem := testMemory()
	mem.Source = domain.MemorySourceFile
	mem.SourcePath = "/safe/docs/readme.txt"

	svc := &mockIngestService{
		saveFileFn: func(_ context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "/safe/docs/readme.txt", req.FilePath)
			return mem, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"file_path": "/safe/docs/readme.txt",
		"tags":      []string{"doc"},
	})
	rr := httptest.NewRecorder()
	newIngestTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories/file", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "mem-1", got.ID)
}

func TestIngestHandler_SaveFile_PathNotAllowed(t *testing.T) {
	svc := &mockIngestService{
		saveFileFn: func(_ context.Context, _ memoryuc.SaveFileRequest) (*domain.Memory, error) {
			return nil, domain.NewAppError(domain.ErrCodePathNotAllowed, "file path is not in an allowed directory")
		},
	}

	body, _ := json.Marshal(map[string]any{
		"file_path": "/etc/passwd",
	})
	rr := httptest.NewRecorder()
	newIngestTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories/file", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestIngestHandler_SaveURL_ContentTooLarge(t *testing.T) {
	svc := &mockIngestService{
		saveURLFn: func(_ context.Context, _ memoryuc.SaveURLRequest) (*domain.Memory, error) {
			return nil, domain.NewAppError(domain.ErrCodeContentTooLarge, "content exceeds maximum size")
		},
	}

	body, _ := json.Marshal(map[string]any{
		"url": "https://example.com/huge",
	})
	rr := httptest.NewRecorder()
	newIngestTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories/url", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}
