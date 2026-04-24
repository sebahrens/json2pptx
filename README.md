# json2pptx

**Structured presentations from structured data.** Define what your slides *mean* -- json2pptx handles what they *look like*.

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Philosophy

PowerPoint is a visual tool. You drag boxes, pick fonts, nudge alignment. This works for one-off decks, but it falls apart when presentations need to be generated programmatically, updated from data, or produced at scale.

json2pptx takes the same approach that LaTeX brought to documents: **separate content from presentation.** In LaTeX, you declare `\section{Introduction}` and the typesetter handles margins, fonts, and spacing. In json2pptx, you declare `"type": "chart", "chart_value": {...}` and the engine handles layout selection, SVG rendering, and placement into the right template slot.

This is *what you mean is what you get* for presentations:

- You say "bar chart with Q1-Q4 revenue" -- json2pptx picks the layout, renders the chart as SVG, and places it
- You say "three bullet points" -- json2pptx selects a content layout that fits, applies the template's typography, and handles overflow
- You say "SWOT analysis" -- json2pptx generates the diagram with proper quadrant geometry and colors from your theme

The input is a JSON document. The output is a `.pptx` file that opens in PowerPoint, Keynote, or Google Slides -- no post-editing required. Templates control the visual identity; your JSON controls the content and structure.

## How It Works

### The 3 Inputs

```
JSON (content) + Template (.pptx)  -->  json2pptx  -->  Presentation (.pptx)
```

1. **Template** -- A real PowerPoint file with pre-designed slide layouts, colors, and fonts (e.g., `midnight-blue.pptx`). You never edit this directly.
2. **JSON** -- Your content: what goes on each slide, which layout to use, and what type of content (text, bullets, charts, diagrams, tables, shape grids).
3. **The binary** -- `json2pptx generate` reads both, matches content to template placeholders, and writes the final `.pptx`.

### Slides, Layouts, and Placeholders

Each slide has a **layout** (picked by `slide_type`) and **content items** targeting **placeholders**:

```json
{
  "slide_type": "content",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Revenue"},
    {"placeholder_id": "body",  "type": "chart", "chart_value": {"type": "bar", "data": {"Q1": 12, "Q2": 14}}}
  ]
}
```

- **`slide_type`** -- semantic hint that picks the right layout: `title`, `content`, `chart`, `section`, `two-column`, `blank`, `diagram`
- **`placeholder_id`** -- canonical slot name: `title`, `subtitle`, `body`. These are portable across all templates.
- **`type`** -- what kind of content: `text`, `bullets`, `chart`, `diagram`, `table`, `image`, `body_and_bullets`, `bullet_groups`

You don't need to know internal layout IDs. The system resolves them automatically -- the same JSON works with any template.

### Two Rendering Paths

| Path | What | How | Result |
|------|------|-----|--------|
| **Native OOXML shapes** | SWOT, Porter's, BMC, pyramids, process flows, value chains, heatmaps, and more (12 types) | Generated as real PowerPoint shapes | Editable, crisp at any zoom |
| **SVG-rendered charts** | Bar, line, pie, radar, waterfall, gauge, treemap, and more (15 types) | Rendered by `svggen` and embedded as images | High-quality visuals in PowerPoint/Keynote |

### Shape Grids

For custom visual layouts (consulting-style panels, process steps, matrices), `shape_grid` lets you define a grid of shapes directly on a slide. Each cell can hold a shape, table, icon, or image. These render as native OOXML shapes -- fully editable in PowerPoint.

### At a Glance

