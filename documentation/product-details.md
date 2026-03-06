# ingatan - Product Details

## Memory Lifecycle

### Save

A memory is created via `memory_save` (MCP) or `POST /stores/{store}/memories` (REST).

1. **Validate** -- Title and content are required. The principal must have `owner` or `writer` role on the target store.
2. **Chunk** -- Content is split into overlapping segments using recursive chunking (default: 512 tokens, 64 overlap). Character-based token approximation.
3. **Embed** -- If an embedding provider is configured, each chunk is embedded into a float32 vector. If no provider is configured, memories are saved without vectors (keyword search only).
4. **Index** -- Chunk vectors are upserted into the per-store HNSW index. Chunk text is added to the per-store BM25 index.
5. **Persist** -- The memory record and its chunks are written to file-based JSON storage.

On the first memory saved to a store, the embedding model name and dimensions are recorded on the store record and become immutable.

### Retrieve

- **Get by ID**: Returns the full memory record (without chunk details).
- **List**: Paginated listing with optional tag and source filters.
- **Search**: Hybrid (default), semantic-only, or keyword-only search. Returns ranked memories with score components.
- **Similar**: Given a memory ID, returns semantically similar memories using its chunk vectors.

### Update

Partial updates via `memory_update` or `PUT /stores/{store}/memories/{id}`. Updatable fields: title, content, tags, metadata. When content changes, chunks are re-generated, re-embedded, and re-indexed.

### Delete

Removes the memory record, all chunks, and all index entries (both HNSW and BM25).

## Search

### Modes

| Mode | Index | Use Case |
|------|-------|----------|
| `hybrid` (default) | HNSW + BM25, fused via RRF | General-purpose retrieval |
| `semantic` | HNSW only | Meaning-based similarity |
| `keyword` | BM25 only | Exact term matching, works without embeddings |

### Reciprocal Rank Fusion (RRF)

Hybrid mode retrieves top-K results from both indexes independently, then merges them using RRF: `score = sum(1 / (k + rank))` where k=60. This balances semantic relevance with keyword precision without requiring score normalization.

### Similar Memories

`memory_similar` / `GET /stores/{store}/memories/{id}/similar` retrieves the first chunk of the target memory, then queries the HNSW index for nearest neighbors. Returns ranked memories excluding the query memory itself.

## Ingest Pipeline

### URL Ingest (`memory_save_url`)

1. Fetch URL content via HTTP GET (respects `robots.txt` if `robots_txt_compliance: true`)
2. Extract readable text using readability algorithm (strips nav, ads, boilerplate)
3. Title is extracted from the page; content is the cleaned text
4. Proceeds through standard save pipeline (chunk, embed, index, persist)
5. `source` is set to `url`; `source_url` records the original URL

### File Ingest (`memory_save_file`)

1. Validate file path against `ingest.allowed_paths` whitelist (empty list = deny all)
2. Read file content based on extension:
   - `.md`, `.txt`, `.html`, `.json`, `.go`, `.py`, `.js`, `.ts`, `.rs`, `.java`, `.c`, `.cpp`, `.h`, `.rb`, `.sh`, `.yaml`, `.yml`, `.toml`, `.xml`, `.csv`, `.sql`, `.r`, `.swift`, `.kt` -- read as UTF-8 text
   - `.pdf` -- extract text via pdfcpu (wrapped with `recover()` for stability)
3. Title defaults to the filename
4. Proceeds through standard save pipeline
5. `source` is set to `file`; `source_path` records the file path

### Size Limits

- `ingest.max_content_bytes` (default: 10 MiB) caps both URL and file content
- `chunking.max_content_bytes` (default: 10 MiB) caps content passed to the chunker

## Store Model

### Store Properties

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Globally unique slug matching `^[a-z0-9-]+$`. Immutable. |
| `description` | string | Human-readable description. |
| `owner_id` | string | Principal ID of the store creator. |
| `members` | []StoreMember | List of `{principal_id, role}` pairs. |
| `embedding_model` | string | Recorded on first memory save. Immutable. |
| `embedding_dims` | int | Recorded alongside embedding_model. |
| `created_at` | time.Time | Creation timestamp. |

### RBAC Roles

| Role | Read | Write | Delete Memories | Manage Store |
|------|------|-------|----------------|--------------|
| `owner` | Yes | Yes | Yes | Yes (delete store, manage members) |
| `writer` | Yes | Yes | Own memories | No |
| `reader` | Yes | No | No | No |

Admins (`role: admin` in JWT) bypass RBAC and can access all stores.

### Personal Stores

