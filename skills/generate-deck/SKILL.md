---
name: generate-deck
description: >
  Generate consulting-quality PowerPoint decks from user prompts using json2pptx.
  Applies constrained generation: 2-stage workflow (outline then fill), pattern-based
  shape grids, invariant enforcement, and validate-repair loop. Use when the user asks
  to create, generate, or build a presentation or slide deck.
---

# Deck Generation Skill

You generate structured JSON for the json2pptx engine. Your output must be valid input
for the `generate_presentation` MCP tool (or the CLI `json2pptx generate -json`).

Read `~/.claude/skills/template-deck/TEMPLATE_GUIDE.md` for the complete field reference
(content types, chart types, diagram types, shape grid properties, patch operations).
This skill covers the **generation workflow and patterns** — not the field reference.

---

## Workflow: Plan, Generate, Validate

### Stage 1: Plan the Deck

Before writing any JSON, produce a short outline:

```
Deck: [title]
Template: [template name]
Slides:
  1. [layout] — [title] — [visual pattern or content type]
  2. [layout] — [title] — [visual pattern or content type]
  ...
```

Each line picks a `layout_id` and a visual approach. For shape grid slides, name the
pattern (run `json2pptx patterns list` for the catalog). For content slides, note the
content type (bullets, chart, table, diagram).

Present the outline to the user. Proceed to Stage 2 only after approval or if the user
asked for the full deck directly.

**Narrative coherence matters.** A consulting deck tells a story: situation, complication,
resolution, evidence, implementation, call to action. The outline is where you design
the argument arc. Do not fragment this across stages.

### Stage 2: Generate Full JSON

Generate the complete JSON in one pass. Use named patterns for shape grid slides — run
`json2pptx patterns show <name>` for each pattern's value schema, then fill in content.

### Stage 3: Validate, Render, Verify, Repair

Validation is NOT verification. `validate_input` checks JSON structure; it does not judge whether the deck looks right. Contrast auto-fix, sizing choices, overflowing text, and mis-chosen layouts are all visible in pixels and invisible in JSON. **Images are truth.**

1. **Schema check.** Call `validate_input` (MCP) or have the user run `json2pptx validate`. Fix any errors — fix only failing slides, don't regenerate the deck.
2. **Generate.** Call `generate_presentation` (MCP) or `json2pptx generate`.
3. **Render to images.** Run `pptx2jpg -input <out.pptx> -output <dir>/ -density 150`. Requires LibreOffice + ImageMagick; if unavailable, **say so explicitly** and flag data-dense slides for manual inspection before declaring done.
4. **Inspection checklist (per slide).** Before handing back to the user, confirm:
   - [ ] Text fits its shape or cell — no clipping, no visible overflow.
   - [ ] Chart axes/legends are readable at deck-viewing size.
   - [ ] Every placeholder and grid cell shows the content you intended.
   - [ ] Text color is intentional — no surprise grays from contrast auto-fix (see Invariant 16).
   - [ ] Footer and source render where expected; no "Source: Source:" double prefix (see Anti-Pattern list).
5. **Repair.** If a slide fails the checklist, the fix is in the JSON, not in PowerPoint. Common repairs:
   - Text clipping or overflow → lower font size, increase cell/row allocation, or split content across slides.
   - Unexpected gray text → swap fill to an accent with ≥3.0 contrast against white, OR switch text color to `dk1`, OR set `"contrast_check": false` if the gray is wrong and the accent is already a compliant color (see Invariant 16).

Do not tell the user the deck is done until the checklist passes or you have explicitly flagged what you couldn't verify.

---

