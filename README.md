# json2pptx

**Structured presentations from structured data.** Define what your slides *mean* -- json2pptx handles what they *look like*.

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Philosophy

PowerPoint is a visual tool. You drag boxes, pick fonts, nudge alignment. This works for one-off decks, but it falls apart when presentations need to be generated programmatically, updated from data, or produced at scale.

json2pptx takes the same approach that LaTeX brought to documents: **separate content from presentation.** In LaTeX, you declare `\section{Introduction}` and the typesetter handles margins, fonts, and spacing. In json2pptx, you declare `"type": "chart", "chart_value": {...}` -- or `"pattern": {"name": "kpi-3up", ...}` -- and the engine handles layout selection, SVG rendering, shape geometry, contrast correction, and placement.

This is *what you mean is what you get* for presentations:

- You say "bar chart with Q1-Q4 revenue" -- json2pptx picks the layout, renders the chart as SVG, and places it
- You say "three KPIs across the top" -- a named pattern (`kpi-3up`) expands into a typed shape grid with the right rhythm
- You say "SWOT analysis" -- the engine generates the diagram with proper quadrant geometry and colors from your theme
- You say "compose a hero stat over a row of icons" -- the `compose` envelope merges two patterns into one slide

The input is a JSON document. The output is a `.pptx` file that opens in PowerPoint, Keynote, or Google Slides -- no post-editing required. Templates control the visual identity; your JSON controls the content and structure. Generation is deterministic: same JSON + template + binary version -> byte-identical PPTX.

## How It Works

### The 3 Inputs

```
JSON (content) + Template (.pptx)  -->  json2pptx  -->  Presentation (.pptx)
```

1. **Template** -- A real PowerPoint file with pre-designed slide layouts, colors, and fonts (e.g., `midnight-blue.pptx`). You never edit this directly.
2. **JSON** -- Your content: what goes on each slide, which layout/pattern to use, and what type of content (text, bullets, charts, diagrams, tables, shape grids, named patterns).
3. **The binary** -- `json2pptx generate` reads both, matches content to template placeholders, expands patterns, runs fit/contrast checks, and writes the final `.pptx`.

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

- **`slide_type`** -- semantic hint that picks the right layout: `title`, `content`, `chart`, `section`, `two-column`, `comparison`, `image`, `blank`, `diagram`
- **`placeholder_id`** -- canonical slot name: `title`, `subtitle`, `body`, `body_2`. Portable across all templates.
- **`type`** -- what kind of content: `text`, `bullets`, `body_and_bullets`, `body_and_lead`, `bullet_groups`, `table`, `chart`, `diagram`, `image`

You don't need to know internal layout IDs. The system resolves them automatically -- the same JSON works with any template.

### Three Authoring Levels

| Level | When to use | What you write |
|-------|-------------|----------------|
| **Placeholders** | Standard slides (title, bullets, single chart) | `content[]` items targeting placeholder IDs |
| **Named patterns** | Recurring layouts (KPIs, comparisons, BMC, timelines) | `pattern: {name, values, style}` -- expands into a typed shape grid |
| **Raw shape grid** | Custom geometry the patterns don't cover | `shape_grid: {columns, rows, cells}` with explicit shapes/tables/icons |

You can also combine patterns on one slide with a `compose` envelope (e.g. a hero stat above a row of icons).

### Two Rendering Paths

| Path | What | How | Result |
|------|------|-----|--------|
| **Native OOXML shapes** | SWOT, Porter's, BMC, pyramids, value chains, panel layouts, KPI dashboards, heatmaps and more | Generated as real PowerPoint shapes | Editable, crisp at any zoom |
| **SVG-rendered visuals** | 15 chart types and the remaining diagram families | Rendered by the `svggen/` module and embedded as PNG (default), EMF, or native SVG | High-quality visuals in PowerPoint/Keynote |

### Diagnostics & Self-Repair

Every generation (and every dry-run) emits structured **fit findings** (`code`, `severity`, `action`, `fix`, JSON Pointer path). An agent can call the `repair_slide` MCP tool with a list of typed fixes (`reduce_text`, `swap_layout`, `split_at_row`, `swap_pattern`, `reshape_grid`, `set_pattern_style`, `replace_color`, `use_semantic_color`, ...) to patch a single slide rather than regenerating the whole deck. See [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) and [docs/REPAIR_LOOP.md](docs/REPAIR_LOOP.md).

### Output Validation Guarantee

