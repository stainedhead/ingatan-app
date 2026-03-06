# ingatan Server v1.0 - Status Tracking

**Project:** ingatan Server v1.0 Implementation
**Version:** 1.0
**Created:** 2026-03-05
**Last Updated:** 2026-03-05 (Phase 7 M7 Hardening complete)

---

## Overall Progress

**Status:** 🟡 In Progress
**Completion:** 90% (Phases 0–7 complete; M8 Release remaining)
**Estimated Total Time:** 14 weeks (was 18; M6 WebUI removed from ingatan-app scope)
**Time Spent:** ~14 hours
**Current Phase:** Phase 7 - Hardening (M7)

---

## Phase Status

| Phase | Status | Tasks | Completed | Percentage | Est. Time |
|-------|--------|-------|-----------|------------|-----------|
| **Phase 0: Planning** | ✅ Complete | 8 | 8 | 100% | 1-2h |
| **Phase 1: Foundation (M0)** | ✅ Complete | 8 | 8 | 100% | 2w |
| **Phase 2: Memory Core (M1)** | ✅ Complete | 9 | 9 | 100% | 2w |
| **Phase 3: Search (M2)** | ✅ Complete | 5 | 5 | 100% | 2w |
| **Phase 4: Ingest (M3)** | ✅ Complete | 6 | 6 | 100% | 2w |
| **Phase 5: Stores & Auth (M4)** | ✅ Complete | 8 | 8 | 100% | 2w |
| **Phase 6: Conversations (M5)** | ✅ Complete | 6 | 6 | 100% | 2w |
| ~~**Phase 7: WebUI (M6)**~~ | 🚫 Dropped | — | — | — | — |
| **Phase 7: Hardening (M7)** | ✅ Complete | 6 | 6 | 100% | 2w |
| **Phase 8: Release (M8)** | ⬜ Not Started | N | 0 | 0% | 2w |

---

## Phase 0: Planning

**Status:** ✅ Complete
**Progress:** 8/8 tasks (100%)
**Time Spent:** ~2 hours

### Tasks

- [x] **P0.1** - PRD reviewed and spec.md created
- [x] **P0.2** - Research phase: existing code exploration, dependency validation (codebase empty; all deps pinned; MCP SDK → mark3labs/mcp-go v0.32.0)
- [x] **P0.3** - Data dictionary completed: all domain types, repository interfaces, service interfaces, MCP tool schemas, config types
- [x] **P0.4** - Architecture document completed: component diagram, layer responsibilities, concurrency model, 5 sequence diagrams
- [x] **P0.5** - Implementation plan finalized: file-level breakdown per milestone (M0–M8)
- [x] **P0.6** - Tasks breakdown: detailed M0 TDD tasks (M0.1–M0.8) with Red/Green/Refactor steps
- [x] **P0.7** - ARM compatibility confirmed: all deps pure Go, no CGO
- [x] **P0.8** - Status updated

**Deliverables:**
- [x] `ingatan-PRD-v1.0`
- [x] `specs/ingatan-v1.0/spec.md`
- [x] `specs/ingatan-v1.0/research.md` — Complete (deps pinned, Q1–Q4 resolved)
- [x] `specs/ingatan-v1.0/data-dictionary.md` — Complete (all interfaces, service types, MCP schemas)
- [x] `specs/ingatan-v1.0/architecture.md` — Complete (sequence diagrams added)
- [x] `specs/ingatan-v1.0/plan.md` — Complete (M0–M8 file-level breakdown)
- [x] `specs/ingatan-v1.0/tasks.md` — Complete (M0 detailed TDD tasks)
- [x] `specs/ingatan-v1.0/status.md` (this file)
- [x] `specs/ingatan-v1.0/implementation-notes.md` (placeholder)

---

## Phase 1: Foundation — M0 (Week 1-2)

**Status:** ✅ Complete
**Progress:** 8/8 tasks (100%)

### Tasks

