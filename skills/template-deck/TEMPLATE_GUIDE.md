# Template Guide

Reference for template selection, layout capabilities, and placeholder naming in go-slide-creator.

## Placeholder IDs

All placeholders use **normalized canonical IDs**. The binary automatically normalizes OOXML placeholder names at load time, so JSON input and skill-info output always use these IDs:

| Canonical ID | OOXML `ph.Type` | Description |
|---|---|---|
| `title` | `title`, `ctrTitle` | Slide title (one per layout) |
| `subtitle` | `subTitle` | Subtitle (typically on title slides) |
| `body` | `body` (or implicit) | Primary body/content area (leftmost) |
| `body_2` | `body` | Second body placeholder (by X position) |
| `body_3` | `body` | Third body placeholder |
| `image` | `pic` | Primary image placeholder (leftmost) |
| `image_2` | `pic` | Second image placeholder |

**Numbering rule:** The first placeholder in each role has no suffix. Subsequent placeholders are suffixed `_2`, `_3`, etc., ordered left-to-right by X offset (top-to-bottom as tiebreaker).

**Utility placeholders** (`dt`, `ftr`, `sldNum`, `hdr`) retain their original OOXML names and are not content-addressable.

### Legacy OOXML names (aliases)

Raw OOXML names like `Title 1`, `Content Placeholder 2`, `Text Placeholder 3` are accepted as legacy aliases in `placeholder_id`. The resolver maps them to the corresponding semantic role. However, always prefer canonical IDs (`title`, `body`, `body_2`) in new JSON input -- they are stable across templates.

## Layout Tags

Each layout receives classification tags based on its placeholder structure and display name. Use these tags to select the right layout for your slide type.

### Structural tags (from placeholder analysis)

| Tag | Rule |
|---|---|
| `title-slide` | Visible title, no body/image/chart placeholders |
| `content` | Visible title + at least one usable body placeholder |
| `title-hidden` | Title placeholder off-screen (Y < 0), body present |
| `title-at-bottom` | Title in lower 50% of slide, body present |
| `two-column` | 2+ body placeholders side-by-side (X gap > 10%) |
| `comparison` | Same as two-column (both tags applied together) |
| `image-left` | Image placeholder left of body |
| `image-right` | Image placeholder right of body |
| `full-image` | Large image (>50% slide area), no body |
| `chart-capable` | Contains chart placeholder |
| `blank` | No placeholders |

### Semantic tags (from layout display name)

| Tag | Matched keywords |
|---|---|
| `quote` | quote, quotation |
| `statement` | statement |
| `big-number` | number, metric, kpi, stats, statistic |
| `section-header` | section, divider, break, transition |
| `agenda` | agenda, outline, contents, overview |
| `timeline-capable` | timeline, process, roadmap, milestone |
| `icon-grid` | icon, grid, matrix |
| `closing` | closing, close, final, conclusion, end (word boundary) |
| `thank-you` | thank, thanks, q&a, questions |

## Layout-to-Slide-Type Mapping

When choosing a `layout_id`, pick a layout whose tags match your slide type:

| Slide type | Look for tags | Typical layout names |
|---|---|---|
| Title slide | `title-slide` | "Title Slide" |
| Content | `content` | "One Content", "Content" |
| Two-column | `two-column` | "Two Column (50/50)", "Two Content" |
| Section divider | `section-header` | "Section Divider", "Section Header" |
| Image slide | `image-left`, `image-right`, `full-image` | "Picture with Caption" |
| Chart slide | `chart-capable` or any `content` layout | "Content" (charts render as SVG in body) |
| Closing | `closing` | "Closing", "End Slide" |
| Blank | `blank` | "Blank" |

## Discovering Templates at Runtime

Use `json2pptx skill-info` to discover templates and their capabilities:

```bash
# List available templates
json2pptx skill-info --mode=list

# Get layout names and supported types
json2pptx skill-info --mode=compact --template=midnight-blue

# Get full placeholder details per layout
json2pptx skill-info --mode=full --template=midnight-blue
```

