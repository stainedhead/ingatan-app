// Package mcp provides MCP tool handlers for ingatan.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

// StoreTools registers all store-related MCP tools on an MCPServer.
type StoreTools struct {
	svc storeuc.Service
}

// NewStoreTools creates a new StoreTools with the given store service.
func NewStoreTools(svc storeuc.Service) *StoreTools {
	return &StoreTools{svc: svc}
}

// Register adds all four store tools to the MCP server.
func (t *StoreTools) Register(srv *server.MCPServer) {
	srv.AddTool(storeListTool(), t.handleList)
	srv.AddTool(storeGetTool(), t.handleGet)
	srv.AddTool(storeCreateTool(), t.handleCreate)
	srv.AddTool(storeDeleteTool(), t.handleDelete)
}

// storeListTool defines the store_list MCP tool.
func storeListTool() mcp.Tool {
	return mcp.NewTool("store_list",
		mcp.WithDescription("List all stores accessible to the caller"),
	)
}

// storeGetTool defines the store_get MCP tool.
func storeGetTool() mcp.Tool {
	return mcp.NewTool("store_get",
		mcp.WithDescription("Get details of a store by name"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Store name")),
	)
}

// storeCreateTool defines the store_create MCP tool.
func storeCreateTool() mcp.Tool {
	return mcp.NewTool("store_create",
		mcp.WithDescription("Create a new store"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Store name (slug: [a-z0-9-]+)")),
		mcp.WithString("description", mcp.Description("Human-readable description of the store")),
	)
}

// storeDeleteTool defines the store_delete MCP tool.
func storeDeleteTool() mcp.Tool {
	return mcp.NewTool("store_delete",
		mcp.WithDescription("Delete a store and all its contents. The confirm field must equal the store name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Store name to delete")),
		mcp.WithString("confirm", mcp.Required(), mcp.Description("Must equal the store name to confirm deletion")),
	)
}

// handleList implements the store_list tool handler.
func (t *StoreTools) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = req
	principal := principalFromContext(ctx)

	stores, err := t.svc.List(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("list stores: %w", err)
	}

	return marshalResult(map[string]any{"stores": stores})
}

// handleGet implements the store_get tool handler.
func (t *StoreTools) handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	name := stringArg(args, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	store, err := t.svc.Get(ctx, name, principalFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get store: %w", err)
	}

	return marshalResult(store)
}

// handleCreate implements the store_create tool handler.
func (t *StoreTools) handleCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	name := stringArg(args, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	createReq := storeuc.CreateRequest{
		Name:        name,
		Description: stringArg(args, "description"),
		Principal:   principalFromContext(ctx),
	}

	store, err := t.svc.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	return marshalResult(store)
}

// handleDelete implements the store_delete tool handler.
func (t *StoreTools) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	name := stringArg(args, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	confirm := stringArg(args, "confirm")
	if confirm == "" {
		return nil, fmt.Errorf("confirm is required")
	}

	if err := t.svc.Delete(ctx, name, confirm, principalFromContext(ctx)); err != nil {
		return nil, fmt.Errorf("delete store: %w", err)
	}

	return marshalResult(map[string]bool{"deleted": true})
}
