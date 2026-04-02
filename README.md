# Go Slide Creator

Convert JSON slide definitions to PowerPoint presentations with intelligent layout selection, built-in chart rendering, and consulting-grade diagram generation.

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Features

- **JSON-to-PPTX conversion** -- define slides as structured JSON, get polished PowerPoint files
- **Template-aware layout selection** -- automatically picks the right layout based on your content
- **15 chart types** -- bar, line, pie, donut, area, radar, scatter, bubble, stacked bar, grouped bar, stacked area, waterfall, funnel, gauge, treemap
- **18 diagram types** -- SWOT, timeline, process flow, org chart, Gantt, Business Model Canvas, Porter's Five Forces, and more
- **Shape grid engine** -- consulting-style custom layouts with preset geometry shapes, row/column grids, and cell spanning
- **Tables** -- data tables with header styling, column alignment, striped rows, and merged cells
- **Inline formatting** -- `<b>bold</b>`, `<i>italic</i>`, `<u>underline</u>` in text and bullets
- **Theme overrides** -- per-deck color and font customization
- **Footer injection** -- configurable left-text footers with slide numbering
- **Speaker notes and source attribution** -- per-slide metadata
- **Slide transitions and build animations** -- fade, push, wipe, cover, cut; bullet-by-bullet reveal
- **HTTP API** -- REST endpoints for programmatic generation
- **MCP server** -- Model Context Protocol support for AI-assisted deck creation
- **Claude Code skill** -- integrated skill for AI-driven presentation authoring

## Installation

### Prerequisites

