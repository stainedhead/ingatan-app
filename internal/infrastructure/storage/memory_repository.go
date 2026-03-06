// Package storage provides file-based JSON persistence for ingatan.
package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// Compile-time interface checks.
var _ memoryuc.Repository = (*MemoryRepo)(nil)
var _ memoryuc.ChunkRepository = (*ChunkRepo)(nil)

// MemoryRepo implements memory.Repository using FileStore.
// Records are persisted as JSON files under stores/{store}/memories/{memoryID}.json.
type MemoryRepo struct {
	fs *FileStore
}

// NewMemoryRepo creates a MemoryRepo backed by the given FileStore.
func NewMemoryRepo(fs *FileStore) *MemoryRepo {
	return &MemoryRepo{fs: fs}
}

// ChunkRepo implements memory.ChunkRepository using FileStore.
// Chunk arrays are persisted as JSON files under stores/{store}/memories/{memoryID}-chunks.json.
type ChunkRepo struct {
	fs *FileStore
}

// NewChunkRepo creates a ChunkRepo backed by the given FileStore.
func NewChunkRepo(fs *FileStore) *ChunkRepo {
	return &ChunkRepo{fs: fs}
}

// memoryPath returns the relative path for a memory record.
func memoryPath(store, memoryID string) string {
	return filepath.Join("stores", store, "memories", memoryID+".json")
}

// chunksPath returns the relative path for a memory's chunk array.
func chunksPath(store, memoryID string) string {
	return filepath.Join("stores", store, "memories", memoryID+"-chunks.json")
}

// memoriesDir returns the relative directory path for a store's memory files.
func memoriesDir(store string) string {
	return filepath.Join("stores", store, "memories")
}

// Save writes a memory record to disk.
func (r *MemoryRepo) Save(_ context.Context, m *domain.Memory) error {
	return r.fs.Write(memoryPath(m.Store, m.ID), m)
}

// Get reads a memory record from disk.
// Returns ErrNotFound if the memory does not exist.
func (r *MemoryRepo) Get(_ context.Context, store, memoryID string) (*domain.Memory, error) {
	var m domain.Memory
	if err := r.fs.Read(memoryPath(store, memoryID), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Update overwrites an existing memory record on disk.
func (r *MemoryRepo) Update(_ context.Context, m *domain.Memory) error {
	return r.fs.Write(memoryPath(m.Store, m.ID), m)
}

// Delete removes a memory record from disk. Returns nil if the record does not exist.
func (r *MemoryRepo) Delete(_ context.Context, store, memoryID string) error {
	return r.fs.Delete(memoryPath(store, memoryID))
}

// List returns memories in the given store that match filter, with pagination.
// All matching memories are counted for the total; only the page [offset:offset+limit] is returned.
// A limit of 0 returns all matching memories.
func (r *MemoryRepo) List(_ context.Context, store string, filter memoryuc.Filter, limit, offset int) ([]*domain.Memory, int, error) {
	paths, err := r.fs.List(memoriesDir(store))
	if err != nil {
		return nil, 0, fmt.Errorf("storage: list memories dir: %w", err)
	}

	var matched []*domain.Memory
	for _, p := range paths {
		// Skip chunk files; only process memory records.
		if strings.HasSuffix(p, "-chunks.json") {
			continue
		}

		var m domain.Memory
		if err := r.fs.Read(p, &m); err != nil {
			return nil, 0, fmt.Errorf("storage: read memory %q: %w", p, err)
		}

		if !matchesFilter(&m, filter) {
			continue
		}
		matched = append(matched, &m)
	}

	total := len(matched)

	if offset >= total {
		return []*domain.Memory{}, total, nil
	}

	end := total
	if limit > 0 && offset+limit < total {
		end = offset + limit
	}

	return matched[offset:end], total, nil
}

// matchesFilter returns true when m satisfies all conditions in filter.
// Tag matching uses AND semantics: the memory must contain every requested tag.
func matchesFilter(m *domain.Memory, filter memoryuc.Filter) bool {
	if filter.Source != nil && m.Source != *filter.Source {
		return false
	}
	for _, required := range filter.Tags {
		if !containsTag(m.Tags, required) {
			return false
		}
	}
	return true
}

// containsTag reports whether tag appears in tags.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SaveChunks persists the chunk array for a memory.
func (r *ChunkRepo) SaveChunks(_ context.Context, store, memoryID string, chunks []*domain.MemoryChunk) error {
	return r.fs.Write(chunksPath(store, memoryID), chunks)
}

// GetChunks returns all chunks for a memory.
// Returns an empty slice (not an error) if no chunk file exists.
func (r *ChunkRepo) GetChunks(_ context.Context, store, memoryID string) ([]*domain.MemoryChunk, error) {
	var chunks []*domain.MemoryChunk
	if err := r.fs.Read(chunksPath(store, memoryID), &chunks); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []*domain.MemoryChunk{}, nil
		}
		return nil, fmt.Errorf("storage: read chunks for memory %q: %w", memoryID, err)
	}
	return chunks, nil
}

// GetChunkByID looks up a single chunk by its ID within the given store.
// It scans all chunk files in the store's memories directory to find the matching chunk.
func (r *ChunkRepo) GetChunkByID(_ context.Context, store, chunkID string) (*domain.MemoryChunk, error) {
	paths, err := r.fs.List(memoriesDir(store))
	if err != nil {
		return nil, fmt.Errorf("storage: list chunk files: %w", err)
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, "-chunks.json") {
			continue
		}
		var chunks []*domain.MemoryChunk
		if err := r.fs.Read(p, &chunks); err != nil {
			continue
		}
		for _, c := range chunks {
			if c.ChunkID == chunkID {
				return c, nil
			}
		}
	}
	return nil, ErrNotFound
}

// DeleteChunks removes the chunk file for a memory. Returns nil if it does not exist.
func (r *ChunkRepo) DeleteChunks(_ context.Context, store, memoryID string) error {
	return r.fs.Delete(chunksPath(store, memoryID))
}