- [x] **M0.1** - go.mod init + all deps + .golangci.yml (golangci-lint v2 format)
- [x] **M0.2** - Domain layer: errors.go, principal.go, store.go, memory.go, conversation.go
- [x] **M0.3** - Config loading: koanf v2 + confmap defaults + env override (TDD — 4 tests pass)
- [x] **M0.4** - FileStore: atomic write-temp-rename, Read/Delete/List (TDD — 6 tests pass)
- [x] **M0.5** - JWT middleware: HS256, principal injection (TDD — 6 tests pass)
- [x] **M0.6** - Chi server + /api/v1/health endpoint (TDD — 3 tests pass)
- [x] **M0.7** - HNSW index wrapper: per-store RWMutex, save/load, concurrent reads (TDD — 7 tests pass)
- [x] **M0.8** - All quality gates pass; ARM64 binary builds to bin/ingatan-arm64 (10MB)

**Deliverables:**
- [x] `go.mod` + `go.sum` + `.golangci.yml`
- [x] `config.example.yaml`
- [x] `internal/domain/` — 5 entity files (errors, principal, store, memory, conversation)
- [x] `internal/infrastructure/config/config.go` + config_test.go
- [x] `internal/infrastructure/storage/file_store.go` + file_store_test.go
- [x] `internal/adapter/rest/middleware/auth.go` + auth_test.go
- [x] `internal/adapter/rest/server.go` + server_test.go
- [x] `internal/infrastructure/index/hnsw.go` + hnsw_test.go
- [x] `cmd/ingatan/main.go`
- [x] `bin/ingatan` (darwin/arm64, 11MB)
- [x] `bin/ingatan-arm64` (linux/arm64, 10MB)

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... ✅ build ✅ --help ✅ ARM64

**Dependencies:** Phase 0 complete ✅
**Priority:** P0 (Critical)

---

## Phase 2: Memory Core — M1 (Week 3-4)

**Status:** ✅ Complete
**Progress:** 9/9 tasks (100%)

### Tasks

- [x] **M1.1** - Add M1 deps: mark3labs/mcp-go v0.32.0, openai-go v0.1.0-alpha.62, jonathanhecl/chunker v0.0.1
- [x] **M1.2** - Use case types: Service/Repository/ChunkRepository/Chunker/Embedder interfaces + request/response types
- [x] **M1.3** - Memory service: TDD implementation (13 tests pass)
- [x] **M1.4** - Memory repository + chunk repository: file-based JSON storage (11 tests pass)
- [x] **M1.5** - Chunker: RecursiveChunker wrapping jonathanhecl/chunker (5 tests pass)
- [x] **M1.6** - OpenAI embedder: mock HTTP server TDD, float64→float32 conversion (7 tests pass)
- [x] **M1.7** - REST memory handler: 5 routes + error mapping (8 tests pass)
- [x] **M1.8** - MCP memory tools: 5 tools registered on MCPServer (8 tests pass)
- [x] **M1.9** - Wire in main.go (FileStore → MemoryRepo/ChunkRepo → chunker → embedder → service → handler + MCP mount); all quality gates pass

**Deliverables:**
- [x] `internal/usecase/memory/types.go` — Service/Repository interfaces + request/response types
- [x] `internal/usecase/memory/service.go` + `service_test.go` — 13 tests
- [x] `internal/infrastructure/storage/memory_repository.go` + `memory_repository_test.go` — 11 tests
- [x] `internal/infrastructure/ingest/chunker.go` + `chunker_test.go` — 5 tests
- [x] `internal/infrastructure/embed/openai.go` + `openai_test.go` — 7 tests
- [x] `internal/adapter/rest/memory_handler.go` + `memory_handler_test.go` — 8 tests
- [x] `internal/adapter/mcp/memory_tools.go` + `memory_tools_test.go` — 8 tests
- [x] `cmd/ingatan/main.go` — wired with all M1 components + MCP server mount

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... (62 tests) ✅ build ✅ --help ✅ ARM64 (13MB)

---

## Phase 3: Search — M2 (Week 5-6)

**Status:** ✅ Complete
**Progress:** 5/5 tasks (100%)

### Tasks

