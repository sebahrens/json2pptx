# Fit Findings

Fit findings are structured diagnostics emitted when generated slide content may not render correctly — text overflowing placeholders, shapes falling outside slide bounds, or tables exceeding density limits. They are surfaced via the MCP `generate_presentation` tool (text-fit findings detected by `strict_fit` are merged into `fit_findings` unconditionally when `strict_fit != "off"`; the full preflight detector set runs when `fit_report=true`) and the CLI `json2pptx generate -json` and `validate -fit-report` commands (the JSON output's `fit_findings` always includes the active `strict_fit` findings).

**Chart / diagram dry-render.** `validate_input` and `preview_presentation_plan` also drive svggen's layout/labeling pass for every `chart_value` / `diagram_value` content item **and every diagram surface embedded in a slide's `shape_grid`** — cell `diagram`s, composite cell `sub_diagram`s, and diagrams inside recursively nested sub-grids — merging the resulting `chart.*` findings (e.g. `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`) into the fit-finding stream. Content-item findings keep the legacy `slides[i].content[j].chart_value` path; shape_grid findings use the slidepath JSON Pointer convention shared with the structural detectors (e.g. `/slides/0/shape_grid/rows/1/cells/2/diagram`, `.../composite/sub_diagram`, or a nested `.../cells/2/grid/rows/0/cells/1/diagram`). Agents see render-time chart issues at validate / preview time without paying for full generation. The strict-fit severity ladder applies identically to the generate path. The svggen top-level helper is `svggen.DryRender(req) ([]Finding, error)`; the corresponding MCP entry point is `render_diagram` with `dry_run: true`.

## Output-Validation Findings — Separate Category

This document catalogs **fit findings** (`patterns.FitFinding`) emitted by the layout/textfit/chart preflight and runtime. They are distinct from **output-validation findings** (`pptx.Finding`) emitted by `internal/pptx.OutputValidator` after the `.pptx` is serialized, and from **visual-QA findings** emitted by the `slide-visual-qa` Haiku skill from rendered screenshots:

| | Fit findings | Output-validation findings | Visual-QA findings |
|---|---|---|---|
| Source type | `patterns.FitFinding` (`internal/patterns/fit_finding.go`) | `pptx.Finding` (`internal/pptx/output_validator.go`) | JSON `findings[]` block emitted by `skills/slide-visual-qa/SKILL.md` |
| When emitted | Before / during render (text overflow, density, chart layout) | After write, against the serialized package | After render-to-image, from screenshot inspection |
| Severity values | `action` ∈ `refuse` / `shrink_or_split` / `review` / `info` | `severity` ∈ `blocking` / `warning` | `severity` ∈ `blocking` / `warning` / `info` |
| Code prefixes | bare codes (`placeholder_overflow`), `chart.*`, `BODY_TOO_LONG`, etc. (this document) | `OPC_*` (package integrity) and `OOXML_*` (content validity) | aesthetic codes (`ACCENT_OVERLOAD`, `BASELINE_MISALIGN`, `MISSING_TAKEAWAY`, `CHART_BORDER`, `CHART_VERTICAL_GRIDLINES`, `REDUNDANT_LEGEND`, `NON_TABULAR_NUMS`, `EYEBROW_NO_CAPS`) plus the screenshot-rendering categories listed in the skill |
| MCP response field | `fit_findings[]` on the success envelope | error envelope `findings[]` (strict mode) or `output_validation_findings[]` (warn mode) | not an MCP tool — consumed by `auto_repair` after the skill runs |
| Repair path | `repair_slide` with a `Fix.Kind` directive | `repair_slide` with a directive chosen per finding's `code` + `scope` | mapped to `repair_slide` directives via `internal/visualqa/repair_map.go` |

Output-validation codes are the "zero needs repair" contract: in strict mode (the default) a blocking `OPC_*` or `OOXML_*` finding fails generation outright. See [skills/generate-deck/SKILL.md → Output Validation Guarantee](../skills/generate-deck/SKILL.md#output-validation-guarantee) for the envelope shape and response protocol. Authoritative code list: `opcCodeMap` and `ooxmlCodeMap` in `internal/pptx/output_validator.go`.

Visual-QA codes are subjective image-derived findings. They never block generation; they raise the bar from "renders correctly" to "looks consulting-grade." The full catalog of aesthetic codes is below in the [Visual-QA Aesthetic Findings](#visual-qa-aesthetic-findings) section.

## Finding Structure

Every finding is a `FitFinding` (defined in `internal/patterns/fit_finding.go`) that embeds `ValidationError`. In JSON output, all fields are flattened to the top level:

```json
{
  "pattern": "placeholder",
  "path": "/slides/0/content/body",
  "code": "placeholder_overflow",
  "message": "text overflows placeholder by 42% (360pt frame, autofit=none); overflow persists at minimum font scale",
  "fix": { "kind": "reduce_text" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 7772400, "height_emu": 6515100 },
  "allowed": { "width_emu": 7772400, "height_emu": 4572000 },
  "overflow_ratio": 1.42
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `pattern` | string | Source context: `"placeholder"`, `"table"`, `"shape_grid"` |
| `path` | string | JSON Pointer (RFC 6901) to the offending element, e.g. `/slides/2/content/body`. See [PATH_GRAMMAR.md](PATH_GRAMMAR.md). |
| `code` | string | Machine-readable code (see catalog below) |
| `message` | string | Human-readable description |
| `fix` | object | Structured remediation: `{kind, params?}` |
| `action` | string | Recommended severity/remediation action |
| `measured` | object | Actual content extent in EMU (omitted when N/A) |
| `allowed` | object | Available frame extent in EMU (omitted when N/A) |
| `overflow_ratio` | float | `measured / allowed` as a fraction (omitted when 0) |
| `next_tool_call` | object | Machine-readable MCP tool suggestion: `{tool, args_template}` (omitted for `info` findings) |
| `segment_index` | integer | 0-based child segment index inside a compose envelope when the finding is attributable to one (omitted otherwise). Populated for compose-emitted findings (`COMPOSE_HORIZONTAL_TRUNCATION`, `COMPOSE_SEGMENT_BOUNDS_IGNORED`, `COMPOSE_SEGMENT_EXPAND_FAILED`) and for per-cell findings whose merged-grid row/col falls inside a segment's `row_range`/`col_range` (see `preview_presentation_plan` → `resolved_slides[].expanded_compose`). |

### `next_tool_call` Envelope

Findings with action `refuse`, `shrink_or_split`, or `review` include a `next_tool_call` object that tells agents exactly which MCP tool to call next, without requiring them to infer the protocol from prose. The envelope has two fields:

| Field | Type | Description |
|-------|------|-------------|
| `tool` | string | MCP tool name, e.g. `"repair_slide"` or `"recommend_pattern"` |
| `args_template` | object | Template for the tool arguments — agents can invoke directly or merge with additional context |

Routing logic:
- Fix kinds in the `repair_slide` vocabulary (`reduce_text`, `split_at_row`, `shorten_title`, `replace_color`, `use_semantic_color`, `split_pattern`, `swap_layout`, `use_one_of`) → `next_tool_call.tool = "repair_slide"`
- `swap_pattern` and `adopt_pattern` fix kinds → `next_tool_call.tool = "recommend_pattern"`
- Findings with action `"info"` never have `next_tool_call`

Example:

```json
{
  "code": "placeholder_overflow",
  "path": "/slides/0/content/body",
  "fix": { "kind": "reduce_text" },
  "action": "shrink_or_split",
  "next_tool_call": {
    "tool": "repair_slide",
    "args_template": { "slide_index": 0, "fixes": [{ "kind": "reduce_text" }] }
  }
}
```

## Actions

Actions indicate severity and recommended remediation. They are ranked from most to least severe:

| Rank | Action | Meaning |
|------|--------|---------|
| 3 | `refuse` | Content cannot be rendered correctly. In `strict` mode, generation is blocked. |
| 2 | `shrink_or_split` | Content overflows significantly. Agent should reduce text, split the slide, or restructure. |
| 1 | `review` | Content may not render ideally. Human or agent review recommended but not blocking. |
| 0 | `info` | Informational signal. No action required. |

The `ActionRank(action)` function returns these numeric ranks. Unknown actions return -1.

### Sort invariant

Every `fit_report` / `findings` array crosses serialization boundaries in the canonical order

  `(action_rank desc, slide_index asc, code asc)`

so `findings[0]` is always the most important fix and the order is deterministic across runs and tools. Implemented by `patterns.SortCanonical`. The invariant is asserted at every gate that emits findings (validate / generate / preview / score / repair) — see `cmd/json2pptx/mcp_response_fingerprint_test.go` for the cross-tool test.

Deck-level findings whose path does not match `/slides/N/...` (slide index extracts to `-1`) sort before slide 0 at equal severity.

## Finding Codes

### `placeholder_overflow`

**Action:** `shrink_or_split`
**Pattern:** `placeholder`
**Fix kind:** `reduce_text`

Text in a body or content placeholder overflows its frame. Emitted only when all three conditions hold simultaneously:

1. **Significant overshoot** — measured height exceeds frame height by >15% (the `overflowThreshold` constant filters measurement noise).
2. **No autofit** — the placeholder's OOXML autofit mode is `noAutofit` or absent. When `normAutofit` or `spAutoFit` is active, PowerPoint auto-shrinks text, so the finding is suppressed.
3. **Unfixable at min scale** — even at the minimum autofit font scale, `textfit.Calculate` still reports overflow. If hypothetically adding normAutofit would fix it, the finding is not emitted.

```json
{
  "pattern": "placeholder",
  "path": "/slides/0/content/body",
  "code": "placeholder_overflow",
  "message": "text overflows placeholder by 42% (360pt frame, autofit=none); overflow persists at minimum font scale",
  "fix": { "kind": "reduce_text" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 7772400, "height_emu": 6515100 },
  "allowed": { "width_emu": 7772400, "height_emu": 4572000 },
  "overflow_ratio": 1.42
}
```

### `title_wraps`

**Action:** `review`
**Pattern:** `placeholder`
**Fix kind:** `shorten_title`

Title text wraps to multiple lines within its placeholder. This is common and often acceptable, so the action is `review` rather than `shrink_or_split`. Emitted when the measured text height exceeds a single-line height (computed as `fontSize * 1.2 line spacing`).

```json
{
  "pattern": "placeholder",
  "path": "/slides/1/content/title",
  "code": "title_wraps",
  "message": "title wraps to multiple lines (36pt font, 9.1\" wide placeholder)",
  "fix": { "kind": "shorten_title" },
  "action": "review",
  "measured": { "width_emu": 8229600, "height_emu": 731520 },
  "allowed": { "width_emu": 8229600, "height_emu": 548640 },
  "overflow_ratio": 1.33
}
```

### `slide_bounds_overflow`

**Action:** `shrink_or_split`
**Pattern:** `shape_grid`
**Fix kind:** `reposition_shape`

A JSON-authored shape's center falls outside the slide rectangle. Uses center-based threshold (not corner-based) to avoid false positives from 1-EMU rounding. Only checks shapes authored in JSON input — layout-inherited shapes are excluded.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/2/shape_grid/rows/1/cells/0",
  "code": "slide_bounds_overflow",
  "message": "shape center (10058400, 7315200) EMU falls outside slide bounds (9144000 x 6858000) vertically",
  "fix": { "kind": "reposition_shape" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 4572000, "height_emu": 3429000 },
  "allowed": { "width_emu": 9144000, "height_emu": 6858000 }
}
```

### `footer_collision`

**Action:** `review` (default) or `refuse` (strict mode)
**Pattern:** `shape_grid`
**Fix kind:** `reposition_shape`

A JSON-authored shape intrudes into the footer reserved area. The action depends on the `strict_fit` setting: `"strict"` produces `refuse`, `"warn"` produces `review`, `"off"` suppresses entirely.

Only fires when the slide's resolved layout declares a footer placeholder (date, footer text, or slide number). This prevents false positives on layouts that use heuristic fallback positioning.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/3/shape_grid/rows/2/cells/0",
  "code": "footer_collision",
  "message": "shape bottom edge (6400000 EMU) intrudes 142000 EMU into footer area (top=6258000 EMU)",
  "fix": { "kind": "reposition_shape" },
  "action": "review",
  "measured": { "width_emu": 4572000, "height_emu": 3429000 },
  "allowed": { "width_emu": 4572000, "height_emu": 2829000 }
}
```

### `title_collision`

**Action:** `review` (default) or `refuse` (strict mode)
**Pattern:** `shape_grid`
**Fix kind:** `reposition_shape`

A JSON-authored shape's top edge starts above the resolved content zone's title bottom edge — the grid intrudes upward into the title chrome. This is the title-side mirror of `footer_collision`. The action depends on the `strict_fit` setting: `"strict"` produces `refuse`, `"warn"` produces `review`, `"off"` suppresses entirely.

Preflight resolves shape_grid geometry through the **same** layout-aware helper generation uses (`resolveGridGeometry` → `resolveGridBounds`), so the cell coordinates evaluated here match what renders. This is what lets preflight catch the title-overlap class — most commonly a "title at bottom, body/content placeholder above it" layout (roles flipped relative to size) whose virtual-layout fallback anchors the grid above the title — instead of only surfacing it after LibreOffice rendering.

Only fires when a title-anchored content zone was resolved for the slide (`LayoutDeclaresTitle`). Slides whose zone is a generic fallback with no title anchor are skipped.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/0/shape_grid/rows/0/cells/0",
  "code": "title_collision",
  "message": "shape top edge (400000 EMU) intrudes 880160 EMU into title area (bottom=1280160 EMU)",
  "fix": { "kind": "reposition_shape" },
  "action": "review",
  "measured": { "width_emu": 4000000, "height_emu": 2000000 },
  "allowed": { "width_emu": 4000000, "height_emu": 1119840 }
}
```

### `sparse_layout`

**Action:** `review`
**Pattern:** `shape_grid`
**Fix kind:** `grow_pattern`

Content occupies less than 40% of the available bounds height — the slide is mostly empty. Since grid bounds are authoritative (never shrink), this fires when the estimated content extent is under 40% of the allocated bounds height.

The fix params include `filled_pct`, `bounds_height`, and `content_height`.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/1/shape_grid",
  "code": "sparse_layout",
  "message": "content occupies 25% of bounds height (1270000 / 5080000 EMU) — slide is mostly empty",
  "fix": { "kind": "grow_pattern", "params": { "filled_pct": 0.25, "bounds_height": 5080000, "content_height": 1270000 } },
  "action": "review",
  "measured": { "height_emu": 1270000 },
  "allowed": { "height_emu": 5080000 },
  "overflow_ratio": 0.25
}
```

### `pattern_underfilled`

**Action:** `review`
**Pattern:** pattern name (e.g. `kpi-3up`, `card-grid`)
**Fix kind:** `swap_pattern`

A pattern grid has less than 50% of its slots populated — the content is too sparse for the chosen pattern. The fix suggests using `recommend_pattern` to find a better-fitting pattern for the item count.

```json
{
  "pattern": "kpi-3up",
  "path": "/slides/2/shape_grid",
  "code": "pattern_underfilled",
  "message": "kpi-3up: 1 of 3 slots filled (33%) — grid is underpopulated",
  "fix": { "kind": "swap_pattern", "params": { "filled_pct": 0.33, "filled_slots": 1, "total_slots": 3, "reason": "reshape_grid" } },
  "action": "review",
  "overflow_ratio": 0.33,
  "next_tool_call": { "tool": "recommend_pattern", "args_template": { "item_count": 1 } }
}
```

### `pattern_overcrowded`

**Action:** `review`
**Pattern:** pattern name (e.g. `card-grid`, `kpi-4up`)
**Fix kind:** `split_pattern`

A pattern grid exceeds the pattern's recommended maximum cell count. The fix suggests splitting across two slides using `split_pattern`, with params indicating the recommended split point.

```json
{
  "pattern": "card-grid",
  "path": "/slides/3/shape_grid",
  "code": "pattern_overcrowded",
  "message": "card-grid: 12 cells exceeds recommended max of 8 — consider splitting",
  "fix": { "kind": "split_pattern", "params": { "filled_slots": 12, "recommended_max": 8, "first": 6, "second": 6, "title_part_2": "(continued)" } },
  "action": "review",
  "overflow_ratio": 1.5,
  "next_tool_call": { "tool": "repair_slide", "args_template": { "slide_index": 3, "fixes": [{ "kind": "split_pattern", "params": { "first": 6, "title_part_2": "(continued)" } }] } }
}
```

### `grid_diagram_narrow`

**Action:** `review`
**Fix kind:** `reshape_grid`

A complex diagram (org_chart, fishbone, swot, heatmap, etc.) is placed in a grid cell whose width is less than 50% of the slide width. At this width, dense diagram labels and structural elements become illegible. The finding is emitted at generation time (not pre-flight) because it requires resolved cell bounds.

```json
{
  "path": "/slides/0/shape_grid/rows/0/cells/1/diagram",
  "code": "grid_diagram_narrow",
  "message": "complex org_chart diagram (8 items) in narrow grid cell (width 33% of slide) may be illegible — consider a wider cell or full-width layout",
  "fix": { "kind": "reshape_grid", "params": { "diagram_type": "org_chart", "complexity": 8, "cell_width_pct": 33.3, "cell_width_emu": 4064000, "threshold_emu": 6096000 } },
  "action": "review",
  "measured": { "width_emu": 4064000, "height_emu": 0 },
  "allowed": { "width_emu": 6096000, "height_emu": 0 },
  "overflow_ratio": 1.5
}
```

### `diagram_aspect_mismatch`

**Action:** `review`
**Fix kind:** `reshape_grid`
**Emitted at:** preflight + render time

A diagram with **both** explicit `DiagramSpec.Width` and `Height` has a render-frame aspect ratio that differs from those pinned (authored) dimensions by more than 25%. Because the explicit dimensions fix the rendered SVG aspect regardless of the frame, the chart is either stretched or letterboxed, both of which read as visual noise. The agent action is to widen/shorten the cell, set `cell.fit` to `contain` / `fit-width` / `fit-height`, or change the explicit `diagram.width` / `diagram.height` to match the frame.

This finding does **not** fire for diagrams with unset or single-axis (`width`-only / `height`-only) dimensions: the renderer resolves the missing dimension(s) from the render frame (via `ResolveDiagramRenderDimensions`), so the rendered SVG adopts the frame aspect and there is nothing to flag. Natural-aspect diagram types (`timeline`, `gantt`, `org_chart`) that ignore unset dimensions are covered by [`diagram_aspect_conflict`](#diagram_aspect_conflict) instead.

**Authored vs effective evidence.** The flagged deviation is the **authored** aspect vs the **post-fit render frame** (the frame the SVG is actually sized into — what render emits). The fix params carry four independent aspect signals so an agent can tell apart an authoring mistake (authored dims fight the cell) from a fit-driven render mismatch:

- `authored_width` / `authored_height` / `authored_aspect` — the explicit dimensions the spec pinned.
- `effective_width` / `effective_height` / `effective_aspect` + `dimension_source` — the dimensions the resolver produced for the frame. For an explicit spec these equal the authored dims and `dimension_source` is `"explicit"`.
- `cell_width_emu` / `cell_height_emu` / `cell_aspect` — the **original (pre-fit)** cell allocation.
- `render_width_emu` / `render_height_emu` / `render_aspect` — the **post-fit** frame; `fit_adjusted` is `true` when a `cell.fit` reshaped the cell into a different frame.

When `fit_adjusted` is `true`, the message additionally reports the original cell aspect so the fit's effect is visible.

```json
{
  "path": "/slides/0/shape_grid/rows/0/cells/0/diagram",
  "code": "diagram_aspect_mismatch",
  "message": "bar_chart authored aspect 1.33 (explicit 800×600) differs from the rendered cell aspect 0.50 by 62% — chart will be stretched or letterboxed; resize the cell, set cell.fit, or change diagram.width/height",
  "fix": { "kind": "reshape_grid", "params": { "diagram_type": "bar_chart", "authored_width": 800, "authored_height": 600, "authored_aspect": 1.333, "effective_width": 800, "effective_height": 600, "effective_aspect": 1.333, "dimension_source": "explicit", "cell_width_emu": 3000000, "cell_height_emu": 6000000, "cell_aspect": 0.5, "render_width_emu": 3000000, "render_height_emu": 6000000, "render_aspect": 0.5, "fit_adjusted": false, "deviation": 0.625 } },
  "action": "review",
  "measured": { "width_emu": 3000000, "height_emu": 6000000 },
  "allowed": { "width_emu": 800, "height_emu": 600 },
  "overflow_ratio": 0.375
}
```

> The example above assumes the `bar_chart` carries explicit `diagram.width: 800` / `diagram.height: 600` and no `cell.fit` (so `cell_*` and `render_*` coincide). With a `cell.fit`, `render_*` reflects the post-fit frame and `fit_adjusted` is `true`. `measured` is the post-fit render frame; `allowed` is the effective render dimensions.

### `diagram_aspect_conflict`

**Action:** `review`
**Fix kind:** `reshape_grid`
**Emitted at:** preflight + render time

A non-chart diagram cell's aspect ratio differs from the diagram type's natural svggen viewBox aspect by more than 30%. Currently emitted for diagram types whose renderer pins a non-container natural aspect via `svggen.NaturalAspect` — `timeline` (2:1), `gantt` (~1.8:1), and `org_chart` (~1.57:1, baseline before data-driven scaling). The check is silent for chart types (their aspect issues come from svggen dry-render `chart.*` findings) and for diagrams with explicit `DiagramSpec.Width`/`Height` (those are handled by `diagram_aspect_mismatch`). Available at validate and preview time without invoking resvg/inkscape.

```json
{
  "path": "/slides/0/shape_grid/rows/0/cells/0/diagram",
  "code": "diagram_aspect_conflict",
  "message": "timeline cell aspect 0.52 conflicts with diagram natural aspect 2.00 (deviation 74%) — render will be letterboxed or distorted; resize the cell, set cell.fit, or set explicit diagram.width/height",
  "fix": { "kind": "reshape_grid", "params": { "diagram_type": "timeline", "natural_aspect": 2.0, "cell_aspect": 0.52, "deviation": 0.74, "cell_width_emu": 3048000, "cell_height_emu": 5829300 } },
  "action": "review",
  "measured": { "width_emu": 3048000, "height_emu": 5829300 },
  "overflow_ratio": 0.26
}
```

### `diagram_clamped`

**Action:** `review`
**Fix kind:** `swap_layout`
**Emitted at:** render time

A diagram placeholder's width or height was below the engine's minimum threshold and was clamped up. The diagram renders but may look different than expected because the original dimensions were too small. The deterministic agent action is to switch to a wider layout via `repair_slide`.

```json
{
  "path": "/slides/1/content/body",
  "code": "diagram_clamped",
  "message": "diagram placeholder width clamped: 2000000 EMU → 3048000 EMU minimum",
  "fix": { "kind": "swap_layout", "params": { "dimension": "width", "original_emu": 2000000, "clamped_emu": 3048000 } },
  "action": "review",
  "next_tool_call": { "tool": "repair_slide", "args_template": { "slide_index": 1, "fixes": [{ "kind": "swap_layout", "params": { "dimension": "width", "original_emu": 2000000, "clamped_emu": 3048000 } }] } }
}
```

### `diagram_render_failed`

**Action:** `review`
**Fix kind:** `review` (no auto-fix)
**Emitted at:** render time

Diagram rendering failed entirely; a placeholder image was inserted instead. This is review-only — no deterministic auto-fix is available. The agent must inspect the diagram data and decide whether to simplify the diagram, change its type, or regenerate the slide.

```json
{
  "path": "/slides/2/content/body",
  "code": "diagram_render_failed",
  "message": "diagram render failed, placeholder image inserted: SVG parse error",
  "fix": { "kind": "review", "params": { "diagram_type": "org_chart", "reason": "SVG parse error" } },
  "action": "review"
}
```

### `chart_value_coerced`

**Action:** `review`
**Pattern:** chart type (e.g. `bar`, `line`)
**Fix kind:** `provide_numeric_value`

A non-numeric value in the chart data map was coerced to zero. The finding indicates a likely data error — the original value and its type are included in the fix params.

```json
{
  "pattern": "bar",
  "path": "/slides/0/content/1",
  "code": "chart_value_coerced",
  "message": "slide 1, content 2: non-numeric value for \"Revenue\" coerced to 0 (was string: N/A)",
  "fix": { "kind": "provide_numeric_value", "params": { "column": "Revenue", "original_value": "N/A", "original_type": "string" } },
  "action": "review"
}
```

### `chart_shape_inferred`

**Action:** `review`
**Pattern:** chart type (e.g. `gauge`, `radar`)
**Fix kind:** `provide_native_format`

The chart received flat key-value data but expects a structured format (e.g. `series` array for multi-series charts, or nested objects for gauge). The engine inferred the structure, but the result may not be what was intended.

```json
{
  "pattern": "gauge",
  "path": "/slides/1/content/0",
  "code": "chart_shape_inferred",
  "message": "slide 2, content 1: gauge chart received flat data; expected gauge format",
  "fix": { "kind": "provide_native_format", "params": { "chart_type": "gauge", "expected_format": "gauge" } },
  "action": "review"
}
```

### `chart_data_empty`

**Action:** `refuse`
**Pattern:** chart type
**Fix kind:** `provide_data`

The chart's data map is empty — the output would be a blank chart placeholder. This is a `refuse`-level finding because a blank chart is never the intended result.

```json
{
  "pattern": "bar",
  "path": "/slides/0/content/0",
  "code": "chart_data_empty",
  "message": "slide 1, content 1: bar data is empty; output will be blank",
  "fix": { "kind": "provide_data", "params": { "chart_type": "bar" } },
  "action": "refuse"
}
```

### `CHART_PLACEHOLDER_EMPTY`

**Action:** `review`
**Pattern:** `chart-insights-split`
**Fix kind:** — (no auto-fix; agent must supply a chart or swap patterns)

The `chart-insights-split` pattern was expanded without a `chart` spec, so the left chart panel collapses and the insights column renders full-width. Emitted from the pattern's `PostExpandWarnings` hook so it appears in `preview_presentation_plan` and `generate_presentation` fit reports. Use this finding to either (a) supply a chart, or (b) switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`).

