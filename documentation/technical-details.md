# ingatan - Technical Details

## Architecture

ingatan follows Clean Architecture with strict inward-only dependency flow:

```
cmd/ingatan/main.go          -- Entry point, dependency injection
  |
  v
internal/adapter/            -- Interface adapters (REST, MCP)
  |
  v
internal/usecase/            -- Business logic, service interfaces
  |
  v
internal/domain/             -- Pure entities, no external deps
  ^
  |
internal/infrastructure/     -- Concrete implementations (storage, index, embed, llm, ingest, backup, config)
```

Inner layers define interfaces; outer layers implement them. The domain layer imports only stdlib. The usecase layer defines repository and service interfaces consumed by adapters and implemented by infrastructure.

## Layer Details

### Domain (`internal/domain/`)

Pure value types with no behavior beyond simple helpers:

- `Memory` / `MemoryChunk` -- core stored artifacts
- `Store` / `StoreMember` / `StoreRole` -- named collection with RBAC
- `Principal` / `PrincipalType` / `InstanceRole` -- authenticated identity
- `Conversation` / `Message` / `ConversationSummary` / `MessageRole` -- conversation tracking
- `VectorSearchResult` / `KeywordSearchResult` -- search result types shared across layers
- `AppError` -- structured error type with code, message, and optional details

### Use Case (`internal/usecase/`)

Each subdomain has its own package with a `Service` interface, repository interfaces, and request/response types:

| Package | Service Interface | Key Interfaces |
|---------|------------------|----------------|
| `usecase/memory` | `Service` (Save/Get/Update/Delete/List/Search/Similar/SaveURL/SaveFile) | `Repository`, `ChunkRepository`, `Chunker`, `Embedder`, `VectorIndex`, `KeywordIndex`, `URLFetcher`, `FileReader`, `StoreAccess` |
| `usecase/store` | `Service` (Create/Get/List/Delete) | `Repository` |
| `usecase/principal` | `Service` (GetOrCreate/WhoAmI/List) | defined in own package |
| `usecase/conversation` | `Service` (Start/AddMessage/Get/List/Summarize/Promote/Delete) | `Repository`, `MessageRepository`, `LLMProvider`, `MemorySaver` |

### Adapter (`internal/adapter/`)

**REST** (`adapter/rest/`): Chi HTTP router with RouteRegistrar pattern. Each domain handler implements `Register(r chi.Router)` and is passed to `rest.NewRouter()`. Middleware stack: slog logger > OTel tracing > rate limiting > JWT auth > principal enrichment.

**MCP** (`adapter/mcp/`): Tool handler structs with `Register(*server.MCPServer)` methods. Uses `mark3labs/mcp-go` v0.32.0. Served via streamable HTTP at `/mcp` (stateless mode). Helper functions: `argsMap`, `stringArg`, `intArg`, `stringSliceArg`, `marshalResult`, `principalFromContext`.

**Middleware** (`adapter/rest/middleware/`):
- `auth.go` -- JWT validation (HS256/RS256), principal lookup callback
- `otel.go` -- OpenTelemetry span creation, principal enrichment
- `rate_limit.go` -- Per-IP token bucket rate limiting (sync.Map of limiters)
- `mtls.go` -- Client CA loading, TLS client auth configuration
- `slog_logger.go` -- Structured request logging (method, path, status, duration_ms, principal_id)

### Infrastructure (`internal/infrastructure/`)

| Package | Responsibility |
|---------|---------------|
| `config/` | koanf v2 config loading: compiled defaults, YAML file, env vars (`INGATAN_` prefix) |
| `storage/` | File-based JSON persistence via `FileStore`. Repos: memory, chunk, store, principal, conversation, message |
| `index/hnsw_store.go` | Per-store HNSW vector index registry. `coder/hnsw` library. RWMutex for concurrent access. |
| `index/bm25_store.go` | Per-store BM25 keyword index registry. GOB-serialized. Mutex for all operations. |
| `embed/` | Embedding providers: OpenAI-compatible HTTP client (covers OpenAI, Ollama, Bedrock) |
| `ingest/` | `RecursiveChunker`, `HTTPURLFetcher` (readability), `MultiFileReader`, `PDFExtractor` (pdfcpu) |
| `llm/anthropic.go` | Anthropic Messages API via `anthropic-sdk-go` |
| `llm/openai.go` | OpenAI Chat Completions via `openai-go`; Ollama support via configurable BaseURL |
| `backup/s3.go` | S3 backup: tar data dir, upload via aws-sdk-go-v2 (path-style endpoint support) |
| `backup/git.go` | Git backup: go-git v5, init/open, AddGlob, commit, optional push |

