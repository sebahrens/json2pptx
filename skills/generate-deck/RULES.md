# Rules

Non-negotiable. Violating these causes broken or incorrect slides.

## Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]`. Widths (and row `height` / `flex`) must be finite and >= 0 and the widths must not all be 0 — `validate` rejects them | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5 rows with `row_gap: 10` burn 40pt (~5% height). Tighten gaps before shrinking content |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, `diagram`, `composite`, `pattern`, or `grid` | Combining silently drops content. `composite` bundles text + sub-diagram (6a); `pattern` / `grid` host a nested layout (6c) |
| 6a | `composite: {text: {...shape...}, sub_diagram: {...}, split: "top"\|"bottom", ratio: 0.0–1.0}` packs a native text shape and an embedded chart into one cell, split vertically (KPI + sparkline, headline + mini chart). `split` defaults to `"top"` (text above); `ratio` (text share) to 0.5. No other content key on the same cell | One cell instead of hand-tuned adjacent cells; accent_bar / connectors target the pair as one cell |
| 6b | `card-grid` cells and `icon-row` items accept one optional `secondary: {type, values, categories?, color?}`: `type` is `sparkline`, `bar_chart` or `line_chart`; `values` 2–12 numbers; `categories` (if set) the same length. E.g. `{"header": "Revenue", "body": "Q1–Q4 trend", "secondary": {"type": "sparkline", "values": [100, 120, 110, 145]}}` | Expands to a composite cell automatically, keeping pattern-level validation |
| 6c | A cell may host a nested layout via `pattern: {name, values, overrides?, cell_overrides?}` (the slide-level payload) or `grid: {…ShapeGridInput…}`, rendered inside the cell with a 4pt inset; exclusive with each other and every other content key. Nested patterns inherit the deck's `accent_strategy` at the parent's slide/section index. E.g. a `kpi-3up` in a `matrix-2x2` quadrant | Avoids slide-level `compose` for one embedded block; overlay `anchor_cell` lookups still resolve |
| 6d | A raster `image` cell takes optional `geometry`: `"rect"` (default) or `"ellipse"` (clips the cover-cropped picture to an ellipse — pair with `fit: "contain"` for a circular headshot), and optional `image.fit`: `"cover"` (default, crops to fill the frame) or `"contain"` (whole picture, centred — screenshots whose edges matter) | Only the frame preset / placement changes; any other value is a validation error |
| 6e | Row `connector: {style, color, width}` chains the visible cells of ONE row. For a flow across rows (swimlane hand-off) add grid-level `links: [{"from": [row, col], "to": [row, col], "connector": {...}}]`, each cell given by row and starting grid column | Same-column links run straight down; others turn in the column gutter, so set `col_gap` ≥ 10. Links to a spacer (no fill, no outline) are dropped |
| 6f | Point at something IN a screenshot or photo with an overlay `kind: "callout"`: `text` (12pt bold native label, top-left at `from`) and `to: {"anchor_image": {row, col, x, y, units?}}` — `x`/`y` in 0–1 fractions of the source image, or `units: "px"` intrinsic pixels. `anchor_image` also works on arrow / line / badge endpoints | The target follows the picture's own cover/contain placement, so it survives frame, fit and template changes; slide-percent endpoints do not. A target the crop hides reports `OVERLAY_TARGET_CROPPED` and its leader is omitted (label kept) — switch the image to `fit: "contain"` or move the target. Place labels beside the picture so leaders do not cross other labels |
| 7 | Do NOT set `inset_*`: every shape's text already keeps 0.5 cm from each edge (unfilled, left-aligned first-column text starts on the title's text edge) | Overrides shrink that margin; size rows for it instead (~43pt per 12pt line) |

In raw `shape_grid` text, bullet a paragraph with `paragraphs[].bullet: true`
(or `"–"`) — a real hanging-indent bullet — never a typed "• ".

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

