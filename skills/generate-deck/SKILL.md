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
pattern (see Pattern Library below). For content slides, note the content type
(bullets, chart, table, diagram).

Present the outline to the user. Proceed to Stage 2 only after approval or if the user
asked for the full deck directly.

**Narrative coherence matters.** A consulting deck tells a story: situation, complication,
resolution, evidence, implementation, call to action. The outline is where you design
the argument arc. Do not fragment this across stages.

### Stage 2: Generate Full JSON

Generate the complete JSON in one pass. Use the pattern recipes below for shape grid
slides — do not invent grid structures from scratch. Copy the pattern, change the content.

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

These are the proven shape grid patterns extracted from the sovereign-ai-strategy
reference deck. When you need a shape grid, pick the closest pattern and fill in content.
Do NOT invent new grid structures unless no pattern fits.

### Pattern 1: Icon Rows (2-column: icon badge + text)

**Use for:** agenda items, key points with icons, regulatory requirements, risk factors

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 3, "y": 18, "width": 94, "height": 74},
    "gap": 8,
    "row_gap": 12,
    "columns": [10, 90],
    "rows": [
      {
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent1", "icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "CONTENT", "size": 16, "align": "l", "vertical_align": "ctr", "inset_left": 8, "inset_right": 8, "inset_top": 6, "inset_bottom": 6}}}
        ]
      }
    ]
  }
}
```

Repeat the row block 2-5 times. Use the same accent fill for all icon cells within a slide.

### Pattern 2: Card Grid (N-column: header + body)

**Use for:** strategic pillars, capabilities, dimensions, categories

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 5, "y": 18, "width": 90, "height": 72},
    "columns": 3,
    "gap": 10,
    "row_gap": 10,
    "rows": [
      {
        "height": 22,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent1", "icon": {"name": "ICON", "fill": "#FFFFFF"}, "text": {"content": "Header 1", "size": 14, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent1", "icon": {"name": "ICON", "fill": "#FFFFFF"}, "text": {"content": "Header 2", "size": 14, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent1", "icon": {"name": "ICON", "fill": "#FFFFFF"}, "text": {"content": "Header 3", "size": 14, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Body 1", "size": 14, "color": "dk1", "align": "l", "vertical_align": "t", "inset_left": 8, "inset_right": 8, "inset_top": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Body 2", "size": 14, "color": "dk1", "align": "l", "vertical_align": "t", "inset_left": 8, "inset_right": 8, "inset_top": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Body 3", "size": 14, "color": "dk1", "align": "l", "vertical_align": "t", "inset_left": 8, "inset_right": 8, "inset_top": 8, "inset_bottom": 8}}}
        ]
      }
    ]
  }
}
```

For 2, 3, or 4 columns: change `columns` and match the cell count in every row.

**Header color choice.** The pattern above uses uniform `accent1` — always safe, produces a clean consulting look. If you want to distinguish categories, cycle **only among accents that pass WCAG against white**. On most templates `accent1` and `accent2` are dark enough; `accent3`-`accent6` are frequently light/pastel and will either (a) silently trigger contrast auto-fix (text replaced with dark gray, breaking the design) or (b) warn and render unreadable white-on-pastel. See the **Safe Color Pairings** table below. When in doubt, keep headers uniform and distinguish cards by icon or body copy.

### Pattern 3: Labeled 2x2 Matrix

**Use for:** strategic positioning, tradeoff analysis, 2-axis comparisons

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 3, "y": 18, "width": 94, "height": 76},
    "gap": 8,
    "row_gap": 8,
    "columns": [8, 44, 44],
    "rows": [
      {
        "height": 8,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "", "size": 24, "align": "ctr", "vertical_align": "ctr", "color": "#FFFFFF", "bold": true}}},
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "X-Axis Left", "size": 13, "bold": true, "align": "ctr", "vertical_align": "b", "color": "#444444"}}},
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "X-Axis Right", "size": 13, "bold": true, "align": "ctr", "vertical_align": "b", "color": "#444444"}}}
        ]
      },
      {
        "height": 38,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "Y-Axis\nTop", "size": 13, "bold": true, "align": "ctr", "vertical_align": "ctr", "color": "#444444"}}},
          {"shape": {"geometry": "roundRect", "fill": "accent2", "text": {"content": "Quadrant 1\n\nDescription", "size": 13, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "accent1", "text": {"content": "Quadrant 2 (recommended)\n\nDescription", "size": 13, "color": "#FFFFFF", "bold": true, "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}}
        ]
      },
      {
        "height": 38,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "Y-Axis\nBottom", "size": 13, "bold": true, "align": "ctr", "vertical_align": "ctr", "color": "#444444"}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Quadrant 3\n\nDescription", "size": 13, "color": "dk1", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Quadrant 4\n\nDescription", "size": 13, "color": "dk1", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}}
        ]
      },
      {
        "height": 6,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "", "size": 24, "align": "ctr", "vertical_align": "ctr", "color": "#FFFFFF", "bold": true}}},
          {"col_span": 2, "shape": {"geometry": "rect", "fill": "none", "text": {"content": "X-Axis Label", "size": 13, "bold": true, "align": "ctr", "vertical_align": "t", "color": "#444444"}}}
        ]
      }
    ]
  }
}
```

The recommended quadrant uses `accent1` + bold text to visually highlight it. Non-recommended quadrants use `lt2` with `dk1` text for low-emphasis contrast. Do NOT use `accent3`-`accent6` as quadrant fills with white text — on most templates they are too light and the contrast auto-fix will replace the white text with gray, inverting the emphasis.

### Pattern 4: Two-Column Header + Body

**Use for:** pros/cons, before/after, comparison of two options

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 5, "y": 18, "width": 90, "height": 72},
    "columns": 2,
    "gap": 12,
    "row_gap": 10,
    "rows": [
      {
        "height": 12,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "Option A", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent2", "text": {"content": "Option B", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Details for Option A", "size": 14, "color": "dk1", "align": "l", "vertical_align": "t", "inset_left": 8, "inset_right": 8, "inset_top": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Details for Option B", "size": 14, "color": "dk1", "align": "l", "vertical_align": "t", "inset_left": 8, "inset_right": 8, "inset_top": 8, "inset_bottom": 8}}}
        ]
      }
    ]
  }
}
```