## Pattern Library

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, and two-column
comparisons, use json2pptx's named patterns. Named patterns expand to validated
`shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with
~100 tokens.

- **Browse the catalog:** `json2pptx patterns list`
- **View a pattern's value schema:** `json2pptx patterns show <name>`
- **Validate before generating:** `json2pptx patterns validate <name> <values.json>`

Do NOT hand-roll shape grids when a named pattern exists. Use the pattern, fill in
the values, and let the engine handle grid structure, bounds, and gap arithmetic.

---

## Invariants (Rules That Prevent Rendering Errors)

These are non-negotiable. Violating them causes broken slides.

### Shape Grid

1. **Cell count must match columns.** Every row: `sum(cell.col_span or 1 for each cell) == column count`. If `columns: 3`, every row needs cells spanning exactly 3 columns total.
2. **`columns` type matters.** Integer `3` = three equal columns. Array `[10, 90]` = proportional widths. Never use `[3]` (single column at width 3%) when you mean `3` (three equal columns).
3. **`bounds` uses percentages (0-100).** Not points, not EMU. `{"x": 5, "y": 18, "width": 90, "height": 72}` means 5% from left, 18% from top.
4. **`gap`/`row_gap`/`col_gap` are typographic points, not percentages.** Default 8. Typical 4-12 for spacious slides; **1-4 for data-dense slides**. They are cumulative: a 5-row grid with `row_gap: 10` burns 40pt (~5% of slide height) on empty space. Tighten gaps before shrinking content. (TEMPLATE_GUIDE.md previously said percentages — that was wrong; source of truth is `internal/shapegrid/grid.go`.)
5. **Row `height` is a percentage of grid height.** `"height": 22` = 22% of `bounds.height`. Rows without height split remaining space equally.
6. **One content type per cell.** Exactly one of: `shape`, `table`, `icon`, `image`, or `diagram`. Never combine them.
7. **Body text in shape grids MUST include all insets.** Always set `inset_left`, `inset_right`, `inset_top`, `inset_bottom` (typically 6-10pt) on body/content cells. Without them, text jams against shape edges.

### Charts

8. **Series values must match categories length.** For multi-series charts: every `series[i].values` array must have exactly `len(categories)` elements.
9. **Chart types use underscores.** `stacked_bar`, `grouped_bar`, `stacked_area`. Never hyphens (`stacked-bar`).
10. **Two data formats — don't mix them.**
    - Single series: `"data": {"Q1": 10, "Q2": 15, "Q3": 22}`
    - Multi-series: `"data": {"categories": [...], "series": [{"name": "...", "values": [...]}]}`
    - Waterfall: `"data": {"points": [{"label": "...", "value": N, "type": "increase|decrease|subtotal|total"}]}`

### Content and Layout

11. **`layout_id` accepts canonical semantic names only.** Use `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda`. Do NOT use display names like `"Title Slide"` or `"One Content"` — those are what `json2pptx skill-info` prints, not valid input. (Source: `internal/layout/canonical.go`.) **`placeholder_id` per layout type:**
    - `title` / `closing` layout: `title`, `subtitle`
    - `content` layout: `title`, `body`
    - `two-column` layout: `title`, `body`, `body_2`
    - `blank` layout: `title` only in `content`; body content goes in `shape_grid`
    - `section` layout: `title`, `subtitle`
12. **`fill` vocabulary — prefer semantic over hex.** Semantic names (`accent1`-`accent6`, `lt1`, `lt2`, `dk1`, `dk2`, `bg1`, `bg2`, `tx1`, `tx2`, `hlink`, `folHlink`, `none`) now emit `<a:schemeClr>` in the output PPTX, so colors **adapt when the template theme changes**. Hex (`"#RRGGBB"`) bakes the color in. Always prefer semantic. For tints/shades use `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` — 75000/25000 is a common 75% tint — rather than pre-calculating hex. Alpha: `{"color": "accent1", "alpha": 50}`. Never raw names like `"blue"`.
13. **`align` values:** `"l"`, `"ctr"`, `"r"`, `"just"`. NOT `"left"`, `"center"`, `"right"`.
14. **`vertical_align` values:** `"t"`, `"ctr"`, `"b"`. NOT `"top"`, `"middle"`, `"bottom"`.
15. **Templates:** `forest-green`, `midnight-blue`, `modern-template`, `warm-coral`. Additional custom templates may live in the templates directory; inspect with `json2pptx skill-info` before use.

### Contrast Auto-Fix

16. **The engine auto-fixes low-contrast text.** When text color would fail WCAG AA Large against its fill (ratio < ~3.0), the engine replaces the text color with a calculated dark gray. This fires *silently* for scheme-color fills and produces "warning preserved" for hex fills — the behaviors are being unified; do not rely on either. Consequences:
    - White text on `accent3`-`accent6` (usually light/pastel accents) will become gray on most templates.
    - The design you intended (white-on-peach brand color) is NOT what ships.
    - To disable the auto-fix on a specific slide or shape, set `"contrast_check": false` on the slide or element. Use sparingly — only when you have verified contrast by another means (e.g., the fill is actually dark enough but the engine's calculation disagrees, or the gray text is intentional for a decorative element).
    - Preferred path: pick a fill that passes (see **Safe Color Pairings**). Second: switch text to `dk1`. Opt-out is last resort.

---

## Anti-Patterns That Silently Break

These do NOT produce an error. They produce a broken deck that looks plausible in JSON. Every one of these has bitten real deck production.

| Anti-Pattern | What happens | Fix |
|---|---|---|
| `"footer": "My Footer"` (string) | Engine crashes: `cannot unmarshal string into footer` | Must be an object: `"footer": {"enabled": true, "left_text": "My Footer"}` |
| `"source": "Source: Internal Analytics"` | Renders as "Source: Source: Internal Analytics" | Omit the prefix: `"source": "Internal Analytics"` — engine prepends "Source: " |
| `{"type": "chart", "chart": {...}}` or `{"type": "table", "table": {...}}` in the content array | Engine error or silent empty slide | Field names are `chart_value` / `table_value` / `diagram_value` / `text_value` — always the `_value` suffix |
| `"layout_id": "Title Slide"` (display name) | Fails to resolve | Use canonical names: `title`, `content`, `two-column`, `blank`, `section`, `closing` (see Invariant 11) |
| White text on `accent3`-`accent6` fill | Contrast auto-fix silently replaces text with dark gray, design intent lost | Use `accent1` (or verified dark accent); OR switch text to `dk1`; see Safe Color Pairings |

---

## Safe Color Pairings

Accent values are template-specific. Before cycling accents across a slide, read the template's theme via `json2pptx skill-info` and prefer these patterns:

| Intent | Fill | Text | Notes |
|---|---|---|---|
| Primary header / emphasis | `accent1` | `#FFFFFF` | On all 4 bundled templates, accent1 is dark enough to pass |
| Secondary header | `accent2` | `#FFFFFF` | Usually safe, but verify per template |
| Body / supporting cell | `lt2` | `dk1` | Always safe; the go-to for card bodies |
| Subtle background | `{"color": "accent1", "lumMod": 20000, "lumOff": 80000}` | `dk1` | 20% accent tint — light wash, dark text |
| Light section header | `{"color": "lt2", "alpha": 30}` | `dk1` | Barely-there divider |
| Table header | `accent1` (via `header_background`) | (auto) | Only accent1 is safe on all bundled templates |
| Highlighted quadrant | `accent1` bold | `#FFFFFF` | Pattern 3 recommended |
| De-emphasized quadrant | `lt2` | `dk1` | Pattern 3 non-recommended |

