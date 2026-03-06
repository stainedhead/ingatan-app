package conversation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// --- mock types ---

type mockRepo struct {
	conversations map[string]*domain.Conversation
	saveErr       error
	getErr        error
	updateErr     error
}

func newMockRepo() *mockRepo {
	return &mockRepo{conversations: make(map[string]*domain.Conversation)}
}

func (m *mockRepo) Save(_ context.Context, c *domain.Conversation) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.conversations[c.ConversationID] = c
	return nil
}

func (m *mockRepo) Get(_ context.Context, id string) (*domain.Conversation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.conversations[id]
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "conversation not found: "+id)
	}
	return c, nil
}

func (m *mockRepo) Update(_ context.Context, c *domain.Conversation) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.conversations[c.ConversationID] = c
	return nil
}

func (m *mockRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.conversations[id]; !ok {
		return domain.NewAppError(domain.ErrCodeNotFound, "conversation not found: "+id)
	}
	delete(m.conversations, id)
	return nil
}

func (m *mockRepo) List(_ context.Context, _ string, _, _ int) ([]*domain.Conversation, int, error) {
	result := make([]*domain.Conversation, 0, len(m.conversations))
	for _, c := range m.conversations {
		result = append(result, c)
	}
	return result, len(result), nil
}

type mockMsgRepo struct {
	messages  map[string][]*domain.Message
	appendErr error
}

func newMockMsgRepo() *mockMsgRepo {
	return &mockMsgRepo{messages: make(map[string][]*domain.Message)}
}

func (m *mockMsgRepo) Append(_ context.Context, conversationID string, msg *domain.Message) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.messages[conversationID] = append(m.messages[conversationID], msg)
	return nil
}

func (m *mockMsgRepo) GetAll(_ context.Context, conversationID string) ([]*domain.Message, error) {
	return m.messages[conversationID], nil
}

func (m *mockMsgRepo) DeleteAll(_ context.Context, conversationID string) error {
	delete(m.messages, conversationID)
	return nil
}

type mockLLM struct {
	result string
	err    error
	called bool
}

func (m *mockLLM) Summarize(_ context.Context, _ []*domain.Message, _ string) (string, error) {
	m.called = true
	return m.result, m.err
}

type mockMemorySaver struct {
	memory  *domain.Memory
	err     error
	called  bool
	lastReq CreateMemoryRequest
}

func (m *mockMemorySaver) CreateFromConversation(_ context.Context, req CreateMemoryRequest) (*domain.Memory, error) {
	m.called = true
	m.lastReq = req
	return m.memory, m.err
}

// --- test helpers ---

func adminPrincipal() *domain.Principal {
	return &domain.Principal{ID: "admin-1", Role: domain.InstanceRoleAdmin}
}

func userPrincipal(id string) *domain.Principal {
	return &domain.Principal{ID: id, Role: domain.InstanceRoleUser}
}

func defaultService(repo *mockRepo, msgRepo *mockMsgRepo) Service {
	return NewService(repo, msgRepo, nil, nil, AutoSummarizeConfig{})
}

func seedConversation(repo *mockRepo, id, ownerID, store string) *domain.Conversation {
	now := time.Now().UTC()
	c := &domain.Conversation{
		ConversationID: id,
		Title:          "Test Conversation",
		Store:          store,
		OwnerID:        ownerID,
		CreatedAt:      now,
		UpdatedAt:      now,
		MessageCount:   0,
	}
	repo.conversations[id] = c
	return c
}

// --- Start ---

