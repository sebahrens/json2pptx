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

## Color Roles

Each template exposes `color_roles` in `json2pptx skill-info` output — use `primary_fill` / `secondary_fill` for header cells with white text, `body_fill` + `body_text` for card bodies, and check `white_text_safe` before using any accent with `#FFFFFF` text. For tints, use luminance modifiers: `{"color": "accent1", "lumMod": 20000, "lumOff": 80000}` (20% tint with `dk1` text).

---

## Table Density Reference

Run `json2pptx tables guide` for font size and row-count guidance when building table slides in shape grids.

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

## Reference

For complete field specifications (connectors, accent bars, callout geometries, speaker notes, footers, backgrounds, theme overrides, patch operations, all chart/diagram types, and more), see `~/.claude/skills/template-deck/TEMPLATE_GUIDE.md` or run `json2pptx validate-template <path>`.
