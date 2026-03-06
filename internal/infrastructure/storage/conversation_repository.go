package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
)

// Compile-time interface check.
var _ conversationuc.Repository = (*ConversationRepo)(nil)

// ConversationRepo implements conversation.Repository using FileStore.
// Records are persisted as conversations/{id}.json.
type ConversationRepo struct {
	fs *FileStore
}

// NewConversationRepo creates a ConversationRepo backed by the given FileStore.
func NewConversationRepo(fs *FileStore) *ConversationRepo {
	return &ConversationRepo{fs: fs}
}

// conversationPath returns the relative path for a conversation record.
func conversationPath(id string) string {
	return "conversations/" + id + ".json"
}

// Save persists a conversation record to disk.
func (r *ConversationRepo) Save(_ context.Context, c *domain.Conversation) error {
	if err := r.fs.Write(conversationPath(c.ConversationID), c); err != nil {
		return fmt.Errorf("storage: save conversation %q: %w", c.ConversationID, err)
	}
	return nil
}

// Get reads a conversation record from disk.
// Returns a NOT_FOUND AppError if the conversation does not exist.
func (r *ConversationRepo) Get(_ context.Context, conversationID string) (*domain.Conversation, error) {
	var c domain.Conversation
	if err := r.fs.Read(conversationPath(conversationID), &c); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "conversation not found: "+conversationID)
		}
		return nil, fmt.Errorf("storage: read conversation %q: %w", conversationID, err)
	}
	return &c, nil
}

// Update overwrites an existing conversation record with the provided value.
func (r *ConversationRepo) Update(_ context.Context, c *domain.Conversation) error {
	if err := r.fs.Write(conversationPath(c.ConversationID), c); err != nil {
		return fmt.Errorf("storage: update conversation %q: %w", c.ConversationID, err)
	}
	return nil
}

// Delete removes a conversation record and its associated messages directory.
func (r *ConversationRepo) Delete(_ context.Context, conversationID string) error {
	if err := r.fs.Delete(conversationPath(conversationID)); err != nil {
		return fmt.Errorf("storage: delete conversation %q: %w", conversationID, err)
	}
	if err := r.fs.DeleteDir("conversations/" + conversationID + "-messages"); err != nil {
		return fmt.Errorf("storage: delete messages dir for conversation %q: %w", conversationID, err)
	}
	return nil
}

// List returns conversations filtered by store, sorted by UpdatedAt descending,
// with pagination applied. If store is empty all conversations are returned.
// Returns the filtered slice and the total count before pagination.
func (r *ConversationRepo) List(_ context.Context, store string, limit, offset int) ([]*domain.Conversation, int, error) {
	paths, err := r.fs.List("conversations")
	if err != nil {
		return nil, 0, fmt.Errorf("storage: list conversations dir: %w", err)
	}

	var all []*domain.Conversation
	for _, p := range paths {
		var c domain.Conversation
		if err := r.fs.Read(p, &c); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, 0, fmt.Errorf("storage: read conversation file %q: %w", p, err)
		}
		if store != "" && c.Store != store {
			continue
		}
		all = append(all, &c)
	}

	// Sort by UpdatedAt descending (newest first).
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})

	total := len(all)

	// Apply offset.
	if offset > total {
		offset = total
	}
	all = all[offset:]

	// Apply limit (0 means no limit).
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all, total, nil
}
