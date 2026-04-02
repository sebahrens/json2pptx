# 25 — Shape Grid

## Summary

The shape grid system allows slides to contain a grid of preset geometry shapes (rectangles, rounded rectangles, circles, etc.) positioned using a flexible row/column layout engine. Grid cells can hold shapes with styled text or data tables.

## JSON Schema

### ShapeGridInput

Top-level grid definition, placed on a `SlideInput` via the `shape_grid` field.

```json
{
  "shape_grid": {
    "bounds": {"x": 5, "y": 18, "width": 90, "height": 72},
    "gap": 2,
    "col_gap": 2,
    "row_gap": 3,
    "columns": 3,
    "rows": [
      { "cells": [ ... ] }
    ]
  }
}
```

| Field     | Type                | Required | Default      | Description |
|-----------|---------------------|----------|--------------|-------------|
| `bounds`  | `GridBoundsInput`   | No       | Auto-derived | Bounding rectangle as slide percentages |
| `gap`     | `number`            | No       | 0            | Uniform gap between cells (percentage of grid dimension) |
| `col_gap` | `number`            | No       | `gap`        | Column gap override (percentage of grid width) |
| `row_gap` | `number`            | No       | `gap`        | Row gap override (percentage of grid height) |
| `columns` | `number \| number[]` | No       | Cell count   | Number of equal columns, or array of column width percentages |
| `rows`    | `GridRowInput[]`    | Yes      | —            | Array of row definitions |

### GridBoundsInput

Bounding rectangle in slide-percentage coordinates.

| Field    | Type     | Description |
|----------|----------|-------------|
| `x`      | `number` | Left edge (0–100, percentage of slide width) |
| `y`      | `number` | Top edge (0–100, percentage of slide height) |
| `width`  | `number` | Width (percentage of slide width) |
| `height` | `number` | Height (percentage of slide height) |

When omitted, bounds are auto-derived from the layout (see [24-virtual-layouts](24-virtual-layouts.md)).

### GridRowInput

| Field         | Type              | Required | Default | Description |
|---------------|-------------------|----------|---------|-------------|
| `cells`       | `GridCellInput[]` | Yes      | —       | Array of cells in this row |
| `height`      | `number`          | No       | 0       | Row height as percentage of grid height (0 = equal split) |
| `auto_height` | `boolean`         | No       | `false` | Estimate row height from text content |
| `connector`   | `ConnectorInput`  | No       | —       | Draw lines/arrows between adjacent cells |

When `height` is 0 (default), all rows share remaining height equally after fixed-height rows are allocated.

#### ConnectorInput

Draws connecting lines or arrows between adjacent cells in a row.

| Field   | Type     | Default    | Description |
|---------|----------|------------|-------------|
| `style` | `string` | `"line"`   | `"arrow"` (with arrowhead) or `"line"` (plain) |
| `color` | `string` | `"000000"` | Hex color or scheme reference (e.g., `"accent1"`) |
| `width` | `number` | `1.0`      | Line width in points |
| `dash`  | `string` | `"solid"`  | `"solid"`, `"dash"`, `"dot"`, `"lgDash"`, `"dashDot"` |

### GridCellInput

Each cell holds one of `shape`, `table`, `icon`, or `image` (mutually exclusive). An empty/null cell leaves the grid position blank.

| Field        | Type              | Required | Default | Description |
|--------------|-------------------|----------|---------|-------------|
| `col_span`   | `integer`         | No       | 1       | Number of columns this cell spans |
| `row_span`   | `integer`         | No       | 1       | Number of rows this cell spans |
| `shape`      | `ShapeSpecInput`  | No       | —       | Preset geometry shape |
| `table`      | `TableInput`      | No       | —       | Data table (same schema as content type `table`) |
| `icon`       | `IconCellInput`   | No       | —       | SVG icon (bundled or custom) |
| `image`      | `ImageCellInput`  | No       | —       | Raster image (PNG, JPG) with optional overlay |
| `fit`        | `string`          | No       | stretch | Shape scaling: `"contain"`, `"fit-width"`, `"fit-height"`, or omit for stretch |
| `accent_bar` | `AccentBarInput`  | No       | —       | Decorative bar alongside the cell |

### IconCellInput

