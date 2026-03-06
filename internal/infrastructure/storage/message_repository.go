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
var _ conversationuc.MessageRepository = (*MessageRepo)(nil)

// MessageRepo implements conversation.MessageRepository using FileStore.
// Each message is persisted as conversations/{conversationID}-messages/{messageID}.json.
type MessageRepo struct {
	fs *FileStore
}

// NewMessageRepo creates a MessageRepo backed by the given FileStore.
func NewMessageRepo(fs *FileStore) *MessageRepo {
	return &MessageRepo{fs: fs}
}

// messageDirPath returns the relative directory path for a conversation's messages.
func messageDirPath(conversationID string) string {
	return "conversations/" + conversationID + "-messages"
}

// messagePath returns the relative path for a single message file.
func messagePath(conversationID, messageID string) string {
	return messageDirPath(conversationID) + "/" + messageID + ".json"
}

// Append writes a single message to disk.
func (r *MessageRepo) Append(_ context.Context, conversationID string, m *domain.Message) error {
	if err := r.fs.Write(messagePath(conversationID, m.MessageID), m); err != nil {
		return fmt.Errorf("storage: append message %q to conversation %q: %w", m.MessageID, conversationID, err)
	}
	return nil
}

// GetAll reads all messages for a conversation from disk, sorted by CreatedAt ascending.
// Returns an empty (nil) slice without error if no messages exist.
func (r *MessageRepo) GetAll(_ context.Context, conversationID string) ([]*domain.Message, error) {
	paths, err := r.fs.List(messageDirPath(conversationID))
	if err != nil {
		return nil, fmt.Errorf("storage: list messages for conversation %q: %w", conversationID, err)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	msgs := make([]*domain.Message, 0, len(paths))
	for _, p := range paths {
		var m domain.Message
		if err := r.fs.Read(p, &m); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("storage: read message file %q: %w", p, err)
		}
		msgs = append(msgs, &m)
	}

	// Sort by CreatedAt ascending (chronological order).
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})

	return msgs, nil
}

// DeleteAll removes all message files for a conversation.
func (r *MessageRepo) DeleteAll(_ context.Context, conversationID string) error {
	if err := r.fs.DeleteDir(messageDirPath(conversationID)); err != nil {
		return fmt.Errorf("storage: delete messages for conversation %q: %w", conversationID, err)
	}
	return nil
}