**Never-do pairings (on unknown templates):** white text on `accent3`, `accent4`, `accent5`, `accent6` without verifying their hex via `skill-info` and checking WCAG ≥ 3.0 against white.

---

## Table Density Reference

Rules of thumb for fitting a table in a shape_grid. Assumes `auto_height: true` on the table's row, `bounds.height: 82`, tight gaps.

| Data rows | `style.font_size` | Max columns | Notes |
|---|---|---|---|
| 1-4 | 12-14 | 6 | Default font works; keep spacing generous |
| 5-7 | 10-11 | 6 | Explicit `font_size` required |
| 8-10 | 9 | 6 | Use `bounds.y: 15`, `row_gap: 1-2` |
| 11-13 | 8 | 5 | Tight; consider dropping a column |
| 14-16 | 7 | 4 | The last stop before splitting |
| 17+ | — | — | Split across two slides |

**Multiline cells eat budget.** A cell with 3 text lines at font_size 8 needs roughly the same vertical space as 3 single-line rows. If you use multiline cells, count each line as a row when sizing.

---

## Common Mistakes

| Mistake | Fix |
|---|---|
| `"columns": [3]` (array with one element) | Use `"columns": 3` (integer) for 3 equal columns |
| Missing insets on body text cells | Always include all 4 insets (6-10pt) |
| `"type": "stacked-bar"` | Use underscores: `"type": "stacked_bar"` |
| `"align": "center"` | Use short form: `"align": "ctr"` |
| `"placeholder_id": "body"` on blank layout | Blank layouts only have `title` placeholder; body goes in `shape_grid` |
| Rows with wrong cell count | Count col_spans: must sum to column count per row |
| `"fill": "blue"` | Use semantic names (`accent1`) or hex (`"#0066CC"`) |
| Body text with `"body_left"` / `"body_right"` | Two-column uses `"body"` and `"body_2"` |
| Mixing chart data formats in one chart | Pick one: flat map, categories+series, or points |

