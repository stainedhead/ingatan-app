package memory

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/stainedhead/ingatan/internal/domain"
)

// serviceImpl is the concrete implementation of Service.
type serviceImpl struct {
	repo        Repository
	chunkRepo   ChunkRepository
	chunker     Chunker
	embedder    Embedder
	vectorIdx   VectorIndex  // optional; nil if no embedder configured
	keywordIdx  KeywordIndex // optional; nil for keyword-less deployment
	urlFetcher  URLFetcher   // optional; nil disables URL ingest
	fileReader  FileReader   // optional; nil disables file ingest
	ingestOpts  IngestOptions
	storeAccess StoreAccess // optional; nil disables per-store access checks
}

// NewService constructs a Service with the provided dependencies.
// embedder, vectorIdx, keywordIdx, urlFetcher, fileReader, and storeAccess may be nil.
func NewService(repo Repository, chunkRepo ChunkRepository, chunker Chunker, embedder Embedder, vectorIdx VectorIndex, keywordIdx KeywordIndex, urlFetcher URLFetcher, fileReader FileReader, ingestOpts IngestOptions, storeAccess StoreAccess) Service {
	return &serviceImpl{
		repo:        repo,
		chunkRepo:   chunkRepo,
		chunker:     chunker,
		embedder:    embedder,
		vectorIdx:   vectorIdx,
		keywordIdx:  keywordIdx,
		urlFetcher:  urlFetcher,
		fileReader:  fileReader,
		ingestOpts:  ingestOpts,
		storeAccess: storeAccess,
	}
}

// requireRead returns an error if the principal does not have at least reader access to the store.
// Admin principals bypass all store access checks.
// Returns nil when storeAccess is not configured (backward compatible).
func (s *serviceImpl) requireRead(ctx context.Context, storeName string, principal *domain.Principal) error {
	if s.storeAccess == nil || principal == nil {
		return nil
	}
	if principal.Role == domain.InstanceRoleAdmin {
		return nil
	}
	role, err := s.storeAccess.GetMemberRole(ctx, storeName, principal.ID)
	if err != nil {
		return err
	}
	if role == "" {
		return domain.NewAppError(domain.ErrCodeForbidden, "not a member of store: "+storeName)
	}
	return nil
}

// requireWrite returns an error if the principal does not have at least writer access to the store.
func (s *serviceImpl) requireWrite(ctx context.Context, storeName string, principal *domain.Principal) error {
	if s.storeAccess == nil || principal == nil {
		return nil
	}
	if principal.Role == domain.InstanceRoleAdmin {
		return nil
	}
	role, err := s.storeAccess.GetMemberRole(ctx, storeName, principal.ID)
	if err != nil {
		return err
	}
	if role != domain.StoreRoleOwner && role != domain.StoreRoleWriter {
		return domain.NewAppError(domain.ErrCodeForbidden, "insufficient permissions for store: "+storeName)
	}
	return nil
}

// generateID returns a random UUID v4 string.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// autoTitle generates a title from the first 60 characters of content.
func autoTitle(content string) string {
	runes := []rune(content)
	if len(runes) <= 60 {
		return string(runes)
	}
	return string(runes[:60])
}

// Save validates the request, creates a Memory, chunks and embeds its content, then
// persists both the memory and its chunks.
func (s *serviceImpl) Save(ctx context.Context, req SaveRequest) (*domain.Memory, error) {
	if req.Store == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "store must not be empty")
	}
	if req.Content == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "content must not be empty")
	}
	if err := s.requireWrite(ctx, req.Store, req.Principal); err != nil {
		return nil, err
	}

	title := req.Title
	if title == "" {
		title = autoTitle(req.Content)
	}

	source := req.Source
	if source == "" {
		source = domain.MemorySourceManual
	}

	now := time.Now().UTC()
	mem := &domain.Memory{
		ID:         generateID(),
		Store:      req.Store,
		Title:      title,
		Content:    req.Content,
		Tags:       req.Tags,
		Source:     source,
		SourceRef:  req.SourceRef,
		SourceURL:  req.SourceURL,
		SourcePath: req.SourcePath,
		Metadata:   req.Metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	chunks, err := s.buildChunks(ctx, mem.Store, mem.ID, req.Content)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, mem); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	if err := s.chunkRepo.SaveChunks(ctx, mem.Store, mem.ID, chunks); err != nil {
		return nil, fmt.Errorf("save chunks: %w", err)
	}

	// Index chunks for search.
	s.indexChunks(mem.Store, chunks)

	return mem, nil
}

// Get retrieves a single memory by store and ID.
// Returns a NOT_FOUND AppError when the memory does not exist.
func (s *serviceImpl) Get(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error) {
	if err := s.requireRead(ctx, store, principal); err != nil {
		return nil, err
	}
	mem, err := s.repo.Get(ctx, store, memoryID)
	if err != nil {
		return nil, s.wrapNotFound(err, memoryID)
	}
	return mem, nil
}

