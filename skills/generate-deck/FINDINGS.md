# Finding Codes and Fix Kinds

Complete catalog of finding codes (emitted by `validate_input`, `preview_presentation_plan`, `generate_presentation`, and svggen) and the `fix.kind` vocabularies (`fit-report` enum and `repair_slide` apply-only superset). All findings follow the `{path, code, severity, action, message, fix}` envelope. Stable fields for programmatic matching: `code`, `severity`, `action`, `fix.kind`, `fix.params`. Advisory (may change): `message`.

For the contributor-facing catalog (with longer rationale + emission paths) see `../../docs/FIT_FINDINGS.md`.

**Runtime lookup:** for any single unfamiliar code, call the `describe_finding` MCP tool — it returns the same `{summary, severity, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}` envelope this catalog documents, sourced from the engine's own registry so it cannot drift. One tool call resolves any code without loading this file.

---

## Layout Finding Codes

Native (non-chart) findings. No prefix — the `chart.*` namespace below covers charts and diagrams.

### Pre-flight codes — emitted when measuring the deck before render

| Code | When emitted | Default action | Typical `fix.kind` |
|------|-------------|----------------|--------------------|
| `placeholder_overflow` | Body text overflows placeholder after autofit (three-condition gate: overshoot > 15%, autofit off/unavailable, can't fit at min font) | `shrink_or_split` | `reduce_text` |
| `title_wraps` | Title placeholder measures >1 line (informational, distinct from `placeholder_overflow`) | `review` | `reduce_text` |
| `slide_bounds_overflow` | JSON-authored shape center falls outside slide rect (center-based threshold, not corners) | `shrink_or_split` | `reduce_text` |
| `footer_collision` | Authored shape bbox intersects footer area on a layout that declares a footer placeholder | `review` (strict: `refuse`) | `reduce_text` |
| `fit_overflow` | Per-cell: text needs more lines than cell height allows at the declared font | `refuse` | `split_at_row` / `reduce_text` |
| `density_exceeded` | Table rows × cols beyond TDR ceiling at the declared font (Rule 20) | `review` | `split_at_row` |
| `stacked_tables` | Sibling tables in a shape_grid with `row_gap < 4pt` (two-tables-one-grid anti-pattern) | `review` | `split_at_row` |
| `divider_too_thin` | Divider shape height < 4% of slide height | `review` | — |
| `hex_fill_non_brand` | Non-allowlisted `#RRGGBB` fill on a shape | `review` | `use_semantic_color` |
| `mixed_fill_scheme` | Slide mixes semantic (`accent1`, `lt2`) and hex fills (hex-fill mix anti-pattern) | `review` | `use_semantic_color` |
| `accent_overload` | Slide uses more than two distinct accent hues (`accent1`..`accent6`) — pick one base accent and use `cell_accent_mode` for variety | `review` | `consolidate_accents` |
| `cell_underfilled` | Per-cell: text uses <60% of cell character capacity (density bands: <40% = warning, 40-59% = info) | `review` | `add_detail_or_resize` |
| `CHART_PLACEHOLDER_EMPTY` | `chart-insights-split` pattern rendered without a `chart` spec — left panel collapses and insights expand to full width. `action: review`. Agent action: supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`) | `review` | — |
| `HEADLINE_TOO_LONG` | Title-class placeholder text exceeds 12 whitespace-separated words. Fires on `title`, `headline`, `ctrTitle` text content items. Advisory — never blocks render. `fix.params: {current_words, max_words}` | `review` | `shorten_title` |
| `BODY_TOO_LONG` | A single text block exceeds 80 whitespace-separated words. Applies to `text`, `bullets`, `body_and_bullets`, and `bullet_groups` content items (bullet words aggregate per block). Advisory — never blocks render. `fix.params: {current_words, max_words}` | `review` | `reduce_text` |
| `BULLET_NESTING_DEEP` | Bullet list nests more than 2 levels. Depth is measured from leading whitespace (each tab or every 2 leading spaces = 1 indent unit); `bullet_groups` bullets start at level 2 (header is level 1). Advisory — never blocks render. `fix.params: {current_depth, max_depth}` | `review` | `reduce_text` |

### Density-band severity for cell capacity findings

`fit_overflow` and `cell_underfilled` share the density axis:

| Density band | Code | Severity | Action | Guidance |
|---|---|---|---|---|
| >130% | `fit_overflow` | `error` | `refuse` | Must reduce text — blocks under `strict_fit: "strict"` |
| 110–130% | `fit_overflow` | `warning` | `review` | Should reduce text before shipping |
| 60–110% | *(none)* | — | — | Healthy range — no finding emitted |
| 40–59% | `cell_underfilled` | `info` | `review` | Consider adding detail; not blocking |
| <40% | `cell_underfilled` | `warning` | `review` | Strongly consider adding detail or using a smaller grid |

**How to triage capacity findings:**
- **`info` severity** — consider acting; not blocking under any `strict_fit` mode
- **`warning` severity** — should act before shipping; not blocking under `strict_fit: "warn"` or `"off"`
- **`error` severity** — must act; blocks generation under `strict_fit: "strict"` (MCP returns `IsError=true`)

`strict_fit` interaction: only `error`-severity findings with action `refuse` block generation in strict mode. `cell_underfilled` never blocks because its maximum severity is `warning`.

### Render-time codes — emitted during `generate_presentation` when the engine adjusted content to fit

| Code | When emitted |
|------|-------------|
| `text_trimmed` | Trailing paragraphs trimmed to fit placeholder |
| `text_overflow` | Text still overflows placeholder after trimming |
| `readability_trimmed` | Paragraphs trimmed for readability floor |
| `no_autofit_overflow` | Text overflows placeholder that has `noAutofit` set |
| `table_rows_truncated` | Table rows truncated to fit row height |
| `table_font_scaled` | Table font scaled down to the minimum floor |
| `diagram_clamped` | Diagram placeholder dimensions clamped to minimum. `action: review`, `fix.kind: swap_layout`, `fix.params: {dimension, original_emu, clamped_emu}`. Agent action: switch to a wider layout via `repair_slide` |
| `diagram_render_failed` | Diagram render failed; placeholder image inserted. `action: review`, `fix.kind: review` (no auto-fix). Agent must inspect diagram data and decide whether to simplify, change type, or regenerate |
| `column_width_deficit` | Column widths fell back to global floor |
| `pagination_default_threshold` | Pagination used default threshold (no template capacity available) |
| `contrast_autofixed` | Text color auto-replaced for WCAG AA. `action: info`, `fix.kind: replace_color`, `fix.params: {original_color, replacement_color, background_color, contrast_ratio_before, contrast_ratio_after}` |
| `contrast_predicted` | Preflight prediction (validate / preview) that a shape-grid text color will be auto-replaced for WCAG AA at render time. Same `fix.kind` (`replace_color`) as `contrast_autofixed`, with `fix.params.predicted_replacement` and `source` (e.g. `shape_grid`). `action: info` |
| `placeholder_remapped` | A content `placeholder_id` was resolved to a different placeholder on the layout (e.g. `subtitle` → `body` on the `section` layout). Emitted both pre-flight (`preview_presentation_plan` and `generate_presentation` with `fit_report:true` — one finding per `resolved_slides[].placeholders[]` with `remapped:true`) and at render time. `action: info`, `fix.kind: remap_placeholder`, `fix.params: {from, to}`. Path targets `/slides/{i}/content/{j}/placeholder_id`. Agent action: author the resolved id directly to avoid the implicit rewrite |
| `grid_diagram_narrow` | Complex diagram in a narrow grid cell (<50% slide width). `action: review`, `fix.kind: reshape_grid`, `fix.params: {diagram_type, complexity, cell_width_pct, cell_width_emu, threshold_emu}`. Path targets the diagram field (e.g. `/slides/0/shape_grid/rows/0/cells/1/diagram`). Agent action: widen cell via `repair_slide` with `reshape_grid` fix |
| `diagram_aspect_mismatch` | Diagram cell aspect differs from rendered SVG aspect by >25% (default svggen aspect is 4:3 unless `DiagramSpec.Width/Height` overrides). Chart will be stretched or letterboxed. `action: review`, `fix.kind: reshape_grid`, `fix.params: {diagram_type, svg_aspect, cell_aspect, deviation, cell_width_emu, cell_height_emu, svg_width, svg_height}`. Path targets the diagram field. Agent action: resize the cell, set `cell.fit` (`contain`/`fit-width`/`fit-height`), or set explicit `diagram.width`/`diagram.height` matched to the cell |
| `diagram_aspect_conflict` | Non-chart diagram cell (or placeholder) aspect deviates from the diagram type's natural svggen viewBox aspect by >30%. Emitted for diagrams pinned to fixed natural aspects via `svggen.NaturalAspect` (currently `timeline` 2:1, `gantt` ~1.8:1, `org_chart` ~1.57:1). Silent for chart types (covered by svggen dry-render `chart.*`) and for diagrams with explicit `DiagramSpec.Width/Height` (covered by `diagram_aspect_mismatch`). `action: review`, `fix.kind: reshape_grid`, `fix.params: {diagram_type, natural_aspect, cell_aspect, deviation, cell_width_emu, cell_height_emu}`. Path targets the diagram field. Agent action: resize the cell, set `cell.fit`, or set explicit `diagram.width`/`diagram.height`. Available at validate + preview without invoking resvg/inkscape |

### Budget summary code

Emitted when more than `DefaultFindingBudget` (5) findings exist on a slide and `verbose_fit:false`:

| Code | When emitted |
|------|-------------|
| `findings_truncated` | Per-slide finding budget exceeded; remaining findings suppressed. `action: info`, `fix.kind: truncation_summary`, `fix.params: {suppressed_count: int, top_codes: ["code:count", ...] sorted by count desc}`. Pass `verbose_fit: true` (MCP) or `--verbose-fit` (CLI) to see all findings without truncation |

### Icon preflight codes — emitted before render to catch broken `icon.name` / `icon.path` fields

| Code | When emitted |
|------|-------------|
| `ICON_BUNDLED_NAME_UNKNOWN` | `icon.name` does not resolve in the bundled icon registry. Emitted by `validate_input` and `generate_presentation` preflight so agents can fix typos and missing `filled:` prefixes without burning a generate cycle. `severity: error`. `details: {input_value, suggestions: ["chart-pie", ...], slide_index, remediation}`. Path targets the icon node, e.g. `/slides/0/shape_grid/rows/0/cells/0/icon`. `suggestions[0]` is the highest-ranked Levenshtein match (or qualified cross-set form when the bare base name only resolves in a non-default set). Agent action: replace `icon.name` with the suggested value, or call `list_icons` to discover the canonical `qualified_name` |
| `ICON_NOT_FOUND` | `icon.path` does not point at an existing file after resolution against the JSON input directory (CLI) or server CWD (MCP). `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}`. Agent action: verify the file path is correct, switch to a bundled icon via `name`, or supply inline `svg_data` |
| `ICON_PATH_EXT_INVALID` | `icon.path` extension is not `.svg`. Icons must be SVG so they can be re-tinted; raster images should use `image_value` or shape-grid `image` cells. `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |
| `ICON_PATH_TRAVERSAL` | `icon.path` contains `..` components. Rejected before `filepath.Clean` collapses them, so agents can't escape the base directory via a constructed relative path. `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |
| `ICON_PATH_SYMLINK_ESCAPE` | `icon.path` is relative and its symlink chain resolves outside the base directory. `severity: error`. `details: {input_value, resolved_path, asset_kind: "icon", slide_index, remediation}`. Agent action: pin an absolute path explicitly, or remove the offending symlink |
| `ICON_PATH` | Other `icon.path` resolution failures not covered by the more specific codes above (symlink loop, permission denied, etc.). `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |

### Action semantics (shared with chart codes)

- `refuse` — with `strict_fit: "strict"`, generation is blocked and MCP returns `IsError=true`; with `warn`, emits finding only
- `shrink_or_split` — content will be adjusted or distributed; strict promotes to `refuse` for content-loss codes
- `review` — informational; agent should inspect but no automatic remediation
- `info` — advisory/telemetry only, never promoted

---

## `fix.kind` enum (fit-report — stable for programmatic matching)

This is the enum the engine emits in `validate_input` / `preview_presentation_plan` / `generate_presentation` fit-report findings. `repair_slide` accepts a *superset* — see the next section.

| Kind | Semantics | Params |
|------|-----------|--------|
| `reduce_text` | Shorten text content in the indicated path | — |
| `split_at_row` | Emit `split_slide` at the given row index | `row: int` |
| `use_semantic_color` | Replace hex fill with `accent1`/`lt2`/`dk1`/… | `message?` |
| `replace_color` | Swap one explicit color for another | `from, to` |
| `replace_value` | Replace an invalid value with a suggested one | `suggestion, allowed?` |
| `provide_value` | Required field is missing | `field` |
| `use_one_of` | Value must be one of an allowed set | `allowed` |
| `rename_field` | Unknown field name close to a known one | `from, to` |
| `reshape_value` | Value has wrong structure (array vs object, etc.) | `path, value` |
| `remove_field` | Unknown field should be removed | — |
| `add_detail_or_resize` | Cell is underfilled — add more text or use a smaller grid | `current_density_pct: int` |

Chart/diagram codes (below) introduce their own `fix.kind` values: `reduce_items`, `explicit_scale`, `truncate_or_split`, `align_series`, `increase_canvas`.

---

## Fix kinds for `repair_slide` — complete table

The apply-only superset accepted by `repair_slide` is broader than the fit-report `fix.kind` enum: fit-report only emits kinds the engine can derive automatically, while `repair_slide` also accepts kinds the *agent* decides to apply (e.g., `swap_layout`, `swap_pattern`, `autofix_visual`). Every kind below is a `case` in `applyRepairFix` (`cmd/json2pptx/mcp_repair.go`); the drift test `cmd/json2pptx/skill_drift_test.go` enforces this list and the kinds advertised by `get_capabilities().vocabularies.repair_fix_kinds` stay in sync.

| Kind | Semantics | Required params | Optional params |
|------|-----------|-----------------|-----------------|
| `reduce_text` | Shorten bullets / body text on a content item | — | `max_items: int` (bullets), `max_length: int` (text), `path: string` (JSON Pointer to one content item) |
| `shorten_title` | Truncate the title placeholder text | — | `max_length: int` (default 50), `path: string` |
| `split_at_row` | Wrap the slide in a `split_slide` envelope, distributing table rows across pages | `row: int` (rows per page; alias `group_size`) | `title_suffix: string` (default ` ({page}/{total})`), `repeat_headers: bool` (default true), `path: string` |
| `swap_layout` | Change the slide's `layout_id` | `layout_id: string` | — |
| `use_one_of` | Replace a slide-level enum field (`layout_id`, `transition`, `transition_speed`, `build`) or a content `type` with an allowed value | `path: string`, `value: string` | — |
| `replace_color` | Replace a specific fill color anywhere in `shape_grid` cells (string or object form). Accepts `contrast_autofixed` finding params as aliases. | `from: string` (or `original_color`), `to: string` (or `replacement_color`) | — |
| `use_semantic_color` | Replace hex fills with a semantic scheme name (`accent1`, `dk1`, ...). With `path` set, targets one cell; without, replaces all hex fills on the slide. | `value: string` (scheme color name) | `path: string` (cell fill path) |
| `split_pattern` | Split a pattern-driven shape_grid into two slides at a computed row boundary. Useful for overflowing grids without changing the pattern. | — | `first: int` (cells on slide 1; default = half), `title_part_2: string` (suffix; default `"(continued)"`) |
| `swap_pattern` | Replace the slide's pattern with a different one; optionally replace `values`, `overrides`, `cell_overrides`. Clears any expanded `shape_grid` for re-expansion. | `to: string` (target pattern name) | `values: object`, `overrides: object`, `cell_overrides: object` |
| `reshape_grid` | Change grid dimensions. For pattern slides, updates `rows`/`columns` in pattern values; for raw `shape_grid` slides, redistributes cells into a new row/column layout. | One of `rows: int` or `columns: int \| [int]` | both |
| `set_pattern_style` | Set the `style` key in the pattern's `overrides` (e.g., `timeline-horizontal` from `"dots"` to `"chevron"`) and clear expanded grid for re-expansion. | `style: string` | — |
| `reduce_cell_text` | Truncate one `shape_grid` cell's text to a character budget, appending U+2026 and stripping orphaned markdown emphasis markers. Use only when the agent should not rephrase the text. | `cell_path: string` (JSON Pointer e.g. `"/slides/0/shape_grid/rows/1/cells/2"`), `max_chars: int` (> 1) | — |
| `rename_field` | Rename a top-level key. Searches pattern values first, then slide-level fields via JSON round-trip. | `from: string`, `to: string` | — |
| `reshape_value` | Replace a pattern-values field with a restructured replacement (array→object, etc.). | `path: string` (field name), `value: any` | — |
| `provide_value` | Set a pattern-values field that is missing | `path: string`, `value: any` | — |
| `replace_value` | Replace an existing pattern-values field with a new value (e.g., to bring it within valid bounds) | `path: string`, `value: any` | — |
| `reduce_items` | Truncate a pattern-values array field to `max_items` | `path: string`, `max_items: int` (> 0) | — |
| `add_items` | Append agent-supplied items to a pattern-values array field | `path: string`, `items: array` | — |
| `resize_list` | Resize a pattern-values array field to exactly `count` items. Truncates if too many; returns not-applied if too few (agent must follow up with `add_items`). | `path: string`, `count: int` (> 0) | — |
| `remove_key` | Delete a key from the pattern's `overrides` (preferred) or `values` | `key: string` | — |
| `remove_field` | Delete a top-level field from pattern values or from the slide (via JSON round-trip) | `path: string` | — |
| `autofix_visual` | Map a visual-QA finding category to one or more candidate fix kinds and try them in order. Caller-supplied params are forwarded (caller wins). | `category: string` (visual QA finding category) | any params forwarded to the underlying kind |

Unsupported kinds return:

```json
{
  "applied": false,
  "code": "kind_not_supported",
  "message": "kind_not_supported",
  "supported_kinds": ["add_items", "autofix_visual", "provide_value", ...],
  "next_tool_call": {"tool": "get_capabilities", "args_template": {}}
}
```

`supported_kinds` is the full authoritative vocabulary inline — recover by retrying with one of those kinds instead of issuing a separate `get_capabilities` call. The `next_tool_call` is still surfaced as a fallback for agents that want to consume the canonical capabilities snapshot. The list is identical to `get_capabilities().vocabularies.repair_fix_kinds` and a compile-time test (`TestBuildCapabilitiesContract/repair_fix_kinds matches applyRepairFix switch cases`) keeps the two in lock-step.

---

## Chart Finding Codes

Charts and diagrams emit structured findings at render time, following the same `{path, code, message, fix}` envelope as native layout findings. Codes use the `chart.*` prefix.

**Dry-render parity:** `validate_input` (with `fit_report: true`) and `preview_presentation_plan` now invoke svggen's layout/labeling pass for every `chart_value` / `diagram_value` content item and merge the resulting `chart.*` findings into `fit_findings`. Agents see `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`, `chart.label_truncated`, and `chart.scatter_label_skipped` BEFORE calling `generate_presentation` — no full render required. The same strict-fit severity ladder applies. For ad-hoc per-diagram dry-runs use the svggen-mcp `render_diagram` tool with `dry_run: true`.

### Data-integrity codes — indicate bad input data

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.invalid_numeric` | NaN/Inf values clamped during render | `replace_value` |
| `chart.zero_sum_pie` | Pie/donut with all-zero or all-negative values | `replace_value` |
| `chart.negative_on_log` | Negative values on a log-scale chart | `explicit_scale` |
| `chart.all_zero_series` | All series values are zero (flat chart) | `replace_value` |
| `chart.capacity_exceeded` | Series/points/categories exceed renderer limits | `reduce_items` |
| `chart.invalid_time_format` | Time-series string cannot be parsed | `replace_value` |

