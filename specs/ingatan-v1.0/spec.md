# ingatan Server v1.0 - Specification

**Created:** 2026-03-05
**Version:** 1.0
**Status:** Draft
**Source PRD:** `ingatan-PRD-v1.0`

---

## Executive Summary

ingatan (Tagalog: "to remember") is a self-hosted, zero-external-dependency second brain service built for agentic workflows. It provides persistent semantic memory, conversation tracking, and hybrid full-text search accessible via three protocol surfaces: the Model Context Protocol (MCP), a REST API, and an embedded web UI. A companion CLI tool (`ingat`) consumes the REST API as a thin client.

The system runs as a single Go binary deployable on commodity hardware — including ARM-based edge nodes — without requiring any external databases, search engines, or cloud services. All storage, vector indexing, and keyword indexing is file-based and in-process. Remote embedding and LLM inference are supported through OpenAI-compatible APIs, Amazon Bedrock, and Ollama (local), with pluggable provider selection at configuration time.

**Key Deliverables:**
- Single Go binary (`ingatan`) exposing MCP, REST API, and embedded WebUI
- Companion CLI (`ingat`) consuming the REST API
- 23 MCP tools covering memory, store, conversation, and system domains
- File-based storage with HNSW vector index and BM25 keyword index
- JWT and mTLS authentication
- Pluggable embedding (OpenAI-compatible, Bedrock, Ollama) and LLM (Anthropic, OpenAI, Bedrock, Ollama) providers

**Timeline:** 18 weeks (M0–M8 milestones)

---

## Design Mandate

- Single binary. No external services required at runtime.
- ARM-first deployment target (aarch64), with amd64 support.
- MCP, REST, and CLI surfaces are thin adapters over a shared service layer.
- All business logic lives in the service layer — never in transport handlers.
- Dependency minimization: prefer stdlib and in-process implementations over external services.

---

## Problem Statement

### Current State
AI agents lack a persistent, self-hosted memory store that is accessible via standard protocols (MCP), requires no cloud dependencies, and can run on edge hardware.

### Pain Points
- AI agents lose context between sessions — no persistent semantic memory
- Existing solutions require external databases, cloud services, or complex infrastructure
- No standard MCP-compatible memory server available for self-hosted deployments
- Long conversations exceed context windows with no summarization and promotion workflow
- No hybrid search combining semantic and keyword retrieval in a single deployable binary

### Desired State
A single binary that agents and humans can deploy anywhere, providing persistent memory with hybrid search, conversation management, and multi-store tenancy — all accessible via MCP, REST, and a browser UI.

---

## Goals and Non-Goals

### Goals
- Persistent semantic memory store accessible to AI agents via MCP tools
- Hybrid search (semantic + BM25 keyword) over stored memories
- Conversation tracking with LLM-generated summaries for long-context management
- Conversation content promotable into persistent memory
- REST API surface-identical to the MCP tool set
- Multi-store tenancy with principal-based access control (reader / writer / owner)
- Human principals (JWT-authenticated) and agent principals (JWT-authenticated)
- Content ingestion from raw text, URLs (readability extraction), and local files (including PDF)
- Single binary, no external runtime dependencies
- Pluggable embedding providers: OpenAI-compatible, Bedrock, Ollama
- Pluggable LLM providers for summarization: Anthropic, OpenAI, Bedrock, Ollama
- Structured OpenTelemetry traces and metrics
- mTLS support for secure agent-to-service communication
- Backup interface (S3-compatible and git-based)

### Non-Goals (v1)
- Multi-instance clustering or distributed storage
- Real-time collaborative editing of memories
- Image, audio, or video memory storage (text-only)
- Async job polling API (large-file ingestion is synchronous)
- Principal-level allowed-path configuration for file ingestion (instance-level only)
- Multi-agent conversation threading (single-author only)
- Built-in OAuth2 identity provider
- GraphQL API surface

---

## User Requirements

### Functional Requirements

#### FR-001: Memory Management (9 MCP Tools)
**Priority:** P0 (Critical)

