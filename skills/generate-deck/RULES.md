# Rules

Non-negotiable. Violating these causes broken or incorrect slides.

## Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]` | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5-row grid with `row_gap: 10` burns 40pt (~5% height). Tighten gaps before shrinking content |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, `diagram`, `composite`, `pattern`, or `grid` | Combining silently drops content. `composite` is the sole exception — it bundles a native text shape + sub-diagram inside one cell (see rule 6a). `pattern` and `grid` host a nested layout (see rule 6c) |
| 6a | `composite: {text: {...shape...}, sub_diagram: {...}, split: "top"\|"bottom", ratio: 0.0–1.0}` packs a native text shape and an embedded chart into one cell, split vertically. Use for KPI + sparkline, headline + mini chart, callout + small diagram. `split` defaults to `"top"` (text on top, diagram below); `ratio` defaults to 0.5 (text portion gets half the cell height). Composite cells must NOT also set `shape`/`table`/`icon`/`image`/`diagram` | Eliminates the "split each KPI into ≥2 adjacent cells with hand-tuned spans" hack. Resolves to two ResolvedCells sharing the same (row,col), so accent_bar/connectors target the pair as one cell |
| 6b | `card-grid` cells and `icon-row` items accept an optional `secondary: {type, values, categories?, color?}` slot. `type` is restricted to `sparkline`, `bar_chart`, or `line_chart`; `values` is a 2–12 element numeric array; `categories` (when set) must match `values` length. At most one secondary per cell. The pattern expands to a composite cell automatically — no need to drop to raw `shape_grid`. Example: `{"header": "Revenue", "body": "Q1–Q4 trend", "secondary": {"type": "sparkline", "values": [100, 120, 110, 145]}}` | Lets a card or icon-row cell host a small inline chart while keeping pattern-level validation (cell-count, headers, captions) intact |
| 6c | A grid cell may host a nested layout via `pattern: {name, values, overrides?, cell_overrides?}` (the same payload used at the slide level) or via `grid: {…ShapeGridInput…}` (a recursive sub-grid). The nested layout is rendered inside the cell rectangle with a small 4pt inset. `pattern` and `grid` are mutually exclusive with each other and with `shape`/`table`/`icon`/`image`/`diagram`/`composite` on the same cell. Accent inheritance follows the deck's `accent_strategy` — nested patterns see the same slide/section index as the parent. Example: a `matrix-2x2` with `pattern: {name: "kpi-3up", values: [...]}` in its bottom-right cell | Lets agents drop a `kpi-3up` into a quadrant or an `icon-row` into a `strategy-house` foundation row without escalating to slide-level `compose`. Cells hosting a nested layout become bounds-only `subgrid` placeholders; the nested cells are appended to the parent `ResolvedCell` list so overlay `anchor_cell` lookups still work |
| 7 | Body text cells MUST set all 4 insets (6-10pt each) | Without insets, text jams against shape edges |

## Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types use underscores: `stacked_bar`, `grouped_bar` | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |

## Content and Layout

| # | Rule | Rationale |
|---|---|---|
| 11 | `layout_id` must be a **canonical ID** — not a display name. Use one of: `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda`. Display names like `"Title Slide"` or `"One Content"` are **not valid** `layout_id` values and will fail to resolve | The engine resolves canonical IDs via tag-based matching (see `internal/layout/canonical.go`). Display names returned by `list_templates` `layout_names` are informational only — they show what the template provides, but `layout_id` must use the canonical form |
| 12 | Semantic fills (`accent1`, `lt2`, `dk1`) required; hex `#RRGGBB` forbidden unless in brand-color allowlist. **Never mix semantic and hex fills on the same slide.** Never use raw names like `"blue"` | Semantic colors adapt to template theme; use `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` for tints. Mixed hex+semantic on one slide breaks visual consistency and is always a bug |
| 13 | `align`: `"l"`, `"ctr"`, `"r"`, `"just"` | NOT `"left"`, `"center"`, `"right"` |
| 14 | `vertical_align`: `"t"`, `"ctr"`, `"b"` | NOT `"top"`, `"middle"`, `"bottom"` |
| 15 | Templates: `forest-green`, `midnight-blue`, `modern-template`, `warm-coral` | Inspect via `list_templates` (MCP) or `json2pptx skill-info` (CLI). Returns `canonical_layout_ids`, `color_roles`, `table_styles[]`, `white_text_safe`, `layout_names`, and `data_format_hints_digest`. Templates that fail analysis appear with an `error` field and no layout/theme data — do not use them for generation |

