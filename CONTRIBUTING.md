# Contributing to Go Slide Creator

Thank you for your interest in contributing to Go Slide Creator! This document provides guidelines for contributing to the project.

## Development Setup

### Prerequisites

- Go 1.25 or later
- golangci-lint (for linting)
- librsvg or resvg (for SVG-to-PNG chart conversion)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx

# Install dependencies
go mod download

# Build the project
go build ./...

# Run tests
go test ./... -v
```

### Running the Server

```bash
# Development mode
json2pptx serve

# With debug logging
LOG_LEVEL=debug json2pptx serve
```

### Docker Development

Docker provides a consistent development environment:

```bash
# Build and run with docker-compose
docker-compose up --build

# Run in background
docker-compose up -d --build

# View logs
docker-compose logs -f server

# Stop services
docker-compose down
```

**Development overrides** are automatically applied via `docker-compose.override.yml`:
- Debug logging enabled
- Auth disabled for easier testing
- Test fixtures mounted at `/app/testdata`
- Examples mounted at `/app/examples`

**Running tests in Docker:**

```bash
# Run tests inside the container
docker-compose exec server go test ./... -v

# Or run a one-off container
docker-compose run --rm server go test ./... -v
```

**Rebuilding after changes:**

```bash
# Rebuild and restart
docker-compose up --build

# Force full rebuild (no cache)
docker-compose build --no-cache && docker-compose up
```

## Beads Workflow (Optional)

This repo uses [Beads](https://github.com/steveyegge/beads) (`bd`) for issue
tracking, but **Beads is opt-in local workflow state — it is not required to
build, test, or contribute to json2pptx.** The standard `go build ./...` and
`go test ./...` workflow above needs none of it.

A few things to know so the `.beads/` directory doesn't surprise you:

- **`.beads/` is local-only and gitignored.** It is not committed, so it will
  not be present in a fresh clone, and you should not assume any other
  contributor shares your local issue database. Nothing under `.beads/` is part
  of the source you build against.
- **You only need `bd` if you choose to use it** — for example, when a task
  explicitly asks you to track work in Beads, or you find it convenient for
  managing your own work items locally.
- **AGENTS.md and CLAUDE.md assume `bd` is already set up** because they target
  agents working in a configured environment. If you are a human contributor and
  those `bd prime` / `bd ready` commands aren't available, that's expected — you
  haven't opted in yet.

To opt in, install Beads and initialize it in your checkout:

```bash
# Install (see the Beads docs for other methods)
curl -sSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash

# Initialize local issue tracking in your clone
bd init

# Load workflow context and find work
bd prime
bd ready
```

See [.beads/README.md](.beads/README.md) for background on what Beads is and the
everyday command reference.

## Code Style

### General Guidelines

- Follow standard Go conventions and idioms
- Keep functions focused and concise
- Prefer clarity over cleverness

### Error Handling

Return `error` as the last return value and wrap errors with context:

```go
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### Logging

Use `slog` for structured logging with context:

```go
slog.Info("processing request",
    "request_id", reqID,
    "template", templateName,
)
```

### Interfaces

Define interfaces in the consumer package, not the provider:

```go
// In the package that uses the interface
type TemplateReader interface {
    Read(path string) (*Template, error)
}
```

## Testing Requirements

### All Changes Need Tests

Every code change should include corresponding tests. We maintain a minimum 70% coverage threshold.

### Table-Driven Tests

