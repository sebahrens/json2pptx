# JSON Input Format — Tutorial

`json2pptx` accepts a JSON object describing a presentation and renders it into a `.pptx` file. This page walks through the format with worked examples; for the **canonical list of fields, types, enums, and required-vs-optional flags**, query the schema directly.

## Canonical schema (single source of truth)

- MCP: `get_input_schema` — returns the JSON Schema for `PresentationInput`, with `x-field-scope` (`deck` / `slide` / `content` / `shape` / `split`), inline `enum` arrays, and discriminator constraints. Digest-cacheable.
- CLI: `json2pptx input-schema` — same payload, printed to stdout.

Both are generated from the Go input structs in `cmd/json2pptx/json_schema.go`. This tutorial only describes shapes and gives examples; the schema is authoritative on field names, types, and required-vs-optional.

## Minimum complete deck

```json
{
  "template": "warm-coral",
  "output_filename": "review.pptx",
  "slides": [
    {
      "layout_id": "title",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Q1 Review"}
      ]
    }
  ]
}
```

A presentation is a top-level object with a `template` and a non-empty `slides` array. Each slide pins a layout via `layout_id` (canonical id, e.g. `title`, `content`, `two-column`, `section`, `blank`) or hints `slide_type` (`title`, `content`, `chart`, `section`, `two-column`, `diagram`, `image`, `comparison`, `blank`). At least one of the two must be set.

## A typical content slide

```json
{
  "layout_id": "content",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Highlights"},
    {"placeholder_id": "body",  "type": "bullets",
     "bullets_value": ["Revenue +25%", "NPS at all-time high", "OpEx flat"]}
  ],
  "speaker_notes": "Hit the revenue point first."
}
```

A slide's `content` array is a list of typed items, each targeting a placeholder by its canonical id (`title`, `subtitle`, `body`, `body_2`, `body_3`, `image`, `image_2`). The schema enforces the `type` → typed-value pairing via an `if`/`then` discriminator chain:

- `type: "text"`             → requires `text_value` (string)
- `type: "bullets"`          → requires `bullets_value` (string array)
- `type: "body_and_bullets"` → requires `body_and_bullets_value`
- `type: "body_and_lead"`    → requires `body_and_lead_value`
- `type: "bullet_groups"`    → requires `bullet_groups_value`
- `type: "table"`            → requires `table_value`
- `type: "chart"`            → requires `chart_value`
- `type: "diagram"`          → requires `diagram_value`
- `type: "image"`            → requires `image_value`

Other `*_value` fields are forbidden for the chosen type. The legacy raw `value` field is still accepted (unconstrained) for backward compatibility.

## Inline formatting

Text and bullet strings accept three inline tags: `<b>`, `<i>`, `<u>`. They can be nested: `<b><i>bold italic</i></b>`. Plain dashes/arrows (`→`, `•`, en/em dashes) are allowed; **emoji codepoints are rejected anywhere** in deck JSON.

## A custom visual: `shape_grid`

