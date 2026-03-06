package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPMemoryService is a test double for memoryuc.Service used by MCP tests.
type mockMCPMemoryService struct {
	saveFn   func(ctx context.Context, req memoryuc.SaveRequest) (*domain.Memory, error)
	getFn    func(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error)
	updateFn func(ctx context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error)
	deleteFn func(ctx context.Context, store, memoryID string, principal *domain.Principal) error
	listFn   func(ctx context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error)
}

func (m *mockMCPMemoryService) Save(ctx context.Context, req memoryuc.SaveRequest) (*domain.Memory, error) {
	return m.saveFn(ctx, req)
}

func (m *mockMCPMemoryService) Get(ctx context.Context, store, memoryID string, principal *domain.Principal) (*domain.Memory, error) {
	return m.getFn(ctx, store, memoryID, principal)
}

func (m *mockMCPMemoryService) Update(ctx context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error) {
	return m.updateFn(ctx, req)
}

func (m *mockMCPMemoryService) Delete(ctx context.Context, store, memoryID string, principal *domain.Principal) error {
	return m.deleteFn(ctx, store, memoryID, principal)
}

func (m *mockMCPMemoryService) List(ctx context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
	return m.listFn(ctx, req)
}

func (m *mockMCPMemoryService) Search(_ context.Context, _ memoryuc.SearchRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMCPMemoryService) Similar(_ context.Context, _ memoryuc.SimilarRequest) (*memoryuc.SearchResponse, error) {
	return &memoryuc.SearchResponse{}, nil
}

func (m *mockMCPMemoryService) SaveURL(_ context.Context, _ memoryuc.SaveURLRequest) (*domain.Memory, error) {
	return nil, nil
}

func (m *mockMCPMemoryService) SaveFile(_ context.Context, _ memoryuc.SaveFileRequest) (*domain.Memory, error) {
	return nil, nil
}

// sampleMemory returns a reusable test domain.Memory.
func sampleMemory() *domain.Memory {
	return &domain.Memory{
		ID:        "mem-1",
		Store:     "default",
		Title:     "Sample Memory",
		Content:   "Sample content",
		Tags:      []string{"go"},
		Source:    domain.MemorySourceManual,
		Metadata:  map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// callRequest builds a mcp.CallToolRequest with the provided arguments map.
func callRequest(name string, args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// decodeTextResult parses the JSON text from a CallToolResult into v.
func decodeTextResult(t *testing.T, result *mcp.CallToolResult, v any) {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), v))
}

func TestHandleSave_Success(t *testing.T) {
	mem := sampleMemory()
	svc := &mockMCPMemoryService{
		saveFn: func(_ context.Context, req memoryuc.SaveRequest) (*domain.Memory, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, "Sample content", req.Content)
			return mem, nil
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_save", map[string]any{
		"store":   "default",
		"content": "Sample content",
	})

	result, err := tools.handleSave(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "mem-1", got.ID)
}

func TestHandleSave_MissingStore(t *testing.T) {
	svc := &mockMCPMemoryService{}
	tools := NewMemoryTools(svc)

	req := callRequest("memory_save", map[string]any{
		"content": "Some content",
	})

	_, err := tools.handleSave(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store is required")
}

func TestHandleSave_MissingContent(t *testing.T) {
	svc := &mockMCPMemoryService{}
	tools := NewMemoryTools(svc)

	req := callRequest("memory_save", map[string]any{
		"store": "default",
	})

	_, err := tools.handleSave(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestHandleGet_Success(t *testing.T) {
	mem := sampleMemory()
	svc := &mockMCPMemoryService{
		getFn: func(_ context.Context, store, memoryID string, _ *domain.Principal) (*domain.Memory, error) {
			assert.Equal(t, "default", store)
			assert.Equal(t, "mem-1", memoryID)
			return mem, nil
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_get", map[string]any{
		"store":     "default",
		"memory_id": "mem-1",
	})

	result, err := tools.handleGet(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "mem-1", got.ID)
}

func TestHandleGet_NotFound(t *testing.T) {
	svc := &mockMCPMemoryService{
		getFn: func(_ context.Context, _, _ string, _ *domain.Principal) (*domain.Memory, error) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "memory not found")
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_get", map[string]any{
		"store":     "default",
		"memory_id": "missing",
	})

	_, err := tools.handleGet(context.Background(), req)
	require.Error(t, err)

	var appErr *domain.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

func TestHandleList_Success(t *testing.T) {
	mems := []*domain.Memory{sampleMemory()}
	svc := &mockMCPMemoryService{
		listFn: func(_ context.Context, req memoryuc.ListRequest) (*memoryuc.ListResponse, error) {
			assert.Equal(t, "default", req.Store)
			assert.Equal(t, 10, req.Limit)
			assert.Equal(t, 5, req.Offset)
			return &memoryuc.ListResponse{Memories: mems, Total: 1}, nil
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_list", map[string]any{
		"store":  "default",
		"limit":  float64(10),
		"offset": float64(5),
	})

	result, err := tools.handleList(context.Background(), req)
	require.NoError(t, err)

	var got map[string]any
	decodeTextResult(t, result, &got)
	assert.Equal(t, float64(1), got["total"])
	memories, ok := got["memories"].([]any)
	require.True(t, ok)
	assert.Len(t, memories, 1)
}

func TestHandleDelete_Success(t *testing.T) {
	svc := &mockMCPMemoryService{
		deleteFn: func(_ context.Context, store, memoryID string, _ *domain.Principal) error {
			assert.Equal(t, "default", store)
			assert.Equal(t, "mem-1", memoryID)
			return nil
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_delete", map[string]any{
		"store":     "default",
		"memory_id": "mem-1",
	})

	result, err := tools.handleDelete(context.Background(), req)
	require.NoError(t, err)

	var got map[string]bool
	decodeTextResult(t, result, &got)
	assert.True(t, got["deleted"])
}

func TestHandleUpdate_TitleOnly(t *testing.T) {
	updated := sampleMemory()
	updated.Title = "New Title"

	svc := &mockMCPMemoryService{
		updateFn: func(_ context.Context, req memoryuc.UpdateRequest) (*domain.Memory, error) {
			require.NotNil(t, req.Title)
			assert.Equal(t, "New Title", *req.Title)
			assert.Nil(t, req.Content)
			assert.Nil(t, req.Tags)
			return updated, nil
		},
	}

	tools := NewMemoryTools(svc)
	req := callRequest("memory_update", map[string]any{
		"store":     "default",
		"memory_id": "mem-1",
		"title":     "New Title",
	})

	result, err := tools.handleUpdate(context.Background(), req)
	require.NoError(t, err)

	var got domain.Memory
	decodeTextResult(t, result, &got)
	assert.Equal(t, "New Title", got.Title)
}