The `full` mode output shows normalized placeholder IDs, character limits, and EMU dimensions for each layout. Use `max_chars` to stay within placeholder capacity.

## Example: Selecting a Layout by Tag

```json
{
  "template": "midnight-blue",
  "slides": [
    {
      "layout_id": "Title Slide",
      "content": [
        {"placeholder_id": "title", "type": "text", "value": "Q1 2026 Review"},
        {"placeholder_id": "subtitle", "type": "text", "value": "Strategy Team"}
      ]
    },
    {
      "layout_id": "One Content",
      "content": [
        {"placeholder_id": "title", "type": "text", "value": "Key Metrics"},
        {"placeholder_id": "body", "type": "bullets", "value": ["Revenue +15%", "DAU +22%", "Churn -3%"]}
      ]
    },
    {
      "layout_id": "Two Column (50/50)",
      "content": [
        {"placeholder_id": "title", "type": "text", "value": "Comparison"},
        {"placeholder_id": "body", "type": "bullets", "value": ["Before: manual", "Slow turnaround"]},
        {"placeholder_id": "body_2", "type": "bullets", "value": ["After: automated", "Real-time"]}
      ]
    }
  ]
}
```

## Content Types

### text

Plain text. Value: `string` (via `text_value` or `value`).

### bullets

Bullet list. Value: `string[]` (via `bullets_value` or `value`).

### body_and_bullets

Body paragraph followed by bullet points in the same placeholder.

```json
{"placeholder_id": "body", "type": "body_and_bullets", "body_and_bullets_value": {
  "body": "Overview paragraph without bullet marker",
  "bullets": ["First point", "Second point"],
  "trailing_body": "Optional closing paragraph"
}}
```

### bullet_groups

Grouped bullets with per-group headers and body text. Useful for structured content like feature categories or team responsibilities.

```json
{"placeholder_id": "body", "type": "bullet_groups", "bullet_groups_value": {
  "body": "Optional introductory paragraph",
  "groups": [
    {"header": "Group A", "body": "Optional description", "bullets": ["Item 1", "Item 2"]},
    {"header": "Group B", "bullets": ["Item 3", "Item 4"]}
  ],
  "trailing_body": "Optional closing paragraph"
}}
```

### table

Data table with optional styling.

```json
{"placeholder_id": "body", "type": "table", "table_value": {
  "headers": ["Metric", "Q1", "Q2"],
  "rows": [["Revenue", "$10M", "$12M"], ["Growth", "8%", "15%"]],
  "style": {"header_background": "accent1", "borders": "horizontal", "striped": true},
  "column_alignments": ["l", "r", "r"]
}}
```

| Field | Type | Description |
|---|---|---|
| `headers` | `string[]` | Column headers |
| `rows` | `(string \| {content, col_span, row_span})[][]` | Row data — cells can be plain strings or objects for spanning |
| `style` | `object` | `header_background` (color), `borders` (`"horizontal"`, `"all"`, `"none"`), `striped` (bool) |
| `column_alignments` | `string[]` | Per-column alignment: `"l"`, `"c"`, `"r"` |

### chart

SVG chart rendered into a placeholder. Value: `ChartSpec` (via `chart_value` or `value`).

```json
{"placeholder_id": "body", "type": "chart", "chart_value": {
  "type": "bar",
  "title": "Revenue by Quarter",
  "data": {"categories": ["Q1", "Q2", "Q3"], "series": [{"name": "Revenue", "values": [10, 15, 22]}]}
}}
```

**Chart types** (15 types, use the short name in `type`):

