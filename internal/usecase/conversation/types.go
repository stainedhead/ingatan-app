// Package conversation provides the use case layer for conversation management in ingatan.
package conversation

import (
	"context"

	"github.com/stainedhead/ingatan/internal/domain"
)

// Repository is the persistence interface for conversation records.
// Defined here (use case layer) and implemented by the infrastructure layer.
type Repository interface {
	Save(ctx context.Context, c *domain.Conversation) error
	Get(ctx context.Context, conversationID string) (*domain.Conversation, error)
	Update(ctx context.Context, c *domain.Conversation) error
	Delete(ctx context.Context, conversationID string) error
	List(ctx context.Context, store string, limit, offset int) ([]*domain.Conversation, int, error)
}

// MessageRepository is the persistence interface for conversation messages.
// Defined here (use case layer) and implemented by the infrastructure layer.
type MessageRepository interface {
	Append(ctx context.Context, conversationID string, m *domain.Message) error
	GetAll(ctx context.Context, conversationID string) ([]*domain.Message, error)
	DeleteAll(ctx context.Context, conversationID string) error
}

// LLMProvider generates text summaries from conversation messages.
// Defined here (use case layer) and implemented by the infrastructure layer.
type LLMProvider interface {
	Summarize(ctx context.Context, messages []*domain.Message, systemPrompt string) (string, error)
}

// MemorySaver creates a memory record from promoted conversation content.
// Implemented by an adapter in cmd/ingatan that wraps the memory service,
// avoiding a direct use case → use case dependency.
type MemorySaver interface {
	CreateFromConversation(ctx context.Context, req CreateMemoryRequest) (*domain.Memory, error)
}

// CreateMemoryRequest carries the data needed to promote a conversation to memory.
type CreateMemoryRequest struct {
	Store          string
	Title          string
	Content        string
	Tags           []string
	ConversationID string
	Principal      *domain.Principal
}

// Service exposes conversation management operations.
type Service interface {
	Start(ctx context.Context, req StartRequest) (*domain.Conversation, error)
	AddMessage(ctx context.Context, req AddMessageRequest) (*domain.Message, error)
	Get(ctx context.Context, conversationID string, principal *domain.Principal) (*GetResponse, error)
	List(ctx context.Context, req ListRequest) (*ListResponse, error)
	Summarize(ctx context.Context, conversationID string, principal *domain.Principal) (*domain.ConversationSummary, error)
	Promote(ctx context.Context, req PromoteRequest) (*domain.Memory, error)
	Delete(ctx context.Context, conversationID, confirm string, principal *domain.Principal) error
}

// StartRequest carries input for Service.Start.
type StartRequest struct {
	Title     string
	Store     string
	Principal *domain.Principal
}

// AddMessageRequest carries input for Service.AddMessage.
type AddMessageRequest struct {
	ConversationID string
	Role           domain.MessageRole
	Content        string
	Metadata       map[string]any
	Principal      *domain.Principal
}

// GetResponse is the response for Service.Get, including the conversation and its messages.
type GetResponse struct {
	Conversation *domain.Conversation
	Messages     []*domain.Message
}

// ListRequest carries input for Service.List.
type ListRequest struct {
	Store     string
	Limit     int
	Offset    int
	Principal *domain.Principal
}

// ListResponse is the response for Service.List.
type ListResponse struct {
	Conversations []*domain.Conversation
	Total         int
}

// PromoteRequest carries input for Service.Promote.
type PromoteRequest struct {
	ConversationID string
	Store          string
	Title          string
	Tags           []string
	UseSummary     bool // true = promote summary text; false = promote full transcript
	Principal      *domain.Principal
}

// AutoSummarizeConfig holds the thresholds for auto-summarization.
type AutoSummarizeConfig struct {
	// MessageThreshold triggers auto-summarize when MessageCount reaches this value.
	// 0 means disabled.
	MessageThreshold int
	// TokenEstimateThreshold triggers auto-summarize when estimated token count reaches this value.
	// Estimated as len(content)/4. 0 means disabled.
	TokenEstimateThreshold int
}
