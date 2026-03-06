package domain

import "time"

// MessageRole identifies the author role of a conversation message.
type MessageRole string

const (
	// MessageRoleUser is a message from the human user.
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant is a message from the AI assistant.
	MessageRoleAssistant MessageRole = "assistant"
	// MessageRoleSystem is a system-level instruction message.
	MessageRoleSystem MessageRole = "system"
	// MessageRoleTool is a tool call or tool result message.
	MessageRoleTool MessageRole = "tool"
)

// ConversationSummary is an LLM-generated summary of a conversation up to a point.
type ConversationSummary struct {
	Content                string
	GeneratedAt            time.Time
	CoversThroughMessageID string // The last message ID included in the summary.
}

// Conversation tracks a thread of messages, optionally associated with a store.
type Conversation struct {
	ConversationID string
	Title          string
	Store          string
	OwnerID        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	MessageCount   int
	Summary        *ConversationSummary // Nil until auto-summarized or explicitly requested.
}

// Message is a single turn in a Conversation.
type Message struct {
	MessageID      string
	ConversationID string
	Role           MessageRole
	Content        string
	Metadata       map[string]any
	CreatedAt      time.Time
}
