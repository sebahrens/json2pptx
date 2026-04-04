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

### Stage 3: Validate and Repair

After generating JSON:

1. Call `validate_input` (MCP) or note that the user should run `json2pptx validate`.
2. If errors are returned, fix **only the failing slides** — do not regenerate the whole deck.
3. Then call `generate_presentation` or output the final JSON.

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
          {"shape": {"geometry": "rect", "fill": "accent2", "icon": {"name": "ICON", "fill": "#FFFFFF"}, "text": {"content": "Header 2", "size": 14, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent3", "icon": {"name": "ICON", "fill": "#FFFFFF"}, "text": {"content": "Header 3", "size": 14, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
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
Cycle accent fills across header cells: accent1, accent2, accent3, accent4, accent5, accent6.
To add a second header+body pair (2x3 grid), repeat both rows with accent4-accent6.

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
          {"shape": {"geometry": "roundRect", "fill": "accent1", "text": {"content": "Quadrant 1\n\nDescription", "size": 13, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "accent3", "text": {"content": "Quadrant 2\n\nDescription", "size": 13, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}}
        ]
      },
      {
        "height": 38,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "none", "text": {"content": "Y-Axis\nBottom", "size": 13, "bold": true, "align": "ctr", "vertical_align": "ctr", "color": "#444444"}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Quadrant 3\n\nDescription", "size": 13, "color": "#333333", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "accent2", "text": {"content": "Quadrant 4\n\nDescription", "size": 13, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr", "inset_top": 10, "inset_left": 10, "inset_right": 10, "inset_bottom": 10}}}
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

The recommended quadrant uses accent3 + bold text to visually highlight it.

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

### Pattern 5: Table in Grid (with optional accent header bar)

**Use for:** data tables, scenario comparisons, financial summaries

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "TITLE"}
  ],
  "shape_grid": {
    "bounds": {"x": 5, "y": 18, "width": 90, "height": 72},
    "gap": 10,
    "row_gap": 10,
    "columns": 1,
    "rows": [
      {
        "height": 8,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent2", "text": {"content": "Table Title", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {
            "table": {
              "headers": ["Column A", "Column B", "Column C"],
              "rows": [
                ["Row 1 A", "Row 1 B", "Row 1 C"],
                ["Row 2 A", "Row 2 B", "Row 2 C"]
              ],
              "style": {"header_background": "accent1", "borders": "horizontal", "striped": true}
            }
          }
        ]
      }
    ]
  }
}
```

### Pattern 6: Card Grid with Spanning Chart Row

**Use for:** maturity models, capability assessments with supporting data

Same as Pattern 2, but add a third row with a spanning diagram cell:

```json
{
  "height": 50,
  "cells": [
    {
      "col_span": 4,
      "diagram": {
        "type": "bar",
        "title": "Assessment Scores",
        "data": {
          "categories": ["Cat A", "Cat B", "Cat C", "Cat D"],
          "series": [
            {"name": "Current", "values": [2.1, 3.4, 2.8, 1.9]},
            {"name": "Target", "values": [4.5, 4.8, 4.2, 3.8]}
          ]
        }
      }
    }
  ]
}
```

Note: use `diagram` (not `shape`) at the cell level for charts inside shape grids.
The `col_span` must equal the grid's column count.

---

## Invariants (Rules That Prevent Rendering Errors)

These are non-negotiable. Violating them causes broken slides.

### Shape Grid

1. **Cell count must match columns.** Every row: `sum(cell.col_span or 1 for each cell) == column count`. If `columns: 3`, every row needs cells spanning exactly 3 columns total.
2. **`columns` type matters.** Integer `3` = three equal columns. Array `[10, 90]` = proportional widths. Never use `[3]` (single column at width 3%) when you mean `3` (three equal columns).
3. **`bounds` uses percentages (0-100).** Not points, not EMU. `{"x": 5, "y": 18, "width": 90, "height": 72}` means 5% from left, 18% from top.
4. **`gap`/`row_gap`/`col_gap` are points.** Typical values: 8-12. These are NOT percentages.
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

11. **`placeholder_id` per layout type:**
    - `title` / `closing` layout: `title`, `subtitle`
    - `content` layout: `title`, `body`
    - `two-column` layout: `title`, `body`, `body_2`
    - `blank` layout: `title` only in `content`; body content goes in `shape_grid`
    - `section` layout: `title`, `subtitle`
12. **`fill` vocabulary.** Semantic: `accent1`-`accent6`, `lt1`, `lt2`, `dk1`, `dk2`, `hlink`, `folHlink`, `none`. Hex: `"#RRGGBB"`. Object: `{"color": "accent1", "alpha": 50}`. Never raw names like `"blue"`.
13. **`align` values:** `"l"`, `"ctr"`, `"r"`, `"just"`. NOT `"left"`, `"center"`, `"right"`.
14. **`vertical_align` values:** `"t"`, `"ctr"`, `"b"`. NOT `"top"`, `"middle"`, `"bottom"`.
15. **Templates:** `forest-green`, `midnight-blue`, `modern-template`, `warm-coral`.

---

## Common Mistakes

| Mistake | Fix |
|---|---|
| `"columns": [3]` (array with one element) | Use `"columns": 3` (integer) for 3 equal columns |
| Missing insets on body text cells | Always include all 4 insets (6-10pt) |
| `"type": "stacked-bar"` | Use underscores: `"type": "stacked_bar"` |
| `"align": "center"` | Use short form: `"align": "ctr"` |
| Chart in shape grid using `"shape"` | Use `"diagram"` at cell level for charts in grids |
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

- **Uniform:** All header cells use `accent1` (clean, professional)
- **Cycling:** Headers use `accent1`, `accent2`, `accent3`... (distinguishes categories)
- **Semantic:** Green accent for positive, red/coral for negative, neutral for baseline

Across the deck, keep accent usage consistent. If slide 3 uses accent1 for "Infrastructure",
slide 8 should use accent1 for "Infrastructure" too.

Body cells almost always use `"fill": "lt2"` with `"color": "dk1"` text.
Header cells use accent fills with `"color": "#FFFFFF"` text.

---

## Reference

For complete field specifications, chart types, diagram types, shape properties,
patch operations, and theme overrides, see:

`~/.claude/skills/template-deck/TEMPLATE_GUIDE.md`
