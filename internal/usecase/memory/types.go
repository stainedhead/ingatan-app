// Package memory provides the use case layer for memory management in ingatan.
package memory

import (
	"context"

	"github.com/stainedhead/ingatan/internal/domain"
)

// Repository is the file-based persistence interface for memory records.
// Defined here (use case layer) and implemented by the infrastructure layer.
type Repository interface {
	Save(ctx context.Context, m *domain.Memory) error
	Get(ctx context.Context, store, memoryID string) (*domain.Memory, error)
	Update(ctx context.Context, m *domain.Memory) error
	Delete(ctx context.Context, store, memoryID string) error
	List(ctx context.Context, store string, filter Filter, limit, offset int) ([]*domain.Memory, int, error)
}

// ChunkRepository persists and retrieves memory chunks.
type ChunkRepository interface {
	SaveChunks(ctx context.Context, store, memoryID string, chunks []*domain.MemoryChunk) error
	GetChunks(ctx context.Context, store, memoryID string) ([]*domain.MemoryChunk, error)
	GetChunkByID(ctx context.Context, store, chunkID string) (*domain.MemoryChunk, error)
	DeleteChunks(ctx context.Context, store, memoryID string) error
}

// Chunker splits raw text into overlapping segments.
type Chunker interface {
	Chunk(content string) ([]string, error)
}

// Embedder converts text segments into float32 embedding vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	Model() string
}

// Filter constrains which memories are returned by Repository.List.
type Filter struct {
	Tags   []string
	Source *domain.MemorySource
}

// SaveRequest carries the input for Service.Save.
type SaveRequest struct {
	Store      string
	Title      string
	Content    string
	Tags       []string
	Source     domain.MemorySource
	SourceRef  string
	SourceURL  string // URL provenance — set when Source == MemorySourceURL.
	SourcePath string // File path provenance — set when Source == MemorySourceFile.
	Metadata   map[string]any
	Principal  *domain.Principal
}

// UpdateRequest carries fields to update on an existing memory.
// Nil pointer fields mean "no change"; a non-nil empty slice for Tags clears all tags.
type UpdateRequest struct {
	Store     string
	MemoryID  string
	Title     *string
	Content   *string
	Tags      *[]string
	Metadata  map[string]any // merged into existing metadata
	Principal *domain.Principal
}

// ListRequest carries pagination and filter parameters for Service.List.
type ListRequest struct {
	Store     string
	Tags      []string
	Source    *domain.MemorySource
	Limit     int
	Offset    int
	Principal *domain.Principal
}

// ListResponse is the result of Service.List.
type ListResponse struct {
	Memories []*domain.Memory
	Total    int
}

// SearchMode is the retrieval strategy for memory search.
type SearchMode string

const (
	// SearchModeHybrid combines semantic and keyword search via Reciprocal Rank Fusion.
	SearchModeHybrid SearchMode = "hybrid"
	// SearchModeSemantic uses only HNSW vector similarity.
	SearchModeSemantic SearchMode = "semantic"
	// SearchModeKeyword uses only BM25 keyword matching.
	SearchModeKeyword SearchMode = "keyword"
)

// SearchRequest carries parameters for a memory search.
type SearchRequest struct {
	Store     string
	Query     string
	Mode      SearchMode
	TopK      int
	Tags      []string
	Principal *domain.Principal
}

// SearchResponse is the result of a memory search.
type SearchResponse struct {
	Results []SearchResult
}

// SearchResult is a single ranked memory with its relevance scores.
type SearchResult struct {
	Memory          *domain.Memory
	Score           float64
	ScoreComponents ScoreComponents
}

// ScoreComponents breaks down the hybrid search score.
type ScoreComponents struct {
	Semantic float32
	Keyword  float64
	RRF      float64
}

// SimilarRequest carries parameters for similar memory search.
type SimilarRequest struct {
	Store     string
	MemoryID  string
	TopK      int
	Principal *domain.Principal
}

// VectorIndex manages per-store HNSW vector index operations.
// Defined here (use case layer); implemented by the infrastructure layer.
// Writes are auto-persisted to disk by the implementation.
type VectorIndex interface {
	Upsert(store, chunkID string, vector []float32) error
	Search(store string, query []float32, topK int) ([]domain.VectorSearchResult, error)
	Delete(store, chunkID string) error
}

// KeywordIndex manages per-store BM25 keyword index operations.
// Defined here (use case layer); implemented by the infrastructure layer.
// Writes are auto-persisted to disk by the implementation.
type KeywordIndex interface {
	Add(store, chunkID, content string) error
	Search(store, query string, topK int) ([]domain.KeywordSearchResult, error)
	Delete(store, chunkID string) error
}

// Service exposes the memory CRUD use case operations.
// It is the boundary between the adapter layer and the domain + infrastructure layers.
type Service interface {
	Save(ctx context.Context, req SaveRequest) (*domain.Memory, error)
	Get(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error)
	Update(ctx context.Context, req UpdateRequest) (*domain.Memory, error)
	Delete(ctx context.Context, store, memoryID string, principal *domain.Principal) error
	List(ctx context.Context, req ListRequest) (*ListResponse, error)
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
	Similar(ctx context.Context, req SimilarRequest) (*SearchResponse, error)
	SaveURL(ctx context.Context, req SaveURLRequest) (*domain.Memory, error)
	SaveFile(ctx context.Context, req SaveFileRequest) (*domain.Memory, error)
}

// URLFetcher fetches the content of a URL and extracts readable text.
// Defined here (use case layer); implemented by the infrastructure layer.
type URLFetcher interface {
	Fetch(ctx context.Context, rawURL string) (title, content string, err error)
}

// FileReader reads content from a local file path.
// Defined here (use case layer); implemented by the infrastructure layer.
type FileReader interface {
	Read(path string) (content string, err error)
	Supports(path string) bool
}

// IngestOptions configures path restrictions and size limits for URL/file ingest operations.
type IngestOptions struct {
	AllowedPaths    []string // If non-empty, file paths must start with one of these prefixes.
	MaxContentBytes int      // Maximum content bytes; 0 means use default (10 MiB).
}

// SaveURLRequest carries parameters for Service.SaveURL.
type SaveURLRequest struct {
	Store     string
	URL       string
	Tags      []string
	Metadata  map[string]any
	Principal *domain.Principal
}

// SaveFileRequest carries parameters for Service.SaveFile.
type SaveFileRequest struct {
	Store     string
	FilePath  string
	Tags      []string
	Metadata  map[string]any
	Principal *domain.Principal
}

// StoreAccess checks a principal's membership role in a named store.
// Defined here (use case layer); implemented by the store repository or service layer.
// Returns the principal's StoreRole, or an empty string if not a member.
// Returns a NOT_FOUND AppError if the store does not exist.
type StoreAccess interface {
	GetMemberRole(ctx context.Context, storeName, principalID string) (domain.StoreRole, error)
}