The table publishes the type-scale tokens in [`internal/tokens/typography.go`](../../internal/tokens/typography.go) (a test keeps them in sync). 11pt is the body minimum; 10pt only in tables and dense matrices. The engine settles off-scale `shape_grid` sizes onto the step at or below them (16pt → 14pt, 20pt → 18pt), keeps display figures and 28pt+ text as measured, and never goes below the `viewing_mode` floor: in `present` (default) body and card text under 12pt and captions under 10pt are refused as `TEXT_BELOW_READABLE_MIN`; `read` floors are limits, not sizes to author. Out-of-range pattern text sizes are rejected, not clamped. Short bold ALL-CAPS labels (headers, eyebrows, badges) are tracked +7% automatically — write them in caps, do not add spaces; regular-weight caps (an owner "VP CS") are not. Bold headings, titles and names are balanced so no line ends on a lone word. Serif (major font) is for titles and display figures only. `body_and_lead` bullets get the body density size; the bold lead sits one 2pt step above. Leave insets at the 0.5 cm default. Examples: [`docs/INPUT_FORMAT.md`](../../docs/INPUT_FORMAT.md).

## Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types use underscores: `stacked_bar`, `grouped_bar` | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |
| 10a | Visibly label the measure and supplied units; never invent units. Raw `bar_chart`: `title` and `data.y_label`/`data.y_axis_title`; otherwise check the live schema. | `series.name`/`alt` alone may be invisible; an identified measure needs no redundant legend. |
| 10c | Charts are labelled by default (raw and pattern paths): bars ≤16, single-series line / area ≤12 points (no value axis or gridlines then), treemaps (name + value + share). Opt out: `style.show_values: false` or `data.data_labels: false`. | Direct labels, no chart junk. |
| 10d | Single-series bars of non-time, non-ordinal categories sort descending; `data.sort: "none"` keeps a meaningful order. `data.orientation: "horizontal"` (bar / grouped / stacked) for rankings or names over ~14 chars. | Ranked horizontal bars read in one glance. |
| 10e | Single-series bar charts (and `horizontal-bar-with-callouts`) are neutral grey with accent1 only on `highlight` bars (0-based indices or names; default the last period of a time series, else the top bar) — set it to the bar(s) the title names. Multi-series line / area / grouped bar: `data.highlight` (series names / indices) keeps that series accent1, the rest grey; default = the series the title / takeaway names. Markers drop past 12 points. Stacked bars: ≥10pt segment labels + a total per stack. Waterfalls accent the decreases. | Emphasis passes the 3-second test. |
| 10f | Pie / donut: sorted largest-first; ≤6 slices labelled "Name NN%" without legend; <3% slices fold into "Other" (`data.group_small_below_pct`, 0 = never); `data.highlight` = accent1 slice, rest grey. >7 slices: `CHART_OVERLOADED` (`use_type: "bar_chart"`). | Pies only for a few coarse shares. |
| 10g | Auto series colours skip the template's semantic red / green while other colours remain and never use dk1 / dk2; set `style.colors` when a colour must carry meaning. | Red reads as a verdict; black as a fault. |
| 10h | Percent data: set `style.value_format.input_scale` to `fraction` (0.25 → 25%) or `percentage_points` (0.25 → 0.25%); left `auto` it guesses and reports `chart.percent_scale_ambiguous`. Waterfall point `type` sets the sign (`chart.waterfall_total_mismatch`); a funnel must narrow (`chart.funnel_stage_increase`); bubble area is proportional to size on one chart-wide scale (negative sizes rejected). CJK / emoji / Arabic labels lack embedded glyphs (`chart.glyph_missing`): check the render. | Wrong scale or sign misstates the data. |

