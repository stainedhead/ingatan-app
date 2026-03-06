// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
)

// PrincipalHandler handles HTTP requests for principal management.
// Implements RouteRegistrar.
//
// Routes under /api/v1:
//
//	GET /principals/me  → whoami (current authenticated principal)
//	GET /principals     → list all principals (admin only — enforced by service)
type PrincipalHandler struct {
	svc principaluc.Service
}

// NewPrincipalHandler creates a new PrincipalHandler with the given principal service.
func NewPrincipalHandler(svc principaluc.Service) *PrincipalHandler {
	return &PrincipalHandler{svc: svc}
}

// Register mounts principal routes on the given Chi router.
func (h *PrincipalHandler) Register(r chi.Router) {
	r.Get("/principals/me", h.whoAmI)
	r.Get("/principals", h.listPrincipals)
}

func (h *PrincipalHandler) whoAmI(w http.ResponseWriter, r *http.Request) {
	principal := apimw.PrincipalFromContext(r.Context())

	resp, err := h.svc.WhoAmI(r.Context(), principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PrincipalHandler) listPrincipals(w http.ResponseWriter, r *http.Request) {
	caller := apimw.PrincipalFromContext(r.Context())

	principals, err := h.svc.List(r.Context(), caller)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"principals": principals})
}

// handleError maps domain errors to HTTP status codes and writes error responses.
func (h *PrincipalHandler) handleError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			WriteError(w, http.StatusNotFound, appErr.Code, appErr.Message)
		case domain.ErrCodeForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodeInvalidRequest:
			WriteError(w, http.StatusUnprocessableEntity, appErr.Code, appErr.Message)
		default:
			WriteError(w, http.StatusInternalServerError, appErr.Code, appErr.Message)
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
}