// Update applies partial updates to an existing memory.
// If content changes, chunks are re-built and re-embedded.
func (s *serviceImpl) Update(ctx context.Context, req UpdateRequest) (*domain.Memory, error) {
	if err := s.requireWrite(ctx, req.Store, req.Principal); err != nil {
		return nil, err
	}
	mem, err := s.repo.Get(ctx, req.Store, req.MemoryID)
	if err != nil {
		return nil, s.wrapNotFound(err, req.MemoryID)
	}

	contentChanged := false

	if req.Title != nil {
		mem.Title = *req.Title
	}
	if req.Content != nil && *req.Content != mem.Content {
		mem.Content = *req.Content
		contentChanged = true
	}
	if req.Tags != nil {
		mem.Tags = *req.Tags
	}
	for k, v := range req.Metadata {
		if mem.Metadata == nil {
			mem.Metadata = make(map[string]any)
		}
		mem.Metadata[k] = v
	}

	if contentChanged {
		chunks, buildErr := s.buildChunks(ctx, mem.Store, mem.ID, mem.Content)
		if buildErr != nil {
			return nil, buildErr
		}
		// Remove old chunks from indexes.
		if oldChunks, getErr := s.chunkRepo.GetChunks(ctx, mem.Store, mem.ID); getErr == nil {
			s.removeChunksFromIndex(mem.Store, oldChunks)
		}
		if delErr := s.chunkRepo.DeleteChunks(ctx, mem.Store, mem.ID); delErr != nil {
			return nil, fmt.Errorf("delete old chunks: %w", delErr)
		}
		if saveErr := s.chunkRepo.SaveChunks(ctx, mem.Store, mem.ID, chunks); saveErr != nil {
			return nil, fmt.Errorf("save new chunks: %w", saveErr)
		}
		// Re-index new chunks.
		s.indexChunks(mem.Store, chunks)
	}

	mem.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, mem); err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}

	return mem, nil
}

// Delete removes a memory and all its associated chunks.
func (s *serviceImpl) Delete(ctx context.Context, store, memoryID string, principal *domain.Principal) error {
	if err := s.requireWrite(ctx, store, principal); err != nil {
		return err
	}
	if _, err := s.repo.Get(ctx, store, memoryID); err != nil {
		return s.wrapNotFound(err, memoryID)
	}

	// Remove chunks from indexes before deletion.
	if chunks, getErr := s.chunkRepo.GetChunks(ctx, store, memoryID); getErr == nil {
		s.removeChunksFromIndex(store, chunks)
	}

	if err := s.chunkRepo.DeleteChunks(ctx, store, memoryID); err != nil {
		return fmt.Errorf("delete chunks: %w", err)
	}

	if err := s.repo.Delete(ctx, store, memoryID); err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	return nil
}

// List returns a paginated, filtered list of memories in a store.
func (s *serviceImpl) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
	if err := s.requireRead(ctx, req.Store, req.Principal); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	filter := Filter{
		Tags:   req.Tags,
		Source: req.Source,
	}

	memories, total, err := s.repo.List(ctx, req.Store, filter, limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	return &ListResponse{Memories: memories, Total: total}, nil
}

// buildChunks splits content into chunks, optionally embeds them, and returns
// a slice of MemoryChunk ready for persistence.
func (s *serviceImpl) buildChunks(ctx context.Context, store, memoryID, content string) ([]*domain.MemoryChunk, error) {
	texts, err := s.chunker.Chunk(content)
	if err != nil {
		return nil, fmt.Errorf("chunk content: %w", err)
	}

	var vectors [][]float32
	if s.embedder != nil {
		vectors, err = s.embedder.Embed(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embed chunks: %w", err)
		}
	}

	chunks := make([]*domain.MemoryChunk, len(texts))
	for i, text := range texts {
		c := &domain.MemoryChunk{
			ChunkID:    generateID(),
			MemoryID:   memoryID,
			Store:      store,
			ChunkIndex: i,
			Content:    text,
		}
		if vectors != nil && i < len(vectors) {
			c.Vector = vectors[i]
		}
		chunks[i] = c
	}

	return chunks, nil
}

// wrapNotFound converts a repository error into a domain NOT_FOUND AppError when
// the underlying error is itself already an AppError with that code; otherwise it
// wraps the raw error and returns a generic NOT_FOUND.
func (s *serviceImpl) wrapNotFound(err error, id string) error {
	var appErr *domain.AppError
	if asAppError(err, &appErr) && appErr.Code == domain.ErrCodeNotFound {
		return appErr
	}
	return domain.NewAppError(domain.ErrCodeNotFound, fmt.Sprintf("memory not found: %s", id))
}

// asAppError reports whether err wraps a *domain.AppError and sets target if so.
func asAppError(err error, target **domain.AppError) bool {
	return errors.As(err, target)
}

// indexChunks adds all chunks to keyword and vector indexes (best-effort).
func (s *serviceImpl) indexChunks(store string, chunks []*domain.MemoryChunk) {
	for _, chunk := range chunks {
		if s.keywordIdx != nil {
			_ = s.keywordIdx.Add(store, chunk.ChunkID, chunk.Content)
		}
		if s.vectorIdx != nil && len(chunk.Vector) > 0 {
			_ = s.vectorIdx.Upsert(store, chunk.ChunkID, chunk.Vector)
		}
	}
}

// removeChunksFromIndex removes all chunk IDs from keyword and vector indexes (best-effort).
func (s *serviceImpl) removeChunksFromIndex(store string, chunks []*domain.MemoryChunk) {
	for _, chunk := range chunks {
		if s.keywordIdx != nil {
			_ = s.keywordIdx.Delete(store, chunk.ChunkID)
		}
		if s.vectorIdx != nil {
			_ = s.vectorIdx.Delete(store, chunk.ChunkID)
		}
	}
}