`generate_presentation` (MCP) and `json2pptx generate` (CLI) default to **strict** output validation: every successful response implies the generated `.pptx` passed the full OPC + OOXML validator (`internal/pptx.ValidateOutputFile`) with zero blocking findings. This is the "zero needs repair" contract — PowerPoint and Keynote will not show the *"we found a problem with some content"* prompt on a strict-mode success. When validation fails, the response is an error envelope `{summary, findings[], next_tool_call: {tool: "repair_slide", args_template}}` carrying `OPC_*` / `OOXML_*` findings with per-finding `scope` (`source` / `template` / `generator`). See [skills/generate-deck/SKILL.md#output-validation-guarantee](skills/generate-deck/SKILL.md#output-validation-guarantee) for the envelope shape, full code catalog, and response protocol.

## Features

- **JSON-to-PPTX conversion** -- structured slide definitions become polished PowerPoint files
- **4 bundled templates** -- `forest-green`, `midnight-blue`, `modern-template`, `warm-coral` (any `.pptx` works as a template)
- **Template-aware layout selection** -- picks the right layout based on your content; synthesizes missing standard layouts
- **15 chart types** -- bar, grouped_bar, stacked_bar, line, area, stacked_area, pie, donut, scatter, bubble, radar, waterfall, funnel, gauge, treemap
- **21 diagram types** -- SWOT, timeline, process flow, pyramid, venn, org chart, Gantt, KPI dashboard, heatmap, fishbone, PESTEL, Porter's Five Forces, value chain, Business Model Canvas, nine box talent, house diagram, panel layout, icon columns/rows, stat cards, matrix 2x2
- **20 named patterns** -- agenda, arch-stack, before-after, bmc-canvas, card-grid, comparison-2col, icon-row, kpi-2up...kpi-6up (parametric), matrix-2x2, process-flow, pull-quote, pyramid, roadmap-phased, stat-hero, swimlane, timeline-horizontal
- **Pattern composition** (`compose`) -- 2-4 patterns merged into one slide with vertical/horizontal layout
- **Shape grid engine** -- consulting-style custom layouts with preset geometries, row/column grids, cell spanning, authoritative bounds
- **Tables with auto-pagination** (`split_slide`) -- header rows repeat across continuation slides
- **Inline formatting** -- `<b>bold</b>`, `<i>italic</i>`, `<u>underline</u>` in text and bullets
- **Constrained vs free authoring** (`design_mode`) -- constrained refuses raw hex and absolute font sizes to keep decks on-brand; `free` unlocks them
- **Accent rotation strategies** (`accent_strategy`) -- `primary`, `rotate`, or `section-keyed`
- **Style defaults** -- deck-level `defaults.table_style` / `defaults.cell_style`, swap-only semantics
- **Deck structure** -- `structure` block with sections, auto-agenda, cover/closing slides
- **Persistent chrome** -- footer, header, page numbers, with skip rules
- **Theme overrides** -- per-deck color and font customization
- **Speaker notes, sources, and alt-text** -- per-slide metadata, accessibility-aware
- **Slide transitions and build animations** -- fade, push, wipe, cover, cut; bullet-by-bullet reveal
- **Contrast enforcement** -- WCAG AA auto-fix on layout backgrounds; warn-only on user-specified shape colors
- **Text fit checking** (`strict_fit`: `off|warn|strict`) -- structured findings; refuse on overflow when strict
- **Fit findings + repair loop** -- typed diagnostics with JSON Pointer paths; `repair_slide` patches one slide at a time
- **Deterministic visual scoring** -- `score_deck` runs full generation and grades the output 0-100 with a composition axis
- **Deck rhythm analysis** -- `analyze_deck_rhythm` flags pattern repetition, accent imbalance, density variance
- **Deck planning** -- `plan_deck` turns a brief into a structured slide outline with rhythm rules
- **PPTX read-back** -- `read_presentation` extracts placeholders/shapes/tables from an existing PPTX without LibreOffice
- **HTTP API** -- REST endpoints for programmatic generation
- **MCP server** -- 45 Model Context Protocol tools for AI-assisted deck creation, validation, planning, recommendation, scoring, and repair, including the one-call `make_deck` cold-start facade
- **Claude Code skills** -- 3 integrated skills for AI-driven deck generation, template setup, and visual QA

## Installation

### Prerequisites

- **Go 1.25+** -- [download](https://go.dev/dl/)
- **Git** -- for cloning and version info
- **Make** -- build automation (see platform notes below)
- **librsvg** or **resvg** -- for SVG-to-PNG chart/diagram rendering (optional but recommended)
- **LibreOffice + ImageMagick** -- only required for `pptx2jpg`, `render_slide_image`, and `render_deck_thumbnails`

### macOS

```sh
brew install go
brew install librsvg            # recommended for charts/diagrams
brew install --cask libreoffice # only needed for slide-image rendering
brew install imagemagick

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
sudo apt update
sudo apt install -y golang-go make librsvg2-bin
# Optional: sudo apt install -y libreoffice imagemagick

git clone https://github.com/sebahrens/json2pptx.git
cd json2pptx
make install
```

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

> **WSL2 note:** Generated `.pptx` files are accessible from Windows at `\\wsl$\<distro>\home\<user>\...` or via the output directory you specify.

### Windows (native PowerShell)

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
.\install.ps1 -SkipMcp               # Skip MCP config
.\install.ps1 -SkipTemplates         # Skip template files
```

### Quick Install Script (macOS / Linux / WSL2)

```sh
./install.sh                    # Build + install to ~/.local
./install.sh --prefix /usr/local
./install.sh --skip-skill       # Binary only, no Claude Code skills
```

### Docker (all platforms)

```sh
cp .env.example .env
docker-compose up --build
```

The HTTP API is available at `http://localhost:8080`.

```sh
docker run -d \
  -p 8080:8080 \
  -v ./templates:/app/templates:ro \
  -v ./output:/app/output:rw \
  ghcr.io/sebahrens/json2pptx:latest
```

## Quick Start

### CLI Usage

```sh
# Generate a deck
json2pptx generate -json examples/varied-pitch-deck.json -output ./output

# Validate without generating
json2pptx validate examples/varied-pitch-deck.json

# Dry-run: show layout selections + fit findings without writing a file
json2pptx generate -dry-run -json examples/varied-pitch-deck.json

# Read JSON from stdin, write structured result to file
cat slides.json | json2pptx generate -json - -json-output result.json

# Override the template specified inside the JSON
json2pptx generate -json examples/varied-pitch-deck.json -template forest-green -output ./output

# Override the deck's design_mode for one run (allow raw hex colors / absolute sizes)
json2pptx generate --design-mode=free -json examples/business-model-canvas.json -output ./output

# Use your own external template
mkdir my-templates && cp /path/to/my-corporate-theme.pptx my-templates/
json2pptx generate \
  -json examples/varied-pitch-deck.json \
  -template my-corporate-theme \
  -templates-dir ./my-templates \
  -output ./output
```

### Example Decks

The repo ships with 19 example decks (and a `diagrams/` subdirectory with one JSON per diagram type). The most useful ones to learn from, in order:

| Example | What it shows |
|---------|---------------|
| `examples/basic-deck.json` | Hello-world: title, content, bullets, plain placeholders |
| `examples/charts.json` | Cleanest reference for `chart_value` syntax across chart types |
| `examples/shape-grid-panels.json` | Easiest entry point to raw `shape_grid` |
| `examples/patterns-smoke.json` | Compact tour of named patterns, callouts, and `cell_overrides` |
| `examples/varied-pitch-deck.json` | Realistic 13-slide pitch -- 10 patterns plus a `compose` slide combining `stat-hero` and `kpi-3up` |
| `examples/sovereign-ai-strategy.json` | Flagship 25-slide consulting deck (charts + grids + sources + notes) |
| `examples/business-model-canvas.json` | Raw `shape_grid` BMC with merged rows/columns |
| `examples/consulting-layouts.json` | Chevron flows, 2x2 matrices, takeaways, side-by-side |
| `examples/split-slide-vendor-matrix.json` | `split_slide` table pagination with repeated headers |
| `examples/visual-maturity-stress-test.json` | Comprehensive grid regression coverage |
| `examples/contrast-fixer-test.json` | Contrast autofix on dark/accent fills |
| `examples/full-showcase.json` | Mixed traditional slides + several chart types |

### HTTP API

```sh
json2pptx serve                  # default port 8080
json2pptx serve --port 3000

curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {"slide_type": "title",
       "content": [
         {"placeholder_id": "title",    "type": "text", "text_value": "My Presentation"},
         {"placeholder_id": "subtitle", "type": "text", "text_value": "Welcome"}
       ]},
      {"slide_type": "content",
       "content": [
         {"placeholder_id": "title", "type": "text",    "text_value": "Key Points"},
         {"placeholder_id": "body",  "type": "bullets", "bullets_value": ["First point", "Second point"]}
       ]}
    ]
  }'
```

## Claude Code Integration

json2pptx ships with three Claude Code skills and an MCP server that let an AI agent plan, create, validate, score, and repair presentations from natural language prompts.

### What Gets Installed

`make install` (or `./install.sh` / `.\install.ps1`) sets up everything:

| Component | Location | Purpose |
|-----------|----------|---------|
| MCP server config | `~/.claude/mcp.json` | Connects Claude Code to json2pptx tools |
| **generate-deck** skill | `~/.claude/skills/generate-deck/` | 4-phase deck workflow (Plan -> Vary -> Render -> Repair) |
| **template-deck** skill | `~/.claude/skills/template-deck/` | Template setup and conformance reference |
| **slide-visual-qa** skill | `~/.claude/skills/slide-visual-qa/` | Composition review + per-slide screenshot inspection |

Skip skill installation with `--skip-skill` (shell) or `-SkipSkill` (PowerShell).

### MCP Server (45 Tools)

Start manually for debugging:

```sh
json2pptx mcp --templates-dir ~/.json2pptx/templates --output ./output
```

The installer configures this automatically in `~/.claude/mcp.json`. The fastest
way to learn the workflow at runtime is the `get_started` tool: it returns the
recommended single-call **fast path** plus the ordered manual **sequence** it
composes, scoped to `brief` / `revise` / `validate-only`. Each tool carries
classification metadata in `get_capabilities` (`kind`, `phase`, `cli_counterpart`,
`mcp_only_reason`, `primitive_alternatives`) so agents can pick the right tool
without prose-parsing descriptions. The tables below are grouped by the same
`phase` taxonomy `get_capabilities` reports.

Most MCP tools have a 1:1 CLI counterpart so you can drive the same workflows
from a shell; the **MCP-only** tools below have no direct subcommand (the CLI
workaround for each is printed by `json2pptx help`).

**Cold-start fast path** — the workflow facades (`kind: workflow_facade`) that
collapse a whole chain into one call:

| Tool | Purpose | CLI |
|------|---------|-----|
| `make_deck` | ONE call from a natural-language outline to a validated, auto-repaired PPTX (chains `plan_deck` → expand patterns → `auto_repair`). Recommended cold-start entry point. | MCP-only |
| `auto_repair` | Server-side convergence loop (`generate` → inspect → `repair`) against a configurable quality gate. | MCP-only |

**Discovery / introspection** (`phase: discovery`, read-only)

| Tool | Purpose | CLI |
|------|---------|-----|
| `get_started` | Recommended fast path + ordered manual sequence for a task | `get-started` |
| `get_capabilities` | Schema version, tool inventory + classification, deprecations, feature flags, vocabularies | `capabilities` |
| `get_input_schema` | Authoritative JSON Schema for PresentationInput with digest-based caching | `input-schema` |
| `get_data_format_hints` | Full chart/diagram data-shape hints (with digest for caching) | `data-format-hints` |
| `list_templates` | Discover bundled/external templates, layouts, palette, canonical taxonomy | `skill-info` |
| `get_chart_capabilities` | Per-chart limits and label strategy | `capabilities` |
| `get_diagram_capabilities` | Per-diagram limits and field reference | `capabilities` |
| `get_shape_catalog` | Preset geometries grouped by use case | `shape-catalog` |
| `resolve_theme` | Resolve theme colors and fonts for a template | `resolve-theme` |
| `list_icons` | Bundled icon names by set | `icons` |
| `preview_icon` | Render a single icon spec to SVG + PNG | `preview-icon` |
| `list_patterns` | Catalog of named patterns grouped by category | `patterns list` |
| `show_pattern` | Pattern contract: `use_when`, `not_when`, schema, version | `patterns show` |
| `table_density_guide` | Table density tiers, hard limits, multiline guidance | `tables` |
| `list_template_settings` | List template-side named table/cell styles | `template-settings list` |
| `examine_template` | Deep read-only template capability report (canonical roles, bounds, findings) | `examine-template` |
| `describe_finding` | Resolve a finding code to summary, severity, remediation steps | `describe-finding` |

**Plan** (`phase: plan`)

| Tool | Purpose | CLI |
|------|---------|-----|
| `plan_deck` | Turn a brief into an ordered slide outline with rhythm rules (template-aware) | `plan-deck` |
| `recommend_visual` | Unified router across placeholder layouts, patterns, charts, diagrams, raw grids | `recommend-visual` |
| `recommend_pattern` | Rank named patterns for a slide intent (legacy subset of `recommend_visual`) | `recommend-pattern` |
| `validate_pattern` | Validate pattern values without expanding | `patterns validate` |
| `expand_pattern` | Expand a pattern into a full `shape_grid` | `patterns expand` |
| `expand_patterns` | Batch-expand N patterns under one template load | MCP-only |

**Vary** (`phase: vary`)

| Tool | Purpose | CLI |
|------|---------|-----|
| `analyze_deck_rhythm` | Pattern repetition, accent balance, density variance, composition score | `analyze-rhythm` |
| `score_candidates` | Rank candidate slide JSONs for one slot without rendering | `score-candidates` |

**Render** (`phase: render`)

| Tool | Purpose | CLI |
|------|---------|-----|
| `validate_input` | Schema + static checks (+ optional `fit_report`), no render | `validate` |
| `preview_presentation_plan` | Resolve layouts/placeholders/findings without rendering | `preview` |
| `preview_slide_wireframe` | Annotated per-slide wireframe (SVG + PNG) without LibreOffice | `preview-wireframe` |
| `validate_presentation_output` | Validate a generated PPTX for structural + OOXML correctness | `validate-output` |
| `generate_presentation` | Render a PPTX from JSON; optional `fit_report`, `strict_fit`, `strict_unknown_keys` | `generate` |
| `make_deck` | Cold-start facade (see fast path above) | MCP-only |

**Repair** (`phase: repair`)

| Tool | Purpose | CLI |
|------|---------|-----|
| `repair_slide` | Apply targeted fixes to one slide without regenerating the deck | `repair` |
| `repair_slides_batch` | Apply fixes to multiple slides in one call | MCP-only |
| `propose_repairs` | Translate fit/visual-QA findings into ranked `repair_slide` directives | MCP-only |
| `apply_deck_patch` | Pure deck-JSON transform: bounded structural ops (insert/remove/move/replace) | MCP-only |
| `auto_repair` | Server-side convergence loop (see fast path above) | MCP-only |
| `inspect_slide_images` | Vision/heuristic visual QA of rendered slides with pre-mapped fixes | `inspect` |
| `render_slide_image` | Render one PPTX slide to PNG (requires LibreOffice + ImageMagick) | `render-slide` |
| `render_slide_image_from_json` | Render one slide directly from its JSON (requires LibreOffice + ImageMagick) | `render-slide-from-json` |
| `render_deck_thumbnails` | Render all slides to low-res thumbnails (requires LibreOffice + ImageMagick) | `render-thumbnails` |
| `read_presentation` | Extract placeholders/shapes/tables/notes from an existing PPTX (no LibreOffice) | `read` |
| `audit_palette` | Render to PNG and report ΔE between chart pics and adjacent solid-filled shapes | `audit-palette` |
| `score_deck` | Deterministic 0-100 score with composition axis and a quality gate | `score` |

**Settings** (`phase: settings`, gated by `JSON2PPTX_ALLOW_SETTINGS_WRITE=1`)

| Tool | Purpose | CLI |
|------|---------|-----|
| `register_template_setting` | Persist a named table/cell style | `template-settings register` |
| `delete_template_setting` | Delete a named style | `template-settings delete` |

### Example Workflow

**Fast path (one call).** When the agent does not need to hand-author per-slide
content, `make_deck` collapses the entire chain into a single tool call:

```
You:     "Build a board presentation about our Q1 results.
          Include revenue charts, team growth, and strategic priorities.
          Use midnight-blue template, 10 slides."

Claude:  [calls make_deck -> plans, fills patterns, generates, and auto-repairs
          to a quality gate -> output/make_deck.pptx]
```

**Controllable path (manual primitives).** Drive each step yourself when you want
control over copy, patterns, and layout — this is the `sequence` `get_started`
returns for `task=brief`:

```
Claude:  [calls plan_deck -> structured slide outline]
         [presents outline for approval]
         [generates JSON using card grids for KPIs, bar charts for revenue]
         [calls validate_input -> reads back fit findings]
         [calls score_deck and analyze_deck_rhythm -> spots monotony]
         [calls repair_slide on the two flagged slides]
         [calls generate_presentation -> output/q1-board.pptx]

You:     "Stacked bar for revenue. Add a 2x2 matrix for priorities."

Claude:  [calls repair_slide twice -> patches only the affected slides]
         [regenerates]
```

## JSON Input Format

The authoritative input schema is generated from the Go input structs and published via `get_input_schema` (MCP) or `json2pptx input-schema` (CLI). See [docs/INPUT_FORMAT.md](docs/INPUT_FORMAT.md) and [SLIDE_FORMAT.md](SLIDE_FORMAT.md) for worked-example tutorials, and [docs/PATTERNS.md](docs/PATTERNS.md) for the named-pattern authoring guide.

### Top-Level Schema

```json
{
  "template": "warm-coral",
  "output_filename": "Q1_Review.pptx",
  "design_mode": "constrained",
  "accent_strategy": "rotate",
  "footer": {"enabled": true, "left_text": "Acme Corp | Confidential"},
  "chrome": {"page_numbers": true, "skip": ["title", "section"]},
  "theme_override": {
    "colors": {"accent1": "#E31837"},
    "title_font": "Georgia",
    "body_font": "Arial"
  },
  "defaults": {
    "table_style": {"style_id": "@template-default"},
    "cell_style": {"font_size": 11}
  },
  "structure": {
    "sections": [{"title": "Strategy", "slides": [/* ... */]}],
    "auto_agenda": true
  },
  "slides": [/* ... */]
}
```

| Top-level field | Purpose |
|-----------------|---------|
| `template` | Template name (without `.pptx`) |
| `design_mode` | `"constrained"` (default, refuses raw hex + absolute font sizes) or `"free"` |
| `accent_strategy` | `"primary"` (default), `"rotate"`, or `"section-keyed"` |
| `footer` / `chrome` | Persistent footer/header/page-number chrome with skip rules |
| `theme_override` | Per-deck color and font overrides resolved against the scheme |
| `defaults` | Deck-level `table_style` / `cell_style`, swap-only (inline always wins). See [docs/STYLE_DEFAULTS.md](docs/STYLE_DEFAULTS.md) |
| `structure` | Sections, `auto_agenda`, `cover`, `closing` -- expanded into flat slides |
| `slides` | Array of slide definitions |

### Slide Schema

A slide can be plain placeholder content, a raw `shape_grid`, a named `pattern`, or a `compose` envelope.

```json
{
  "layout_id": "One Content",
  "slide_type": "content",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Revenue Overview"},
    {"placeholder_id": "body",  "type": "bullets",
     "bullets_value": ["Revenue up <b>25%</b> YoY", "Margins improved to 68%"]}
  ],
  "speaker_notes": "Emphasize the Q4 recovery.",
  "source": "Company Annual Report, FY2025"
}
```

#### Named Pattern

```json
{
  "slide_type": "blank",
  "content": [{"placeholder_id": "title", "type": "text", "text_value": "Q1 KPIs"}],
  "pattern": {
    "name": "kpi-3up",
    "values": {
      "kpis": [
        {"value": "$12M", "label": "Revenue"},
        {"value": "+25%", "label": "YoY"},
        {"value": "84%",  "label": "NPS"}
      ]
    }
  }
}
```

#### Compose Envelope

```json
{
  "slide_type": "blank",
  "content": [{"placeholder_id": "title", "type": "text", "text_value": "Why now"}],
  "compose": {
    "direction": "vertical",
    "gap": 8,
    "segments": [
      {"size_pct": 40, "pattern": {"name": "stat-hero", "values": {"value": "$2.4B", "label": "TAM"}}},
      {"pattern": {"name": "icon-row", "values": {"items": [/* ... */]}}}
    ]
  }
}
```

#### Raw Shape Grid

```json
{
  "slide_type": "blank",
  "content": [{"placeholder_id": "title", "type": "text", "text_value": "Process Steps"}],
  "shape_grid": {
    "columns": 3, "gap": 2,
    "rows": [{"cells": [
      {"shape": {"geometry": "roundRect", "fill": "accent1",
                 "text": {"content": "Step 1", "size": 14, "bold": true, "color": "lt1"}}},
      {"shape": {"geometry": "roundRect", "fill": "accent2",
                 "text": {"content": "Step 2", "size": 14, "color": "lt1"}}},
      {"shape": {"geometry": "roundRect", "fill": "accent3",
                 "text": {"content": "Step 3", "size": 14, "color": "lt1"}}}
    ]}]
  }
}
```

When `shape_grid` is present with `slide_type: "blank"` (or no `layout_id`), the system uses **virtual layout resolution** to derive safe grid bounds from the template's title and footer geometry. You can also supply explicit `bounds` as slide percentages.

### Content Types

| Type | Description | Value Field |
|------|-------------|-------------|
| `text` | Plain or formatted text | `text_value` |
| `bullets` | Bullet point list | `bullets_value` |
| `body_and_bullets` | Body text followed by bullets | `body_and_bullets_value` |
| `body_and_lead` | Lead sentence + body paragraph | `body_and_lead_value` |
| `bullet_groups` | Grouped bullets with headers | `bullet_groups_value` |
| `table` | Data table (auto-paginates via `split_slide`) | `table_value` |
| `chart` | SVG chart (15 types) | `chart_value` |
| `diagram` | Business diagram (21 types; many native OOXML) | `diagram_value` |
| `image` | Image file | `image_value` |

### Chart Types (15)

`bar`, `grouped_bar`, `stacked_bar`, `line`, `area`, `stacked_area`, `pie`, `donut`, `scatter`, `bubble`, `radar`, `waterfall`, `funnel`, `gauge`, `treemap`

### Diagram Types (21)

`swot`, `timeline`, `process_flow`, `pyramid`, `venn`, `org_chart`, `gantt`, `kpi_dashboard`, `heatmap`, `fishbone`, `pestel`, `porters_five_forces`, `value_chain`, `business_model_canvas`, `nine_box_talent`, `house_diagram`, `panel_layout`, `icon_columns`, `icon_rows`, `stat_cards`, `matrix_2x2`

### Named Patterns (20)

| Pattern | Description |
|---------|-------------|
| `agenda` | Numbered section list for agenda / table-of-contents slides |
| `arch-stack` | Architecture stack diagram with tiers and optional side rails |
| `before-after` | Two-column before/after with transition chevron |
| `bmc-canvas` | Formal 9-cell Business Model Canvas (Osterwalder) |
| `card-grid` | Parameterized N x M grid of titled cards (with optional icons) |
| `comparison-2col` | Two-column comparison with optional headers |
| `icon-row` | Horizontal row of icon+caption pairs |
| `kpi-2up` ... `kpi-6up` | Big-number KPI cards (parametric: 2-6 cards per row) |
| `matrix-2x2` | 2x2 quadrant matrix with axis labels |
| `process-flow` | Left-to-right process flow with steps and decision points |
| `pull-quote` | Italic quote block with attribution |
| `pyramid` | Stacked trapezoid hierarchy (3-5 tiers) |
| `roadmap-phased` | Phased roadmap with workstreams and time periods |
| `stat-hero` | Single oversized statistic with label and optional context |
| `swimlane` | Horizontal swimlane diagram with actors and steps |
| `timeline-horizontal` | Linear horizontal timeline with stops |

Aliases: `timeline`->`timeline-horizontal`, `bmc`->`bmc-canvas`, `matrix`->`matrix-2x2`, `comparison`->`comparison-2col`, `roadmap`->`roadmap-phased`, `architecture`->`arch-stack`, `hero`->`stat-hero`, `quote`->`pull-quote`.

## Templates

| Template | Description |
|----------|-------------|
| `forest-green` | Clean green corporate theme |
| `midnight-blue` | Professional dark blue theme |
| `modern-template` | Modern layout with contemporary styling |
| `warm-coral` | Warm coral accent theme |

Each template provides standard layouts: Title Slide, One Content, Two Content, Section Divider, Closing, and Blank. Missing standard layouts are synthesized at load time.

```sh
json2pptx skill-info --mode=list
json2pptx skill-info --mode=full --template=midnight-blue
json2pptx template-check templates/midnight-blue.pptx     # conformance against docs/TEMPLATE_SPEC.md
```

Every file shipped under `templates/` is gated by `internal/template/conformance_corpus_test.go`, which runs as part of `go test ./...` and asserts zero `FAIL` and zero `WARN`. See [CONTRIBUTING.md → "Adding a Template"](CONTRIBUTING.md#adding-a-template) for the procedure and the sha256-keyed allow-list mechanism for legacy known-exceptions.

### Creating a Custom Template

Any `.pptx` file can serve as a template. Open PowerPoint (or Keynote, Google Slides, LibreOffice Impress), design your slides in **Slide Master** view, save as `.pptx`. json2pptx analyzes the layouts at runtime and maps your JSON content to the right placeholders.

#### Required Layouts (4 minimum)

| Layout | Purpose | Placeholders | Maps to `slide_type` |
|--------|---------|--------------|----------------------|
| **Title Slide** | Opening slide | Title + Subtitle | `title` |
| **Content** | Main content slides | Title + Body | `content`, `chart`, `diagram` |
| **Section Divider** | Section breaks | Title (+ optional Body) | `section` |
| **Closing** | Final slide | Title + Subtitle | `blank` (closing tag) |

Layouts are detected by **placeholder structure**, not by name. A "Two Content" layout and a "Blank" layout are synthesized automatically when missing.

#### Theme and Colors

json2pptx extracts your template's theme colors (`accent1`-`accent6`, `dk1`, `dk2`, `lt1`, `lt2`) and uses them for native diagram shapes, chart palettes, shape grid fills (via scheme references like `"fill": "accent1"`), and automatic text contrast correction.

```sh
mkdir my-templates && cp my-corporate-theme.pptx my-templates/

json2pptx generate -json slides.json -template my-corporate-theme -templates-dir my-templates
json2pptx validate-template my-templates/my-corporate-theme.pptx
json2pptx skill-info --mode=full --template=my-corporate-theme --templates-dir=my-templates
```

#### Tips

- Design in **16:9** aspect ratio
- Edit in **Slide Master** view, not on individual slides
- Use standard PowerPoint placeholder types (Title, Body, Subtitle), not plain text boxes
- Test classification with `json2pptx skill-info --mode=full`

## CLI

The `json2pptx` binary is the primary tool: batch converter, HTTP API server, and MCP server.

### Subcommands (39)

Run `json2pptx help` for the authoritative list, or `json2pptx <command> -h` for
per-command flags. A handful of MCP tools have no direct subcommand (e.g.
`make_deck`, `auto_repair`, `apply_deck_patch`, `repair_slides_batch`,
`expand_patterns`, `propose_repairs`) — `json2pptx help` prints the CLI workaround
for each under "MCP-only tools".

| Command | Description |
|---------|-------------|
| `generate` | Convert JSON input to PPTX (default if subcommand omitted) |
| `read` | Read PPTX and output extracted content as JSON |
| `validate` | Validate JSON input without generating (see [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) for `-fit-report`) |
| `preflight` | Run every static check on a deck (stage-based, emits the finding envelope) |
| `validate-output` | Check generated PPTX for OOXML correctness |
| `validate-template` | Check template compatibility |
| `template-check` | Check template conformance against `docs/TEMPLATE_SPEC.md` |
| `examine-template` | Emit a full template capability report (visual + XML + canonical roles) |
| `patterns` | List, show, validate, and expand named patterns |
| `icons` | List available icon sets and icons |
| `preview-icon` | Render a single icon spec to SVG + PNG preview |
| `tables` | Table style guide and density reference |
| `skill-info` | Show template capabilities for Claude Code skill integration |
| `capabilities` | Show schema version, tools (with classification), features, and vocabularies |
| `get-started` | Print the recommended fast path + manual sequence for a task (brief/revise/validate-only) |
| `describe-finding` | Print the agent-facing description for a single finding code |
| `input-schema` | Print the JSON input schema (full or compact) |
| `resolve-theme` | Resolve theme colors and fonts for a template |
| `recommend-pattern` | Recommend patterns matching an intent |
| `recommend-visual` | Recommend visual approaches for a slide intent |
| `plan-deck` | Plan a deck outline from a brief |
| `preview` | Preview generation plan without rendering |
| `preview-wireframe` | Render a slide-plan wireframe (PNG) before generating |
| `preview-patterns` | Pre-render PNG previews for every named pattern |
| `repair` | Apply targeted fixes to a single slide |
| `score` | Score a JSON deck spec for visual quality (deterministic) |
| `score-candidates` | Rank candidate slides for one slot without rendering |
| `inspect` | Run vision-based visual QA on rendered slide images |
| `analyze-rhythm` | Analyze deck visual rhythm and pattern repetition |
| `render-slide` | Render a single slide to PNG (requires LibreOffice + ImageMagick) |
| `render-slide-from-json` | Render one slide directly from JSON (no full deck render) |
| `render-thumbnails` | Render all slides as PNG thumbnails (requires LibreOffice + ImageMagick) |
| `audit-palette` | Render PPTX to PNG and report ΔE between chart pics and adjacent solid-filled shapes |
| `template-settings` | Manage named styles (list/register/delete) |
| `data-format-hints` | Show data format hints for chart/diagram types |
| `shape-catalog` | List available preset geometries |
| `serve` | Start HTTP API server |
| `mcp` | Start MCP server over stdio |
| `version` | Show version, commit, and build information |

### Generate Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-json` | (required) | Path to JSON input file, or `-` for stdin |
| `-template` | | Template name (without `.pptx`) |
| `-templates-dir` | `./templates` | Directory containing templates |
| `-output` | `./output` | Output directory for generated PPTX files (or a `.pptx` file path, e.g. `/tmp/deck.pptx`) |
| `-json-output` | | Path for JSON result output (headless mode) |
| `-dry-run` / `-n` | `false` | Validate input and show layout selections without generating |
| `-strict-fit` | `warn` | Text-fit checking mode: `off`, `warn`, or `strict` (refuse on overflow) |
| `-fit-report` | `false` | Emit structured fit findings |
| `-verbose` | `false` | Enable verbose output |
| `-config` | | Path to config file |

### Companion Tools

| Binary | Purpose |
|--------|---------|
| `pptx2jpg` | Convert PPTX to JPG/PNG via LibreOffice + ImageMagick |
| `mktemplate` | Template authoring helper |
| `templatecaps` | Template capabilities inspector |
| `debugcolors` | Theme color introspector |
| `validatepptx` | Standalone PPTX validator |
| `testrand` | Random deck generator (fuzz harness) |

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check with version info |
| `GET` | `/api/v1/templates` | List available templates |
| `GET` | `/api/v1/templates/{name}` | Template details (layouts, colors, fonts) |
| `GET` | `/api/v1/slide-types` | List supported slide types |
| `POST` | `/api/v1/convert` | Convert JSON slides to PPTX |
| `GET` | `/api/v1/download/{filename}` | Download generated file (expires after 1 hour by default) |
| `GET` | `/api/v1/patterns` | List available named patterns |
| `GET` | `/api/v1/patterns/{name}` | Pattern details and schema |
| `POST` | `/api/v1/patterns/{name}/validate` | Validate input against a pattern's schema |
| `POST` | `/api/v1/patterns/{name}/expand` | Expand a pattern into a shape grid |

See [docs/api/README.md](docs/api/README.md) for complete API documentation.

### Convert Request Fields

| Field | Required | Description |
|-------|----------|-------------|
| `template` | Yes | Template name (without `.pptx`) |
| `slides` | Yes | Array of slide definitions |
| `options.output_format` | No | `"file"` or `"base64"` (default: `"file"`) |
| `options.svg_scale` | No | SVG render scale factor, 0.5-10.0 (default: 2.0) |
| `options.exclude_template_slides` | No | Exclude template's built-in slides from output |

### Convert Response

```json
{
  "success": true,
  "file_url": "/api/v1/download/abc123.pptx",
  "expires_at": "2026-01-17T12:00:00Z",
  "stats": {"slide_count": 10, "processing_time_ms": 1500, "warnings": []}
}
```

## Configuration

Set environment variables directly or via a `.env` file. See `.env.example` for the complete reference.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `TEMPLATES_DIR` | `./templates` | Template directory |
| `OUTPUT_DIR` | `./output` | Output directory |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `TEMPLATE_VALIDATION_MODE` | `soft` | `strict` or `soft` |
| `TEMP_FILE_MAX_AGE` | `3600` | Max age for temp files before cleanup (seconds) |
| `TEMP_CLEANUP_INTERVAL` | `300` | Temp file cleanup interval (seconds) |
| `SVG_STRATEGY` | `png` | `png`, `emf`, or `native` |
| `SVG_SCALE` | `2.0` | Scale factor for PNG conversion |
| `SVG_NATIVE_COMPATIBILITY` | `warn` | `warn`, `fallback`, `strict`, `ignore` |
| `SVG_PNG_CONVERTER` | `auto` | `auto`, `rsvg-convert`, or `resvg` |
| `SERVER_AUTH_MODE` | | HTTP API auth mode |
| `ALLOWED_ORIGINS` | | CORS allowlist |
| `CONVERT_RATE_LIMIT` | | Rate limit for `/convert` |
| `CONVERT_TIMEOUT` | | Per-request timeout for `/convert` |
| `ALLOWED_IMAGE_PATHS` | | Allowed local image roots |
| `JSON2PPTX_ALLOW_SETTINGS_WRITE` | `0` | Set to `1` to enable `register_template_setting` / `delete_template_setting` |
| `PPROF_PORT` / `PPROF_BIND` | | pprof endpoint |

### SVG Conversion Strategies

| Strategy | Description | Requirements | Compatibility |
|----------|-------------|--------------|---------------|
| `png` | Convert SVG to PNG (default) | `rsvg-convert` or `resvg` | Universal |
| `emf` | Convert SVG to EMF vector format | Inkscape | PowerPoint 2010+ |
| `native` | Embed SVG directly with PNG fallback | `rsvg-convert` or `resvg` for fallback | PowerPoint 2016+ |

```sh
brew install librsvg          # macOS
apt install librsvg2-bin      # Ubuntu/Debian
```

### Config File

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
  json2pptx/        Main CLI + HTTP API + MCP server (39 subcommands, 45 MCP tools)
  pptx2jpg/         PPTX to image conversion via LibreOffice
  mktemplate/       Template authoring helper
  debugcolors/      Theme color debugging tool
  templatecaps/     Template capabilities inspector
  validatepptx/     Standalone PPTX validator
  testrand/         Random deck generator (fuzz harness)
internal/
  api/              HTTP API handlers and routing
  config/           Configuration loading
  diagnostics/      Structured diagnostic envelopes
  generator/        Core PPTX generation engine, contrast, shapes
  jsonschema/       JSON Schema validation
  layout/           Layout selection (heuristic scoring)
  layoutpreview/    Generation plan previewing
  pagination/       Slide pagination and content splitting
  patterns/         Named shape grid pattern registry (20 patterns)
  pipeline/         Generation pipeline orchestration
  pptx/             Low-level OOXML manipulation
  pptxread/         PPTX read-back (for read_presentation)
  render/           Slide-image rendering
  resource/         Embedded resource handling
  safeyaml/         Safe YAML parsing
  shapegrid/        Shape grid layout engine (flex-like rows, authoritative bounds)
  slidepath/        JSON Pointer path helpers
  template/         PPTX template analysis and layout classification
  templatesettings/ Template-side named style sidecars
  textfit/          Text fitting and overflow handling
  testrand/         Random test data generation
  testutil/         Test helpers
  types/            Shared data types and input schema
  utils/            Utilities
  visualqa/         Visual QA agent integration (deterministic + screenshot)
svggen/             SVG chart and diagram generation (separate Go module)
templates/          Built-in PPTX templates (4)
examples/           Example JSON input files (19 + diagrams/ subdirectory)
docs/               Specs: PATTERNS, FIT_FINDINGS, REPAIR_LOOP, INPUT_FORMAT,
                    STYLE_DEFAULTS, TEMPLATE_SPEC, VISUAL_CRITERIA, ...
skills/             Claude Code skills (3)
```

### Pipeline Flow

```
JSON Input + Template PPTX
        |
        v
  Parse JSON Input + apply deck defaults --> Slides[], Template, ThemeOverride, Defaults, Structure
        |
        v
  Expand structure (sections, auto-agenda, cover/closing) --> flat slide list
        |
        v
  Validate (schema, unknown keys, design_mode rules)
        |
        v
  Analyze Template --> Layouts[], Placeholders[], ThemeColors; synthesize missing layouts
        |
        v
  Resolve patterns/compose --> shape_grid; resolve virtual layout bounds
        |
        v
  Select Layouts and map portable placeholders
        |
        v
  Generate PPTX                                        --> Populated PPTX file
    |-- Charts (svggen)        --> SVG -> PNG/EMF/native
    |-- Diagrams (svggen + native) --> some intercepted as native OOXML shapes
    |-- Tables                  --> with auto-pagination via split_slide
    |-- Shape Grid              --> custom shape layout
    |-- Images                  --> embedded
    '-- Contrast + Text Fit     --> auto-fix or report findings
        |
        v
  Result + FitFindings + ContrastSwaps + ContentHash
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

# svggen is a separate Go module
cd svggen && go test ./...
cd svggen && golangci-lint run ./...
```

### Pre-Commit Checklist

1. `golangci-lint run ./...`
2. `cd svggen && golangci-lint run ./...`
3. `go test ./...`
4. `cd svggen && go test ./...`
5. `go build ./cmd/json2pptx`

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

## Documentation

- `get_input_schema` (MCP) / `json2pptx input-schema` (CLI) -- canonical JSON Schema for `PresentationInput`
- [docs/INPUT_FORMAT.md](docs/INPUT_FORMAT.md) -- tutorial with worked examples
- [SLIDE_FORMAT.md](SLIDE_FORMAT.md) -- short quickstart
- [docs/PATTERNS.md](docs/PATTERNS.md) -- named-pattern authoring guide
- [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) -- fit findings catalog and action semantics
- [docs/REPAIR_LOOP.md](docs/REPAIR_LOOP.md) -- structured repair workflow
- [docs/STYLE_DEFAULTS.md](docs/STYLE_DEFAULTS.md) -- defaults block semantics
- [docs/TEMPLATE_SPEC.md](docs/TEMPLATE_SPEC.md) -- template conformance spec
- [docs/VISUAL_CRITERIA.md](docs/VISUAL_CRITERIA.md) -- composition scoring criteria
- [docs/SCHEMA_CHANGELOG.md](docs/SCHEMA_CHANGELOG.md) -- schema/version history
- [docs/PATH_GRAMMAR.md](docs/PATH_GRAMMAR.md) -- JSON Pointer path grammar for fit findings
- [docs/api/README.md](docs/api/README.md) -- HTTP API reference

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style guidelines, testing requirements, and the pull request process.

## License

Licensed under the Apache License, Version 2.0. You may use, modify, and distribute this software freely, including for commercial purposes, provided you include the [LICENSE](LICENSE) and [NOTICE](NOTICE) files as required by the license. See [LICENSE](LICENSE) for the full terms.

**Commercial licensing** is available for organizations that need different terms (white-label use without attribution, warranty, support/SLA). Contact platon2001@icloud.com.

Third-party license information is documented in [LICENSE-THIRD-PARTY.md](LICENSE-THIRD-PARTY.md).
