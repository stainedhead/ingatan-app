// Package rest provides the Chi HTTP server and REST API handlers for ingatan.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// SearchHandler handles HTTP requests for memory search operations.
type SearchHandler struct {
	svc memoryuc.Service
}

// NewSearchHandler creates a new SearchHandler with the given memory service.
func NewSearchHandler(svc memoryuc.Service) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Register mounts search routes on the given Chi router.
func (h *SearchHandler) Register(r chi.Router) {
	r.Post("/stores/{store}/memories/search", h.searchMemories)
	r.Get("/stores/{store}/memories/{memoryID}/similar", h.similarMemories)
}

// searchBody is the request body for POST /stores/{store}/memories/search.
type searchBody struct {
	Query string              `json:"query"`
	Mode  memoryuc.SearchMode `json:"mode"`
	TopK  int                 `json:"top_k"`
	Tags  []string            `json:"tags"`
}

func (h *SearchHandler) searchMemories(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	var body searchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Query == "" {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "query is required")
		return
	}

	mode := body.Mode
	if mode == "" {
		mode = memoryuc.SearchModeHybrid
	}
	topK := body.TopK
	if topK <= 0 {
		topK = 10
	}

	req := memoryuc.SearchRequest{
		Store:     store,
		Query:     body.Query,
		Mode:      mode,
		TopK:      topK,
		Tags:      body.Tags,
		Principal: principal,
	}

	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *SearchHandler) similarMemories(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	memoryID := chi.URLParam(r, "memoryID")
	principal := apimw.PrincipalFromContext(r.Context())

	topK := 10
	if topKStr := r.URL.Query().Get("top_k"); topKStr != "" {
		if v, err := strconv.Atoi(topKStr); err == nil && v > 0 {
			topK = v
		}
	}

	req := memoryuc.SimilarRequest{
		Store:     store,
		MemoryID:  memoryID,
		TopK:      topK,
		Principal: principal,
	}

	resp, err := h.svc.Similar(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleError maps domain errors to HTTP status codes.
func (h *SearchHandler) handleError(w http.ResponseWriter, err error) {
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
