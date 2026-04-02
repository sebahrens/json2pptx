# Claude Code Skill Integration

Turn go-slide-creator into a self-contained Claude Code skill for AI-driven presentation generation.

## Scope

This specification covers the full integration of `json2pptx` as a Claude Code skill:
- Unified binary with subcommand architecture
- SKILL.md and supporting reference files
- JSON input as primary Claude interface
- Template introspection and validation subcommands
- Template search path resolution
- Cross-platform distribution
- Quality feedback in JSON output

It does NOT cover:
- Changes to core rendering logic (see `04-pptx-generator.md`, `18-svggen-package.md`)
- Visual inspection pipeline (see `11-visual-inspection.md`, `17-visual-inspection-loop.md`)
- JSON input schema extensions (see `docs/INPUT_FORMAT.md`)

## Motivation

The go-slide-creator binary already converts JSON slide definitions to professional PPTX files using real PowerPoint templates. Packaging it as a Claude Code skill enables:

1. **AI-powered deck creation**: Claude generates JSON input, invokes the binary, and delivers polished PPTX files -- all within a single conversation.
2. **Iterative refinement**: Validation and quality feedback let Claude fix content issues before committing to generation.
3. **Template-aware authoring**: Template introspection gives Claude knowledge of available layouts, character limits, and placeholder types, so content fits from the first draft.
4. **Zero-dependency distribution**: A single static binary with embedded templates requires no runtime setup.

## Architecture

### Unified Binary with Subcommands

Restructure the binary from separate `cmd/json2pptx` and `cmd/server` entry points into a single cobra-based command tree:

```
json2pptx
  ├── generate       Convert JSON to PPTX (default subcommand)
  ├── serve           Start HTTP API server
  ├── skill-info      Template discovery and introspection
  ├── validate        Pre-generation JSON validation
  └── validate-template   Template compatibility checking
```

**Current state:**
- `cmd/json2pptx/main.go` -- CLI with flag-based parsing, supports JSON input
- `cmd/server/main.go` -- HTTP API server (separate binary)

**Target state:**
- Single `cmd/json2pptx/main.go` using cobra for subcommand routing
- `generate` becomes the default subcommand (backward compatible)
- `serve` replaces the standalone server binary
- New subcommands: `skill-info`, `validate`, `validate-template`

```
┌─────────────────────────────────────────────────────────────────────┐
│                     json2pptx (unified binary)                        │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ generate │  │  serve   │  │skill-info│  │ validate │          │
│  │          │  │          │  │          │  │          │          │
│  │ md → pptx│  │ HTTP API │  │ template │  │ content  │          │
│  │ json→pptx│  │ :8080    │  │ discovery│  │ checking │          │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
│       │              │              │              │                │
│       └──────────────┴──────────────┴──────────────┘                │
│                              │                                      │
│                    ┌─────────┴─────────┐                           │
│                    │  Shared Services   │                           │
│                    │  - Template cache  │                           │
│                    │  - Config loading  │                           │
│                    │  - Template resolver│                          │
│                    │  - Pipeline        │                           │
│                    └───────────────────┘                            │
└─────────────────────────────────────────────────────────────────────┘
```

### Shared Flags (Root Command)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--templates-dir` | string | (search path) | Override template search path |
| `--verbose` | bool | false | Enable debug logging |
| `--json` | bool | false | Force JSON output to stdout |
| `--version` | bool | - | Print version and exit |
| `--no-color` | bool | auto | Disable colored output |

### Stream Discipline

Following CLI best practices for agent integration:

- **stdout**: Machine-readable result (JSON when `--json` is set, or the primary output)
- **stderr**: Human-readable log messages, progress events, warnings
- **Exit codes**: `0` = success, `1` = invalid input, `2` = missing dependency, `3` = render failure, `4` = template error

## SKILL.md Structure

The SKILL.md file is the entry point for Claude Code skill discovery and invocation.

### YAML Frontmatter

