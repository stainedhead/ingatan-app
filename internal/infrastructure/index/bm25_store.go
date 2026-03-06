package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/stainedhead/ingatan/internal/domain"
)

// BM25Store manages per-store BM25 keyword indexes.
// It lazily loads each store's index on first access and auto-saves after writes.
// Safe for concurrent use.
type BM25Store struct {
	baseDir string
	mu      sync.RWMutex
	indexes map[string]*BM25Index
}

// NewBM25Store creates a BM25Store rooted at baseDir.
func NewBM25Store(baseDir string) *BM25Store {
	return &BM25Store{
		baseDir: baseDir,
		indexes: make(map[string]*BM25Index),
	}
}

// Add indexes chunkID with content in the named store's BM25 index.
// The index is persisted after the write.
func (s *BM25Store) Add(store, chunkID, content string) error {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return err
	}
	if err := idx.Add(chunkID, content); err != nil {
		return err
	}
	return saveBM25(idx, s.gobPath(store))
}

// Search returns the top-K BM25 results for query in the named store.
func (s *BM25Store) Search(store, query string, topK int) ([]domain.KeywordSearchResult, error) {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return nil, err
	}
	return idx.Search(query, topK)
}

// Delete removes chunkID from the named store's BM25 index.
// The index is persisted after the write.
func (s *BM25Store) Delete(store, chunkID string) error {
	idx, err := s.getOrLoad(store)
	if err != nil {
		return err
	}
	if err := idx.Delete(chunkID); err != nil {
		return err
	}
	return saveBM25(idx, s.gobPath(store))
}

// getOrLoad returns the BM25Index for store, loading from disk if needed.
func (s *BM25Store) getOrLoad(store string) (*BM25Index, error) {
	s.mu.RLock()
	idx, ok := s.indexes[store]
	s.mu.RUnlock()
	if ok {
		return idx, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if idx, ok = s.indexes[store]; ok {
		return idx, nil
	}

	sp := s.storeDirPath(store)
	if err := os.MkdirAll(sp, 0o700); err != nil {
		return nil, fmt.Errorf("create store dir %s: %w", sp, err)
	}
	loaded, err := loadBM25(s.gobPath(store))
	if err != nil {
		return nil, fmt.Errorf("load bm25 for store %s: %w", store, err)
	}
	s.indexes[store] = loaded
	return loaded, nil
}

func (s *BM25Store) storeDirPath(store string) string {
	return filepath.Join(s.baseDir, "stores", store)
}

func (s *BM25Store) gobPath(store string) string {
	return filepath.Join(s.baseDir, "stores", store, "bm25.gob")
}