| Concept | What it does |
|---------|-------------|
| Template | Provides design (colors, fonts, layouts) |
| JSON | Describes content (what goes where) |
| `slide_type` | Picks the right layout automatically |
| `placeholder_id` | Targets a slot (`title`, `body`, `subtitle`) |
| Content `type` | Determines rendering (text, chart, diagram, etc.) |
| Native shapes | Editable PowerPoint objects for business diagrams |
| SVG charts | High-quality rendered images for data visualizations |
| Shape grids | Custom grid layouts with shapes, tables, icons, images |

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
- **Claude Code skills** -- 3 integrated skills for AI-driven deck generation, field reference, and visual QA

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
git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx
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
git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx
make install
```

Binaries are installed to `~/.local/bin/`. Add to your PATH if needed:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

> **WSL2 note:** The generated `.pptx` files are regular files accessible from Windows at `\\wsl$\<distro>\home\<user>\...` or via the output directory you specify.

### Windows (native PowerShell -- no bash required)

Install [Go](https://go.dev/dl/) using the Windows installer, then:

```powershell
git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx
.\install.ps1
```

This builds all binaries, installs them to `%LOCALAPPDATA%\json2pptx\bin\`, copies templates, installs Claude Code skills, and configures the MCP server. Options:

```powershell
.\install.ps1 -Prefix "C:\tools"     # Custom install prefix
.\install.ps1 -SkipSkill             # Skip Claude Code skill
.\install.ps1 -SkipMcp              # Skip MCP config
.\install.ps1 -SkipTemplates        # Skip template files
```

### Windows (WSL2)

Follow the Linux instructions above inside your WSL2 distro. Generated `.pptx` files are accessible from Windows at `\\wsl$\<distro>\home\<user>\...`.

### Windows (Git Bash / MSYS2)

If you prefer Make, use Git Bash or MSYS2 which provide the bash shell the Makefile requires:

```sh
git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx
make install
```

Cross-compile from any platform:

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
  ghcr.io/sebahrens/json2pptx:latest
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

## Claude Code Integration

json2pptx ships with three Claude Code skills and an MCP server that let an AI agent create, validate, and refine presentations from natural language prompts.

### What Gets Installed

`make install` (or `./install.sh` / `.\install.ps1`) sets up everything:

| Component | Location | Purpose |
|-----------|----------|---------|
| MCP server config | `~/.claude/mcp.json` | Connects Claude Code to json2pptx tools |
| **generate-deck** skill | `~/.claude/skills/generate-deck/` | Constrained deck generation workflow |
| **template-deck** skill | `~/.claude/skills/template-deck/` | Complete field reference (layouts, charts, shapes) |
| **slide-visual-qa** skill | `~/.claude/skills/slide-visual-qa/` | Visual QA for rendered slide images |

Skip skill installation with `--skip-skill` (shell) or `-SkipSkill` (PowerShell).

### MCP Server

The MCP server exposes json2pptx as a set of tools that Claude Code can call directly:

| Tool | Description |
|------|-------------|
| `list_templates` | Discover templates, layouts, and placeholder IDs |
| `validate_input` | Check JSON for errors before generating |
| `generate_presentation` | Generate a PPTX from JSON input |

Start manually (for debugging):

```sh
json2pptx mcp --templates-dir ~/.json2pptx/templates --output ./output
```

The installer configures this automatically in `~/.claude/mcp.json`.

### Using the Generate Deck Skill

The `generate-deck` skill (`/generate-deck` in Claude Code) guides the AI through a constrained generation workflow optimized for consulting-quality decks.

**Ask Claude Code to create a deck:**

```
Create a 12-slide strategy deck about our cloud migration plan.
Use the warm-coral template.
```

**The skill enforces a 3-stage workflow:**

1. **Plan** -- Claude produces a slide outline (layout types, visual patterns, narrative arc) and presents it for your approval before writing any JSON.

2. **Generate** -- Claude writes the full JSON in one pass, using proven shape grid patterns (icon rows, card grids, 2x2 matrices, comparison tables) rather than inventing structures from scratch.

3. **Validate & Repair** -- Claude calls `validate_input` to catch structural errors, then fixes only the failing slides rather than regenerating the entire deck.

**Built-in pattern library:** The skill includes 6 battle-tested shape grid patterns extracted from the `sovereign-ai-strategy` reference deck:

| Pattern | Use for |
|---------|---------|
| Icon Rows | Agenda items, key points with icons, risk factors |
| Card Grid | Strategic pillars, capabilities, dimensions |
| Labeled 2x2 Matrix | Tradeoff analysis, strategic positioning |
| Two-Column Header + Body | Pros/cons, before/after comparisons |
| Table in Grid | Data tables, scenario comparisons |
| Card Grid + Chart | Maturity models, assessments with data |

**Invariants are enforced automatically.** The skill encodes rules like "cell col_spans must sum to column count", "chart series values must match categories length", and "use `ctr` not `center` for alignment" -- preventing the most common structural errors.

### Using the Template Deck Skill

The `template-deck` skill (`~/.claude/skills/template-deck/TEMPLATE_GUIDE.md` after installation) is the complete field reference for the JSON format. It documents:

- All content types (text, bullets, charts, diagrams, tables, images, body_and_bullets, bullet_groups)
- All 15 chart types with data format examples
- All 21 diagram types with data schemas
- Complete shape grid properties (bounds, columns, rows, cells, shapes, icons, images)
- Patch operations for incremental slide updates
- Footer, theme override, and slide-level field reference

The generate-deck skill references this automatically when it needs field details.

### Using the Visual QA Skill

After generating a deck, convert slides to images and run visual QA:

```sh
# Convert PPTX to images (requires LibreOffice + ImageMagick)
pptx2jpg -input output/my-deck.pptx -output /tmp/slides/ -density 150

