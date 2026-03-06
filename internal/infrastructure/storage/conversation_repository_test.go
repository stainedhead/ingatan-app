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

// newTestConversation returns a Conversation with deterministic values for testing.
func newTestConversation(id, title, store, ownerID string, updatedAt time.Time) *domain.Conversation {
	return &domain.Conversation{
		ConversationID: id,
		Title:          title,
		Store:          store,
		OwnerID:        ownerID,
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:      updatedAt,
		MessageCount:   0,
	}
}

// TestConversationRepo_SaveAndGet verifies that a saved conversation can be read back with all fields intact.
func TestConversationRepo_SaveAndGet(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	c := newTestConversation("conv-1", "My Title", "store-a", "alice", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	require.NoError(t, repo.Save(ctx, c))

	got, err := repo.Get(ctx, "conv-1")
	require.NoError(t, err)

	assert.Equal(t, c.ConversationID, got.ConversationID)
	assert.Equal(t, c.Title, got.Title)
	assert.Equal(t, c.Store, got.Store)
	assert.Equal(t, c.OwnerID, got.OwnerID)
	assert.Equal(t, c.CreatedAt.UTC(), got.CreatedAt.UTC())
	assert.Equal(t, c.UpdatedAt.UTC(), got.UpdatedAt.UTC())
}

// TestConversationRepo_Get_NotFound verifies that Get returns a NOT_FOUND AppError for missing conversations.
func TestConversationRepo_Get_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	require.Error(t, err)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestConversationRepo_Update verifies that Update overwrites the existing record.
func TestConversationRepo_Update(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	c := newTestConversation("conv-u", "Original Title", "store-a", "alice", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, repo.Save(ctx, c))

	c.Title = "Updated Title"
	c.MessageCount = 5
	require.NoError(t, repo.Update(ctx, c))

	got, err := repo.Get(ctx, "conv-u")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", got.Title)
	assert.Equal(t, 5, got.MessageCount)
}

// TestConversationRepo_Delete verifies that a deleted conversation returns NOT_FOUND on subsequent Get.
func TestConversationRepo_Delete(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	c := newTestConversation("conv-del", "To Delete", "store-a", "alice", time.Now())
	require.NoError(t, repo.Save(ctx, c))

	require.NoError(t, repo.Delete(ctx, "conv-del"))

	_, err := repo.Get(ctx, "conv-del")
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestConversationRepo_Delete_AlsoRemovesMessagesDir verifies that Delete removes the messages directory.
func TestConversationRepo_Delete_AlsoRemovesMessagesDir(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	msgRepo := NewMessageRepo(fs)
	ctx := context.Background()

	c := newTestConversation("conv-delmsg", "With Messages", "store-a", "alice", time.Now())
	require.NoError(t, repo.Save(ctx, c))

	msg := &domain.Message{
		MessageID:      "msg-1",
		ConversationID: "conv-delmsg",
		Role:           domain.MessageRoleUser,
		Content:        "hello",
		CreatedAt:      time.Now(),
	}
	require.NoError(t, msgRepo.Append(ctx, "conv-delmsg", msg))

	require.NoError(t, repo.Delete(ctx, "conv-delmsg"))

	// Messages should be gone too.
	msgs, err := msgRepo.GetAll(ctx, "conv-delmsg")
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

// TestConversationRepo_List_FilterByStore verifies that List filters by store.
func TestConversationRepo_List_FilterByStore(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Save(ctx, newTestConversation("c1", "T1", "store-a", "alice", base.Add(3*time.Hour))))
	require.NoError(t, repo.Save(ctx, newTestConversation("c2", "T2", "store-a", "alice", base.Add(2*time.Hour))))
	require.NoError(t, repo.Save(ctx, newTestConversation("c3", "T3", "store-b", "alice", base.Add(1*time.Hour))))

	list, total, err := repo.List(ctx, "store-a", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, list, 2)
	// Sorted newest first.
	assert.Equal(t, "c1", list[0].ConversationID)
	assert.Equal(t, "c2", list[1].ConversationID)
}

// TestConversationRepo_List_NoFilter verifies that List with empty store returns all conversations.
func TestConversationRepo_List_NoFilter(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Save(ctx, newTestConversation("c1", "T1", "store-a", "alice", base.Add(2*time.Hour))))
	require.NoError(t, repo.Save(ctx, newTestConversation("c2", "T2", "store-b", "alice", base.Add(1*time.Hour))))

	list, total, err := repo.List(ctx, "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, list, 2)
}

// TestConversationRepo_List_Pagination verifies that limit and offset paginate results correctly.
func TestConversationRepo_List_Pagination(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		c := newTestConversation(
			fmt.Sprintf("c%d", i),
			fmt.Sprintf("Title %d", i),
			"store-a",
			"alice",
			base.Add(time.Duration(i)*time.Hour),
		)
		require.NoError(t, repo.Save(ctx, c))
	}

	// page 1: limit=2, offset=0
	page1, total, err := repo.List(ctx, "store-a", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 2)

	// page 2: limit=2, offset=2
	page2, total2, err := repo.List(ctx, "store-a", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, page2, 2)

	// page 3: limit=2, offset=4
	page3, _, err := repo.List(ctx, "store-a", 2, 4)
	require.NoError(t, err)
	assert.Len(t, page3, 1)
}

// TestConversationRepo_List_Empty verifies that List returns nil slice (not an error) when no conversations exist.
func TestConversationRepo_List_Empty(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewConversationRepo(fs)
	ctx := context.Background()

	list, total, err := repo.List(ctx, "store-a", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Nil(t, list)
}
