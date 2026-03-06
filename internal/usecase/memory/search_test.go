package memory

import (
	"context"
	"testing"

	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockVectorIndex struct {
	upsertCalled int
	deleteCalled int
	searchResult []domain.VectorSearchResult
	searchErr    error
}

func (m *mockVectorIndex) Upsert(_, _ string, _ []float32) error {
	m.upsertCalled++
	return nil
}

func (m *mockVectorIndex) Search(_ string, _ []float32, _ int) ([]domain.VectorSearchResult, error) {
	return m.searchResult, m.searchErr
}

func (m *mockVectorIndex) Delete(_, _ string) error {
	m.deleteCalled++
	return nil
}

type mockKeywordIndex struct {
	addCalled    int
	deleteCalled int
	searchResult []domain.KeywordSearchResult
	searchErr    error
}

func (m *mockKeywordIndex) Add(_, _, _ string) error {
	m.addCalled++
	return nil
}

func (m *mockKeywordIndex) Search(_, _ string, _ int) ([]domain.KeywordSearchResult, error) {
	return m.searchResult, m.searchErr
}

func (m *mockKeywordIndex) Delete(_, _ string) error {
	m.deleteCalled++
	return nil
}

// --- Helpers ---

// newSearchTestService creates a service wired with all mocks including indexes.
func newSearchTestService(t *testing.T, vectorIdx VectorIndex, keywordIdx KeywordIndex) (Service, *mockRepo, *mockChunkRepo, *mockEmbedder) {
	t.Helper()
	repo := newMockRepo()
	cr := newMockChunkRepo()
	emb := &mockEmbedder{}
	svc := NewService(repo, cr, &mockChunker{}, emb, vectorIdx, keywordIdx, nil, nil, IngestOptions{}, nil)
	return svc, repo, cr, emb
}

// seedMemory creates a memory with chunks in the mock repos and returns the memory ID and chunk ID.
func seedMemory(t *testing.T, repo *mockRepo, cr *mockChunkRepo, store, content string, tags []string, vector []float32) (string, string) {
	t.Helper()
	memID := generateID()
	chunkID := generateID()

	mem := &domain.Memory{
		ID:      memID,
		Store:   store,
		Title:   autoTitle(content),
		Content: content,
		Tags:    tags,
		Source:  domain.MemorySourceManual,
	}
	repo.memories[store+"/"+memID] = mem

	chunk := &domain.MemoryChunk{
		ChunkID:    chunkID,
		MemoryID:   memID,
		Store:      store,
		ChunkIndex: 0,
		Content:    content,
		Vector:     vector,
	}
	cr.chunks[store+"/"+memID] = []*domain.MemoryChunk{chunk}

	return memID, chunkID
}

// --- Tests ---

// TestSearch_Keyword tests keyword-only search.
func TestSearch_Keyword(t *testing.T) {
	kwIdx := &mockKeywordIndex{}
	svc, repo, cr, _ := newSearchTestService(t, nil, kwIdx)

	memID, chunkID := seedMemory(t, repo, cr, "default", "golang concurrency patterns", nil, nil)

	kwIdx.searchResult = []domain.KeywordSearchResult{
		{ChunkID: chunkID, Score: 2.5},
	}

	resp, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "concurrency",
		Mode:  SearchModeKeyword,
		TopK:  5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memID, resp.Results[0].Memory.ID)
	assert.Greater(t, resp.Results[0].Score, 0.0)
	assert.Equal(t, 2.5, resp.Results[0].ScoreComponents.Keyword)
}

// TestSearch_Semantic tests semantic-only search.
func TestSearch_Semantic(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	svc, repo, cr, _ := newSearchTestService(t, vecIdx, nil)

	memID, chunkID := seedMemory(t, repo, cr, "default", "machine learning basics", nil, []float32{0.1, 0.2, 0.3})

	vecIdx.searchResult = []domain.VectorSearchResult{
		{ChunkID: chunkID, Score: 0.95},
	}

	resp, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "deep learning",
		Mode:  SearchModeSemantic,
		TopK:  5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, memID, resp.Results[0].Memory.ID)
	assert.InDelta(t, 0.95, resp.Results[0].ScoreComponents.Semantic, 0.01)
}

