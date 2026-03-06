package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPConvService is a test double for conversationuc.Service used by MCP tests.
type mockMCPConvService struct {
	startFn     func(ctx context.Context, req conversationuc.StartRequest) (*domain.Conversation, error)
	addMsgFn    func(ctx context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error)
	getFn       func(ctx context.Context, conversationID string, principal *domain.Principal) (*conversationuc.GetResponse, error)
	listFn      func(ctx context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error)
	summarizeFn func(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error)
	promoteFn   func(ctx context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error)
	deleteFn    func(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error
}

func (m *mockMCPConvService) Start(ctx context.Context, req conversationuc.StartRequest) (*domain.Conversation, error) {
	return m.startFn(ctx, req)
}

func (m *mockMCPConvService) AddMessage(ctx context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error) {
	return m.addMsgFn(ctx, req)
}

func (m *mockMCPConvService) Get(ctx context.Context, conversationID string, principal *domain.Principal) (*conversationuc.GetResponse, error) {
	return m.getFn(ctx, conversationID, principal)
}

func (m *mockMCPConvService) List(ctx context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error) {
	return m.listFn(ctx, req)
}

func (m *mockMCPConvService) Summarize(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error) {
	return m.summarizeFn(ctx, conversationID, principal)
}

func (m *mockMCPConvService) Promote(ctx context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error) {
	return m.promoteFn(ctx, req)
}

func (m *mockMCPConvService) Delete(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error {
	return m.deleteFn(ctx, conversationID, confirm, principal)
}

// sampleConversation returns a reusable test domain.Conversation.
func sampleConversation() *domain.Conversation {
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

// sampleMessage returns a reusable test domain.Message.
func sampleMessage() *domain.Message {
	return &domain.Message{
		MessageID:      "msg-1",
		ConversationID: "conv-1",
		Role:           domain.MessageRoleUser,
		Content:        "Hello!",
		Metadata:       map[string]any{},
		CreatedAt:      time.Now(),
	}
}

func TestConversationTools_Start_Success(t *testing.T) {
	conv := sampleConversation()
	svc := &mockMCPConvService{
		startFn: func(_ context.Context, req conversationuc.StartRequest) (*domain.Conversation, error) {
			assert.Equal(t, "my-store", req.Store)
			assert.Equal(t, "Test Conversation", req.Title)
			return conv, nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_start", map[string]any{
		"store": "my-store",
		"title": "Test Conversation",
	})

	result, err := tools.handleStart(context.Background(), req)
	require.NoError(t, err)

	var got domain.Conversation
	decodeTextResult(t, result, &got)
	assert.Equal(t, "conv-1", got.ConversationID)
}

func TestConversationTools_Start_MissingStore(t *testing.T) {
	svc := &mockMCPConvService{}
	tools := NewConversationTools(svc)

	req := callRequest("conversation_start", map[string]any{"title": "No store"})
	_, err := tools.handleStart(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store is required")
}

func TestConversationTools_AddMessage_Success(t *testing.T) {
	msg := sampleMessage()
	svc := &mockMCPConvService{
		addMsgFn: func(_ context.Context, req conversationuc.AddMessageRequest) (*domain.Message, error) {
			assert.Equal(t, "conv-1", req.ConversationID)
			assert.Equal(t, domain.MessageRoleUser, req.Role)
			assert.Equal(t, "Hello!", req.Content)
			return msg, nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_add_message", map[string]any{
		"conversation_id": "conv-1",
		"role":            "user",
		"content":         "Hello!",
	})

	result, err := tools.handleAddMessage(context.Background(), req)
	require.NoError(t, err)

	var got domain.Message
	decodeTextResult(t, result, &got)
	assert.Equal(t, "msg-1", got.MessageID)
}

func TestConversationTools_Get_Success(t *testing.T) {
	conv := sampleConversation()
	msgs := []*domain.Message{sampleMessage()}
	svc := &mockMCPConvService{
		getFn: func(_ context.Context, conversationID string, _ *domain.Principal) (*conversationuc.GetResponse, error) {
			assert.Equal(t, "conv-1", conversationID)
			return &conversationuc.GetResponse{Conversation: conv, Messages: msgs}, nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_get", map[string]any{
		"conversation_id": "conv-1",
	})

	result, err := tools.handleGet(context.Background(), req)
	require.NoError(t, err)

	var got conversationuc.GetResponse
	decodeTextResult(t, result, &got)
	assert.NotNil(t, got.Conversation)
	assert.Len(t, got.Messages, 1)
}

func TestConversationTools_List_Success(t *testing.T) {
	convs := []*domain.Conversation{sampleConversation()}
	svc := &mockMCPConvService{
		listFn: func(_ context.Context, req conversationuc.ListRequest) (*conversationuc.ListResponse, error) {
			assert.Equal(t, 20, req.Limit)
			assert.Equal(t, 0, req.Offset)
			return &conversationuc.ListResponse{Conversations: convs, Total: 1}, nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_list", map[string]any{})

	result, err := tools.handleList(context.Background(), req)
	require.NoError(t, err)

	var got conversationuc.ListResponse
	decodeTextResult(t, result, &got)
	assert.Len(t, got.Conversations, 1)
	assert.Equal(t, 1, got.Total)
}

func TestConversationTools_Promote_Success(t *testing.T) {
	mem := &domain.Memory{
		ID:    "mem-1",
		Store: "my-store",
		Title: "Promoted Conversation",
	}
	svc := &mockMCPConvService{
		promoteFn: func(_ context.Context, req conversationuc.PromoteRequest) (*domain.Memory, error) {
			assert.Equal(t, "conv-1", req.ConversationID)
			assert.Equal(t, "my-store", req.Store)
			assert.False(t, req.UseSummary)
			return mem, nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_promote", map[string]any{
		"conversation_id": "conv-1",
		"store":           "my-store",
		"title":           "Promoted Conversation",
	})

	result, err := tools.handlePromote(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "mem-1", got.ID)
}

func TestConversationTools_Delete_Success(t *testing.T) {
	svc := &mockMCPConvService{
		deleteFn: func(_ context.Context, conversationID, confirm string, _ *domain.Principal) error {
			assert.Equal(t, "conv-1", conversationID)
			assert.Equal(t, "conv-1", confirm)
			return nil
		},
	}

	tools := NewConversationTools(svc)
	req := callRequest("conversation_delete", map[string]any{
		"conversation_id": "conv-1",
		"confirm":         "conv-1",
	})

	result, err := tools.handleDelete(context.Background(), req)
	require.NoError(t, err)

	var got map[string]bool
	decodeTextResult(t, result, &got)
	assert.True(t, got["deleted"])
}