```json
{
  "pattern": "chart-insights-split",
  "path": "/slides/2/pattern",
  "code": "CHART_PLACEHOLDER_EMPTY",
  "message": "slide 3: chart-insights-split: chart-insights-split rendered insights-only; provide a chart spec to fill the left panel",
  "action": "review"
}
```

### `fit_overflow`

**Action:** `shrink_or_split` (mapped from internal `unfittable`)
**Pattern:** `table`
**Fix kind:** `reduce_text` (headers) or `split_at_row` (data cells)

Text in a table cell exceeds the available cell height. The `split_at_row` fix includes a `row` parameter suggesting where to split the table.

```json
{
  "pattern": "table",
  "path": "/slides/0/content/0/rows/3/1",
  "code": "fit_overflow",
  "message": "text needs 4 lines @ 12pt; cell allows 2",
  "fix": { "kind": "split_at_row", "params": { "row": 4 } },
  "action": "shrink_or_split",
  "measured": { "height_emu": 609600 },
  "allowed": { "height_emu": 370840 },
  "overflow_ratio": 1.64
}
```

### `density_exceeded`

**Action:** `shrink_or_split` (mapped from internal `unfittable`)
**Pattern:** `table`
**Fix kind:** `split_at_row`

Table has more cells than the TDR (Table Density Ratio) ceiling allows for the computed font size. The ceiling varies by font size: 60 cells at 18pt, 80 at 14pt, 100 at 12pt, 120 at 10pt.

