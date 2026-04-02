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
cd go-slide-creator

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
