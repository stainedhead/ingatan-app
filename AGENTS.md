# AGENTS.md

Rules and guidelines for AI agents working on the ingatan server.

ingatan is a self-hosted memory server exposing MCP tools, a REST API, and an embedded
web UI. It runs as a single Go binary on ARM64 and amd64 edge hardware with no external
runtime dependencies (file-based storage, in-process HNSW + BM25 indexes).

## Architecture: Clean Architecture

This project follows Clean Architecture principles with strict dependency rules.

### Layer Structure

```
internal/
├── domain/          # Entities and business rules (innermost, no dependencies)
├── usecase/         # Application business logic (depends only on domain)
├── adapter/         # Interface adapters: REST handlers, MCP handlers, WebUI (depends on usecase)
│   ├── rest/        # Chi HTTP handlers + middleware
│   ├── mcp/         # MCP tool handlers (mark3labs/mcp-go)
│   └── webui/       # templ + HTMX templates
└── infrastructure/  # External concerns: file storage, HNSW, BM25, embed/LLM providers

cmd/
└── ingatan/         # Application entry point, dependency injection
```

### Dependency Rules

1. **Domain Layer**: Pure business entities and interfaces. No external imports except stdlib.
2. **Use Case Layer**: Orchestrates domain entities. Defines repository/service interfaces.
3. **Adapter Layer**: Implements interfaces defined in use case layer. Converts external data to domain models.
4. **Infrastructure Layer**: Concrete implementations — file storage, HNSW index, BM25 index, embedding/LLM providers.

Dependencies flow inward only. Inner layers define interfaces; outer layers implement them.

---

## Development Methodology: TDD

Follow strict Test-Driven Development with the complete Red-Green-Refactor cycle.

### Red-Green-Refactor Cycle

**IMPORTANT:** All three phases are mandatory. Do not skip the refactor phase.

1. **Red**: Write a failing test that defines expected behavior
   - Test must fail for the right reason
   - Verify the test actually exercises the code path

2. **Green**: Write minimal code to make the test pass
   - Focus on making tests pass, not on perfection
   - Avoid premature optimization
   - Get to green as quickly as possible

3. **Refactor**: Improve code quality while keeping tests green (MANDATORY)
   - **This phase is NOT optional** — refactoring must be done after tests pass
   - Eliminate duplication (DRY principle)
   - Improve naming and readability
   - Extract functions/methods for clarity
   - Simplify complex logic
   - Apply design patterns where appropriate
   - **Run tests after each refactoring step** to ensure they stay green
   - Continue refactoring until code meets quality standards

**The cycle is complete only after refactoring.** Moving to the next feature without refactoring accumulates technical debt.

### Test Organization

```
<package>/
├── <file>.go
└── <file>_test.go    # Tests in same package for white-box testing
```

For black-box testing, use `<package>_test` package name.

### Test Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestFunctionName ./path/to/package

# Run tests with verbose output
go test -v ./...

# Generate coverage report
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

---

## Code Quality Standards

### Build and Run

**All builds output to the `bin/` directory. Always run the executable from there.**

```bash
# Build the executable to bin/
go build -o bin/ingatan ./cmd/ingatan

# Run the executable from bin/
./bin/ingatan --help
./bin/ingatan --config ~/.ingatan/config.yaml

# Build and run in one command
go build -o bin/ingatan ./cmd/ingatan && ./bin/ingatan --help
```

**Rules:**
- Never run `go run ./cmd/ingatan` in production testing — always build first
- The `bin/` directory is gitignored; binaries are never committed
- Use `./bin/ingatan` for all manual testing and verification
- Cross-compile for deployment targets:

```bash
# Primary ARM64 target (Raspberry Pi, Apple Silicon Linux)
GOOS=linux GOARCH=arm64 go build -o bin/ingatan-arm64 ./cmd/ingatan

# Secondary amd64 target
GOOS=linux GOARCH=amd64 go build -o bin/ingatan-amd64 ./cmd/ingatan

# macOS (development)
GOOS=darwin GOARCH=arm64 go build -o bin/ingatan-darwin ./cmd/ingatan
```

### Lint Commands

```bash
# Format code
go fmt ./...

# Vet for suspicious constructs
go vet ./...

# Run linter (golangci-lint v2 — see .golangci.yml)
golangci-lint run

# Tidy dependencies
go mod tidy
```

### Idiomatic Go Practices

- **Naming**: Use MixedCaps, not underscores. Acronyms stay uppercase (HTTPServer, not HttpServer).
- **Errors**: Return errors as the last return value. Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- **Interfaces**: Define interfaces where they are used, not where implemented. Keep interfaces small.
- **Packages**: Package names are lowercase, single words. Avoid `util`, `common`, `helpers`.
- **Documentation**: All exported types and functions have doc comments starting with the name.

---

## Quality Gates

**All quality gates must pass before completing any development task.**