```json
{
  "pattern": "table",
  "path": "/slides/0/content/0",
  "code": "density_exceeded",
  "message": "table has 72 cells (8 rows × 9 cols) at 12pt; TDR ceiling is 60",
  "fix": { "kind": "split_at_row", "params": { "row": 4 } },
  "action": "shrink_or_split"
}
```

### `cell_underfilled`

**Action:** `review`
**Pattern:** `shape_grid`
**Fix kind:** `add_detail_or_resize`

A shape-grid cell's text content uses less than 60% of its character capacity. Capacity is computed by the `internal/textcapacity` package, which estimates `MaxChars` from the cell's height, width, and font size.

Two severity bands:

| Density | Severity | Guidance |
|---------|----------|----------|
| 40–59% | `info` | Consider adding detail; not blocking |
| <40% | `warning` | Strongly consider adding detail or using a smaller grid |

The 60–110% range is the healthy zone — no finding is emitted. Above 110%, see `fit_overflow`.

`strict_fit` interaction: `cell_underfilled` never blocks generation. Its maximum severity is `warning` and its action is `review`, which is never promoted to `refuse` regardless of `strict_fit` mode.

See also: [PATTERNS.md Cell Capacity Contract](PATTERNS.md) for pattern-level capacity guidance.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/1/shape_grid/rows/0/cells/0/shape/text",
  "code": "cell_underfilled",
  "severity": "warning",
  "message": "cell content is 12 chars (28% of capacity) — consider adding detail or smaller grid",
  "fix": { "kind": "add_detail_or_resize", "params": { "current_density_pct": 28 } },
  "action": "review"
}
```

### `accent_overload`

**Action:** `review`
**Pattern:** `shape_grid`
**Fix kind:** `consolidate_accents`

Emitted by `DetectStructuralSmells` (via `validate_input`, dry-run, and the pipeline's per-slide checks) when a single slide's `shape_grid` uses more than two distinct accent semantic fills (`accent1` … `accent6`). The cap is two so that a slide can still draw a paired comparison (current vs. proposed, before vs. after) without losing focus, but three or more accents on one slide reads as visual noise — the audience cannot tell which item is the argument.

Mechanics:

- Only semantic accent names count. Hex fills are ignored here (the hex/scheme mixing check is a separate finding). Neutrals like `lt1`, `dk1`, `lt2` are not accents and do not count.
- Object-form fills with tint/shade modifiers (`{"color": "accent1", "lumMod": 75000}`) count as the same hue as the bare scheme name — three `accent1` tints are still one accent.
- The fix suggestion's `params.accents_used` lists the distinct accents found, and the recommended remedy is to pick one base accent and use `cell_accent_mode` (`alternate` or `progressive`) when a grid genuinely needs item-level differentiation.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/2/shape_grid",
  "code": "accent_overload",
  "severity": "warning",
  "message": "slide 3: shape_grid uses 4 distinct accent hues (accent1, accent2, accent3, accent4); max 2 — pick one base accent and use cell_accent_mode for within-slide variety",
  "fix": {
    "kind": "consolidate_accents",
    "params": {
      "accents_used": ["accent1","accent2","accent3","accent4"],
      "max_accents": 2,
      "guidance": "keep at most two accent hues per slide; use cell_accent_mode (alternate/progressive) for grids that need item differentiation"
    }
  },
  "action": "review"
}
```