**Description:** Store, retrieve, update, delete, list, and search memories. Support manual text, URL ingest (readability), and file ingest (PDF, markdown, text, HTML, JSON, code).

**Tools:** `memory_save`, `memory_search`, `memory_get`, `memory_update`, `memory_delete`, `memory_list`, `memory_similar`, `memory_save_url`, `memory_save_file`

**Acceptance Criteria:**
- [ ] Content chunked and embedded server-side on save
- [ ] Hybrid search (semantic + BM25, fused via RRF) returns ranked results with score components
- [ ] URL fetch uses readability extraction (article mode default)
- [ ] File ingest supports .md, .txt, .html, .htm, .pdf, .json, code files
- [ ] PDF extraction wraps pdfcpu panics via `recover()` and returns structured error
- [ ] `memory_save_file` validates paths against `allowed_paths` config
- [ ] Max content size enforced at 10MB (configurable)

#### FR-002: Store Management (4 MCP Tools)
**Priority:** P0 (Critical)

**Description:** Create, list, get, and delete named stores. Stores are the primary authorization boundary.

**Tools:** `store_list`, `store_get`, `store_create`, `store_delete`

**Acceptance Criteria:**
- [ ] Store name matches `[a-z0-9-]+`, globally unique, immutable after creation
- [ ] Personal store auto-created for each principal on registration
- [ ] Personal stores cannot be deleted
- [ ] `store_delete` requires `confirm` field equal to store name
- [ ] Store deletion removes all memory JSON, chunk files, HNSW segments, BM25 gob

#### FR-003: Conversation Management (6 MCP Tools)
**Priority:** P1 (High)

**Description:** Start conversations, append messages, retrieve threads, summarize with LLM, promote to persistent memory, delete.

**Tools:** `conversation_start`, `conversation_add_message`, `conversation_get`, `conversation_list`, `conversation_summarize`, `conversation_promote`, `conversation_delete`

**Acceptance Criteria:**
- [ ] Auto-summarization triggers at configured message/token thresholds
- [ ] Summarization runs asynchronously; callers poll via `conversation_get`
- [ ] `conversation_promote` with `use_summary=true` promotes summary text, not full transcript
- [ ] Only conversation owner may append messages
- [ ] `conversation_delete` requires `confirm` field; does not affect promoted memories

#### FR-004: System & Principal Domain (4 MCP Tools)
**Priority:** P0 (Critical)

**Tools:** `principal_whoami`, `principal_list`, `system_health`

**Acceptance Criteria:**
- [ ] `principal_whoami` returns identity, store membership, and derived capabilities
- [ ] `principal_list` returns 403 for non-admin callers
- [ ] `system_health` returns provider info, store/principal counts, uptime, chunk config

#### FR-005: REST API
**Priority:** P0 (Critical)

**Description:** REST API surface-identical to MCP tool set. Every MCP tool maps to exactly one REST endpoint. Business logic never duplicated.

**Acceptance Criteria:**
- [ ] Base path `/api/v1`
- [ ] Bearer JWT in `Authorization` header
- [ ] Error format: `{error: {code: string, message: string, details?: object}}`
- [ ] Timestamps ISO 8601 UTC
- [ ] Pagination via `limit` + `offset`; `total` in response envelope

#### FR-006: Authentication & Security
**Priority:** P0 (Critical)

**Acceptance Criteria:**
- [ ] JWT validation (HS256 or RS256)
- [ ] JWT claims: `sub`, `name`, `type`, `role`, `exp`
- [ ] mTLS optional, configured per-listener
- [ ] TLS required; plaintext HTTP not supported; minimum TLS 1.2
- [ ] `memory_save_file` path traversal (`../`) detection and rejection

#### FR-007: Admin WebUI (re-enabled as M9)

An embedded, admin-only browser UI served at `/webui`. Restricted to localhost access
only. Secured by a one-time startup token (separate from JWT). Configurable via
`webui.enabled` (default: true).

**Screens:** Login, Dashboard (health/counts), Principals (CRUD), Stores (list/delete),
System (config summary, backup trigger).

