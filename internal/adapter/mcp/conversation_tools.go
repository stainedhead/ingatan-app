// Package mcp provides MCP tool handlers for ingatan.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stainedhead/ingatan/internal/domain"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
)

// ConversationTools registers all conversation-related MCP tools on an MCPServer.
type ConversationTools struct {
	svc conversationuc.Service
}

// NewConversationTools creates a new ConversationTools with the given conversation service.
func NewConversationTools(svc conversationuc.Service) *ConversationTools {
	return &ConversationTools{svc: svc}
}

// Register adds all conversation tools to the MCP server.
func (t *ConversationTools) Register(srv *server.MCPServer) {
	srv.AddTool(conversationStartTool(), t.handleStart)
	srv.AddTool(conversationAddMessageTool(), t.handleAddMessage)
	srv.AddTool(conversationGetTool(), t.handleGet)
	srv.AddTool(conversationListTool(), t.handleList)
	srv.AddTool(conversationSummarizeTool(), t.handleSummarize)
	srv.AddTool(conversationPromoteTool(), t.handlePromote)
	srv.AddTool(conversationDeleteTool(), t.handleDelete)
}

// conversationStartTool defines the conversation_start MCP tool.
func conversationStartTool() mcp.Tool {
	return mcp.NewTool("conversation_start",
		mcp.WithDescription("Start a new conversation thread"),
		mcp.WithString("store", mcp.Required(), mcp.Description("Store to associate this conversation with")),
		mcp.WithString("title", mcp.Description("Optional title for the conversation")),
	)
}

// conversationAddMessageTool defines the conversation_add_message MCP tool.
func conversationAddMessageTool() mcp.Tool {
	return mcp.NewTool("conversation_add_message",
		mcp.WithDescription("Add a message to a conversation"),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("Conversation ID")),
		mcp.WithString("role", mcp.Required(), mcp.Description("Message role: user | assistant | system | tool")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Message content")),
	)
}

// conversationGetTool defines the conversation_get MCP tool.
func conversationGetTool() mcp.Tool {
	return mcp.NewTool("conversation_get",
		mcp.WithDescription("Get a conversation with all its messages"),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("Conversation ID")),
	)
}

// conversationListTool defines the conversation_list MCP tool.
func conversationListTool() mcp.Tool {
	return mcp.NewTool("conversation_list",
		mcp.WithDescription("List conversations, optionally filtered by store"),
		mcp.WithString("store", mcp.Description("Filter by store name")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 20)")),
		mcp.WithNumber("offset", mcp.Description("Pagination offset (default 0)")),
	)
}

// conversationSummarizeTool defines the conversation_summarize MCP tool.
func conversationSummarizeTool() mcp.Tool {
	return mcp.NewTool("conversation_summarize",
		mcp.WithDescription("Generate an LLM summary of a conversation"),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("Conversation ID")),
	)
}

// conversationPromoteTool defines the conversation_promote MCP tool.
func conversationPromoteTool() mcp.Tool {
	return mcp.NewTool("conversation_promote",
		mcp.WithDescription("Promote a conversation to a persistent memory"),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("Conversation ID")),
		mcp.WithString("store", mcp.Required(), mcp.Description("Target store for the memory")),
		mcp.WithString("title", mcp.Description("Title for the memory")),
		mcp.WithBoolean("use_summary", mcp.Description("Use summary text instead of full transcript (default false)")),
	)
}

// conversationDeleteTool defines the conversation_delete MCP tool.
func conversationDeleteTool() mcp.Tool {
	return mcp.NewTool("conversation_delete",
		mcp.WithDescription("Delete a conversation and all its messages. confirm must equal the conversation_id."),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("Conversation ID")),
		mcp.WithString("confirm", mcp.Required(), mcp.Description("Must equal conversation_id to confirm deletion")),
	)
}

// handleStart implements the conversation_start tool handler.
func (t *ConversationTools) handleStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}

	conv, err := t.svc.Start(ctx, conversationuc.StartRequest{
		Title:     stringArg(args, "title"),
		Store:     store,
		Principal: principalFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("start conversation: %w", err)
	}

	return marshalResult(conv)
}

// handleAddMessage implements the conversation_add_message tool handler.
func (t *ConversationTools) handleAddMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	conversationID := stringArg(args, "conversation_id")
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	role := stringArg(args, "role")
	if role == "" {
		return nil, fmt.Errorf("role is required")
	}
	content := stringArg(args, "content")
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	msg, err := t.svc.AddMessage(ctx, conversationuc.AddMessageRequest{
		ConversationID: conversationID,
		Role:           domain.MessageRole(role),
		Content:        content,
		Principal:      principalFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("add message: %w", err)
	}

	return marshalResult(msg)
}

// handleGet implements the conversation_get tool handler.
func (t *ConversationTools) handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	conversationID := stringArg(args, "conversation_id")
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	resp, err := t.svc.Get(ctx, conversationID, principalFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}

	return marshalResult(resp)
}

// handleList implements the conversation_list tool handler.
func (t *ConversationTools) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	limit := intArg(args, "limit", 20)
	if limit <= 0 {
		limit = 20
	}

	resp, err := t.svc.List(ctx, conversationuc.ListRequest{
		Store:     stringArg(args, "store"),
		Limit:     limit,
		Offset:    intArg(args, "offset", 0),
		Principal: principalFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	return marshalResult(resp)
}

// handleSummarize implements the conversation_summarize tool handler.
func (t *ConversationTools) handleSummarize(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	conversationID := stringArg(args, "conversation_id")
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	summary, err := t.svc.Summarize(ctx, conversationID, principalFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("summarize conversation: %w", err)
	}

	return marshalResult(summary)
}

// handlePromote implements the conversation_promote tool handler.
func (t *ConversationTools) handlePromote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	conversationID := stringArg(args, "conversation_id")
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	store := stringArg(args, "store")
	if store == "" {
		return nil, fmt.Errorf("store is required")
	}

	useSummary := false
	if v, ok := args["use_summary"].(bool); ok {
		useSummary = v
	}

	mem, err := t.svc.Promote(ctx, conversationuc.PromoteRequest{
		ConversationID: conversationID,
		Store:          store,
		Title:          stringArg(args, "title"),
		UseSummary:     useSummary,
		Principal:      principalFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("promote conversation: %w", err)
	}

	return marshalResult(mem)
}

// handleDelete implements the conversation_delete tool handler.
func (t *ConversationTools) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := argsMap(req)
	if args == nil {
		args = map[string]any{}
	}

	conversationID := stringArg(args, "conversation_id")
	if conversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	confirm := stringArg(args, "confirm")
	if confirm == "" {
		return nil, fmt.Errorf("confirm is required")
	}

	if err := t.svc.Delete(ctx, conversationID, confirm, principalFromContext(ctx)); err != nil {
		return nil, fmt.Errorf("delete conversation: %w", err)
	}

	return marshalResult(map[string]bool{"deleted": true})
}
