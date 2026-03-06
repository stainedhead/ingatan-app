package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// IngestHandler handles HTTP requests for URL and file ingestion.
type IngestHandler struct {
	svc memoryuc.Service
}

// NewIngestHandler creates a new IngestHandler.
func NewIngestHandler(svc memoryuc.Service) *IngestHandler {
	return &IngestHandler{svc: svc}
}

// Register mounts ingest routes on the given Chi router.
func (h *IngestHandler) Register(r chi.Router) {
	r.Post("/stores/{store}/memories/url", h.saveURL)
	r.Post("/stores/{store}/memories/file", h.saveFile)
}

// saveURLBody is the request body for POST /stores/{store}/memories/url.
type saveURLBody struct {
	URL      string         `json:"url"`
	Tags     []string       `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

// saveFileBody is the request body for POST /stores/{store}/memories/file.
type saveFileBody struct {
	FilePath string         `json:"file_path"`
	Tags     []string       `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

func (h *IngestHandler) saveURL(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	var body saveURLBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.URL == "" {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "url is required")
		return
	}

	mem, err := h.svc.SaveURL(r.Context(), memoryuc.SaveURLRequest{
		Store:     store,
		URL:       body.URL,
		Tags:      body.Tags,
		Metadata:  body.Metadata,
		Principal: principal,
	})
	if err != nil {
		handleIngestError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mem)
}

func (h *IngestHandler) saveFile(w http.ResponseWriter, r *http.Request) {
	store := chi.URLParam(r, "store")
	principal := apimw.PrincipalFromContext(r.Context())

	var body saveFileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.FilePath == "" {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "file_path is required")
		return
	}

	mem, err := h.svc.SaveFile(r.Context(), memoryuc.SaveFileRequest{
		Store:     store,
		FilePath:  body.FilePath,
		Tags:      body.Tags,
		Metadata:  body.Metadata,
		Principal: principal,
	})
	if err != nil {
		handleIngestError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mem)
}

// handleIngestError maps domain errors to HTTP status codes for ingest operations.
func handleIngestError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			WriteError(w, http.StatusNotFound, appErr.Code, appErr.Message)
		case domain.ErrCodeForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodeInvalidRequest:
			WriteError(w, http.StatusBadRequest, appErr.Code, appErr.Message)
		case domain.ErrCodeContentTooLarge:
			WriteError(w, http.StatusRequestEntityTooLarge, appErr.Code, appErr.Message)
		case domain.ErrCodePathNotAllowed:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodePDFExtractionError:
			WriteError(w, http.StatusUnprocessableEntity, appErr.Code, appErr.Message)
		default:
			WriteError(w, http.StatusInternalServerError, appErr.Code, appErr.Message)
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
}