**Security:** localhost-only middleware (`net.IP.IsLoopback()`); session cookie
(`ingatan-admin-session`); startup token from `crypto/rand`; admin context for all
service calls.

See `specs/admin-webui/spec.md` for full detail.

#### FR-007: Observability
**Priority:** P2 (Medium)

**Acceptance Criteria:**
- [ ] OTel spans on all HTTP handlers and service methods
- [ ] Span attributes: `principal_id`, `store_name`, `tool_name`, `memory_id`, `error`
- [ ] Metrics: `ingatan_requests_total`, `ingatan_request_duration_ms`, `ingatan_memories_total`, `ingatan_chunks_total`, `ingatan_embedding_duration_ms`
- [ ] Structured JSON logging via `log/slog`; no sensitive data in logs

### Non-Functional Requirements

#### NFR-001: Single Binary Deployment
**Category:** Usability
**Description:** Must compile to a single static binary with no external runtime dependencies.
**Metrics:**
- Binary runs without external services on ARM (aarch64) and amd64

#### NFR-002: ARM-First
**Category:** Performance / Compatibility
**Description:** Primary build target is `GOOS=linux GOARCH=arm64`. All dependencies must be ARM-compatible.
**Metrics:**
- HNSW library (`coder/hnsw`) confirmed ARM-compatible

#### NFR-003: File-Based Storage
**Category:** Reliability
**Description:** All data stored as files under `$DATA_DIR`. No external database.
**Metrics:**
- All reads/writes go through file-based JSON + HNSW segments + BM25 gob

#### NFR-004: Embedding Model Consistency
**Category:** Reliability
**Description:** Embedding model name and dimensions recorded in store metadata on first write. ingatan refuses to start if configured model mismatches recorded model.

---

## System Architecture

### Affected Layers
- [x] Domain Layer — Store, Memory, MemoryChunk, Conversation, Message, Principal entities
- [x] Use Case Layer — MemoryService, StoreService, ConversationService, PrincipalService, SystemService
- [x] Infrastructure Layer — HNSW index, BM25 index, file storage, embedding/LLM providers
- [x] Adapter Layer — MCP server, REST API (Chi router)

### Component Map

| Layer | Components | Responsibility |
|-------|-----------|----------------|
| Transport | MCP Server, REST API (Chi) | Protocol translation only. No business logic. |
| Service | MemoryService, StoreService, ConversationService, PrincipalService, SystemService | All business logic, validation, orchestration. |
| Index | HNSW Vector Index (coder/hnsw), BM25 In-Memory Index (crawlab-team/bm25) | Semantic and keyword search. Per-store instances. |
| Ingest | Chunker, Embedder, URL Fetcher, File Reader, PDF Extractor | Content normalization and vectorization pipeline. |
| Storage | File-based JSON store, HNSW segment files, BM25 gob files | Durable persistence. No external database. |
| Cross-cutting | Auth (JWT/mTLS), OTel Tracing, Config (koanf/v2) | Applied via middleware and service initialization. |

### Process & Deployment Model
- Single OS process, single HTTPS listener (default `:8443`)
- MCP traffic: `POST /mcp` (streamable HTTP transport)
- REST API: `/api/v1/*`
- All traffic TLS (minimum 1.2); mTLS optional
- Data directory: `~/.ingatan/data` (configurable)

### Request Lifecycle
1. TLS termination
2. Middleware: OTel span → JWT/mTLS auth → principal injection → rate limiting
3. Transport handler parses and validates request
4. Handler calls service method (validated input struct, never raw request data)
5. Service executes business logic, interacts with indexes/storage
6. Handler serializes result to wire format (MCP JSON-RPC or HTTP JSON)
7. OTel span closed

---

## Scope of Changes

### Files to Create
- `cmd/ingatan/main.go` — Entry point, dependency injection
- `internal/domain/` — Store, Memory, MemoryChunk, Conversation, Message, Principal entities
- `internal/usecase/` — Service interfaces and implementations
- `internal/adapter/mcp/` — MCP tool handlers
- `internal/adapter/rest/` — Chi HTTP handlers
- `internal/infrastructure/storage/` — File-based JSON persistence
- `internal/infrastructure/index/` — HNSW + BM25 index wrappers
- `internal/infrastructure/embed/` — Embedding provider adapters
- `internal/infrastructure/llm/` — LLM provider adapters
- `internal/infrastructure/ingest/` — Chunker, URL fetcher, file reader, PDF extractor

