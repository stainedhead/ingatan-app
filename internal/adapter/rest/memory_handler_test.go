package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMemoryService is a test double for memoryuc.Service.
type mockMemoryService struct {
	saveFn   func(ctx context.Context, req memoryuc.SaveRequest) (*domain.Memory, error)
	getFn    func(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error)
	updateFn func(ctx context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error)
	deleteFn func(ctx context.Context, store, memoryID string, principal *domain.Principal) error
	listFn   func(ctx context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error)
}

func (m *mockMemoryService) Save(ctx context.Context, req memoryuc.SaveRequest) (*domain.Memory, error) {
	return m.saveFn(ctx, req)
}

func (m *mockMemoryService) Get(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error) {
	return m.getFn(ctx, store, memoryID, principal)
}

func (m *mockMemoryService) Update(ctx context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error) {
	return m.updateFn(ctx, req)
}

func (m *mockMemoryService) Delete(ctx context.Context, store, memoryID string, principal *domain.Principal) error {
	return m.deleteFn(ctx, store, memoryID, principal)
}

func (m *mockMemoryService) List(ctx context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
	return m.listFn(ctx, req)
}

func (m *mockMemoryService) Search(_ context.Context, _ memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMemoryService) Similar(_ context.Context, _ memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMemoryService) SaveURL(_ context.Context, _ memoryuc.SaveURLRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockMemoryService) SaveFile(_ context.Context, _ memoryuc.SaveFileRequest) (*domain.Memory, error) {
	return nil, nil
}

// testMemory returns a sample domain.Memory for use in tests.
func testMemory() *domain.Memory {
	return &domain.Memory{
		ID:        "mem-1",
		Store:     "default",
		Title:     "Test Memory",
		Content:   "Some content",
		Tags:      []string{"go", "test"},
		Source:    domain.MemorySourceManual,
		Metadata:  map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// memHandlerTestSecret is the HMAC secret used for JWT tokens in memory handler tests.
var memHandlerTestSecret = []byte("memory-handler-test-secret-32!!")

// memHandlerTestLookup resolves principals from JWT claims for memory handler tests.
func memHandlerTestLookup(_ context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	return &domain.Principal{
		ID:   claims.Subject,
		Name: claims.Name,
		Type: claims.Type,
		Role: claims.Role,
	}, nil
}

// memHandlerToken produces a signed JWT for the test principal.
func memHandlerToken() string {
	claims := apimw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Name: "Test User",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleUser,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(memHandlerTestSecret)
	return signed
}

// newTestRouter wires a MemoryHandler onto a fresh Chi router with JWT auth.
func newTestRouter(svc memoryuc.Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewMemoryHandler(svc)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(memHandlerTestSecret, memHandlerTestLookup))
		h.Register(r)
	})
	return r
}

// newReq builds an authenticated HTTP request with a valid JWT header.
func newReq(method, path string, body *bytes.Buffer) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+memHandlerToken())
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestSaveMemory_Success(t *testing.T) {
	mem := testMemory()
	svc := &mockMemoryService{
		saveFn: func(_ context.Context, req memoryuc.SaveRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "Some content", req.Content)
			return mem, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"title":   "Test Memory",
		"content": "Some content",
		"tags":    []string{"go", "test"},
	})
	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "mem-1", got.ID)
}

func TestSaveMemory_InvalidBody(t *testing.T) {
	svc := &mockMemoryService{}

	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories", bytes.NewBufferString("not-json")))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSaveMemory_MissingContent(t *testing.T) {
	svc := &mockMemoryService{}

	body, _ := json.Marshal(map[string]any{"title": "No content"})
	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores/default/memories", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetMemory_Success(t *testing.T) {
	mem := testMemory()
	svc := &mockMemoryService{
		getFn: func(_ context.Context, store, memoryID string, _ *domain.Principal) (*domain.Memory, error) {
			assert.Equal(t, "default", store)
			assert.Equal(t, "mem-1", memoryID)
			return mem, nil
		},
	}

	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores/default/memories/mem-1", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "mem-1", got.ID)
}

func TestGetMemory_NotFound(t *testing.T) {
	svc := &mockMemoryService{
		getFn: func(_ context.Context, _, _ string, _ *domain.Principal) (*domain.Memory, error) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "memory not found")
		},
	}

	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores/default/memories/missing", nil))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListMemories_Success(t *testing.T) {
	mems := []*domain.Memory{testMemory()}
	svc := &mockMemoryService{
		listFn: func(_ context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, 20, req.Limit)
			assert.Equal(t, 0, req.Offset)
			return &memoryuc.ListResponse{Memories: mems, Total: 1}, nil
		},
	}

	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores/default/memories", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["total"])
	memories, ok := resp["memories"].([]any)
	require.True(t, ok)
	assert.Len(t, memories, 1)
}

func TestUpdateMemory_Success(t *testing.T) {
	updated := testMemory()
	updated.Title = "Updated Title"

	svc := &mockMemoryService{
		updateFn: func(_ context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error) {
			require.NotNil(t, req.Title)
			assert.Equal(t, "Updated Title", *req.Title)
			assert.Nil(t, req.Content)
			return updated, nil
		},
	}

	body, _ := json.Marshal(map[string]any{"title": "Updated Title"})
	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPut, "/api/v1/stores/default/memories/mem-1", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusOK, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "Updated Title", got.Title)
}

func TestDeleteMemory_Success(t *testing.T) {
	svc := &mockMemoryService{
		deleteFn: func(_ context.Context, store, memoryID string, _ *domain.Principal) error {
			assert.Equal(t, "default", store)
			assert.Equal(t, "mem-1", memoryID)
			return nil
		},
	}

	rr := httptest.NewRecorder()
	newTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/stores/default/memories/mem-1", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp["deleted"])
}
