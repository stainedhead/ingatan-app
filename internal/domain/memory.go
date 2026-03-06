package domain

import "time"

// MemorySource identifies how a memory was created.
type MemorySource string

const (
	// MemorySourceManual is content entered directly by a user or agent.
	MemorySourceManual MemorySource = "manual"
	// MemorySourceConversation is content promoted from a conversation.
	MemorySourceConversation MemorySource = "conversation"
	// MemorySourceImport is content imported in bulk.
	MemorySourceImport MemorySource = "import"
	// MemorySourceAgent is content saved autonomously by an agent.
	MemorySourceAgent MemorySource = "agent"
	// MemorySourceFile is content extracted from a local file.
	MemorySourceFile MemorySource = "file"
	// MemorySourceURL is content extracted from a web URL.
	MemorySourceURL MemorySource = "url"
)

// Memory is the core stored artifact: a piece of text with metadata, chunked and embedded.
// Chunks and vectors are stored separately and not embedded inline.
type Memory struct {
	ID         string
	Store      string
	Title      string
	Content    string
	Tags       []string
	Source     MemorySource
	SourceRef  string // Conversation ID if Source == MemorySourceConversation.
	SourcePath string // File path if Source == MemorySourceFile.
	SourceURL  string // URL if Source == MemorySourceURL.
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// MemoryChunk is a text segment of a Memory with its embedding vector.
// Vectors are stored in the HNSW index; this struct carries content for BM25 indexing.
type MemoryChunk struct {
	ChunkID    string
	MemoryID   string
	Store      string
	ChunkIndex int // Zero-based position within the parent memory.
	Content    string
	Vector     []float32 // Populated when embedding is available; nil in persisted chunk files.
}

// VectorSearchResult is a ranked chunk result from vector similarity search.
type VectorSearchResult struct {
	ChunkID string
	Score   float32
}

// KeywordSearchResult is a ranked chunk result from keyword (BM25) search.
type KeywordSearchResult struct {
	ChunkID string
	Score   float64
}
