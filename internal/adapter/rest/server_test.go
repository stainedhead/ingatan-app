package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("test-secret-32-bytes-long-enough!")

type mockSystemService struct{}

func (m *mockSystemService) Health() *HealthStatus {
	return &HealthStatus{Status: "ok", Version: "1.0.0"}
}

func testLookup(_ context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	return &domain.Principal{
		ID:   claims.Subject,
		Name: claims.Name,
		Type: claims.Type,
		Role: claims.Role,
	}, nil
}

func validToken() string {
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
	signed, _ := tok.SignedString(testSecret)
	return signed
}

func TestHealth_WithValidJWT(t *testing.T) {
	r := NewRouter(testSecret, testLookup, &mockSystemService{}, ServerOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Authorization", "Bearer "+validToken())
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp HealthStatus
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp.Status)
}

func TestHealth_WithoutJWT(t *testing.T) {
	r := NewRouter(testSecret, testLookup, &mockSystemService{}, ServerOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHealth_NoAuth_DevMode(t *testing.T) {
	r := NewRouter(nil, nil, &mockSystemService{}, ServerOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
