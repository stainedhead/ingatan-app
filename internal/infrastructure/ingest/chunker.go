// Package ingest provides text ingestion utilities for ingatan, including
// chunking, embedding, URL fetching, and file reading.
package ingest

import (
	"github.com/jonathanhecl/chunker"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// compile-time interface check.
var _ memoryuc.Chunker = (*RecursiveChunker)(nil)

const defaultMaxContentBytes = 10 * 1024 * 1024

// RecursiveChunker implements memory.Chunker using the jonathanhecl/chunker library.
// It uses recursive splitting with configurable size and overlap.
type RecursiveChunker struct {
	inner    *chunker.Chunker
	maxBytes int
}

// NewRecursiveChunker creates a RecursiveChunker from the given ChunkingConfig.
// If MaxContentBytes is zero, it defaults to 10 MiB.
func NewRecursiveChunker(cfg config.ChunkingConfig) *RecursiveChunker {
	maxBytes := cfg.MaxContentBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxContentBytes
	}
	inner := chunker.NewChunker(cfg.ChunkSize, cfg.ChunkOverlap, chunker.DefaultSeparators, false, false)
	return &RecursiveChunker{inner: inner, maxBytes: maxBytes}
}

// Chunk splits content into overlapping text segments.
// Returns nil with no error for empty content.
// Returns an AppError with ErrCodeContentTooLarge if content exceeds the configured limit.
func (c *RecursiveChunker) Chunk(content string) ([]string, error) {
	if len(content) > c.maxBytes {
		return nil, &domain.AppError{
			Code:    domain.ErrCodeContentTooLarge,
			Message: "content exceeds maximum size",
		}
	}
	if content == "" {
		return nil, nil
	}
	chunks := c.inner.Chunk(content)
	return chunks, nil
}
