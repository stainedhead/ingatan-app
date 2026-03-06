// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
)

// AuthHandler handles unauthenticated authentication routes.
// Implements RouteRegistrar; routes are mounted outside the JWT middleware group.
//
// Routes:
//
//	POST /auth/token  → exchange an API key for a short-lived JWT
type AuthHandler struct {
	svc      principaluc.Service
	secret   []byte
	tokenTTL time.Duration
}

// NewAuthHandler creates an AuthHandler.
// secret must be the same HS256 secret used by JWTMiddleware.
// tokenTTL is the lifetime of issued tokens (e.g. 24h).
func NewAuthHandler(svc principaluc.Service, secret []byte, tokenTTL time.Duration) *AuthHandler {
	return &AuthHandler{svc: svc, secret: secret, tokenTTL: tokenTTL}
}

// Register mounts auth routes on the given router.
// These routes must be added outside the JWTMiddleware group.
func (h *AuthHandler) Register(r chi.Router) {
	r.Post("/auth/token", h.exchangeToken)
}

// exchangeToken validates an API key and returns a signed JWT.
func (h *AuthHandler) exchangeToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.APIKey == "" {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "api_key is required")
		return
	}

	p, err := h.svc.AuthenticateByAPIKey(r.Context(), req.APIKey)
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeUnauthorized {
			WriteError(w, http.StatusUnauthorized, domain.ErrCodeUnauthorized, "invalid api key")
			return
		}
		WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
		return
	}

	now := time.Now().UTC()
	exp := now.Add(h.tokenTTL)
	claims := apimw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "ingatan",
		},
		Name: p.Name,
		Type: p.Type,
		Role: p.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(h.secret)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      tokenStr,
		"expires_at": exp.Format(time.RFC3339),
	})
}
