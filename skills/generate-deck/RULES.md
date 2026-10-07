# Rules

Non-negotiable. Violating these causes broken or incorrect slides.

## Native slide composition

Use open template-led layouts and substantial exhibits; group with whitespace
and rules. Reject decorative web UI. Review [composition and editability](../slide-visual-qa/SKILL.md#native-slide-composition)
in the rendered slides.

## Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]`. Widths (and row `height` / `flex`) must be finite and >= 0 and the widths must not all be 0 — `validate` rejects them | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5 rows with `row_gap: 10` burn 40pt (~5% height). |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, `diagram`, `composite`, `pattern`, or `grid` | Combining silently drops content |
| 6a | `composite: {text: {...shape...}, sub_diagram: {...}, split: "top"\|"bottom", ratio: 0.0–1.0}` packs a native text shape and an embedded chart into one cell, split vertically. `split` defaults to `"top"` (text above); `ratio` (text share) to 0.5. No other content key on the same cell | One cell instead of hand-tuned adjacent cells |
| 6b | `card-grid` cells and `icon-row` items accept one optional `secondary: {type, values, categories?, color?}`: `type` is `sparkline`, `bar` or `line`; `values` 2–12 numbers; `categories` (if set) the same length | Expands to a composite cell automatically, keeping pattern-level validation |
| 6c | A cell may host a nested layout via `pattern: {name, values, overrides?, cell_overrides?}` (the slide-level payload) or `grid: {…ShapeGridInput…}`, rendered inside the cell with a 4pt inset; exclusive with each other and every other content key. Nested patterns inherit the deck's `accent_strategy` at the parent's slide/section index | Avoids slide-level `compose` for one embedded block |
| 6d | A raster `image` cell takes optional `geometry`: `"rect"` (default) or `"ellipse"` (clips to an ellipse; pair with `fit: "contain"` for a circular headshot), and `image.fit`: `"cover"` (default, crops to fill) or `"contain"` (whole picture, centred — screenshots) | Other values fail validation |
| 6g | `layers: [{frame: {x, y, w, h}, shape}]` stacks shapes in one cell (ring, badge): `frame` is 0–1 of the cell's fitted bounds; later = on top | Findings: `…/cells/C/layers/I` |
| 6e | Row `connector: {style, color, width}` chains the visible cells of ONE row. For a flow across rows (swimlane hand-off) add grid-level `links: [{"from": [row, col], "to": [row, col], "connector": {...}}]`, each cell given by row and starting grid column | Same-column links run straight down; others turn in the column gutter, so set `col_gap` ≥ 10. Links to a spacer (no fill, no outline) are dropped |
| 6f | Point INTO a screenshot or photo with overlay `kind: "callout"`: `text` (12pt bold label, top-left at `from`; omit `from` and the label is placed beside its target, clear of the other callouts), `to: {"anchor_image": {row, col, x, y, units?}}` — `x`/`y` as 0–1 source-image fractions, or `units: "px"`. `anchor_image` also anchors arrow / line / badge endpoints. In a DeckSpec use `image_case` `callouts` | Follows the picture's cover/contain placement through frame, fit and template changes (slide-percent endpoints do not). A cropped-away target reports `OVERLAY_TARGET_CROPPED` and drops its leader — use `fit: "contain"` or move it. Keep labels beside the picture |
| 7 | Do NOT set `inset_*`: every shape's text already keeps 0.5 cm from each edge (unfilled, left-aligned first-column text starts on the title's text edge) | Overrides shrink that margin; size rows for it instead (~43pt per 12pt line) |

In raw `shape_grid` text, bullet a paragraph with `paragraphs[].bullet: true`
(or `"–"`) — a real hanging-indent bullet — never a typed "• ". A typed
leading number renders as typed: only two or more consecutive lines numbered
1, 2, 3 … become an auto-numbered list. `shape.flip_h: true` mirrors a
chevron / arrow for a right-to-left row (text stays upright); do not rotate
it 180°.

## Typography Hierarchy (shape_grid)

Do not invent font sizes. A deck uses one type scale; pick each size by role:

| Role                  | Size    | Weight  | Notes                                                              |
|-----------------------|---------|---------|--------------------------------------------------------------------|
| Display / title       | 28pt    | Regular | Display headings, divider titles; serif (major font) allowed       |
| Lead / banner         | 18pt    | Bold    | Grid header band, lead line                                        |
| Subhead / card title  | 14pt    | Bold    | Card titles (first line, separated by `\n`), column headers       |
| Body / card body      | 12pt    | Regular | Default body copy; the floor in `present` decks                    |
| Caption               | 10pt    | Regular | Only tables, dense matrices, legends and chart labels              |
| KPI value             | 40-48pt | Bold    | Display figures; measured to fit                                   |
| Source line           | 9pt     | Italic  | Engine-rendered from the slide `source` — never author it          |

