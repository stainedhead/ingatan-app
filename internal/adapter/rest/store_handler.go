// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

// StoreHandler handles HTTP requests for store management.
// Implements RouteRegistrar.
//
// Routes under /api/v1:
//
//	GET    /stores           → list stores
//	POST   /stores           → create store
//	GET    /stores/{store}   → get store
//	DELETE /stores/{store}   → delete store (body: {"confirm": "<store-name>"})
type StoreHandler struct {
	svc storeuc.Service
}

// NewStoreHandler creates a new StoreHandler with the given store service.
func NewStoreHandler(svc storeuc.Service) *StoreHandler {
	return &StoreHandler{svc: svc}
}

// Register mounts store routes on the given Chi router.
func (h *StoreHandler) Register(r chi.Router) {
	r.Get("/stores", h.listStores)
	r.Post("/stores", h.createStore)
	r.Get("/stores/{store}", h.getStore)
	r.Delete("/stores/{store}", h.deleteStore)
}

// createStoreBody is the request body for POST /stores.
type createStoreBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// deleteStoreBody is the request body for DELETE /stores/{store}.
type deleteStoreBody struct {
	Confirm string `json:"confirm"`
}

func (h *StoreHandler) createStore(w http.ResponseWriter, r *http.Request) {
	principal := apimw.PrincipalFromContext(r.Context())

	var body createStoreBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "name is required")
		return
	}

	req := storeuc.CreateRequest{
		Name:        body.Name,
		Description: body.Description,
		Principal:   principal,
	}

	store, err := h.svc.Create(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, store)
}

func (h *StoreHandler) listStores(w http.ResponseWriter, r *http.Request) {
	principal := apimw.PrincipalFromContext(r.Context())

	stores, err := h.svc.List(r.Context(), principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"stores": stores})
}

func (h *StoreHandler) getStore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	store, err := h.svc.Get(r.Context(), name, principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, store)
}

func (h *StoreHandler) deleteStore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	var body deleteStoreBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Confirm == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "confirm is required")
		return
	}

	if err := h.svc.Delete(r.Context(), name, body.Confirm, principal); err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// handleError maps domain errors to HTTP status codes and writes error responses.
func (h *StoreHandler) handleError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			WriteError(w, http.StatusNotFound, appErr.Code, appErr.Message)
		case domain.ErrCodeForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodeConflict:
			WriteError(w, http.StatusConflict, appErr.Code, appErr.Message)
		case domain.ErrCodeInvalidRequest:
			WriteError(w, http.StatusUnprocessableEntity, appErr.Code, appErr.Message)
		case domain.ErrCodeStoreDeleteForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		default:
			WriteError(w, http.StatusInternalServerError, appErr.Code, appErr.Message)
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
}
