# json2pptx vs svggen — MCP / API Surface Harmonization TODOS

## 1. Current State: What Already Works

**Charts/diagrams in shape-grid cells ALREADY EXIST and function end-to-end.**

- `GridCellInput` has a `Diagram *types.DiagramSpec` field (`internal/jsonschema/shapegrid.go:71`)
- `shapegrid.CellKindDiagram` is a recognized cell kind (`internal/shapegrid/types.go:21`)
- `generateDiagramCellInserts()` renders svggen diagrams as native SVG into grid cells (`cmd/json2pptx/shape_grid.go:542`)
- A 2x2 grid holding 4 charts is **already possible** by setting `"diagram": {"type": "bar_chart", ...}` in each cell

**Example (already valid input today):**

```json
{
  "layout_id": "blank",
  "shape_grid": {
    "columns": 2,
    "rows": [
      {
        "cells": [
          {"diagram": {"type": "bar_chart", "title": "Q1", "data": {}}},
          {"diagram": {"type": "line_chart", "title": "Q2", "data": {}}}
        ]
      },
      {
        "cells": [
          {"diagram": {"type": "pie_chart", "title": "Q3", "data": {}}},
          {"diagram": {"type": "donut_chart", "title": "Q4", "data": {}}}
        ]
      }
    ]
  }
}
```

---

## 2. MCP Surface Deltas

### 2.1 Two separate, disconnected MCP servers

| | **json2pptx MCP** | **svggen-mcp** |
|---|---|---|
| **Binary** | `json2pptx mcp` | `svggen-mcp` |
| **Transport** | stdio | stdio |
| **Tools** | 24 tools | 4 tools |
| **Context** | Templates, themes, layouts, settings | None (standalone) |
| **Version** | `3.x.x` | `0.1.0` |

**json2pptx tools (24):** `generate_presentation`, `list_templates`, `get_data_format_hints`, `get_chart_capabilities`, `get_diagram_capabilities`, `validate_input`, `recommend_pattern`, `recommend_visual`, `list_patterns`, `show_pattern`, `validate_pattern`, `expand_pattern`, `list_icons`, `table_density_guide`, `resolve_theme`, `render_slide_image`, `render_deck_thumbnails`, `score_deck`, `preview_presentation_plan`, `repair_slide`, `list_template_settings`, `register_template_setting`, `delete_template_setting`, `analyze_deck_rhythm`, `get_capabilities`, `read_presentation`, `get_shape_catalog`

**svggen-mcp tools (4):** `render_diagram`, `list_diagram_types`, `validate_diagram`, `get_diagram_schema`

### 2.2 Naming & taxonomy drift

| Concept | json2pptx MCP | svggen-mcp |
|---|---|---|
| Chart types | `get_chart_capabilities` | No equivalent tool |
| Diagram types | `get_diagram_capabilities` | `list_diagram_types` |
| Type list source | `svggen.ChartCapabilities()` / `svggen.DiagramCapabilitiesReady()` | `svggen.Types()` |
| Rendering | Embedded in `generate_presentation` | `render_diagram` |
| Validation | Embedded in `validate_input` | `validate_diagram` |
| Schema lookup | No per-type schema tool | `get_diagram_schema` |

**Problem:** An agent connected to json2pptx cannot preview/render a single chart for inspection without generating an entire deck. To do that, it must spin up a **second** MCP connection to `svggen-mcp`, which has no template/theme context and uses different parameter shapes.

### 2.3 Parameter shape differences

**json2pptx `generate_presentation` (placeholder content type):**
```json
{"placeholder_id": "body", "type": "chart", "chart_value": {"type": "bar", "title": "...", "data": {}}}
```

**json2pptx `shape_grid` cell (already works):**
```json
{"diagram": {"type": "bar_chart", "title": "...", "data": {}}}
```

**svggen-mcp `render_diagram`:**
```json
{"type": "bar_chart", "data": {}, "format": "svg", "width": 800, "height": 600, "title": "...", "style": {}}
```

**svggen HTTP `POST /render`:**
```json
{"type": "bar_chart", "data": {}, "output": {"format": "svg", "width": 800, "height": 600}}
```

**Gaps:**
- `width`/`height` are top-level floats in svggen-mcp but nested in `output` for svggen HTTP
- json2pptx uses `types.DiagramSpec` (`Type`, `Title`, `Data` flat); svggen uses `RequestEnvelope` (`Type`, `Title`, `Data`, `Output`, `Style`)
- svggen-mcp `render_diagram` supports `style` object (palette overrides); json2pptx grid diagrams pass `nil` theme colors and no style

---

## 3. HTTP API Deltas

| | **json2pptx HTTP** | **svggen HTTP** |
|---|---|---|
| **Health** | `GET /api/v1/health` | `GET /healthz` |
| **Types list** | Inlined in `GET /api/v1/templates` | `GET /types` |
| **Render** | `POST /api/v1/convert` (full deck) | `POST /render` (single diagram) |
| **Batch render** | Not supported | `POST /render/batch` |
| **Auth** | None | Configurable (auth middleware + rate limiter) |
| **Cache** | None | `RenderCache` with TTL |

