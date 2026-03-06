package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stainedhead/ingatan/internal/domain"
)

// serviceImpl is the concrete implementation of Service.
type serviceImpl struct {
	repo        Repository
	msgRepo     MessageRepository
	llm         LLMProvider // nil if not configured
	memorySaver MemorySaver // nil if not configured
	autoSumm    AutoSummarizeConfig
}

// NewService constructs a Service.
// llm and memorySaver may be nil; those features will be disabled.
func NewService(repo Repository, msgRepo MessageRepository, llm LLMProvider, memorySaver MemorySaver, autoSumm AutoSummarizeConfig) Service {
	return &serviceImpl{
		repo:        repo,
		msgRepo:     msgRepo,
		llm:         llm,
		memorySaver: memorySaver,
		autoSumm:    autoSumm,
	}
}

// Start creates a new conversation owned by the requesting principal.
// Store and principal are required. Title defaults to "Conversation <date>" if empty.
func (s *serviceImpl) Start(ctx context.Context, req StartRequest) (*domain.Conversation, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store is required")
	}
	if req.Principal == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "principal is required")
	}

	title := req.Title
	if title == "" {
		title = "Conversation " + time.Now().UTC().Format("2006-01-02")
	}

	now := time.Now().UTC()
	conv := &domain.Conversation{
		ConversationID: uuid.New().String(),
		Title:          title,
		Store:          req.Store,
		OwnerID:        req.Principal.ID,
		CreatedAt:      now,
		UpdatedAt:      now,
		MessageCount:   0,
	}

	if err := s.repo.Save(ctx, conv); err != nil {
		return nil, fmt.Errorf("conversation: start: %w", err)
	}

	return conv, nil
}

// AddMessage appends a message to an existing conversation, checking access rights.
// Auto-summarization is triggered synchronously when the configured threshold is reached.
func (s *serviceImpl) AddMessage(ctx context.Context, req AddMessageRequest) (*domain.Message, error) {
	if req.ConversationID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "conversationID is required")
	}
	if req.Role == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "role is required")
	}
	if req.Content == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "content is required")
	}

	conv, err := s.repo.Get(ctx, req.ConversationID)
	if err != nil {
		return nil, wrapConvNotFound(err, req.ConversationID)
	}

	if !canAccess(conv, req.Principal) {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "access to conversation denied")
	}

	now := time.Now().UTC()
	msg := &domain.Message{
		MessageID:      uuid.New().String(),
		ConversationID: req.ConversationID,
		Role:           req.Role,
		Content:        req.Content,
		Metadata:       req.Metadata,
		CreatedAt:      now,
	}

	if err := s.msgRepo.Append(ctx, req.ConversationID, msg); err != nil {
		return nil, fmt.Errorf("conversation: add message: %w", err)
	}

	conv.MessageCount++
	conv.UpdatedAt = now
	if err := s.repo.Update(ctx, conv); err != nil {
		return nil, fmt.Errorf("conversation: update after add message: %w", err)
	}

	if s.shouldAutoSummarize(conv) {
		_, _ = s.doSummarize(ctx, conv)
	}

	return msg, nil
}

// Get retrieves a conversation and all its messages, checking access rights.
func (s *serviceImpl) Get(ctx context.Context, conversationID string, principal *domain.Principal) (*GetResponse, error) {
	conv, err := s.repo.Get(ctx, conversationID)
	if err != nil {
		return nil, wrapConvNotFound(err, conversationID)
	}

	if !canAccess(conv, principal) {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "access to conversation denied")
	}

	msgs, err := s.msgRepo.GetAll(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation: get messages: %w", err)
	}

	return &GetResponse{Conversation: conv, Messages: msgs}, nil
}

// List returns conversations in the given store.
// Admins see all conversations; non-admins see only conversations they own.
func (s *serviceImpl) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
	all, _, err := s.repo.List(ctx, req.Store, req.Limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("conversation: list: %w", err)
	}

	if req.Principal.Role == domain.InstanceRoleAdmin {
		return &ListResponse{Conversations: all, Total: len(all)}, nil
	}

	var filtered []*domain.Conversation
	for _, c := range all {
		if c.OwnerID == req.Principal.ID {
			filtered = append(filtered, c)
		}
	}
	return &ListResponse{Conversations: filtered, Total: len(filtered)}, nil
}

// Summarize generates an LLM summary for the conversation and persists it.
// Returns LLM_ERROR if no LLM provider is configured.
func (s *serviceImpl) Summarize(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error) {
	conv, err := s.repo.Get(ctx, conversationID)
	if err != nil {
		return nil, wrapConvNotFound(err, conversationID)
	}

	if !canAccess(conv, principal) {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "access to conversation denied")
	}

	return s.doSummarize(ctx, conv)
}

