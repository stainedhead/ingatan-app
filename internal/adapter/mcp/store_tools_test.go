package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stainedhead/ingatan/internal/domain"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPStoreService is a test double for storeuc.Service used by MCP tests.
type mockMCPStoreService struct {
	createFn func(ctx context.Context, req storeuc.CreateRequest) (*domain.Store, error)
	getFn    func(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error)
	listFn   func(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error)
	deleteFn func(ctx context.Context, name, confirm string, principal *domain.Principal) error
}

func (m *mockMCPStoreService) Create(ctx context.Context, req storeuc.CreateRequest) (*domain.Store, error) {
	return m.createFn(ctx, req)
}

func (m *mockMCPStoreService) Get(ctx context.Context, name string, principal *domain.Principal) (*domain.Store, error) {
	return m.getFn(ctx, name, principal)
}

func (m *mockMCPStoreService) List(ctx context.Context, principal *domain.Principal) ([]*domain.Store, error) {
	return m.listFn(ctx, principal)
}

func (m *mockMCPStoreService) Delete(ctx context.Context, name, confirm string, principal *domain.Principal) error {
	return m.deleteFn(ctx, name, confirm, principal)
}

// sampleStore returns a reusable test domain.Store.
func sampleStore() *domain.Store {
	return &domain.Store{
		Name:        "test-store",
		Description: "A test store",
		OwnerID:     "user-1",
		CreatedAt:   time.Now(),
		Members:     []domain.StoreMember{{PrincipalID: "user-1", Role: domain.StoreRoleOwner}},
	}
}

// decodeStoreResult decodes a tool result JSON into a domain.Store.
func decodeStoreResult(t *testing.T, result *mcp.CallToolResult) domain.Store {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	var got domain.Store
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &got))
	return got
}

func TestStoreHandleList_Success(t *testing.T) {
	stores := []*domain.Store{sampleStore()}
	svc := &mockMCPStoreService{
		listFn: func(_ context.Context, _ *domain.Principal) ([]*domain.Store, error) {
			return stores, nil
		},
	}

	tools := NewStoreTools(svc)
	req := callRequest("store_list", map[string]any{})

	result, err := tools.handleList(context.Background(), req)
	require.NoError(t, err)

	var got map[string]any
	decodeTextResult(t, result, &got)
	storeList, ok := got["stores"].([]any)
	require.True(t, ok)
	assert.Len(t, storeList, 1)
}

func TestStoreHandleGet_Success(t *testing.T) {
	s := sampleStore()
	svc := &mockMCPStoreService{
		getFn: func(_ context.Context, name string, _ *domain.Principal) (*domain.Store, error) {
			assert.Equal(t, "test-store", name)
			return s, nil
		},
	}

	tools := NewStoreTools(svc)
	req := callRequest("store_get", map[string]any{"name": "test-store"})

	result, err := tools.handleGet(context.Background(), req)
	require.NoError(t, err)

	got := decodeStoreResult(t, result)
	assert.Equal(t, "test-store", got.Name)
}

func TestStoreHandleGet_MissingName(t *testing.T) {
	svc := &mockMCPStoreService{}
	tools := NewStoreTools(svc)

	req := callRequest("store_get", map[string]any{})
	_, err := tools.handleGet(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestStoreHandleCreate_Success(t *testing.T) {
	s := sampleStore()
	svc := &mockMCPStoreService{
		createFn: func(_ context.Context, req storeuc.CreateRequest) (*domain.Store, error) {
			assert.Equal(t, "test-store", req.Name)
			assert.Equal(t, "A test store", req.Description)
			return s, nil
		},
	}

	tools := NewStoreTools(svc)
	req := callRequest("store_create", map[string]any{
		"name":        "test-store",
		"description": "A test store",
	})

	result, err := tools.handleCreate(context.Background(), req)
	require.NoError(t, err)

	got := decodeStoreResult(t, result)
	assert.Equal(t, "test-store", got.Name)
}

func TestStoreHandleCreate_MissingName(t *testing.T) {
	svc := &mockMCPStoreService{}
	tools := NewStoreTools(svc)

	req := callRequest("store_create", map[string]any{"description": "no name"})
	_, err := tools.handleCreate(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestStoreHandleDelete_Success(t *testing.T) {
	svc := &mockMCPStoreService{
		deleteFn: func(_ context.Context, name, confirm string, _ *domain.Principal) error {
			assert.Equal(t, "test-store", name)
			assert.Equal(t, "test-store", confirm)
			return nil
		},
	}

	tools := NewStoreTools(svc)
	req := callRequest("store_delete", map[string]any{
		"name":    "test-store",
		"confirm": "test-store",
	})

	result, err := tools.handleDelete(context.Background(), req)
	require.NoError(t, err)

	var got map[string]bool
	decodeTextResult(t, result, &got)
	assert.True(t, got["deleted"])
}

func TestStoreHandleDelete_MissingConfirm(t *testing.T) {
	svc := &mockMCPStoreService{}
	tools := NewStoreTools(svc)

	req := callRequest("store_delete", map[string]any{"name": "test-store"})
	_, err := tools.handleDelete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirm is required")
}