- [x] **M2.1** - Domain types (`VectorSearchResult`, `KeywordSearchResult`) + Update `HNSWIndex.Search` return type
- [x] **M2.2** - In-house BM25 index (`BM25Index` + gob persistence) + per-store registries (`HNSWStore`, `BM25Store`)
- [x] **M2.3** - Search service: `VectorIndex`/`KeywordIndex` interfaces + `Search`/`Similar` on serviceImpl with RRF hybrid fusion
- [x] **M2.4** - REST search handler (`POST /stores/{store}/memories/search`, `GET /stores/{store}/memories/{memoryID}/similar`) + MCP tools (`memory_search`, `memory_similar`)
- [x] **M2.5** - Wire in main.go (HNSWStore + BM25Store → NewService) + all quality gates pass + ARM64 build

**Deliverables:**
- [x] `internal/domain/memory.go` — Added `VectorSearchResult`, `KeywordSearchResult`
- [x] `internal/infrastructure/index/bm25.go` + `bm25_test.go` — In-house BM25 (7 tests)
- [x] `internal/infrastructure/index/hnsw_store.go` — Per-store HNSW registry
- [x] `internal/infrastructure/index/bm25_store.go` — Per-store BM25 registry
- [x] `internal/usecase/memory/types.go` — `SearchMode`, `SearchRequest`, `SearchResponse`, `SearchResult`, `ScoreComponents`, `SimilarRequest`, `VectorIndex`, `KeywordIndex` interfaces; `Search`/`Similar` added to `Service`
- [x] `internal/usecase/memory/service.go` — Index hooks in Save/Update/Delete; `NewService` updated
- [x] `internal/usecase/memory/search.go` — `Search` + `Similar` with RRF fusion, centroid search, tag filtering
- [x] `internal/usecase/memory/search_test.go` — 14 tests
- [x] `internal/adapter/rest/search_handler.go` + `search_handler_test.go` — 5 tests
- [x] `internal/adapter/mcp/search_tools.go` — `memory_search` + `memory_similar` tools

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... (all pass) ✅ build ✅ --help ✅ ARM64 (13MB)

---

## Phase 4: Ingest — M3 (Week 7-8)

**Status:** ✅ Complete
**Progress:** 6/6 tasks (100%)

### Tasks

- [x] **M3.1** - Add deps: go-shiori/go-readability + ledongthuc/pdf
- [x] **M3.2** - URLFetcher/FileReader/IngestOptions interfaces + SaveURLRequest/SaveFileRequest types in usecase/memory/types.go
- [x] **M3.3** - SaveURL + SaveFile service methods with path traversal + allowed_paths enforcement (ingest.go)
- [x] **M3.4** - Infrastructure: HTTPURLFetcher (readability), MultiFileReader (.md/.txt/.html/.json/etc), PDFExtractor (ledongthuc/pdf + recover())
- [x] **M3.5** - REST IngestHandler (POST /stores/{store}/memories/url + /file) + MCP IngestTools (memory_save_url + memory_save_file)
- [x] **M3.6** - Wire in main.go, all quality gates pass, ARM64 build (15MB)

**Deliverables:**
- [x] `internal/usecase/memory/types.go` — URLFetcher, FileReader, IngestOptions, SaveURLRequest, SaveFileRequest; SourceURL/SourcePath on SaveRequest; SaveURL/SaveFile on Service
- [x] `internal/usecase/memory/ingest.go` — SaveURL + SaveFile implementations
- [x] `internal/usecase/memory/ingest_test.go` — 13 tests (6 SaveURL + 7 SaveFile)
- [x] `internal/infrastructure/ingest/url_fetcher.go` — HTTPURLFetcher with go-readability
- [x] `internal/infrastructure/ingest/url_fetcher_test.go` — 4 tests
- [x] `internal/infrastructure/ingest/file_reader.go` — MultiFileReader (16 ext, HTML tag strip via x/net/html, PDF delegation)
- [x] `internal/infrastructure/ingest/file_reader_test.go` — 7 tests
- [x] `internal/infrastructure/ingest/pdf_extractor.go` — PDFExtractor with recover() wrapping
- [x] `internal/infrastructure/ingest/pdf_extractor_test.go` — 2 tests
- [x] `internal/adapter/rest/ingest_handler.go` — POST /url + /file, error mapping (413, 403, 422)
- [x] `internal/adapter/rest/ingest_handler_test.go` — 5 tests
- [x] `internal/adapter/mcp/ingest_tools.go` — memory_save_url + memory_save_file
- [x] `internal/adapter/mcp/ingest_tools_test.go` — 5 tests
- [x] `cmd/ingatan/main.go` — wired HTTPURLFetcher + MultiFileReader + PDFExtractor + IngestOptions

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... (all pass) ✅ build ✅ --help ✅ ARM64 (15MB)