### Pre-Completion Checklist

Run these commands in order. All must succeed with zero errors:

```bash
# 1. Format code (auto-fixes formatting issues)
go fmt ./...

# 2. Tidy dependencies (ensures go.mod/go.sum are clean)
go mod tidy

# 3. Vet for suspicious constructs
go vet ./...

# 4. Run linter (catches bugs, style issues, complexity)
golangci-lint run

# 5. Run all tests
go test ./...

# 6. Build the executable to bin/
go build -o bin/ingatan ./cmd/ingatan

# 7. Verify the executable runs
./bin/ingatan --help
```

### Gate Failure Policy

- **Format/Tidy**: Auto-fix and continue
- **Vet warnings**: Must fix before proceeding
- **Lint errors**: Must fix before proceeding
- **Lint warnings**: Fix if trivial, document if complex (create follow-up task)
- **Test failures**: Must fix before proceeding
- **Build failures**: Must fix before proceeding
- **Run failures**: Must fix before proceeding (executable must run without panic/crash)

### Quick Validation Script

For rapid iteration, use this combined command:

```bash
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/ingatan ./cmd/ingatan && ./bin/ingatan --help
```

### golangci-lint Configuration (v2 format)

**golangci-lint v2 is installed.** The v1 config format is not supported. Use this structure:

```yaml
version: "2"

run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - misspell
    - gocritic
    - bodyclose
    - errorlint

formatters:
  enable:
    - gofmt
    - goimports

settings:
  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

Key v2 differences from v1:
- Requires `version: "2"` at the top
- Formatters (`gofmt`, `goimports`) go under `formatters:`, NOT under `linters:`
- `gosimple` no longer exists (absorbed into `staticcheck`)
- `linters-settings` is now `settings`

---

## Code Design Review

**A code design review is required before any `git commit`.** This is a mandatory step
that happens after all quality gates pass but before code is committed to the repository.

### Design Review Checklist

For every set of changes, verify:

**Architecture & Layers**
- [ ] Dependencies flow inward only — domain has no external deps, usecase has no infrastructure deps
- [ ] Business logic lives in the service layer, not in REST/MCP handlers
- [ ] Repository interfaces are defined in `usecase/`, not `infrastructure/`
- [ ] New types belong in the correct layer

**Code Quality**
- [ ] No TODO/FIXME left without a tracking task
- [ ] Error handling is complete — no swallowed errors, no bare `_` ignoring important errors
- [ ] All exported types and functions have doc comments
- [ ] No unexported dead code (would be caught by lint, but double-check)
- [ ] No hardcoded secrets, credentials, or environment-specific paths

**Security**
- [ ] No path traversal vulnerabilities in file operations
- [ ] Auth middleware applied to all `/api/v1` routes
- [ ] No sensitive data (tokens, secrets, passwords) in logs or error messages
- [ ] User input validated before use in service layer

**Concurrency**
- [ ] HNSW index: writes use `mu.Lock()`, reads use `mu.RLock()`
- [ ] BM25 index: all operations use `mu.Lock()` (in-memory, not concurrent-safe)
- [ ] File writes use atomic write (temp + rename) pattern
- [ ] No shared mutable state without synchronization

**Test Coverage**
- [ ] Happy path tested
- [ ] Error paths tested (not found, forbidden, invalid input)
- [ ] Concurrent access tested where relevant (HNSW, BM25)

### Fix Policy

- **Critical issues** (security, data loss, race conditions, wrong layer dependencies): fix before commit, no exceptions
- **Important issues** (missing error handling, architectural violations, missing tests): fix before commit
- **Minor issues** (style, readability): fix if quick (< 5 min); otherwise create a follow-up task and note in commit message

### Design Review in Agent Workflow

After quality gates pass and before updating status/committing:

1. Run through the design review checklist above
2. Fix all critical and important issues found
3. Re-run quality gates after any fixes
4. Document any minor deferred issues in `specs/ingatan-v1.0/implementation-notes.md`
5. Only then update `status.md` and proceed to commit

---

## Agent Workflow

When making changes:

1. **Understand**: Read relevant documentation and code before modifying
2. **Plan**: Identify affected components across all architectural layers
3. **Test First (Red)**: Write tests before implementation — tests must fail initially
4. **Implement (Green)**: Make minimal changes to pass tests
5. **Refactor (Mandatory)**: Improve code quality while keeping tests green
   - Eliminate duplication
   - Improve naming and structure
   - Extract functions for clarity
   - Simplify complex logic
   - **Run tests after each refactoring change**
6. **Quality Gates**: `go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...`, build, `--help`
7. **Fix Issues**: Resolve any failures; if linter suggests improvements, refactor (return to step 5)
8. **Design Review**: Run through the code design review checklist; fix critical/important issues
9. **Document**: Update affected documentation files
10. **Update Status**: Update `specs/ingatan-v1.0/status.md` — MANDATORY after every task
11. **Final Verify**: Re-run quality gates to confirm all pass after any review fixes

**CRITICAL**: After completing ANY task:
1. Update `status.md` immediately — this is non-negotiable
2. Never mark a task complete until all quality gates AND design review pass
3. The executable must build to `bin/ingatan` and run without errors (`./bin/ingatan --help`)

---

## Documentation Maintenance

### Documentation Structure

**`documentation/` - Internal Product Documentation**
- Audience: developers and AI agents
- Required files: `product-summary.md`, `product-details.md`, `technical-details.md`
- Update when making architecturally significant changes

**`support_docs/` - User-Facing Documentation**
- Audience: end users and operators
- Content: how-to guides, quick start, troubleshooting, configuration reference

**`README.md`** — project overview and quick start for both audiences

### Documentation Standards

- **Concise**: No filler words. Every sentence adds value.
- **Current**: Update docs in the same commit as code changes.
- **Structured**: Consistent headings, lists, and code blocks.

---

## Feature Specification Workflow

### Specs Directory Structure

```
specs/
└── <feature-name>/
    ├── spec.md                  # Feature specification and requirements
    ├── status.md                # CRITICAL: Phase progress tracking (update after each task)
    ├── plan.md                  # Implementation plan and architecture decisions
    ├── tasks.md                 # Task breakdown with TDD steps
    ├── research.md              # Research findings, API docs, examples
    ├── data-dictionary.md       # Data structures, types, schemas
    └── implementation-notes.md  # Decisions made, gotchas, deferred items