Place a bundled SVG icon or custom SVG file. Exactly one of `name`, `path`, or `url` must be set.

| Field      | Type     | Description |
|------------|----------|-------------|
| `name`     | `string` | Bundled icon name (e.g., `"chart-pie"`, `"filled:alert-circle"`) |
| `path`     | `string` | File path to a custom SVG icon |
| `url`      | `string` | HTTP/HTTPS URL to download an SVG icon |
| `fill`     | `string` | Fill color override (hex). Only for bundled icons. |
| `position` | `string` | Position relative to text: `"left"`, `"top"`, `"center"`. Auto-detected if empty. |

Icons can also be nested inside a shape cell using `"shape": {"icon": {"name": "shield"}}` to overlay an icon on a filled shape.

### ImageCellInput

Embed a raster image with optional overlay and text label.

| Field     | Type     | Description |
|-----------|----------|-------------|
| `path`    | `string` | File path to the image |
| `url`     | `string` | HTTP/HTTPS URL to download the image |
| `alt`     | `string` | Alt text for accessibility |
| `overlay` | `object` | Semi-transparent overlay: `{"color": "000000", "alpha": 0.4}` |
| `text`    | `object` | Text label on image: `{"content": "...", "size": 14, "bold": true, "color": "FFFFFF", "align": "ctr", "vertical_align": "b"}` |

### AccentBarInput

Decorative bar alongside a cell edge.

| Field      | Type     | Default    | Description |
|------------|----------|------------|-------------|
| `position` | `string` | `"left"`   | `"left"`, `"right"`, `"top"`, `"bottom"` |
| `color`    | `string` | `"accent1"`| Hex color or scheme reference |
| `width`    | `number` | `4.0`      | Bar thickness in points |

### ShapeSpecInput

Defines a preset geometry shape with optional fill, line, text, and rotation.

| Field         | Type               | Required | Default | Description |
|---------------|---------------------|----------|---------|-------------|
| `geometry`    | `string`            | Yes      | —       | OOXML preset geometry name (e.g., `"rect"`, `"roundRect"`, `"ellipse"`) |
| `fill`        | `string \| object`  | No       | —       | Fill color (`"#hex"`), scheme name (`accent1`), `"none"`, or `{"color": "...", "alpha": 0.5}` |
| `line`        | `string \| object`  | No       | —       | Same color syntax, or `{"color": "...", "width": 2, "dash": "dash"}` |
| `text`        | `string \| ShapeTextInput` | No | —     | Text content — string shorthand or full object |
| `icon`        | `object`            | No       | —       | Icon overlay: `{"name": "shield", "fill": "#FFFFFF"}` (see IconCellInput) |
| `rotation`    | `number`            | No       | 0       | Rotation in degrees |
| `adjustments` | `map[string]int64`  | No       | —       | Preset geometry adjustment values |

#### Common Geometry Values

| Geometry      | Description          |
|---------------|----------------------|
| `rect`        | Rectangle            |
| `roundRect`   | Rounded rectangle    |
| `ellipse`     | Ellipse / circle     |
| `diamond`     | Diamond              |
| `triangle`    | Triangle             |
| `hexagon`     | Hexagon              |
| `chevron`     | Chevron arrow        |
| `rightArrow`  | Right arrow          |
| `star5`       | 5-pointed star       |

### ShapeTextInput

Text content and formatting for a shape. When `text` is a plain string, it is treated as `{"content": "..."}` with defaults.

| Field            | Type     | Required | Default   | Description |
|------------------|----------|----------|-----------|-------------|
| `content`        | `string` | Yes      | —         | Text content (supports `<b>`, `<i>`, `<u>` inline formatting) |
| `size`           | `number` | No       | —         | Font size in points |
| `bold`           | `boolean`| No       | `false`   | Bold text |
| `italic`         | `boolean`| No       | `false`   | Italic text |
| `align`          | `string` | No       | `"ctr"`   | Horizontal alignment: `"l"` (left), `"ctr"` (center), `"r"` (right) |
| `vertical_align` | `string` | No       | `"ctr"`   | Vertical alignment: `"t"` (top), `"ctr"` (middle), `"b"` (bottom) |
| `color`          | `string` | No       | —         | Text color: `"#hex"` or scheme name (e.g., `"lt1"`) |
| `font`           | `string` | No       | `"+mn-lt"`| Font name or `"+mn-lt"` for theme body font, `"+mj-lt"` for theme title font |
| `inset_left`     | `number` | No       | 0         | Left text inset in points |
| `inset_right`    | `number` | No       | 0         | Right text inset in points |
| `inset_top`      | `number` | No       | 0         | Top text inset in points |
| `inset_bottom`   | `number` | No       | 0         | Bottom text inset in points |