```yaml
---
name: make-slides
description: >
  Convert JSON slide definitions to professional PowerPoint presentations using real PPTX templates.
  Use when the user asks to create slides, presentations, or decks.
  Supports charts, images, two-column layouts, and 20+ diagram types.
allowed-tools:
  - Bash(json2pptx *)
  - Read
argument-hint: "[generate|validate|skill-info] [--input file] [--template name]"
---
```

### Body Structure

The SKILL.md body should be concise (token-efficient) and reference supporting files for detail:

```markdown
# Make Slides

Convert JSON slide definitions to PPTX using json2pptx.

## Quick Start

1. List templates: `json2pptx skill-info --mode=list`
2. Inspect template: `json2pptx skill-info --mode=compact --template=midnight-blue`
3. Validate content: `json2pptx validate --template=midnight-blue --input=deck.json`
4. Generate deck: `json2pptx generate --template=midnight-blue --input=deck.json --output=deck.pptx --partial --json`

## Workflow

1. Use `skill-info` to discover templates and their capabilities
2. Draft JSON input respecting character limits from skill-info
3. Use `validate` to check for issues before generating
4. Use `generate` with `--partial --json` to produce the PPTX
5. If quality score is low, adjust content and regenerate

## Reference

- [JSON input format](docs/INPUT_FORMAT.md) -- content types, chart data, slide schema
- [Template guide](template-guide.md) -- template selection and capabilities
```

### Supporting Reference Files

| File | Purpose | When Loaded |
|------|---------|-------------|
| `docs/INPUT_FORMAT.md` | JSON input format: content types, chart data, slide schema | On demand via Read tool |
| `template-guide.md` | Template capabilities, layout types, best practices | On demand via Read tool |

## JSON Input as Primary Claude Interface

### Current State

The binary already supports JSON input via `-json <file>` flag (see `cmd/json2pptx/main.go`). The existing `JSONInput`, `JSONSlide`, and `JSONContentItem` types provide structured slide specification.

### Enhanced JSON Input

Extend the existing JSON input to support the skill workflow:

```json
{
  "template": "midnight-blue",
  "output_filename": "quarterly-review.pptx",
  "options": {
    "partial": true,
    "svg_strategy": "png"
  },
  "slides": [
    {
      "layout_id": "Title Slide",
      "content": [
        {"placeholder_id": "title", "type": "text", "value": "Q4 Review"},
        {"placeholder_id": "subtitle", "type": "text", "value": "FY2026"}
      ]
    },
    {
      "layout_id": "Content",
      "content": [
        {"placeholder_id": "title", "type": "text", "value": "Key Metrics"},
        {"placeholder_id": "body", "type": "bullets", "value": ["Revenue +15%", "DAU +22%", "Churn -3%"]}
      ]
    }
  ]
}
```

### JSON Output

All subcommands emit structured JSON to stdout when `--json` is set:

```json
{
  "success": true,
  "output_path": "/path/to/quarterly-review.pptx",
  "slide_count": 12,
  "duration_ms": 850,
  "warnings": ["Slide 7: body text may overflow placeholder by ~20 chars"],
  "quality": {
    "overall_score": 88,
    "fit_score": 82,
    "variety_score": 95,
    "suggestions": []
  }
}
```

### JSON Input

The system uses JSON input for structured, machine-precise slide definitions with explicit layout and placeholder control.

## Template Introspection (skill-info)

### Subcommand: `json2pptx skill-info`

Provides template discovery and capability inspection for Claude.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | enum | `compact` | Output detail level: `list`, `compact`, `full` |
| `--template` | string | (all) | Filter to specific template |
| `--templates-dir` | string | (search path) | Override template directory |

### Output Modes

#### List Mode (`--mode=list`)

Minimal token usage. Returns template names only:

```json
{
  "templates": ["midnight-blue", "minimal", "dark-theme", "report"],
  "source": "~/.json2pptx/templates/",
  "count": 4
}
```

#### Compact Mode (`--mode=compact`)

