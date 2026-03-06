# AGENTS.md

Rules and guidelines for AI agents working on the ingatan server — a self-hosted memory server exposing MCP tools, REST API, and embedded web UI. Single Go binary, file-based storage, in-process HNSW + BM25 indexes.

## Architecture: Clean Architecture

Dependencies flow **inward only**. Inner layers define interfaces; outer layers implement them.

```
domain → usecase → adapter → infrastructure
```

- **domain/**: Pure entities and interfaces. No external imports except stdlib.
- **usecase/**: Orchestrates domain. Defines repository/service interfaces.
- **adapter/rest/**: Chi HTTP handlers + middleware. **adapter/mcp/**: mark3labs/mcp-go tools. **adapter/webui/**: templ + HTMX.
- **infrastructure/**: File storage, HNSW, BM25, embed/LLM providers.
- **cmd/ingatan/main.go**: Entry point + dependency injection.

Business logic lives in service layer, not in handlers. Repository interfaces defined in `usecase/`, not `infrastructure/`.

## TDD: Red → Green → Refactor

All three phases are mandatory. **Refactor is not optional.**

1. **Red**: Write a failing test. Verify it fails for the right reason.
2. **Green**: Minimal code to pass. No premature optimization.
3. **Refactor**: Eliminate duplication, improve naming, extract functions, simplify logic. Run tests after each change.

Tests in `<file>_test.go` (same package, white-box). Black-box: `<package>_test` package name.

## Build and Quality Gates

**Build to `bin/`, always run from there. Never use `go run`.**

```bash
go build -o bin/ingatan ./cmd/ingatan
./bin/ingatan --help
```

Cross-compile targets:
```bash
GOOS=linux GOARCH=arm64 go build -o bin/ingatan-arm64 ./cmd/ingatan
GOOS=linux GOARCH=amd64 go build -o bin/ingatan-amd64 ./cmd/ingatan
```

**Quality gate — all must pass before task complete:**
```bash
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/ingatan ./cmd/ingatan && ./bin/ingatan --help
```

Failure policy: format/tidy auto-fix; vet/lint errors/test failures/build failures — must fix before proceeding.

**golangci-lint v2** (`.golangci.yml` — v2 format required):
- `formatters:` section for `gofmt`/`goimports` (NOT under `linters:`)
- No `gosimple` (absorbed into `staticcheck`)
- `settings:` not `linters-settings:`

## Code Design Review (before every `git commit`)

- [ ] Dependencies flow inward only — domain has no external deps, usecase has no infrastructure deps
- [ ] Business logic in service layer, not handlers
- [ ] Repository interfaces in `usecase/`, not `infrastructure/`
- [ ] Error handling complete — no swallowed errors, no bare `_` on important errors
- [ ] All exported types/functions have doc comments
- [ ] No hardcoded secrets, credentials, or env-specific paths
- [ ] No path traversal vulnerabilities in file operations
- [ ] Auth middleware on all `/api/v1` routes; no sensitive data in logs
- [ ] HNSW writes: `mu.Lock()`, reads: `mu.RLock()`. BM25: all ops `mu.Lock()`. File writes: atomic (temp + rename).
- [ ] Happy path + error paths tested

Fix policy: critical/important issues (security, data loss, wrong layer) — fix before commit. Minor (style) — fix if < 5 min, otherwise create follow-up task.

## Agent Workflow

1. **Understand**: Read relevant docs and code first.
2. **Plan**: Identify affected layers.
3. **Red**: Write failing tests.
4. **Green**: Minimal implementation.
5. **Refactor**: Clean code, run tests after each change.
6. **Quality Gates**: Run the combined gate command.
7. **Design Review**: Run checklist, fix critical/important issues.
8. **Document**: Update `documentation/` for architectural changes.
9. **Update Status**: Update `specs/<feature>/status.md` — MANDATORY after every task.
10. **Final Verify**: Re-run quality gates.

**CRITICAL**: `status.md` must be updated immediately after each task. Never mark complete until all gates AND design review pass.

## Documentation

- **`documentation/`**: Internal docs for developers/agents. Required: `product-summary.md`, `product-details.md`, `technical-details.md`. Update on architecturally significant changes.
- **`support_docs/`**: User-facing guides, tutorials, troubleshooting.
- **`README.md`**: Project overview for both audiences.

## Feature Specs Workflow

```
specs/<feature-name>/
  spec.md           # Requirements and acceptance criteria
  status.md         # CRITICAL: phase tracking — update after every task
  plan.md           # Implementation plan
  tasks.md          # TDD task breakdown
  research.md       # Findings and API notes
  data-dictionary.md
  implementation-notes.md
```

- Create spec directory before starting work.
- Update `status.md` after EVERY task — not optional.
- Archive to `specs/archive/` when stable.
- Specs are gitignored (local planning artifacts only).

## Agent Teams

- Do not specify `model` when spawning teammates unless the user requests a specific model.

## Git

- **No AI attribution in commits.** No `Co-Authored-By: Claude ...` or similar trailers.
- `bin/`, `specs/`, test artifacts, IDE files must be in `.gitignore`.

## Project Structure

```
cmd/ingatan/main.go              # Entry point + DI
internal/
  domain/                        # entities: memory, store, principal, conversation, errors
  usecase/memory/ store/ conversation/ principal/
  adapter/rest/ mcp/ webui/
  infrastructure/config/ storage/ index/ ingest/ embed/ llm/ backup/
documentation/  support_docs/  specs/  bin/
```
