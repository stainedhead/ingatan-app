// Package middleware provides HTTP middleware for the ingatan REST API.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stainedhead/ingatan/internal/domain"
)

type contextKey string

// principalKey is the context key for the authenticated principal.
const principalKey contextKey = "principal"

// JWTClaims holds the expected JWT claims for ingatan principals.
type JWTClaims struct {
	jwt.RegisteredClaims
	Name string               `json:"name"`
	Type domain.PrincipalType `json:"type"`
	Role domain.InstanceRole  `json:"role"`
}

// PrincipalLookup is a function that resolves or creates a Principal from JWT claims.
// Implementations should look up by claims.Subject and create on first sight.
type PrincipalLookup func(ctx context.Context, claims JWTClaims) (*domain.Principal, error)

// JWTMiddleware returns an HTTP middleware that validates Bearer JWT tokens.
// On success it injects the resolved *domain.Principal into the request context.
// On failure it responds with 401 UNAUTHORIZED.
func JWTMiddleware(secret []byte, lookup PrincipalLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}

			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				writeUnauthorized(w, "invalid or expired token")
				return
			}

			if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
				writeUnauthorized(w, "token expired")
				return
			}

			principal, err := lookup(r.Context(), *claims)
			if err != nil {
				writeUnauthorized(w, "principal lookup failed")
				return
			}

			ctx := context.WithValue(r.Context(), principalKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// PrincipalFromContext extracts the authenticated principal from the request context.
// Returns nil if no principal is present.
func PrincipalFromContext(ctx context.Context) *domain.Principal {
	p, _ := ctx.Value(principalKey).(*domain.Principal)
	return p
}

// bearerToken extracts the token string from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

// writeUnauthorized writes a 401 JSON error response.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    domain.ErrCodeUnauthorized,
			"message": message,
		},
	})
}
