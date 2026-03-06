// Package mcp provides MCP tool handlers for ingatan.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// SearchTools registers the memory_search and memory_similar MCP tools.
type SearchTools struct {
	svc memoryuc.Service
}

// NewSearchTools creates a new SearchTools with the given memory service.
func NewSearchTools(svc memoryuc.Service) *SearchTools {
	return &SearchTools{svc: svc}
}

// Register adds memory_search and memory_similar tools to the MCP server.
func (t *SearchTools) Register(srv *server.MCPServer) {
	srv.AddTool(memorySearchTool(), t.handleSearch)
	srv.AddTool(memorySimilarTool(), t.handleSimilar)
}

// memorySearchTool defines the memory_search MCP tool.
func memorySearchTool() mcp.Tool {
	return mcp.NewTool("memory_search",
		mcp.WithDescription("Search memories in a store using semantic, keyword, or hybrid retrieval"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("mode", mcp.Description("Search mode: hybrid (default), semantic, or keyword")),
		mcp.WithNumber("top_k", mcp.Description("Maximum results to return (default 10)")),
		mcp.WithArray("tags", mcp.Description("Filter results to memories with these tags")),
	)
}

// memorySimilarTool defines the memory_similar MCP tool.
func memorySimilarTool() mcp.Tool {
	return mcp.NewTool("memory_similar",
		mcp.WithDescription("Find memories similar to a given memory using vector centroid search"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("Source memory ID to find similar memories for")),
		mcp.WithNumber("top_k", mcp.Description("Maximum results to return (default 10)")),
	)
}

// handleSearch implements the memory_search tool handler.
func (t *SearchTools) handleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	query := stringArg(args, "query")
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mode := memoryuc.SearchMode(stringArg(args, "mode"))
	if mode == "" {
		mode = memoryuc.SearchModeHybrid
	}

	topK := intArg(args, "top_k", 10)

	searchReq := memoryuc.SearchRequest{
		Store:     store,
		Query:     query,
		Mode:      mode,
		TopK:      topK,
		Tags:      stringSliceArg(args, "tags"),
		Principal: principalFromContext(ctx),
	}

	resp, err := t.svc.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}

	return marshalResult(resp)
}

// handleSimilar implements the memory_similar tool handler.
func (t *SearchTools) handleSimilar(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	topK := intArg(args, "top_k", 10)

	similarReq := memoryuc.SimilarRequest{
		Store:     store,
		MemoryID:  memoryID,
		TopK:      topK,
		Principal: principalFromContext(ctx),
	}

	resp, err := t.svc.Similar(ctx, similarReq)
	if err != nil {
		return nil, fmt.Errorf("similar memories: %w", err)
	}

	return marshalResult(resp)
}
