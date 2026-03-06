// Package mcp provides MCP tool handlers for ingatan.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// MemoryTools registers all memory-related MCP tools on an MCPServer.
type MemoryTools struct {
	svc memoryuc.Service
}

// NewMemoryTools creates a new MemoryTools with the given memory service.
func NewMemoryTools(svc memoryuc.Service) *MemoryTools {
	return &MemoryTools{svc: svc}
}

// Register adds all five memory tools to the MCP server.
func (t *MemoryTools) Register(srv *server.MCPServer) {
	srv.AddTool(memorySaveTool(), t.handleSave)
	srv.AddTool(memoryGetTool(), t.handleGet)
	srv.AddTool(memoryUpdateTool(), t.handleUpdate)
	srv.AddTool(memoryDeleteTool(), t.handleDelete)
	srv.AddTool(memoryListTool(), t.handleList)
}

// memorySaveTool defines the memory_save MCP tool.
func memorySaveTool() mcp.Tool {
	return mcp.NewTool("memory_save",
		mcp.WithDescription("Save a new memory to a store"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Memory content")),
		mcp.WithString("title", mcp.Description("Memory title")),
		mcp.WithArray("tags", mcp.Description("Tags to associate with the memory")),
		mcp.WithString("source", mcp.Description("Memory source (manual, agent, file, url, conversation, import)")),
		mcp.WithString("source_ref", mcp.Description("Source reference identifier")),
	)
}

// memoryGetTool defines the memory_get MCP tool.
func memoryGetTool() mcp.Tool {
	return mcp.NewTool("memory_get",
		mcp.WithDescription("Retrieve a memory by ID"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("Memory ID")),
	)
}

// memoryUpdateTool defines the memory_update MCP tool.
func memoryUpdateTool() mcp.Tool {
	return mcp.NewTool("memory_update",
		mcp.WithDescription("Update an existing memory"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithString("title", mcp.Description("New title (omit to keep existing)")),
		mcp.WithString("content", mcp.Description("New content (omit to keep existing)")),
		mcp.WithArray("tags", mcp.Description("Replacement tag list (omit to keep existing)")),
	)
}

// memoryDeleteTool defines the memory_delete MCP tool.
func memoryDeleteTool() mcp.Tool {
	return mcp.NewTool("memory_delete",
		mcp.WithDescription("Delete a memory by ID"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("Memory ID")),
	)
}

// memoryListTool defines the memory_list MCP tool.
func memoryListTool() mcp.Tool {
	return mcp.NewTool("memory_list",
		mcp.WithDescription("List memories in a store with optional filtering"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithArray("tags", mcp.Description("Filter by tags")),
		mcp.WithString("source", mcp.Description("Filter by source type")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 20)")),
		mcp.WithNumber("offset", mcp.Description("Pagination offset (default 0)")),
	)
}

// principalFromContext returns the principal from context or creates an anonymous fallback.
func principalFromContext(ctx context.Context) *domain.Principal {
	p := apimw.PrincipalFromContext(ctx)
	if p == nil {
		return &domain.Principal{ID: "mcp-anonymous", Role: domain.InstanceRoleUser}
	}
	return p
}

// stringArg extracts a string argument, returning "" if absent or wrong type.
func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// stringSliceArg converts a []any argument to []string, returning nil if absent.
func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// intArg extracts a float64 JSON number argument and converts it to int.
func intArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

// marshalResult serialises v to JSON and wraps it in a tool text result.
func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

// argsMap extracts the tool arguments as map[string]any from a CallToolRequest.
// Returns nil if the arguments cannot be cast to the expected type.
func argsMap(req mcp.CallToolRequest) map[string]any {
	m, _ := req.Params.Arguments.(map[string]any)
	return m
}

// handleSave implements the memory_save tool handler.
func (t *MemoryTools) handleSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	content := stringArg(args, "content")
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	source := domain.MemorySource(stringArg(args, "source"))
	if source == "" {
		source = domain.MemorySourceManual
	}

	saveReq := memoryuc.SaveRequest{
		Store:     store,
		Content:   content,
		Title:     stringArg(args, "title"),
		Tags:      stringSliceArg(args, "tags"),
		Source:    source,
		SourceRef: stringArg(args, "source_ref"),
		Principal: principalFromContext(ctx),
	}

	mem, err := t.svc.Save(ctx, saveReq)
	if err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	return marshalResult(mem)
}

// handleGet implements the memory_get tool handler.
func (t *MemoryTools) handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	memoryID := stringArg(args, "memory_id")
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}

	mem, err := t.svc.Get(ctx, store, memoryID, principalFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}

	return marshalResult(mem)
}

// handleUpdate implements the memory_update tool handler.
func (t *MemoryTools) handleUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	memoryID := stringArg(args, "memory_id")
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}

	updateReq := memoryuc.UpdateRequest{
		Store:     store,
		MemoryID:  memoryID,
		Principal: principalFromContext(ctx),
	}

	if titleRaw, ok := args["title"]; ok && titleRaw != nil {
		if s, ok := titleRaw.(string); ok {
			updateReq.Title = &s
		}
	}
	if contentRaw, ok := args["content"]; ok && contentRaw != nil {
		if s, ok := contentRaw.(string); ok {
			updateReq.Content = &s
		}
	}
	if tagsRaw, ok := args["tags"]; ok && tagsRaw != nil {
		tags := stringSliceArg(args, "tags")
		updateReq.Tags = &tags
	}

	mem, err := t.svc.Update(ctx, updateReq)
	if err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}

	return marshalResult(mem)
}

// handleDelete implements the memory_delete tool handler.
func (t *MemoryTools) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	memoryID := stringArg(args, "memory_id")
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}

	if err := t.svc.Delete(ctx, store, memoryID, principalFromContext(ctx)); err != nil {
		return nil, fmt.Errorf("delete memory: %w", err)
	}

	return marshalResult(map[string]bool{"deleted": true})
}

// handleList implements the memory_list tool handler.
func (t *MemoryTools) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}

	limit := intArg(args, "limit", 20)
	offset := intArg(args, "offset", 0)

	var source *domain.MemorySource
	if srcStr := stringArg(args, "source"); srcStr != "" {
		s := domain.MemorySource(srcStr)
		source = &s
	}

	listReq := memoryuc.ListRequest{
		Store:     store,
		Tags:      stringSliceArg(args, "tags"),
		Source:    source,
		Limit:     limit,
		Offset:    offset,
		Principal: principalFromContext(ctx),
	}

	resp, err := t.svc.List(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	return marshalResult(map[string]any{
		"memories": resp.Memories,
		"total":    resp.Total,
	})
}
