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

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