**Gap:** json2pptx HTTP API has no way to render a single diagram or a batch of diagrams for preview. svggen HTTP has batch but no PPTX context.

---

## 4. Data Model / Integration Gaps

### 4.1 Theme colors are NOT passed to grid-cell diagrams

**Placeholder-based diagrams** (`internal/generator/media.go:861`):
```go
rendered, err := renderDiagramSpecFull(diagramSpec, ctx.themeColors, ...)
```

**Grid-cell diagrams** (`cmd/json2pptx/shape_grid.go:543`):
```go
result, err := generator.RenderDiagramSpecWithMetadata(cell.DiagramSpec, nil, 0, true)
//                                               themeColors is ^^^ nil
```

**Impact:** A chart in a grid cell may render with a different color palette than the same chart placed in a placeholder, because the template's `themeColors` are not forwarded.

### 4.2 Narrow-placeholder warnings skip grid cells

The `complexDiagramTypes` + `checkDiagramInNarrowPlaceholder()` logic (`internal/generator/media.go:534`) only runs for placeholder-based `ContentItem` diagrams. A `matrix_2x2` or `org_chart` crammed into a small grid cell gets **no warning**.

### 4.3 Missing `fit_report` for grid diagrams

Grid-cell diagrams are rendered as SVG at the shape_grid resolution phase. Their text-fit findings are NOT collected into the generator's `fitFindings`, unlike placeholder diagrams which emit overflow/clamping findings.

### 4.4 `diagram` cell type is invisible in docs

- `generate_presentation` tool description lists content types (`text`, `bullets`, `table`, `chart`, `diagram`, `image`) but does **not** mention `diagram` can live inside a `shape_grid` cell
- The skill docs say grid cell types are `"shape" or "table"` — omitting `diagram`
- No example in `examples/` shows a grid with chart cells

---

## 5. Action Items

### P1 - Critical (breaks agent predictability)

| # | Issue | Fix |
|---|---|---|
| 1 | **Theme colors not passed to grid diagrams** | Thread `themeColors` through `resolveShapeGrid` → `generateDiagramCellInserts` so grid charts match template palette |
| 2 | **No narrow-cell warnings for grid diagrams** | Run `checkDiagramInNarrowPlaceholder` equivalent against `cell.Bounds` in `generateDiagramCellInserts` |
| 3 | **MCP tool descriptions omit `diagram` grid cells** | Update `mcpGenerateTool()` description to document `{"diagram": {...}}` as a valid grid cell type |

### P2 - High (harmonization)

| # | Issue | Fix |
|---|---|---|
| 4 | **Add svggen rendering tools to json2pptx MCP** | Add `render_diagram_preview` and `validate_diagram_input` tools to json2pptx MCP that proxy to `svggen.RenderMultiFormat` with full template theme context |
| 5 | **Unify parameter shapes** | Either: (a) make svggen-mcp accept `RequestEnvelope` shape, or (b) add a compatibility layer so `type`/`data`/`title` work consistently across both servers |
| 6 | **Add batch render to json2pptx MCP** | Expose `render_diagram_batch` (mirrors svggen HTTP `POST /render/batch`) so agents can preview multiple charts in one call |

### P3 - Medium (quality of life)

| # | Issue | Fix |
|---|---|---|
| 7 | **Add `get_diagram_schema` to json2pptx MCP** | Agents shouldn't need svggen-mcp just to look up a schema; delegate to `svggen` registry from json2pptx |
| 8 | **Collect fit findings for grid diagrams** | Wire `renderDiagramSpecWithMetadata` findings into the generator's `fitFindings` for grid cells |
| 9 | **Add example deck with 2x2 chart grid** | Create `examples/diagrams/grid-2x2-charts.json` showing the pattern |

### P4 - Low (HTTP API parity)

| # | Issue | Fix |
|---|---|---|
| 10 | **json2pptx HTTP has no diagram preview endpoint** | Add `POST /api/v1/preview/diagram` that returns SVG/PNG for a single diagram with template theme applied |
| 11 | **Consider deprecating standalone `svggen-mcp`** | If json2pptx MCP subsumes all svggen tools with proper context, `svggen-mcp` becomes redundant |

---

## 6. Bottom Line

> **The feature "put svggen charts into grid slots" already works.** What's missing is:
> 1. **Theme color forwarding** (grid charts look off-template)
> 2. **Agent discoverability** (docs don't say it's possible)
> 3. **A unified MCP surface** (agents need two servers to do one job)
>
> The highest-ROI fix is to add diagram-preview tools directly into the json2pptx MCP (P2 #4-#6) and fix the theme-color thread (P1 #1), because that eliminates the need for a second MCP connection while making grid charts visually consistent with placeholder charts.
