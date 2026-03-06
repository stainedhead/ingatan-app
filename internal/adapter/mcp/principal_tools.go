// Package mcp provides MCP tool handlers for ingatan.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
)

// PrincipalTools registers all principal-related MCP tools on an MCPServer.
type PrincipalTools struct {
	svc principaluc.Service
}

// NewPrincipalTools creates a new PrincipalTools with the given principal service.
func NewPrincipalTools(svc principaluc.Service) *PrincipalTools {
	return &PrincipalTools{svc: svc}
}

// Register adds both principal tools to the MCP server.
func (t *PrincipalTools) Register(srv *server.MCPServer) {
	srv.AddTool(principalWhoAmITool(), t.handleWhoAmI)
	srv.AddTool(principalListTool(), t.handleList)
}

// principalWhoAmITool defines the principal_whoami MCP tool.
func principalWhoAmITool() mcp.Tool {
	return mcp.NewTool("principal_whoami",
		mcp.WithDescription("Return details about the currently authenticated principal including store memberships and capabilities"),
	)
}

// principalListTool defines the principal_list MCP tool.
func principalListTool() mcp.Tool {
	return mcp.NewTool("principal_list",
		mcp.WithDescription("List all principals on this ingatan instance (admin only)"),
	)
}

// handleWhoAmI implements the principal_whoami tool handler.
func (t *PrincipalTools) handleWhoAmI(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = req
	principal := principalFromContext(ctx)

	resp, err := t.svc.WhoAmI(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("whoami: %w", err)
	}

	return marshalResult(resp)
}

// handleList implements the principal_list tool handler.
func (t *PrincipalTools) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = req
	caller := principalFromContext(ctx)

	principals, err := t.svc.List(ctx, caller)
	if err != nil {
		return nil, fmt.Errorf("list principals: %w", err)
	}

	return marshalResult(map[string]any{"principals": principals})
}
