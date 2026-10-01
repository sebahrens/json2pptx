# Rules

Non-negotiable. Violating these causes broken or incorrect slides.

## Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]`. Widths (and row `height` / `flex`) must be finite and >= 0 and the widths must not all be 0 — `validate` rejects them | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5-row grid with `row_gap: 10` burns 40pt (~5% height). Tighten gaps before shrinking content |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, `diagram`, `composite`, `pattern`, or `grid` | Combining silently drops content. `composite` is the sole exception — it bundles a native text shape + sub-diagram inside one cell (see rule 6a). `pattern` and `grid` host a nested layout (see rule 6c) |
| 6a | `composite: {text: {...shape...}, sub_diagram: {...}, split: "top"\|"bottom", ratio: 0.0–1.0}` packs a native text shape and an embedded chart into one cell, split vertically. Use for KPI + sparkline, headline + mini chart, callout + small diagram. `split` defaults to `"top"` (text on top, diagram below); `ratio` defaults to 0.5 (text portion gets half the cell height). Composite cells must NOT also set `shape`/`table`/`icon`/`image`/`diagram` | Eliminates the "split each KPI into ≥2 adjacent cells with hand-tuned spans" hack. Resolves to two ResolvedCells sharing the same (row,col), so accent_bar/connectors target the pair as one cell |
| 6b | `card-grid` cells and `icon-row` items accept an optional `secondary: {type, values, categories?, color?}` slot. `type` is restricted to `sparkline`, `bar_chart`, or `line_chart`; `values` is a 2–12 element numeric array; `categories` (when set) must match `values` length. At most one secondary per cell. The pattern expands to a composite cell automatically — no need to drop to raw `shape_grid`. Example: `{"header": "Revenue", "body": "Q1–Q4 trend", "secondary": {"type": "sparkline", "values": [100, 120, 110, 145]}}` | Lets a card or icon-row cell host a small inline chart while keeping pattern-level validation (cell-count, headers, captions) intact |
| 6c | A grid cell may host a nested layout via `pattern: {name, values, overrides?, cell_overrides?}` (the same payload used at the slide level) or via `grid: {…ShapeGridInput…}` (a recursive sub-grid). The nested layout is rendered inside the cell rectangle with a small 4pt inset. `pattern` and `grid` are mutually exclusive with each other and with `shape`/`table`/`icon`/`image`/`diagram`/`composite` on the same cell. Accent inheritance follows the deck's `accent_strategy` — nested patterns see the same slide/section index as the parent. Example: a `matrix-2x2` with `pattern: {name: "kpi-3up", values: [...]}` in its bottom-right cell | Lets agents drop a `kpi-3up` into a quadrant or an `icon-row` into a `strategy-house` foundation row without escalating to slide-level `compose`. Cells hosting a nested layout become bounds-only `subgrid` placeholders; the nested cells are appended to the parent `ResolvedCell` list so overlay `anchor_cell` lookups still work |
| 6d | A raster `image` cell takes an optional `geometry`: `"rect"` (default) or `"ellipse"`. `"ellipse"` clips the cover-cropped picture to its frame's ellipse — pair it with `fit: "contain"` for a circular headshot | The picture is not edited; the p:pic frame preset changes. Anything other than `rect` / `ellipse` is a validation error. `contact-directory` uses it for its headshots |
| 6e | Row `connector: {style, color, width}` chains the visible cells of ONE row. For a flow that crosses rows (a swimlane hand-off) add grid-level `links: [{"from": [row, col], "to": [row, col], "connector": {...}}]`, each cell given by the row and grid column it starts in | Same-column links run straight down; other links turn in the column gutter, so give the grid a `col_gap` ≥ 10 for a visible arrow. Links to a spacer (no fill, no outline) are dropped |
| 7 | Do NOT set `inset_*`: every shape's text already keeps 0.5 cm from each edge (unfilled, left-aligned first-column text is pulled in to start on the title's text edge) | Overrides shrink that margin; size rows for it instead (~43pt per 12pt line) |