---

## Deck Sizing Guidelines

| Deck type | Slides | Notes |
|---|---|---|
| Executive summary | 5-8 | Title, 3-5 content, closing |
| Strategy / consulting | 12-20 | Full arc: situation, evidence, solution, implementation, ask |
| Board presentation | 8-12 | Concise with data-heavy slides |
| Training / workshop | 15-30 | More content slides, fewer grids |
| Quick update | 3-5 | Title, 1-3 content, next steps |

---

## Icon Names (Verified Available)

Common icons for consulting decks:

`database`, `globe`, `cpu`, `coins`, `trending-up`, `rocket`, `shield`, `clock`,
`lock`, `users`, `server`, `bulb`, `network`, `cloud`, `briefcase`, `alert-triangle`,
`building-bank`, `chart-pie`, `target`, `award`, `check-circle`, `flag`, `layers`,
`map-pin`, `phone`, `mail`, `calendar`, `settings`, `eye`, `heart`, `star`,
`zap`, `bar-chart`, `pie-chart`, `activity`, `compass`, `anchor`

Use `"icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}` inside a shape, or
`"icon": {"name": "ICON_NAME"}` as a standalone cell.

---

## Accent Color Strategy

Within a single slide, use accent colors intentionally:

- **Uniform:** All header cells use `accent1` (clean, professional; always safe for white text)
- **Cycling:** Headers cycle `accent1`, `accent2` only — or verify accent3-6 against white first
- **Semantic:** Green accent for positive, red/coral for negative, neutral for baseline

Across the deck, keep accent usage consistent. If slide 3 uses accent1 for "Infrastructure",
slide 8 should use accent1 for "Infrastructure" too.

Body cells almost always use `"fill": "lt2"` with `"color": "dk1"` text.
Header cells use `accent1` (or verified dark accent) with `"color": "#FFFFFF"` text.

For tints/shades, prefer luminance modifiers over pre-calculated hex:
```json
{"fill": {"color": "accent1", "lumMod": 20000, "lumOff": 80000}}   // 20% tint
{"fill": {"color": "accent1", "lumMod": 75000}}                    // 75% darker
```

---

## Capability Index

The skill above covers the common cases. For less-frequent features, see `TEMPLATE_GUIDE.md`:

| Feature | Where in GUIDE |
|---|---|
| `connector` on rows (arrows/lines between adjacent cells) | Shape Grid → Row Connector |
| `accent_bar` on cells (decorative side bar) | Cell Definition |
| `column_alignments` on tables (per-column l/c/r) | Content Types → table |
| Callout geometries (`wedgeRoundRectCallout`, etc.) | Shape Properties |
| `speaker_notes` on slides | Slide-Level Fields |
| `source` citation on slides (engine prepends "Source: ") | Slide-Level Fields |
| `footer` (must be object with `enabled: true`) | Footer Configuration |
| `background` image on slides | Slide Background |
| `theme_override` for custom colors/fonts | Theme Overrides |
| `body_and_bullets`, `bullet_groups` content types | Content Types |
| `image` cells in shape grids | Image Cell |
| `row_span` / `col_span` for merged cells | Cell Definition |
| `fit` property (contain / fit-width / fit-height) | Cell Definition |
| `font_size` override on placeholders | Font Size Override |
| Character limits per content type | Character Limits |
| Patch operations (incremental deck edits) | Patch Operations |
| All 15 chart types / 21 diagram types | Content Types → chart / diagram |

Claude reads `TEMPLATE_GUIDE.md` as part of the skill context on each session; navigate to the relevant section when one of the above is needed. Do not duplicate the GUIDE content here — it drifts.

---

## Reference

For complete field specifications, chart types, diagram types, shape properties,
patch operations, and theme overrides, see:

`~/.claude/skills/template-deck/TEMPLATE_GUIDE.md`