// TestSearch_Hybrid tests hybrid RRF search.
func TestSearch_Hybrid(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	kwIdx := &mockKeywordIndex{}
	svc, repo, cr, _ := newSearchTestService(t, vecIdx, kwIdx)

	memID1, chunkID1 := seedMemory(t, repo, cr, "default", "rust ownership model", nil, []float32{0.1, 0.2, 0.3})
	memID2, chunkID2 := seedMemory(t, repo, cr, "default", "go garbage collector", nil, []float32{0.4, 0.5, 0.6})

	// chunkID1 appears in both semantic and keyword results (should rank higher via RRF).
	vecIdx.searchResult = []domain.VectorSearchResult{
		{ChunkID: chunkID1, Score: 0.9},
		{ChunkID: chunkID2, Score: 0.7},
	}
	kwIdx.searchResult = []domain.KeywordSearchResult{
		{ChunkID: chunkID1, Score: 3.0},
	}

	resp, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "memory management",
		Mode:  SearchModeHybrid,
		TopK:  10,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 2)
	// mem1 should be ranked first since it appears in both result sets.
	assert.Equal(t, memID1, resp.Results[0].Memory.ID)
	assert.Equal(t, memID2, resp.Results[1].Memory.ID)
	// First result should have a higher RRF score.
	assert.Greater(t, resp.Results[0].Score, resp.Results[1].Score)
}