| Type | Description |
|---|---|
| `bar` | Vertical bar chart |
| `line` | Line chart with markers |
| `pie` | Pie chart with segments |
| `donut` | Donut chart (pie with center hole) |
| `area` | Area chart (filled line) |
| `radar` | Radar/spider chart |
| `scatter` | Scatter plot (X-Y data) |
| `stacked_bar` | Stacked bar chart |
| `bubble` | Bubble chart (scatter + size) |
| `stacked_area` | Stacked area chart |
| `grouped_bar` | Grouped bar chart (side-by-side) |
| `waterfall` | Waterfall/bridge chart |
| `funnel` | Funnel chart |
| `gauge` | Gauge/speedometer |
| `treemap` | Hierarchical treemap |

### diagram

Native OOXML or SVG diagram rendered into a placeholder. Value: `DiagramSpec` (via `diagram_value` or `value`).

```json
{"placeholder_id": "body", "type": "diagram", "diagram_value": {
  "type": "timeline",
  "data": {"milestones": [{"date": "2026-Q1", "label": "Launch"}, {"date": "2026-Q3", "label": "Scale"}]}
}}
```

**Diagram types** (21 types):

| Type | Rendering | Description |
|---|---|---|
| `timeline` | SVG | Timeline with milestones |
| `process_flow` | Native OOXML | Step-by-step process flow |
| `pyramid` | Native OOXML | Layered pyramid diagram |
| `venn` | SVG | Venn diagram (2-4 sets) |
| `swot` | Native OOXML | SWOT analysis 2x2 |
| `org_chart` | SVG | Organization chart |
| `gantt` | SVG | Gantt chart with tasks |
| `matrix_2x2` | SVG | 2x2 matrix (e.g., BCG) |
| `porters_five_forces` | Native OOXML | Porter's Five Forces |
| `house_diagram` | Native OOXML | House/temple diagram |
| `business_model_canvas` | Native OOXML | Business Model Canvas |
| `value_chain` | Native OOXML | Value chain diagram |
| `nine_box_talent` | Native OOXML | 9-Box talent grid |
| `kpi_dashboard` | Native OOXML | Grid of KPI metric cards |
| `heatmap` | Native OOXML | Color-coded heatmap grid |
| `fishbone` | SVG | Fishbone/Ishikawa diagram |
| `pestel` | Native OOXML | PESTEL analysis |
| `panel_layout` | Native OOXML | Flexible panel layout (set `data.layout` to `"columns"`, `"rows"`, `"stat_cards"`, or `"grid"`) |
| `icon_columns` | Native OOXML | Alias for `panel_layout` with `layout: "columns"` |
| `icon_rows` | Native OOXML | Alias for `panel_layout` with `layout: "rows"` |
| `stat_cards` | Native OOXML | Alias for `panel_layout` with `layout: "stat_cards"` |

### image

Image embedded in a placeholder. Value: `ImageInput` (via `image_value` or `value`).

```json
{"placeholder_id": "image", "type": "image", "image_value": {"path": "assets/photo.jpg", "alt": "Team photo"}}
```

Supports `path` (local file) or `url` (HTTP/HTTPS download).

## Footer Configuration

Enable slide footers at the presentation level:

```json
{
  "template": "midnight-blue",
  "footer": {"enabled": true, "left_text": "Acme Corp | Confidential"},
  "slides": [...]
}
```

| Field | Type | Description |
|---|---|---|
| `enabled` | `boolean` | Master switch — must be `true` to inject footers |
| `left_text` | `string` | Left footer text (e.g., company name, confidentiality notice) |

## Theme Overrides

Override template colors and fonts at the presentation level:

```json
{
  "template": "midnight-blue",
  "theme_override": {
    "colors": {"accent1": "#FF6600", "dk1": "#1A1A1A"},
    "title_font": "Georgia",
    "body_font": "Verdana"
  },
  "slides": [...]
}
```

| Field | Type | Description |
|---|---|---|
| `colors` | `map[string]string` | Override theme color slots (e.g., `accent1`-`accent6`, `dk1`, `lt1`) |
| `title_font` | `string` | Override title font family |
| `body_font` | `string` | Override body font family |

## Slide-Level Fields

