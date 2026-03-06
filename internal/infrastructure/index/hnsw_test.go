package index

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDims = 8

func makeVec(seed int64) []float32 {
	rng := rand.New(rand.NewSource(seed))
	v := make([]float32, testDims)
	for i := range v {
		v[i] = rng.Float32()
	}
	return v
}

func TestHNSWIndex_UpsertAndSearch(t *testing.T) {
	idx := NewHNSWIndex(testDims)

	vec1 := makeVec(1)
	vec2 := makeVec(99)

	require.NoError(t, idx.Upsert("chunk-1", vec1))
	require.NoError(t, idx.Upsert("chunk-2", vec2))

	assert.Equal(t, 2, idx.Len())

	results, err := idx.Search(vec1, 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "chunk-1", results[0].ChunkID)
}

func TestHNSWIndex_Delete(t *testing.T) {
	idx := NewHNSWIndex(testDims)

	require.NoError(t, idx.Upsert("chunk-1", makeVec(1)))
	require.NoError(t, idx.Upsert("chunk-2", makeVec(2)))

	require.NoError(t, idx.Delete("chunk-1"))
	assert.Equal(t, 1, idx.Len())

	results, err := idx.Search(makeVec(1), 5)
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "chunk-1", r.ChunkID)
	}
}

func TestHNSWIndex_Delete_NonExistent(t *testing.T) {
	idx := NewHNSWIndex(testDims)
	assert.NoError(t, idx.Delete("nonexistent"))
}

func TestHNSWIndex_Upsert_WrongDimension(t *testing.T) {
	idx := NewHNSWIndex(testDims)
	err := idx.Upsert("chunk-1", []float32{1, 2, 3}) // wrong dims
	assert.Error(t, err)
}

func TestHNSWIndex_Search_Empty(t *testing.T) {
	idx := NewHNSWIndex(testDims)
	results, err := idx.Search(makeVec(1), 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHNSWIndex_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	idx := NewHNSWIndex(testDims)

	vec1 := makeVec(1)
	vec2 := makeVec(2)
	require.NoError(t, idx.Upsert("chunk-1", vec1))
	require.NoError(t, idx.Upsert("chunk-2", vec2))

	require.NoError(t, idx.Save(dir))

	idx2 := NewHNSWIndex(testDims)
	require.NoError(t, idx2.Load(dir))

	assert.Equal(t, 2, idx2.Len())
	results, err := idx2.Search(vec1, 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "chunk-1", results[0].ChunkID)
}

func TestHNSWIndex_Load_NoData(t *testing.T) {
	dir := t.TempDir()
	idx := NewHNSWIndex(testDims)
	assert.NoError(t, idx.Load(dir))
	assert.Equal(t, 0, idx.Len())
}

func TestHNSWIndex_ConcurrentReads(t *testing.T) {
	idx := NewHNSWIndex(testDims)
	for i := int64(0); i < 10; i++ {
		require.NoError(t, idx.Upsert(fmt.Sprintf("chunk-%d", i), makeVec(i)))
	}

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = idx.Search(makeVec(1), 5)
		}()
	}
	wg.Wait()
}
