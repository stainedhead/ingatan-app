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
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
)

// ConversationHandler handles HTTP requests for conversation management.
// Implements RouteRegistrar.
//
// Routes under /api/v1:
//
//	POST   /conversations                              → start conversation
//	GET    /conversations                              → list conversations
//	POST   /conversations/{conversationID}/messages   → add message
//	GET    /conversations/{conversationID}             → get conversation + messages
//	POST   /conversations/{conversationID}/summarize  → generate summary
//	POST   /conversations/{conversationID}/promote    → promote to memory
//	DELETE /conversations/{conversationID}             → delete conversation
type ConversationHandler struct {
	svc conversationuc.Service
}

// NewConversationHandler creates a new ConversationHandler with the given conversation service.
func NewConversationHandler(svc conversationuc.Service) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

// Register mounts conversation routes on the given Chi router.
func (h *ConversationHandler) Register(r chi.Router) {
	r.Post("/conversations", h.startConversation)
	r.Get("/conversations", h.listConversations)
	r.Post("/conversations/{conversationID}/messages", h.addMessage)
	r.Get("/conversations/{conversationID}", h.getConversation)
	r.Post("/conversations/{conversationID}/summarize", h.summarize)
	r.Post("/conversations/{conversationID}/promote", h.promote)
	r.Delete("/conversations/{conversationID}", h.deleteConversation)
}

// startConversationBody is the request body for POST /conversations.
type startConversationBody struct {
	Title string `json:"title"`
	Store string `json:"store"`
}

// addMessageBody is the request body for POST /conversations/{conversationID}/messages.
type addMessageBody struct {
	Role     string         `json:"role"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// promoteBody is the request body for POST /conversations/{conversationID}/promote.
type promoteBody struct {
	Store      string   `json:"store"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags"`
	UseSummary bool     `json:"use_summary"`
}

// deleteConversationBody is the request body for DELETE /conversations/{conversationID}.
type deleteConversationBody struct {
	Confirm string `json:"confirm"`
}

func (h *ConversationHandler) startConversation(w http.ResponseWriter, r *http.Request) {
	principal := apimw.PrincipalFromContext(r.Context())

	var body startConversationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Store == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "store is required")
		return
	}

	req := conversationuc.StartRequest{
		Title:     body.Title,
		Store:     body.Store,
		Principal: principal,
	}

	conv, err := h.svc.Start(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, conv)
}

func (h *ConversationHandler) addMessage(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationID")
	principal := apimw.PrincipalFromContext(r.Context())

	var body addMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Role == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "role is required")
		return
	}
	if body.Content == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "content is required")
		return
	}

	req := conversationuc.AddMessageRequest{
		ConversationID: conversationID,
		Role:           domain.MessageRole(body.Role),
		Content:        body.Content,
		Metadata:       body.Metadata,
		Principal:      principal,
	}

	msg, err := h.svc.AddMessage(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

func (h *ConversationHandler) getConversation(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationID")
	principal := apimw.PrincipalFromContext(r.Context())

	resp, err := h.svc.Get(r.Context(), conversationID, principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": resp.Conversation,
		"messages":     resp.Messages,
	})
}

func (h *ConversationHandler) listConversations(w http.ResponseWriter, r *http.Request) {
	principal := apimw.PrincipalFromContext(r.Context())

	store := r.URL.Query().Get("store")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	req := conversationuc.ListRequest{
		Store:     store,
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
		"conversations": resp.Conversations,
		"total":         resp.Total,
	})
}

func (h *ConversationHandler) summarize(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationID")
	principal := apimw.PrincipalFromContext(r.Context())

	summary, err := h.svc.Summarize(r.Context(), conversationID, principal)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *ConversationHandler) promote(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationID")
	principal := apimw.PrincipalFromContext(r.Context())

	var body promoteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Store == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "store is required")
		return
	}

	req := conversationuc.PromoteRequest{
		ConversationID: conversationID,
		Store:          body.Store,
		Title:          body.Title,
		Tags:           body.Tags,
		UseSummary:     body.UseSummary,
		Principal:      principal,
	}

	mem, err := h.svc.Promote(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mem)
}

func (h *ConversationHandler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationID")
	principal := apimw.PrincipalFromContext(r.Context())

	var body deleteConversationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, domain.ErrCodeInvalidRequest, "invalid request body")
		return
	}
	if body.Confirm == "" {
		WriteError(w, http.StatusUnprocessableEntity, domain.ErrCodeInvalidRequest, "confirm is required")
		return
	}

	if err := h.svc.Delete(r.Context(), conversationID, body.Confirm, principal); err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// handleError maps domain errors to HTTP status codes and writes error responses.
func (h *ConversationHandler) handleError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			WriteError(w, http.StatusNotFound, appErr.Code, appErr.Message)
		case domain.ErrCodeForbidden:
			WriteError(w, http.StatusForbidden, appErr.Code, appErr.Message)
		case domain.ErrCodeInvalidRequest:
			WriteError(w, http.StatusUnprocessableEntity, appErr.Code, appErr.Message)
		case domain.ErrCodeLLMError:
			WriteError(w, http.StatusServiceUnavailable, appErr.Code, appErr.Message)
		default:
			WriteError(w, http.StatusInternalServerError, appErr.Code, appErr.Message)
		}
		return
	}
	WriteError(w, http.StatusInternalServerError, domain.ErrCodeInternalError, "internal server error")
}
