package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
)

func defaultChunkingConfig() config.ChunkingConfig {
	return config.ChunkingConfig{
		Strategy:        "recursive",
		ChunkSize:       512,
		ChunkOverlap:    64,
		MaxContentBytes: 10 * 1024 * 1024,
	}
}

func TestRecursiveChunker_ShortContent(t *testing.T) {
	c := NewRecursiveChunker(defaultChunkingConfig())
	content := "Hello, world!"

	chunks, err := c.Chunk(content)

	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, content, chunks[0])
}

func TestRecursiveChunker_LongContent(t *testing.T) {
	cfg := defaultChunkingConfig()
	cfg.ChunkSize = 100
	cfg.ChunkOverlap = 10
	c := NewRecursiveChunker(cfg)

	// Build content longer than chunk size using repeated words with spaces.
	word := "ingatan "
	content := strings.Repeat(word, 30) // 240 chars, needs multiple chunks
	chunks, err := c.Chunk(content)

	require.NoError(t, err)
	assert.Greater(t, len(chunks), 1, "long content should produce multiple chunks")
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), cfg.ChunkSize,
			"chunk %d length %d exceeds chunkSize %d", i, len(chunk), cfg.ChunkSize)
	}
}

func TestRecursiveChunker_EmptyContent(t *testing.T) {
	c := NewRecursiveChunker(defaultChunkingConfig())

	chunks, err := c.Chunk("")

	require.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestRecursiveChunker_ContentTooLarge(t *testing.T) {
	cfg := defaultChunkingConfig()
	cfg.MaxContentBytes = 10
	c := NewRecursiveChunker(cfg)
	content := strings.Repeat("x", 11) // 11 bytes, over the 10-byte limit

	chunks, err := c.Chunk(content)

	require.Error(t, err)
	assert.Nil(t, chunks)

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeContentTooLarge, appErr.Code)
}

func TestRecursiveChunker_Overlap(t *testing.T) {
	cfg := defaultChunkingConfig()
	cfg.ChunkSize = 80
	cfg.ChunkOverlap = 20
	c := NewRecursiveChunker(cfg)

	// Build content with clearly separated words so overlap is exercisable.
	words := make([]string, 20)
	for i := range words {
		words[i] = "wordtoken"
	}
	content := strings.Join(words, " ") // 20 * 9 + 19 spaces = 199 chars

	chunks, err := c.Chunk(content)

	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "expected multiple chunks to verify overlap")

	// At least one consecutive pair must share a common suffix/prefix substring,
	// which would be indicative of the overlap sliding window.
	foundOverlap := false
	for i := 0; i < len(chunks)-1; i++ {
		// Check that the end of chunk[i] appears somewhere in chunk[i+1].
		endOfPrev := chunks[i]
		if len(endOfPrev) > cfg.ChunkOverlap {
			endOfPrev = endOfPrev[len(endOfPrev)-cfg.ChunkOverlap:]
		}
		if strings.Contains(chunks[i+1], endOfPrev) {
			foundOverlap = true
			break
		}
	}
	assert.True(t, foundOverlap, "expected overlapping content between consecutive chunks")
}