**Key decisions:**
- Used `ledongthuc/pdf` over `pdfcpu` — better plain-text extraction API; recover() wrapping applied as spec requires
- Path traversal check on raw path BEFORE `filepath.Clean()` (cleaning resolves `..` making the check ineffective)
- IngestOptions defined in usecase layer (not importing config) — clean arch preserved
- SourceURL/SourcePath added to SaveRequest so Save() can set them on Memory directly

---

## Phase 5: Stores & Auth — M4 (Week 9-10)

**Status:** ✅ Complete
**Progress:** 8/8 tasks (100%)

### Tasks

- [x] **M4.1** - `StoreAccess` interface added to memory/types.go; access checks in Save/Get/Update/Delete/List/Search/Similar
- [x] **M4.2** - `internal/usecase/store/types.go` + `service.go` (Create/Get/List/Delete with role enforcement, name regex validation, personal store guard) + 16 tests
- [x] **M4.3** - `internal/usecase/principal/types.go` + `service.go` (GetOrCreate with personal store auto-create, WhoAmI, List admin-only) + 7 tests
- [x] **M4.4** - `internal/infrastructure/storage/store_repository.go` + `store_repository_test.go` (7 tests) + `ListDirs` added to FileStore
- [x] **M4.5** - `internal/infrastructure/storage/principal_repository.go` + `principal_repository_test.go` (7 tests; principals.json array; atomic read-modify-write)
- [x] **M4.6** - `internal/adapter/rest/store_handler.go` + `principal_handler.go` + tests (10 tests)
- [x] **M4.7** - `internal/adapter/mcp/store_tools.go` + `principal_tools.go` + tests (9 tests)
- [x] **M4.8** - Wire `main.go`: `storeAccessImpl`, `PrincipalService.GetOrCreate` replaces JWT stub; all quality gates pass; ARM64 build (15MB)

**Deliverables:**
- [x] Store CRUD (4 tools + 4 REST routes)
- [x] Access control enforcement (owner/writer/reader roles) in memory service
- [x] `principal_whoami`, `principal_list` (MCP + REST)
- [x] Personal store auto-creation on first JWT login
- [x] Store name validation: `^[a-z0-9-]+$`
- [x] `principals.json` file-based persistence

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... (all pass) ✅ build ✅ --help ✅ ARM64 (15MB)

**Key decisions:**
- `StoreAccess` interface defined in `usecase/memory` package (where consumed), implemented by `storeAccessImpl` in `cmd/` — avoids circular imports
- `storeAccessImpl` wraps `storeuc.Repository` directly (not `StoreService`) to keep the check thin
- `principals.json` stores all principals as a JSON array (single-file, read-modify-write); suitable for small-scale edge deployments
- `FileStore.ListDirs` added to enumerate store subdirectories for `StoreRepo.List`
- Admin principals bypass all store membership checks in memory service

---

## Phase 6: Conversations — M5 (Week 11-12)

**Status:** ✅ Complete
**Progress:** 6/6 tasks (100%)

### Tasks