## Data Storage Layout

All data lives under `data_dir` (default `~/.ingatan/data`):

```
{data_dir}/
├── principals.json                          # Array of all principals
├── stores/
│   └── {store-name}/
│       ├── store.json                       # Store metadata
│       ├── memories/
│       │   └── {memory-id}.json             # Memory record
│       ├── chunks/
│       │   └── {memory-id}/
│       │       └── {chunk-id}.json          # Chunk content (vectors in HNSW index)
│       ├── hnsw/                            # HNSW index files (binary)
│       └── bm25.gob                         # BM25 index (GOB-encoded)
└── conversations/
    ├── {conversation-id}.json               # Conversation record
    └── {conversation-id}-messages/
        └── {message-id}.json                # Individual message
```

File writes use atomic write pattern: write to temp file, then rename. This prevents corruption on crash.

## Concurrency Model

| Component | Strategy |
|-----------|----------|
| HNSW index | `sync.RWMutex` per store -- writes use `Lock()`, reads use `RLock()` |
| BM25 index | `sync.Mutex` per store -- all operations lock (not concurrent-safe internally) |
| File storage | Atomic write (temp + rename) -- safe for single-writer |
| Principal repo | Read-modify-write on `principals.json` with full file lock |
| Rate limiter | `sync.Map` of per-IP token buckets -- lock-free reads, atomic updates |

The HNSW `Search` method includes a `recover()` guard to handle panics from the underlying `coder/hnsw` library when searching a single-node graph after deletion.

## Configuration Loading

Configuration uses koanf v2 with three layers (later overrides earlier):

1. **Compiled defaults** -- hardcoded in `config.LoadConfig()`
2. **YAML file** -- `--config` flag or `~/.ingatan/config.yaml`
3. **Environment variables** -- `INGATAN_` prefix, underscore-separated (e.g., `INGATAN_SERVER_PORT=9000`)

Secrets (`auth.secret`, `embedding.api_key`, `llm.api_key`) should be set via environment variables only.

## Dependency Injection

All wiring happens in `cmd/ingatan/main.go`:

1. Load config
2. Create infrastructure: FileStore, repos, chunker, embedder, indexes, fetcher, reader
3. Create use case services: store, principal, memory, conversation
4. Create adapters: REST handlers, MCP tools
5. Wire router with middleware, mount MCP handler
6. Start HTTP server (with or without TLS)

Two adapter structs in `main.go` bridge cross-cutting concerns:
- `memorySaverAdapter` -- wraps memory service as `conversationuc.MemorySaver` (avoids usecase-to-usecase dependency)
- `storeAccessImpl` -- wraps store repository as `memoryuc.StoreAccess` (lets memory service check RBAC without importing store usecase)

## MCP Integration

Uses `mark3labs/mcp-go` v0.32.0:
- `server.NewMCPServer("ingatan", version)` creates the MCP server
- Tool structs register tools with typed handlers
- `server.NewStreamableHTTPServer(mcpSrv, server.WithStateLess(true))` creates HTTP handler
- Mounted at `/mcp` on the Chi router
- JWT auth is applied to `/mcp` routes via the same middleware stack

## OpenTelemetry

- Provider created via `apimw.NewOTelProvider(OTelConfig{Endpoint, ServiceName})`
- `"stdout"` endpoint uses stdout exporter (development)
- Any other endpoint value uses OTLP gRPC exporter (production)
- Tracer passed to router via `ServerOptions.OTelTracer`
- Middleware creates spans per request, enriches with principal ID
- Provider shutdown is deferred in `run()` for clean exit