Names with layout counts and supported slide types:

```json
{
  "templates": [
    {
      "name": "midnight-blue",
      "source": "embedded",
      "aspect_ratio": "16:9",
      "layout_count": 11,
      "slide_types": ["title", "content", "two-column", "comparison", "blank", "section"],
      "supports_charts": true,
      "supports_images": true,
      "has_theme_colors": true
    }
  ]
}
```

#### Full Mode (`--mode=full`)

Complete layout details with placeholder information and character limits:

```json
{
  "templates": [
    {
      "name": "midnight-blue",
      "source": "embedded",
      "aspect_ratio": "16:9",
      "theme": {
        "primary": "#1a365d",
        "accent": "#e53e3e",
        "background": "#ffffff"
      },
      "layouts": [
        {
          "id": "slideLayout1",
          "name": "Title Slide",
          "type": "title",
          "placeholders": [
            {
              "id": "title",
              "type": "title",
              "max_chars": 60,
              "position": {"x": 914400, "y": 2286000, "w": 10363200, "h": 1325563}
            },
            {
              "id": "subtitle",
              "type": "subtitle",
              "max_chars": 120,
              "position": {"x": 914400, "y": 3886200, "w": 10363200, "h": 914400}
            }
          ]
        }
      ]
    }
  ]
}
```

### Implementation

The `skill-info` subcommand reuses existing template analysis infrastructure:

```go
// Uses existing packages:
// - internal/template.OpenTemplate()
// - internal/template.ParseLayouts()
// - internal/template.ParseTheme()
// - internal/template.ValidateTemplateMetadata()

func runSkillInfo(mode string, templateFilter string, templatesDir string) error {
    resolver := NewTemplateResolver(templatesDir)
    templates, err := resolver.Discover()
    // ... format output based on mode
}
```

## Validation Subcommands

### `json2pptx validate`

Pre-generation content validation. Checks JSON input against a template without producing PPTX output.

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--input` | string | yes | Path to JSON input (or `-` for stdin) |
| `--template` | string | yes | Template name |

**Output:**

```json
{
  "valid": false,
  "errors": [
    {"slide": 3, "field": "body", "message": "Unrecognized chart type: scatter3d", "severity": "error"},
    {"slide": 5, "field": "title", "message": "Title exceeds 60 char limit (73 chars)", "severity": "warning"}
  ],
  "slide_count": 8,
  "template": "midnight-blue",
  "layout_assignments": [
    {"slide": 1, "layout": "Title Slide", "confidence": 1.0},
    {"slide": 2, "layout": "Content", "confidence": 0.92}
  ]
}
```

### `json2pptx validate-template`

Template compatibility checking. Verifies a PPTX template works with the generator.

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--template` | string | yes | Path to PPTX template file |
| `--strict` | bool | false | Treat warnings as errors |

**Output:**

```json
{
  "valid": true,
  "template": "custom.pptx",
  "aspect_ratio": "16:9",
  "layout_count": 9,
  "supported_types": ["title", "content", "two-column", "blank"],
  "unsupported_types": ["comparison"],
  "warnings": [
    "No section header layout detected",
    "Layout 'Custom Layout 3' has overlapping placeholders"
  ],
  "recommendations": [
    "Add a layout with two body placeholders for two-column support",
    "Consider adding a layout with a chart placeholder for data visualization"
  ]
}
```

## Template Search Path Resolution

Templates are resolved using a prioritized search path:

```
1. --templates-dir flag          (explicit override)
2. $MD2PPTX_TEMPLATES_DIR       (environment variable)
3. ~/.json2pptx/templates/         (user home directory)
4. ./templates/                  (current working directory)
5. Embedded templates            (go:embed fallback)
```

### Implementation