### `takeaway_missing`

**Action:** `review`
**Pattern:** *(none — slide-level)*
**Fix kind:** `provide_value`

Emitted by `validate_input` and the CLI dry-run when a slide carries chart or matrix content but the slide's `takeaway` field is empty. The takeaway is the slide's headline answer — a single sentence that tells the audience the "so what" of the data. A chart or 2x2 without one forces the audience to derive the argument themselves, which they rarely do correctly.

Triggers when **all** of the following hold:

- `slide.takeaway` is empty (or whitespace-only)
- The slide has at least one of: a `chart` content item, a `diagram` content item whose `diagram_value.type` is chart-shaped (bar, line, area, scatter, bubble, pie, donut, stacked_bar, grouped_bar, waterfall, funnel, radar, gauge, treemap), or a pattern whose `name` starts with `matrix-`

The warning never blocks generation — the takeaway is advisory, not structural. Add a one-sentence `takeaway` to the slide; it renders as bold dark-gray text in the lower band of the slide, above the source note row.

```json
{
  "path": "/slides/3/takeaway",
  "code": "takeaway_missing",
  "severity": "warning",
  "message": "slide 4: chart/matrix slides should set a takeaway headline so the audience knows the 'so what' — currently empty",
  "fix": { "kind": "provide_value", "params": { "field": "takeaway" } },
  "action": "review"
}
```