- **Go 1.25+** -- [download](https://go.dev/dl/)
- **Git** -- for cloning and version info
- **Make** -- build automation (see platform notes below)
- **librsvg** or **resvg** -- for SVG-to-PNG chart/diagram rendering (optional but recommended)

### macOS

```sh
# Install Go (if not already installed)
brew install go

# Install SVG converter (recommended for charts/diagrams)
brew install librsvg

# Clone and install
git clone https://github.com/ahrens/go-slide-creator.git
cd go-slide-creator
make install
```

Binaries are installed to `~/.local/bin/`. Add to your PATH if needed:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

### Linux (including WSL2)

```sh
# Install Go (Ubuntu/Debian)
sudo apt update
sudo apt install -y golang-go

# Or install the latest Go manually
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
export PATH="/usr/local/go/bin:$PATH"

# Install build tools and SVG converter
sudo apt install -y make librsvg2-bin

# Clone and install
git clone https://github.com/ahrens/go-slide-creator.git
cd go-slide-creator
make install
```

Binaries are installed to `~/.local/bin/`. Add to your PATH if needed:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

> **WSL2 note:** The generated `.pptx` files are regular files accessible from Windows at `\\wsl$\<distro>\home\<user>\...` or via the output directory you specify.

### Windows

Windows builds require a bash-compatible shell. Use one of:

- **WSL2** (recommended) -- follow the Linux instructions above inside your WSL2 distro
- **Git Bash / MSYS2** -- provides the bash shell that the Makefile requires

With Git Bash or MSYS2:

```sh
# Ensure Go is installed and in PATH
# https://go.dev/dl/ -- download the Windows installer

# Clone and install
git clone https://github.com/ahrens/go-slide-creator.git
cd go-slide-creator
make install
```

On Windows (non-WSL), binaries install to `%LOCALAPPDATA%\json2pptx\bin\`. Ensure this is in your PATH.

Alternatively, cross-compile from any platform:

```sh
make build-windows-amd64    # Creates bin/json2pptx.exe
```

### Quick Install Script (macOS / Linux / WSL2)

```sh
./install.sh                    # Build + install to ~/.local
./install.sh --prefix /usr/local
./install.sh --skip-skill       # Binary only, no Claude Code skill
```

### Docker (all platforms)

```sh
# Copy and configure environment
cp .env.example .env

# Build and run with Docker Compose
docker-compose up --build
```

The service will be available at `http://localhost:8080`.

Run without Compose:

```sh
docker run -d \
  -p 8080:8080 \
  -v ./templates:/app/templates:ro \
  -v ./output:/app/output:rw \
  ghcr.io/ahrens/go-slide-creator:latest
```

## Quick Start

### CLI Usage

Generate a PPTX from a JSON file:

```sh
json2pptx generate -json slides.json -output ./output
```

Validate without generating (dry-run):

```sh
json2pptx generate -dry-run -json slides.json
```

Read from stdin and write structured output:

```sh
cat slides.json | json2pptx generate -json - -json-output result.json
```

### HTTP API

Start the server:

```sh
json2pptx serve
json2pptx serve --port 3000
```

Convert JSON to PPTX via the API:

```sh
curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "slide_type": "title",
        "content": [
          {"placeholder_id": "title", "type": "text", "text_value": "My Presentation"},
          {"placeholder_id": "subtitle", "type": "text", "text_value": "Welcome"}
        ]
      },
      {
        "slide_type": "content",
        "content": [
          {"placeholder_id": "title", "type": "text", "text_value": "Key Points"},
          {"placeholder_id": "body", "type": "bullets", "bullets_value": ["First point", "Second point"]}
        ]
      }
    ]
  }'
```

### Running the Sovereign AI Example

The repo includes a full example deck at `examples/sovereign-ai-strategy.json` that demonstrates charts, diagrams, shape grids, and multiple slide types. Run it with a built-in template:

```sh
json2pptx generate -json examples/sovereign-ai-strategy.json -output ./output
```

This uses the `warm-coral` template (specified inside the JSON) and writes `sovereign-ai-strategy.pptx` to `./output/`.

To use a different built-in template, override it with `-template`:

```sh
json2pptx generate -json examples/sovereign-ai-strategy.json -template midnight-blue -output ./output
```

To use your own external `.pptx` template, point `-templates-dir` at the directory containing it:

```sh
# Place your template in a directory
cp /path/to/my-corporate-theme.pptx ./my-templates/

# Reference it by filename (without .pptx extension)
json2pptx generate \
  -json examples/sovereign-ai-strategy.json \
  -template my-corporate-theme \
  -templates-dir ./my-templates \
  -output ./output
```

Preview what layouts would be selected without generating (dry-run):

```sh
json2pptx generate -dry-run -json examples/sovereign-ai-strategy.json
```

## JSON Input Format

The system accepts JSON slide definitions. See [docs/INPUT_FORMAT.md](docs/INPUT_FORMAT.md) for the complete reference, and [SLIDE_FORMAT.md](SLIDE_FORMAT.md) for a condensed quick-reference.

### Top-Level Schema

```json
{
  "template": "warm-coral",
  "output_filename": "Q1_Review.pptx",
  "footer": {
    "enabled": true,
    "left_text": "Acme Corp | Confidential"
  },
  "theme_override": {
    "colors": { "accent1": "#E31837" },
    "title_font": "Georgia",
    "body_font": "Arial"
  },
  "slides": [ ... ]
}
```

### Slide Schema

Each slide specifies a `layout_id` (or `slide_type`), content items, and optional metadata:

```json
{
  "layout_id": "One Content",
  "slide_type": "content",
  "content": [
    {
      "placeholder_id": "title",
      "type": "text",
      "text_value": "Revenue Overview"
    },
    {
      "placeholder_id": "body",
      "type": "bullets",
      "bullets_value": [
        "Revenue up <b>25%</b> YoY",
        "Margins improved to 68%",
        "Customer NPS at all-time high"
      ]
    }
  ],
  "speaker_notes": "Emphasize the Q4 recovery.",
  "source": "Company Annual Report, FY2025"
}
```

### Content Types

| Type | Description | Value Field |
|------|-------------|-------------|
| `text` | Plain or formatted text | `text_value` |
| `bullets` | Bullet point list | `bullets_value` |
| `body_and_bullets` | Body text followed by bullets | `body_and_bullets_value` |
| `bullet_groups` | Grouped bullets with headers | `bullet_groups_value` |
| `table` | Data table | `table_value` |
| `chart` | SVG chart (bar, line, pie, etc.) | `chart_value` |
| `diagram` | Business diagram (SWOT, timeline, etc.) | `diagram_value` |
| `image` | Image file | `image_value` |

### Chart Types

`bar`, `line`, `pie`, `donut`, `area`, `radar`, `scatter`, `bubble`, `stacked_bar`, `grouped_bar`, `stacked_area`, `waterfall`, `funnel`, `gauge`, `treemap`

### Diagram Types

`swot`, `timeline`, `process_flow`, `matrix_2x2`, `pyramid`, `venn`, `org_chart`, `gantt`, `kpi_dashboard`, `heatmap`, `fishbone`, `pestel`, `porters_five_forces`, `value_chain`, `business_model_canvas`, `nine_box_talent`, `house_diagram`, `panel_layout`

### Shape Grid

Slides can include a `shape_grid` for custom layouts using preset geometry shapes. The grid supports row/column layouts with gap control, cell spanning, and both shape and table cells.

```json
{
  "slide_type": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Process Steps"}
  ],
  "shape_grid": {
    "columns": 3,
    "gap": 2,
    "rows": [
      {
        "cells": [
          {"shape": {"geometry": "roundRect", "fill": "#4472C4", "text": {"content": "Step 1", "size": 14, "bold": true, "color": "#FFFFFF"}}},
          {"shape": {"geometry": "roundRect", "fill": "#5B9BD5", "text": {"content": "Step 2", "size": 14, "color": "#FFFFFF"}}},
          {"shape": {"geometry": "roundRect", "fill": "#70AD47", "text": {"content": "Step 3", "size": 14, "color": "#FFFFFF"}}}
        ]
      }
    ]
  }
}
```

When `shape_grid` is present with `slide_type: "blank"` (or no `layout_id`), the system uses **virtual layout resolution** to automatically select a blank layout and compute grid bounds from the template's title and footer positions. You can also supply explicit `bounds` as slide percentages. See [docs/INPUT_FORMAT.md](docs/INPUT_FORMAT.md) for the complete shape grid schema.

## Templates

The following templates are included:

| Template | Description |
|----------|-------------|
| `midnight-blue` | Professional dark blue theme |
| `forest-green` | Clean green corporate theme |
| `warm-coral` | Warm coral accent theme |
| `modern-template` | Modern layout template with contemporary styling |

Each template provides six standard layouts: Title Slide, One Content, Two Content, Section Divider, Closing, and Blank. Templates that lack a standard layout (e.g., Two Content) receive one via automatic synthesis at load time.

List available templates:

```sh
json2pptx skill-info --mode=list
```

Get layout details for a template:

```sh
json2pptx skill-info --mode=full --template=midnight-blue
```

## CLI Tool (json2pptx)

The `json2pptx` binary is the primary CLI tool. It works as a batch converter, HTTP API server, and MCP server.

### Subcommands

| Command              | Description                                                   |
|----------------------|---------------------------------------------------------------|
| `generate`           | Convert JSON input to PPTX (default if subcommand is omitted) |
| `validate`           | Validate JSON input without generating a file                 |
| `validate-template`  | Check template compatibility                                  |
| `skill-info`         | Show template capabilities for Claude Code skill integration  |
| `serve`              | Start the HTTP API server                                     |
| `mcp`                | Start MCP (Model Context Protocol) server over stdio          |
| `version`            | Show version, commit, and build information                   |
| `help`               | Show usage help                                               |

### Examples

```sh
# Convert a JSON file
json2pptx generate -json input.json -json-output result.json

# Implicit generate (subcommand can be omitted)
json2pptx -json input.json -json-output result.json

# Validate without generating
json2pptx validate input.json

# Check template
json2pptx validate-template templates/midnight-blue.pptx

# Start HTTP server
json2pptx serve --port 3000

# Start MCP server
json2pptx mcp --templates-dir ./templates --output ./output

# Show template capabilities (JSON)
json2pptx skill-info --templates-dir ./templates --mode full
```

### Generate Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-json` | (required) | Path to JSON input file, or `-` for stdin |
| `-template` | | Template name (without .pptx extension) |
| `-templates-dir` | `./templates` | Directory containing templates |
| `-output` | `./output` | Output directory for generated PPTX files |
| `-json-output` | | Path for JSON result output (headless mode) |
| `-dry-run` / `-n` | `false` | Validate input and show layout selections without generating |
| `-verbose` | `false` | Enable verbose output |
| `-config` | | Path to config file |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check with version info |
| `GET` | `/api/v1/templates` | List available templates |
| `GET` | `/api/v1/templates/{name}` | Get template details (layouts, colors, fonts) |
| `GET` | `/api/v1/slide-types` | List supported slide types |
| `POST` | `/api/v1/convert` | Convert JSON slides to PPTX |
| `GET` | `/api/v1/download/{filename}` | Download generated file (expires after 1 hour) |

See [docs/api/README.md](docs/api/README.md) for complete API documentation with request/response examples.

### Convert Request Fields

| Field | Required | Description |
|-------|----------|-------------|
| template | Yes | Template name (without .pptx) |
| slides | Yes | Array of slide definitions |
| options.output_format | No | `"file"` or `"base64"` (default: `"file"`) |
| options.svg_scale | No | SVG render scale factor, 0.5-10.0 (default: 2.0) |
| options.exclude_template_slides | No | Exclude template's built-in slides from output |

### Convert Response

```json
{
  "success": true,
  "file_url": "/api/v1/download/abc123.pptx",
  "expires_at": "2026-01-17T12:00:00Z",
  "stats": {
    "slide_count": 10,
    "processing_time_ms": 1500,
    "warnings": []
  }
}
```

## Configuration

Environment variables can be set directly or via a `.env` file. See `.env.example` for a complete reference.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| TEMPLATES_DIR | ./templates | Template directory |
| OUTPUT_DIR | ./output | Output directory |
| LOG_LEVEL | info | Logging level (debug, info, warn, error) |
| TEMPLATE_VALIDATION_MODE | soft | Template validation: `strict` or `soft` |
| TEMP_FILE_MAX_AGE | 3600 | Max age for temp files before cleanup (seconds) |
| TEMP_CLEANUP_INTERVAL | 300 | Temp file cleanup interval (seconds) |
| SVG_STRATEGY | png | SVG conversion strategy: `png`, `emf`, or `native` |
| SVG_SCALE | 2.0 | Scale factor for PNG conversion |
| SVG_NATIVE_COMPATIBILITY | warn | Native SVG compatibility: `warn`, `fallback`, `strict`, `ignore` |
| SVG_PNG_CONVERTER | auto | PNG converter: `auto`, `rsvg-convert`, or `resvg` |

### SVG Conversion Strategies

| Strategy | Description | Requirements | Compatibility |
|----------|-------------|--------------|---------------|
| `png` | Convert SVG to PNG (default) | `rsvg-convert` or `resvg` | Universal |
| `emf` | Convert SVG to EMF vector format | Inkscape | PowerPoint 2010+ |
| `native` | Embed SVG directly with PNG fallback | `rsvg-convert` or `resvg` for fallback | PowerPoint 2016+ |

Install `rsvg-convert` for the default PNG strategy:

```sh
brew install librsvg    # macOS
apt install librsvg2-bin  # Ubuntu/Debian
```

### Config File

Create `config.yaml` for more options:

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 60s

templates:
  dir: ./templates
  cache_dir: ./cache/templates

storage:
  output_dir: ./output
  file_retention: 1h

svg:
  strategy: png
  scale: 2.0
  native_compatibility: warn
  preferred_png_converter: auto
```

## Architecture

```
cmd/
  json2pptx/         CLI tool (generate, validate, serve, mcp, skill-info)
  pptx2jpg/          PPTX to image conversion (visual inspection)
  debugcolors/       Template color debugging tool
internal/
  api/               HTTP API handlers and routing
  generator/         Core PPTX generation pipeline
  template/          PPTX template analysis and layout classification
  layout/            Layout selection (heuristic scoring)
  types/             Shared data types and input schema
  parser/            Input parsing
  pipeline/          Generation pipeline orchestration
  config/            Configuration loading
  shapegrid/         Shape grid layout engine
svggen/              SVG chart and diagram generation (separate Go module)
templates/           Built-in PPTX templates (embedded at build time)
```

### Pipeline Flow

```
JSON Input + Template PPTX
        |
        v
  Parse JSON Input --> Slides[], Template, ThemeOverride
        |
        v
  Analyze Template --> Layouts[], Placeholders[], ThemeColors
        |
        v
  Select Layouts   --> Layout assignments per slide
        |
        v
  Generate PPTX    --> Populated PPTX file
    |-- Charts     --> SVG chart rendering
    |-- Diagrams   --> SVG diagram rendering
    |-- Tables     --> Table generation
    |-- Shape Grid --> Custom shape layout
    '-- Images     --> Image embedding
```

## Development

### Prerequisites

- Go 1.25 or later
- golangci-lint (for linting)
- librsvg or resvg (for SVG-to-PNG conversion)

### Building

```sh
make                 # Build all binaries
make build-race      # Build with race detector
make build-cross     # Cross-compile for all platforms
```

### Testing

```sh
make test            # Run all tests
make test-race       # Tests with race detector
make test-cover      # Tests with coverage report
make check           # Build + test + vet
make ci              # Full CI pipeline (fmt + lint + test + vulncheck)
```

### Integration Tests

```sh
./tests/integration/run_pptx_tests.sh    # PPTX generation tests
./tests/integration/run_svg_tests.sh     # SVG diagram tests
```

### Distribution

```sh
make dist-linux      # Linux amd64 tar.gz
make dist-windows    # Windows amd64 tar.gz
make release         # All platforms (requires clean tree)
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style guidelines, testing requirements, and the pull request process.

## License

MIT License. See [LICENSE](LICENSE) for details.

Third-party license information is documented in [LICENSE-THIRD-PARTY.md](LICENSE-THIRD-PARTY.md).
