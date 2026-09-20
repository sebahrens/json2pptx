# Fit Findings

Fit findings are structured diagnostics emitted when generated slide content may not render correctly — text overflowing placeholders, shapes falling outside slide bounds, or tables exceeding density limits. They are surfaced via the MCP `generate_presentation` tool (text-fit findings detected by `strict_fit` are merged into `fit_findings` unconditionally when `strict_fit != "off"`; the full preflight detector set runs when `fit_report=true`) and the CLI `json2pptx generate -json` and `validate -fit-report` commands (the JSON output's `fit_findings` always includes the active `strict_fit` findings).

**Render-time chart findings reach the report too.** A chart or diagram in a content placeholder raises the same `chart.*` findings while it is actually being drawn; those used to be collected for tables and dropped for diagrams, so what the renderer gave up was reported nowhere (go-slide-creator-p142). Both sides now go through one conversion (`generator.SvggenFindingsToFit`), so a dry-run finding and the same finding raised at render agree on code, path and action.

**Chart / diagram dry-render.** `validate_input` and `preview_presentation_plan` also drive svggen's layout/labeling pass for every `chart_value` / `diagram_value` content item **and every diagram surface embedded in a slide's `shape_grid` or in a named pattern's expanded grid (e.g. `chart-insights-split`, paths rooted at `/slides/{i}/pattern`)** — cell `diagram`s, composite cell `sub_diagram`s, and diagrams inside recursively nested sub-grids — merging the resulting `chart.*` / `diagram.*` findings (e.g. `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`, `chart.plot_area_collapsed`, `diagram.org_chart_depth_pruned`, `diagram.items_dropped`) into the fit-finding stream. Content-item findings keep the legacy `slides[i].content[j].chart_value` path; shape_grid findings use the slidepath JSON Pointer convention shared with the structural detectors (e.g. `/slides/0/shape_grid/rows/1/cells/2/diagram`, `.../composite/sub_diagram`, or a nested `.../cells/2/grid/rows/0/cells/1/diagram`). Agents see render-time chart issues at validate / preview time without paying for full generation. The strict-fit severity ladder applies identically to the generate path. The svggen top-level helper is `svggen.DryRender(req) ([]Finding, error)`; the corresponding MCP entry point is `render_diagram` with `dry_run: true`.

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
| `fix` | object | Structured remediation: `{kind, params?}`. The kind is either **executable** (`repair_slide` applies it — `get_capabilities().vocabularies.repair_fix_kinds`) or **advisory** (the remedy is an authoring decision — `advisory_fix_kinds`). See [Fix-kind classes](#fix-kind-classes). |
| `action` | string | Recommended severity/remediation action |
| `measured` | object | Actual content extent in EMU (omitted when N/A) |
| `allowed` | object | Available frame extent in EMU (omitted when N/A) |
| `overflow_ratio` | float | `measured / allowed` as a fraction (omitted when 0) |
| `next_tool_call` | object | Machine-readable MCP tool suggestion: `{tool, args_template}` (omitted for `info` findings) |
| `segment_index` | integer | 0-based child segment index inside a compose envelope when the finding is attributable to one (omitted otherwise). Populated for compose-emitted findings (`COMPOSE_HORIZONTAL_TRUNCATION`, `COMPOSE_SEGMENT_BOUNDS_IGNORED`, `COMPOSE_SEGMENT_EXPAND_FAILED`) and for per-cell findings whose merged-grid row/col falls inside a segment's `row_range`/`col_range` (see `preview_presentation_plan` → `resolved_slides[].expanded_compose`). |

### Fix-kind classes

Every `fix.kind` belongs to one of two vocabularies, both registered in `internal/patterns/fix_kinds.go` and advertised by `get_capabilities().vocabularies`:

- **executable** (`repair_fix_kinds`) — `repair_slide` applies it mechanically from `fix.params`: `reduce_text`, `shorten_title`, `reduce_cell_text`, `split_at_row`, `split_pattern`, `swap_layout`, `swap_pattern`, `reshape_grid`, `set_pattern_style`, `set_max_height_pct`, `use_one_of`, `replace_color`, `use_semantic_color`, `rename_field`, `reshape_value`, `provide_value`, `replace_value`, `reduce_items`, `add_items`, `resize_list`, `remove_key`, `remove_field`, `autofix_visual`.
- **advisory** (`advisory_fix_kinds`) — the remedy needs an authoring decision or a human eye, so no mechanical edit exists: `add_detail_or_resize`, `adopt_pattern`, `consolidate_accents`, `fix_structure`, `grow_pattern`, `increase_gap`, `increase_row_height`, `provide_data`, `provide_native_format`, `provide_numeric_value`, `reduce_columns`, `remap_placeholder`, `remove_emoji`, `remove_field_or_switch_pattern`, `replace_placeholder`, `reposition_shape`, `review`, `review_layout`, `rewrite_field`, `set_design_mode_free`, `shrink_text`, `text`, `truncation_summary`.

The distinction is load-bearing for the repair loop. Sent to `repair_slide`, an advisory kind returns `{applied: false, code: "advisory_fix_kind", message: "<the decision to make>", alternatives: [executable kinds addressing the same defect]}` — not `kind_not_supported`, which reads as a caller mistake. `propose_repairs` routes them into `advisory[]` (with `guidance` and `alternatives`) and counts them in `summary.advisory_findings`, instead of burying them in `unmapped[]` as `fix_kind_not_repairable:<kind>`.

On a well-formed deck most remaining fix-carrying findings are advisory — `cell_underfilled`, `SPARSE_FILL` and `SLIDE_UNDERUSED` are all "this slide has room for more argument", which no edit can supply. A repair loop that ends with advisory findings only has finished, not failed.

Adding a kind means registering it: `TestEveryEmittedFixKindIsRegistered` fails on an unregistered kind, `TestRepairFixKindsMatchApplySwitch` pins the executable list against `applyRepairFix`, and `TestAdvisoryFixKindsCarryGuidance` requires every advisory kind to carry actionable guidance.

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

### `PATTERN_UNKNOWN_FIELD`

**Action:** `refuse`
**Pattern:** the slide's pattern
**Fix kind:** `rename_field` (with `did_you_mean`) or `remove_field`
**Emitted at:** pattern input inspection — `validate` / `validate_input` / `generate` / `expand_pattern`

A key in a pattern's `values` / `overrides` / `cell_overrides` is never read by the pattern, so the text written under it never reaches the slide. The check is empirical, not schema-guessing: the raw payload is decoded with the pattern's **own** decoder, re-marshalled, and compared against what the caller wrote. A key is reported only when its content is demonstrably absent from the decoded value, so the tolerated aliases (a KPI cell's `{value, label}` for `{big, small}`, the `"$4.2M | ARR"` string shorthand) never trip it, and an object whose schema declares no properties — a chart's map-form `data`, keyed by series name — is never judged at all.

Keys **inside a cell** count too: `values[0].bogus` on kpi-3up, `values.cells[2].bodySize` on card-grid. Every pattern whose values are a list of cells declares the element as `oneOf{shorthand string, object}`, and the field names live in the object branch; the inspector used to ask the `oneOf` itself, get nothing back, and read that as "free-form — nothing here can be unknown", so a typo inside a cell was dropped in silence and the deck validated clean (go-slide-creator-4cqh). `json2pptx patterns validate` reports the same findings and exits non-zero on them, so a pattern payload cannot be told it is valid while a field of it is being discarded.

`did_you_mean` names a property the caller has **not** already filled, preferring one in the same slide vocabulary (`title` → `role`, `label` → `name`, `columns` → `rows`), one that contains the unknown name (`date` → `date_label`), one within edit distance, and — failing all of those — the single required property still missing. When nothing is close, `fix.params.allowed` carries the property list instead.

```json
{
  "code": "PATTERN_UNKNOWN_FIELD",
  "path": "/slides/0/pattern/values/members/0/title",
  "message": "team-bios: unknown field \"title\" at values.members[0].title is dropped — its content never reaches the slide; did you mean \"role\"?",
  "fix": { "kind": "rename_field", "params": { "path": "/slides/0/pattern/values/members/0/title", "from": "title", "to": "role", "did_you_mean": "role" } },
  "next_tool_call": { "tool": "show_pattern", "args_template": { "name": "team-bios" } },
  "action": "refuse"
}
```

Its shape-level sibling is `invalid_shape`, emitted when the payload's JSON type is one the pattern cannot read at all. Both name the expected shape in schema terms — never a Go type — and carry a copy-ready `example` where one can be derived from what the caller wrote:

```json
{
  "code": "invalid_shape",
  "path": "/slides/0/pattern/values/steps/0",
  "message": "process-flow: values.steps[0] must be an object {label, type?}; got the string \"A\". Example: {\"label\":\"A\"}",
  "fix": { "kind": "reshape_value", "params": { "path": "/slides/0/pattern/values/steps/0", "expected": "object", "got": "string", "example": { "label": "A" } } },
  "action": "refuse"
}
```

A wrapper object around the array a pattern wants is called out by name, because the edit is one unwrap:

```json
{
  "code": "invalid_shape",
  "path": "/slides/0/pattern/values",
  "message": "timeline-horizontal: values must be an array of objects {label, body?, date?, end_date?}, not an object wrapping one; the key \"stops\" is never read — send its value as values directly",
  "fix": { "kind": "reshape_value", "params": { "path": "/slides/0/pattern/values", "expected": "array", "got": "object", "unwrap_key": "stops", "items": 3 } },
  "action": "refuse"
}
```

### `TEXT_OVER_IMAGE_UNVERIFIED`

**Action:** `review`
**Fix kind:** `provide_value`
**Emitted at:** preflight (validate / preview / score)

A slide sets `background.image` (or `background.url`) with no `background.overlay`, and puts text on it. The contrast pass reads a **solid** background fill — `extractLayoutBackgroundColor` parses `<p:bg><p:bgPr><a:solidFill>` — so an image background is invisible to it: a photo has no single colour to compute a ratio against. The template's own title colour then lands wherever the picture happens to be dark or light, and nothing reports it.

The finding is advisory because only the author knows whether the photo is uniform under the text. Silent slides: a picture-only slide (no text), and a slide that has already opted out with `contrast_check: false`.

```json
{
  "path": "/slides/1/background",
  "code": "TEXT_OVER_IMAGE_UNVERIFIED",
  "message": "slide 2 puts text on a background image with no overlay — a photo has no single colour, so the contrast pass cannot check the text against it and the template's own title colour may land on a dark part of the picture",
  "fix": { "kind": "provide_value", "params": { "path": "/slides/1/background/overlay", "value": { "color": "dk1", "alpha": 0.45 }, "hint": "add a scrim over the photo; the contrast pass then judges the text against the scrim colour" } },
  "action": "review"
}
```

With an overlay present, the scrim decides the text's background: at `alpha` ≥ 0.35 the contrast pass judges the text against the scrim colour rather than falling back to the layout's fill.

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

**Measured escalation (go-slide-creator-vjwn).** Titles are also measured against the resolved title placeholder with the template's inherited title style (master size, all-caps, line spacing; canonical `layout_id`s such as `"content"` are resolved) using exact glyph widths — the same measurement the generator applies at render time. When the title only fits below the comfort size (80% of the template title size, capped at 32pt for large display titles) or only with reduced line spacing, `title_wraps` is emitted with `action: shrink_or_split`, `fix.kind: shorten_title` and `fix.params: {current_chars, max_chars, fit_scale_pct}` (`max_chars` = longest prefix that fits at the comfort size). When it cannot fit at all, `TITLE_OVERFLOW` is emitted instead. `validate` and the generate `quality` score use the same verdict, and they now say it in the same words: the validate-time diagnostic is the very same finding converted to a diagnostic, carrying the identical code (`title_wraps` / `TITLE_OVERFLOW`), path and message, so the findings envelope collapses the two into one entry instead of reporting one wrapped title twice under two codes (`max_length` no longer appears for titles). The measured verdict is now the **only** title-length rule:

- The 60-character score fallback is gone. An unmeasurable title scores clean rather than against a number nothing renders; `HEADLINE_TOO_LONG` (12 words) is the fallback lint for that case, and it stands down for any title the measurement covered.
- The `max_chars` reported for a **title** placeholder (`validate_input` / `generate -dry-run` `slides[].placeholders[]`, discovery `layout_summaries[]` / `layouts[]`, `examine_template` `report.json`) is the **measured capacity at the comfort size** for the resolved font, not the geometric area estimate. The estimate said 29 characters for a Blank+Title box that renders ~100, so agents aimed at a target no slide ever used (go-slide-creator-jcph). The number is a property of the box, not of the current text: `len(title) <= max_chars` now means exactly "no title finding".
- `fix.params.max_chars` on a flagged title stays the longest word-prefix of *that* title which fits, so it is a shortening target that is guaranteed to fit and is never above the placeholder's capacity in practice.

The measurement no longer requires an explicit `layout_id`. A slide that gives only `slide_type` — or a compiled DeckSpec slide, which never carries a `layout_id` — is measured against the layout **the heuristic selector will actually pick**, resolved in deck order with the same running context generation uses. Before this the same 112-character title produced the measured verdict with `layout_id: "content"` and the legacy `title too long (112 chars, max 60)` with `slide_type: "content"`, scoring 85 and 70 for the same deck; on the DeckSpec path — the one `get_started` recommends — no title was measured at all (go-slide-creator-t64e). Placeholder-existence validation still uses the author's own `layout_id`: the generator auto-maps placeholder IDs, so validating against a predicted layout would invent `placeholder_not_found` errors.

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

### `TITLE_OVERFLOW`

**Action:** `shrink_or_split`
**Pattern:** `placeholder`
**Fix kind:** `shorten_title`

The title does not fit its title placeholder even at the minimum autofit size (60% font scale, or the readability floor if higher, plus the maximum 20% line-spacing reduction). Title placeholders on generated slides usually inherit their geometry from the layout and their font size, all-caps and line spacing from the slide master's `titleStyle`; the measurement resolves all of these (e.g. modern-template: 45pt, `cap="all"`, 80% line spacing) and measures with exact glyph widths.

Emitted at render time (generate) and predicted by preflight (`validate --fit-report`, `validate_input`, preview), which share the same measurement. When the title fits by shrinking, the generator writes the reduced size (and any line-spacing reduction) explicitly into the title runs instead of relying on `<a:normAutofit fontScale>` — LibreOffice ignores the stored scale and re-shrinks by compressing line spacing, which made long title lines collide.

`fix.params`: `current_chars`, `max_chars` (longest word-prefix length that fits at the minimum size), `font_pt` (template size), `min_font_pt`.

```json
{
  "pattern": "placeholder",
  "path": "/slides/1/content/title",
  "code": "TITLE_OVERFLOW",
  "message": "title (212 chars) does not fit its 11.4\"x1.2\" title placeholder even at 27pt (template size 45pt); lines will collide or spill",
  "fix": { "kind": "shorten_title", "params": { "current_chars": 212, "max_chars": 118, "font_pt": 45, "min_font_pt": 27 } },
  "action": "shrink_or_split"
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

The limit counts **grid cells**, not the pattern's items: a pattern that draws each item as a stack of cells (`timeline-horizontal`'s dots layout emits a date, a dot and a label per stop) carries a limit scaled accordingly.

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

### `SPARSE_SINGLE_ROW_FLOW`

**Action:** `review`
**Pattern:** `process-flow` or `timeline-horizontal`
**Fix kind:** `swap_pattern`

A slide-level single-row sequence pattern — `process-flow`, or the default single-row `dots` style of `timeline-horizontal` — of 3–6 cells whose average per-cell text is below the sparse threshold (~40 chars) and which has no `bounds` / `max_height_pct` cap. Its lone row is left to fill the slide's content area, so the boxes stretch vertically into oversized shapes around a few words (the diamond/box aspect ratio drives the row height).

The check reads `slide.pattern` directly, so it fires only for a standalone slide-level pattern. Compose envelopes and nested cell patterns are exempt — a second zone already absorbs the slide height. The multi-row `chevron` and `gantt` timeline styles are also exempt (they are not single-row).

The fix is a `swap_pattern` suggestion ranked toward `numbered-step-strip` (whose per-step detail zone fills the vertical space), with `process-grid-2row` (two parallel tracks) and `phase-roadmap` (dated milestones with descriptions) as alternatives. Setting `max_height_pct` on the existing pattern also clears the finding. `fix.params` carry `from`, `item_count`, `avg_chars`, `reason: "single_row_sparse"`, and `suggested: [{to, rationale}, …]`.

```json
{
  "pattern": "process-flow",
  "path": "/slides/3/pattern",
  "code": "SPARSE_SINGLE_ROW_FLOW",
  "message": "slide 4: process-flow is a single horizontal row of 4 sparse cells (avg 6 chars) with no height cap — boxes stretch to fill the slide; switch to numbered-step-strip / process-grid-2row / phase-roadmap, or set max_height_pct",
  "fix": { "kind": "swap_pattern", "params": { "from": "process-flow", "item_count": 4, "avg_chars": 6, "reason": "single_row_sparse", "suggested": [{ "to": "numbered-step-strip", "rationale": "ordered steps with a per-step detail zone fill the vertical space" }] } },
  "action": "review",
  "next_tool_call": { "tool": "recommend_pattern", "args_template": { "item_count": 0 } }
}
```

### `OVERTALL_FLOW_LANE`

**Action:** `review`
**Pattern:** `process-flow` or `timeline-horizontal`
**Fix kind:** `swap_pattern`
**Class:** `pattern_choice`

The complement to [`SPARSE_SINGLE_ROW_FLOW`](#sparse_single_row_flow): a slide-level `process-flow` / single-row `timeline-horizontal` whose estimated lane height exceeds ~50% of the content area with short average per-cell text (< ~40 chars), in the cases the sparse guard does **not** cover:

- a `max_height_pct` cap that is still too tall (≥ 50), or
- a 7–8 step row whose narrow boxes still stretch vertically (the sparse guard caps at 6 items).

The detector defers to `SPARSE_SINGLE_ROW_FLOW` whenever that guard owns the case (uncapped, 3–6 items), so the two never fire on the same slide. The lane height is estimated from `max_height_pct` / `bounds.height` when set, else ~100% (an uncapped single-row flow fills the content zone).

The fix is a `swap_pattern` suggestion toward `numbered-step-strip` (whose per-step detail zone fills the vertical space) or `process-grid-2row`; capping `max_height_pct` to ~35 also clears it. `fix.params` carry `from`, `item_count`, `avg_chars`, `lane_height_pct`, `reason: "overtall_flow_lane"`, and `suggested: [{to, rationale}, …]`.

```json
{
  "pattern": "process-flow",
  "path": "/slides/8/pattern",
  "code": "OVERTALL_FLOW_LANE",
  "message": "slide 9: process-flow lane of 7 sparse cells (avg 8 chars) occupies ~100% of the content height — the boxes stretch vertically around a few words; switch to numbered-step-strip / process-grid-2row, or cap max_height_pct to ~35",
  "fix": { "kind": "swap_pattern", "params": { "from": "process-flow", "item_count": 7, "avg_chars": 8, "lane_height_pct": 100, "reason": "overtall_flow_lane", "suggested": [{ "to": "numbered-step-strip", "rationale": "per-step detail zone fills the vertical space instead of stretching the boxes" }] } },
  "action": "review"
}
```

### `FLOW_DIAMOND_NO_CONTENT`

**Action:** `review`
**Pattern:** `process-flow`
**Fix kind:** `swap_pattern`
**Class:** `pattern_choice`

A standalone `process-flow` carries at least one decision diamond (`steps[].type: "decision"`) but has no supporting content zone. A lone single-row flow has nowhere to explain the yes/no branch outcomes a decision implies. The detector reads `slide.pattern` directly, so compose envelopes and nested cell patterns are exempt (a second zone carries the explanation).

The fix is a `swap_pattern` suggestion toward `numbered-step-strip` (per-step detail) or `compose` (pair the flow with an explanatory panel). `fix.params` carry `from`, `diamond_count`, `reason: "decision_without_branch_zone"`, and `suggested: [{to, rationale}, …]`.

```json
{
  "pattern": "process-flow",
  "path": "/slides/3/pattern",
  "code": "FLOW_DIAMOND_NO_CONTENT",
  "message": "slide 4: process-flow has 1 decision diamond(s) but no supporting content zone to explain the branch outcomes — a lone single-row flow cannot show the yes/no paths; add an explanatory zone via compose, or switch to numbered-step-strip with per-step detail",
  "fix": { "kind": "swap_pattern", "params": { "from": "process-flow", "diamond_count": 1, "reason": "decision_without_branch_zone", "suggested": [{ "to": "numbered-step-strip", "rationale": "per-step detail zone explains each decision outcome" }] } },
  "action": "review"
}
```

### `TOC_FLOWCHART_VOCAB`

**Action:** `review`
**Pattern:** `process-flow` / `process-flow-compact` / `swimlane` / `timeline-horizontal`
**Fix kind:** `swap_pattern`
**Class:** `pattern_choice`

An agenda / table-of-contents slide is drawn with sequential flowchart vocabulary. The slide's title must match the agenda vocabulary (`agenda`, `table of contents`, `what we'll cover` / `what we will cover`) **and** the slide-level pattern must be one of the flowchart families. A contents list is not a sequence with arrows; the flowchart vocabulary implies a causal/temporal order the agenda does not have.

The fix is a `swap_pattern` suggestion toward `agenda` (numbered section list) or `numbered-step-strip` in `toc` style. `fix.params` carry `from`, `reason: "toc_as_flowchart"`, and `suggested: [{to, rationale}, …]`.

```json
{
  "pattern": "process-flow",
  "path": "/slides/1/pattern",
  "code": "TOC_FLOWCHART_VOCAB",
  "message": "slide 2: agenda / table-of-contents slide (\"Agenda\") is drawn with process-flow flowchart vocabulary — a contents list is not a sequence with arrows; use the agenda pattern or numbered-step-strip in 'toc' style",
  "fix": { "kind": "swap_pattern", "params": { "from": "process-flow", "reason": "toc_as_flowchart", "suggested": [{ "to": "agenda", "rationale": "numbered section list is the canonical agenda / table-of-contents layout" }] } },
  "action": "review"
}
```

### `MATRIX_AXIS_IMBALANCE`

**Action:** `review`
**Pattern:** `shape_grid`
**Fix kind:** `autofix_visual`
**Class:** `rendering`

A `shape_grid` cell whose text-bearing shape is rotated within ~15° of 90°/270° and spans rows or columns (an axis band). Rotating the whole band flips its width/height about its center, so a narrow-tall axis band renders wide-short (or vice versa) and intrudes into the adjacent quadrants/cells — the J2P-MATRIX-005 anti-pattern. This is a rendering-geometry smell, not a pattern-choice one: `matrix-2x2` now renders axis labels with `vert270` text direction in an **unrotated** band, so the check guards against regressions and hand-authored rotated bands.

The fix is `autofix_visual`: set the band shape's rotation to 0 and rotate only the text via `vert: "vert270"` so the fill geometry is never transformed. `fix.params` carry `reason: "rotated_band_aspect_flip"`, `rotation_deg`, and `guidance`.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/7/shape_grid/rows/1/cells/0/shape",
  "code": "MATRIX_AXIS_IMBALANCE",
  "message": "slide 8: a spanning text band is rotated 270° — rotating the band flips its width/height about its center, so it renders wide-short (or tall-narrow) and intrudes into the adjacent cells; render the label with vert text direction (vert270) in an unrotated band instead of rotating the shape",
  "fix": { "kind": "autofix_visual", "params": { "reason": "rotated_band_aspect_flip", "rotation_deg": 270, "guidance": "set the band shape rotation to 0 and rotate only the text via vert=\"vert270\" (vertical text direction) so the fill geometry is never transformed" } },
  "action": "review"
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

**Action:** `review` (render time); `refuse` (validate / preview time, shape_grid and pattern diagram cells)
**Fix kind:** `review` (no auto-fix)
**Emitted at:** render time; validate / preview time via chart dry-render

Diagram rendering failed entirely; a placeholder image was inserted instead.

At validate / preview time the chart dry-render emits this code with `action: refuse` for diagrams inside a `shape_grid` cell or a pattern-expanded grid (e.g. `chart-insights-split`), because generate aborts the whole deck on those instead of inserting a placeholder. The path is the cell's JSON Pointer (`/slides/{i}/shape_grid/rows/{r}/cells/{c}/diagram`, or `/slides/{i}/pattern/rows/{r}/cells/{c}/diagram` for pattern-embedded charts) and `fix.params.reason` carries the svggen error generate would report. This is review-only — no deterministic auto-fix is available. The agent must inspect the diagram data and decide whether to simplify the diagram, change its type, or regenerate the slide.

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
**Pattern:** `table`, `shape_grid`
**Fix kind:** `split_at_row` (table data cells), `reduce_text` (table headers), `reduce_cell_text` (shape_grid cells), `increase_row_height` (advisory; shape_grid rows)

Text exceeds the height available to it. The `split_at_row` fix includes a `row` parameter suggesting where to split the table.

On a **shape_grid cell** it is emitted only when the wrapped text does not fit **even at the smallest autofit shrink the renderer applies** (`textcapacity.AutofitFloorScale`, 20%) — i.e. text is genuinely clipped. A cell the renderer merely shrinks into (density >110% but `fits`) renders every word; the defect there is the resulting size, reported as `TEXT_BELOW_READABLE_MIN` at `review`. Reporting the shrink itself as `refuse` refused the project's own showcase decks — `business-model-canvas` scored 62 with 4 refusals on cells whose bullets are fully visible (go-slide-creator-lmpu). The fix is `reduce_cell_text` with the cell's own `cell_path` and `max_chars` budget — apply it verbatim. `reduce_text` walks *content items* and cannot reach grid text, so aiming it at a cell answers `code: "wrong_kind_for_target"` with `did_you_mean: "reduce_cell_text"` and a corrected `next_tool_call` (go-slide-creator-9zof). A **grid row** whose content exceeds its `max_height` is not itself a text target: its finding carries the advisory `increase_row_height` (guidance + `reshape_grid` / `reduce_cell_text` alternatives), while each overfull cell in it carries its own executable fix.

```json
{
  "pattern": "shape_grid",
  "path": "/slides/0/shape_grid/rows/0/cells/0/shape/text",
  "code": "fit_overflow",
  "message": "text needs 174 chars @ 11pt; cell allows 90 (193% of capacity)",
  "fix": { "kind": "reduce_cell_text", "params": { "cell_path": "/slides/0/shape_grid/rows/0/cells/0", "max_chars": 90 } },
  "action": "refuse"
}
```

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

The 35–110% range is the healthy zone — no finding is emitted. Above 110%, see `fit_overflow`.

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

### `chrome_band_no_fit`

**Action:** `review`
**Pattern:** *(none — slide-level)*
**Fix kind:** `swap_layout` (`params.layout_id` = the template's One Content layout; omitted when the slide is already on it)

Emitted by `validate_input`, the CLI dry-run / `validate -fit-report`, and every other preflight surface that runs the structural detectors when a slide carries a `takeaway` or `source` whose band cannot be placed on the slide's layout. The band stack is derived from the template profile (`template.ResolveChromeFrame`), not slide-size percentages:

- **Horizontal extent** = the union of the layout's body / content / picture / chart / table placeholders (`x .. x+cx`); layouts without one borrow the One Content layout's column.
- **Bottom** = the top of the layout's footer chrome (min Y of the visible `dt` / `ftr` / `sldNum` placeholders, layout first, else slide master) minus a gap. Without footer placeholders the bottom margin line is used.
- The source note sits at the bottom, the takeaway above it; body/chart/table placeholders on the slide are shrunk so they stop above the band.

The band **does not fit** when the stack would climb into the title, or — on layouts with their own content placeholders — would leave under 20% of the slide height for content. Generation then **skips the band** (and logs a warning) rather than overlapping the title or footers. Move the slide to a roomier layout, drop the source note, or fold the takeaway into the title/body.

```json
{
  "path": "/slides/2/takeaway",
  "code": "chrome_band_no_fit",
  "message": "slide 3: the takeaway/source band does not fit layout \"slideLayout4\" (it would overlap the title or leave too little room for content); the band is skipped at render",
  "fix": { "kind": "swap_layout", "params": { "layout_id": "slideLayout2" } },
  "action": "review"
}
```

The resolved frame per layout (content area, takeaway band, source band, footer top, `fits`) is visible up front in `examine_template`'s `layouts[].profile_geometry`.

### `takeaway_missing`

**Action:** `review`
**Pattern:** *(none — slide-level)*
**Fix kind:** `provide_value`

Emitted by `validate_input` and the CLI dry-run when a slide argues from data but says nothing about what the data means. The takeaway is the slide's headline answer — a single sentence that tells the audience the "so what". A chart or 2x2 without one forces the audience to derive the argument themselves, which they rarely do correctly.

Triggers when **all** of the following hold:

- `slide.takeaway` is empty (or whitespace-only)
- The slide has at least one of: a `chart` content item, a `diagram` content item whose `diagram_value.type` is chart-shaped (bar, line, area, scatter, bubble, pie, donut, stacked_bar, grouped_bar, waterfall, funnel, radar, gauge, treemap), or a **pattern marked `data_visual` in its taxonomy**
- The slide **title** is not itself the takeaway — a title of six words or more containing a finite verb ("Prioritise the four initiatives in the top-right quadrant") suppresses the finding, because asking for a second sentence saying the same thing is noise

`data_visual` is the pattern's own declaration that its job is to argue from data, exposed on `PatternTaxonomy` and returned by `show_pattern` / `list_patterns`. It currently covers `chart-insights-split`, `waterfall-bridge`, `horizontal-bar-with-callouts`, `table-highlight` and `matrix-2x2`. It replaced a `matrix-` name-prefix test that fired on exactly one pattern and never on `chart-insights-split` — the pattern that embeds a chart — nor on the two charts drawn as shape grids (go-slide-creator-g2cy). It is deliberately narrower than `category: "data-display"`: a card grid and an icon row display content without making a quantitative claim, and a 2x2 makes one while being structural.

The verb test is biased toward false negatives: a title with a verb the check does not know keeps its nudge, which is the old behaviour, while a wrong suppression silently removes the signal.

The warning never blocks generation — the takeaway is advisory, not structural. Add a one-sentence `takeaway` to the slide; it renders as 14pt bold dark-gray text in the layout-derived band above the footer placeholders, above the source note row (see `chrome_band_no_fit`).

```json
{
  "path": "/slides/3/takeaway",
  "code": "takeaway_missing",
  "severity": "warning",
  "message": "slide 4: this slide argues from data — set a takeaway headline (or make the title a full sentence) so the audience knows the 'so what'",
  "fix": { "kind": "provide_value", "params": { "field": "takeaway" } },
  "action": "review"
}
```

### `HEADLINE_TOO_LONG`

**Action:** `review`
**Pattern:** *(none — content lint)*
**Fix kind:** `shorten_title`

Emitted when a title-class placeholder (`title`, `headline`, `ctrTitle`) carries more than 12 whitespace-separated words **and the title could not be measured against a resolved title placeholder**. Single-line headlines at 36–40pt fit roughly 12 words across a 16:9 slide; longer ones wrap or shrink. Advisory only — never blocks render.

The word count is a proxy for "does this headline wrap?". Where the answer has actually been measured — any slide whose layout resolves, which includes every slide with a `layout_id`, a canonical layout name, or a predictable `slide_type` — the measured verdict (`title_wraps` / `TITLE_OVERFLOW`) is the only title-length finding, and this check stands down. Before that an agent saw `title too long (106 chars, max 60)` from the quality score, `headline is 13 words; trim to 12 or fewer` from this lint, and `max_chars: 29` in the placeholder metadata — three numbers for one question, with no single target to aim at (go-slide-creator-jcph).

Mechanics:

- Word counting splits on whitespace via `strings.Fields`, so punctuation attached to a word does not inflate the count.
- The check runs on `text` content items whose `placeholder_id` matches a title slot; non-title text is checked against `BODY_TOO_LONG` instead.
- It is skipped for any title the measured check covered, so the two never report the same headline.
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

**The indent is what the renderer reads too.** A bullet's leading whitespace sets the paragraph's OOXML `lvl` (base level + depth, clamped at 4), and the whitespace is stripped from the text, so a nested bullet inherits the slide master's smaller size and secondary glyph. Both sides read the depth through the same parser (`pptx.BulletIndentDepth`) so the lint and the render cannot drift: this finding used to describe nesting the renderer ignored, emitting every paragraph at `lvl="0"` with the glyph left-aligned and the text pushed right, which read as a typo (go-slide-creator-gyfl).

```json
{
  "path": "/slides/4/content/body",
  "code": "BULLET_NESTING_DEEP",
  "message": "slide 5: bullets nest 3 levels; flatten to 2 or fewer — deep nesting reads as visual noise",
  "fix": { "kind": "reduce_text", "params": { "current_depth": 3, "max_depth": 2 } },
  "action": "review"
}
```

### `NUMBERED_LIST_NOT_APPLIED`

**Action:** `review`
**Pattern:** *(none — content lint)*
**Fix kind:** `renumber_bullets`

Emitted when a bullets list carries typed `"N. "` prefixes the renderer will NOT turn into OOXML auto-numbering, so the author's numbers print beside the layout's own bullet glyph: `• 1. First do this` (go-slide-creator-6or2).

A list is auto-numbered — `<a:buAutoNum type="arabicPeriod"/>` on every paragraph, with the typed prefixes removed from the text — only when EVERY entry carries a prefix and the numbers run 1, 2, 3 … with no gaps. That is the shape an author writing an ordered list produces, and it cannot be reached by accident. Anything else keeps the text verbatim and draws this finding:

- a partially numbered list (`["1. …", "…", "3. …"]`)
- a list that starts at another number, repeats one, or skips one
- a list of one item (a single line opening with a number is prose — `"2024. A big year"` — and is exempt entirely)

`repair_slide(kind: "renumber_bullets", params: {path})` renumbers the list from 1; `params: {path, strip: true}` removes the prefixes instead.

```json
{
  "path": "/slides/2/content/body",
  "code": "NUMBERED_LIST_NOT_APPLIED",
  "message": "slide 3: 2 of 3 bullets start with a typed \"N. \" but the list is not numbered 1..3, so the numbers print beside the layout's bullet glyph as a double marker — number every bullet from 1 (the engine then supplies the numbers) or drop the prefixes",
  "fix": { "kind": "renumber_bullets", "params": { "path": "/slides/2/content/body", "prefixed": 2, "total": 3, "expected_format": "1. , 2. , 3. …" } },
  "action": "review"
}
```

### `LOW_CONTRAST_HIGHLIGHT`

**Action:** `review`
**Pattern:** `value-chain`
**Fix kind:** *(none — an authoring choice)*

Emitted when a pattern's AUTHORED highlight colour does not read as a highlight against the structure it sits in: under 3:1 fill-vs-fill, the WCAG non-text bar, below which the two fills are one block of colour at any viewing distance.

Contrast between two scheme slots is template-dependent, which is what made this invisible: value-chain's old fixed default of `accent2` on `dk2` measures 3.21:1 on midnight-blue and 1.48:1 on warm-coral, where the highlighted step was indistinguishable from its neighbours (go-slide-creator-ah5s). The DEFAULT is now chosen by that measurement — the first of `accent1`, `accent2` … `accent6`, `lt2` that clears the bar — so it cannot fail; only an authored `highlight_color` can, and it is honoured rather than overridden.

```json
{
  "path": "/slides/1/pattern",
  "code": "LOW_CONTRAST_HIGHLIGHT",
  "message": "slide 2: value-chain: value-chain highlight_color \"accent2\" reads at 1.48:1 against the step fill (dk2) — below 3.0:1 the highlighted step is not distinguishable from its neighbours; omit highlight_color to let the engine pick an accent that clears the bar",
  "action": "review"
}
```

### `MISSING_ALT_TEXT`

**Action:** `review`
**Pattern:** *(none — accessibility lint)*
**Fix kind:** `provide_value`

Emitted when a visual has no `alt` text (empty or whitespace-only): an image or icon asset sourced from `path`, `url`, or `svg_data`, or a chart, diagram or table. Bundled built-in icons referenced by `IconInput.name` are exempt — the qualified bundled name itself supplies an implicit caption (`preview_icon` returns it via the `alt` field).

The lint walks six surfaces:

- `slide.content[].image_value` (`ImageInput.Path` / `ImageInput.URL`)
- `slide.shape_grid.rows[].cells[].image` (`GridImageInput.Path` / `GridImageInput.URL`)
- `slide.shape_grid.rows[].cells[].icon` (cell-level `IconInput`)
- `slide.shape_grid.rows[].cells[].shape.icon` (shape-overlay `IconInput`)
- `slide.content[].chart_value` / `slide.content[].diagram_value`, and the `diagram` key of an author-written `shape_grid` cell
- `slide.content[].table_value`, and the `table` key of an author-written `shape_grid` cell

A chart, diagram or table always renders, with a description derived from its own payload when the author wrote none (`"Bar chart, Quarterly revenue ($M). 4 categories, 1 series (Revenue), values from 34 to 48."`, `"Process flow. 5 steps."`, `"Table, 4 columns by 5 rows. Columns: Segment, FY25 revenue, FY26 revenue, Change."`). The finding says the derivation is in use, not that the visual is broken — only the author knows what the visual is FOR.

**Pattern-expanded cells are exempt.** The fit-report walker expands named patterns into a `shape_grid` before the lint runs, so a `table-highlight` or `chart-insights-split` slide carries cells the author never wrote and cannot annotate. Grid-level `diagram` and `table` cells are therefore only checked on slides whose `shape_grid` is the author's own. Grid `image` and `icon` cells need no such gate — a pattern's placeholder assets carry no `path` / `url` / `svg_data`, so the source test already exempts them.

The finding never blocks render — it appears as a `review` action so agents that optimize only for `passes validation` cannot ship visually-fine but accessibility-incomplete decks. Because the action is `review`, `score_deck` deducts 5 points per occurrence from the affected slide's score (and from the overall correctness axis), creating scoring pressure to set alt text.

`fix.params` carry the asset `kind` (`image_value`, `image`, or `icon`) and the `source` field (`path`, `url`, or `svg_data`) so an agent can route the fix to the right authoring surface. For a chart, diagram or table the `kind` is `chart` / `diagram` / `table` and `on` names the payload field to set `alt` on (`chart_value`, `diagram_value`, `table_value`, `diagram`, `table`) — there is no `source`, because the visual is generated rather than loaded.

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

```json
{
  "path": "/slides/0/content/1/diagram_value",
  "code": "MISSING_ALT_TEXT",
  "message": "slide 1: diagram has no alt text — a screen reader gets the description derived from its data; set alt to one sentence saying what it shows",
  "fix": {
    "kind": "provide_value",
    "params": { "field": "alt", "kind": "diagram", "on": "diagram_value" }
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

### `TEXT_EXCEEDS_SHAPE`

**Action:** `review`
**Pattern:** the slide's pattern (empty for raw `shape_grid`)
**Fix kind:** `reduce_text`
**Emitted at:** preflight (validate / preview / score), deterministic geometry

A word in a shape_grid shape's text is wider than the text rectangle the shape's preset geometry leaves after text insets, so the renderer breaks it mid-word or the shape outline clips it. The widest whitespace-delimited word of every paragraph (single glyphs such as arrows are ignored) is measured with the template body font at the size the renderer uses (authored size floored to 12pt, default 14pt, bold honoured) and compared with the geometry's text width per ECMA-376 `presetShapeDefinitions`: `chevron` keeps `w − 2·min(w,h)·adj`, `homePlate` `w − min(w,h)·adj/2`, `diamond` / `triangle` / `flowChartDecision` `w/2`, `ellipse` `w·0.707`, `hexagon` / `octagon` their inset rectangles, everything else the full width. Default OOXML insets (0.1" left/right) apply unless the text authors `inset_*`. Charts, tables and images are not measured.

Grids produced by named patterns are expanded first (paths rooted at `/slides/{i}/pattern`), and nested sub-grids are resolved inside their parent cell like the renderer does. One finding is emitted per slide: `path` is the first offending cell, `fix.params.cells` lists every offending cell path, and `word` / `required_pt` / `available_pt` / `geometry` describe the worst case. Typical trigger: `numbered-step-strip` `style: "chevron"` labels, whose notches leave almost no text width.

```json
{
  "pattern": "numbered-step-strip",
  "path": "/slides/6/pattern/rows/0/cells/0/shape/text",
  "code": "TEXT_EXCEEDS_SHAPE",
  "message": "5 shapes have words wider than their text area (Attract, Convert, Onboard, Retain, Advocate); worst: \"Advocate\" needs 59pt but the chevron shape leaves 0pt of text width after geometry and insets — it will break mid-word or be clipped",
  "fix": { "kind": "reduce_text", "params": { "cells": ["/slides/6/pattern/rows/0/cells/0/shape/text", "…"], "word": "Advocate", "required_pt": 59.2, "available_pt": 0, "geometry": "chevron", "hint": "shorten the label, lower text size, or use a geometry with a wider text area (rect/homePlate instead of chevron)" } },
  "action": "review",
  "measured": { "width_emu": 751840, "height_emu": 0 },
  "allowed": { "width_emu": 0, "height_emu": 0 }
}
```

### `PATTERN_CONTENT_MISMATCH`

**Action:** `review`
**Pattern:** the pattern that was asked for
**Fix kind:** `swap_pattern` (`params: {from, to}`)
**Emitted at:** preflight, from the authored `slides[].pattern.values`

Every other check asks whether the content **fits**. This one asks whether it **belongs**. The calibration corpus's `B07_wrong_pattern` draws three KPIs as a timeline and a four-month implementation plan as a 2×2 quadrant matrix: the field counts are right, nothing overflows, and a human graded the deck 30/100 while every geometric detector saw a clean slide.

Three rules, each requiring **every** item to match so a single odd entry cannot trip it:

| Pattern | Fires when | Swap to |
|---------|-----------|---------|
| `timeline-horizontal` | 3+ stops where every `label` is a measurement (`$48M`, `118%`, `41d`) and no `date` is a period — the axes are inverted | `kpi-Nup` sized to the stop count |
| `matrix-2x2` | all four quadrant headers name points in time (`January`, `Q3`, `2026`) — a 2×2 shows two dimensions crossing, and dates run along one | `phase-roadmap` |
| `process-flow` / `process-flow-compact` / `pyramid` | 3+ items where every step label or tier is a short (≤3-word) label carrying a figure (`EMEA $12M`) — measurements side by side, not stages that follow one another | `kpi-Nup` sized to the item count |

What deliberately does **not** fire, because a false mismatch on a conforming deck is worse than a miss (the previous attempt at this signal, `pattern_overcrowded`, fired on every conforming timeline and 2×2 alike — go-slide-creator-wrsb):

- A **bare number** is a count, not a measure: a timeline stop reading `2026` or `3` is fine.
- A timeline of metrics against **real periods** (`$12M` at `Q1`) is a trend, and is correct.
- **One** named stop among measurements, or one non-date quadrant among three dates, clears the rule.
- A process step that **mentions** a figure in prose ("Collect the first $1M in deposits") is still a step; only short label-plus-figure items count.
- Fewer than 3 items is too few to judge.

```json
{
  "pattern": "timeline-horizontal",
  "path": "/slides/2/pattern",
  "code": "PATTERN_CONTENT_MISMATCH",
  "message": "slide 3: every stop on this timeline is a measurement (\"$48M\") against a label that is not a period (\"Revenue\") — this is a set of metrics, not a sequence in time",
  "fix": { "kind": "swap_pattern", "params": { "from": "timeline-horizontal", "to": "kpi-3up" } },
  "action": "review"
}
```

### `SPARSE_FILL`

**Action:** `review`
**Pattern:** the slide's pattern (empty for raw `shape_grid`)
**Fix kind:** `add_detail_or_resize`
**Emitted at:** preflight, deterministic geometry

A filled shape covering more than **10% of the slide area** holds text whose estimated wrapped block (word-wrapped at the shape's text width, 1.2 line height) covers less than **20% of the shape** — a large, mostly empty coloured box (e.g. `kpi-3up` cards with one number, tall process boxes with a two-word label). Fills of `none` / transparent, alpha below 20%, or the background colours `lt1` / `bg1` / white do not count as filled. One finding per slide; `fix.params.cells` lists the offending shapes and `max_text_area_pct` the densest of them. Fix by adding detail, capping the grid height (`bounds` / `max_height_pct`), or switching to a compact / unfilled variant.

```json
{
  "pattern": "kpi-3up",
  "path": "/slides/8/pattern/rows/0/cells/0/shape",
  "code": "SPARSE_FILL",
  "message": "3 filled shape(s) each cover >10% of the slide (up to 19%) but their text fills only 7–13% of the box — large, mostly empty blocks",
  "fix": { "kind": "add_detail_or_resize", "params": { "cells": ["/slides/8/pattern/rows/0/cells/0/shape", "…"], "max_text_area_pct": 13, "hint": "add detail, cap the grid height (bounds / max_height_pct), or use a compact / unfilled variant" } },
  "action": "review"
}
```

### `SLIDE_UNDERUSED`

**Action:** `review`
**Pattern:** the slide's pattern (empty for raw `shape_grid`)
**Fix kind:** `add_detail_or_resize`
**Emitted at:** preflight, deterministic geometry

The bounding box of the slide's grid "ink" covers too little of the safe content area (the layout's content zone below the title, as used for `bounds_relative_to_content_area`). Ink is every filled shape, every table / image / icon / diagram / composite cell, accent bars, and — for unfilled text shapes — the estimated text block placed by the text's `align` / `vertical_align`. Slides that also put content into a non-title placeholder are skipped (the grid then shares the area); a near-empty placeholder slide is `SLIDE_NEARLY_EMPTY`'s business, not this one.

**The threshold depends on who chose the band's height** (`fix.params.band_capped_by`):

| `band_capped_by` | threshold | why |
|---|---|---|
| `author` | 45% | the slide sets `bounds` or `max_height_pct`, so the height is an authoring decision and "the cap is too tight" is actionable |
| `pattern` | 22% | the pattern derived its height from its content ([go-slide-creator-7km8](SCHEMA_CHANGELOG.md)), so it is SUPPOSED to be shorter than the zone — only a genuine sliver is worth reporting, and the advice is about content, never about a cap the author never set |

Fires for e.g. an insights-only `chart-insights-split`, a `kpi-inline` capped to a thin band, or a four-stop `timeline-horizontal` (10% of the zone).

```json
{
  "path": "/slides/3",
  "code": "SLIDE_UNDERUSED",
  "message": "slide content covers 12% of the safe content area (threshold 45%) — the bounds / max_height_pct cap on this slide leaves it a thin strip",
  "fix": { "kind": "add_detail_or_resize", "params": { "content_area_pct": 12, "threshold_pct": 45, "band_capped_by": "author", "hint": "raise or remove the bounds / max_height_pct cap on this slide, add a supporting zone, or merge with another slide" } },
  "action": "review"
}
```

### `CONTENT_DROPPED`

**Action:** `review`
**Pattern:** *(none — shared content-drop diagnostic)*
**Fix kind:** `review` (no deterministic auto-fix)

The single, shared signal for **any** path that fails to place author-provided content. Today several paths can drop content — a slide skipped in `--partial` mode, a content block with no available placeholder, a column that did not fit, an unknown payload field stripped during semantic parsing. Each used to drop silently (or only as a free-text warning); `CONTENT_DROPPED` turns every such drop into one consistent, machine-actionable finding so an agent sees the loss and can repair it.

The drop has *already happened* by the time the finding is emitted — it is advisory and never blocks generation (action `review`). The fix carries `params.locator` (a short label for what was dropped — `"slide 4"`, `"content block 3"`, `"left column"`) and `params.reason` (why placement failed) so an agent can route the repair without re-deriving the cause. There is no single deterministic auto-fix (fix kind `review`, mirroring `diagram_render_failed`): the agent restructures or splits the slide, or fixes the underlying spec error.

Emitted today from the partial-mode slide-skip path in slide conversion, from the **multi-visual collision** path in image preparation — when two or more visual content blocks (chart / table / image / diagram) resolve to the *same* placeholder, only the first is rendered and each subsequent one is dropped (rather than silently overlapping the first at identical bounds), each with its own `CONTENT_DROPPED` finding pointing at `/slides/{i}/content/{n}` and a `reason` suggesting the author split the slide or use `compose` to give each visual its own region — and from the **missing-placeholder** path (see below). Other drop paths (dense-pattern section-divider overflow) adopt the same `patterns.ContentDropped(path, locator, reason)` constructor as they are hardened.

#### Hard drops: `params.cause = "placeholder_not_found"`

One drop cause is **not** advisory. When a content block targets a `placeholder_id` the resolved layout does not declare, the content is simply absent from the artifact: the render "succeeds" and the slide is missing the author's image or copy. Those findings carry `fix.params.cause = "placeholder_not_found"` plus `placeholder_id`, `layout_id` and the `available` placeholder ids, and they change two things in the response:

- **`success` is `false`** under `output_validation: "strict"` (the default). Under `warn` / `off` the finding is still emitted but `success` stays `true`.
- **`placeholders_used` excludes the dropped placeholder**, which instead appears in the new `placeholders_dropped` array on that slide's entry in `slides[]`. Previously `placeholders_used` echoed the *requested* ids, so it claimed a placeholder that was never populated.

Every other `CONTENT_DROPPED` cause (partial-mode slide skips, visual collisions) stays advisory and leaves `success` untouched.

```json
{
  "path": "/slides/3/content/1",
  "code": "CONTENT_DROPPED",
  "message": "author-provided content dropped (content block 2 (image)): placeholder_id \"body\" does not exist in layout \"slideLayout5\", so the content was not rendered",
  "fix": {
    "kind": "review",
    "params": {
      "cause": "placeholder_not_found",
      "placeholder_id": "body",
      "layout_id": "slideLayout5",
      "available": ["title", "subtitle"],
      "locator": "content block 2 (image)",
      "reason": "placeholder_id \"body\" does not exist in layout \"slideLayout5\", so the content was not rendered"
    }
  },
  "action": "review"
}
```

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

### `UNSUPPORTED_INLINE_MARKUP`

**Action:** `review`
**Pattern:** `inline_markup`
**Fix kind:** `remove_key`

The renderer understands a fixed inline-markup vocabulary — `<b>`, `<i>`, `<u>`, `<sup>`, `<sub>` — and passes anything else straight through to the text run, so an unsupported tag **prints literally on the slide**. This used to be silent: a deck using `<a>`, `<color>` or `<code>` validated clean and shipped visible XML-ish garbage.

The finding is advisory (`review`): the deck renders, it just renders the tag as text. It fires once per offending string, naming every distinct unsupported tag in it. `fix.params` carries both `unsupported` (what was found) and `supported` (the full vocabulary) so an agent can repair without a second lookup.

The scan walks **every** authored string in the input — typed fields, raw overrides, shape grids, pattern values — via the shared `internal/policy/textwalk` traversal the no-emoji policy uses, so it cannot miss a field that policy already reaches. Strings with no `<` are skipped, so ordinary prose costs nothing.

`<sup>` / `<sub>` are supported precisely because footnote markers are near-universal in consulting decks; they render as real OOXML baseline shifts (`a:rPr baseline="30000"` / `"-25000"`), not as text.

```json
{
  "path": "slides[0].content[1].bullets_value[1]",
  "code": "UNSUPPORTED_INLINE_MARKUP",
  "message": "slides[0].content[1].bullets_value[1] uses inline tag(s) <a>, <code>, <color> which the renderer does not support — they print literally on the slide; supported tags are <b>, <i>, <u>, <sup>, <sub>",
  "fix": {
    "kind": "remove_key",
    "params": {
      "path": "slides[0].content[1].bullets_value[1]",
      "unsupported": ["a", "code", "color"],
      "supported": ["b", "i", "u", "sup", "sub"],
      "hint": "remove the tag, or express the intent with a supported one (a footnote marker is <sup>1</sup>)"
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

The detector walks shape-grid cells that author both a fill color (on the shape) and a text color (on `shape.text` or on a paragraph in `shape.text.paragraphs`). Scheme names (`accent1`, `lt1`, …) are resolved against the template theme. Object-form fills that carry `lumMod` / `lumOff` / `alpha` modifiers (the light tints patterns paint behind highlighted rows and callouts) are composed into their effective colour over `lt1` first, so dark text on a light accent tint is judged against the tint rather than the untinted accent. Pairs that cannot be parsed are skipped.

The detector also walks **placeholder text on a background the slide sets itself** (`source: "slide_background"`). Nothing in the JSON names that text's colour — it is the template's, resolved from the layout placeholder — and only the background is the author's, so the shape-grid walk never saw the case: a dark statement slide validated clean and then reported a `contrast_autofixed` swap at generate time. The background is resolved by the renderer's own rule (`background.color`, or the scrim colour when an overlay is opaque enough to decide what the text sits on); a slide with `contrast_check: false`, or one whose background comes from its layout, draws nothing. A layout that leaves its placeholder colour to the master's `txStyles` is still only reported at generate time, as `source: "master-txStyles"`.

`fix.params.replacement_mode` discloses which branch of the algorithm produced the color:

- `flip` — the foreground is a pure neutral (literal white/black, or scheme `lt1`/`bg1`/`dk1`/`tx1`); it is snapped to the first template color that meets WCAG AA normal (4.5:1), tried in order `lt1`, `dk2`, `dk1`, then a darker/lighter shade of the fill hue (falling back to the first meeting AA large 3:1). Preferring `dk2` keeps the fix inside the template palette instead of literal `#000000`. This is why white text on a light accent predicts a clean dark theme color rather than a muddy mid-gray. Render-time swaps are logged at `WARN` with before/after colors and ratios.
- `lerp` — any other foreground; it is darkened or lightened toward black/white via `EnsureContrast` until it clears the ratio its text size requires.

**Provenance decides which branch runs** (go-slide-creator-s7wmh). On a background the AUTHOR introduced — `background.color`, or an opaque scrim — the template's text colour was never chosen against it, so there is no hue intent to preserve and the fix `flip`s to the palette whatever the foreground was. On the template's own background the hue is intentional and the fix stays a `lerp`. A `shape_grid` cell is not an author background in this sense: the author chose the fill and the text colour together, so those keep the `lerp`. Before this rule a navy title on an author-set black lerped to a mid-grey at exactly the WCAG floor and rendered barely visible; it now snaps to the template's `lt1` (ratio 1.5 → 21).

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

Text color was automatically replaced to meet WCAG AA contrast requirements against the resolved background. For `shape_grid` cells the background is the *effective* fill: `alpha` (composited over the theme `lt1`) and `lumMod`/`lumOff` modifiers on the solid fill are applied before the ratio is computed, so a light accent tint is not mistaken for the saturated base accent. This is informational — the fix has already been applied. The `fix.params` include the original and replacement colors, the background color, the contrast ratios before and after the swap, and the text surface `source`.

The finding's `path` locates the swap so an agent can map it back to the offending element. For `shape_grid` cell swaps the path is the flat rendered-shape index `"/slides/{i}/shape_grid/shapes/{n}"` (the original grid row/cell coordinates are not retained on the raw shape XML at render time). For template layout text (the `lstStyle`/`run` sources) the path is the slide-level `"/slides/{i}"`. Deck chrome — the footer line and the page number — is reported at `"/slides/{i}/chrome"` with `source: "chrome"`. The owning slide index is derived from `path` like every other finding. `fix.params.source` names the surface: `shape_grid`, `shape_grid_group`, `lstStyle`, `run`, `chrome`, `layout-lstStyle` or `master-txStyles`.

`shape_grid_group` is one decision covering several sibling cells (go-slide-creator-tnx3e): cells that start from the same text colour AND whose fills are one visual family (pairwise contrast ≤ 3:1 — progressive tints of an accent, say) are decided together, against the worst fill in the group, so a tinted stack cannot come out white on one tier and black on the next. `fix.params.cells` counts the cells the decision moved, the path is the grid (`/slides/{i}/shape_grid`) rather than a single shape, and the ratios are measured against that worst fill. The candidate order prefers a tonal shade of the fill's own hue over the palette's near-black when the palette misses the bar narrowly. Cells whose fills are not alike keep their per-cell decisions.

The last two are INHERITED text (go-slide-creator-ucmgr): a placeholder whose runs and list style state no color at all takes it from the layout's placeholder `lstStyle`, or failing that from the master's `txStyles`, in both cases through the layout's `clrMapOvr`. The earlier pass could not see that case — there was no color in the slide to rewrite — so a section divider whose override turns the master's `tx1` into white put white bullets on a white background and reported nothing. The resolved color is now checked against the background the slide actually shows, and when it fails, an explicit color is written onto the slide's runs and the swap is reported at `/slides/{i}` like any other layout-text swap.

A background the AUTHOR set — `background.color`, or a scrim opaque enough to decide what the text sits on — changes how the swap is chosen rather than whether one happens: the template's text colour was chosen against the template's background, so on the author's background there is no hue intent to preserve and the fix snaps to the palette (go-slide-creator-s7wmh). Template backgrounds and `shape_grid` cell fills are unaffected.

Chrome is injected as `schemeClr tx1`, so a layout that inverts its color map (`<a:overrideClrMapping tx1="lt1">`, as modern-template's section divider does) would otherwise draw the footer in the slide's own background color. Scheme colors are now resolved through the layout's override wherever the engine reasons about them — the background, the slide's text, and chrome alike — and chrome is pinned to an explicit palette color when the default would fall below WCAG AA (go-slide-creator-hln7). When nothing in the template's palette reads better than the default, no swap is claimed: the response carries a warning naming the layout and the ratio instead.

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

### `TEXT_BELOW_READABLE_MIN`

**Action:** `review`
**Fix kind:** `reduce_text` (`fix.params.strategy`: `shorten` or `split`)

Text ends up below the readability floor for its role in the deck's `viewing_mode` (go-slide-creator-vbic). Floors come from `tokens.MinReadableHPt`:

| Role | `present` (default) | `read` |
|------|--------------------|--------|
| title | 20pt | 20pt |
| body / card-title / card-body | 12pt | 10pt / 12pt / 9pt |
| caption (KPI labels, deltas, chips) / footnote | 10pt | 7pt |
| kpi-value | 18pt | 18pt |

Emitted from two fit sites, each tagging its text role:

- **Placeholders (validate AND generate):** measured autofit shrinks title / body text below the floor. Template-native sizes already below the floor are not reported (shortening would not change them). The policy never changes the fitted size — it reports rather than trimming. The preflight predicts the same scale as the render because both read the master's own `bodyStyle` — its size, its line spacing and its per-paragraph space-before. That last one decides it: fourteen bullets on an 8pt `spcBef` is 112pt of height, and a predictor that ignored it reported nothing at all where generate reported 10pt text (go-slide-creator-nlrg). The density thresholds that lower the floor for a long list (10+ and 12+ paragraphs) are one definition, `generator.AutofitDensityPolicy`, shared by both sides.
- **shape_grid cells (fit report / preflight):** the renderer writes cell text at its authored size with `<a:normAutofit/>` and shrinks every paragraph by one factor when the text overflows the cell. The fit report predicts that factor by measuring the cell's paragraphs in its text rectangle and reports the paragraph furthest below its floor. Roles are inferred per paragraph: ≥24pt → `kpi-value`, bold → `card-title`, ≤40 chars → `caption`, else `card-body`.

`fix.params`: `strategy` (`split` when the text has more than 3 paragraphs, else `shorten`), `role`, `actual_pt`, `min_pt`, `viewing_mode`.

```json
{
  "path": "/slides/2/shape_grid/rows/0/cells/0/shape/text",
  "code": "TEXT_BELOW_READABLE_MIN",
  "message": "caption text renders at 7.4pt, below the 10pt minimum for viewing_mode \"present\" (cell text overflows; autofit shrinks 12pt to ~7.4pt); shorten the text",
  "fix": { "kind": "reduce_text", "params": { "strategy": "shorten", "role": "caption", "actual_pt": 7.4, "min_pt": 10, "viewing_mode": "present" } },
  "action": "review"
}
```

## Scope Rules

Fit findings are scoped to **JSON-authored content only**. Content inherited from template layouts or masters is never checked.

### What is checked

- **Placeholder text** — body, content, and title placeholders populated from `slides[].content[]`
- **Shape grid cells** — shapes and tables authored in `slides[].shape_grid`
- **Pattern slides** — `slides[].pattern` is expanded once before the detectors run, so a named pattern is measured exactly like the equivalent hand-authored grid. Its findings are rooted at `/slides/N/pattern/rows/R/cells/C/...` (the deck has no `shape_grid` at that index) and any `reduce_cell_text` fix is replaced by the advisory `rewrite_field`, carrying the measured `max_chars` and the pattern name — there is no cell in the deck JSON to edit, so the remedy is to shorten the pattern's own values. Before go-slide-creator-adur the whole text-density and readability family skipped pattern slides, so the surface the skill recommends reported "no issues" on decks whose body text renders at 4-8pt.
- **Compose envelopes** — `slides[].compose` is expanded the same way
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

Responses are always compact JSON; the server still advertises `experimental.compact_responses: true` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.

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
