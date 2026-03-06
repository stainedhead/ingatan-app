package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("test-secret-32-bytes-long-enough!")

func testLookup(_ context.Context, claims JWTClaims) (*domain.Principal, error) {
	return &domain.Principal{
		ID:   claims.Subject,
		Name: claims.Name,
		Type: claims.Type,
		Role: claims.Role,
	}, nil
}

func makeToken(secret []byte, sub, name string, ptype domain.PrincipalType, role domain.InstanceRole, exp time.Time) string {
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Name: name,
		Type: ptype,
		Role: role,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(secret)
	return signed
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	token := makeToken(testSecret, "user-1", "Alice", domain.PrincipalTypeHuman, domain.InstanceRoleUser, time.Now().Add(time.Hour))

	mw := JWTMiddleware(testSecret, testLookup)
	var gotPrincipal *domain.Principal
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrincipal = PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, gotPrincipal)
	assert.Equal(t, "user-1", gotPrincipal.ID)
	assert.Equal(t, "Alice", gotPrincipal.Name)
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	mw := JWTMiddleware(testSecret, testLookup)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_InvalidSignature(t *testing.T) {
	wrongSecret := []byte("wrong-secret-32-bytes-long-enough!")
	token := makeToken(wrongSecret, "user-1", "Alice", domain.PrincipalTypeHuman, domain.InstanceRoleUser, time.Now().Add(time.Hour))

	mw := JWTMiddleware(testSecret, testLookup)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	token := makeToken(testSecret, "user-1", "Alice", domain.PrincipalTypeHuman, domain.InstanceRoleUser, time.Now().Add(-time.Hour))

	mw := JWTMiddleware(testSecret, testLookup)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_MalformedHeader(t *testing.T) {
	mw := JWTMiddleware(testSecret, testLookup)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer sometoken")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPrincipalFromContext_NoPrincipal(t *testing.T) {
	assert.Nil(t, PrincipalFromContext(context.Background()))
}
