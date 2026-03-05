# AGENTS.md

Rules and guidelines for AI agents working on this Golang CLI application.

## Architecture: Clean Architecture

This project follows Clean Architecture principles with strict dependency rules.

### Layer Structure

```
internal/
├── domain/          # Entities and business rules (innermost, no dependencies)
├── usecase/         # Application business logic (depends only on domain)
├── adapter/         # Interface adapters: CLI handlers, repositories (depends on usecase)
└── infrastructure/  # External concerns: API clients, file I/O (outermost)

cmd/
└── <app>/           # Application entry point, dependency injection
```

### Dependency Rules

1. **Domain Layer**: Pure business entities and interfaces. No external imports except stdlib.
2. **Use Case Layer**: Orchestrates domain entities. Defines repository/service interfaces.
3. **Adapter Layer**: Implements interfaces defined in use case layer. Converts external data to domain models.
4. **Infrastructure Layer**: Concrete implementations for external services, databases, APIs.

Dependencies flow inward only. Inner layers define interfaces; outer layers implement them.

## Development Methodology: TDD

Follow strict Test-Driven Development with the complete Red-Green-Refactor cycle:

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
   - **This phase is NOT optional** - refactoring must be done after tests pass
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

## Code Quality Standards

### Build and Run

**All builds output to the `bin/` directory. Always run the executable from there.**

```bash
# Build the executable to bin/
go build -o bin/nuimanbot ./cmd/nuimanbot

# Run the executable from bin/
./bin/nuimanbot --help
./bin/nuimanbot auth status
./bin/nuimanbot mail list --format json

# Build and run in one command
go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

**Rules:**
- Never run `go run ./cmd/nuimanbot` in production testing—always build first
- The `bin/` directory is gitignored; binaries are never committed
- Use `./bin/nuimanbot` for all manual testing and verification
- Cross-compile for other platforms into `bin/` as well:

```bash
# Cross-compile examples
GOOS=linux GOARCH=amd64 go build -o bin/goog-linux-amd64 ./cmd/goog
GOOS=windows GOARCH=amd64 go build -o bin/goog-windows-amd64.exe ./cmd/goog
GOOS=darwin GOARCH=arm64 go build -o bin/goog-darwin-arm64 ./cmd/goog
```

### Lint Commands

```bash
# Format code
go fmt ./...

# Vet for suspicious constructs
go vet ./...

# Run linter (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
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

### Code Generation Workflow

1. Generate initial code structure to satisfy interfaces
2. Run tests - expect failures
3. Implement until tests pass
4. **Refactor for clarity, performance, and maintainability (MANDATORY)**
   - Eliminate duplication
   - Improve naming and structure
   - Simplify complex code
   - This step is required, not optional
5. Verify tests still pass after refactoring
6. Run linters and fix issues
7. If linter suggests improvements, refactor again (return to step 4)

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
go build -o bin/nuimanbot ./cmd/nuimanbot

# 7. Verify the executable runs
./bin/nuimanbot --help
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
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

### golangci-lint Configuration

If `.golangci.yml` doesn't exist, create with sensible defaults:

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck      # Check error returns
    - gosimple      # Simplify code
    - govet         # Suspicious constructs
    - ineffassign   # Unused assignments
    - staticcheck   # Static analysis
    - unused        # Unused code
    - gofmt         # Formatting
    - goimports     # Import organization
    - misspell      # Spelling errors
    - gocritic      # Code quality

linters-settings:
  errcheck:
    check-blank: true
  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

## Documentation Maintenance

### Documentation Structure

This project maintains two distinct documentation directories with different purposes:

**`documentation/` - Internal Product Documentation**
- **Audience**: Developers and AI agents
- **Purpose**: Technical context, architecture, and development guidance
- **Required minimum files**:
  - `product-summary.md` - Executive overview of the product and its objectives
  - `product-details.md` - Detailed product requirements, workflows, and constraints
  - `technical-details.md` - Architecture, system design, technical decisions, API docs, data flows
- **Optional files**: Additional developer/agent-focused documentation (architecture diagrams, ADRs, etc.)
- **Rules**:
  - Keep focused on technical implementation and architecture
  - Update when making architecturally significant changes
  - Limit to developer/agent context needs
  - Avoid user-facing how-to guides (those go in `support_docs/`)

