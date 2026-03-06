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
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthTestRouter(svc *mockPrincipalService) *chi.Mux {
	r := chi.NewRouter()
	h := NewAuthHandler(svc, memHandlerTestSecret, 24*time.Hour)
	h.Register(r)
	return r
}

// newUnauthReq builds an unauthenticated HTTP request (no Authorization header).
func newUnauthReq(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestExchangeToken_ValidAPIKey_ReturnsJWT(t *testing.T) {
	p := &domain.Principal{
		ID:   "alice",
		Name: "Alice",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleUser,
	}
	svc := &mockPrincipalService{
		authenticateByKeyFn: func(_ context.Context, apiKey string) (*domain.Principal, error) {
			assert.Equal(t, "igt_validkey", apiKey)
			return p, nil
		},
	}

	rr := httptest.NewRecorder()
	newAuthTestRouter(svc).ServeHTTP(rr, newUnauthReq(http.MethodPost, "/auth/token", []byte(`{"api_key":"igt_validkey"}`)))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
	assert.NotEmpty(t, resp["expires_at"])
}

func TestExchangeToken_InvalidAPIKey_Returns401(t *testing.T) {
	svc := &mockPrincipalService{
		authenticateByKeyFn: func(_ context.Context, _ string) (*domain.Principal, error) {
			return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "invalid api key")
		},
	}

	rr := httptest.NewRecorder()
	newAuthTestRouter(svc).ServeHTTP(rr, newUnauthReq(http.MethodPost, "/auth/token", []byte(`{"api_key":"igt_badkey"}`)))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestExchangeToken_MissingAPIKey_Returns400(t *testing.T) {
	svc := &mockPrincipalService{}

	rr := httptest.NewRecorder()
	newAuthTestRouter(svc).ServeHTTP(rr, newUnauthReq(http.MethodPost, "/auth/token", []byte(`{}`)))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
