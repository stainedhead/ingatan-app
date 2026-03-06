# ingatan - Product Summary

## What is ingatan?

ingatan is a self-hosted memory server for AI agents and humans. It stores, indexes, and retrieves knowledge as structured memories, enabling persistent context across conversations and sessions.

It ships as a single Go binary with no external runtime dependencies. Storage is file-based JSON, and search indexes (HNSW vector + BM25 keyword) run in-process. It targets ARM64 and amd64 edge hardware -- Raspberry Pi, home servers, and development machines.

## Who is it for?

- **AI agent developers** who need persistent memory for their agents across sessions
- **Knowledge workers** who want a personal, self-hosted knowledge base with semantic search
- **Teams** running multiple agents that share knowledge through isolated stores with RBAC

## Key Capabilities

**Memory Management** -- Save, retrieve, update, and delete memories. Content is automatically chunked and optionally embedded for vector search. Sources include manual input, URL extraction (with readability parsing), file ingest (Markdown, text, HTML, JSON, PDF, code), and conversation promotion.

**Hybrid Search** -- Combines HNSW vector similarity (semantic) and BM25 keyword matching via Reciprocal Rank Fusion (RRF). Supports hybrid, semantic-only, and keyword-only modes. Find similar memories by ID.

**Multi-Store Isolation** -- Named stores provide isolated memory collections with independent indexes. Role-based access control (owner/writer/reader) governs per-store permissions. Each authenticated principal gets an auto-created personal store.

**Conversation Tracking** -- Full conversation lifecycle: start, add messages, summarize (via LLM), and promote conversations to permanent memories. Auto-summarization triggers at configurable message/token thresholds.

**Dual Protocol Access** -- REST API (Chi HTTP router) and MCP (Model Context Protocol) tools via streamable HTTP. 23 MCP tools cover the full feature set. Both protocols share the same JWT authentication.

**Production Hardening** -- OpenTelemetry tracing, per-IP rate limiting, mTLS support, S3 and git backup providers, structured JSON request logging.

## Integration Points

- **MCP clients**: Claude Desktop, Claude Code, or any MCP-compatible agent connects via streamable HTTP at `/mcp`
- **REST clients**: Any HTTP client (curl, SDKs, custom apps) uses the `/api/v1` REST API
- **Embedding providers**: OpenAI API, Amazon Bedrock, Ollama (OpenAI-compatible)
- **LLM providers**: Anthropic, OpenAI, Ollama (for conversation summarization)
- **Backup targets**: S3-compatible object storage, git repositories

## Non-Goals

- ingatan is not a vector database -- it is an application server with embedded indexes
- It does not provide a web UI (planned as a separate application)
- It does not manage LLM conversations directly -- it stores and retrieves context for agents that do
