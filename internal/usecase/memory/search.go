package memory

import (
	"context"
	"fmt"
	"sort"

	"github.com/stainedhead/ingatan/internal/domain"
)

const rrfK = 60 // standard RRF constant

// Search performs memory search using the configured mode (hybrid, semantic, or keyword).
// Results are ranked by relevance and deduplicated to one result per memory.
func (s *serviceImpl) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store must not be empty")
	}
	if req.Query == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "query must not be empty")
	}
	if err := s.requireRead(ctx, req.Store, req.Principal); err != nil {
		return nil, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}

	// Resolve effective mode based on available indexes.
	mode := s.effectiveMode(req.Mode)

	// Semantic search (HNSW).
	var semanticResults []domain.VectorSearchResult
	if (mode == SearchModeHybrid || mode == SearchModeSemantic) && s.embedder != nil && s.vectorIdx != nil {
		vecs, err := s.embedder.Embed(ctx, []string{req.Query})
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		if len(vecs) > 0 {
			semanticResults, err = s.vectorIdx.Search(req.Store, vecs[0], topK*3)
			if err != nil {
				return nil, fmt.Errorf("vector search: %w", err)
			}
		}
	}

	// Keyword search (BM25).
	var keywordResults []domain.KeywordSearchResult
	if (mode == SearchModeHybrid || mode == SearchModeKeyword) && s.keywordIdx != nil {
		var err error
		keywordResults, err = s.keywordIdx.Search(req.Store, req.Query, topK*3)
		if err != nil {
			return nil, fmt.Errorf("keyword search: %w", err)
		}
	}

	// Fuse results.
	chunkScores := s.fuseRRF(semanticResults, keywordResults)
	if len(chunkScores) == 0 {
		return &SearchResponse{}, nil
	}

	// Map chunk IDs to memory IDs, keeping best score per memory.
	memoryScores := s.chunkScoresToMemoryScores(ctx, req.Store, chunkScores, topK)

	// Fetch memories and build response.
	results := make([]SearchResult, 0, len(memoryScores))
	for memID, cs := range memoryScores {
		mem, err := s.repo.Get(ctx, req.Store, memID)
		if err != nil {
			continue // memory may have been deleted; skip
		}
		// Apply tag filter.
		if len(req.Tags) > 0 && !hasAllTags(mem.Tags, req.Tags) {
			continue
		}
		results = append(results, SearchResult{
			Memory:          mem,
			Score:           cs.rrf,
			ScoreComponents: ScoreComponents{Semantic: cs.semantic, Keyword: cs.keyword, RRF: cs.rrf},
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}

	return &SearchResponse{Results: results}, nil
}

// Similar finds memories similar to the given memory using its chunk vectors (centroid search).
func (s *serviceImpl) Similar(ctx context.Context, req SimilarRequest) (*SearchResponse, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store must not be empty")
	}
	if req.MemoryID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "memory_id must not be empty")
	}
	if err := s.requireRead(ctx, req.Store, req.Principal); err != nil {
		return nil, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}

	// Verify memory exists.
	if _, err := s.repo.Get(ctx, req.Store, req.MemoryID); err != nil {
		return nil, s.wrapNotFound(err, req.MemoryID)
	}

	if s.vectorIdx == nil {
		return &SearchResponse{}, nil
	}

	// Get chunks for the source memory.
	chunks, err := s.chunkRepo.GetChunks(ctx, req.Store, req.MemoryID)
	if err != nil || len(chunks) == 0 {
		return &SearchResponse{}, nil
	}

	// Compute centroid vector.
	centroid := centroidVector(chunks)
	if centroid == nil {
		return &SearchResponse{}, nil
	}

	// Search HNSW for nearest neighbours.
	vecResults, err := s.vectorIdx.Search(req.Store, centroid, topK*3+len(chunks))
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	// Map chunk IDs to memory IDs, excluding the source memory.
	memScores := make(map[string]float32)
	for i, vr := range vecResults {
		chunk, getErr := s.chunkRepo.GetChunkByID(ctx, req.Store, vr.ChunkID)
		if getErr != nil || chunk == nil || chunk.MemoryID == req.MemoryID {
			continue
		}
		score := 1.0 / float32(i+1)
		if existing, ok := memScores[chunk.MemoryID]; !ok || score > existing {
			memScores[chunk.MemoryID] = score
		}
	}

	results := make([]SearchResult, 0, len(memScores))
	for memID, score := range memScores {
		mem, getErr := s.repo.Get(ctx, req.Store, memID)
		if getErr != nil {
			continue
		}
		results = append(results, SearchResult{
			Memory:          mem,
			Score:           float64(score),
			ScoreComponents: ScoreComponents{Semantic: score},
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}

	return &SearchResponse{Results: results}, nil
}

// chunkScore holds fused scores for a single chunk.
type chunkScore struct {
	chunkID  string
	semantic float32
	keyword  float64
	rrf      float64
}

// fuseRRF combines semantic and keyword results using Reciprocal Rank Fusion.
func (s *serviceImpl) fuseRRF(semantic []domain.VectorSearchResult, keyword []domain.KeywordSearchResult) map[string]*chunkScore {
	scores := make(map[string]*chunkScore)

	for i, r := range semantic {
		cs := scoreFor(scores, r.ChunkID)
		cs.semantic = r.Score
		cs.rrf += 1.0 / float64(rrfK+i+1)
	}
	for i, r := range keyword {
		cs := scoreFor(scores, r.ChunkID)
		cs.keyword = r.Score
		cs.rrf += 1.0 / float64(rrfK+i+1)
	}
	return scores
}

func scoreFor(m map[string]*chunkScore, chunkID string) *chunkScore {
	if cs, ok := m[chunkID]; ok {
		return cs
	}
	cs := &chunkScore{chunkID: chunkID}
	m[chunkID] = cs
	return cs
}

// memoryScore holds the best chunk scores mapped to a memory.
type memoryScore struct {
	semantic float32
	keyword  float64
	rrf      float64
}

// chunkScoresToMemoryScores maps chunk-level scores to memory IDs, keeping the
// best RRF score per memory. Returns at most topK memory IDs.
func (s *serviceImpl) chunkScoresToMemoryScores(ctx context.Context, store string, chunkScores map[string]*chunkScore, topK int) map[string]memoryScore {
	// Sort chunks by RRF score descending.
	sorted := make([]*chunkScore, 0, len(chunkScores))
	for _, cs := range chunkScores {
		sorted = append(sorted, cs)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].rrf > sorted[j].rrf
	})

	memScores := make(map[string]memoryScore)
	for _, cs := range sorted {
		if len(memScores) >= topK*3 { // collect more than needed for tag filtering
			break
		}
		chunk, err := s.chunkRepo.GetChunkByID(ctx, store, cs.chunkID)
		if err != nil || chunk == nil {
			continue
		}
		if existing, ok := memScores[chunk.MemoryID]; !ok || cs.rrf > existing.rrf {
			memScores[chunk.MemoryID] = memoryScore{
				semantic: cs.semantic,
				keyword:  cs.keyword,
				rrf:      cs.rrf,
			}
		}
	}
	return memScores
}