### `HEADLINE_TOO_LONG`

**Action:** `review`
**Pattern:** *(none — content lint)*
**Fix kind:** `shorten_title`

Emitted when a title-class placeholder (`title`, `headline`, `ctrTitle`) carries more than 12 whitespace-separated words. Single-line headlines at 36–40pt fit roughly 12 words across a 16:9 slide; longer ones wrap or shrink. Advisory only — never blocks render.

Mechanics:

- Word counting splits on whitespace via `strings.Fields`, so punctuation attached to a word does not inflate the count.
- The check runs on `text` content items whose `placeholder_id` matches a title slot; non-title text is checked against `BODY_TOO_LONG` instead.
- `fix.params` carry `current_words` and `max_words` so agents can decide between hand-trim and `repair_slide(kind=shorten_title)`.

```json
{
  "path": "/slides/0/content/title",
  "code": "HEADLINE_TOO_LONG",
  "message": "slide 1: headline is 20 words; trim to 12 or fewer for readability",
  "fix": { "kind": "shorten_title", "params": { "current_words": 20, "max_words": 12 } },
  "action": "review"
}
```

### `BODY_TOO_LONG`

**Action:** `review`
**Pattern:** *(none — content lint)*
**Fix kind:** `reduce_text`

Emitted when a single text block exceeds 80 whitespace-separated words. Applies to `text`, `bullets`, `body_and_bullets`, and `bullet_groups` content items — bullet items are aggregated per content block, so a 10-bullet list of 10-word bullets trips the budget. 80 words is roughly five tight 12pt lines, which is the upper bound an audience reads while still listening.

```json
{
  "path": "/slides/2/content/body",
  "code": "BODY_TOO_LONG",
  "message": "slide 3: body text is 120 words; trim to 80 or fewer (audiences read at most 5 lines per slide)",
  "fix": { "kind": "reduce_text", "params": { "current_words": 120, "max_words": 80 } },
  "action": "review"
}
```

### `BULLET_NESTING_DEEP`

**Action:** `review`
**Pattern:** *(none — content lint)*
**Fix kind:** `reduce_text`

Emitted when a bullet list nests more than two levels deep. Depth is measured per bullet from leading whitespace: each tab counts as one indent unit; every two leading spaces count as one indent unit. The base level is 1 for `bullets` / `body_and_bullets` content items and 2 for bullets inside a `bullet_groups` header (header is level 1, bullets render at level 2). Depth 3 or deeper triggers the finding.

```json
{
  "path": "/slides/4/content/body",
  "code": "BULLET_NESTING_DEEP",
  "message": "slide 5: bullets nest 3 levels; flatten to 2 or fewer — deep nesting reads as visual noise",
  "fix": { "kind": "reduce_text", "params": { "current_depth": 3, "max_depth": 2 } },
  "action": "review"
}
```

### `MISSING_ALT_TEXT`

**Action:** `review`
**Pattern:** *(none — accessibility lint)*
**Fix kind:** `provide_value`

Emitted when an image or icon asset is sourced from `path`, `url`, or `svg_data` but its `alt` field is empty (or whitespace-only). Bundled built-in icons referenced by `IconInput.name` are exempt — the qualified bundled name itself supplies an implicit caption (`preview_icon` returns it via the `alt` field).

The lint walks four surfaces:

- `slide.content[].image_value` (`ImageInput.Path` / `ImageInput.URL`)
- `slide.shape_grid.rows[].cells[].image` (`GridImageInput.Path` / `GridImageInput.URL`)
- `slide.shape_grid.rows[].cells[].icon` (cell-level `IconInput`)
- `slide.shape_grid.rows[].cells[].shape.icon` (shape-overlay `IconInput`)

The finding never blocks render — it appears as a `review` action so agents that optimize only for `passes validation` cannot ship visually-fine but accessibility-incomplete decks. Because the action is `review`, `score_deck` deducts 5 points per occurrence from the affected slide's score (and from the overall correctness axis), creating scoring pressure to set alt text.

`fix.params` carry the asset `kind` (`image_value`, `image`, or `icon`) and the `source` field (`path`, `url`, or `svg_data`) so an agent can route the fix to the right authoring surface.

```json
{
  "path": "/slides/0/content/0/image_value",
  "code": "MISSING_ALT_TEXT",
  "message": "slide 1: image_value sourced from path is missing alt text — set alt for screen-reader accessibility",
  "fix": {
    "kind": "provide_value",
    "params": { "field": "alt", "kind": "image_value", "source": "path" }
  },
  "action": "review"
}
```

### `DUPLICATE_TITLE`

**Action:** `review`
**Pattern:** *(none — deck-level content lint)*
**Fix kind:** `shorten_title`

Emitted when two or more content slides share the same title text after case-folding and collapsing whitespace runs. The earliest occurrence is treated as canonical and is not annotated — every later slide in the duplicate group carries the finding so agents can target the renaming work without re-touching the original.

Slide selection:

- Title slides (cover, "Thank You", "Q&A") and section dividers are exempt — these slide types legitimately repeat phrasing across decks and sections.
- A slide is treated as a content slide when (a) its `slide_type` is set to anything other than `title` / `section`, or (b) it carries a `shape_grid`, `pattern`, or `compose` block, or (c) `inferSlideType` does not classify it as `title` / `section`.
- An empty or whitespace-only title is ignored; the finding requires non-empty text.