### Dependencies (Key)
- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/knadh/koanf/v2` — Configuration
- `github.com/modelcontextprotocol/go-sdk` — MCP (pin to commit hash until stable)
- `github.com/coder/hnsw` — HNSW vector index (ARM-compatible)
- `github.com/crawlab-team/bm25` — BM25 keyword index
- `github.com/golang-jwt/jwt/v5` — JWT authentication
- `github.com/anthropics/anthropic-sdk-go` — Anthropic LLM provider
- `github.com/openai/openai-go` — OpenAI-compatible embedding + LLM
- `github.com/aws/aws-sdk-go-v2` — Bedrock + S3
- `github.com/ollama/ollama/api` — Ollama local provider
- `codeberg.org/readeck/go-readability/v2` — HTML readability extraction
- `github.com/pdfcpu/pdfcpu` — PDF text extraction (Alpha — wrap with `recover()`)
- `github.com/jonathanhecl/chunker` — Text chunking
- `go.opentelemetry.io/otel` — Distributed tracing and metrics
- `github.com/go-git/go-git/v5` — Git-based backup

---

## Data Model Summary

| Entity | Key Fields |
|--------|-----------|
| Store | name (slug, unique), description, owner_id, members (role pairs) |
| Memory | id (UUID), store, title, content, tags, source, source_ref/path/url, metadata, created_at/updated_at |
| MemoryChunk | chunk_id, memory_id, store, chunk_index, content, vector (float32) |
| Conversation | conversation_id, title, store, owner_id, message_count, summary |
| Message | message_id, conversation_id, role (user/assistant/system/tool), content, metadata |
| Principal | id (UUID), name, type (human/agent), email, role (user/admin) |

---

## Authorization Model

### Role Hierarchy
| Role | Scope | Capabilities |
|------|-------|-------------|
| admin | Instance | All operations. Principal management, cross-store access. |
| user | Instance | Default. Authenticate, create stores, manage own conversations. |
| owner | Store | Full control: read, write, delete, manage members. |
| writer | Store | Read + write memories. Cannot delete store or manage members. |
| reader | Store | Read memories and search. Cannot write or delete. |

---

## Storage Layout

```
$DATA_DIR/
  config.yaml
  principals.json
  stores/
    {store-name}/
      store.json
      memories/
        {memory-id}.json
        {memory-id}-chunks.json
      hnsw/              # HNSW vector index segment files
      bm25.gob           # BM25 term-frequency corpus
  conversations/
    {conversation-id}.json
    {conversation-id}-messages/
      {message-id}.json
  backups/