### Pattern 5: Table in Grid (realistic density)

**Use for:** data tables, scenario comparisons, financial summaries, vendor comparisons

Consulting tables are dense: 6-11 data rows, 4-6 columns. Budget the table's height first, then size the header around it. Set `style.font_size` explicitly so you control density rather than inheriting a default.

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 3, "y": 15, "width": 94, "height": 82},
    "gap": 4,
    "row_gap": 2,
    "columns": 1,
    "rows": [
      {
        "height": 5,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "Table Title", "size": 12, "bold": true, "color": "#FFFFFF", "align": "l", "vertical_align": "ctr", "inset_left": 10}}}
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {
            "table": {
              "headers": ["Metric", "Q1 Actual", "Q1 Target", "Q2 Actual", "Q2 Target", "Variance"],
              "rows": [
                ["Revenue",          "$12.4M", "$12.0M", "$14.1M", "$13.5M", "+$0.6M"],
                ["New Customers",    "312",    "300",    "388",    "350",    "+38"],
                ["Gross Margin",     "62%",    "60%",    "64%",    "62%",    "+2pp"],
                ["CAC",              "$482",   "$500",   "$445",   "$480",   "-$35"],
                ["LTV",              "$4,210", "$4,000", "$4,680", "$4,200", "+$480"],
                ["Churn (monthly)",  "2.1%",   "2.5%",   "1.9%",   "2.3%",   "-0.4pp"],
                ["NPS",              "52",     "50",     "58",     "55",     "+3"],
                ["Headcount",        "87",     "90",     "94",     "95",     "-1"],
                ["R&D % of Rev",     "22%",    "20%",    "21%",    "20%",    "+1pp"]
              ],
              "style": {"header_background": "accent1", "borders": "horizontal", "striped": true, "font_size": 9},
              "column_alignments": ["l", "r", "r", "r", "r", "r"]
            }
          }
        ]
      }
    ]
  }
}
```

**Why the numbers look like this:**
- `bounds.y: 15, height: 82` — data-dense slide pushes toward the title; defaults (`y:18 height:72`) leave wasted space.
- `gap: 4, row_gap: 2` — tiny gaps. A 5-row grid with default `row_gap: 10` burns 40pt of vertical space for nothing.
- Header row `height: 5` (% of grid height) — just enough for a 12pt bold label.
- Table row `auto_height: true` — takes all remaining space. With the bounds above, that's ~77% of slide height, enough for 9 rows + header at font_size 9.
- `style.font_size: 9` — fits 8-10 rows comfortably at this bound allocation.
- `header_background: "accent1"` — the only accent guaranteed safe for white header text on every bundled template. Others may trigger contrast auto-fix.
- `column_alignments` — first column left (labels), numeric columns right.

See the **Table Density Reference** below for row-count vs. font-size sizing rules.

### Pattern 5b: Two Tables on One Slide

**Use for:** classification + penalty, tiers + detail, before/after comparisons

Common in consulting decks. Budget tables FIRST; section labels take whatever is left.

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 3, "y": 15, "width": 94, "height": 82},
    "gap": 2,
    "row_gap": 1,
    "columns": 1,
    "rows": [
      {"height": 3, "cells": [{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "A. Classification", "size": 10, "bold": true, "color": "dk1", "align": "l", "vertical_align": "ctr"}}}]},
      {"height": 42, "cells": [{"table": {
        "headers": ["Tier", "Criteria", "Example", "Obligations"],
        "rows": [
          ["Prohibited",   "Unacceptable risk", "Social scoring",      "Banned"],
          ["High",         "Safety/fundamental rights impact", "CV screening", "Full compliance"],
          ["Limited",      "Transparency needed", "Chatbots",         "Disclosure"],
          ["Minimal",      "Low/no risk",       "Spam filters",         "None"]
        ],
        "style": {"header_background": "accent1", "borders": "horizontal", "striped": true, "font_size": 9},
        "column_alignments": ["l", "l", "l", "l"]
      }}]},
      {"height": 3, "cells": [{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "B. Penalty Structure", "size": 10, "bold": true, "color": "dk1", "align": "l", "vertical_align": "ctr"}}}]},
      {"auto_height": true, "cells": [{"table": {
        "headers": ["Violation", "Max Fine", "% of Global Revenue"],
        "rows": [
          ["Prohibited AI use",        "€35M",  "7%"],
          ["High-risk non-compliance", "€15M",  "3%"],
          ["Incorrect info to authorities", "€7.5M", "1.5%"]
        ],
        "style": {"header_background": "accent1", "borders": "horizontal", "striped": true, "font_size": 9},
        "column_alignments": ["l", "r", "r"]
      }}]}
    ]
  }
}
```

**Allocation logic:** 3% + 42% + 3% + (remaining ~50%) = ~100%. Section labels at 3% with font_size 10 are readable; anything smaller disappears. If you have a third or fourth section header, drop to `size: 9` on the labels.

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
