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
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreService is a test double for storeuc.Service.
type mockStoreService struct {
	createFn func(ctx context.Context, req storeuc.CreateRequest) (*domain.Store, error)
	getFn    func(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error)
	listFn   func(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error)
	deleteFn func(ctx context.Context, name, confirm string, principal *domain.Principal) error
}

func (m *mockStoreService) Create(ctx context.Context, req storeuc.CreateRequest) (*domain.Store, error) {
	return m.createFn(ctx, req)
}

func (m *mockStoreService) Get(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error) {
	return m.getFn(ctx, name, principal)
}

func (m *mockStoreService) List(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error) {
	return m.listFn(ctx, principal)
}

func (m *mockStoreService) Delete(ctx context.Context, name, confirm string, principal *domain.Principal) error {
	return m.deleteFn(ctx, name, confirm, principal)
}

// testStore returns a sample domain.Store for use in tests.
func testStore() *domain.Store {
	return &domain.Store{
		Name:        "my-store",
		Description: "A test store",
		OwnerID:     "user-1",
		CreatedAt:   time.Now(),
		Members:     []domain.StoreMember{{PrincipalID: "user-1", Role: domain.StoreRoleOwner}},
	}
}

// newStoreTestRouter wires a StoreHandler onto a fresh Chi router with JWT auth.
func newStoreTestRouter(svc storeuc.Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewStoreHandler(svc)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(memHandlerTestSecret, memHandlerTestLookup))
		h.Register(r)
	})
	return r
}

func TestCreateStore_Success(t *testing.T) {
	s := testStore()
	svc := &mockStoreService{
		createFn: func(_ context.Context, req storeuc.CreateRequest) (*domain.Store, error) {
			assert.Equal(t, "my-store", req.Name)
			assert.Equal(t, "A test store", req.Description)
			return s, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"name":        "my-store",
		"description": "A test store",
	})
	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Store
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "my-store", got.Name)
}

func TestCreateStore_MissingName(t *testing.T) {
	svc := &mockStoreService{}

	body, _ := json.Marshal(map[string]any{"description": "No name"})
	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestCreateStore_Conflict(t *testing.T) {
	svc := &mockStoreService{
		createFn: func(_ context.Context, _ storeuc.CreateRequest) (*domain.Store, error) {
			return nil, domain.NewAppError(domain.ErrCodeConflict, "store already exists")
		},
	}

	body, _ := json.Marshal(map[string]any{"name": "existing-store"})
	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/stores", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestListStores_Success(t *testing.T) {
	stores := []*domain.Store{testStore()}
	svc := &mockStoreService{
		listFn: func(_ context.Context, _ *domain.Principal) ([]*domain.Store, error) {
			return stores, nil
		},
	}

	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	storeList, ok := resp["stores"].([]any)
	require.True(t, ok)
	assert.Len(t, storeList, 1)
}

func TestGetStore_Found(t *testing.T) {
	s := testStore()
	svc := &mockStoreService{
		getFn: func(_ context.Context, name string, _ *domain.Principal) (*domain.Store, error) {
			assert.Equal(t, "my-store", name)
			return s, nil
		},
	}

	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores/my-store", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var got domain.Store
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "my-store", got.Name)
}

func TestGetStore_NotFound(t *testing.T) {
	svc := &mockStoreService{
		getFn: func(_ context.Context, _ string, _ *domain.Principal) (*domain.Store, error) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "store not found")
		},
	}

	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/stores/missing", nil))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteStore_Success(t *testing.T) {
	svc := &mockStoreService{
		deleteFn: func(_ context.Context, name, confirm string, _ *domain.Principal) error {
			assert.Equal(t, "my-store", name)
			assert.Equal(t, "my-store", confirm)
			return nil
		},
	}

	body, _ := json.Marshal(map[string]any{"confirm": "my-store"})
	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/stores/my-store", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp["deleted"])
}

func TestDeleteStore_Forbidden(t *testing.T) {
	svc := &mockStoreService{
		deleteFn: func(_ context.Context, _, _ string, _ *domain.Principal) error {
			return domain.NewAppError(domain.ErrCodeStoreDeleteForbidden, "cannot delete personal store")
		},
	}

	body, _ := json.Marshal(map[string]any{"confirm": "my-store"})
	rr := httptest.NewRecorder()
	newStoreTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/stores/my-store", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}