| Field | Type | Description |
|---|---|---|
| `speaker_notes` | `string` | Speaker notes text for the slide |
| `source` | `string` | Source attribution displayed on slide |
| `transition` | `string` | Slide transition type |
| `transition_speed` | `string` | Transition speed |
| `build` | `string` | Build animation for content |

## Patch Operations

Incrementally modify a presentation without regenerating all slides:

```json
{
  "base": {
    "template": "midnight-blue",
    "slides": [
      {"layout_id": "Title Slide", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Original Title"}]},
      {"layout_id": "One Content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Slide 2"}]}
    ]
  },
  "operations": [
    {"op": "replace", "slide_index": 0, "slide": {"layout_id": "Title Slide", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Updated Title"}]}},
    {"op": "add", "slide_index": 2, "slide": {"layout_id": "One Content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "New Slide"}]}},
    {"op": "remove", "slide_index": 1}
  ]
}
```

| Op | Description |
|---|---|
| `replace` | Replace slide at `slide_index` with `slide` |
| `add` | Insert `slide` at `slide_index` (shifts subsequent slides right) |
| `remove` | Remove slide at `slide_index` (shifts subsequent slides left) |

Operations are applied in order. Adjust indices for preceding add/remove ops.

## Shape Grid

The `shape_grid` field on a slide places a grid of preset geometry shapes (rectangles, rounded rectangles, chevrons, etc.) using a row/column layout engine. Use it for consulting-style layouts: process flows, 2x2 matrices, numbered panels, KPI cards, and any custom arrangement that placeholders can't express.

### Slide Setup

Set `slide_type` to `"blank"` (or `"virtual"`) and omit `layout_id`. The engine auto-selects a blank layout with title and computes grid bounds below the title area:

```json
{
  "slide_type": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "value": "Slide Title"}
  ],
  "shape_grid": { ... }
}
```

### Grid Structure

| Field     | Type                | Default      | Description |
|-----------|---------------------|--------------|-------------|
| `bounds`  | `{x, y, width, height}` | Auto-derived | Bounding rectangle as slide percentages (0-100) |
| `gap`     | `number`            | 0            | Uniform gap between cells (% of grid dimension) |
| `col_gap` | `number`            | `gap`        | Column gap override (% of grid width) |
| `row_gap` | `number`            | `gap`        | Row gap override (% of grid height) |
| `columns` | `number \| number[]` | Cell count   | Equal columns (`3`) or explicit widths (`[30, 40, 30]`) |
| `rows`    | `GridRowInput[]`    | ---          | Array of row definitions |

Each row has `cells` (array of cell objects), an optional `height` (percentage of grid height; 0 = equal split), and an optional `auto_height` (boolean; estimate height from text content). Rows can also have a `connector` object to draw lines or arrows between adjacent cells.

#### Row Connector

| Field   | Type   | Default  | Description |
|---------|--------|----------|-------------|
| `style` | string | `"line"` | `"arrow"` (with arrowhead) or `"line"` (plain) |
| `color` | string | `"000000"` | Hex color or scheme ref (e.g., `"accent1"`) |
| `width` | number | `1.0` | Line width in points |
| `dash`  | string | `"solid"` | `"solid"`, `"dash"`, `"dot"`, `"lgDash"`, `"dashDot"` |

### Cell Definition

Each cell holds one of `shape`, `table`, `icon`, or `image` (mutually exclusive). Optional `col_span` / `row_span` for merged cells (default 1). Optional `fit` controls shape scaling: `"contain"` (fit in cell preserving 1:1 ratio), `"fit-width"` (match cell width), `"fit-height"` (match cell height), or omit for stretch (default). An empty/null cell leaves the grid position blank.

Optional `accent_bar` adds a decorative bar alongside the cell:

| Field      | Type   | Default    | Description |
|------------|--------|------------|-------------|
| `position` | string | `"left"`   | `"left"`, `"right"`, `"top"`, `"bottom"` |
| `color`    | string | `"accent1"`| Hex color or scheme ref |
| `width`    | number | `4.0`      | Bar thickness in points |