# Then in Claude Code:
/slide-visual-qa /tmp/slides/
```

The skill inspects each slide image for layout issues, text overflow, contrast problems, and spacing defects.

### Example Workflow

A typical AI-assisted workflow:

```
You:     "Build a board presentation about our Q1 results.
          Include revenue charts, team growth, and strategic priorities.
          Use midnight-blue template, 10 slides."

Claude:  [Plans outline, presents for approval]
         [Generates JSON using card grids for KPIs, bar charts for revenue]
         [Validates with validate_input, fixes any errors]
         [Calls generate_presentation → output/q1-board.pptx]

You:     "The revenue chart should be stacked bar, not regular bar.
          Also add a 2x2 matrix for strategic priorities."

Claude:  [Patches only the affected slides, regenerates]
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

### Creating a Custom Template

Any `.pptx` file can serve as a template. Open PowerPoint (or Google Slides, Keynote, LibreOffice Impress), design your slides in the **Slide Master** view, and save as `.pptx`. json2pptx analyzes the layouts at runtime and maps your JSON content to the right placeholders.

#### Required Layouts (4 minimum)

Your template must contain at least these 4 slide layouts for json2pptx to work:

| Layout | Purpose | Placeholders | Maps to `slide_type` |
|--------|---------|--------------|----------------------|
| **Title Slide** | Opening slide | Title + Subtitle | `title` |
| **Content** | Main content slides | Title + Body | `content`, `chart`, `diagram` |
| **Section Divider** | Section breaks | Title (+ optional Body) | `section` |
| **Closing** | Final slide | Title + Subtitle | `blank` (closing tag) |

The layouts are detected by their **placeholder structure**, not by name. A layout with one title and one large body placeholder is classified as "content" regardless of what you call it.

#### Recommended Layouts (6 for full coverage)

For the best experience, also include:

| Layout | Purpose | Placeholders | Maps to `slide_type` |
|--------|---------|--------------|----------------------|
| **Two Content** | Side-by-side content | Title + 2 Body placeholders | `two-column`, `comparison` |
| **Blank** | Empty slide (for shape grids) | None | `blank` |

If these are missing, json2pptx **synthesizes** them automatically from your content layout. A "Blank + Title" layout is also synthesized when needed for shape grid slides.

#### Placeholder Rules

- Each layout should use standard PowerPoint **placeholder types** (Title, Body, Subtitle) -- not plain text boxes
- The **Title** placeholder is what json2pptx targets with `placeholder_id: "title"`
- The **Body/Content** placeholder is targeted with `placeholder_id: "body"`
- The **Subtitle** placeholder is targeted with `placeholder_id: "subtitle"`
- Placeholder size determines character capacity -- json2pptx calculates `max_chars` from the bounding box

#### Theme and Colors

