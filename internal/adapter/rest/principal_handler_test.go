package rest

import (
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
	getOrCreateFn func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error)
	whoAmIFn      func(ctx context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error)
	listFn        func(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error)
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
