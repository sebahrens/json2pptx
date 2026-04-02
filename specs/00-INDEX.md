# Go Slide Creator — Specification Index

Convert JSON slide definitions to PowerPoint presentations with intelligent layout selection.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HTTP API (06)                                   │
│                         POST /api/v1/convert                                │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          PIPELINE STAGES                                     │
│                                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌───────────┐ │
│  │    JSON      │───▶│   Template   │───▶│    Layout    │───▶│   PPTX    │ │
│  │    Input     │    │   Analyzer   │    │   Selector   │    │ Generator │ │
│  │             │    │     (02)     │    │     (03)     │    │    (04)   │ │
│  └──────────────┘    └──────────────┘    └──────────────┘    └─────┬─────┘ │
│                                                                     │       │
│                                          ┌──────────────────────────┤       │
│                                          │                          │       │
│                                          ▼                          ▼       │
│                                   ┌────────────┐            ┌────────────┐  │
│                                   │   Chart    │            │   Image    │  │
│                                   │  Renderer  │            │  Embedding │  │
│                                   │    (05)    │            │    (08)    │  │
│                                   └────────────┘            └────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SUPPORT SYSTEMS                                     │
│                                                                              │
│          ┌──────────────┐    ┌──────────────┐    ┌───────────┐           │
│          │     EMU      │    │Relationships │    │  Visual   │           │
│          │   Helpers    │    │   Manager    │    │ Inspector │           │
│          │     (09)     │    │     (10)     │    │    (11)   │           │
│          └──────────────┘    └──────────────┘    └───────────┘           │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

```
JSON Input + Template PPTX
        │
        ▼
┌───────────────────┐
│  Parse JSON Input │  → Slides[], Template, ThemeOverride
│                   │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Analyze Template  │  → Layouts[], Placeholders[], ThemeColors
│       (02)        │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│  Select Layouts   │  → LayoutAssignments[] (heuristic + optional LLM)
│       (03)        │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│  Generate PPTX    │  → Populated PPTX file
│       (04)        │
│   ├─ Charts (05)  │  → Rendered chart images/SVG
│   └─ Images (08)  │  → Embedded images with relationships
└───────────────────┘
```

## Specification Files

### Core Pipeline (read in order for new implementations)

| Spec | Purpose | Key Types |
|------|---------|-----------|
| [02-template-analyzer](02-template-analyzer.md) | Analyze PPTX template capabilities | `TemplateAnalysis`, `LayoutInfo`, `PlaceholderInfo` |
| [03-layout-selector](03-layout-selector.md) | Match slides to layouts | `SelectionRequest`, `SelectionResult`, scoring |
| [04-pptx-generator](04-pptx-generator.md) | Populate template with content | `GenerationRequest`, `SlideSpec`, `ContentItem` |

### Content Rendering

| Spec | Purpose | Key Types |
|------|---------|-----------|
| [05-chart-renderer](05-chart-renderer.md) | Generate charts as images | `ChartSpec`, `ChartResult`, style templates |
| [08-image-embedding-fix](08-image-embedding-fix.md) | Embed images in PPTX | `ImageContent`, relationship management |
| [18-svggen-package](18-svggen-package.md) | Canvas-native SVG generation | `RequestEnvelope`, `Diagram`, `SVGBuilder` |

### Infrastructure

| Spec | Purpose | When to Read |
|------|---------|--------------|
| [06-http-api](06-http-api.md) | REST API endpoints | Adding/modifying endpoints |
| [09-emu-helpers](09-emu-helpers.md) | EMU ↔ pixel/inch conversion | Working with PPTX coordinates |
| [10-relationships-manager](10-relationships-manager.md) | OOXML relationship IDs | Adding new relationship types |

### Quality & Testing

| Spec | Purpose | When to Read |
|------|---------|--------------|
| [11-visual-inspection](11-visual-inspection.md) | LLM-based visual QA | Visual quality issues |

### Decision Records