// effectiveMode returns the search mode adjusted for what indexes are available.
func (s *serviceImpl) effectiveMode(requested SearchMode) SearchMode {
	if requested == "" {
		requested = SearchModeHybrid
	}
	hasVector := s.embedder != nil && s.vectorIdx != nil
	hasKeyword := s.keywordIdx != nil
	switch requested {
	case SearchModeSemantic:
		if !hasVector {
			return SearchModeKeyword
		}
	case SearchModeKeyword:
		if !hasKeyword {
			return SearchModeSemantic
		}
	case SearchModeHybrid:
		if !hasVector && !hasKeyword {
			return SearchModeHybrid // will return empty
		}
		if !hasVector {
			return SearchModeKeyword
		}
		if !hasKeyword {
			return SearchModeSemantic
		}
	}
	return requested
}

// centroidVector computes the mean vector of all chunks that have a non-nil vector.
// Returns nil if no vectors are available.
func centroidVector(chunks []*domain.MemoryChunk) []float32 {
	var vecs [][]float32
	for _, c := range chunks {
		if len(c.Vector) > 0 {
			vecs = append(vecs, c.Vector)
		}
	}
	if len(vecs) == 0 {
		return nil
	}
	dims := len(vecs[0])
	centroid := make([]float32, dims)
	for _, v := range vecs {
		for i, x := range v {
			centroid[i] += x
		}
	}
	n := float32(len(vecs))
	for i := range centroid {
		centroid[i] /= n
	}
	return centroid
}

// hasAllTags returns true if memTags contains every tag in required.
func hasAllTags(memTags, required []string) bool {
	tagSet := make(map[string]struct{}, len(memTags))
	for _, t := range memTags {
		tagSet[t] = struct{}{}
	}
	for _, t := range required {
		if _, ok := tagSet[t]; !ok {
			return false
		}
	}
	return true
}