For slides where placeholders aren't expressive enough, use `shape_grid` on a `blank` layout:

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Strategic Pillars"}
  ],
  "shape_grid": {
    "columns": 3,
    "gap": 4,
    "rows": [
      {"cells": [
        {"shape": {"geometry": "roundRect", "fill": "accent1",
          "text": {"content": "Innovation\nR&D + emerging tech",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}},
        {"shape": {"geometry": "roundRect", "fill": "accent2",
          "text": {"content": "Growth\nNew markets",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}},
        {"shape": {"geometry": "roundRect", "fill": "accent3",
          "text": {"content": "Efficiency\nAutomation",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}}
      ]}
    ]
  }
}
```

Each cell holds exactly one of: `shape`, `table`, `icon`, `image`, `diagram`, or `composite`. Cells can span columns/rows via `col_span` / `row_span`. Slide-level `overlays` can float arrows, lines, and badges over the grid; anchor them to cells by `(row, col, at)` or by percent-of-slide coordinates.

## Named patterns (prefer over hand-built grids)

For common business slide shapes — KPI cards, process flows, BMC canvas, matrix-2x2, roadmap, strategy house, SCQA summary, agenda, comparison, pull-quote — use a named pattern instead of hand-authoring a `shape_grid`. Patterns are registered in `internal/patterns/` and discoverable via:

- MCP: `list_patterns`, `show_pattern`, `expand_pattern`, `recommend_visual`
- CLI: `json2pptx patterns list`

Pattern field shapes and overrides are documented in `docs/PATTERNS.md`.

## Charts and diagrams

```json
{
  "placeholder_id": "body",
  "type": "chart",
  "chart_value": {
    "type": "bar_chart",
    "title": "Revenue by Quarter ($M)",
    "data": [
      {"label": "Q1", "value": 12},
      {"label": "Q2", "value": 18}
    ]
  }
}
```

Supported chart types: `bar_chart`, `line_chart`, `pie_chart`, `donut_chart`, `area_chart`, `radar_chart`, `scatter_chart`, `bubble_chart`, `stacked_bar_chart`, `stacked_area_chart`, `grouped_bar_chart`, `waterfall`, `funnel_chart`, `gauge_chart`, `treemap_chart`.

### Legend defaults: direct labels for 2–4 series

Bar / line / grouped-bar / area charts with 2–4 series default to **inline
series labels** (at the line endpoint, or above the last bar of each series)
in place of a legend — per `tokens.ChartDirectLabelMaxSeries`. Above 4 series
the legend reappears because in-plot labels collide. Force the legend back on
with `chart_value.style.show_legend: true`. Stacked variants and non-Cartesian
chart types (pie, donut, scatter, radar, waterfall, funnel, gauge, treemap)
are unaffected.

### Per-slide chart-style overrides

Add a `chart_style` block on a `chart_value` (or `diagram_value`) to flip an
executive-default token for one chart. Omit the block to keep the deck-wide
defaults. Each field is optional:

| field | type | default | effect |
|---|---|---|---|
| `show_vertical_gridlines` | bool | `false` | When `true`, draws vertical gridlines on bar/line/area charts in addition to the horizontal ones. |
| `show_single_series_legend` | bool | `false` | When `true`, renders the legend even when the chart has a single series (default suppresses it because the title carries the label). |

```json
{
  "type": "chart",
  "chart_value": {
    "type": "bar",
    "data": {"Q1": 12, "Q2": 18, "Q3": 22, "Q4": 31},
    "chart_style": {
      "show_vertical_gridlines": true
    }
  }
}
```

```json
{
  "placeholder_id": "body",
  "type": "diagram",
  "diagram_value": {
    "type": "swot",
    "data": {
      "strengths":     ["Strong brand", "Loyal customers"],
      "weaknesses":    ["High costs"],
      "opportunities": ["Emerging markets"],
      "threats":       ["Competition"]
    }
  }
}
```

Supported diagram types: `timeline`, `process_flow`, `pyramid`, `venn`, `swot`, `org_chart`, `gantt`, `matrix_2x2`, `porters_five_forces`, `house_diagram`, `business_model_canvas`, `value_chain`, `nine_box_talent`, `kpi_dashboard`, `heatmap`, `fishbone`, `pestel`, `panel_layout`. See `ChartSpec.type` and `DiagramSpec.type` in the schema for the authoritative list.

Charts and diagrams render to SVG and embed into the slide.

## Theme override

The deck's visual identity comes from the chosen `template`. Override deck-wide colors or fonts with `theme_override`:

```json
{
  "theme_override": {
    "colors": {"accent1": "#E31837"},
    "title_font": "Georgia",
    "body_font": "Arial"
  }
}
```

In `design_mode: "constrained"` (the default), raw hex colors are restricted; switch to `design_mode: "free"` for exploratory/artistic decks.

## Footer, page numbers, and structure

`footer`, `chrome.page_numbers`, and `structure.sections` configure deck chrome and section grouping. See the schema for the full set; agents typically only set `footer.enabled` and `footer.left_text`.

## Patch input

Editing tools accept a *patch* envelope rather than a full `PresentationInput`:

```json
{
  "base": {
    "template": "midnight-blue",
    "slides": [
      {"layout_id": "title",   "content": [{"placeholder_id": "title", "type": "text", "text_value": "Original"}]},
      {"layout_id": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Slide 2"}]}
    ]
  },
  "operations": [
    {"op": "replace", "slide_index": 0, "slide": {"layout_id": "title",   "content": [{"placeholder_id": "title", "type": "text", "text_value": "Updated"}]}},
    {"op": "add",     "slide_index": 2, "slide": {"layout_id": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "New"}]}},
    {"op": "remove",  "slide_index": 1}
  ]
}
```

Operations are applied in order; indices are 0-based. The patch envelope is detected automatically when an input contains an `operations` array.

## Local asset paths

`icon.path`, `image_value.path`, shape-grid `image.path`, and `background.image` accept two convenience expansions before resolution against the input base directory:

- A leading `~/` (or bare `~`) expands to the invoking user's home directory.
- `$VAR` and `${VAR}` expand via the process environment.

So `~/assets/logo.svg` and `$BRAND_ASSETS/logo.svg` resolve as expected instead of being passed literally to the filesystem. Unset environment variables yield an `ASSET_PATH_ENV_UNSET` finding (with `details.env_variable` naming the missing var) rather than silently collapsing to an empty path. Traversal and symlink protections still apply against the expanded path.

## Validating before generating

- CLI: `json2pptx validate <input.json>` — same validator the engine runs.
- CLI: `json2pptx validate <input.json> -fit-report` — adds layout-fit diagnostics.
- MCP: `validate_input` — wraps both.

Errors carry structured `code` fields catalogued in `docs/FIT_FINDINGS.md` (with severity and recommended action). The validator accepts both canonical and alias enum values for backward compatibility; the schema publishes only the canonical names.

## Where to go next

- `SLIDE_FORMAT.md` — even shorter quickstart.
- `docs/PATTERNS.md` — named-pattern authoring guide.
- `docs/FIT_FINDINGS.md` — finding-code catalogue.
- `docs/STYLE_DEFAULTS.md` — deck-level defaults for table and cell styles.
- `skills/generate-deck/` — agent-facing workflow, rules, and pattern recommendations.
