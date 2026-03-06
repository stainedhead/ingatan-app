package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/stainedhead/ingatan/internal/domain"
)

// HNSWStore manages per-store HNSW vector indexes.
// It lazily loads each store's index on first access and auto-saves after writes.
// Safe for concurrent use.
type HNSWStore struct {
	baseDir string
	dims    int
	mu      sync.RWMutex
	indexes map[string]*HNSWIndex
}

// NewHNSWStore creates an HNSWStore rooted at baseDir with vectors of the given dimensionality.
func NewHNSWStore(baseDir string, dims int) *HNSWStore {
	return &HNSWStore{
		baseDir: baseDir,
		dims:    dims,
		indexes: make(map[string]*HNSWIndex),
	}
}

// Upsert adds or updates a chunk's vector in the named store's index.
// The index is persisted to disk after the write.
func (s *HNSWStore) Upsert(store, chunkID string, vector []float32) error {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return err
	}
	if err := idx.Upsert(chunkID, vector); err != nil {
		return err
	}
	return idx.Save(s.storePath(store))
}

// Search finds the topK nearest neighbours in the named store's index.
func (s *HNSWStore) Search(store string, query []float32, topK int) ([]domain.VectorSearchResult, error) {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return nil, err
	}
	return idx.Search(query, topK)
}

// Delete removes a chunk from the named store's index.
// The index is persisted to disk after the write.
func (s *HNSWStore) Delete(store, chunkID string) error {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return err
	}
	if err := idx.Delete(chunkID); err != nil {
		return err
	}
	return idx.Save(s.storePath(store))
}

// getOrLoad returns the HNSWIndex for store, loading it from disk if needed.
func (s *HNSWStore) getOrLoad(store string) (*HNSWIndex, error) {
	s.mu.RLock()
	idx, ok := s.indexes[store]
	s.mu.RUnlock()
	if ok {
		return idx, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock.
	if idx, ok = s.indexes[store]; ok {
		return idx, nil
	}

	idx = NewHNSWIndex(s.dims)
	sp := s.storePath(store)
	if err := os.MkdirAll(sp, 0o700); err != nil {
		return nil, fmt.Errorf("create store dir %s: %w", sp, err)
	}
	if err := idx.Load(sp); err != nil {
		return nil, fmt.Errorf("load hnsw for store %s: %w", store, err)
	}
	s.indexes[store] = idx
	return idx, nil
}

// storePath returns the filesystem path for the named store under baseDir.
func (s *HNSWStore) storePath(store string) string {
	return filepath.Join(s.baseDir, "stores", store)
}