## Typography Hierarchy (shape_grid)

Do not invent font sizes. A deck uses one type scale; when generating `shape_grid` JSON, pick each size from it by role:

| Role                  | Size    | Weight  | Notes                                                              |
|-----------------------|---------|---------|--------------------------------------------------------------------|
| Display / title       | 28pt    | Regular | Display headings, divider titles; serif (major font) allowed       |
| Lead / banner         | 18pt    | Bold    | Grid header band, lead line                                        |
| Subhead / card title  | 14pt    | Bold    | Card titles (first line, separated by `\n`), column headers       |
| Body / card body      | 12pt    | Regular | Default body copy; the floor in `present` decks                    |
| Caption               | 10pt    | Regular | Only tables, dense matrices, legends and chart labels              |
| KPI value             | 40-48pt | Bold    | Display figures; measured to fit                                   |
| Source line           | 9pt     | Italic  | Engine-rendered from the slide `source` — never author it          |

Leave text insets at the 0.5 cm default. A size off this scale should be a deliberate, named-pattern override — never an ad-hoc choice. See [`docs/INPUT_FORMAT.md`](../../docs/INPUT_FORMAT.md) for full examples.

Every size in the table sits on the **type scale**: 28pt display / title, 18pt lead, 14pt subhead (bold card titles, grid headers), 12pt body, 10pt caption, plus a 40–48pt display step for KPI values; 11pt is the body minimum and 10pt is allowed only in tables and dense matrices. The engine settles off-scale `shape_grid` sizes onto the step at or below them (16pt renders at 14pt, 20pt at 18pt), keeps display figures and 28pt+ text as measured, and never goes below a `viewing_mode` readability floor: in `present` (the default) body and card text under 12pt and captions under 10pt are refused as `TEXT_BELOW_READABLE_MIN`; `read` decks have lower floors for on-screen reading, which are limits, not sizes to author. Short bold ALL-CAPS labels (headers, eyebrows, badges) are tracked +7% automatically; regular-weight caps such as initialisms in body cells are not. Serif (the template's major font) belongs to titles and display figures only; body text uses the minor font.

The table is the published surface of the type-scale tokens in [`internal/tokens/typography.go`](../../internal/tokens/typography.go) (`TypeScale*`, `SourceLineHPt`). A regression test (`internal/tokens/tokens_test.go`) verifies this table stays in sync with the constants — if you change one, change the other in the same PR.

## Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types use underscores: `stacked_bar`, `grouped_bar` | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |
| 10a | Visibly label the measure and supplied units; never invent units. Raw `bar_chart`: `title` and `data.y_label`/`data.y_axis_title`; otherwise check the live schema. | Use a title, axis, direct label or legend. `series.name`/`alt` alone may be invisible; an identified measure needs no redundant legend. |

## Slide Takeaway (the "so what" line)

| # | Rule | Rationale |
|---|---|---|
| 10b | Chart and matrix slides MUST set `slide.takeaway` (one sentence — the headline answer). | Omission emits `takeaway_missing`. It renders as 14pt bold dk1 text beside a flush 3pt accent1 bar (no fill, no outline) above the source/footer, spanning the body column with 16pt of air above and 12pt below; content frames shrink above it. Budget two lines. Without room, the band is skipped and preflight emits `chrome_band_no_fit`. |

```json
{
  "layout_id": "blank-title",
  "takeaway": "Margin contraction is driven by the EU region — not company-wide.",
  "pattern": { "name": "matrix-2x2", "values": { ... } }
}
```

## Content and Layout

| # | Rule | Rationale |
|---|---|---|
| 11 | `layout_id` must be a **canonical ID** — not a display name. The vocabulary includes `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank-title`, `blank-canvas`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda`, but optional roles are not present in every template. Check `list_templates.canonical_layout_availability` before pinning one. Display names like `"Title Slide"` or `"One Content"` are **not valid** `layout_id` values | The engine resolves canonical IDs via tag-based matching (see `internal/layout/canonical.go`). Every discovery projection's `canonical_layout_ids` contains only resolvable names; unavailable canonical names fail with a list of available alternatives |
| 11a | The two blank roles are distinct: `blank-title` = a title-only canvas (host a `shape_grid`/`pattern` below a title); `blank-canvas` = a truly empty layout with no template placeholders. A content-bearing raw `blank-canvas` must set slide-level `headline`; the engine renders it in a theme-aware reserved title band. `blank` is a legacy alias that resolves to `blank-title` (falling back to the empty Blank only when the template has no Blank + Title). For new decks, target a role explicitly with `blank-title` or `blank-canvas` | The explicit IDs map 1:1 to the `blank-title` / `blank` layout tags. An empty interstitial canvas may omit `headline`; a pattern/grid/compose/media canvas may not |
| 12 | Semantic fills (`accent1`, `lt2`, `dk1`) required; hex `#RRGGBB` forbidden unless in brand-color allowlist. **Never mix semantic and hex fills on the same slide.** Never use raw names like `"blue"`. `theme_override.colors` values must be exact 6-digit hex (`#1A2B3C` or `1A2B3C`); `#abc` shorthand or names like `"navy"` fail validation with `invalid_color` at `theme_override/colors/<slot>`; control characters in font names fail with `INVALID_PARAMETER` | Semantic colors adapt to template theme; use `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` for tints. Mixed hex+semantic on one slide breaks visual consistency and is always a bug |
| 13 | `align`: `"l"`, `"ctr"`, `"r"`, `"just"` | NOT `"left"`, `"center"`, `"right"` |
| 14 | `vertical_align`: `"t"`, `"ctr"`, `"b"` | NOT `"top"`, `"middle"`, `"bottom"` |
| 15 | Template names come from `list_templates` (MCP) or `json2pptx skill-info` (CLI); never assume a fixed list — the shipped set grows and a server may register more | Default MCP discovery returns `canonical_layout_ids`, `color_roles`, `table_styles[]`, `white_text_safe_body` (4.5:1), `white_text_safe_large` (3:1), `ink_on_accent` (readable ink per accent fill), and `data_format_hints_digest`; request `mode="compact"` or `fields="full"` for `layout_names`. Templates that fail analysis appear with an `error` field and no layout/theme data — do not use them for generation |

**`placeholder_id` per layout:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank-title` (and legacy `blank`) → `title` only (body goes in `shape_grid`); `blank-canvas` → no placeholders (all content via `shape_grid`/`pattern`); `section` → `title` (some templates use a body placeholder for decorative numbering, not supporting text). A section `subtitle` is unsupported: use a layout with a subtitle or a `shape_grid` text box. For authoritative per-template lists, use `json2pptx skill-info` or `list_templates` (MCP).

Section dividers are numbered automatically `01`, `02`, … in deck order. Omit the `Section Number` placeholder from authored content; `auto_filled: true` in template discovery marks it as engine-owned.

## Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | Engine auto-replaces low-contrast text (WCAG AA for its size: 3:1 only at ≥18pt / ≥14pt bold) with a template text color (`lt1`/`dk2`/`dk1`), one color per fill per slide. White / black inks are judged at the body's smallest text; a brand-coloured run (an accent KPI value, stat, insight header) is judged at its OWN size, so a ≥18pt (≥14pt bold) accent that clears 3:1 keeps the accent and one that misses is darkened minimally in its hue, not snapped to black. KPI peer cards on their neutral surface draw the big number in the accent. It keeps your color when a swap would barely raise the ratio, and never recolors text matching a transparent cell's canvas (hidden on purpose). Check `fit_findings` for `contrast_autofixed` (before/after ratios) before re-authoring colors. Pattern-drawn accent fills (process steps, pyramid tiers, panel headers, banners, badges) whose white label misses the bar are first deepened by a minimal `shade` of the same accent so the type stays white; only a pale accent or tint takes dark ink | White on light accents → dark text. Fix: a darker accent fill, `dk1` text, or `"contrast_check": false` (last resort, after checking contrast yourself) |

## Icons (no emoji)

| # | Rule | Rationale |
|---|---|---|
| 15a | **Never emit emoji codepoints anywhere in deck JSON.** Icon fields (`card-grid` cells, `icon-row` items, `herodetail` icon, raw `shape_grid` cell `icon`) MUST be a bundled icon name (preferred) or a loadable user icon via `path` / `url` / `svg_data`. Emoji glyphs (`🚀`, `📈`, `✅`, `⚡`, etc.) and pictographic characters in the Unicode emoji range are rejected by pattern validators and produce broken/off-brand output when they slip into text. Plain Unicode symbols outside the emoji range (`→`, `←`, `•`, en-dash) are still allowed in text. | Bundled SVG icons inherit theme colors and scale crisply; emoji rasterize at fixed sizes, fall back to system fonts (inconsistent across viewers), and break the design system |

Accepted `IconInput` sources, exactly one per icon: `name` (bundled), `path` (local file), `url` (remote), `svg_data` (inline SVG). Run `json2pptx icons list` (CLI) or call `list_icons` (MCP) for the bundled catalog.

## Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` (renders on one line across the dt+ftr footer width; over-long text shrinks to 8pt then ellipsizes — keep it short) | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

## Table Density (TDR — enforced, not advisory)

| # | Rule | Rationale |
|---|---|---|
| 20 | **MUST split** if rows > 7 OR cols > 6 OR font_size < 9pt. No exceptions. | Tables exceeding these limits overflow, clip, or become unreadable at presentation-viewing distance. Emit `split_slide` instead of cramming |

**Row width.** `headers` defines the table's columns. A row with fewer cells (counting `col_span`) is padded with empty cells; a row wider than `headers` is rejected by `validate` / `generate` with `INVALID_PARAMETER` at `...table_value.rows` (grid cells: `.../cells/N/table.rows`) ("table row N spans M columns ... but headers define K").

**Multiline cell counting.** A table cell containing `\n` or a comma-list with ≥3 items counts as N logical rows where N = max(line_count, ceil(comma_items / 1)). Apply this adjusted row count BEFORE the rows > 7 check. A 5-row table where 3 cells each contain 2 lines = 5 + 3 = 8 logical rows → must split.

**Refusal wording.** When TDR forces a split, emit exactly: *"This table has [N] logical rows × [M] columns; per Rule 20 I cannot fit this — emitting split_slide to distribute rows across slides."* Do not silently shrink fonts below 9pt to avoid the split.

**Default look.** A table with no `style` renders as a consulting table: no header fill (never a black header bar), 11pt bold header over a 1pt rule, 12pt rows separated by 0.5pt 15% hairlines, no zebra, bold first column, numeric columns right-aligned, content-height rows top-anchored under the title. Prefer it; add `header_background` / `borders` / `striped` only for a deliberate exception — each is additive: `header_background` only fills the header (text flips to light/dark by contrast) and keeps the 12pt/11pt type, hairline rules and unbanded rows; `borders` / `striped` opt in to grid lines / zebra individually. `table-highlight` uses the same look with an accent 10% tint and 3pt accent bar on the highlighted row.

Call `table_density_guide` (MCP) or run `json2pptx tables guide` (CLI, add `--json` for the structured envelope) for detailed font size and row-count guidance when building table slides in shape grids. Pass `{template: "..."}` (MCP) or `--template <name>` (CLI) to scope results to a specific template's `table_styles[]`. `style_id` narrows to one of those styles and requires `template`.

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

### Pattern and accent monotony (deck-level)

Moved to [WORKFLOW.md](WORKFLOW.md) → Phase 2 (Pattern monotony, Accent monotony).

### Sparse single-row flow

Filling a whole slide with one row of 3-6 short boxes — a lone `process-flow` or `timeline-horizontal` strip floating in whitespace — reads as unfinished. `process-flow` earns a full slide only when the sequence actually **branches** (decision diamonds); `timeline-horizontal` only for **true calendar milestones** with real dates. `recommend_visual` ranks `numbered-step-strip` first for a sequence with no decision language.

Bad — a straight 4-step chain stretched across a bare slide:
```
Slide 2: process-flow — "Intake → Route → Execute → Verify"   (no branches, no detail)
```

Good — give the sequence vertical mass and a named pattern that fits:
```
Slide 2: numbered-step-strip — 4 steps, each with a per-step detail zone
   (or value-chain for described steps · phase-roadmap for dated phases ·
    process-grid-2row for two aligned tracks · a shape_grid lane + detail zone)
```

If no named pattern fits, build a two-row `shape_grid`: a narrow numbered
lane above an aligned detail row with the same column count. Inspect the
expanded/rendered result; do not let a decorative strip stand in for the
actual step descriptions. Query `list_patterns` / `show_pattern` for the
current alternatives and value schemas.

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

**Which patterns support it.** All grid-shaped patterns (those with multiple peer cells rendered by the shape grid engine) support `cell_accent_mode`. Non-grid patterns (single-cell heroes, axis-bound matrices, fixed-progression layouts) do not expose it because their accent logic is structurally determined. Use `show_pattern` or `list_patterns` to check whether a specific pattern's overrides schema includes `cell_accent_mode`. `pyramid` supports it per tier.

**Only set overrides the schema lists.** `pyramid`, `process-flow` and `process-flow-compact` have no header text, so they do not publish `header_size` and reject it with `unknown_key` (fix `remove_key`); size their labels with `body_size`.

**Anti-patterns:**

- `progressive` on peer cells that are not ordered or graded data → keep `uniform`: one accent per slide is the consulting default, and accent belongs on the one cell the title is about.
- Mixing `alternate` with `section-keyed` accent strategy without validation → render and check the slide reads as one emphasis, not a rainbow.

**Validation loop.** `analyze_deck_rhythm` no longer asks for more accents. It flags accent *heaviness*: `accent_heavy_slide` (a raw grid with 4+ solid-accent cells — give the rest a neutral `lt2` / dk1-tint fill) and `strong_accent_run` (3+ consecutive slides using `accent_weight: "strong"` patterns — swap one for a normal or subtle pattern).

---

## Charts: Subtitle vs Footnote

`chart_value` accepts both `subtitle` and `footnote` (top-level keys beside `title`, not inside `data`). Use `subtitle` for contextual text rendered below the chart title inside the chart image (e.g., "FY2024 Q1-Q4", "$M"). Use `footnote` for source attribution rendered at the chart's bottom edge (e.g., "Source: Company filings, FY24"); a chart with a `footnote` counts as sourced for `DATA_WITHOUT_SOURCE`. These are separate fields routed to different render positions — do not use `footnote` when you mean `subtitle`. Bar, line, area, stacked, scatter, pie and donut charts draw the footnote; for other chart types, and for the slide-level source zone, set the slide's `source` instead.

## Font Availability

The SVG chart renderer (`svggen/`) requires at least one usable font at boot time. If the requested font, system fallbacks (Arial, Helvetica), and the embedded Liberation Sans font all fail to load, the renderer returns an error immediately rather than producing charts with missing text. This is a hard failure — no silent degradation.

## JSON Schema Validation

Input JSON is validated with `additionalProperties: false` at every object level. Unknown keys produce structured warnings identifying the unexpected field and its JSON path. This catches typos (e.g., `chart` instead of `chart_value`) and obsolete fields early, before generation.