**`placeholder_id` per layout:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank` → `title` only (body goes in `shape_grid`); `section` → `title`, `body` (engine remaps `subtitle` → `body` with a `placeholder_remapped` finding). For authoritative per-template lists, use `json2pptx skill-info` or `list_templates` (MCP).

## Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | Engine auto-replaces low-contrast text with dark gray (WCAG AA, ratio < ~3.0). Auto-fixes are now visible — check `fit_findings` for `contrast_autofixed` entries (with before/after ratios) before deciding whether to re-author colors | White on `accent3`-`accent6` → surprise gray. Fix: use `accent1`/`accent2` fill, or `dk1` text, or `"contrast_check": false` (last resort — only when you've verified contrast manually) |

## Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

## Table Density (TDR — enforced, not advisory)

| # | Rule | Rationale |
|---|---|---|
| 20 | **MUST split** if rows > 7 OR cols > 6 OR font_size < 9pt. No exceptions. | Tables exceeding these limits overflow, clip, or become unreadable at presentation-viewing distance. Emit `split_slide` instead of cramming |

**Multiline cell counting.** A table cell containing `\n` or a comma-list with ≥3 items counts as N logical rows where N = max(line_count, ceil(comma_items / 1)). Apply this adjusted row count BEFORE the rows > 7 check. A 5-row table where 3 cells each contain 2 lines = 5 + 3 = 8 logical rows → must split.

**Refusal wording.** When TDR forces a split, emit exactly: *"This table has [N] logical rows × [M] columns; per Rule 20 I cannot fit this — emitting split_slide to distribute rows across slides."* Do not silently shrink fonts below 9pt to avoid the split.

Call `table_density_guide` (MCP) or run `json2pptx tables guide` (CLI) for detailed font size and row-count guidance when building table slides in shape grids. Pass `{template: "..."}` to scope results to a specific template's `table_styles[]`.

---

## Anti-patterns

### Two-tables-one-grid

Sibling tables stacked in the same `shape_grid` with `row_gap < 4pt` or a divider shape between them with height < 4% of slide height. This creates a visual collision — the tables read as one broken table.

Bad — two tables jammed together:
```json
{
  "rows": [
    {"cells": [{"table": {"headers": ["Q1","Q2"], "rows": [["10","20"]]}}]},
    {"height": 2, "cells": [{"shape": {"type": "rect", "fill": "accent1"}}]},
    {"cells": [{"table": {"headers": ["Q3","Q4"], "rows": [["30","40"]]}}]}
  ],
  "row_gap": 2
}
```

Good — separate slides or adequate spacing:
```json
{
  "rows": [
    {"cells": [{"table": {"headers": ["Q1","Q2"], "rows": [["10","20"]]}}]},
    {"height": 8, "cells": [{"shape": {"type": "rect", "fill": "accent1"}}]},
    {"cells": [{"table": {"headers": ["Q3","Q4"], "rows": [["30","40"]]}}]}
  ],
  "row_gap": 6
}
```
Or better: put each table on its own slide.

### Hex-fill mix

A slide containing both semantic fills (`accent1`, `lt2`, etc.) AND non-allowlisted `#RRGGBB` hex fills. This always indicates a mistake — either commit to semantic colors or to a documented brand palette, never both on one slide.

Bad — mixed fills on one slide:
```json
{
  "cells": [
    {"shape": {"fill": "accent1", "text": "Revenue"}},
    {"shape": {"fill": "#FF6B35", "text": "Costs"}}
  ]
}
```

Good — all semantic:
```json
{
  "cells": [
    {"shape": {"fill": "accent1", "text": "Revenue"}},
    {"shape": {"fill": "accent2", "text": "Costs"}}
  ]
}
```

### Pattern monotony (deck-level)

Generating N slides with the same pattern (e.g., 5 card-grids in a row) produces a visually flat deck. The audience cannot distinguish slides. This is the single most common agent mistake.