`chart_value` takes top-level `subtitle` (context under the chart title, e.g. "FY2024 Q1-Q4", "$M") and `footnote` (source at the chart's bottom edge; counts as sourced for `DATA_WITHOUT_SOURCE`) beside `title` — not inside `data`, and not interchangeable. Bar, line, area, stacked, scatter, pie and donut draw the footnote; otherwise set the slide's `source`. The SVG renderer fails hard (no silent missing text) if no font, fallback or embedded Liberation Sans loads.

## Slide Takeaway (the "so what" line)

| # | Rule | Rationale |
|---|---|---|
| 10b | Chart and matrix slides MUST set `slide.takeaway` (one sentence — the headline answer). | Omission emits `takeaway_missing`. It renders as 14pt bold dk1 text beside a flush 3pt accent1 bar (no fill, no outline) above the source/footer, spanning the body column with 16pt of air above and 12pt below; content frames shrink above it. Budget two lines. Without room, the band is skipped and preflight emits `chrome_band_no_fit`. |

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
| 15 | Template names come from `list_templates` (MCP) or `json2pptx skill-info` (CLI); never assume a fixed list | Default discovery returns `canonical_layout_ids`, `color_roles`, `table_styles[]`, `white_text_safe_body` (4.5:1), `white_text_safe_large` (3:1), `ink_on_accent`, and `data_format_hints_digest`; `mode="compact"` or `fields="full"` adds `layout_names`. A template with an `error` field failed analysis — do not use it |

**`placeholder_id` per layout:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank-title` (and `blank`) → `title` only (body in `shape_grid`); `blank-canvas` → none; `section` → `title` (a section `subtitle` is unsupported: use a layout with one or a `shape_grid` text box). Per-template lists: `json2pptx skill-info` or `list_templates`.

Section dividers are numbered `01`, `02`, … automatically; omit the `Section Number` placeholder (`auto_filled: true` in discovery marks it engine-owned). In a two-column comparison, open each column's bullets with a short label line to get a bold column header.

## Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | The engine replaces low-contrast text (WCAG AA for its size: 3:1 only at ≥18pt / ≥14pt bold) with a template text color (`lt1`/`dk2`/`dk1`), one per fill per slide. White / black inks are judged at the body's smallest text; a brand-coloured run (accent KPI value, stat, insight header) at its OWN size, so large accent text that clears 3:1 keeps the accent and one that misses is darkened minimally in its hue, not snapped to black. KPI peer cards draw the big number in the accent on their neutral surface. Your color is kept when a swap would barely help, and text matching a transparent cell's canvas (hidden on purpose) is never recolored. Pattern-drawn accent fills whose white label misses the bar get a minimal `shade` of the same accent so the type stays white (no black-on-orange); only a pale accent or tint takes dark ink. Check `fit_findings` for `contrast_autofixed` (before/after ratios) before re-authoring colors | Fix: a darker accent fill, `dk1` text, or `"contrast_check": false` (last resort, after checking contrast yourself) |

## Icons (no emoji)

| # | Rule | Rationale |
|---|---|---|
| 15a | **Never emit emoji codepoints anywhere in deck JSON.** Icon fields (`card-grid` cells, `icon-row` items, `hero-detail` icon, raw `shape_grid` cell `icon`) MUST be a bundled icon name (preferred) or a loadable user icon. Emoji (`🚀`, `📈`, `✅`, …) are rejected by pattern validators; plain symbols outside the emoji range (`→`, `•`, en-dash) are fine in text | Bundled SVG icons inherit theme colors and scale crisply; emoji fall back to inconsistent system fonts |

`IconInput` takes exactly one of `name` (bundled), `path`, `url`, `svg_data`. Bundled catalog: `list_icons` (MCP) or `json2pptx icons list` (CLI).

## Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` (one line across the footer width; over-long text shrinks to 8pt then ellipsizes) | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

Input JSON is validated with `additionalProperties: false` at every level: unknown keys produce structured warnings with their JSON path (catches `chart` for `chart_value`).

## Table Density (TDR — enforced, not advisory)

| # | Rule | Rationale |
|---|---|---|
| 20 | **MUST split** if rows > 7 OR cols > 6 OR font_size < 9pt. No exceptions. | Larger tables overflow, clip, or become unreadable at viewing distance. Emit `split_slide` instead of cramming |

**Row width.** `headers` defines the columns. A shorter row (counting `col_span`) is padded; a wider one is rejected with `INVALID_PARAMETER` at `...table_value.rows` (grid cells: `.../cells/N/table.rows`).

**Multiline cells.** A cell with `\n` or a comma-list of ≥3 items counts as N logical rows (N = max(line_count, comma_items)); apply this BEFORE the rows > 7 check. 5 rows where 3 cells hold 2 lines = 8 logical rows → split.

**Refusal wording.** When TDR forces a split, emit exactly: *"This table has [N] logical rows × [M] columns; per Rule 20 I cannot fit this — emitting split_slide to distribute rows across slides."* Never shrink fonts below 9pt to avoid the split.

**Default look.** A table with no `style` renders as a consulting table: no header fill (never a black header bar), 11pt bold header over a 1pt rule, 12pt rows separated by 0.5pt 15% hairlines, no zebra, bold first column, numeric columns right-aligned, content-height rows top-anchored under the title. Prefer it. Opt-ins are additive: `header_background` only fills the header (text flips by contrast; type, rules and unbanded rows stay); `borders` / `striped` add grid lines / zebra. `table-highlight` uses the same look with an accent 10% tint and 3pt accent bar on the highlighted row.

`table_density_guide` (MCP) or `json2pptx tables guide` (CLI, `--json` for the envelope) gives font-size and row-count guidance; scope with `{template}` / `--template <name>`, and `style_id` (requires `template`) narrows to one of its `table_styles[]`.

---

## Anti-patterns

**Two tables in one grid.** Sibling tables in one `shape_grid` with `row_gap < 4pt`, or a divider shorter than 4% of slide height between them, read as one broken table. Use `row_gap` ≥ 6 and a divider row of height ≥ 8 — or better, one table per slide.

**Hex-fill mix.** Semantic fills and non-allowlisted hex on one slide (Rule 12): `{"fill": "accent1"}` beside `{"fill": "#FF6B35"}` → make the second `accent2`.

**Sparse single-row flow.** One row of 3-6 short boxes floating in whitespace reads as unfinished. `process-flow` earns a full slide only when the sequence **branches** (decision diamonds); `timeline-horizontal` only for **true calendar milestones**. `recommend_visual` ranks `numbered-step-strip` first for a sequence with no decision language. Give the sequence vertical mass: `numbered-step-strip` with per-step detail, `value-chain` for described steps, `phase-roadmap` for dated phases, `process-grid-2row` for two tracks — or a two-row `shape_grid` (numbered lane above an aligned detail row). Pattern and accent monotony across slides: [WORKFLOW.md](WORKFLOW.md) → Phase 2.

---

## Cell Accent Variety

Grid-shaped patterns expose a `cell_accent_mode` override that walks from the slide's base accent (resolved by `accent_strategy`):

| Mode | Behavior | When to use |
|------|----------|-------------|
| `uniform` (default) | Every cell uses the base accent | Timelines, processes, sequences; the consulting default |
| `alternate` | Base and base+1 alternate (wraps accent6→accent1) | Paired comparisons, two-tier hierarchies, before/after |
| `progressive` | base, base+1, base+2, … (wraps) | 4+ ordered or graded cells only |

E.g. `section-keyed` resolves slide 5 to `accent3`; `progressive` then gives `accent3`, `accent4`, `accent5`, `accent6`, `accent1`, … . Non-grid patterns (heroes, axis-bound matrices) do not expose it; `pyramid` supports it per tier — check the overrides schema in `show_pattern`. Do not use `progressive` on unordered peers (accent belongs on the one cell the title is about), and render-check `alternate` under `section-keyed` so it reads as one emphasis, not a rainbow.

**Only set overrides the schema lists.** `pyramid`, `process-flow` and `process-flow-compact` have no header text and reject `header_size` with `unknown_key` (fix `remove_key`); size their labels with `body_size`.

`analyze_deck_rhythm` flags accent *heaviness*, not too few accents: `accent_heavy_slide` (a raw grid with 4+ solid-accent cells — give the rest a neutral `lt2` / dk1-tint fill) and `strong_accent_run` (3+ consecutive `accent_weight: "strong"` patterns — swap one for a normal or subtle pattern).
