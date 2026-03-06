# ingatan Server v1.0 - Implementation Plan

**Created:** 2026-03-05
**Version:** 1.0
**Status:** Draft — to be detailed after Phase 0 research

---

## Overview

18-week implementation plan following the PRD milestone schedule. Each phase corresponds to a PRD milestone. All phases follow TDD (Red-Green-Refactor).

---

## Milestone Plan

### M0 — Foundation (Week 1-2)

**Goal:** Working skeleton: server boots, config loads, JWT auth works, file storage persists, HNSW integrates.

**Files to Create:**
```
cmd/ingatan/main.go
go.mod / go.sum
internal/domain/errors.go
internal/domain/principal.go
internal/domain/store.go
internal/infrastructure/config/config.go
internal/infrastructure/storage/file_store.go
internal/infrastructure/storage/file_store_test.go
internal/adapter/rest/server.go
internal/adapter/rest/middleware/auth.go
internal/adapter/rest/middleware/auth_test.go
internal/infrastructure/index/hnsw.go
internal/infrastructure/index/hnsw_test.go
```

**Acceptance:** Server boots, returns 401 without JWT, returns 200 on `/api/v1/health` with valid JWT.

---

### M1 — Memory Core (Week 3-4)

**Goal:** Full memory CRUD over MCP + REST.

**Files to Create:**
```
internal/domain/memory.go
internal/usecase/memory/service.go
internal/usecase/memory/service_test.go
internal/usecase/memory/types.go
internal/infrastructure/storage/memory_repository.go
internal/infrastructure/storage/memory_repository_test.go
internal/infrastructure/ingest/chunker.go
internal/infrastructure/ingest/chunker_test.go
internal/infrastructure/embed/openai.go
internal/infrastructure/embed/bedrock.go
internal/infrastructure/embed/ollama.go
internal/adapter/rest/memory_handler.go
internal/adapter/rest/memory_handler_test.go
internal/adapter/mcp/memory_tools.go
internal/adapter/mcp/memory_tools_test.go
```

**Acceptance:** All 5 memory tools (save, get, update, delete, list) pass integration tests.

---

### M2 — Search (Week 5-6)

**Goal:** HNSW semantic search, BM25 keyword search, hybrid search via RRF.

**Files to Create:**
```
internal/infrastructure/index/bm25.go
internal/infrastructure/index/bm25_test.go
internal/usecase/memory/search.go
internal/usecase/memory/search_test.go
internal/adapter/rest/search_handler.go
internal/adapter/mcp/search_tools.go  (memory_search, memory_similar)
```

**Acceptance:** `memory_search` returns ranked results with score_components. `memory_similar` uses centroid of chunk vectors.

---

### M3 — Ingest (Week 7-8)

**Goal:** URL and file ingestion with readability extraction and PDF support.

**Files to Create:**
```
internal/infrastructure/ingest/url_fetcher.go
internal/infrastructure/ingest/url_fetcher_test.go
internal/infrastructure/ingest/file_reader.go
internal/infrastructure/ingest/file_reader_test.go
internal/infrastructure/ingest/pdf_extractor.go
internal/infrastructure/ingest/pdf_extractor_test.go
internal/adapter/rest/ingest_handler.go
internal/adapter/mcp/ingest_tools.go  (memory_save_url, memory_save_file)
```

**Acceptance:** URL ingest returns readability-extracted content. PDF extraction wraps panics. Path traversal rejected.

---

### M4 — Stores & Auth (Week 9-10)

**Goal:** Full store CRUD, access control enforcement, principal management.

**Files to Create:**
```
internal/usecase/store/service.go
internal/usecase/store/service_test.go
internal/infrastructure/storage/store_repository.go
internal/usecase/principal/service.go
internal/adapter/rest/store_handler.go
internal/adapter/rest/principal_handler.go
internal/adapter/mcp/store_tools.go
internal/adapter/mcp/principal_tools.go
```

**Acceptance:** Role enforcement tested at service layer. Personal store auto-created. `store_delete` requires confirm field.

---

### M5 — Conversations (Week 11-12)

**Goal:** Full conversation management with LLM summarization.

**Files to Create:**
```
internal/domain/conversation.go
internal/usecase/conversation/service.go
internal/usecase/conversation/service_test.go
internal/infrastructure/storage/conversation_repository.go
internal/infrastructure/llm/anthropic.go
internal/infrastructure/llm/openai.go
internal/infrastructure/llm/bedrock.go
internal/infrastructure/llm/ollama.go
internal/adapter/rest/conversation_handler.go
internal/adapter/mcp/conversation_tools.go
```

**Acceptance:** Auto-summarize triggers at threshold. `conversation_promote` creates memory with `source=conversation`.

---

### ~~M6 — WebUI~~ (Dropped)

WebUI is a separate application consuming the ingatan REST API. No embedded UI in this binary.

---

### M7 — Hardening (Week 13-14)

**Goal:** OTel instrumentation, rate limiting, backup, security review.

**Files to Create/Modify:**
```
internal/adapter/rest/middleware/otel.go
internal/adapter/rest/middleware/rate_limit.go
internal/infrastructure/backup/s3.go
internal/infrastructure/backup/git.go
internal/adapter/rest/middleware/mtls.go
```

**Acceptance:** OTel spans visible in local collector. Rate limiting returns 429 at limit. S3 backup completes without error.

---

### M8 — Release (Week 15-16)

**Goal:** Full integration test suite, ARM build verification, documentation.

**Files to Create:**
```
test/integration/memory_test.go
test/integration/search_test.go
test/integration/conversation_test.go
test/integration/store_test.go
documentation/technical-details.md  (update)
documentation/product-details.md    (update)
support_docs/getting-started.md
support_docs/configuration.md
```

**Acceptance:** All quality gates pass. ARM binary builds and boots. PRD sign-off.

---

## Quality Gates (Every Phase)

```bash
go fmt ./...
go mod tidy
go vet ./...
golangci-lint run
go test ./...
go build -o bin/ingatan ./cmd/ingatan
./bin/ingatan --help
```

---

*Detailed task breakdown for each milestone will be added to `tasks.md` as each phase begins.*