| Spec | Decision | Status |
|------|----------|--------|
| [14-performance-optimizations](14-performance-optimizations.md) | Performance improvements | Partial |
| [15-svg-handling-decision](15-svg-handling-decision.md) | SVG → PNG/EMF/Native strategies | Implemented |
| [16-chart-backend-decision](16-chart-backend-decision.md) | Chart rendering backend | Implemented |
| [17-visual-inspection-loop](17-visual-inspection-loop.md) | Visual QA tooling | Implemented |
| [19-pptx-svg-compatibility](19-pptx-svg-compatibility.md) | PPTX/SVG viewer compatibility | Implemented |
| [22-svg-text-sizing-investigation](22-svg-text-sizing-investigation.md) | SVG text sizing architecture | Recommendation |

### Layout & Grid

| Spec | Purpose | When to Read |
|------|---------|--------------|
| [24-virtual-layouts](24-virtual-layouts.md) | Virtual layout resolution for shape grids | Adding grid features, changing layout selection |
| [25-shape-grid](25-shape-grid.md) | Shape grid JSON schema and layout engine | Shape grid features, cell types, text formatting |

### Integration & Distribution

| Spec | Purpose | When to Read |
|------|---------|--------------|
| [20-claude-code-skill](20-claude-code-skill.md) | Claude Code skill integration | Skill packaging, subcommands, distribution |

## Quick Reference: Which Spec to Read

| Task | Start With |
|------|------------|
| Add new slide type | 03, then 04 |
| Support new chart type | 05 |
| Add new diagram type | 18 (svggen), then 05 |
| Add new placeholder type | 02, then 04 |
| Change layout scoring | 03 |
| Add API endpoint | 06 |
| Embed new media type | 08, 10 |
| Fix visual rendering issue | 11, then relevant component |
| Fix SVG text sizing issue | 22, then 18 (svggen) |
| Security concern | 12 |
| Add shape grid feature | 25, then 24 |
| Change virtual layout resolution | 24 |
| Claude Code skill packaging | 20 |

## Key Files by Package

```
internal/
├── api/            → 06-http-api
├── chartutil/      → Chart utility helpers
├── config/         → Configuration management
├── data/           → Embedded data assets
├── generator/      → 04-pptx-generator (includes native OOXML diagram shapes)
├── layout/         → 03-layout-selector, canonical layout resolution
├── pagination/     → Content pagination logic
├── parser/         → Input parsing
├── pipeline/       → Generation pipeline orchestration
├── pptx/           → 08, 09, 10 (low-level PPTX manipulation)
├── preview/        → Slide preview generation
├── resource/       → URL resolution and resource fetching
├── shapegrid/      → 25-shape-grid (grid layout engine, shape XML generation)
├── template/       → 02-template-analyzer (normalization, synthesis, font resolver)
├── testrand/       → Random E2E test generator
├── textfit/        → Text autofit and font scaling
├── themegen/       → Theme generation utilities
├── types/          → Shared types (EMU, ChartSpec, DiagramSpec, TableSpec, etc.)
├── utils/          → General utilities
├── visualqa/       → Visual QA tooling
└── workerpool/     → Concurrent worker pool

svggen/             → 18-svggen-package (SVG chart/diagram generation, separate Go module)
├── core/           → Foundational types, interfaces, registry (no diagram implementations)
└── (diagrams)      → Built-in diagram implementations (bar, line, pie, etc.)

icons/              → Bundled SVG icon library (outline + filled variants)

cmd/
├── debugcolors/    → Debug color scheme rendering
├── json2pptx/      → CLI tool (generate, validate, serve, skill-info, MCP server)
├── pptx2jpg/       → PPTX to image conversion (for visual inspection)
├── testrand/       → Random E2E test runner
└── validatepptx/   → PPTX structural validator
```

## Implementation Status

All core pipeline specs (02-06) are **implemented and tested**.

See individual specs for detailed acceptance criteria and test coverage.