```go
type TemplateResolver struct {
    explicitDir string  // from --templates-dir flag
}

type ResolvedTemplate struct {
    Name   string // e.g., "midnight-blue"
    Path   string // absolute path (or "embedded://midnight-blue.pptx")
    Source string // "flag", "env", "home", "local", "embedded"
}

func (r *TemplateResolver) Discover() ([]ResolvedTemplate, error) {
    // Walk search path in priority order
    // Return all unique templates with source annotation
    // Skip inaccessible directories gracefully
}

func (r *TemplateResolver) Resolve(name string) (*ResolvedTemplate, error) {
    // Find first matching template in search path
    // Return error if not found anywhere
}
```

### Embedded Templates via go:embed

```go
//go:embed templates/*.pptx
var embeddedTemplates embed.FS

// Used as final fallback in template resolution
```

**Binary size considerations:**
- Each template adds ~200-500 KB to the binary
- Ship 2-3 default templates (e.g., `midnight-blue`, `minimal`, `dark`)
- Document binary size impact in build output

## Cross-Platform Distribution

### Build Targets

| Platform | Architecture | Binary Name | Notes |
|----------|-------------|-------------|-------|
| macOS | amd64 | json2pptx-darwin-amd64 | Intel Macs |
| macOS | arm64 | json2pptx-darwin-arm64 | Apple Silicon |
| Linux | amd64 | json2pptx-linux-amd64 | Servers, CI |
| Linux | arm64 | json2pptx-linux-arm64 | ARM servers |

### Build Configuration

```makefile
VERSION := $(shell git describe --tags --always)
COMMIT  := $(shell git rev-parse --short HEAD)
LDFLAGS := -X main.Version=$(VERSION) -X main.CommitSHA=$(COMMIT) -X main.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build-all:
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/json2pptx-darwin-amd64  ./cmd/json2pptx
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/json2pptx-darwin-arm64  ./cmd/json2pptx
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/json2pptx-linux-amd64   ./cmd/json2pptx
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/json2pptx-linux-arm64   ./cmd/json2pptx
```

### CGO Dependency Analysis

The binary currently depends on:
- `tdewolff/canvas` (for SVG generation) -- requires CGO for some backends
- Evaluate static linking feasibility per platform
- Document any platform-specific limitations

### Distribution Options

1. **GitHub Releases**: Attach platform binaries to tagged releases
2. **Homebrew tap**: `brew install ahrens/tap/json2pptx` (macOS)
3. **Direct download**: Documented in SKILL.md installation section

## Quality Feedback in JSON Output

### Fit Score

Per-slide content fit assessment based on placeholder character limits:

```go
type FitScore struct {
    SlideIndex    int     `json:"slide_index"`
    Score         float64 `json:"score"`          // 0-100
    Overflows     []Overflow `json:"overflows,omitempty"`
}

type Overflow struct {
    PlaceholderID string `json:"placeholder_id"`
    MaxChars      int    `json:"max_chars"`
    ActualChars   int    `json:"actual_chars"`
    Excess        int    `json:"excess"`
}
```

### Variety Score

Layout diversity assessment across the deck:

- Consecutive identical layouts penalized
- Ratio of unique layouts to total slides
- Section header usage for long decks

### Overall Quality Score

Weighted combination:
- Fit score: 50% (content fits placeholders)
- Variety score: 30% (layout diversity)
- Completeness: 20% (all required fields populated)

## Migration Path from Current Architecture

### Phase 1: Add Subcommand Infrastructure (P1)

1. Add cobra dependency
2. Create root command with shared flags
3. Wrap existing `run()` as `generate` subcommand
4. Default to `generate` when no subcommand specified (backward compatible)
5. All existing flags continue to work unchanged

### Phase 2: New Subcommands (P1)

1. Implement `skill-info` subcommand (reuse template analysis)
2. Implement `validate` subcommand (reuse parser + validator)
3. Implement `validate-template` subcommand (reuse template validator)
4. Create SKILL.md and reference files

### Phase 3: Unification and Distribution (P2)

1. Move `cmd/server` logic into `serve` subcommand
2. Add `go:embed` for default templates
3. Implement template search path resolver
4. Set up cross-platform build pipeline

