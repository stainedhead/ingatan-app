package index

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBM25Index_AddAndSearch(t *testing.T) {
	idx := NewBM25Index()

	require.NoError(t, idx.Add("chunk-1", "the quick brown fox jumps over the lazy dog"))
	require.NoError(t, idx.Add("chunk-2", "golang is a statically typed compiled language"))
	require.NoError(t, idx.Add("chunk-3", "the fox ran quickly through the forest"))

	assert.Equal(t, 3, idx.Len())

	results, err := idx.Search("fox", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	// "chunk-1" and "chunk-3" both contain "fox"; both should be ranked above "chunk-2"
	for _, r := range results {
		assert.NotEqual(t, "chunk-2", r.ChunkID)
	}
}

func TestBM25Index_Delete(t *testing.T) {
	idx := NewBM25Index()
	require.NoError(t, idx.Add("chunk-1", "quick brown fox"))
	require.NoError(t, idx.Add("chunk-2", "lazy dog"))

	require.NoError(t, idx.Delete("chunk-1"))
	assert.Equal(t, 1, idx.Len())

	results, err := idx.Search("fox", 5)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "chunk-1", r.ChunkID)
	}
}

func TestBM25Index_Delete_NonExistent(t *testing.T) {
	idx := NewBM25Index()
	assert.NoError(t, idx.Delete("nonexistent"))
}

func TestBM25Index_Search_Empty(t *testing.T) {
	idx := NewBM25Index()
	results, err := idx.Search("fox", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBM25Index_GobRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/bm25.gob"

	idx := NewBM25Index()
	require.NoError(t, idx.Add("chunk-1", "the quick brown fox"))
	require.NoError(t, idx.Add("chunk-2", "lazy dog"))

	require.NoError(t, saveBM25(idx, path))

	idx2, err := loadBM25(path)
	require.NoError(t, err)
	assert.Equal(t, 2, idx2.Len())

	results, err := idx2.Search("fox", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "chunk-1", results[0].ChunkID)
}

func TestBM25Index_LoadMissing(t *testing.T) {
	idx, err := loadBM25("/nonexistent/path/bm25.gob")
	require.NoError(t, err)
	assert.Equal(t, 0, idx.Len())
}

func TestBM25Index_Upsert(t *testing.T) {
	idx := NewBM25Index()
	require.NoError(t, idx.Add("chunk-1", "original content"))
	require.NoError(t, idx.Add("chunk-1", "updated content with new terms"))
	assert.Equal(t, 1, idx.Len()) // Still just 1 doc

	results, err := idx.Search("updated", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "chunk-1", results[0].ChunkID)
}
