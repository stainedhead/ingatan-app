# ingatan

A self-hosted memory server for AI agents and humans. Stores, indexes, and retrieves knowledge as structured memories — enabling persistent context across conversations and sessions.

Ships as a **single Go binary** with no external runtime dependencies. File-based storage, in-process HNSW vector index and BM25 keyword index. Targets ARM64 and amd64 edge hardware (Raspberry Pi, home servers, development machines).

---

## The Name

*ingatan* comes from Bahasa Melayu, where it means **memory** or **remembrance** — the recollection of things past, held in mind.

In Tagalog, *ingatan* carries a related but distinct meaning: **to keep safe**, to guard or preserve something with care. The shared root *ingat* runs through both languages, evoking mindfulness and attention.

The name captures what this project does: it remembers, and it keeps knowledge safe.

---

## Features

- **Memory CRUD** — save, get, update, delete, list memories with automatic chunking
- **Hybrid search** — HNSW semantic + BM25 keyword via Reciprocal Rank Fusion (RRF); hybrid, semantic, or keyword modes
- **Ingest** — URL readability extraction, file ingest (Markdown, text, HTML, JSON, PDF, code)
- **Multi-store isolation** — named stores with RBAC (owner/writer/reader roles); personal store auto-created on first login
- **Conversations** — full lifecycle with LLM summarization and promotion to permanent memories
- **MCP + REST** — 23 MCP tools and a full REST API, both secured by JWT
- **Production hardening** — OTel tracing, per-IP rate limiting, mTLS, S3 and git backup, structured logging

## Quick Start

### Build

```bash
git clone https://github.com/stainedhead/ingatan.git
cd ingatan

# Current platform
go build -o bin/ingatan ./cmd/ingatan

# Raspberry Pi / ARM64
GOOS=linux GOARCH=arm64 go build -o bin/ingatan-arm64 ./cmd/ingatan
```

### Configure

```bash
mkdir -p ~/.ingatan
cp config.example.yaml ~/.ingatan/config.yaml

# Required: JWT secret (env only, never in config file)
export INGATAN_AUTH_SECRET="your-secret-key-at-least-32-chars"

# Optional: enable semantic search
export INGATAN_EMBEDDING_PROVIDER=openai
export INGATAN_EMBEDDING_MODEL=text-embedding-3-small
export INGATAN_EMBEDDING_API_KEY=sk-...
```

### Run

```bash
# Plain HTTP (development)
./bin/ingatan --config ~/.ingatan/config.yaml

# TLS (production) — set cert_file and key_file in config
./bin/ingatan --config ~/.ingatan/config.yaml
```

Server starts on `0.0.0.0:8443` by default.

### First Steps

```bash
# Health check (no auth required)
curl http://localhost:8443/api/v1/health

# Issue a JWT (HS256, sub must be [a-z0-9-]+)
TOKEN=$(jwt-cli encode --secret "$INGATAN_AUTH_SECRET" \
  --sub "alice" --name "Alice" --type "human" --role "user" \
  --exp "+1h")

# Whoami (auto-creates principal + personal store on first call)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8443/api/v1/principal/whoami

# Save a memory
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Go concurrency","content":"Goroutines are lightweight threads managed by the Go runtime.","tags":["go","concurrency"]}' \
  http://localhost:8443/api/v1/stores/alice/memories

# Search
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"goroutines","mode":"keyword","top_k":5}' \
  http://localhost:8443/api/v1/stores/alice/memories/search
```

## MCP Integration

### Claude Desktop

```json
{
  "mcpServers": {
    "ingatan": {
      "url": "http://localhost:8443/mcp",
      "headers": {
        "Authorization": "Bearer <your-jwt>"
      }
    }
  }
}
```

### Claude Code

```bash
claude mcp add ingatan --transport http \
  --header "Authorization: Bearer <your-jwt>" \
  http://localhost:8443/mcp
```

## MCP Tools (23)

| Category | Tools |
|----------|-------|
| Memory | `memory_save`, `memory_get`, `memory_update`, `memory_delete`, `memory_list` |
| Search | `memory_search`, `memory_similar` |
| Ingest | `memory_save_url`, `memory_save_file` |
| Store | `store_create`, `store_get`, `store_list`, `store_delete` |
| Principal | `principal_whoami`, `principal_list` |
| Conversation | `conversation_start`, `conversation_add_message`, `conversation_get`, `conversation_list`, `conversation_summarize`, `conversation_promote`, `conversation_delete` |

## REST API

All routes under `/api/v1`, require `Authorization: Bearer <jwt>`.

```
GET    /health
POST   /stores/{store}/memories
GET    /stores/{store}/memories
GET    /stores/{store}/memories/{id}
PUT    /stores/{store}/memories/{id}
DELETE /stores/{store}/memories/{id}
POST   /stores/{store}/memories/search
GET    /stores/{store}/memories/{id}/similar
POST   /stores/{store}/memories/url
POST   /stores/{store}/memories/file
POST   /stores
GET    /stores
GET    /stores/{store}
DELETE /stores/{store}
GET    /principal/whoami
GET    /principal/list          (admin only)
POST   /conversations
GET    /conversations
GET    /conversations/{id}
POST   /conversations/{id}/messages
POST   /conversations/{id}/summarize
POST   /conversations/{id}/promote
DELETE /conversations/{id}
POST   /admin/backup            (admin only)
```

## Configuration

See [`config.example.yaml`](config.example.yaml) for all options with defaults.
All fields are overridable via `INGATAN_*` environment variables (e.g. `INGATAN_SERVER_PORT=9000`).

Full reference: [`support_docs/configuration.md`](support_docs/configuration.md)

## Documentation

| Document | Audience |
|----------|----------|
| [`documentation/product-summary.md`](documentation/product-summary.md) | Overview |
| [`documentation/product-details.md`](documentation/product-details.md) | Features and data model |
| [`documentation/technical-details.md`](documentation/technical-details.md) | Architecture and internals |
| [`support_docs/getting-started.md`](support_docs/getting-started.md) | Install and first use |
| [`support_docs/configuration.md`](support_docs/configuration.md) | Full config reference |

## Development

```bash
# Run tests
go test ./...

# Full quality gate
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./...

# Build
go build -o bin/ingatan ./cmd/ingatan
```

Requires Go 1.22+. All dependencies are pure Go — no CGO, cross-compiles cleanly.

## License

See [LICENSE](LICENSE).