Bad — monotonous sequence:
```
Slide 2: card-grid — "Market Segments"
Slide 3: card-grid — "Product Lines"
Slide 4: card-grid — "Competitor Analysis"
Slide 5: card-grid — "Team Structure"
```

Good — varied sequence with rhythm breaks:
```
Slide 2: card-grid     — "Market Segments"
Slide 3: comparison-2col — "Product Lines"
Slide 4: stat-hero     — "Key Differentiator"     ← narrative break
Slide 5: matrix-2x2    — "Competitor Positioning"
Slide 6: icon-row      — "Team Strengths"
```

Rules: no pattern should appear 3+ times consecutively. Insert a narrative-break pattern (stat-hero, pull-quote) every ~5 slides. Use `analyze_deck_rhythm` to detect violations before generating.

### Accent monotony

Using the default `accent_strategy: "primary"` on a 10+ slide deck makes every shape the same color. Set `"rotate"` or `"section-keyed"` for longer decks, or manually assign different accents to shape fills.

---

## Cell Accent Variety

**Why it matters.** When every cell in a multi-cell grid uses the same accent color, the slide reads as a monochrome block — the audience cannot visually parse distinct items. Accent variety within a slide creates hierarchy and makes each cell scannable at presentation-viewing distance.

**The three modes.** Grid-shaped patterns expose a `cell_accent_mode` override that controls per-cell accent color variation. The mode operates on the resolved base accent (after `accent_strategy` has picked the slide-level accent):

| Mode | Behavior | When to use |
|------|----------|-------------|
| `uniform` (default) | Every cell uses the same base accent | Timelines, process flows, sequential steps — consistency aids comprehension of order |
| `alternate` | Cells alternate between base and base+1 (wraps at accent6→accent1) | Paired comparisons, two-tier hierarchies, before/after — distinguishes two groups |
| `progressive` | Each cell walks base, base+1, base+2, ... (wraps at accent6→accent1) | 4+ peer cells where differentiation matters — feature lists, benefit grids, KPI dashboards |

**Interaction with `accent_strategy`.** The deck-level `accent_strategy` resolves the base accent per slide (e.g., `section-keyed` assigns one accent per section). `cell_accent_mode` then walks *from* that base within the slide. Example: if `section-keyed` resolves slide 5 to `accent3` and `cell_accent_mode` is `progressive`, cells get `accent3`, `accent4`, `accent5`, `accent6`, `accent1`, `accent2`.

**Which patterns support it.** All grid-shaped patterns (those with multiple peer cells rendered by the shape grid engine) support `cell_accent_mode`. Non-grid patterns (single-cell heroes, axis-bound matrices, fixed-progression layouts) do not expose it because their accent logic is structurally determined. Use `show_pattern` or `list_patterns` to check whether a specific pattern's overrides schema includes `cell_accent_mode`.

**Anti-patterns:**

- All cells same accent on a slide with 4+ peer cells → use `progressive` to differentiate items visually.
- Mixing `alternate` with `section-keyed` accent strategy without validation → verify the result with `analyze_deck_rhythm` to confirm within-slide accent variety reads well against the section-level base.

**Validation loop.** After generating, check `analyze_deck_rhythm` for `within_slide_accent_variety` recommendations. If the tool flags low variety on a grid-heavy slide, set `cell_accent_mode: "progressive"` in the pattern's overrides and re-analyze.

---

## Charts: Subtitle vs Footnote

Charts accept both `subtitle` and `footnote` fields. Use `subtitle` for contextual text rendered below the chart title (e.g., "FY2024 Q1-Q4"). Use `footnote` for source attribution rendered at the chart bottom. These are separate fields routed to different render positions — do not use `footnote` when you mean `subtitle`.

## Font Availability

The SVG chart renderer (`svggen/`) requires at least one usable font at boot time. If the requested font, system fallbacks (Arial, Helvetica), and the embedded Liberation Sans font all fail to load, the renderer returns an error immediately rather than producing charts with missing text. This is a hard failure — no silent degradation.

## JSON Schema Validation

Input JSON is validated with `additionalProperties: false` at every object level. Unknown keys produce structured warnings identifying the unexpected field and its JSON path. This catches typos (e.g., `chart` instead of `chart_value`) and obsolete fields early, before generation.