- [x] **M5.1** - `internal/usecase/conversation/types.go` — Service/Repository/LLMProvider/MemorySaver interfaces + all request/response types + AutoSummarizeConfig
- [x] **M5.2** - `internal/infrastructure/storage/conversation_repository.go` + test (9 tests) + `message_repository.go` + test (6 tests)
- [x] **M5.3** - `internal/infrastructure/llm/anthropic.go` + test (6 tests) + `openai.go` + test (6 tests); `github.com/anthropics/anthropic-sdk-go` added
- [x] **M5.4** - `internal/usecase/conversation/service.go` + test (19 tests); auto-summarize on message threshold; `doSummarize` helper
- [x] **M5.5** - `internal/adapter/rest/conversation_handler.go` + test (8 tests) + `internal/adapter/mcp/conversation_tools.go` + test (7 tests)
- [x] **M5.6** - Wire `cmd/ingatan/main.go`: `ConversationRepo`, `MessageRepo`, LLM provider (nil-safe), `memorySaverAdapter`, `ConversationService`, `ConversationHandler`, `ConversationTools`; all quality gates pass; ARM64 build (18MB)

**Deliverables:**
- [x] All 7 conversation MCP tools (`conversation_start`, `conversation_add_message`, `conversation_get`, `conversation_list`, `conversation_summarize`, `conversation_promote`, `conversation_delete`)
- [x] 7 REST routes under `/api/v1/conversations`
- [x] Auto-summarization trigger on `MessageThreshold`
- [x] `conversation_promote` → creates memory with `source=conversation`
- [x] LLM provider adapters: Anthropic (primary) + OpenAI/Ollama-compat
- [x] `memorySaverAdapter` in cmd/ bridges conversation → memory service (clean arch preserved)
- [x] `recover()` guard added to HNSWIndex.Search to handle coder/hnsw library panic on delete

**Quality Gates:** ✅ go fmt ✅ go mod tidy ✅ go vet ✅ golangci-lint (0 issues) ✅ go test ./... (all pass) ✅ build ✅ --help ✅ ARM64 (18MB)

**Key decisions:**
- `MemorySaver` interface defined in conversation usecase, implemented by `memorySaverAdapter` in cmd/ — avoids use case → use case import
- LLM providers nil-safe: Summarize returns `LLM_ERROR` if no provider configured
- Auto-summarize triggered synchronously in `AddMessage` (simpler, avoids goroutine leak)
- Anthropic SDK added (`v1.26.0`); OpenAI LLM provider reuses existing openai-go package
- hnsw panic (library bug on single-node graph after delete) fixed with `recover()` guard; pre-existing issue triggered under parallel test load

---

## ~~Phase 7: WebUI — M6~~ (Dropped)

**Status:** 🚫 Dropped — out of scope for ingatan-app

**Decision:** The WebUI will be a separate application powered by the ingatan REST API.
The embedded UI (templ + HTMX + `go:embed`) is no longer part of this binary.
The REST API (complete as of M5) is the integration surface for the WebUI app.

---

## Phase 7: Hardening — M7 (Week 13-14)

**Status:** ✅ Complete
**Progress:** 6/6 tasks (100%)

### Tasks

- [x] **M7.1** - Add M7 deps + OTel middleware (HTTP spans, OTLP/stdout exporter)
- [x] **M7.2** - Rate limiting middleware (per-IP token bucket, 429 response)
- [x] **M7.3** - mTLS support (client CA, ClientAuth config, CN extraction helper)
- [x] **M7.4** - Backup infrastructure (S3 + git, admin REST endpoint)
- [x] **M7.5** - slog structured logging improvements (request logger, handler logging, no sensitive data)
- [x] **M7.6** - Security review + all quality gates pass + ARM64 build

**Deliverables:**
- [x] `internal/adapter/rest/middleware/otel.go` + test (4 tests; stdout + noop providers)
- [x] `internal/adapter/rest/middleware/rate_limit.go` + test (4 tests; per-IP token bucket)
- [x] `internal/adapter/rest/middleware/mtls.go` + test (9 tests; LoadClientCA, ApplyClientAuth, ClientCertCN)
- [x] `internal/adapter/rest/middleware/slog_logger.go` + test (5 tests; structured JSON per-request)
- [x] `internal/infrastructure/backup/backup.go` — `Backuper` interface
- [x] `internal/infrastructure/backup/s3.go` + test (4 tests; mock HTTP S3 server)
- [x] `internal/infrastructure/backup/git.go` + test (4 tests; temp dir repo)
- [x] `internal/adapter/rest/backup_handler.go` + test (admin-only POST /admin/backup)
- [x] Updated `cmd/ingatan/main.go` — wired OTel, rate limit, mTLS, backup, slog request logger
- [x] `internal/infrastructure/config/config.go` — OTel, rate limit, mTLS, backup config fields (pre-existing)