### Phase 4: Quality Enhancements (P3)

1. Add quality scoring to generate output
2. Add `--progress` flag for NDJSON progress events
3. Add JSON patch mode for slide-level edits

### Backward Compatibility

- `json2pptx <files>` continues to work (defaults to `generate`)
- All existing flags are preserved on the `generate` subcommand
- The HTTP API is unchanged (just accessed via `json2pptx serve`)
- Existing JSON input format is a subset of the enhanced format

## Testing Strategy

### Unit Tests

| Component | Test File | Coverage |
|-----------|-----------|----------|
| Template resolver | `internal/template/resolver_test.go` | Search path priority, missing dirs, embedded fallback |
| Skill-info output | `cmd/json2pptx/skillinfo_test.go` | All three modes, filtering, JSON schema compliance |
| Validate subcommand | `cmd/json2pptx/validate_test.go` | Parse errors, warnings, character limits |
| Quality scoring | `internal/quality/score_test.go` | Fit, variety, completeness calculations |
| JSON input parsing | `cmd/json2pptx/json_input_test.go` | Schema validation, type coercion, error messages |

### Integration Tests

| Scenario | Validates |
|----------|-----------|
| `skill-info --mode=list` | Template discovery from all search path sources |
| `validate` with known-bad JSON input | Error messages match expected format |
| `generate --json` end-to-end | Full pipeline produces valid PPTX with JSON output |
| `validate-template` with custom template | Compatibility report accuracy |
| Embedded template generation | Binary works without external templates directory |

### Skill Tests (End-to-End)

Test the SKILL.md integration with simulated Claude workflows:

```bash
# Test 1: Discovery workflow
json2pptx skill-info --mode=compact | jq '.templates[0].name'

# Test 2: Validate-then-generate workflow
json2pptx validate --template=midnight-blue --input=test.json
json2pptx generate --template=midnight-blue --input=test.json --output=test.pptx --partial --json

# Test 3: JSON input via stdin
echo '{"template":"midnight-blue","slides":[...]}' | json2pptx generate --json - --output=test.pptx

# Test 4: Error recovery
json2pptx validate --template=midnight-blue --input=bad.json  # should exit 1 with JSON errors
```

### Cross-Platform CI

GitHub Actions matrix build:
- Test on macOS (arm64), Linux (amd64)
- Verify binary starts and runs `skill-info` on each platform
- Verify embedded templates are accessible

## Key Files

```
cmd/json2pptx/
├── main.go              # Cobra root command + subcommand registration
├── generate.go          # generate subcommand (refactored from current main.go)
├── serve.go             # serve subcommand (refactored from cmd/server/)
├── skillinfo.go         # skill-info subcommand
├── validate.go          # validate subcommand
├── validate_template.go # validate-template subcommand
└── *_test.go            # Tests for each subcommand

internal/
├── template/
│   └── resolver.go      # Template search path resolution
├── quality/
│   └── score.go         # Quality scoring (fit, variety, completeness)
└── progress/
    └── reporter.go      # NDJSON progress event emitter

SKILL.md                 # Claude Code skill definition
docs/INPUT_FORMAT.md     # JSON input format reference
template-guide.md        # Template selection guide (supporting file)
```

## Related Specifications

| Spec | Relationship |
|------|-------------|
| [02-template-analyzer](02-template-analyzer.md) | Template analysis reused by `skill-info` and `validate-template` |
| [04-pptx-generator](04-pptx-generator.md) | Generation pipeline invoked by `generate` |
| [06-http-api](06-http-api.md) | HTTP API moved into `serve` subcommand |

## References

- [Claude Code Skills documentation](https://docs.anthropic.com/en/docs/claude-code/skills)
- [CLAUDE-SKILL-HOWTO.md](../CLAUDE-SKILL-HOTWTO.md) -- CLI best practices for skill integration
- [Cobra CLI framework](https://github.com/spf13/cobra)