// TestSearch_EmptyQuery tests search with empty query returns error.
func TestSearch_EmptyQuery(t *testing.T) {
	svc, _, _, _ := newSearchTestService(t, nil, nil)

	_, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

// TestSearch_TagFilter tests that tag filtering works on results.
func TestSearch_TagFilter(t *testing.T) {
	kwIdx := &mockKeywordIndex{}
	svc, repo, cr, _ := newSearchTestService(t, nil, kwIdx)

	_, chunkID1 := seedMemory(t, repo, cr, "default", "tagged content one", []string{"go", "tutorial"}, nil)
	_, chunkID2 := seedMemory(t, repo, cr, "default", "tagged content two", []string{"rust"}, nil)

	kwIdx.searchResult = []domain.KeywordSearchResult{
		{ChunkID: chunkID1, Score: 2.0},
		{ChunkID: chunkID2, Score: 1.5},
	}

	resp, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "tagged content",
		Mode:  SearchModeKeyword,
		Tags:  []string{"go"},
		TopK:  10,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	// Only mem1 has the "go" tag.
	require.Len(t, resp.Results, 1)
	assert.Equal(t, []string{"go", "tutorial"}, resp.Results[0].Memory.Tags)
}

// TestSimilar_NoVector tests Similar when chunks have no vectors.
func TestSimilar_NoVector(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	svc, repo, cr, _ := newSearchTestService(t, vecIdx, nil)

	memID, _ := seedMemory(t, repo, cr, "default", "no vector content", nil, nil)

	resp, err := svc.Similar(context.Background(), SimilarRequest{
		Store:    "default",
		MemoryID: memID,
		TopK:     5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Results)
}

// TestSimilar_WithVector tests Similar with vector-enabled chunks.
func TestSimilar_WithVector(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	svc, repo, cr, _ := newSearchTestService(t, vecIdx, nil)

	// Source memory.
	sourceID, _ := seedMemory(t, repo, cr, "default", "source memory", nil, []float32{0.1, 0.2, 0.3})

	// Similar memory.
	similarID, similarChunkID := seedMemory(t, repo, cr, "default", "similar memory", nil, []float32{0.15, 0.25, 0.35})

	vecIdx.searchResult = []domain.VectorSearchResult{
		{ChunkID: similarChunkID, Score: 0.92},
	}

	resp, err := svc.Similar(context.Background(), SimilarRequest{
		Store:    "default",
		MemoryID: sourceID,
		TopK:     5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, similarID, resp.Results[0].Memory.ID)
	assert.Greater(t, resp.Results[0].Score, 0.0)
}

// TestSimilar_ExcludesSource tests that Similar excludes the source memory from results.
func TestSimilar_ExcludesSource(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	svc, repo, cr, _ := newSearchTestService(t, vecIdx, nil)

	sourceID, sourceChunkID := seedMemory(t, repo, cr, "default", "source memory", nil, []float32{0.1, 0.2, 0.3})

	// Vector search returns the source memory's own chunk -- should be excluded.
	vecIdx.searchResult = []domain.VectorSearchResult{
		{ChunkID: sourceChunkID, Score: 1.0},
	}

	resp, err := svc.Similar(context.Background(), SimilarRequest{
		Store:    "default",
		MemoryID: sourceID,
		TopK:     5,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Results)
}

// TestSave_IndexesChunks tests that Save calls index Add/Upsert.
func TestSave_IndexesChunks(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	kwIdx := &mockKeywordIndex{}
	svc, _, _, _ := newSearchTestService(t, vecIdx, kwIdx)

	_, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "content to index",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, vecIdx.upsertCalled, "vector index should have been called for the chunk")
	assert.Equal(t, 1, kwIdx.addCalled, "keyword index should have been called for the chunk")
}

// TestDelete_RemovesFromIndex tests that Delete calls index Delete.
func TestDelete_RemovesFromIndex(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	kwIdx := &mockKeywordIndex{}
	svc, _, _, _ := newSearchTestService(t, vecIdx, kwIdx)

	mem, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "content to delete",
	})
	require.NoError(t, err)

	// Reset counters after save.
	vecIdx.deleteCalled = 0
	kwIdx.deleteCalled = 0

	err = svc.Delete(context.Background(), "default", mem.ID, principal())

	require.NoError(t, err)
	assert.Equal(t, 1, vecIdx.deleteCalled, "vector index delete should have been called")
	assert.Equal(t, 1, kwIdx.deleteCalled, "keyword index delete should have been called")
}

// TestUpdate_ReIndexesChunks tests that Update with content change re-indexes.
func TestUpdate_ReIndexesChunks(t *testing.T) {
	vecIdx := &mockVectorIndex{}
	kwIdx := &mockKeywordIndex{}
	svc, _, _, _ := newSearchTestService(t, vecIdx, kwIdx)

	mem, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "original content",
	})
	require.NoError(t, err)

	// Reset counters after initial save.
	vecIdx.upsertCalled = 0
	vecIdx.deleteCalled = 0
	kwIdx.addCalled = 0
	kwIdx.deleteCalled = 0

	newContent := "updated content"
	_, err = svc.Update(context.Background(), UpdateRequest{
		Store:    "default",
		MemoryID: mem.ID,
		Content:  &newContent,
	})

	require.NoError(t, err)
	// Old chunks removed from index.
	assert.Equal(t, 1, vecIdx.deleteCalled, "old chunk should be removed from vector index")
	assert.Equal(t, 1, kwIdx.deleteCalled, "old chunk should be removed from keyword index")
	// New chunks added to index.
	assert.Equal(t, 1, vecIdx.upsertCalled, "new chunk should be upserted to vector index")
	assert.Equal(t, 1, kwIdx.addCalled, "new chunk should be added to keyword index")
}

// TestSearch_EmptyStore tests search with empty store returns error.
func TestSearch_EmptyStore(t *testing.T) {
	svc, _, _, _ := newSearchTestService(t, nil, nil)

	_, err := svc.Search(context.Background(), SearchRequest{
		Store: "",
		Query: "test",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

// TestSearch_NoIndexes tests search with no indexes returns empty results.
func TestSearch_NoIndexes(t *testing.T) {
	svc, _, _, _ := newSearchTestService(t, nil, nil)

	resp, err := svc.Search(context.Background(), SearchRequest{
		Store: "default",
		Query: "test",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Results)
}

// TestSimilar_MemoryNotFound tests Similar with non-existent memory returns error.
func TestSimilar_MemoryNotFound(t *testing.T) {
	svc, _, _, _ := newSearchTestService(t, nil, nil)

	_, err := svc.Similar(context.Background(), SimilarRequest{
		Store:    "default",
		MemoryID: "nonexistent",
	})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}