The table publishes the type-scale tokens in [`internal/tokens/typography.go`](../../internal/tokens/typography.go) (test-synced). 11pt is the body minimum; 10pt only in tables and dense matrices. Off-scale `shape_grid` sizes settle onto the step at or below (16pt → 14pt); display figures and 28pt+ text stay as measured; nothing goes below the `viewing_mode` floor: in `present` (default) body / card text under 12pt and captions under 10pt are refused as `TEXT_BELOW_READABLE_MIN`. Out-of-range pattern text sizes are rejected, not clamped. Short bold ALL-CAPS labels get +7% tracking automatically — write caps, no added spaces. On all-caps templates a number-unit token in a title keeps its case (`€2.2m`, `6 mo`, `3pp`): write units as you mean them. Serif (major font) only for titles and display figures.

## Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types are short names with underscores: `bar`, `line`, `stacked_bar`, `grouped_bar` (`<type>_chart` is accepted everywhere; responses write the short name) | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |
| 10a | Visibly label the measure and supplied units; never invent units. Raw `bar` chart: `title` and `data.y_label`/`data.y_axis_title`; otherwise check the live schema. | `series.name`/`alt` alone may be invisible; an identified measure needs no redundant legend. |
| 10c | Charts are labelled by default: bars ≤16, single-series line / area ≤12 points (no value axis or gridlines then), treemaps (name + value + share). Opt out: `style.show_values: false` or `data.data_labels: false`. One precision per chart: the fewest decimals (≤ 4) keeping labels distinct; pin with `data.data_labels.format` or `style.value_format.decimals`. | Direct labels, no chart junk. |
| 10d | Single-series bars of non-time, non-ordinal categories sort descending; `data.sort: "none"` keeps a meaningful order. `data.orientation: "horizontal"` (bar / grouped / stacked) for rankings or names over ~14 chars. | Ranked horizontal bars read in one glance. |
| 10e | Single-series bar charts (and `horizontal-bar-with-callouts`) are neutral grey with accent1 only on `highlight` bars (0-based indices or names; default the last period of a time series, else the top bar) — set it to the bar(s) the title names. Multi-series line / area / grouped bar: `data.highlight` (series names / indices) keeps that series accent1, the rest grey; default = the series the title / takeaway names. Markers drop past 12 points. Stacked bars: ≥10pt segment labels + a total per stack. Waterfalls accent the deltas. | Emphasis passes the 3-second test. |
| 10f | Pie / donut: sorted largest-first; ≤6 slices labelled "Name NN%" without legend; <3% slices fold into "Other" (`data.group_small_below_pct`, 0 = never); `data.highlight` = accent1 slice, rest grey. >7 slices: `CHART_OVERLOADED` (`use_type: "bar"`). | Pies only for a few coarse shares. |
| 10g | Auto series colours skip the template's semantic red / green while other colours remain and never use dk1 / dk2; set `style.colors` when a colour must carry meaning. | Red reads as a verdict; black as a fault. |
| 10h | Percent data: set `style.value_format.input_scale` to `fraction` (0.25 → 25%) or `percentage_points` (0.25 → 0.25%); left `auto` it guesses and reports `chart.percent_scale_ambiguous`. Waterfall point `type` sets the sign (`chart.waterfall_total_mismatch`); a funnel must narrow (`chart.funnel_stage_increase`). CJK / emoji / Arabic labels (`chart.glyph_missing`): check the render. Value axes start at zero unless a value is negative; `data.y_min`/`y_max` zoom on purpose (axis kept; cutting a bar is rejected, hiding over half of one reports `chart.axis_not_zero`). | Wrong scale, sign or baseline misstates the data. |
| 10i | Same measure for 2–6 groups over the same periods (revenue by region by quarter): `chart_value.type: "small_multiples"`, `{categories, series: [{name, values}]}`, one aligned panel per series on a shared x scale and, by default, ONE y domain (`data.y_scale: "independent"` opts out; the chart says so). `data.highlight` greys the other panels. <2 or >6 panels fail validation; no readable grid gives `chart.plot_area_collapsed` (`shrink_or_split`, `fix.params.max_panels_per_slide`). Unit in `title` / `subtitle`. | Overlapping lines hide each trend; separate charts rescale and hide the magnitude gap. |

