package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConversationService is a test double for conversationuc.Service.
type mockConversationService struct {
	startFn     func(ctx context.Context, req conversationuc.StartRequest) (*domain.Conversation, error)
	addMsgFn    func(ctx context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error)
	getFn       func(ctx context.Context, conversationID string, principal *domain.Principal) (*conversationuc.GetResponse, error)
	listFn      func(ctx context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error)
	summarizeFn func(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error)
	promoteFn   func(ctx context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error)
	deleteFn    func(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error
}

func (m *mockConversationService) Start(ctx context.Context, req conversationuc.StartRequest) (*domain.Conversation, error) {
	return m.startFn(ctx, req)
}

func (m *mockConversationService) AddMessage(ctx context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error) {
	return m.addMsgFn(ctx, req)
}

func (m *mockConversationService) Get(ctx context.Context, conversationID string, principal *domain.Principal) (*conversationuc.GetResponse, error) {
	return m.getFn(ctx, conversationID, principal)
}

func (m *mockConversationService) List(ctx context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error) {
	return m.listFn(ctx, req)
}

func (m *mockConversationService) Summarize(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error) {
	return m.summarizeFn(ctx, conversationID, principal)
}

func (m *mockConversationService) Promote(ctx context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error) {
	return m.promoteFn(ctx, req)
}

func (m *mockConversationService) Delete(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error {
	return m.deleteFn(ctx, conversationID, confirm, principal)
}

// testConversation returns a sample domain.Conversation for use in tests.
func testConversation() *domain.Conversation {
	return &domain.Conversation{
		ConversationID: "conv-1",
		Title:          "Test Conversation",
		Store:          "my-store",
		OwnerID:        "user-1",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		MessageCount:   0,
	}
}

// testMessage returns a sample domain.Message for use in tests.
func testMessage() *domain.Message {
	return &domain.Message{
		MessageID:      "msg-1",
		ConversationID: "conv-1",
		Role:           domain.MessageRoleUser,
		Content:        "Hello!",
		Metadata:       map[string]any{},
		CreatedAt:      time.Now(),
	}
}

// newConversationTestRouter wires a ConversationHandler onto a fresh Chi router with JWT auth.
func newConversationTestRouter(svc conversationuc.Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewConversationHandler(svc)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(memHandlerTestSecret, memHandlerTestLookup))
		h.Register(r)
	})
	return r
}

func TestConversationHandler_Start_Success(t *testing.T) {
	conv := testConversation()
	svc := &mockConversationService{
		startFn: func(_ context.Context, req conversationuc.StartRequest) (*domain.Conversation, error) {
			assert.Equal(t, "my-store", req.Store)
			assert.Equal(t, "Test Conversation", req.Title)
			return conv, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"store": "my-store",
		"title": "Test Conversation",
	})
	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/conversations", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Conversation
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "conv-1", got.ConversationID)
}

func TestConversationHandler_Start_MissingStore(t *testing.T) {
	svc := &mockConversationService{}

	body, _ := json.Marshal(map[string]any{"title": "No store"})
	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/conversations", bytes.NewBuffer(body)))

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestConversationHandler_AddMessage_Success(t *testing.T) {
	msg := testMessage()
	svc := &mockConversationService{
		addMsgFn: func(_ context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error) {
			assert.Equal(t, "conv-1", req.ConversationID)
			assert.Equal(t, domain.MessageRoleUser, req.Role)
			assert.Equal(t, "Hello!", req.Content)
			return msg, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": "Hello!",
	})
	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/conversations/conv-1/messages", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusCreated, rr.Code)
	var got domain.Message
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "msg-1", got.MessageID)
}

func TestConversationHandler_Get_Success(t *testing.T) {
	conv := testConversation()
	msgs := []*domain.Message{testMessage()}
	svc := &mockConversationService{
		getFn: func(_ context.Context, conversationID string, _ *domain.Principal) (*conversationuc.GetResponse, error) {
			assert.Equal(t, "conv-1", conversationID)
			return &conversationuc.GetResponse{Conversation: conv, Messages: msgs}, nil
		},
	}

	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/conversations/conv-1", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotNil(t, resp["conversation"])
	assert.NotNil(t, resp["messages"])
}

func TestConversationHandler_Get_NotFound(t *testing.T) {
	svc := &mockConversationService{
		getFn: func(_ context.Context, _ string, _ *domain.Principal) (*conversationuc.GetResponse, error) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "conversation not found")
		},
	}

	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/conversations/missing", nil))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestConversationHandler_List_Success(t *testing.T) {
	convs := []*domain.Conversation{testConversation()}
	svc := &mockConversationService{
		listFn: func(_ context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error) {
			assert.Equal(t, 20, req.Limit)
			assert.Equal(t, 0, req.Offset)
			return &conversationuc.ListResponse{Conversations: convs, Total: 1}, nil
		},
	}

	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodGet, "/api/v1/conversations", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	convList, ok := resp["conversations"].([]any)
	require.True(t, ok)
	assert.Len(t, convList, 1)
	assert.Equal(t, float64(1), resp["total"])
}

func TestConversationHandler_Delete_Success(t *testing.T) {
	svc := &mockConversationService{
		deleteFn: func(_ context.Context, conversationID, confirm string, _ *domain.Principal) error {
			assert.Equal(t, "conv-1", conversationID)
			assert.Equal(t, "conv-1", confirm)
			return nil
		},
	}

	body, _ := json.Marshal(map[string]any{"confirm": "conv-1"})
	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodDelete, "/api/v1/conversations/conv-1", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp["deleted"])
}

func TestConversationHandler_Promote_Success(t *testing.T) {
	mem := &domain.Memory{
		ID:    "mem-1",
		Store: "my-store",
		Title: "Promoted Conversation",
	}
	svc := &mockConversationService{
		promoteFn: func(_ context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error) {
			assert.Equal(t, "conv-1", req.ConversationID)
			assert.Equal(t, "my-store", req.Store)
			return mem, nil
		},
	}

	body, _ := json.Marshal(map[string]any{
		"store": "my-store",
		"title": "Promoted Conversation",
	})
	rr := httptest.NewRecorder()
	newConversationTestRouter(svc).ServeHTTP(rr, newReq(http.MethodPost, "/api/v1/conversations/conv-1/promote", bytes.NewBuffer(body)))

	require.Equal(t, http.StatusOK, rr.Code)
	var got domain.Memory
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "mem-1", got.ID)
}
