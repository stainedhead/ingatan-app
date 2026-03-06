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
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPrincipalService is a test double for principaluc.Service.
type mockPrincipalService struct {
	getOrCreateFn       func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error)
	whoAmIFn            func(ctx context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error)
	listFn              func(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error)
	createFn            func(ctx context.Context, caller *domain.Principal, req principaluc.CreateRequest) (*principaluc.CreateResponse, error)
	reissueAPIKeyFn     func(ctx context.Context, caller *domain.Principal, id string) (string, error)
	revokeAPIKeyFn      func(ctx context.Context, caller *domain.Principal, id string) error
	authenticateByKeyFn func(ctx context.Context, apiKey string) (*domain.Principal, error)
}

func (m *mockPrincipalService) GetOrCreate(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	return m.getOrCreateFn(ctx, claims)
}

func (m *mockPrincipalService) WhoAmI(ctx context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error) {
	return m.whoAmIFn(ctx, p)
}

func (m *mockPrincipalService) List(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error) {
	return m.listFn(ctx, caller)
}

func (m *mockPrincipalService) Create(ctx context.Context, caller *domain.Principal, req principaluc.CreateRequest) (*principaluc.CreateResponse, error) {
	return m.createFn(ctx, caller, req)
}

func (m *mockPrincipalService) ReissueAPIKey(ctx context.Context, caller *domain.Principal, id string) (string, error) {
	return m.reissueAPIKeyFn(ctx, caller, id)
}

func (m *mockPrincipalService) RevokeAPIKey(ctx context.Context, caller *domain.Principal, id string) error {
	return m.revokeAPIKeyFn(ctx, caller, id)
}

func (m *mockPrincipalService) AuthenticateByAPIKey(ctx context.Context, apiKey string) (*domain.Principal, error) {
	return m.authenticateByKeyFn(ctx, apiKey)
}

// testPrincipal returns a sample domain.Principal for use in tests.
func testPrincipal() *domain.Principal {
	return &domain.Principal{
		ID:         "user-1",
		Name:       "Test User",
		Type:       domain.PrincipalTypeHuman,
		Email:      "test@example.com",
		Role:       domain.InstanceRoleUser,
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}
}

// newPrincipalTestRouter wires a PrincipalHandler onto a fresh Chi router with JWT auth.
func newPrincipalTestRouter(svc principaluc.Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewPrincipalHandler(svc)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(memHandlerTestSecret, memHandlerTestLookup))
		h.Register(r)
	})
	return r
}

func TestWhoAmI_Success(t *testing.T) {
	p := testPrincipal()
	svc := &mockPrincipalService{
		whoAmIFn: func(_ context.Context, principal *domain.Principal) (*principaluc.WhoAmIResponse, error) {
			assert.Equal(t, "user-1", principal.ID)
			return &principaluc.WhoAmIResponse{
				Principal:        p,
				StoreMemberships: []principaluc.StoreMembership{},
				Capabilities:     []string{"memory:read", "memory:write"},
			}, nil
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/principals/me", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var got principaluc.WhoAmIResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.NotNil(t, got.Principal)
	assert.Equal(t, "user-1", got.Principal.ID)
}

func TestListPrincipals_AdminSuccess(t *testing.T) {
	principals := []*domain.Principal{testPrincipal()}
	svc := &mockPrincipalService{
		listFn: func(_ context.Context, caller *domain.Principal) ([]*domain.Principal, error) {
			assert.Equal(t, "user-1", caller.ID)
			return principals, nil
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/principals", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	pList, ok := resp["principals"].([]any)
	require.True(t, ok)
	assert.Len(t, pList, 1)
}

func TestListPrincipals_Forbidden(t *testing.T) {
	svc := &mockPrincipalService{
		listFn: func(_ context.Context, _ *domain.Principal) ([]*domain.Principal, error) {
			return nil, domain.NewAppError(domain.ErrCodeForbidden, "admin role required")
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/principals", nil))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCreatePrincipal_Success(t *testing.T) {
	p := testPrincipal()
	svc := &mockPrincipalService{
		createFn: func(_ context.Context, _ *domain.Principal, req principaluc.CreateRequest) (*principaluc.CreateResponse, error) {
			assert.Equal(t, "Bob", req.Name)
			return &principaluc.CreateResponse{Principal: p, APIKey: "igt_testkey"}, nil
		},
	}

	body := bytes.NewBufferString(`{"Name":"Bob","Type":"human","Role":"user"}`)
	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/principals", body))

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "igt_testkey", resp["api_key"])
}

func TestCreatePrincipal_Forbidden(t *testing.T) {
	svc := &mockPrincipalService{
		createFn: func(_ context.Context, _ *domain.Principal, _ principaluc.CreateRequest) (*principaluc.CreateResponse, error) {
			return nil, domain.NewAppError(domain.ErrCodeForbidden, "admin only")
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/principals", bytes.NewBufferString(`{"Name":"Bob"}`)))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestReissueAPIKey_Success(t *testing.T) {
	svc := &mockPrincipalService{
		reissueAPIKeyFn: func(_ context.Context, _ *domain.Principal, id string) (string, error) {
			assert.Equal(t, "bob", id)
			return "igt_newkey", nil
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/principals/bob/api-key", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "igt_newkey", resp["api_key"])
}

func TestRevokeAPIKey_Success(t *testing.T) {
	svc := &mockPrincipalService{
		revokeAPIKeyFn: func(_ context.Context, _ *domain.Principal, id string) error {
			assert.Equal(t, "bob", id)
			return nil
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/principals/bob/api-key", nil))

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestRevokeAPIKey_NotFound(t *testing.T) {
	svc := &mockPrincipalService{
		revokeAPIKeyFn: func(_ context.Context, _ *domain.Principal, _ string) error {
			return domain.NewAppError(domain.ErrCodeNotFound, "principal not found")
		},
	}

	rr := httptest.NewRecorder()
	newPrincipalTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/principals/nobody/api-key", nil))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