**`support_docs/` - User-Facing Documentation**
- **Audience**: End users, operators, and administrators
- **Purpose**: Usage guides, tutorials, troubleshooting, and adoption support
- **Content examples**:
  - How-to guides and tutorials
  - User manuals and quick-start guides
  - Troubleshooting and FAQs
  - Installation and setup instructions
  - Best practices and tips
- **Rules**:
  - Write for non-technical users
  - Focus on "how to use" rather than "how it works"
  - Include step-by-step instructions with examples
  - Keep separate from internal technical documentation

**`README.md` (Root)**
- **Audience**: Both developers and users
- **Purpose**: Project overview, quick start, high-level features
- **Content**: Balance technical and user-facing information

### Product Documentation Files

These files in the `documentation/` directory are collectively called **product docs** and must be kept current for architecturally significant changes:

| File | Purpose |
|------|---------|
| `README.md` | Project overview, quick start, usage examples |
| `documentation/product-summary.md` | Executive overview of the product and its objectives |
| `documentation/product-details.md` | Detailed product requirements, workflows, and constraints |
| `documentation/technical-details.md` | Architecture, system design, and technical decisions, API docs, data flows |

**Rules:**
- Treat product docs as a key delivery artifact for changes
- Keep content professional, concise, and up to date
- Agents must use these docs to inform their understanding
- Update in the same commit as architecturally significant code changes

### Documentation Standards

- **Concise**: No filler words. Every sentence adds value.
- **Current**: Update docs in the same commit as code changes.
- **Dual-audience**: Write for both humans and AI agents to understand quickly (for `documentation/`), or for end users (for `support_docs/`).
- **Structured**: Use consistent headings, lists, and code blocks.
- **Separation of Concerns**: Technical implementation details go in `documentation/`, user guides go in `support_docs/`.

## Feature Specification Workflow

### Specs Directory Structure

All feature development uses the `specs/` directory for planning and tracking. Each feature gets its own subdirectory named after the feature.

**Directory Structure:**
```
specs/
└── <feature-name>/
    ├── spec.md                  # Feature specification and requirements
    ├── status.md                # **CRITICAL**: Phase progress tracking (update after each task)
    ├── plan.md                  # Implementation plan and architecture decisions
    ├── tasks.md                 # Task breakdown and progress tracking
    ├── research.md              # Research findings, API docs, examples
    ├── data-dictionary.md       # Data structures, types, schemas
    └── implementation-notes.md  # Implementation details, gotchas, decisions
```

### Progressive Documentation Build

Documents are created progressively as the feature develops:

1. **spec.md** - Start here. Define what the feature does, user requirements, acceptance criteria.
2. **status.md** - **CRITICAL**: Initialize with phases and update after each task completion. Track overall progress.
3. **research.md** - Gather API documentation, explore existing code, collect examples.
4. **data-dictionary.md** - Define domain entities, data structures, types needed.
5. **plan.md** - Design the implementation approach, identify affected layers, list files to modify.
6. **tasks.md** - Break down the work into concrete, testable tasks.
7. **implementation-notes.md** - Record decisions made during implementation, edge cases, lessons learned.

**MANDATORY**: Update `status.md` after completing each task or phase. This file is the single source of truth for progress tracking.

### Specs Workflow Rules

- **Create feature directory** before starting any new feature work
- **Update progressively** as understanding evolves - specs are living documents
- **Update status.md ALWAYS** after completing each task, phase, or milestone - this is MANDATORY
- **Reference from commits** - link to spec directory in commit messages
- **Archive completed** - move to `specs/archive/` when feature is fully implemented and stable
- **Gitignored** - specs are local planning artifacts, not committed to the repository

**Critical Rule**: Every time you complete a task, update `status.md` immediately to reflect:
- Task completion status
- Phase progress percentage
- Any blockers or issues encountered
- Next steps

### Example Feature Development Flow