// Promote converts a conversation (or its summary) into a persistent memory record.
// Returns INTERNAL_ERROR if no memory saver is configured.
func (s *serviceImpl) Promote(ctx context.Context, req PromoteRequest) (*domain.Memory, error) {
	if req.ConversationID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "conversationID is required")
	}
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store is required")
	}

	conv, err := s.repo.Get(ctx, req.ConversationID)
	if err != nil {
		return nil, wrapConvNotFound(err, req.ConversationID)
	}

	if !canAccess(conv, req.Principal) {
		return nil, domain.NewAppError(domain.ErrCodeForbidden, "access to conversation denied")
	}

	if s.memorySaver == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "memory saver not configured")
	}

	var content string
	if req.UseSummary && conv.Summary != nil {
		content = conv.Summary.Content
	} else {
		msgs, err := s.msgRepo.GetAll(ctx, req.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("conversation: promote get messages: %w", err)
		}
		content = buildTranscript(msgs)
	}

	title := req.Title
	if title == "" {
		title = conv.Title
	}
	if title == "" {
		title = "Conversation " + req.ConversationID[:8]
	}

	mem, err := s.memorySaver.CreateFromConversation(ctx, CreateMemoryRequest{
		Store:          req.Store,
		Title:          title,
		Content:        content,
		Tags:           req.Tags,
		ConversationID: req.ConversationID,
		Principal:      req.Principal,
	})
	if err != nil {
		return nil, fmt.Errorf("conversation: promote: %w", err)
	}

	return mem, nil
}

// Delete removes a conversation and all its messages.
// The confirm parameter must equal the conversationID.
func (s *serviceImpl) Delete(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error {
	conv, err := s.repo.Get(ctx, conversationID)
	if err != nil {
		return wrapConvNotFound(err, conversationID)
	}

	if confirm != conversationID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "confirm must equal the conversationID")
	}

	if !canAccess(conv, principal) {
		return domain.NewAppError(domain.ErrCodeForbidden, "access to conversation denied")
	}

	if err := s.msgRepo.DeleteAll(ctx, conversationID); err != nil {
		return fmt.Errorf("conversation: delete messages: %w", err)
	}

	if err := s.repo.Delete(ctx, conversationID); err != nil {
		return fmt.Errorf("conversation: delete: %w", err)
	}

	return nil
}

// --- helpers ---

// canAccess returns true if the principal is the conversation owner or an instance admin.
func canAccess(conv *domain.Conversation, principal *domain.Principal) bool {
	if principal == nil {
		return false
	}
	if principal.Role == domain.InstanceRoleAdmin {
		return true
	}
	return conv.OwnerID == principal.ID
}

// shouldAutoSummarize returns true when the auto-summarize threshold has been reached.
func (s *serviceImpl) shouldAutoSummarize(conv *domain.Conversation) bool {
	if s.llm == nil {
		return false
	}
	if s.autoSumm.MessageThreshold > 0 && conv.MessageCount >= s.autoSumm.MessageThreshold {
		return true
	}
	return false
}

// doSummarize fetches messages, calls the LLM, persists the summary, and returns it.
func (s *serviceImpl) doSummarize(ctx context.Context, conv *domain.Conversation) (*domain.ConversationSummary, error) {
	if s.llm == nil {
		return nil, domain.NewAppError(domain.ErrCodeLLMError, "no LLM provider configured")
	}

	msgs, err := s.msgRepo.GetAll(ctx, conv.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation: summarize get messages: %w", err)
	}

	const systemPrompt = "Summarize the following conversation concisely."
	text, err := s.llm.Summarize(ctx, msgs, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("conversation: summarize: %w", err)
	}

	var lastMsgID string
	if len(msgs) > 0 {
		lastMsgID = msgs[len(msgs)-1].MessageID
	}

	summary := &domain.ConversationSummary{
		Content:                text,
		GeneratedAt:            time.Now().UTC(),
		CoversThroughMessageID: lastMsgID,
	}

	conv.Summary = summary
	if err := s.repo.Update(ctx, conv); err != nil {
		return nil, fmt.Errorf("conversation: summarize update: %w", err)
	}

	return summary, nil
}

// buildTranscript formats a slice of messages as a readable text transcript.
func buildTranscript(msgs []*domain.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", m.Role, m.Content))
	}
	return sb.String()
}

// wrapConvNotFound converts a storage not-found error into a domain NOT_FOUND AppError.
func wrapConvNotFound(err error, id string) error {
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeNotFound {
		return appErr
	}
	return domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("conversation not found: %s", id))
}
