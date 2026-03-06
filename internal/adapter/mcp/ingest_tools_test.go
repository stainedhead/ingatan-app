package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPIngestService is a test double for memoryuc.Service used by MCP ingest tests.
type mockMCPIngestService struct {
	saveURLFn  func(ctx context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error)
	saveFileFn func(ctx context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error)
}

func (m *mockMCPIngestService) Save(_ context.Context, _ memoryuc.SaveRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockMCPIngestService) Get(_ context.Context, _, _ string, _ *domain.Principal) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockMCPIngestService) Update(_ context.Context, _ memoryuc.UpdateRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockMCPIngestService) Delete(_ context.Context, _, _ string, _ *domain.Principal) error {
	return nil
}

func (m *mockMCPIngestService) List(_ context.Context, _ memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
	return &memoryuc.ListResponse{}, nil
}

func (m *mockMCPIngestService) Search(_ context.Context, _ memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMCPIngestService) Similar(_ context.Context, _ memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMCPIngestService) SaveURL(ctx context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error) {
	return m.saveURLFn(ctx, req)
}

func (m *mockMCPIngestService) SaveFile(ctx context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error) {
	return m.saveFileFn(ctx, req)
}

// ingestSampleMemory returns a reusable test memory for ingest tests.
func ingestSampleMemory() *domain.Memory {
	return &domain.Memory{
		ID:        "mem-ingest-1",
		Store:     "default",
		Title:     "Ingested Memory",
		Content:   "Ingested content",
		Tags:      []string{"web"},
		Source:    domain.MemorySourceURL,
		SourceURL: "https://example.com",
		Metadata:  map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestHandleSaveURL_Success(t *testing.T) {
	mem := ingestSampleMemory()
	svc := &mockMCPIngestService{
		saveURLFn: func(_ context.Context, req memoryuc.SaveURLRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "https://example.com", req.URL)
			return mem, nil
		},
	}

	tools := NewIngestTools(svc)
	req := callRequest("memory_save_url", map[string]any{
		"store": "default",
		"url":   "https://example.com",
	})

	result, err := tools.handleSaveURL(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "mem-ingest-1", got.ID)
}

func TestHandleSaveURL_MissingStore(t *testing.T) {
	svc := &mockMCPIngestService{}
	tools := NewIngestTools(svc)

	req := callRequest("memory_save_url", map[string]any{
		"url": "https://example.com",
	})

	_, err := tools.handleSaveURL(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store is required")
}

func TestHandleSaveURL_MissingURL(t *testing.T) {
	svc := &mockMCPIngestService{}
	tools := NewIngestTools(svc)

	req := callRequest("memory_save_url", map[string]any{
		"store": "default",
	})

	_, err := tools.handleSaveURL(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is required")
}

func TestHandleSaveFile_Success(t *testing.T) {
	mem := ingestSampleMemory()
	mem.Source = domain.MemorySourceFile
	mem.SourcePath = "/safe/readme.txt"

	svc := &mockMCPIngestService{
		saveFileFn: func(_ context.Context, req memoryuc.SaveFileRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "/safe/readme.txt", req.FilePath)
			return mem, nil
		},
	}

	tools := NewIngestTools(svc)
	req := callRequest("memory_save_file", map[string]any{
		"store":     "default",
		"file_path": "/safe/readme.txt",
	})

	result, err := tools.handleSaveFile(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "mem-ingest-1", got.ID)
}

func TestHandleSaveFile_MissingFilePath(t *testing.T) {
	svc := &mockMCPIngestService{}
	tools := NewIngestTools(svc)

	req := callRequest("memory_save_file", map[string]any{
		"store": "default",
	})

	_, err := tools.handleSaveFile(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file_path is required")
}