### Icon Cell

Place a bundled SVG icon or custom SVG file in a grid cell. Exactly one of `name`, `path`, or `url` must be set.

| Field      | Type   | Description |
|------------|--------|-------------|
| `name`     | string | Bundled icon name (e.g., `"chart-pie"`, `"filled:alert-circle"`) |
| `path`     | string | File path to a custom SVG icon (relative to JSON input directory) |
| `url`      | string | HTTP/HTTPS URL to download an SVG icon from |
| `fill`     | string | Fill color override (hex, e.g., `"#FF0000"`). Only for bundled icons. |
| `position` | string | Position relative to text in shape: `"left"`, `"top"`, `"center"`. Auto-detected if empty. |

Icons can also be nested inside a shape cell using `"shape": {"geometry": "roundRect", "fill": "accent1", "icon": {"name": "shield"}}` to overlay an icon on a filled shape.

### Image Cell

Embed a raster image (PNG, JPG) in a grid cell with optional overlay and text.

| Field     | Type   | Description |
|-----------|--------|-------------|
| `path`    | string | File path to the image |
| `url`     | string | HTTP/HTTPS URL to download the image from |
| `alt`     | string | Alt text for accessibility |
| `overlay` | object | Semi-transparent overlay: `{"color": "000000", "alpha": 0.4}` |
| `text`    | object | Text label on top of image: `{"content": "...", "size": 14, "bold": true, "color": "FFFFFF", "align": "ctr", "vertical_align": "b"}` |

### Shape Properties

| Field         | Type               | Default   | Description |
|---------------|---------------------|-----------|-------------|
| `geometry`    | `string`            | ---       | Preset name: `rect`, `roundRect`, `ellipse`, `diamond`, `triangle`, `hexagon`, `chevron`, `rightArrow`, `star5`, `plus`, `donut`, `flowChartProcess`, `flowChartDecision`, `flowChartTerminator` |
| `fill`        | `string \| object`  | none      | `"#hex"`, scheme name (`accent1`, `lt1`, `dk2`, etc.), `"none"`, or `{"color": "...", "alpha": 0.5}` |
| `line`        | `string \| object`  | none      | Same color syntax, or `{"color": "...", "width": 2, "dash": "dash"}` |
| `text`        | `string \| object`  | ---       | Plain string or full text object (see below) |
| `icon`        | `object`            | ---       | Optional icon overlay: `{"name": "shield", "fill": "#FFFFFF"}` (see Icon Cell) |
| `rotation`    | `number`            | 0         | Rotation in degrees |
| `adjustments` | `map[string]int64`  | ---       | OOXML adjustment values for geometry tweaking |

#### Fill: Theme Color Names

Use these scheme names for template-consistent colors: `accent1`-`accent6`, `dk1`, `dk2`, `lt1`, `lt2`, `tx1`, `tx2`, `bg1`, `bg2`, `hlink`, `folHlink`.

#### Text Object

| Field            | Type     | Default   | Description |
|------------------|----------|-----------|-------------|
| `content`        | `string` | ---       | Text content (supports `\n` for line breaks, `<b>`, `<i>`, `<u>` inline) |
| `size`           | `number` | ---       | Font size in points |
| `bold`           | `boolean`| `false`   | Bold text |
| `italic`         | `boolean`| `false`   | Italic text |
| `align`          | `string` | `"ctr"`   | Horizontal: `"l"`, `"ctr"`, `"r"` |
| `vertical_align` | `string` | `"ctr"`   | Vertical: `"t"`, `"ctr"`, `"b"` |
| `color`          | `string` | ---       | `"#hex"` or scheme name |
| `font`           | `string` | `"+mn-lt"`| Font name or theme reference |
| `inset_top` / `inset_bottom` / `inset_left` / `inset_right` | `number` | 0 | Text insets in points |

### Example: Process Flow (Chevron Header + Detail Cards)

