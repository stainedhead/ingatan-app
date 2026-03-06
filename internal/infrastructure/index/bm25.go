// Package index provides vector and keyword index wrappers for ingatan.
package index

import (
	"bytes"
	"encoding/gob"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/stainedhead/ingatan/internal/domain"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// BM25Index is an in-process BM25 keyword index for a single store.
// All methods are protected by mu. Use GobEncode/GobDecode for persistence.
type BM25Index struct {
	mu          sync.Mutex
	docs        map[string][]string // chunkID -> tokens
	df          map[string]int      // term -> document frequency
	n           int                 // total document count
	totalLength int                 // sum of all doc lengths
}

// NewBM25Index creates an empty BM25Index.
func NewBM25Index() *BM25Index {
	return &BM25Index{
		docs: make(map[string][]string),
		df:   make(map[string]int),
	}
}

// Add indexes chunkID with the given content.
// If chunkID already exists it is replaced.
func (b *BM25Index) Add(chunkID, content string) error {
	tokens := tokenize(content)
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.docs[chunkID]; exists {
		b.removeDocLocked(chunkID)
	}
	b.docs[chunkID] = tokens
	b.n++
	b.totalLength += len(tokens)
	seen := make(map[string]struct{})
	for _, t := range tokens {
		if _, ok := seen[t]; !ok {
			b.df[t]++
			seen[t] = struct{}{}
		}
	}
	return nil
}

// Delete removes chunkID from the index. No-op if chunkID is absent.
func (b *BM25Index) Delete(chunkID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.docs[chunkID]; !exists {
		return nil
	}
	b.removeDocLocked(chunkID)
	return nil
}

// removeDocLocked removes chunkID without acquiring the mutex. Caller must hold mu.
func (b *BM25Index) removeDocLocked(chunkID string) {
	tokens := b.docs[chunkID]
	b.totalLength -= len(tokens)
	b.n--
	seen := make(map[string]struct{})
	for _, t := range tokens {
		if _, ok := seen[t]; !ok {
			b.df[t]--
			if b.df[t] <= 0 {
				delete(b.df, t)
			}
			seen[t] = struct{}{}
		}
	}
	delete(b.docs, chunkID)
}

// Search returns the top-K chunk IDs ranked by BM25 score for query.
// Returns nil without error when the index is empty or topK == 0.
func (b *BM25Index) Search(query string, topK int) ([]domain.KeywordSearchResult, error) {
	if topK <= 0 {
		return nil, nil
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.n == 0 {
		return nil, nil
	}

	avgdl := float64(b.totalLength) / float64(b.n)
	scores := make(map[string]float64)

	for _, term := range qTokens {
		df := float64(b.df[term])
		if df == 0 {
			continue
		}
		idf := math.Log((float64(b.n)-df+0.5)/(df+0.5) + 1)
		for chunkID, tokens := range b.docs {
			tf := 0
			for _, t := range tokens {
				if t == term {
					tf++
				}
			}
			if tf == 0 {
				continue
			}
			dl := float64(len(tokens))
			score := idf * float64(tf) * (bm25K1 + 1) / (float64(tf) + bm25K1*(1-bm25B+bm25B*dl/avgdl))
			scores[chunkID] += score
		}
	}

	results := make([]domain.KeywordSearchResult, 0, len(scores))
	for id, sc := range scores {
		results = append(results, domain.KeywordSearchResult{ChunkID: id, Score: sc})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// Len returns the number of documents in the index.
func (b *BM25Index) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.n
}

// bm25State is the gob-serializable snapshot of BM25Index.
type bm25State struct {
	Docs        map[string][]string
	DF          map[string]int
	N           int
	TotalLength int
}

// GobEncode implements encoding.GobEncoder.
func (b *BM25Index) GobEncode() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := bm25State{
		Docs:        b.docs,
		DF:          b.df,
		N:           b.n,
		TotalLength: b.totalLength,
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(state); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements encoding.GobDecoder.
func (b *BM25Index) GobDecode(data []byte) error {
	var state bm25State
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&state); err != nil {
		return err
	}
	b.docs = state.Docs
	b.df = state.DF
	b.n = state.N
	b.totalLength = state.TotalLength
	return nil
}

// tokenize splits text into lowercase word tokens.
func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// saveBM25 persists a BM25Index to path using gob encoding.
func saveBM25(idx *BM25Index, path string) error {
	data, err := idx.GobEncode()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// loadBM25 loads a BM25Index from a gob file. Returns a new empty index if the
// file does not exist.
func loadBM25(path string) (*BM25Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewBM25Index(), nil
		}
		return nil, err
	}
	idx := NewBM25Index()
	if err := idx.GobDecode(data); err != nil {
		return nil, err
	}
	return idx, nil
}
