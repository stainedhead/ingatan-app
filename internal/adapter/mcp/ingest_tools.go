package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
)

// IngestTools registers MCP tools for URL and file ingestion.
type IngestTools struct {
	svc memoryuc.Service
}

// NewIngestTools creates a new IngestTools.
func NewIngestTools(svc memoryuc.Service) *IngestTools {
	return &IngestTools{svc: svc}
}

// Register adds memory_save_url and memory_save_file tools to the MCP server.
func (t *IngestTools) Register(srv *server.MCPServer) {
	srv.AddTool(memorySaveURLTool(), t.handleSaveURL)
	srv.AddTool(memorySaveFileTool(), t.handleSaveFile)
}

// memorySaveURLTool defines the memory_save_url MCP tool.
func memorySaveURLTool() mcp.Tool {
	return mcp.NewTool("memory_save_url",
		mcp.WithDescription("Fetch a URL and save its content as a memory"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL to fetch and save")),
		mcp.WithArray("tags", mcp.Description("Tags to associate with the memory")),
	)
}

// memorySaveFileTool defines the memory_save_file MCP tool.
func memorySaveFileTool() mcp.Tool {
	return mcp.NewTool("memory_save_file",
		mcp.WithDescription("Read a local file and save its content as a memory"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store name")),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the local file")),
		mcp.WithArray("tags", mcp.Description("Tags to associate with the memory")),
	)
}

// handleSaveURL implements the memory_save_url tool handler.
func (t *IngestTools) handleSaveURL(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	url := stringArg(args, "url")
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	saveReq := memoryuc.SaveURLRequest{
		Store:     store,
		URL:       url,
		Tags:      stringSliceArg(args, "tags"),
		Principal: principalFromContext(ctx),
	}

	mem, err := t.svc.SaveURL(ctx, saveReq)
	if err != nil {
		return nil, fmt.Errorf("save URL: %w", err)
	}

	return marshalResult(mem)
}

// handleSaveFile implements the memory_save_file tool handler.
func (t *IngestTools) handleSaveFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}
	filePath := stringArg(args, "file_path")
	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	saveReq := memoryuc.SaveFileRequest{
		Store:     store,
		FilePath:  filePath,
		Tags:      stringSliceArg(args, "tags"),
		Principal: principalFromContext(ctx),
	}

	mem, err := t.svc.SaveFile(ctx, saveReq)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	return marshalResult(mem)
}
