// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
)

// SystemService provides health status information.
type SystemService interface {
	Health() *HealthStatus
}

// HealthStatus is the response payload for GET /api/v1/health.
type HealthStatus struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// RouteRegistrar can mount its routes onto a Chi router.
// Each domain handler (memory, store, conversation, …) implements this interface
// and is passed to NewRouter for registration under /api/v1.
type RouteRegistrar interface {
	Register(r chi.Router)
}

// NewRouter builds and returns the Chi router with all middleware and routes wired up.
// jwtSecret and principalLookup are required for JWT authentication.
// Pass a nil jwtSecret to disable auth (development only).
// Additional domain handlers are registered via registrars.
func NewRouter(jwtSecret []byte, lookup apimw.PrincipalLookup, svc SystemService, registrars ...RouteRegistrar) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Authenticated API routes.
	r.Route("/api/v1", func(r chi.Router) {
		if jwtSecret != nil {
			r.Use(apimw.JWTMiddleware(jwtSecret, lookup))
		}
		r.Get("/health", healthHandler(svc))
		for _, reg := range registrars {
			reg.Register(r)
		}
	})

	return r
}

// healthHandler returns an http.HandlerFunc for GET /api/v1/health.
func healthHandler(svc SystemService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := svc.Health()
		writeJSON(w, http.StatusOK, status)
	}
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes a structured JSON error response.
// Used by REST handlers throughout the API.
func WriteError(w http.ResponseWriter, code int, errCode, message string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]string{
			"code":    errCode,
			"message": message,
		},
	})
}