```bash
# 1. Create feature spec directory
mkdir -p specs/gmail-send-command

# 2. Start with spec.md - define requirements
cat > specs/gmail-send-command/spec.md << 'EOF'
# Gmail Send Command Specification
## Overview
Add `goog mail send` command to send emails via Gmail API...
EOF

# 3. Initialize status.md - set up phase tracking
cat > specs/gmail-send-command/status.md << 'EOF'
# Status: Gmail Send Command
Phase 1: Research - In Progress (0%)
...
EOF

# 4. Research and plan
# Create research.md, data-dictionary.md, plan.md
# UPDATE status.md after completing research phase

# 5. Break into tasks
# Create tasks.md with concrete steps

# 6. Implement following TDD workflow
# Update implementation-notes.md as you go
# **CRITICAL**: Update status.md after EACH task completion

# 7. Archive when complete and stable
# Ensure status.md shows 100% completion before archiving
mv specs/gmail-send-command specs/archive/
```

## Agent Teams

### Model Selection

When spawning agent teams using the TeamCreate and Task tools:

- **Default to the current/active model** for all teammates unless another model is expressly requested
- Teammates inherit the model being used by the spawning agent by default
- Only specify a different model when:
  - The user explicitly requests a specific model (e.g., "use Opus for this team")
  - The task has specific model requirements different from the current context

**Rule**: Do not specify the `model` parameter when spawning teammates via the Task tool unless the user has specifically requested a different model. Let teammates inherit the current model automatically.

## Agent Workflow

When making changes:

1. **Understand**: Read relevant documentation and code before modifying
2. **Plan**: Identify affected components across all architectural layers
3. **Test First (Red)**: Write or update tests before implementation
   - Tests should fail initially
4. **Implement (Green)**: Make minimal changes to pass tests
   - Get tests passing quickly
5. **Refactor (Mandatory)**: Improve code quality while keeping tests green
   - Eliminate duplication
   - Improve naming and structure
   - Extract functions for clarity
   - Simplify complex logic
   - **Run tests after each refactoring change**
   - Continue until code is clean and maintainable
6. **Quality Gates**: Run all quality gate checks (format, tidy, vet, lint, test, build)
7. **Fix Issues**: Resolve any failures from quality gates
   - If linter suggests improvements, refactor (return to step 5)
8. **Document**: Update all affected documentation files
9. **Update Status**: **MANDATORY** - Update `specs/<feature-name>/status.md` with task completion
   - Mark task as complete
   - Update phase progress percentage
   - Note any blockers or issues
   - Update overall completion status
10. **Final Verify**: Re-run quality gates to confirm all pass

**CRITICAL**: After completing ANY task:
1. Update `status.md` immediately - this is non-negotiable
2. Never mark a task complete until all quality gates pass
3. The executable must build successfully to `bin/nuimanbot` and run without errors (`./bin/nuimanbot --help`)

## Git Configuration

### Required .gitignore

Every Go project must have a `.gitignore` file. Create one if it doesn't exist:

```gitignore
# Binaries
bin/
*.exe
*.exe~
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

# Vendor (if not committing)
# vendor/
```

**Rule**: Before first commit, verify `.gitignore` exists and covers build outputs (`bin/`), test artifacts, and IDE files.

## Project Structure Reference

```
.
├── CLAUDE.md              # References this file
├── AGENTS.md              # This file - agent guidelines
├── README.md              # Project overview, quick start, usage examples
├── .gitignore             # Git ignore rules (required)
├── .golangci.yml          # Linter configuration
├── go.mod                 # Go module definition
├── go.sum                 # Dependency checksums
├── cmd/
│   └── nuimanbot/
│       └── main.go        # Entry point
├── internal/
│   ├── domain/            # Business entities
│   ├── usecase/           # Application logic
│   ├── adapter/
│   │   ├── cli/           # CLI command handlers
│   │   └── repository/    # Data access implementations
│   └── infrastructure/    # External service clients
├── pkg/                   # Public libraries (if any)
├── documentation/         # Internal product documentation (developer/agent focused)
│   ├── product-summary.md     # Executive overview
│   ├── product-details.md     # Product requirements, workflows, constraints
│   └── technical-details.md   # Architecture, system design, technical decisions
├── support_docs/          # User-facing documentation (usage, tutorials, how-tos)
├── specs/                 # Feature specs (gitignored)
│   ├── <feature-name>/
│   │   ├── spec.md
│   │   ├── status.md              # **Update after each task completion**
│   │   ├── plan.md
│   │   ├── tasks.md
│   │   ├── research.md
│   │   ├── data-dictionary.md
│   │   └── implementation-notes.md
│   └── archive/           # Completed features
└── bin/                   # Build output (gitignored) - run ./bin/nuimanbot
```