`fix.params` carry `duplicate_of_slide` (1-based slide number of the earliest occurrence), `duplicate_slide_numbers` (sorted 1-based slide numbers of every slide in the group), `duplicate_count` (group size), and `placeholder_id` (the title placeholder the duplicate text lives on) so the agent can quickly compose a rename without re-walking the deck.

```json
{
  "path": "/slides/4/content/title",
  "code": "DUPLICATE_TITLE",
  "message": "slide 5: title duplicates slide 3 (3 slides share this title: 3, 5, 8); rename so each headline announces a distinct point",
  "fix": {
    "kind": "shorten_title",
    "params": {
      "duplicate_of_slide": 3,
      "duplicate_slide_numbers": [3, 5, 8],
      "duplicate_count": 3,
      "placeholder_id": "title"
    }
  },
  "action": "review"
}
```

### `CONTENT_DROPPED`

**Action:** `review`
**Pattern:** *(none — shared content-drop diagnostic)*
**Fix kind:** `review` (no deterministic auto-fix)

The single, shared signal for **any** path that fails to place author-provided content. Today several paths can drop content — a slide skipped in `--partial` mode, a content block with no available placeholder, a column that did not fit, an unknown payload field stripped during semantic parsing. Each used to drop silently (or only as a free-text warning); `CONTENT_DROPPED` turns every such drop into one consistent, machine-actionable finding so an agent sees the loss and can repair it.

The drop has *already happened* by the time the finding is emitted — it is advisory and never blocks generation (action `review`). The fix carries `params.locator` (a short label for what was dropped — `"slide 4"`, `"content block 3"`, `"left column"`) and `params.reason` (why placement failed) so an agent can route the repair without re-deriving the cause. There is no single deterministic auto-fix (fix kind `review`, mirroring `diagram_render_failed`): the agent restructures or splits the slide, or fixes the underlying spec error.

Emitted today from the partial-mode slide-skip path in slide conversion, and from the **multi-visual collision** path in image preparation — when two or more visual content blocks (chart / table / image / diagram) resolve to the *same* placeholder, only the first is rendered and each subsequent one is dropped (rather than silently overlapping the first at identical bounds), each with its own `CONTENT_DROPPED` finding pointing at `/slides/{i}/content/{n}` and a `reason` suggesting the author split the slide or use `compose` to give each visual its own region. Other drop paths (dense-pattern section-divider overflow) adopt the same `patterns.ContentDropped(path, locator, reason)` constructor as they are hardened.

```json
{
  "path": "/slides/3",
  "code": "CONTENT_DROPPED",
  "message": "author-provided content dropped (slide 4): skipped in partial mode: slide 4: layout_id is required (no template layouts available for auto-selection)",
  "fix": {
    "kind": "review",
    "params": {
      "locator": "slide 4",
      "reason": "skipped in partial mode: slide 4: layout_id is required (no template layouts available for auto-selection)"
    }
  },
  "action": "review"
}
```

### `CUSTOM_COLOR_DROPPED`

**Action:** `info`
**Pattern:** `design_mode`
**Fix kind:** `set_design_mode_free`

Advisory signal that a diagram's **data payload** embeds raw hex colors in per-item fields (e.g. `pyramid` `levels[].color`) which the engine ignores in **constrained** design mode (the default) — the diagram renders with the template scheme instead. Unlike the documented `diagram_value.style.colors` / `style.background` surface (which constrained mode *refuses* as a `design_mode_violation`, blocking generation), these data-embedded colors are not part of the validated override surface, so they were previously dropped **silently**. This finding turns that drop into a visible, machine-actionable signal.

The drop is intended behavior in constrained mode (brand consistency), so the finding never blocks generation (action `info`). The fix carries `params.dropped_colors` (the raw hex values that were ignored) and `params.path` (the diagram data path). On the MCP boundary the equivalent diagnostic is emitted at `warning` severity with a `next_tool_call` suggesting `generate_presentation` with `design_mode: "free"` — rerun in free mode to honor the custom colors. Scheme-color names (`accent1`, `dk1`, …) inside the same data payload are allowed and never reported.

```json
{
  "path": "/slides/2/content/0/diagram_value/data",
  "code": "CUSTOM_COLOR_DROPPED",
  "message": "slide 3: pyramid data embeds custom color(s) #FF0000, #00FF00 — constrained mode (default) renders with the template scheme and ignores them; rerun with design_mode \"free\" to honor custom colors",
  "fix": {
    "kind": "set_design_mode_free",
    "params": {
      "path": "/slides/2/content/0/diagram_value/data",
      "dropped_colors": ["#FF0000", "#00FF00"]
    }
  },
  "action": "info"
}
```

### `contrast_predicted`

**Action:** `info`
**Pattern:** *(none — preflight)*
**Fix kind:** `replace_color`

Preflight prediction that the renderer's contrast auto-fix pass would replace a text color to meet WCAG AA Large (3:1) contrast against its background. Emitted by `validate` and `preview_presentation_plan` (and downstream tools like `repair_slide` / `score_candidates`) using only theme colors and JSON content — no rendering. The predictor calls the **same replacement algorithm** the renderer uses (`contrastReplacement`), so the predicted color matches the `contrast_autofixed` swap that fires when the same input is generated.

The detector walks shape-grid cells that author both a fill color (on the shape) and a text color (on `shape.text` or on a paragraph in `shape.text.paragraphs`). Scheme names (`accent1`, `lt1`, …) are resolved against the template theme. Pairs that cannot be parsed are skipped.

`fix.params.replacement_mode` discloses which branch of the algorithm produced the color:

- `flip` — the foreground is a pure neutral (literal white/black, or scheme `lt1`/`bg1`/`dk1`/`tx1`); it is snapped to the opposite theme extreme (`dk1`/`lt1`). This is why white text on a light accent predicts a clean dark color rather than a muddy mid-gray.
- `lerp` — any other foreground; it is darkened or lightened toward black/white via `EnsureContrast` until it clears 3:1.

```json
{
  "path": "/slides/1/shape_grid/rows/0/cells/0/shape/text",
  "code": "contrast_predicted",
  "message": "predicted: low-contrast text will be auto-replaced — #FFFFFF → #1A1A1A (on #FFE8D4, ratio 1.3 → 14.7)",
  "fix": {
    "kind": "replace_color",
    "params": {
      "original_color": "#FFFFFF",
      "predicted_replacement": "#1A1A1A",
      "background_color": "#FFE8D4",
      "contrast_ratio_before": 1.3,
      "contrast_ratio_after": 14.7,
      "replacement_mode": "flip",
      "source": "shape_grid"
    }
  },
  "action": "info"
}
```

### `contrast_autofixed`

**Action:** `info`
**Fix kind:** `replace_color`

Text color was automatically replaced to meet WCAG AA contrast requirements against the resolved background. This is informational — the fix has already been applied. The `fix.params` include the original and replacement colors, the background color, the contrast ratios before and after the swap, and the text surface `source`.

The finding's `path` locates the swap so an agent can map it back to the offending element. For `shape_grid` cell swaps the path is the flat rendered-shape index `"/slides/{i}/shape_grid/shapes/{n}"` (the original grid row/cell coordinates are not retained on the raw shape XML at render time). For template layout text (the `lstStyle`/`run` sources) the path is the slide-level `"/slides/{i}"`. The owning slide index is derived from `path` like every other finding. `fix.params.source` names the surface: `shape_grid`, `lstStyle`, or `run`.

```json
{
  "path": "/slides/3/shape_grid/shapes/2",
  "code": "contrast_autofixed",
  "message": "auto-fixed low-contrast text: #FFFFFF → #1A1A1A (on #E8A838, ratio 1.6 → 14.7)",
  "fix": {
    "kind": "replace_color",
    "params": {
      "original_color": "#FFFFFF",
      "replacement_color": "#1A1A1A",
      "background_color": "#E8A838",
      "contrast_ratio_before": 1.6,
      "contrast_ratio_after": 14.7,
      "source": "shape_grid"
    }
  },
  "action": "info"
}
```