```json
{
  "slide_type": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "value": "Our Process"}
  ],
  "shape_grid": {
    "gap": 3,
    "columns": 4,
    "rows": [
      {
        "height": 22,
        "cells": [
          {"shape": {"geometry": "chevron", "fill": "accent1", "text": {"content": "Discover", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "chevron", "fill": "accent1", "text": {"content": "Design", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "chevron", "fill": "accent1", "text": {"content": "Deliver", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "chevron", "fill": "accent1", "text": {"content": "Sustain", "size": 13, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "cells": [
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Stakeholder interviews\nCurrent state assessment", "size": 9, "align": "l", "vertical_align": "t", "inset_top": 8, "inset_left": 6, "inset_right": 6}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Solution architecture\nRoadmap development", "size": 9, "align": "l", "vertical_align": "t", "inset_top": 8, "inset_left": 6, "inset_right": 6}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Agile implementation\nChange management", "size": 9, "align": "l", "vertical_align": "t", "inset_top": 8, "inset_left": 6, "inset_right": 6}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Performance monitoring\nKnowledge transfer", "size": 9, "align": "l", "vertical_align": "t", "inset_top": 8, "inset_left": 6, "inset_right": 6}}}
        ]
      }
    ]
  }
}
```

### Example: 2x2 Matrix with Column Spanning Header

```json
{
  "slide_type": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "value": "Priority Matrix"}
  ],
  "shape_grid": {
    "gap": 4,
    "columns": 2,
    "rows": [
      {
        "height": 15,
        "cells": [
          {"col_span": 2, "shape": {"geometry": "roundRect", "fill": "accent1", "text": {"content": "Impact vs Effort", "size": 16, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "cells": [
          {"shape": {"geometry": "roundRect", "fill": "accent1", "text": {"content": "Quick Wins\n\nHigh Impact / Low Effort", "size": 11, "color": "#FFFFFF", "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "accent2", "text": {"content": "Strategic Bets\n\nHigh Impact / High Effort", "size": 11, "color": "#FFFFFF", "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 10}}}
        ]
      },
      {
        "cells": [
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Fill-Ins\n\nLow Impact / Low Effort", "size": 11, "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 10}}},
          {"shape": {"geometry": "roundRect", "fill": "lt2", "text": {"content": "Deprioritize\n\nLow Impact / High Effort", "size": 11, "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 10}}}
        ]
      }
    ]
  }
}
```

### Tips

- **Gaps**: Use `gap: 3`-`6` for clean spacing. Use `row_gap` / `col_gap` for asymmetric spacing.
- **Column widths**: `columns: [8, 92]` creates a narrow numbering column + wide content column.
- **Row heights**: Set `height` on header rows (e.g., `18`-`22`) and leave content rows at 0 (equal split).
- **Text insets**: Always set `inset_top`, `inset_left`, `inset_right` (6-12pt) on content-heavy shapes to avoid text touching edges.
- **Theme colors**: Prefer scheme names (`accent1`, `lt2`) over hex codes for template portability.

## Character Limits

Each placeholder has a `max_chars` value computed from its physical dimensions and default font size. Exceeding this limit causes text overflow or auto-shrinking.

Guidelines:
- **Title**: typically 60-350 chars depending on layout
- **Subtitle**: typically 25-120 chars
- **Body**: typically 400-1000 chars (varies widely by layout)

Always check the `max_chars` from `skill-info --mode=full` for the specific template and layout you are using.

## Slide Background

Any slide can have a custom background image:

```json
{
  "layout_id": "One Content",
  "background": {"image": "assets/hero.jpg", "fit": "cover"},
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Title Over Image"}
  ]
}
```

Use `"url"` instead of `"image"` to download from a URL. Fit modes: `"cover"` (default, fills slide), `"stretch"`, `"tile"`.

## Font Size Override

Override the template default font size on any text-based content item:

```json
{"placeholder_id": "title", "type": "text", "text_value": "Big Title", "font_size": 72}
```
