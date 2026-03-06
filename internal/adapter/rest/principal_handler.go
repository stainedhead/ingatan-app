// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
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
//	GET    /principals/me           → whoami (current authenticated principal)
//	GET    /principals              → list all principals (admin only)
//	POST   /principals              → create principal and return API key (admin only)
//	POST   /principals/{id}/api-key → re-issue API key (admin only)
//	DELETE /principals/{id}/api-key → revoke API key (admin only)
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
	r.Post("/principals", h.createPrincipal)
	r.Post("/principals/{id}/api-key", h.reissueAPIKey)
	r.Delete("/principals/{id}/api-key", h.revokeAPIKey)
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

func (h *PrincipalHandler) createPrincipal(w http.ResponseWriter, r *http.Request) {
	caller := apimw.PrincipalFromContext(r.Context())

	var req principaluc.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Create(r.Context(), caller, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"principal": resp.Principal,
		"api_key":   resp.APIKey,
	})
}

func (h *PrincipalHandler) reissueAPIKey(w http.ResponseWriter, r *http.Request) {
	caller := apimw.PrincipalFromContext(r.Context())
	id := chi.URLParam(r, "id")

	apiKey, err := h.svc.ReissueAPIKey(r.Context(), caller, id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"api_key": apiKey})
}

func (h *PrincipalHandler) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	caller := apimw.PrincipalFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.RevokeAPIKey(r.Context(), caller, id); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