`chart_value` takes top-level `subtitle` (context under the chart title, e.g. "FY2024 Q1-Q4", "$M") and `footnote` (source at the chart's bottom edge; counts as sourced for `DATA_WITHOUT_SOURCE`) beside `title` — not inside `data`, and not interchangeable. Bar, line, area, stacked, scatter, pie and donut draw the footnote; otherwise set the slide's `source`. The SVG renderer fails hard (no silent missing text) if no font, fallback or embedded Liberation Sans loads.

## Slide Takeaway (the "so what" line)

| # | Rule | Rationale |
|---|---|---|
| 10b | Chart and matrix slides MUST set `slide.takeaway` (one sentence — the headline answer). | Omission emits `takeaway_missing`. It renders as a dark band of 14pt bold text above the source/footer; content frames shrink above it. Budget two lines. Without room, the band is skipped and preflight emits `chrome_band_no_fit`. |

```json
{"layout_id": "blank-title", "takeaway": "Margin contraction is driven by the EU region — not company-wide.",
 "pattern": {"name": "matrix-2x2", "values": {...}}}
```

## Content and Layout

| # | Rule | Rationale |
|---|---|---|
| 11 | `layout_id` must be a **canonical ID**, not a display name (`"Title Slide"`, `"One Content"` are invalid). Vocabulary: `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank-title`, `blank-canvas`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda`; optional roles are not in every template — check `list_templates.canonical_layout_availability` before pinning one | Canonical IDs resolve by layout tags; discovery's `canonical_layout_ids` lists only resolvable names, and an unavailable one fails with the alternatives |
| 11a | `blank-title` = title-only canvas (a `shape_grid`/`pattern` below a title); `blank-canvas` = truly empty layout. A content-bearing raw `blank-canvas` must set slide-level `headline` (rendered in a theme-aware title band); an empty interstitial may omit it. `blank` is a legacy alias for `blank-title` (the empty Blank only when the template has no Blank + Title) — target a role explicitly in new decks | The explicit IDs map 1:1 to the `blank-title` / `blank` layout tags |
| 12 | Semantic fills (`accent1`, `lt2`, `dk1`) required; hex `#RRGGBB` forbidden unless in the brand-color allowlist. **Never mix semantic and hex fills on the same slide** (always a bug). Never raw names like `"blue"`. `theme_override.colors` values must be exact 6-digit hex (`#1A2B3C` / `1A2B3C`); shorthand or names fail with `invalid_color` at `theme_override/colors/<slot>`; control characters in font names fail with `INVALID_PARAMETER` | Semantic colors adapt to the theme; tint with `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` |
| 13 | `align`: `"l"`, `"ctr"`, `"r"`, `"just"` | NOT `"left"`, `"center"`, `"right"` |
| 14 | `vertical_align`: `"t"`, `"ctr"`, `"b"` | NOT `"top"`, `"middle"`, `"bottom"` |
| 15 | Template names come from `list_templates` (MCP) or `json2pptx templates` (CLI); never assume a fixed list | Compact discovery returns `canonical_layout_ids`, `color_roles` (`white_text_safe_body` 4.5:1, `white_text_safe_large` 3:1, `ink_on_accent`) and `table_styles[]`; `fields="full"` adds `layout_names`. A template with an `error` field failed analysis — do not use it |

**`placeholder_id` per layout** (each content item needs one the layout has; generate refuses otherwise, as validate does)**:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank-title` (and `blank`) → `title` only (body in `shape_grid`); `blank-canvas` → none; `section` → `title` (a section `subtitle` is unsupported: use a layout with one or a `shape_grid` text box). Per-template lists: `json2pptx skill-info` or `list_templates`.

Section dividers are numbered `01`, `02`, … automatically; omit the `Section Number` placeholder (`auto_filled: true` in discovery marks it engine-owned). In a two-column comparison, open each column's bullets with a short label line to get a bold column header.

## Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | The engine replaces low-contrast text (WCAG AA for its size: 3:1 only at ≥18pt / ≥14pt bold) with a template text color (`lt1`/`dk2`/`dk1`), one per fill per slide. A brand-coloured run (accent KPI value, stat) is judged at its OWN size: passing large accent text keeps the accent, failing text is darkened minimally in its hue. A pattern accent fill whose white label fails gets a minimal `shade` of that accent; only a pale accent or tint takes dark ink. Text matching a transparent cell's canvas is never recolored. Check `contrast_autofixed` (before/after ratios) before re-authoring colors: it is reported at the authored element (`/slides/N/shape_grid/rows/R/cells/C/shape/text`; under `/slides/N/pattern` on a pattern slide), the path `contrast_predicted` names at validate, with a `replace_color` fix on an authored raw grid cell. `render_deck_spec` reports a swap once: `contrast_predicted` when validate forecast it, `contrast_autofixed` (same `semantic_path`) only when it did not | Fix: a darker accent fill, `dk1` text, or `"contrast_check": false` (last resort) |

## Icons (no emoji)

| # | Rule | Rationale |
|---|---|---|
| 15a | **Never emit emoji codepoints anywhere in deck JSON.** Icon fields (`card-grid` cells, `icon-row` items, `hero-detail` icon, raw `shape_grid` cell `icon`) MUST be a bundled icon name (preferred) or a loadable user icon. Emoji (`🚀`, `📈`, `✅`, …) are rejected by pattern validators; plain symbols outside the emoji range (`→`, `•`, en-dash) are fine in text | Bundled SVG icons inherit theme colors and scale crisply; emoji fall back to inconsistent system fonts |

`IconInput` takes exactly one of `name` (bundled), `path`, `url`, `svg_data`. Bundled catalog: `list_icons` (MCP) or `json2pptx icons list` (CLI).

## Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` (one line; too long shrinks to 8pt, then ellipsizes: `CHROME_TRUNCATED`) | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

Input JSON is validated with `additionalProperties: false` at every level: unknown keys produce structured warnings with their JSON path (catches `chart` for `chart_value`).

## Table Density (TDR — enforced, not advisory)

| # | Rule | Rationale |
|---|---|---|
| 20 | **MUST split** if rows > 7 OR cols > 6 OR font_size < 9pt. No exceptions. | Larger tables overflow, clip, or become unreadable at viewing distance. Emit `split_slide` instead of cramming |

**Row width.** `headers` defines the columns. A shorter row (counting `col_span`) is padded; a wider one is rejected with `INVALID_PARAMETER` at `...table_value.rows` (grid cells: `.../cells/N/table.rows`).

**Multiline cells.** A cell with `\n`, or a comma-list of ≥3 items in a cell of 48+ characters, counts as N logical rows (N = max(line_count, comma_items)); apply this BEFORE the rows > 10 check (header included; past 7 the rows render at a compact pitch). 8 rows where 3 cells hold 2 lines = 12 logical rows → split.

When TDR forces a split, say so ("N logical rows × M columns; per Rule 20 emitting split_slide") and never shrink fonts below 9pt to avoid it.

**Default look.** A table with no `style` renders as a consulting table: no header fill, 11pt bold header over a rule, 12pt rows separated by hairlines, no zebra, bold first column, numeric columns right-aligned, content-height rows. Prefer it. Opt-ins are additive: `header_background` only fills the header (text flips by contrast; type, rules and unbanded rows stay); `borders` / `striped` add grid lines / zebra; `style.highlight_column` (1-based) tints a column with the template's `color_roles.primary_fill`.

`table_density_guide` (MCP) or `json2pptx tables guide` (CLI, `--json` for the envelope) gives font-size and row-count guidance; scope with `{template}` / `--template <name>`, and `style_id` (requires `template`) narrows to one of its `table_styles[]`.

---

## Anti-patterns

**Two tables in one grid.** Sibling tables in one `shape_grid` with `row_gap < 4pt` read as one broken table. Use `row_gap` ≥ 6 and a divider row — or better, one table per slide.

**Sparse single-row flow** (`SPARSE_SINGLE_ROW_FLOW`, review). A one-row `process-flow` or `dots` `timeline-horizontal` of 3–6 short cells as the slide's only content is sized to its text and leaves most of the slide empty; a height cap does not resize it. Give the sequence vertical mass: `numbered-step-strip` with per-step detail, `value-chain` for described steps, `phase-roadmap` for dated phases, `process-grid-2row` for two tracks, or a second zone (`compose`). `process-flow` draws one path and no yes/no branches (`FLOW_DIAMOND_NO_CONTENT`): explain a decision in a second zone; keep `timeline-horizontal` for true calendar milestones. Pattern and accent monotony across slides: [WORKFLOW.md](WORKFLOW.md) → Phase 2.

---

## Cell Accent Variety

Grid-shaped patterns expose a `cell_accent_mode` override that walks from the slide's base accent (resolved by `accent_strategy`): `uniform` (default; every cell the base accent), `alternate` (base and base+1: paired comparisons, two-tier hierarchies) or `progressive` (base, base+1, base+2, …, wrapping accent6→accent1: 4+ ordered or graded cells only). Non-grid patterns do not expose it; `pyramid` supports it per tier — check the overrides schema in `show_pattern`. Do not use `progressive` on unordered peers (accent belongs on the one cell the title is about), and render-check `alternate` under `section-keyed` so it reads as one emphasis, not a rainbow.

**Only set overrides the schema lists.** `pyramid`, `process-flow` and `process-flow-compact` have no header text and reject `header_size` with `unknown_key` (fix `remove_key`); size their labels with `body_size`. `process-flow-compact` rejects `rows` the same way.

`analyze_deck_rhythm` flags accent *heaviness*, not too few accents: `accent_heavy_slide` (a raw grid with 4+ solid-accent cells — give the rest a neutral `lt2` / dk1-tint fill) and `strong_accent_run` (3+ consecutive `accent_weight: "strong"` patterns — swap one for a normal or subtle pattern).