func TestStart_Success(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	p := userPrincipal("alice")
	conv, err := svc.Start(context.Background(), StartRequest{
		Title:     "My Convo",
		Store:     "my-store",
		Principal: p,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, conv.ConversationID)
	assert.Equal(t, "My Convo", conv.Title)
	assert.Equal(t, "my-store", conv.Store)
	assert.Equal(t, "alice", conv.OwnerID)
	assert.Equal(t, 0, conv.MessageCount)
}

func TestStart_EmptyStore(t *testing.T) {
	svc := defaultService(newMockRepo(), newMockMsgRepo())

	_, err := svc.Start(context.Background(), StartRequest{
		Title:     "No Store",
		Store:     "",
		Principal: userPrincipal("alice"),
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestStart_DefaultTitle(t *testing.T) {
	svc := defaultService(newMockRepo(), newMockMsgRepo())

	conv, err := svc.Start(context.Background(), StartRequest{
		Title:     "",
		Store:     "my-store",
		Principal: userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.Contains(t, conv.Title, "Conversation ")
}

// --- AddMessage ---

func TestAddMessage_Success(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	conv := seedConversation(repo, "conv-1", "alice", "my-store")

	msg, err := svc.AddMessage(context.Background(), AddMessageRequest{
		ConversationID: conv.ConversationID,
		Role:           domain.MessageRoleUser,
		Content:        "Hello!",
		Principal:      userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, msg.MessageID)
	assert.Equal(t, domain.MessageRoleUser, msg.Role)
	assert.Equal(t, "Hello!", msg.Content)
	assert.Equal(t, 1, repo.conversations["conv-1"].MessageCount)
}

func TestAddMessage_UnknownConversation(t *testing.T) {
	svc := defaultService(newMockRepo(), newMockMsgRepo())

	_, err := svc.AddMessage(context.Background(), AddMessageRequest{
		ConversationID: "no-such-id",
		Role:           domain.MessageRoleUser,
		Content:        "Hi",
		Principal:      userPrincipal("alice"),
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

func TestAddMessage_Forbidden(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	conv := seedConversation(repo, "conv-1", "alice", "my-store")

	_, err := svc.AddMessage(context.Background(), AddMessageRequest{
		ConversationID: conv.ConversationID,
		Role:           domain.MessageRoleUser,
		Content:        "Hi",
		Principal:      userPrincipal("bob"),
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

func TestAddMessage_AutoSummarize(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	llm := &mockLLM{result: "Short summary."}
	svc := NewService(repo, msgRepo, llm, nil, AutoSummarizeConfig{MessageThreshold: 2})

	conv := seedConversation(repo, "conv-1", "alice", "my-store")
	conv.MessageCount = 1
	repo.conversations["conv-1"] = conv

	_, err := svc.AddMessage(context.Background(), AddMessageRequest{
		ConversationID: "conv-1",
		Role:           domain.MessageRoleUser,
		Content:        "Second message",
		Principal:      userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.True(t, llm.called, "LLM summarize should have been called")
	assert.NotNil(t, repo.conversations["conv-1"].Summary)
	assert.Equal(t, "Short summary.", repo.conversations["conv-1"].Summary.Content)
}

// --- Get ---

func TestGet_Success(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	conv := seedConversation(repo, "conv-1", "alice", "my-store")
	_ = msgRepo.Append(context.Background(), "conv-1", &domain.Message{
		MessageID: "msg-1", ConversationID: "conv-1", Role: domain.MessageRoleUser, Content: "Hi",
	})

	resp, err := svc.Get(context.Background(), conv.ConversationID, userPrincipal("alice"))

	require.NoError(t, err)
	assert.Equal(t, "conv-1", resp.Conversation.ConversationID)
	assert.Len(t, resp.Messages, 1)
}

func TestGet_Forbidden(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	conv := seedConversation(repo, "conv-1", "alice", "my-store")

	_, err := svc.Get(context.Background(), conv.ConversationID, userPrincipal("bob"))

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}

// --- List ---

func TestList_AdminSeesAll(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	seedConversation(repo, "conv-alice", "alice", "store-a")
	seedConversation(repo, "conv-bob", "bob", "store-b")

	resp, err := svc.List(context.Background(), ListRequest{
		Store:     "",
		Limit:     100,
		Offset:    0,
		Principal: adminPrincipal(),
	})

	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Conversations, 2)
}

func TestList_UserSeesOwn(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	seedConversation(repo, "conv-alice", "alice", "store-a")
	seedConversation(repo, "conv-bob", "bob", "store-b")

	resp, err := svc.List(context.Background(), ListRequest{
		Store:     "",
		Limit:     100,
		Offset:    0,
		Principal: userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Conversations, 1)
	assert.Equal(t, "alice", resp.Conversations[0].OwnerID)
}

// --- Summarize ---

func TestSummarize_Success(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	llm := &mockLLM{result: "Conversation summary."}
	svc := NewService(repo, msgRepo, llm, nil, AutoSummarizeConfig{})

	conv := seedConversation(repo, "conv-1", "alice", "my-store")
	_ = msgRepo.Append(context.Background(), "conv-1", &domain.Message{
		MessageID: "msg-1", ConversationID: "conv-1", Role: domain.MessageRoleUser, Content: "Hello",
	})

	summary, err := svc.Summarize(context.Background(), conv.ConversationID, userPrincipal("alice"))

	require.NoError(t, err)
	assert.Equal(t, "Conversation summary.", summary.Content)
	assert.Equal(t, "msg-1", summary.CoversThroughMessageID)
	assert.NotNil(t, repo.conversations["conv-1"].Summary)
}

func TestSummarize_NoLLM(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo) // no LLM

	conv := seedConversation(repo, "conv-1", "alice", "my-store")

	_, err := svc.Summarize(context.Background(), conv.ConversationID, userPrincipal("alice"))

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeLLMError, appErr.Code)
}

// --- Promote ---

func TestPromote_UseSummary(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	mem := &domain.Memory{ID: "mem-1", Title: "Promoted"}
	saver := &mockMemorySaver{memory: mem}
	svc := NewService(repo, msgRepo, nil, saver, AutoSummarizeConfig{})

	conv := seedConversation(repo, "conv-1", "alice", "my-store")
	conv.Summary = &domain.ConversationSummary{Content: "Summary text."}
	repo.conversations["conv-1"] = conv

	result, err := svc.Promote(context.Background(), PromoteRequest{
		ConversationID: "conv-1",
		Store:          "my-store",
		Title:          "My Memory",
		UseSummary:     true,
		Principal:      userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.Equal(t, "mem-1", result.ID)
	assert.True(t, saver.called)
	assert.Equal(t, "Summary text.", saver.lastReq.Content)
}

func TestPromote_Transcript(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	mem := &domain.Memory{ID: "mem-2", Title: "Transcript"}
	saver := &mockMemorySaver{memory: mem}
	svc := NewService(repo, msgRepo, nil, saver, AutoSummarizeConfig{})

	seedConversation(repo, "conv-1", "alice", "my-store")
	_ = msgRepo.Append(context.Background(), "conv-1", &domain.Message{
		MessageID: "msg-1", ConversationID: "conv-1", Role: domain.MessageRoleUser, Content: "Hello there",
	})

	result, err := svc.Promote(context.Background(), PromoteRequest{
		ConversationID: "conv-1",
		Store:          "my-store",
		UseSummary:     false,
		Principal:      userPrincipal("alice"),
	})

	require.NoError(t, err)
	assert.Equal(t, "mem-2", result.ID)
	assert.True(t, saver.called)
	assert.Contains(t, saver.lastReq.Content, "Hello there")
	assert.Contains(t, saver.lastReq.Content, "[user]")
}

func TestPromote_NoMemorySaver(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo) // no memory saver

	seedConversation(repo, "conv-1", "alice", "my-store")

	_, err := svc.Promote(context.Background(), PromoteRequest{
		ConversationID: "conv-1",
		Store:          "my-store",
		Principal:      userPrincipal("alice"),
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInternalError, appErr.Code)
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	seedConversation(repo, "conv-1", "alice", "my-store")
	_ = msgRepo.Append(context.Background(), "conv-1", &domain.Message{
		MessageID: "msg-1", ConversationID: "conv-1", Role: domain.MessageRoleUser, Content: "Hi",
	})

	err := svc.Delete(context.Background(), "conv-1", "conv-1", userPrincipal("alice"))

	require.NoError(t, err)
	_, exists := repo.conversations["conv-1"]
	assert.False(t, exists)
	assert.Empty(t, msgRepo.messages["conv-1"])
}

func TestDelete_WrongConfirm(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	seedConversation(repo, "conv-1", "alice", "my-store")

	err := svc.Delete(context.Background(), "conv-1", "wrong-id", userPrincipal("alice"))

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

func TestDelete_Forbidden(t *testing.T) {
	repo := newMockRepo()
	msgRepo := newMockMsgRepo()
	svc := defaultService(repo, msgRepo)

	seedConversation(repo, "conv-1", "alice", "my-store")

	err := svc.Delete(context.Background(), "conv-1", "conv-1", userPrincipal("bob"))

	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeForbidden, appErr.Code)
}