Use table-driven tests with `t.Run()` subtests:

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "hello",
            want:  "HELLO",
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Foo(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Foo() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Foo() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out
```

## Adding a Template

Every file under `templates/*.pptx` MUST pass `json2pptx template-check` with **zero FAIL and zero WARN** before it ships. The gate is enforced by the corpus test `internal/template/conformance_corpus_test.go`, which runs as part of `go test ./...` (and therefore on every PR in CI).

Before adding or modifying a template, check [docs/TEMPLATE_ANALYSIS.md](docs/TEMPLATE_ANALYSIS.md) to see whether the file is **programmable** (regenerable from `cmd/mktemplate`) or **designer-owned** (must be repaired in place). Do NOT run `mktemplate` against a designer-owned template — it cannot reproduce embedded decorative assets, custom layout shapes, or intentional theme polarities.

### Where the gate runs

| Surface              | Command                                                                | When                                            |
|----------------------|------------------------------------------------------------------------|-------------------------------------------------|
| Local dev            | `make template-check`                                                  | On demand                                       |
| Pre-commit hook      | `scripts/git-hooks/pre-commit` (install via `make install-git-hooks`)  | On every commit that touches a template or the allow-list |
| Make CI pipeline     | `make ci`                                                              | Before pushing                                  |
| GitHub Actions       | `test` job (`go test ./...`) and `corpus-headless`                     | On every PR                                     |

All four use the same `internal/template.CheckConformance` API the `json2pptx template-check` CLI is built on, so results are byte-identical regardless of entry point.

To enable the pre-commit hook for your checkout, run once:

```bash
make install-git-hooks   # sets git config core.hooksPath = scripts/git-hooks
```

The hook only fires when a commit touches `templates/*.pptx` or `internal/template/testdata/conformance_allowlist.json`, so unrelated commits incur no cost.

### Checklist

1. Build the template (see `docs/TEMPLATE_SPEC.md` for the required layouts, placeholders, theme colors, and fonts).
2. Run the conformance check locally:

   ```bash
   json2pptx template-check templates/<your-template>.pptx
   ```

   Iterate until the output is `PASS` with zero `[WARN]` and zero `[FAIL]` lines.
3. Add the file under `templates/` and confirm the corpus gate passes:

   ```bash
   make template-check
   ```

   (equivalent to `go test ./internal/template/ -run TestConformanceCorpus -v`)
4. If you genuinely cannot reach a clean check today (e.g. a legacy designer template whose repair is tracked in a separate issue), add an entry to `internal/template/testdata/conformance_allowlist.json`:

   ```json
   {
     "template": "<filename>.pptx",
     "sha256": "<sha256 from `template-check --json`>",
     "tracking_issue": "<bd issue id>",
     "reason": "<one-sentence explanation of what's broken>"
   }
   ```

   The allow-list is keyed by sha256. As soon as the template is repaired its sha256 changes, the allow-list entry no longer matches, and the conformance gate is automatically re-engaged. The allow-list MUST shrink over time, not grow.

## Pull Request Process

### Branch Naming

Use descriptive branch names:

- `feat/add-chart-animation` - New features
- `fix/text-overflow-bug` - Bug fixes
- `docs/update-api-docs` - Documentation
- `refactor/simplify-parser` - Code refactoring

### Commit Messages

Follow conventional commit format:

```
type: short description

Longer description if needed.

Beads: issue-id
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

### Required Checks

Before submitting a PR, ensure:

1. **Build passes**: `go build ./...`
2. **Tests pass**: `go test ./... -v`
3. **Linter passes**: `golangci-lint run ./...`
4. **Formatting correct**: `gofmt -l .`

### Review Process

1. Create a pull request with a clear description
2. Link any related issues
3. Wait for CI checks to pass
4. Address review feedback
5. Maintainers will merge once approved

## Issue Reporting

### Bug Reports

When reporting bugs, include:

1. **Description**: Clear description of the issue
2. **Steps to reproduce**: Minimal steps to reproduce the bug
3. **Expected behavior**: What should happen
4. **Actual behavior**: What actually happens
5. **Environment**: Go version, OS, etc.

### Feature Requests

For feature requests, describe:

1. **Problem**: The problem you're trying to solve
2. **Proposed solution**: Your suggested approach
3. **Alternatives**: Other solutions you've considered
4. **Use cases**: How this feature would be used

## Project Structure

```
cmd/
  json2pptx/         # CLI tool (generate, serve, validate, mcp, skill-info)
  pptx2jpg/          # PPTX to image conversion
  debugcolors/       # Template color debugging
internal/            # Private packages
  api/               # HTTP handlers
  generator/         # PPTX file generation
  template/          # PPTX template analysis
  layout/            # Layout selection (heuristic scoring)
  types/             # Shared data types
  parser/            # Input parsing
  pipeline/          # Generation pipeline
  config/            # Configuration loading
  shapegrid/         # Shape grid engine
svggen/              # SVG chart and diagram generation (separate module)
specs/               # Design specifications
```

## Questions?

If you have questions about contributing, feel free to open an issue for discussion.