### Content-loss codes — successful degradation that dropped or truncated payload; promoted under `warn`

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.legend_overflow_dropped` | Legend entries dropped (area exceeded) | `reduce_items` |
| `chart.overflow_suppressed` | Overflow content suppressed or truncated | `reduce_items` |

(`chart.capacity_exceeded` is also a content-loss code but is grouped with data-integrity above because strict promotes it all the way to `refuse`.)

### Advisory codes — informational fitting/labeling adjustments; never promoted

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.auto_log_scale_applied` | Auto-switched to log scale based on data range | `explicit_scale` |
| `chart.tick_thinned` | Axis tick labels thinned to prevent overlap | `reduce_items` |
| `chart.scatter_label_skipped` | Scatter label skipped due to collision | `increase_canvas` |
| `chart.label_truncated` | Label truncated to fit available space | `increase_canvas` |
| `chart.label_ellipsized` | Label shortened with ellipsis | `increase_canvas` |
| `chart.label_clipped` | Label hard-clipped at container boundary | `increase_canvas` |

### Strict-fit promotion ladder for chart codes

Matches `svggen/core/finding_codes.go::promotionTable`:

| Level | `chart.capacity_exceeded` | `chart.legend_overflow_dropped`, `chart.overflow_suppressed` | Data-integrity codes (5) | Advisory codes (6) |
|-------|---------------------------|-----------------------------------------------------|------------------------|--------------------|
| `off` | (no promotion) | (no promotion) | (no promotion) | (no promotion) |
| `warn` | `shrink_or_split` | `shrink_or_split` | (no promotion) | (no promotion) |
| `strict` | `refuse` | `shrink_or_split` | `refuse` | (no promotion) |

Example chart finding in a fit report:

```json
{
  "path": "slides[1].content.chart_value",
  "code": "chart.capacity_exceeded",
  "message": "12 series exceeds max_series=50 — truncated to first 50",
  "severity": "shrink_or_split",
  "fix": { "kind": "reduce_items", "params": { "limit": 50 } }
}
```
