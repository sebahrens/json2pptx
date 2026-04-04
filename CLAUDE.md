# json2pptx

Go CLI and library for generating PowerPoint presentations from structured JSON input.

## Quick Reference

```bash
# Build
make                                    # Build all binaries
go build ./cmd/json2pptx                # Build just the main CLI

# Test
go test ./...                           # All tests
go test ./internal/generator/...        # Specific package
cd svggen && go test ./...              # SVG generation (separate go.work module)

# Lint (MUST pass before committing)
golangci-lint run ./...
cd svggen && golangci-lint run ./...

# Generate a deck
json2pptx generate -json examples/basic-deck.json -template midnight-blue -templates-dir templates -output /tmp/out

# Convert to images (needs LibreOffice + ImageMagick)
pptx2jpg -input /tmp/out/basic-deck.pptx -output /tmp/slides/ -density 150

# Validate
json2pptx validate examples/basic-deck.json
json2pptx validate-template templates/midnight-blue.pptx
```

## Before Committing

Always run this checklist before declaring work complete:

1. `golangci-lint run ./...` -- fix all lint errors in main module
2. `cd svggen && golangci-lint run ./...` -- fix all lint errors in svggen module
3. `go test ./...` -- all tests pass in main module
4. `cd svggen && go test ./...` -- all tests pass in svggen module
5. `go build ./cmd/json2pptx` -- binary builds cleanly

## Project Structure

```
cmd/
  json2pptx/      # Main CLI (generate, validate, serve, mcp)
  pptx2jpg/       # PPTX to JPG conversion via LibreOffice
  mktemplate/     # Template creation helper
  debugcolors/    # Theme color debug tool
  templatecaps/   # Template capabilities inspector
  validatepptx/   # PPTX validation tool
  testrand/       # Random deck generator for testing

internal/
  generator/      # Core PPTX generation engine (slide_preparation, text_contrast, shapes)
  pptx/           # Low-level OOXML manipulation (XML types, fills, bullets)
  template/       # Template parsing (themes, fonts, layouts)
  shapegrid/      # Shape grid layout engine
  types/          # Shared data types (Presentation, Slide, Template)
  api/            # HTTP API server
  layout/         # Layout matching and selection
  textfit/        # Text fitting and overflow handling
  pagination/     # Slide pagination / content splitting
  pipeline/       # Generation pipeline orchestration
  visualqa/       # Visual QA agent integration
  resource/       # Embedded resource handling
  config/         # Configuration
  safeyaml/       # Safe YAML parsing
  testrand/       # Random test data generation
  testutil/       # Test helpers
  utils/          # Utilities

svggen/           # SVG chart/diagram generation (separate Go module via go.work)
  charts.go       # Bar, line, pie, etc.
  contrast.go     # WCAG contrast calculations
  style.go        # Theme-aware styling

templates/        # PPTX template files (4: forest-green, midnight-blue, modern-template, warm-coral)
examples/         # Example JSON input files (14 decks)
```

## Key Architectural Decisions

- **Template-driven**: All visual identity comes from `.pptx` template files. The engine never hardcodes colors/fonts.
- **Semantic colors**: JSON uses scheme names (`accent1`, `lt2`, `dk1`) not hex. The engine resolves via the template's theme.
- **Contrast enforcement**: `internal/generator/text_contrast.go` auto-fixes low-contrast text on layout backgrounds (WCAG AA). Shape grid text is warn-only (user-specified colors preserved).
- **SVG for charts/diagrams**: The `svggen/` module renders charts as SVG, embedded as EMF in the PPTX.
- **Shape grids**: Complex layouts (BMC, KPI dashboards, timelines) use `shape_grid` in JSON, rendered by `internal/shapegrid/`.

## Testing Notes

- Golden file tests use `testdata/` directories within packages
- Font metrics differ across platforms (macOS vs Linux CI) -- some tests use `t.Logf` instead of `t.Errorf` for font-dependent assertions
- `svggen/` is a separate module -- run its tests with `cd svggen && go test ./...`
- CI runs on GitHub Actions (`.github/workflows/ci.yml`)

## Templates

4 bundled templates: `forest-green`, `midnight-blue`, `modern-template`, `warm-coral`. Each has its own theme colors, fonts, and slide layouts. Use `json2pptx validate-template` to inspect.

## Common Patterns

- Slide types: `title`, `content`, `section`, `two-column`, `blank`, `chart`, `diagram`
- Placeholder IDs: `title`, `subtitle`, `body`, `body_2` (portable across templates)
- Content types: `text`, `bullets`, `chart`, `diagram`, `table`, `image`, `body_and_bullets`, `bullet_groups`
