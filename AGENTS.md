# AGENTS.md - Go Slide Creator

## Build Commands

```bash
# Build all binaries
make

# Build with race detection (development)
make build-race
```

## Validation Commands

```bash
# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -cover -coverprofile=coverage.out -covermode=atomic

# Check coverage threshold (CI enforces 70% minimum)
go tool cover -func=coverage.out | tail -1

# Type checking (Go is statically typed - build is the check)
go build ./...

# Lint
golangci-lint run ./...

# Format check
gofmt -l .
```

## Run Commands

```bash
# Development mode
json2pptx serve

# With debug logging
LOG_LEVEL=debug json2pptx serve

# With config file
json2pptx serve --config config.yaml
```

## Codebase Patterns

- **Error Handling**: Return `error` as last return value; wrap with `fmt.Errorf("context: %w", err)`
- **Interfaces**: Define in consumer package, not provider
- **Logging**: Use `slog` structured logging with context
- **Testing**: Table-driven tests with `t.Run()` subtests
- **Configuration**: Use environment variables with fallback to config file
- **HTTP Handlers**: Use `http.HandlerFunc` pattern; middleware via composition

## Shape Grid Typography Hierarchy

When generating `shape_grid` JSON, use consistent font sizes:

| Role              | Size   | Weight | Notes                              |
|-------------------|--------|--------|------------------------------------|
| Grid header/banner| 14-18pt| Bold   | White on accent fill, full-width   |
| Card title        | 12-14pt| Bold   | First line, separated by `\n`      |
| Card body         | 9-11pt | Regular| 11pt for 3-4 cols, 10pt for 5+    |
| Step number       | 20-24pt| Bold   | White on accent, narrow column     |
| Footnote/source   | 7-8pt  | Regular| Grey (#666666)                     |

Always set text insets (6-12pt) on body cells. See `docs/INPUT_FORMAT.md` for full examples.

## Project Structure

```
cmd/json2pptx/       # CLI tool (generate, validate, serve, mcp, skill-info)
cmd/pptx2jpg/        # PPTX to image conversion
cmd/debugcolors/     # Template color debugging
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
templates/           # Built-in PPTX templates
specs/               # Design specifications
```

## Dependencies

- Go 1.25+
- librsvg or resvg (for SVG-to-PNG conversion)
- Optional: Inkscape (for EMF conversion)