```

---

## Breaking Changes
None — this is the initial v1.0 implementation.

---

## Success Criteria

### Acceptance Criteria
- [ ] All 23 MCP tools implemented and tested
- [ ] Full REST API implemented (surface-identical to MCP tools)
- [ ] JWT auth + mTLS support operational
- [ ] Hybrid search (semantic + BM25 + RRF) functional
- [ ] Conversation summarization with Anthropic/OpenAI/Bedrock/Ollama providers
- [ ] File and URL ingestion (including PDF with panic recovery)
- [ ] S3 and git backup integration
- [ ] ARM cross-compile verified
- [ ] OTel instrumentation complete

### Quality Gates
- [ ] All tests pass (`go test ./...`)
- [ ] Code coverage >80%
- [ ] `go fmt ./...` clean
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean
- [ ] `go build -o bin/ingatan ./cmd/ingatan` succeeds
- [ ] Binary runs on ARM (aarch64) and amd64

---

## Risks and Mitigation

### Risk 1: pdfcpu Alpha stability
**Likelihood:** Medium | **Impact:** Medium
**Mitigation:** Wrap all pdfcpu calls with `recover()`. Return structured error `PDF_EXTRACTION_ERROR`. Log failed file. Document as known limitation.

### Risk 2: BM25 index rebuild latency on startup for large stores
**Likelihood:** Low | **Impact:** Medium
**Mitigation:** Pre-serialize BM25 gob on every write. Cold rebuild from chunks is degraded-but-functional fallback. Log rebuild time at INFO level.

### Risk 3: Embedding model change invalidates all vectors
**Likelihood:** Low | **Impact:** High
**Mitigation:** Record model name + dimensions in store metadata on first write. Reject startup if mismatch. Document migration path: `ingat admin re-embed <store>`.

### Risk 4: Single-process HNSW — no concurrent write safety
**Likelihood:** Medium | **Impact:** Medium
**Mitigation:** Serialize all write operations to a per-store mutex. Reads are concurrent-safe.

### Risk 5: MCP Go SDK instability (marked unstable until mid-2026)
**Likelihood:** Medium | **Impact:** Medium
**Mitigation:** Pin to a specific commit hash in `go.mod`. `mark3labs/mcp-go` available as fallback.

---

## Timeline and Milestones

| Milestone | Deliverable | Target |
|-----------|-------------|--------|
| M0 — Foundation | `go.mod`, chi server, koanf config, JWT middleware, file storage layer, HNSW integration test | Week 2 |
| M1 — Memory Core | `memory_save`, `memory_get`, `memory_update`, `memory_delete`, `memory_list` (MCP + REST) | Week 4 |
| M2 — Search | HNSW semantic search, BM25 index, hybrid search via RRF, `memory_search`, `memory_similar` | Week 6 |
| M3 — Ingest | `memory_save_url` (readability), `memory_save_file` (PDF + text), chunker, source provenance | Week 8 |
| M4 — Stores & Auth | Store CRUD, access control enforcement, `principal_whoami`, `principal_list` | Week 10 |
| M5 — Conversations | All 6 conversation tools, auto-summarization, `conversation_promote` | Week 12 |
| M9 — Admin WebUI | Admin-only embedded UI (localhost, startup token, templ+HTMX) | Post-v1.0 |
| M7 — Hardening | OTel instrumentation, structured logging, rate limiting, backup, security review, mTLS | Week 14 |
| M8 — v1.0 Release | Full integration test suite, ARM cross-compile, documentation, PRD sign-off | Week 16 |

**Total Estimated Duration:** 16 weeks (was 18; M6 WebUI removed)

### v1.1 Scope (Post-Release)
- Async ingestion job API for large files and slow URLs
- Principal-level `allowed_paths` for `memory_save_file`
- `memory_export` / `memory_import` for bulk operations
- WebUI: file upload, admin principal management
- BM25 in-package implementation to eliminate `crawlab-team/bm25` dependency

---

## Error Code Reference

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| UNAUTHORIZED | 401 | Missing or invalid JWT / mTLS certificate |
| FORBIDDEN | 403 | Valid principal but insufficient role |
| NOT_FOUND | 404 | Resource does not exist |
| CONFLICT | 409 | Uniqueness violation (e.g., store name exists) |
| CONTENT_TOO_LARGE | 413 | Content exceeds `max_content_bytes` |
| INVALID_REQUEST | 422 | Missing required parameters or validation failure |
| PATH_NOT_ALLOWED | 403 | `memory_save_file` path outside `allowed_paths` |
| PDF_EXTRACTION_ERROR | 422 | pdfcpu failed to extract text |
| STORE_DELETE_FORBIDDEN | 403 | Attempt to delete a personal store |
| EMBEDDING_ERROR | 503 | Embedding provider error or unavailable |
| LLM_ERROR | 503 | LLM provider error during summarization |
| INTERNAL_ERROR | 500 | Unexpected server error |

---

## References

- **Source PRD:** `ingatan-PRD-v1.0`
- **Companion CLI PRD:** `PRD-ingat-v1.0`
- **WebUI PRD:** `PRD-webui-v1.0`
- **MCP Protocol Spec:** modelcontextprotocol.io
- **A2A Protocol Spec:** a2a.dev/protocol
