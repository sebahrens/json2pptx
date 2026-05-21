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

## Template Reference

Template structure, placeholder IDs, and layout rules are documented in two canonical docs — read these instead of relying on scattered summaries:

- **Using a template** (placeholder IDs, layout tags, layout→slide-type mapping) → [skills/template-deck/TEMPLATE_GUIDE.md](skills/template-deck/TEMPLATE_GUIDE.md)
- **Authoring / validating a template** (mandatory layouts, theme requirements, conformance checks) → [docs/TEMPLATE_SPEC.md](docs/TEMPLATE_SPEC.md)

## Project Structure

```
cmd/json2pptx/       # CLI tool (generate, validate, serve, mcp, skill-info, patterns, icons, tables)
cmd/pptx2jpg/        # PPTX to image conversion
cmd/debugcolors/     # Template color debugging
internal/            # Private packages
  api/               # HTTP handlers
  config/            # Configuration loading
  generator/         # PPTX file generation
  jsonschema/        # JSON Schema validation
  layout/            # Layout selection (heuristic scoring)
  pagination/        # Slide pagination and content splitting
  patterns/          # Named shape grid pattern registry
  pipeline/          # Generation pipeline
  pptx/              # Low-level OOXML manipulation
  resource/          # Embedded resource handling
  safeyaml/          # Safe YAML parsing
  shapegrid/         # Shape grid engine
  template/          # PPTX template analysis
  textfit/           # Text fitting and overflow handling
  types/             # Shared data types
  utils/             # Utilities
  visualqa/          # Visual QA agent integration
svggen/              # SVG chart and diagram generation (separate module)
templates/           # Built-in PPTX templates
specs/               # Design specifications
```

## Dependencies

- Go 1.25+
- librsvg or resvg (for SVG-to-PNG conversion)
- Optional: Inkscape (for EMF conversion)

## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` for full workflow context; use `bd ready` / `bd show <id>` / `bd update <id> --claim` / `bd close <id>` for everyday work. Use `bd` for ALL task tracking (not TodoWrite or markdown TODO lists) and `bd remember` for persistent knowledge.

The canonical command reference and the mandatory session-close checklist (including the required `git push` at session end) live in [CONTRIBUTING.md](CONTRIBUTING.md#beads-command-and-session-close-reference) — follow it before ending any session.