When a principal authenticates for the first time, `PrincipalService.GetOrCreate` automatically:
1. Creates the principal record
2. Creates a personal store named after the principal's `sub` claim
3. Assigns the principal as `owner` of that store

Personal stores cannot be deleted (guarded by `STORE_DELETE_FORBIDDEN`).

### Store Deletion

`store_delete` / `DELETE /stores/{store}` requires:
- Principal must be the store owner (or admin)
- Request body must include `{"confirm": "store-name"}` matching the store name
- Personal stores cannot be deleted

## Principal Management

### JWT Authentication

All API requests require a JWT bearer token. Claims used:

| Claim | Maps To | Required |
|-------|---------|----------|
| `sub` | Principal.ID | Yes |
| `name` | Principal.Name | No (defaults to sub) |
| `type` | Principal.Type (`human`/`agent`) | No (defaults to `human`) |
| `email` | Principal.Email | No |
| `role` | Principal.Role (`user`/`admin`) | No (defaults to `user`) |

Supported algorithms: HS256 (shared secret), RS256 (public key).

### Auto-Creation

On first authentication, the principal is persisted and a personal store is created. Subsequent requests update `last_seen_at`.

### Endpoints

- `principal_whoami` / `GET /principal/whoami` -- Returns the authenticated principal with store memberships and capabilities
- `principal_list` / `GET /principal/list` -- Admin-only listing of all principals

## Conversations

### Lifecycle

1. **Start** (`conversation_start`) -- Create a conversation with a title, optionally scoped to a store
2. **Add Messages** (`conversation_add_message`) -- Append user/assistant/system/tool messages
3. **Summarize** (`conversation_summarize`) -- Generate an LLM summary of all messages. Requires an LLM provider.
4. **Promote** (`conversation_promote`) -- Save conversation content (full transcript or summary) as a permanent memory in a store
5. **Delete** (`conversation_delete`) -- Remove conversation and all messages

### Auto-Summarization

When `AddMessage` is called, the service checks two thresholds:
- `auto_summarize_message_threshold` (default: 50) -- triggers when message count reaches this value
- `auto_summarize_token_estimate_threshold` (default: 8000) -- triggers when estimated tokens (content length / 4) reach this value

If either threshold is met and an LLM provider is configured, summarization runs automatically.

### Promotion

`conversation_promote` creates a memory from the conversation:
- `use_summary: true` -- uses the summary text (must exist)
- `use_summary: false` -- concatenates all messages as a transcript
- Source is set to `conversation`; `source_ref` links back to the conversation ID

## Admin Operations

### Backup (`POST /admin/backup`)

Admin-only endpoint that triggers configured backup providers:

- **S3**: Tars the data directory and uploads to an S3-compatible bucket (supports path-style addressing for MinIO, etc.)
- **Git**: Initializes or opens a git repository in the data directory, stages all files, commits, and optionally pushes to a remote

Returns per-provider success/failure results.

## MCP Tools (23 total)

| Category | Tools |
|----------|-------|
| Memory | `memory_save`, `memory_get`, `memory_update`, `memory_delete`, `memory_list` |
| Search | `memory_search`, `memory_similar` |
| Ingest | `memory_save_url`, `memory_save_file` |
| Store | `store_create`, `store_get`, `store_list`, `store_delete` |
| Principal | `principal_whoami`, `principal_list` |
| Conversation | `conversation_start`, `conversation_add_message`, `conversation_get`, `conversation_list`, `conversation_summarize`, `conversation_promote`, `conversation_delete` |

All MCP tools are served via streamable HTTP at `/mcp`. Authentication uses the same JWT bearer token as the REST API.

## Error Codes

| Code | HTTP Status | Description |
|------|------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid JWT |
| `FORBIDDEN` | 403 | Insufficient store role |
| `NOT_FOUND` | 404 | Memory, store, conversation, or principal not found |
| `CONFLICT` | 409 | Store name already exists |
| `CONTENT_TOO_LARGE` | 413 | Content exceeds size limit |
| `INVALID_REQUEST` | 400 | Malformed input or validation failure |
| `PATH_NOT_ALLOWED` | 403 | File path not in allowed_paths |
| `PDF_EXTRACTION_ERROR` | 500 | PDF text extraction failed |
| `STORE_DELETE_FORBIDDEN` | 403 | Cannot delete personal store |
| `EMBEDDING_ERROR` | 500 | Embedding provider failure |
| `LLM_ERROR` | 500 | LLM provider failure |
| `INTERNAL_ERROR` | 500 | Unexpected server error |
