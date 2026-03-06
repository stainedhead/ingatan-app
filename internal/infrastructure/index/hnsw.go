// Package index provides vector and keyword index wrappers for ingatan.
package index

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coder/hnsw"
	"github.com/stainedhead/ingatan/internal/domain"
)

// HNSWIndex wraps coder/hnsw with per-index mutex safety and chunk ID mapping.
// coder/hnsw uses uint32 node keys; this index maintains a bidirectional mapping
// between chunk IDs (string) and HNSW node keys.
type HNSWIndex struct {
	mu         sync.RWMutex
	graph      *hnsw.Graph[uint32]
	dims       int
	keyToChunk map[uint32]string // HNSW node key → chunk ID
	chunkToKey map[string]uint32 // chunk ID → HNSW node key
	nextKey    uint32
}

// NewHNSWIndex creates an empty HNSWIndex for vectors of the given dimensionality.
func NewHNSWIndex(dims int) *HNSWIndex {
	return &HNSWIndex{
		graph:      hnsw.NewGraph[uint32](),
		dims:       dims,
		keyToChunk: make(map[uint32]string),
		chunkToKey: make(map[string]uint32),
	}
}

// Upsert adds or updates a chunk's vector in the index.
// If a vector for chunkID already exists, it is replaced.
func (h *HNSWIndex) Upsert(chunkID string, vector []float32) error {
	if len(vector) != h.dims {
		return fmt.Errorf("vector dimension mismatch: got %d, want %d", len(vector), h.dims)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove existing entry for this chunk if present.
	if existingKey, ok := h.chunkToKey[chunkID]; ok {
		h.graph.Delete(existingKey)
		delete(h.keyToChunk, existingKey)
		delete(h.chunkToKey, chunkID)
	}

	key := h.nextKey
	h.nextKey++

	h.graph.Add(hnsw.MakeNode(key, vector))
	h.keyToChunk[key] = chunkID
	h.chunkToKey[chunkID] = key
	return nil
}

// Search finds the topK nearest neighbours to query. Concurrent-safe for reads.
// Results are ordered closest-first. Score is rank-based: 1/(rank+1).
// A recover() guard is applied around the underlying coder/hnsw call because the library
// can panic when the graph entry point becomes nil after a delete on a small graph.
func (h *HNSWIndex) Search(query []float32, topK int) (results []domain.VectorSearchResult, err error) {
	if len(query) != h.dims {
		return nil, fmt.Errorf("query dimension mismatch: got %d, want %d", len(query), h.dims)
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keyToChunk) == 0 {
		return nil, nil
	}

	if topK > len(h.keyToChunk) {
		topK = len(h.keyToChunk)
	}

	// Guard against coder/hnsw panicking on graphs with deleted entry points.
	var nodes []hnsw.Node[uint32]
	func() {
		defer func() { recover() }() //nolint:errcheck
		nodes = h.graph.Search(query, topK)
	}()
	results = make([]domain.VectorSearchResult, 0, len(nodes))
	for rank, n := range nodes {
		chunkID, ok := h.keyToChunk[n.Key]
		if !ok {
			continue
		}
		results = append(results, domain.VectorSearchResult{
			ChunkID: chunkID,
			Score:   1.0 / float32(rank+1),
		})
	}
	return results, nil
}

// Delete removes a chunk's vector from the index.
func (h *HNSWIndex) Delete(chunkID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	key, ok := h.chunkToKey[chunkID]
	if !ok {
		return nil
	}
	h.graph.Delete(key)
	delete(h.keyToChunk, key)
	delete(h.chunkToKey, chunkID)
	return nil
}

// hnswState is the serializable ID mapping for gob encoding.
// The HNSW graph itself is serialized separately via Export/Import.
type hnswState struct {
	Dims       int
	KeyToChunk map[uint32]string
	ChunkToKey map[string]uint32
	NextKey    uint32
}

// Save serializes the index to storePath/hnsw/ — ID map via gob, graph via hnsw.Export.
func (h *HNSWIndex) Save(storePath string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	dir := filepath.Join(storePath, "hnsw")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// Save ID map.
	mapFile, err := os.Create(filepath.Join(dir, "ids.gob"))
	if err != nil {
		return err
	}
	state := hnswState{
		Dims:       h.dims,
		KeyToChunk: h.keyToChunk,
		ChunkToKey: h.chunkToKey,
		NextKey:    h.nextKey,
	}
	if err := gob.NewEncoder(mapFile).Encode(state); err != nil {
		_ = mapFile.Close()
		return err
	}
	if err := mapFile.Close(); err != nil {
		return err
	}

	// Save HNSW graph.
	graphFile, err := os.Create(filepath.Join(dir, "graph.bin"))
	if err != nil {
		return err
	}
	if err := h.graph.Export(graphFile); err != nil {
		_ = graphFile.Close()
		return err
	}
	return graphFile.Close()
}

// Load restores the index state from storePath/hnsw/.
// Returns nil without error if no saved state exists (fresh store).
func (h *HNSWIndex) Load(storePath string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	dir := filepath.Join(storePath, "hnsw")
	mapPath := filepath.Join(dir, "ids.gob")
	graphPath := filepath.Join(dir, "graph.bin")

	if _, err := os.Stat(mapPath); errors.Is(err, os.ErrNotExist) {
		return nil // No saved state; fresh index.
	}

	mapFile, err := os.Open(mapPath)
	if err != nil {
		return err
	}
	var state hnswState
	if err := gob.NewDecoder(mapFile).Decode(&state); err != nil {
		_ = mapFile.Close()
		return err
	}
	_ = mapFile.Close()

	graphFile, err := os.Open(graphPath)
	if err != nil {
		return err
	}
	g := hnsw.NewGraph[uint32]()
	if err := g.Import(bufio.NewReader(graphFile)); err != nil {
		_ = graphFile.Close()
		return err
	}
	_ = graphFile.Close()

	h.graph = g
	h.dims = state.Dims
	h.keyToChunk = state.KeyToChunk
	h.chunkToKey = state.ChunkToKey
	h.nextKey = state.NextKey
	return nil
}

// Len returns the number of vectors currently indexed.
func (h *HNSWIndex) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.keyToChunk)
}
