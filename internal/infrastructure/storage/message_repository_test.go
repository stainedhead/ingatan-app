package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// newTestMessage returns a Message with deterministic values for testing.
func newTestMessage(msgID, convID string, role domain.MessageRole, content string, createdAt time.Time) *domain.Message {
	return &domain.Message{
		MessageID:      msgID,
		ConversationID: convID,
		Role:           role,
		Content:        content,
		CreatedAt:      createdAt,
	}
}

// TestMessageRepo_AppendAndGetAll verifies that appended messages are returned sorted by CreatedAt.
func TestMessageRepo_AppendAndGetAll(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m1 := newTestMessage("msg-1", "conv-1", domain.MessageRoleUser, "hello", base)
	m2 := newTestMessage("msg-2", "conv-1", domain.MessageRoleAssistant, "world", base.Add(time.Minute))
	m3 := newTestMessage("msg-3", "conv-1", domain.MessageRoleUser, "again", base.Add(2*time.Minute))

	// Append out of chronological order to verify sorting.
	require.NoError(t, repo.Append(ctx, "conv-1", m3))
	require.NoError(t, repo.Append(ctx, "conv-1", m1))
	require.NoError(t, repo.Append(ctx, "conv-1", m2))

	msgs, err := repo.GetAll(ctx, "conv-1")
	require.NoError(t, err)
	require.Len(t, msgs, 3)

	// Verify chronological order.
	assert.Equal(t, "msg-1", msgs[0].MessageID)
	assert.Equal(t, "msg-2", msgs[1].MessageID)
	assert.Equal(t, "msg-3", msgs[2].MessageID)
}

// TestMessageRepo_GetAll_Empty verifies that GetAll returns nil (not an error) when no messages exist.
func TestMessageRepo_GetAll_Empty(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	msgs, err := repo.GetAll(ctx, "conv-nonexistent")
	require.NoError(t, err)
	assert.Nil(t, msgs)
}

// TestMessageRepo_GetAll_FieldsRoundTrip verifies that all message fields survive a round-trip.
func TestMessageRepo_GetAll_FieldsRoundTrip(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	m := &domain.Message{
		MessageID:      "msg-rt",
		ConversationID: "conv-rt",
		Role:           domain.MessageRoleAssistant,
		Content:        "I am the assistant.",
		Metadata:       map[string]any{"token_count": float64(42)},
		CreatedAt:      time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
	}

	require.NoError(t, repo.Append(ctx, "conv-rt", m))

	msgs, err := repo.GetAll(ctx, "conv-rt")
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	got := msgs[0]
	assert.Equal(t, m.MessageID, got.MessageID)
	assert.Equal(t, m.ConversationID, got.ConversationID)
	assert.Equal(t, m.Role, got.Role)
	assert.Equal(t, m.Content, got.Content)
	assert.Equal(t, m.CreatedAt.UTC(), got.CreatedAt.UTC())
	assert.Equal(t, float64(42), got.Metadata["token_count"])
}

// TestMessageRepo_DeleteAll removes all messages for a conversation.
func TestMessageRepo_DeleteAll(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		m := newTestMessage(
			fmt.Sprintf("msg-%d", i),
			"conv-del",
			domain.MessageRoleUser,
			"content",
			base.Add(time.Duration(i)*time.Second),
		)
		require.NoError(t, repo.Append(ctx, "conv-del", m))
	}

	require.NoError(t, repo.DeleteAll(ctx, "conv-del"))

	msgs, err := repo.GetAll(ctx, "conv-del")
	require.NoError(t, err)
	assert.Nil(t, msgs)
}

// TestMessageRepo_DeleteAll_NoMessages verifies that DeleteAll is a no-op when no messages exist.
func TestMessageRepo_DeleteAll_NoMessages(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	// DeleteAll on a non-existent dir should not error.
	require.NoError(t, repo.DeleteAll(ctx, "conv-empty"))
}

// TestMessageRepo_MultipleConversations verifies that messages from different conversations are isolated.
func TestMessageRepo_MultipleConversations(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMessageRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Append(ctx, "conv-a", newTestMessage("msg-a1", "conv-a", domain.MessageRoleUser, "hello a", base)))
	require.NoError(t, repo.Append(ctx, "conv-b", newTestMessage("msg-b1", "conv-b", domain.MessageRoleUser, "hello b", base)))

	msgsA, err := repo.GetAll(ctx, "conv-a")
	require.NoError(t, err)
	require.Len(t, msgsA, 1)
	assert.Equal(t, "msg-a1", msgsA[0].MessageID)

	msgsB, err := repo.GetAll(ctx, "conv-b")
	require.NoError(t, err)
	require.Len(t, msgsB, 1)
	assert.Equal(t, "msg-b1", msgsB[0].MessageID)
}