```

**MANDATORY**: Update `status.md` after completing each task or phase.

### Specs Workflow Rules

- Create feature directory before starting any new feature work
- Update `status.md` after EVERY task completion — not optional
- Archive to `specs/archive/` when feature is fully implemented and stable
- Specs are gitignored — local planning artifacts only

### Example Feature Development Flow (ingatan)

```bash
# 1. Create feature spec directory
mkdir -p specs/memory-search

# 2. Start with spec.md - define requirements
# 3. Initialize status.md with phases and 0% progress
# 4. Research: explore existing code, validate dependencies
# 5. Create data-dictionary.md, plan.md, tasks.md
# 6. Implement following TDD: Red → Green → Refactor
# 7. Design review before each commit
# 8. Update status.md after each task
# 9. Archive when complete
mv specs/memory-search specs/archive/
```

---

## Agent Teams

### Model Selection

- Default to the current/active model for all teammates
- Do not specify the `model` parameter unless the user has requested a specific model

---

## Git Configuration

### Required .gitignore

```gitignore
# Binaries
bin/
*.exe
*.dll
*.so
*.dylib

# Test artifacts
*.test
coverage.out
coverage.html

# Go workspace
go.work
go.work.sum

# IDE and editor
.idea/
.vscode/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Build artifacts
dist/

# Environment and secrets
.env
.env.local
*.pem
*.key

# Specs (local planning artifacts)
specs/

# Vendor (if not committing)
# vendor/
```

---

## Project Structure Reference

```
.
├── CLAUDE.md                  # References AGENTS.md
├── AGENTS.md                  # This file — agent guidelines
├── README.md                  # Project overview and quick start
├── .gitignore
├── .golangci.yml              # golangci-lint v2 configuration
├── go.mod
├── go.sum
├── config.example.yaml        # Annotated example configuration
├── cmd/
│   └── ingatan/
│       └── main.go            # Entry point + dependency injection
├── internal/
│   ├── domain/                # Business entities (no external deps)
│   │   ├── errors.go
│   │   ├── principal.go
│   │   ├── store.go
│   │   ├── memory.go
│   │   └── conversation.go
│   ├── usecase/               # Service interfaces + business logic
│   │   ├── memory/
│   │   ├── store/
│   │   ├── conversation/
│   │   └── principal/
│   ├── adapter/
│   │   ├── rest/              # Chi HTTP handlers + middleware
│   │   │   └── middleware/    # JWT auth, rate limiting, OTel
│   │   ├── mcp/               # MCP tool handlers (mark3labs/mcp-go)
│   │   └── webui/             # templ + HTMX templates
│   └── infrastructure/
│       ├── config/            # koanf configuration loading
│       ├── storage/           # File-based JSON persistence
│       ├── index/             # HNSW vector index + BM25 keyword index
│       ├── ingest/            # Chunker, embedder, URL fetcher, file reader, PDF extractor
│       ├── embed/             # Embedding provider adapters (OpenAI, Bedrock, Ollama)
│       └── llm/               # LLM provider adapters (Anthropic, OpenAI, Bedrock, Ollama)
├── documentation/             # Internal developer/agent docs
├── support_docs/              # User-facing docs
├── specs/                     # Feature specs (gitignored)
└── bin/                       # Build output (gitignored)
```
