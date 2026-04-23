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
   - [ ] Text color is intentional — no surprise grays from contrast auto-fix (see Rule 16).
   - [ ] Footer and source render where expected; no "Source: Source:" double prefix (see Rule 18).
5. **Repair.** If a slide fails the checklist, the fix is in the JSON, not in PowerPoint. Common repairs:
   - Text clipping or overflow → lower font size, increase cell/row allocation, or split content across slides.
   - Unexpected gray text → swap fill to an accent with ≥3.0 contrast against white, OR switch text color to `dk1`, OR set `"contrast_check": false` if the gray is wrong and the accent is already a compliant color (see Rule 16).

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

**Callouts.** Patterns with `supports_callout=true` accept an envelope-level `callout: {text, emphasis?, accent?}` — a full-width band rendered below the pattern. Use for one-line takeaways; text is plain string (no bullets / structured content).

---

## Rules

Non-negotiable. Violating these causes broken or incorrect slides.

### Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]` | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5-row grid with `row_gap: 10` burns 40pt (~5% height). Tighten gaps before shrinking content |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, or `diagram` | Combining silently drops content |
| 7 | Body text cells MUST set all 4 insets (6-10pt each) | Without insets, text jams against shape edges |

### Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types use underscores: `stacked_bar`, `grouped_bar` | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |

### Content and Layout

| # | Rule | Rationale |
|---|---|---|
| 11 | `layout_id` canonical names only: `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda` | Display names like `"Title Slide"` fail to resolve |
| 12 | Prefer semantic fills (`accent1`, `lt2`, `dk1`) over hex; never raw names like `"blue"` | Semantic colors adapt to template theme; use `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` for tints |
| 13 | `align`: `"l"`, `"ctr"`, `"r"`, `"just"` | NOT `"left"`, `"center"`, `"right"` |
| 14 | `vertical_align`: `"t"`, `"ctr"`, `"b"` | NOT `"top"`, `"middle"`, `"bottom"` |
| 15 | Templates: `forest-green`, `midnight-blue`, `modern-template`, `warm-coral` | Inspect with `json2pptx skill-info` |

**`placeholder_id` per layout:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank` → `title` only (body goes in `shape_grid`); `section` → `title`, `subtitle`.

### Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | Engine auto-replaces low-contrast text with dark gray (WCAG AA, ratio < ~3.0) | White on `accent3`-`accent6` → surprise gray. Fix: use `accent1`/`accent2` fill, or `dk1` text, or `"contrast_check": false` (last resort — only when you've verified contrast manually) |

### Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

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

## Deck Sizing Guidelines

| Deck type | Slides | Notes |
|---|---|---|
| Executive summary | 5-8 | Title, 3-5 content, closing |
| Strategy / consulting | 12-20 | Full arc: situation, evidence, solution, implementation, ask |
| Board presentation | 8-12 | Concise with data-heavy slides |
| Training / workshop | 15-30 | More content slides, fewer grids |
| Quick update | 3-5 | Title, 1-3 content, next steps |

---

## Icon Names

Run `json2pptx icons list` for all available names (or `--json` for JSON output). Use `"icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}` inside a shape, or `"icon": {"name": "ICON_NAME"}` as a standalone cell.

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