---

## Phase 8: Release — M8 (Week 15-16)

**Status:** ⬜ Not Started

**Deliverables:**
- [ ] Full integration test suite
- [ ] ARM (aarch64) cross-compile verified
- [ ] Documentation complete
- [ ] PRD sign-off

---

## Blockers & Issues

**Current Blockers:** None

**Known Risks:**
- ⚠️ pdfcpu Alpha stability: wrap with `recover()`, return `PDF_EXTRACTION_ERROR`
- ⚠️ MCP Go SDK unstable until mid-2026: pin to commit hash, `mark3labs/mcp-go` as fallback
- ⚠️ Embedding model change invalidates all vectors: record model metadata, reject on mismatch
- ⚠️ HNSW concurrent write safety: per-store mutex required

---

## Recent Activity

### 2026-03-05 - Phase 7 (M7 Hardening) complete
- [x] M7.1–M7.6 all tasks complete
- [x] New packages: `internal/adapter/rest/middleware/otel.go`, `rate_limit.go`, `mtls.go`, `slog_logger.go`; `internal/infrastructure/backup/backup.go`, `s3.go`, `git.go`; `internal/adapter/rest/backup_handler.go`
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin/arm64, 28MB) + `bin/ingatan-arm64` (linux/arm64, 27MB)
- [x] New deps: OTel v1.41.0, aws-sdk-go-v2 S3, go-git v5.17.0, golang.org/x/time
- [x] Key decisions:
  - OTel: noop provider by default; stdout exporter for dev; OTLP reserved for future
  - Rate limiting: per-IP token bucket via `sync.Map`; config `rate_limit.requests_per_minute`
  - mTLS: `server.tls.client_ca` triggers `RequireAndVerifyClientCert`; `ClientCertCN` helper for agent principal extraction
  - Backup: `Backuper` interface; S3 (path-style, configurable endpoint); git (init/open + AddGlob + commit + optional push)
  - Slog logger: wraps full router; logs method, path, status, duration_ms, principal_id; ERROR on 5xx, WARN on 4xx
  - Security review: no sensitive data in logs; admin check on backup endpoint; path traversal mitigated in prior phases
- [x] Next: Phase 8 M8 — Release (integration tests, documentation, PRD sign-off)

### 2026-03-05 - Phase 6 (M5 Conversations) complete
- [x] M5.1–M5.6 all tasks complete
- [x] 15 storage tests + 12 LLM tests + 19 service tests + 8 REST tests + 7 MCP tests = 61 new tests
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin/arm64, 19MB) + `bin/ingatan-arm64` (linux/arm64, 18MB)
- [x] `github.com/anthropics/anthropic-sdk-go v1.26.0` added to go.mod
- [x] Key decisions: MemorySaver adapter in cmd/ for clean arch; synchronous auto-summarize; nil-safe LLM provider; hnsw library panic fixed with recover()
- [x] Next: Phase 7 M7 — Hardening (OTel, slog, rate limiting, backup, mTLS)
- [x] Phase 7 M6 WebUI dropped — separate WebUI app will consume REST API

### 2026-03-05 - Phase 5 (M4 Stores & Auth) complete
- [x] M4.1–M4.8 all tasks complete
- [x] 16 store service tests + 7 principal service tests + 14 storage tests + 10 REST tests + 9 MCP tests
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin/arm64, 16MB) + `bin/ingatan-arm64` (linux/arm64, 15MB)
- [x] JWT stub replaced with real `PrincipalService.GetOrCreate` (auto-creates principal + personal store on first login)
- [x] Key decisions: StoreAccess interface in usecase/memory; storeAccessImpl in cmd/; principals.json single-file array; admin bypass in memory service
- [x] Path traversal risk now mitigated: store name validated against `^[a-z0-9-]+$` before any file operations
- [x] Next: Phase 6 M5 — Conversations (ConversationService, LLM summarization, promote, 6 MCP tools)