json2pptx extracts your template's **theme colors** (`accent1` through `accent6`, `dk1`, `dk2`, `lt1`, `lt2`) and uses them for:
- Native diagram shapes (SWOT quadrants, Porter's forces, process flow steps)
- Chart color palettes
- Shape grid fills when using scheme references like `"fill": "accent1"`
- Automatic text contrast correction

Define your colors in the slide master's theme. Per-deck overrides are also supported via `theme_override` in JSON.

#### How to Use Your Template

```sh
# Place your .pptx in a directory
mkdir my-templates
cp my-corporate-theme.pptx my-templates/

# Use it (reference by filename without .pptx)
json2pptx generate -json slides.json -template my-corporate-theme -templates-dir my-templates

# Validate it
json2pptx validate-template my-templates/my-corporate-theme.pptx

# Inspect detected layouts
json2pptx skill-info --mode=full --template=my-corporate-theme --templates-dir=my-templates
```

#### Tips

- Design in **16:9 aspect ratio** (standard for modern presentations)
- Use **Slide Master** view in PowerPoint to edit layouts -- don't design on individual slides
- Keep layout names descriptive (e.g., "One Content", "Two Column") -- json2pptx uses names as classification hints
- Test with `json2pptx skill-info --mode=full` to see how your layouts are classified and what tags they receive
- If a layout is misclassified, adjust its placeholder structure (add/remove/resize placeholders)

## CLI Tool (json2pptx)

The `json2pptx` binary is the primary CLI tool. It works as a batch converter, HTTP API server, and MCP server.

### Subcommands

| Command              | Description                                                   |
|----------------------|---------------------------------------------------------------|
| `generate`           | Convert JSON input to PPTX (default if subcommand is omitted) |
| `validate`           | Validate JSON input without generating a file (see [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) for `-fit-report`) |
| `validate-template`  | Check template compatibility                                  |
| `skill-info`         | Show template capabilities for Claude Code skill integration  |
| `patterns`           | List, show, validate, and expand named shape grid patterns    |
| `icons`              | List available icon sets and icons                            |
| `tables`             | Table style guide and density reference                       |
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
| `-strict-fit` | `warn` | Text-fit checking mode: `off`, `warn`, or `strict` (refuse on overflow) |
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
| `GET` | `/api/v1/patterns` | List available named shape grid patterns |
| `GET` | `/api/v1/patterns/{name}` | Show pattern details and schema |
| `POST` | `/api/v1/patterns/{name}/validate` | Validate input against a pattern's schema |
| `POST` | `/api/v1/patterns/{name}/expand` | Expand a pattern into a shape grid |

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
  json2pptx/         CLI tool (generate, validate, serve, mcp, skill-info, patterns, icons, tables)
  pptx2jpg/          PPTX to image conversion (visual inspection)
  debugcolors/       Template color debugging tool
internal/
  api/               HTTP API handlers and routing
  config/            Configuration loading
  generator/         Core PPTX generation pipeline
  jsonschema/        JSON Schema validation
  layout/            Layout selection (heuristic scoring)
  pagination/        Slide pagination and content splitting
  patterns/          Named shape grid pattern registry
  pipeline/          Generation pipeline orchestration
  pptx/              Low-level OOXML manipulation
  resource/          Embedded resource handling
  safeyaml/          Safe YAML parsing
  shapegrid/         Shape grid layout engine
  template/          PPTX template analysis and layout classification
  testrand/          Random test data generation
  testutil/          Test helpers
  textfit/           Text fitting and overflow handling
  types/             Shared data types and input schema
  utils/             Utilities
  visualqa/          Visual QA agent integration
svggen/              SVG chart and diagram generation (separate Go module)
templates/           Built-in PPTX templates
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

Licensed under the Apache License, Version 2.0. You may use, modify, and distribute this software freely, including for commercial purposes, provided you include the [LICENSE](LICENSE) and [NOTICE](NOTICE) files as required by the license. See [LICENSE](LICENSE) for the full terms.

**Commercial licensing** is available for organizations that need different terms (white-label use without attribution, warranty, support/SLA). Contact platon2001@icloud.com.

Third-party license information is documented in [LICENSE-THIRD-PARTY.md](LICENSE-THIRD-PARTY.md).
