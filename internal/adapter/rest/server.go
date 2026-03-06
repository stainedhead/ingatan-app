// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"go.opentelemetry.io/otel/trace"
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

// ServerOptions holds optional middleware and provider configuration for the router.
// Zero value disables all optional features.
type ServerOptions struct {
	// OTelTracer instruments each request with an OTel trace span.
	// nil = noop (no tracing).
	OTelTracer trace.Tracer
	// RateLimitRPS is the token replenishment rate (tokens per second) for
	// per-IP rate limiting. 0 = disabled.
	RateLimitRPS float64
	// RateLimitBurst is the maximum burst size for the token bucket.
	RateLimitBurst int
	// AuthHandler registers unauthenticated auth routes (e.g. POST /auth/token).
	// nil = no auth routes registered.
	AuthHandler *AuthHandler
}

// NewRouter builds and returns the Chi router with all middleware and routes wired up.
// jwtSecret and principalLookup are required for JWT authentication.
// Pass a nil jwtSecret to disable auth (development only).
// opts configures optional OTel tracing and rate limiting (zero value = disabled).
// Additional domain handlers are registered via registrars.
func NewRouter(jwtSecret []byte, lookup apimw.PrincipalLookup, svc SystemService, opts ServerOptions, registrars ...RouteRegistrar) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// OTel tracing: applied at the outer level so all routes are instrumented.
	if opts.OTelTracer != nil {
		r.Use(apimw.OTelMiddleware(opts.OTelTracer))
	}

	// Rate limiting: applied at the outer level before authentication.
	if opts.RateLimitRPS > 0 {
		r.Use(apimw.RateLimitMiddleware(opts.RateLimitRPS, opts.RateLimitBurst))
	}

	// Unauthenticated auth routes (e.g. POST /auth/token).
	if opts.AuthHandler != nil {
		opts.AuthHandler.Register(r)
	}

	// Authenticated API routes.
	r.Route("/api/v1", func(r chi.Router) {
		if jwtSecret != nil {
			r.Use(apimw.JWTMiddleware(jwtSecret, lookup))
		}
		// Enrich the active span with principal.id after JWT resolves the principal.
		if opts.OTelTracer != nil {
			r.Use(apimw.PrincipalEnrichSpan)
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