### 2026-03-05 - Phase 4 (M3 Ingest) complete
- [x] M3.1–M3.6 all tasks complete
- [x] 13 service tests + 4 URL fetcher tests + 7 file reader tests + 2 PDF extractor tests + 5 REST tests + 5 MCP tests
- [x] golangci-lint v2: 0 issues (fixed: unchecked defer Close(), unused const)
- [x] Binary: `bin/ingatan` (darwin/arm64, 15MB) + `bin/ingatan-arm64` (linux/arm64, 15MB)
- [x] Key decisions: ledongthuc/pdf over pdfcpu; path traversal check on raw path; IngestOptions in usecase layer; SourceURL/SourcePath in SaveRequest
- [x] Next: Phase 5 M4 — Stores & Auth (StoreRepository, StoreService, access control, PrincipalService)

### 2026-03-05 - Phase 3 (M2 Search) complete
- [x] M2.1–M2.5 all tasks complete
- [x] All tests pass — 27 service tests + 7 BM25 tests + 5 REST handler tests
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin/arm64, 13MB) + `bin/ingatan-arm64` (linux/arm64, 13MB)
- [x] Key decisions: in-house BM25 (no external dep); `VectorSearchResult`/`KeywordSearchResult` in domain layer for clean arch; per-store HNSWStore/BM25Store registries with lazy-load and auto-save; parallel agent teams used for infra + service layer implementation; index operations are best-effort in Save/Delete (secondary to persistence)
- [x] Path traversal risk noted: store name validation not yet enforced; will be addressed in M4 (Stores & Auth)
- [x] Next: Phase 4 M3 — Ingest (URL fetcher, file reader, PDF extractor, `memory_save_url`, `memory_save_file`)

### 2026-03-05 - Phase 2 (M1 Memory Core) complete
- [x] M1.1–M1.9 all tasks complete
- [x] 62 tests across 8 packages — all pass
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin/arm64, 13MB) + `bin/ingatan-arm64` (linux/arm64, 13MB)
- [x] Key decisions: embedder is nil-safe (memories saved without vectors when no provider configured); `RouteRegistrar` interface added to server.go for extensible handler registration; `asAppError` uses `errors.As` for wrapped error compatibility
- [x] Next: Phase 3 M2 — Search (BM25 + HNSW hybrid search via RRF)

### 2026-03-05 - Phase 1 (M0 Foundation) complete
- [x] M0.1–M0.8 all tasks complete
- [x] 26 tests across 5 packages — all pass
- [x] golangci-lint v2: 0 issues
- [x] Binary: `bin/ingatan` (darwin) + `bin/ingatan-arm64` (linux/arm64)
- [x] Key decisions: koanf v2 + confmap for defaults; bufio.Reader wrapping for hnsw.Import; golangci-lint v2 separates formatters from linters
- [x] Next: Phase 2 M1 — Memory Core (domain service + REST + MCP handlers)

### 2026-03-05 - Phase 0 complete
- [x] PRD (ingatan-PRD-v1.0) reviewed and spec.md created
- [x] research.md completed: codebase confirmed empty, all deps pinned, Q1–Q4 resolved, mark3labs/mcp-go chosen over official go-sdk
- [x] data-dictionary.md completed: all interfaces, service types, repository interfaces, config types, 23 MCP tool schemas
- [x] architecture.md completed: 5 sequence diagrams (memory_save, memory_search, startup, JWT auth, auto-summarize)
- [x] plan.md: M0–M8 file-level breakdown confirmed complete
- [x] tasks.md: M0 detailed TDD tasks (M0.1–M0.8) added with Red/Green/Refactor steps

---

## Next Steps

1. **Phase 8: M8 Release** — ready to begin
   - Full integration test suite (`test/integration/`)
   - ARM cross-compile verified ✅ (done in M7)
   - Documentation complete (`documentation/`, `support_docs/`)
   - PRD sign-off

---

**Document Status:** Active
**Next Update:** After completing Phase 7 (M7 Hardening)
