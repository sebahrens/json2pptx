# Fit Findings

Fit findings are structured diagnostics emitted when generated slide content may not render correctly — text overflowing placeholders, shapes falling outside slide bounds, or tables exceeding density limits. They are surfaced via the MCP `generate_presentation` tool (text-fit findings detected by `strict_fit` are merged into `fit_findings` unconditionally when `strict_fit != "off"`; the full preflight detector set runs when `fit_report=true`) and the CLI `json2pptx generate -json` and `validate -fit-report` commands (the JSON output's `fit_findings` always includes the active `strict_fit` findings).

**Chart / diagram dry-render.** `validate_input` and `preview_presentation_plan` also drive svggen's layout/labeling pass for every `chart_value` / `diagram_value` content item and merge the resulting `chart.*` findings (e.g. `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`) into the fit-finding stream. Agents see render-time chart issues at validate / preview time without paying for full generation. The strict-fit severity ladder applies identically to the generate path. The svggen top-level helper is `svggen.DryRender(req) ([]Finding, error)`; the corresponding MCP entry point is `render_diagram` with `dry_run: true`.

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

The `ActionRank(action)` function returns these numeric ranks. Unknown actions return -1. Findings are sorted by rank descending (most severe first), then by slide index ascending.

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

A diagram cell's aspect ratio differs from the rendered SVG's aspect ratio by more than 25% (the default svggen output is 4:3 — 800×600 — unless `DiagramSpec.Width`/`Height` overrides it). At that point, the chart is either stretched (when svggen ignores aspect) or letterboxed inside the cell, both of which read as visual noise. The agent action is to widen/shorten the cell, set `cell.fit` to `contain` / `fit-width` / `fit-height`, or pass explicit `diagram.width` / `diagram.height` matched to the cell.

```json
{
  "path": "/slides/0/shape_grid/rows/0/cells/0/diagram",
  "code": "diagram_aspect_mismatch",
  "message": "diagram cell aspect 0.50 differs from rendered bar_chart SVG aspect 1.33 by 62% — chart will be stretched or letterboxed; resize the cell, set cell.fit, or set explicit diagram.width/height",
  "fix": { "kind": "reshape_grid", "params": { "diagram_type": "bar_chart", "svg_aspect": 1.333, "cell_aspect": 0.5, "deviation": 0.625, "cell_width_emu": 3000000, "cell_height_emu": 6000000, "svg_width": 800, "svg_height": 600 } },
  "action": "review",
  "measured": { "width_emu": 3000000, "height_emu": 6000000 },
  "allowed": { "width_emu": 800, "height_emu": 600 },
  "overflow_ratio": 0.375
}
```

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

### `contrast_predicted`

**Action:** `info`
**Pattern:** *(none — preflight)*
**Fix kind:** `replace_color`

Preflight prediction that the renderer's contrast auto-fix pass would replace a text color to meet WCAG AA Large (3:1) contrast against its background. Emitted by `validate` and `preview_presentation_plan` (and downstream tools like `repair_slide` / `score_candidates`) using only theme colors and JSON content — no rendering. When the same input is generated, the corresponding `contrast_autofixed` finding will fire at render time with matching colors.

The detector walks shape-grid cells that author both a fill color (on the shape) and a text color (on `shape.text` or on a paragraph in `shape.text.paragraphs`). Scheme names (`accent1`, `lt1`, …) are resolved against the template theme. Pairs that cannot be parsed are skipped.

```json
{
  "path": "/slides/1/shape_grid/rows/0/cells/0/shape/text",
  "code": "contrast_predicted",
  "message": "predicted: low-contrast text will be auto-replaced — #FFFFFF → #595959 (on #FFE8D4, ratio 1.3 → 3.0)",
  "fix": {
    "kind": "replace_color",
    "params": {
      "original_color": "#FFFFFF",
      "predicted_replacement": "#595959",
      "background_color": "#FFE8D4",
      "contrast_ratio_before": 1.3,
      "contrast_ratio_after": 3.0,
      "source": "shape_grid"
    }
  },
  "action": "info"
}
```

### `contrast_autofixed`

**Action:** `info`
**Pattern:** `placeholder`
**Fix kind:** `replace_color`

Text color was automatically replaced to meet WCAG AA contrast requirements against the resolved layout background. This is informational — the fix has already been applied. The `fix.params` include the original and replacement colors, the background color, and the contrast ratios before and after the swap.

```json
{
  "pattern": "placeholder",
  "path": "/slides/1/content/body",
  "code": "contrast_autofixed",
  "message": "auto-fixed low-contrast text: #FFFFFF → #1A1A1A (on #F5F5F5, ratio 1.3 → 15.2)",
  "fix": {
    "kind": "replace_color",
    "params": {
      "original_color": "#FFFFFF",
      "replacement_color": "#1A1A1A",
      "background_color": "#F5F5F5",
      "contrast_ratio_before": 1.3,
      "contrast_ratio_after": 15.2
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

## Scope Rules

Fit findings are scoped to **JSON-authored content only**. Content inherited from template layouts or masters is never checked.

### What is checked

- **Placeholder text** — body, content, and title placeholders populated from `slides[].content[]`
- **Shape grid cells** — shapes and tables authored in `slides[].shape_grid`
- **Content-level tables** — tables in `slides[].content[]` with `type: "table"`

### What is excluded

- **Layout-inherited shapes** — shapes that come from the template's slide layout or master are never checked. Callers filter these before passing to detectors.
- **Decorative shapes** — shapes with `role: "background"` or `role: "decor"` are skipped by `slide_bounds_overflow` and `footer_collision`. These are intentionally placed at edges or off-slide.
- **Sparse grids** — `sparse_layout` fires when the estimated content extent is under 40% of the grid bounds height. Since bounds are authoritative (never shrink), all grids are checked uniformly.
- **Autofit placeholders** — `placeholder_overflow` is suppressed when the placeholder has `normAutofit` or `spAutoFit` set, because PowerPoint will auto-shrink text to fit.
- **Layouts without footer** — `footer_collision` only fires when the slide's resolved layout declares a footer placeholder (dt, ftr, or sldNum). No finding is emitted on layouts using heuristic fallback positioning.

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

MCP clients can negotiate compact (non-indented) JSON output via capability negotiation. Send `experimental.compact_responses: true` in the client capabilities during the MCP `initialize` handshake. The server advertises support for this in its own `experimental` capabilities.

The `MCP_COMPACT_RESPONSES=1` environment variable is still honored as a fallback but is deprecated and will be removed in a future release.