### `placeholder_remapped`

**Action:** `info`
**Pattern:** `placeholder`
**Fix kind:** `remap_placeholder`

A content placeholder ID from the input was resolved to a different placeholder on the layout. This happens when the chosen layout does not declare the requested placeholder (e.g. a `section` layout has no `subtitle` placeholder, so `subtitle` content is remapped onto `body`). The remapping is non-destructive — the content still renders — but agents should consider authoring the resolved ID directly to avoid the implicit rewrite.

Emitted both pre-flight (from `preview_presentation_plan` / `generate_presentation` with `fit_report: true`) and at render time. The pre-flight emission walks every placeholder whose `resolved_id` differs from its `input_id`; the render-time emission fires when the generator's semantic-tier fallback resolves a virtual placeholder ID (today: `subtitle` → body-class placeholder).

```json
{
  "pattern": "placeholder",
  "path": "/slides/2/content/0/placeholder_id",
  "code": "placeholder_remapped",
  "message": "slide 3: placeholder \"subtitle\" remapped to \"body\" for layout \"section\"",
  "fix": {
    "kind": "remap_placeholder",
    "params": {
      "from": "subtitle",
      "to": "body"
    }
  },
  "action": "info"
}
```

### `unresolved_placeholder`

**Action:** `review` (warning by default; `refuse`/error under `placeholder_policy: "strict"`)
**Category:** `POLICY`
**Fix kind:** `replace_placeholder`

A user-visible string still holds the `__FILL__` skeleton placeholder that `plan_deck` emits for agent-supplied content. Skeletons are draft scaffolding: `__FILL__` is a non-empty string, so the deck stays structurally valid (`valid: true`), but a finished deck must not ship the token. The scan is JSON-based (marshal + recursive string walk, mirroring the no-emoji policy; implemented in `internal/policy/placeholder`), so it covers placeholder text values, bullets, speaker notes, shape_grid cell text, table cells, chart/diagram labels, and pattern values in one pass.

Controlled by the `placeholder_policy` parameter on `validate_input` and `generate_presentation` (CLI: `--placeholder-policy` on `validate`):

- `warn` (default) — report each token with its JSON path as a warning; validation/generation still succeeds.
- `strict` — promote unresolved tokens to errors that fail validation and refuse generation. Use for publishable/gated output.
- `off` — skip the scan entirely.

Preflight (`generate -preflight`) always runs the scan at warning severity in its `POLICY` stage.

```json
{
  "code": "unresolved_placeholder",
  "path": "slides[0].content[0].text_value",
  "severity": "warning",
  "message": "slides[0].content[0].text_value still holds the unresolved skeleton placeholder \"__FILL__\" — replace it with real content before publishable generation (pass placeholder_policy=strict to block on it)",
  "fix": {
    "kind": "replace_placeholder",
    "params": {
      "path": "slides[0].content[0].text_value",
      "token": "__FILL__",
      "hint": "overwrite the __FILL__ token with the slide's real content — plan_deck skeletons are scaffolding, not finished text"
    }
  }
}
```

### `RENDER_EVIDENCE_INCOMPLETE`

**Action:** `refuse` (default; `review` when `allow_degraded_scoring` is set)
**Category:** `RENDER`
**Fix kind:** none (not source-repairable)

Emitted by the scoring facades (`score_deck`, `auto_repair`, `make_deck`) — **not** by the `validate -fit-report` pipeline — when the render pass that backs the deterministic score fails to complete. The render pass (`collectRenderFindings`) generates the deck to a temp directory so the score can capture render-time effects (contrast swaps, autofit shrink, pagination, clamping); when slide conversion, temp-dir creation, or generation fails, the render finding set is empty. That empty set must not be read as a clean render, so this synthetic finding makes the failure explicit. As a `refuse` finding it counts toward the P0 gate criterion, so it blocks `score_deck`'s `quality_gate` and `auto_repair`/`make_deck`'s `gate_passed`.

The facades also surface a structured `render_evidence` block (`{complete:false, stage, detail, degraded}`) alongside the finding. With `allow_degraded_scoring: true`, the finding drops to advisory (`review`) and `render_evidence.degraded` is set, but `evidence_complete` stays false and the facades' final `output_validation` still blocks a structurally invalid deck. See the skill's [Validation evidence on the repair facades](../skills/generate-deck/SKILL.md) for the response contract.

```json
{
  "code": "RENDER_EVIDENCE_INCOMPLETE",
  "action": "refuse",
  "message": "render-time validation evidence is incomplete: the \"generate\" stage failed (…); reported findings reflect static analysis only and may miss render-time defects (contrast swaps, autofit shrink, pagination, clamping)"
}
```

## Scope Rules

Fit findings are scoped to **JSON-authored content only**. Content inherited from template layouts or masters is never checked.

### What is checked

- **Placeholder text** — body, content, and title placeholders populated from `slides[].content[]`
- **Shape grid cells** — shapes and tables authored in `slides[].shape_grid`
- **Content-level tables** — tables in `slides[].content[]` with `type: "table"`

### What is excluded

- **Layout-inherited shapes** — shapes that come from the template's slide layout or master are never checked. Callers filter these before passing to detectors.
- **Decorative shapes** — shapes with `role: "background"` or `role: "decor"` are skipped by `slide_bounds_overflow`, `footer_collision`, and `title_collision`. These are intentionally placed at edges or off-slide.
- **Sparse grids** — `sparse_layout` fires when the estimated content extent is under 40% of the grid bounds height. Since bounds are authoritative (never shrink), all grids are checked uniformly.
- **Autofit placeholders** — `placeholder_overflow` is suppressed when the placeholder has `normAutofit` or `spAutoFit` set, because PowerPoint will auto-shrink text to fit.
- **Layouts without footer** — `footer_collision` only fires when the slide's resolved layout declares a footer placeholder (dt, ftr, or sldNum). No finding is emitted on layouts using heuristic fallback positioning.
- **Shared geometry** — `slide_bounds_overflow`, `footer_collision`, and `title_collision` resolve shape_grid cell coordinates through the same layout-aware helper (`resolveGridGeometry` → `resolveGridBounds`) generation uses, so preflight evaluates the geometry that will actually render. `title_collision` only fires when a title-anchored content zone could be resolved for the slide. The text-density findings (`fit_overflow`, `cell_underfilled`) and the `strict_fit` gate resolve shape_grid cells through this **same** helper: `generateFitReport`/`evaluateStrictFit` are passed the resolved template layouts and slide dimensions, so a shape_grid cell is measured against the content-zone / virtual-layout bounds it will render into rather than generic full-slide defaults. CLI `generate`, MCP `generate_presentation` (the `strict_fit` gate), `validate -fit-report`, MCP `validate_input`, and `preview_presentation_plan` therefore agree on shape_grid fit findings and strict-refusal behavior. When no template can be resolved (e.g. `validate -fit-report` on a deck whose template name does not resolve), the report falls back to generic default bounds.

## Fix Kinds

Each finding includes a structured `fix` object with a machine-readable `kind`:

| Kind | Params | Description |
|------|--------|-------------|
| `reduce_text` | — | Shorten the text content to fit the available space |
| `shorten_title` | — | Shorten the title to avoid wrapping |
| `reposition_shape` | — | Move or resize the shape to stay within bounds |
| `split_at_row` | `row: int` | Split the table at the suggested row index |
| `split_pattern` | `filled_slots: int`, `recommended_max: int`, `first: int`, `second: int`, `title_part_2: string` | Split an overcrowded pattern grid across two slides |
| `swap_pattern` | `filled_pct: float`, `filled_slots: int`, `total_slots: int`, `reason: string` | Switch to a different pattern that better fits the content count |
| `use_one_of` | `available: string`, `did_you_mean?: string` | Replace the value with one of the listed alternatives |
| `replace_color` | `original_color: string`, `replacement_color: string`, `background_color: string`, `contrast_ratio_before: float`, `contrast_ratio_after: float` | Text color was auto-replaced for WCAG AA contrast compliance |
| `grow_pattern` | `filled_pct: float`, `bounds_height: int`, `content_height: int` | Content occupies too little of the available bounds — add more content or use a smaller pattern |
| `provide_data` | `chart_type: string` | Chart data is empty — provide data values |
| `provide_numeric_value` | `column: string`, `original_value: string`, `original_type: string` | A non-numeric chart value was coerced to zero — provide a numeric value |
| `provide_native_format` | `chart_type: string`, `expected_format: string` | Chart data shape was inferred — provide data in the native format |
| `add_detail_or_resize` | `current_density_pct: int` | Cell is underfilled — add more text content or use a smaller grid pattern |
| `remap_placeholder` | `from: string`, `to: string` | Author the resolved placeholder ID directly to avoid the engine's implicit remap |

## Per-Slide Finding Budget

To prevent noisy output on dense decks, findings are capped at **5 per slide** by default. Within each slide, findings are ranked by:

1. **Severity** — `refuse` > `shrink_or_split` > `review` > `info`
2. **Actionability** — findings with a `fix` object rank above those without

When more than 5 findings exist on a slide, the top 5 are returned plus a summary finding with code `findings_truncated`:

```json
{
  "path": "/slides/2",
  "code": "findings_truncated",
  "message": "8 more findings suppressed on this slide; use verbose_fit to see all",
  "action": "info"
}
```

To bypass the budget and see all findings, pass `verbose_fit: true` (MCP) or `--verbose-fit` (CLI).

## Accessing Fit Findings

### MCP (generate_presentation)

Pass `fit_report: true` in the tool input. Findings appear in the response under `fit_findings`:

```json
{
  "file_path": "/tmp/out/deck.pptx",
  "fit_findings": [ ... ]
}
```

Pass `verbose_fit: true` to return all findings without the per-slide budget limit.

### CLI (validate)

```bash
json2pptx validate -fit-report examples/basic-deck.json
json2pptx validate -fit-report -verbose-fit examples/basic-deck.json
```

Findings are printed to stderr grouped by slide. Exit code is nonzero only if any finding has action `refuse`.

### Compact Responses

The server advertises `experimental.compact_responses: true` in its `initialize` response; compaction itself is controlled by client opt-in (the client sends `experimental.compact_responses: true` in its capabilities) or the deprecated `MCP_COMPACT_RESPONSES=1` environment variable.

## Visual-QA Aesthetic Findings

These codes are emitted by the `slide-visual-qa` Haiku skill (see [`skills/slide-visual-qa/SKILL.md`](../skills/slide-visual-qa/SKILL.md)) from screenshot inspection of rendered slides. They are subjective image-derived signals — the engine cannot detect them from JSON alone because they depend on the rasterized result. Skill output is a JSON block of the shape `{"findings": [{"slide_index", "code", "severity", "detail"}]}`, consumed by `auto_repair` and other automation.

These findings never block generation. They raise the bar from "renders correctly" to "looks consulting-grade." Severity vocabulary differs from engine fit findings: visual-QA uses `blocking` / `warning` / `info` (matching output-validation findings), not `refuse` / `shrink_or_split` / `review` / `info`.

### `ACCENT_OVERLOAD`

**Severity:** `warning`

More than 2 distinct accent hues visible on a single slide, counting only accent fills (not background, neutrals, or text). Three or more accents on one slide reads as visual noise — the audience cannot tell which item carries the argument.

Engine has a related precondition `accent_overload` (lowercase) that checks accent counts in `shape_grid` JSON, but it only sees what JSON authors named. The visual-qa code catches cases where rendered swatches differ from the JSON (template defaults bleeding through, image content, etc.).

```json
{ "slide_index": 3, "code": "ACCENT_OVERLOAD", "severity": "warning", "detail": "3 distinct accent hues visible: coral, teal, gold" }
```

### `BASELINE_MISALIGN`

**Severity:** `warning`

Body text in adjacent grid cells of a sibling pattern (`card-grid`, `kpi-*`, `comparison-2col`, `team-bios`, …) does not sit on the same horizontal baseline. Visible misalignment between sibling cards breaks the "one row, one beat" reading rhythm.

```json
{ "slide_index": 7, "code": "BASELINE_MISALIGN", "severity": "warning", "detail": "left KPI body baseline ~12px above right KPI body" }
```

### `MISSING_TAKEAWAY`

**Severity:** `info`

A slide carrying a chart or 2×2 matrix is missing a visually-distinct takeaway / "so what" band (typically the bottom 8-12% of the slide, filled with `dk1` or an accent and white text). Without this band the audience has to derive the argument from the chart, which they rarely do correctly.

Engine has a parallel `takeaway_missing` (lowercase, action `review`) that fires when `slide.takeaway` is empty on chart/matrix slides; the visual-qa code catches cases where the takeaway text is present but the band is invisible (rendered with low contrast, off-slide, etc.).

```json
{ "slide_index": 5, "code": "MISSING_TAKEAWAY", "severity": "info", "detail": "bar-chart slide has no takeaway band" }
```

### `CHART_BORDER`

**Severity:** `warning`

A chart is rendered with a visible outer border framing the plot area. Borders add ink, frame the chart as decorative artwork, and clash with executive-style "data only" composition.

```json
{ "slide_index": 4, "code": "CHART_BORDER", "severity": "warning", "detail": "bar chart has a 1pt dk2 border around the plot area" }
```

### `CHART_VERTICAL_GRIDLINES`

**Severity:** `warning`

A bar or line chart is rendered with vertical gridlines along the value axis. Vertical gridlines on bar charts double-encode the bar lengths, and on most line charts they add ink without adding read accuracy. Executive style is horizontal gridlines only.

```json
{ "slide_index": 4, "code": "CHART_VERTICAL_GRIDLINES", "severity": "warning", "detail": "bar chart shows vertical gridlines at 25/50/75/100" }
```

### `REDUNDANT_LEGEND`

**Severity:** `warning`

A single-series chart (one bar/line/area series) carries a legend. A legend with one entry is pure noise — the title or axis label already names the series.

```json
{ "slide_index": 6, "code": "REDUNDANT_LEGEND", "severity": "warning", "detail": "single-series line chart shows a 1-entry legend ('Revenue')" }
```

### `NON_TABULAR_NUMS`

**Severity:** `warning`

Numeric labels (data labels, KPI numbers, table cells) are not right-aligned or do not use tabular figure spacing, so digit columns wobble between rows. Executive style requires tabular alignment so the audience can compare magnitudes by column position.

```json
{ "slide_index": 2, "code": "NON_TABULAR_NUMS", "severity": "warning", "detail": "KPI cards show ragged number widths ('1,243', '987', '12,408')" }
```

### `EYEBROW_NO_CAPS`

**Severity:** `info`

A slide carries an eyebrow / category line above the title that renders in regular body case instead of ALL-CAPS or with noticeably tighter letter spacing. Eyebrows that look like body text fail to anchor the slide as part of a section.

```json
{ "slide_index": 8, "code": "EYEBROW_NO_CAPS", "severity": "info", "detail": "eyebrow 'market context' renders in title case, no caps or letter-spacing distinction" }
```