## HTTP Server Configuration

| Setting | Value |
|---------|-------|
| ReadTimeout | 30s |
| WriteTimeout | 60s |
| IdleTimeout | 120s |
| TLS min version | 1.2 (configurable to 1.3) |
| mTLS | Optional, triggered by `server.tls.client_ca` |

## Cross-Compilation Targets

| Target | GOOS | GOARCH | Notes |
|--------|------|--------|-------|
| Raspberry Pi / ARM64 Linux | linux | arm64 | Primary deployment target |
| x86-64 Linux | linux | amd64 | Secondary target |
| macOS Apple Silicon | darwin | arm64 | Development |

Build command: `go build -o bin/ingatan ./cmd/ingatan`

## Key Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/mark3labs/mcp-go` | MCP protocol server |
| `github.com/knadh/koanf/v2` | Configuration loading |
| `github.com/coder/hnsw` | In-process HNSW vector index |
| `github.com/pdfcpu/pdfcpu` | PDF text extraction |
| `github.com/go-git/go-git/v5` | Git backup provider |
| `github.com/aws/aws-sdk-go-v2` | S3 backup provider |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic LLM provider |
| `github.com/openai/openai-go` | OpenAI embedding + LLM provider |
| `go.opentelemetry.io/otel` | Distributed tracing |
| `github.com/golang-jwt/jwt/v5` | JWT authentication |

## Admin WebUI Adapter

The Admin WebUI is a self-contained adapter in `internal/adapter/webui/` that provides a browser-based admin console.

### Security Model

- **Localhost-only**: `LocalhostOnly` middleware enforces `net.IP.IsLoopback()` on `RemoteAddr`; all non-loopback requests receive 403. No proxy trust headers are honoured.
- **Startup token**: `crypto/rand` 32-byte hex, printed once at INFO level on boot, never persisted.
- **Session cookie**: In-memory `SessionStore` (24h TTL, lost on restart). Cookie: `ingatan-admin-session`, HttpOnly, SameSite=Strict.
- **No JWT**: The `/webui/*` sub-router has its own middleware chain. The JWT middleware from `rest.NewRouter` is NOT applied.

### Routing Architecture

```
root chi.Router
├── /api/v1/*    ← rest.NewRouter (JWT required)
├── /mcp         ← MCP handler (JWT required)
└── /webui/*     ← webui.Handler (LocalhostOnly + SessionAuth)
     ├── /static/*         embedded htmx.min.js + pico.min.css
     ├── /login (GET/POST) unauthenticated
     ├── /logout (POST)    unauthenticated
     └── /* (authenticated)
          ├── /dashboard
          ├── /principals, /principals/new, /principals/{id}
          ├── /stores, /stores/{name}
          └── /system, /system/backup
```

### Template Layer

Templates use `github.com/a-h/templ` v0.3. Source files are in `internal/adapter/webui/templates/*.templ`; generated `*_templ.go` files are committed. Static assets (HTMX 2.0, Pico CSS 2.x) are embedded via `//go:embed static/*`.

- `Layout(title, content templ.Component)` — base shell with Pico CSS nav
- `LoginPage(errMsg)` — standalone login (no nav chrome)
- `DashboardContent`, `PrincipalsListContent`, `PrincipalsNewContent`, `PrincipalCreatedContent`, `PrincipalDetailContent`, `KeyReissuedContent`, `StoresListContent`, `StoreDetailContent`, `SystemContent`, `ErrorContent`

View model types (`PrincipalRow`, `StoreRow`, `PrincipalDetailData`, `StoreDetailData`, `MembershipRow`) are defined in `templates/viewmodels.go`; handlers convert domain types to view models before rendering.

### Admin Context

All WebUI service calls use a synthetic admin principal (`webui-admin`, `InstanceRoleAdmin`) constructed once in `NewHandler`. This bypasses store membership checks (the service layer honours `InstanceRoleAdmin`).

### Enabling/Disabling

Controlled by `webui.enabled` in config (default: `true`). When disabled, no routes are mounted and no startup token is generated.
