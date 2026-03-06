package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// newTestMemory returns a Memory with deterministic values for testing.
func newTestMemory(id, store, title string, tags []string, source domain.MemorySource) *domain.Memory {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &domain.Memory{
		ID:        id,
		Store:     store,
		Title:     title,
		Content:   "content of " + title,
		Tags:      tags,
		Source:    source,
		SourceRef: "ref-" + id,
		Metadata:  map[string]any{"key": "val"},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TestMemoryRepo_SaveAndGet verifies that a saved memory can be read back with all fields intact.
func TestMemoryRepo_SaveAndGet(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	m := newTestMemory("mem-1", "default", "Hello World", []string{"go", "test"}, domain.MemorySourceManual)

	require.NoError(t, repo.Save(ctx, m))

	got, err := repo.Get(ctx, "default", "mem-1")
	require.NoError(t, err)

	assert.Equal(t, m.ID, got.ID)
	assert.Equal(t, m.Store, got.Store)
	assert.Equal(t, m.Title, got.Title)
	assert.Equal(t, m.Content, got.Content)
	assert.Equal(t, m.Tags, got.Tags)
	assert.Equal(t, m.Source, got.Source)
	assert.Equal(t, m.SourceRef, got.SourceRef)
	assert.Equal(t, m.CreatedAt.UTC(), got.CreatedAt.UTC())
	assert.Equal(t, m.UpdatedAt.UTC(), got.UpdatedAt.UTC())
}

// TestMemoryRepo_Get_NotFound verifies that ErrNotFound is returned for a missing memory ID.
func TestMemoryRepo_Get_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	_, err := repo.Get(ctx, "default", "no-such-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

// TestMemoryRepo_Update verifies that updating a memory overwrites the stored record.
func TestMemoryRepo_Update(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	m := newTestMemory("mem-1", "default", "Original Title", nil, domain.MemorySourceManual)
	require.NoError(t, repo.Save(ctx, m))

	m.Title = "Updated Title"
	require.NoError(t, repo.Update(ctx, m))

	got, err := repo.Get(ctx, "default", "mem-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", got.Title)
}

// TestMemoryRepo_Delete verifies that a deleted memory returns ErrNotFound on subsequent Get.
func TestMemoryRepo_Delete(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	m := newTestMemory("mem-1", "default", "To Delete", nil, domain.MemorySourceManual)
	require.NoError(t, repo.Save(ctx, m))

	require.NoError(t, repo.Delete(ctx, "default", "mem-1"))

	_, err := repo.Get(ctx, "default", "mem-1")
	assert.ErrorIs(t, err, ErrNotFound)
}

// TestMemoryRepo_List_All verifies that saving 3 memories and listing all returns all 3.
func TestMemoryRepo_List_All(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	for i, title := range []string{"Alpha", "Beta", "Gamma"} {
		m := newTestMemory(
			"mem-"+string(rune('a'+i)),
			"default",
			title,
			[]string{"all"},
			domain.MemorySourceManual,
		)
		require.NoError(t, repo.Save(ctx, m))
	}

	memories, total, err := repo.List(ctx, "default", memoryuc.Filter{}, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, memories, 3)
}

// TestMemoryRepo_List_FilterTags verifies that tag filtering uses AND semantics
// and only returns memories containing all required tags.
func TestMemoryRepo_List_FilterTags(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, newTestMemory("m1", "s", "T1", []string{"go", "tdd"}, domain.MemorySourceManual)))
	require.NoError(t, repo.Save(ctx, newTestMemory("m2", "s", "T2", []string{"go"}, domain.MemorySourceManual)))
	require.NoError(t, repo.Save(ctx, newTestMemory("m3", "s", "T3", []string{"python"}, domain.MemorySourceManual)))

	// Only m1 has both "go" and "tdd".
	memories, total, err := repo.List(ctx, "s", memoryuc.Filter{Tags: []string{"go", "tdd"}}, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, memories, 1)
	assert.Equal(t, "m1", memories[0].ID)
}

// TestMemoryRepo_List_FilterSource verifies that filtering by source returns only matching records.
func TestMemoryRepo_List_FilterSource(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, newTestMemory("m1", "s", "T1", nil, domain.MemorySourceManual)))
	require.NoError(t, repo.Save(ctx, newTestMemory("m2", "s", "T2", nil, domain.MemorySourceAgent)))
	require.NoError(t, repo.Save(ctx, newTestMemory("m3", "s", "T3", nil, domain.MemorySourceAgent)))

	src := domain.MemorySourceAgent
	memories, total, err := repo.List(ctx, "s", memoryuc.Filter{Source: &src}, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, memories, 2)
}

// TestMemoryRepo_List_Pagination verifies that offset and limit slice the result correctly
// and that total reflects the full count of matching records.
func TestMemoryRepo_List_Pagination(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewMemoryRepo(fs)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		m := newTestMemory(
			"mem-"+string(rune('0'+i)),
			"s",
			"Title",
			nil,
			domain.MemorySourceManual,
		)
		require.NoError(t, repo.Save(ctx, m))
	}

	memories, total, err := repo.List(ctx, "s", memoryuc.Filter{}, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, memories, 2)
}

// TestChunkRepo_SaveAndGet verifies that a saved chunk array can be read back intact.
func TestChunkRepo_SaveAndGet(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewChunkRepo(fs)
	ctx := context.Background()

	chunks := []*domain.MemoryChunk{
		{ChunkID: "c1", MemoryID: "m1", Store: "s", ChunkIndex: 0, Content: "first chunk"},
		{ChunkID: "c2", MemoryID: "m1", Store: "s", ChunkIndex: 1, Content: "second chunk"},
	}

	require.NoError(t, repo.SaveChunks(ctx, "s", "m1", chunks))

	got, err := repo.GetChunks(ctx, "s", "m1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "c1", got[0].ChunkID)
	assert.Equal(t, "first chunk", got[0].Content)
	assert.Equal(t, "c2", got[1].ChunkID)
	assert.Equal(t, "second chunk", got[1].Content)
}

// TestChunkRepo_GetChunks_NotFound verifies that GetChunks returns an empty slice
// (not an error) when no chunk file exists for the given memory.
func TestChunkRepo_GetChunks_NotFound(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewChunkRepo(fs)
	ctx := context.Background()

	chunks, err := repo.GetChunks(ctx, "s", "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

// TestChunkRepo_Delete verifies that deleting chunks causes GetChunks to return an empty slice.
func TestChunkRepo_Delete(t *testing.T) {
	fs := NewFileStore(t.TempDir())
	repo := NewChunkRepo(fs)
	ctx := context.Background()

	chunks := []*domain.MemoryChunk{
		{ChunkID: "c1", MemoryID: "m1", Store: "s", ChunkIndex: 0, Content: "chunk"},
	}
	require.NoError(t, repo.SaveChunks(ctx, "s", "m1", chunks))
	require.NoError(t, repo.DeleteChunks(ctx, "s", "m1"))

	got, err := repo.GetChunks(ctx, "s", "m1")
	require.NoError(t, err)
	assert.Empty(t, got)
}
