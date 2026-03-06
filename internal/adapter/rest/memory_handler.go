// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// MemoryHandler handles HTTP requests for the memory resource.
type MemoryHandler struct {
	svc memoryuc.Service
}

// NewMemoryHandler creates a new MemoryHandler with the given memory service.
func NewMemoryHandler(svc memoryuc.Service) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

// Register mounts memory routes on the given Chi router.
// Call from server.go with the /api/v1 subrouter.
func (h *MemoryHandler) Register(r chi.Router) {
	r.Post("/stores/{store}/memories", h.saveMemory)
	r.Get("/stores/{store}/memories", h.listMemories)
	r.Get("/stores/{store}/memories/{memoryID}", h.getMemory)
	r.Put("/stores/{store}/memories/{memoryID}", h.updateMemory)
	r.Delete("/stores/{store}/memories/{memoryID}", h.deleteMemory)
}

// saveMemoryBody is the request body for POST /stores/{store}/memories.
type saveMemoryBody struct {
	Title     string              `json:"title"`
	Content   string              `json:"content"`
	Tags      []string            `json:"tags"`
	Source    domain.MemorySource `json:"source"`
	SourceRef string              `json:"source_ref"`
	Metadata  map[string]any      `json:"metadata"`
}

// updateMemoryBody is the request body for PUT /stores/{store}/memories/{memoryID}.
// Fields absent in JSON remain nil (no-change semantics).
type updateMemoryBody struct {
	Title    *string        `json:"title"`
	Content  *string        `json:"content"`
	Tags     *[]string      `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

func (h *MemoryHandler) saveMemory(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	var body saveMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Content == "" {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "content is required")
		return
	}

	source := body.Source
	if source == "" {
		source = domain.MemorySourceManual
	}

	req := memoryuc.SaveRequest{
		Store:     store,
		Title:     body.Title,
		Content:   body.Content,
		Tags:      body.Tags,
		Source:    source,
		SourceRef: body.SourceRef,
		Metadata:  body.Metadata,
		Principal: principal,
	}

	mem, err := h.svc.Save(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mem)
}

func (h *MemoryHandler) listMemories(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	limit := 20
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var tags []string
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	var source *domain.MemorySource
	if srcStr := r.URL.Query().Get("source"); srcStr != "" {
		s := domain.MemorySource(srcStr)
		source = &s
	}

	req := memoryuc.ListRequest{
		Store:     store,
		Tags:      tags,
		Source:    source,
		Limit:     limit,
		Offset:    offset,
		Principal: principal,
	}

	resp, err := h.svc.List(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"memories": resp.Memories,
		"total":    resp.Total,
	})
}

func (h *MemoryHandler) getMemory(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	memoryID := chi.URLParam(r, "memoryID")
	principal := apimw.PrincipalFromContext(r.Context())

	mem, err := h.svc.Get(r.Context(), store, memoryID, principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mem)
}

func (h *MemoryHandler) updateMemory(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	memoryID := chi.URLParam(r, "memoryID")
	principal := apimw.PrincipalFromContext(r.Context())

	var body updateMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}

	req := memoryuc.UpdateRequest{
		Store:     store,
		MemoryID:  memoryID,
		Title:     body.Title,
		Content:   body.Content,
		Tags:      body.Tags,
		Metadata:  body.Metadata,
		Principal: principal,
	}

	mem, err := h.svc.Update(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mem)
}

func (h *MemoryHandler) deleteMemory(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	memoryID := chi.URLParam(r, "memoryID")
	principal := apimw.PrincipalFromContext(r.Context())

	if err := h.svc.Delete(r.Context(), store, memoryID, principal); err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// handleError maps domain errors to HTTP status codes and writes error responses.
func (h *MemoryHandler) handleError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			WriteError(w, http.StatusNotFound, appErr.Code, appErr.Message)
		case domain.ErrCodeForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodeInvalidRequest:
			WriteError(w, http.StatusBadRequest, appErr.Code, appErr.Message)
		default:
			WriteError(w, http.StatusInternalServerError, appErr.Code, appErr.Message)
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
}