## Grid Layout Engine

### Column Resolution

- **Integer** `columns: 3` — three equal-width columns.
- **Array** `columns: [30, 40, 30]` — explicit percentage widths (must sum to 100 or are normalized).
- **Omitted** — column count inferred from the maximum cell count across all rows.

### Row Distribution

All rows receive equal height. Row count is determined by the length of the `rows` array.

### Gap Handling

Gaps are subtracted from the total grid dimension before distributing space to cells:

```
available_width  = grid_width  - (col_gap * (num_columns - 1))
available_height = grid_height - (row_gap * (num_rows - 1))
```

### Cell Spanning

Cells with `col_span > 1` or `row_span > 1` occupy multiple grid positions. The layout engine tracks occupied positions to prevent overlaps.

## Cell Content Types

### Shape Cells

Shape cells generate `<p:sp>` XML elements with:

- Preset geometry (`<a:prstGeom>`)
- Solid or gradient fill (`<a:solidFill>` / `<a:gradFill>`)
- Line style (`<a:ln>`)
- Text body with paragraph formatting (`<p:txBody>`)
- Rotation transform

### Table Cells

Table cells generate `<a:graphicFrame>` XML elements using the same table rendering pipeline as the `table` content type. The table fills the cell's computed bounds.

## Validation Rules

`Validate()` in `internal/shapegrid/validate.go` checks grids for structural errors before rendering. All violations are collected and returned as a combined error.

| Rule | Error |
|------|-------|
| Empty columns | `"shape_grid: empty columns"` |
| No rows | `"shape_grid: no rows defined"` |
| Cell has both shape and table | `"cell has both shape and table (only one allowed)"` |
| Invalid fit mode | `"invalid fit mode; valid values are contain, fit-width, fit-height"` |
| col_span exceeds grid width | `"col_span N exceeds grid width M"` |
| row_span exceeds grid height | `"row_span N exceeds grid height M"` |
| Cell overlap from spanning | `"cell overlap at row R col C"` |

## Key Files

| File | Purpose |
|------|---------|
| `cmd/json2pptx/json_schema.go` | JSON input DTOs: `ShapeGridInput`, `GridCellInput`, `ShapeSpecInput`, `ShapeTextInput` |
| `cmd/json2pptx/shape_grid.go` | JSON-to-domain conversion, virtual layout resolution |
| `internal/shapegrid/types.go` | Domain types: `Grid`, `Row`, `Cell`, `ShapeSpec`, `ResolvedCell` |
| `internal/shapegrid/grid.go` | `Resolve()` — grid layout engine, column/row resolution |
| `internal/shapegrid/bounds.go` | Bounds computation helpers |
| `internal/shapegrid/shape.go` | `GenerateShapeXML()`, `ResolveTextInput()`, fill/line parsing |
| `internal/shapegrid/validate.go` | `Validate()` — structural validation before rendering |

## Example: Two-Column Grid with Table and Shape

```json
{
  "layout_id": "slideLayout2",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Q1 Results"}
  ],
  "shape_grid": {
    "columns": [55, 45],
    "gap": 2,
    "rows": [
      {
        "cells": [
          {
            "table": {
              "headers": ["Metric", "Value"],
              "rows": [["Revenue", "$21M"], ["Growth", "+17%"]],
              "style": {"header_background": "accent1", "borders": "horizontal"}
            }
          },
          {
            "shape": {
              "geometry": "roundRect",
              "fill": "#4472C4",
              "text": {
                "content": "Key Takeaway:\nStrong growth across all segments",
                "size": 14,
                "bold": true,
                "color": "#FFFFFF",
                "vertical_align": "ctr",
                "inset_left": 12,
                "inset_right": 12,
                "font": "Arial"
              }
            }
          }
        ]
      }
    ]
  }
}
```
