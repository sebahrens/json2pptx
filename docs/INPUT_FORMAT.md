# JSON Input Format Reference

This document describes the complete JSON input format accepted by `go-slide-creator`. The input types are defined in `cmd/json2pptx/json_schema.go` and `cmd/json2pptx/json_mode.go`.

## Document Structure

A presentation is defined by a single JSON object with a `template` name and an array of `slides`:

```json
{
  "template": "warm-coral",
  "output_filename": "Q1_Review.pptx",
  "slides": [
    {
      "layout_id": "slideLayout1",
      "content": [
        {
          "placeholder_id": "title",
          "type": "text",
          "text_value": "Q1 2026 Business Review"
        }
      ]
    }
  ]
}
```

---

## 1. Top-Level Schema (PresentationInput / JSONInput)

| Field              | Required | Type            | Description                                                    |
|--------------------|----------|-----------------|----------------------------------------------------------------|
| `template`         | Yes      | string          | Template name (without `.pptx` extension)                      |
| `output_filename`  | No       | string          | Desired output filename (default: `output.pptx`)               |
| `footer`           | No       | object          | Footer configuration (see below)                               |
| `theme_override`   | No       | object          | Per-deck color and font overrides (see below)                  |
| `slides`           | Yes      | array           | Array of slide definitions                                     |

### Footer Configuration

```json
{
  "footer": {
    "enabled": true,
    "left_text": "Acme Corp | Confidential"
  }
}
```

| Field       | Type    | Description                                              |
|-------------|---------|----------------------------------------------------------|
| `enabled`   | boolean | Master switch — when false, no footers are injected      |
| `left_text` | string  | Left footer text (e.g., "Acme Corp \| Confidential")    |

### Theme Override

Override template colors and fonts for the entire deck:

```json
{
  "theme_override": {
    "colors": {
      "accent1": "#E31837",
      "accent2": "#2E75B6",
      "dk1": "#000000",
      "lt1": "#FFFFFF"
    },
    "title_font": "Georgia",
    "body_font": "Arial"
  }
}
```

| Field        | Type              | Description                                                   |
|--------------|-------------------|---------------------------------------------------------------|
| `colors`     | map[string]string | Color overrides: `accent1`–`accent6`, `dk1`, `dk2`, `lt1`, `lt2`, `hlink`, `folHlink` |
| `title_font` | string            | Font name for titles                                          |
| `body_font`  | string            | Font name for body text                                       |

---

## 2. Slide Schema (SlideInput)

Each slide specifies a layout, content items, and optional metadata.

```json
{
  "layout_id": "slideLayout2",
  "slide_type": "content",
  "content": [ ... ],
  "speaker_notes": "Emphasize the Q4 recovery.",
  "source": "Company Annual Report, FY2025",
  "transition": "fade",
  "transition_speed": "med",
  "build": "bullets"
}
```

| Field              | Required | Type   | Description                                                        |
|--------------------|----------|--------|--------------------------------------------------------------------|
| `layout_id`        | No       | string | Layout identifier (e.g., `"slideLayout1"`, `"Title Slide"`)       |
| `slide_type`       | No       | string | Type hint: `content`, `title`, `section`, `chart`, `two-column`, `diagram`, `image`, `comparison`, `blank` |
| `content`          | Yes      | array  | Array of content items for placeholders                            |
| `shape_grid`       | No       | object | Grid of preset geometry shapes (see Shape Grid section)            |
| `background`       | No       | object | Slide background image (see Background section below)              |
| `speaker_notes`    | No       | string | Speaker notes text (not rendered on slide)                         |
| `source`           | No       | string | Source attribution text (small text at slide bottom)               |
| `transition`       | No       | string | Slide transition: `fade`, `push`, `wipe`, `cover`, `cut`, `none`  |
| `transition_speed` | No       | string | Transition speed: `slow`, `med`, `fast`                            |
| `build`            | No       | string | Build animation: `"bullets"` for one-by-one bullet reveal          |

Either `layout_id` or `slide_type` (or both) should be provided. `layout_id` takes precedence for layout selection.

### Background

Set a slide background image from a local file or URL:

```json
{
  "background": {
    "image": "assets/hero-bg.png",
    "fit": "cover"
  }
}
```

Or from a URL:

```json
{
  "background": {
    "url": "https://example.com/background.jpg",
    "fit": "stretch"
  }
}
```

| Field   | Type   | Default  | Description                                     |
|---------|--------|----------|-------------------------------------------------|
| `image` | string | ---      | Local file path to background image             |
| `url`   | string | ---      | HTTP/HTTPS URL to download background image     |
| `fit`   | string | `cover`  | Sizing mode: `"cover"`, `"stretch"`, `"tile"`   |

Only one of `image` or `url` should be set.

---

## 3. Content Items (ContentInput)

Each content item targets a placeholder and carries typed content. The `type` field determines which value field to use.

```json
{
  "placeholder_id": "body",
  "type": "bullets",
  "bullets_value": ["Revenue up 25%", "Margins improved"]
}
```

| Field            | Required | Type   | Description                                          |
|------------------|----------|--------|------------------------------------------------------|
| `placeholder_id` | Yes      | string | Target placeholder identifier                        |
| `type`           | Yes      | string | Content type discriminator (see table below)         |
| `value`          | No       | any    | Legacy value field (JSON raw message, for backward compatibility) |
| `font_size`      | No       | number | Override font size in points (e.g., `72`). Only applies to text-based types. |

### Content Types

| Type                | Value Field              | Value Type       | Description                          |
|---------------------|--------------------------|------------------|--------------------------------------|
| `text`              | `text_value`             | string           | Plain or formatted text              |
| `bullets`           | `bullets_value`          | string[]         | Bullet point list                    |
| `body_and_bullets`  | `body_and_bullets_value` | object           | Body text followed by bullets        |
| `bullet_groups`     | `bullet_groups_value`    | object           | Grouped bullets with section headers |
| `table`             | `table_value`            | object           | Data table                           |
| `chart`             | `chart_value`            | object           | SVG chart                            |
| `diagram`           | `diagram_value`          | object           | Business diagram                     |
| `image`             | `image_value`            | object           | Image file                           |

For backward compatibility, the generic `value` field (raw JSON) is also accepted for all types.

---

## 4. Inline Formatting

Text and bullet string values support inline formatting tags:

| Tag              | Effect      |
|------------------|-------------|
| `<b>text</b>`   | **Bold**    |
| `<i>text</i>`   | *Italic*    |
| `<u>text</u>`   | Underline   |

Tags can be nested: `<b><i>bold italic</i></b>`.

Example:
```json
{
  "type": "bullets",
  "bullets_value": [
    "Revenue up <b>25%</b> year-over-year",
    "Customer NPS: <i>all-time high</i>",
    "<b><u>Action required</u></b>: review Q2 targets"
  ]
}
```

---

## 5. Content Type Details

### Text

```json
{
  "placeholder_id": "title",
  "type": "text",
  "text_value": "Q1 2026 Business Review"
}
```

### Bullets

```json
{
  "placeholder_id": "body",
  "type": "bullets",
  "bullets_value": [
    "Revenue growth of <b>15%</b>",
    "New market expansion",
    "Team grew by 20%"
  ]
}
```

### Body and Bullets

Combines introductory body text with a bullet list:

```json
{
  "placeholder_id": "body",
  "type": "body_and_bullets",
  "body_and_bullets_value": {
    "body": "Our company achieved significant growth in 2025.",
    "bullets": [
      "Revenue up 25%",
      "Customer satisfaction at all-time high"
    ],
    "trailing_body": "We expect continued momentum in 2026."
  }
}
```

| Field           | Required | Description                              |
|-----------------|----------|------------------------------------------|
| `body`          | Yes      | Introductory text                        |
| `bullets`       | Yes      | Array of bullet strings                  |
| `trailing_body` | No       | Text after the bullets                   |

### Bullet Groups

Grouped bullets with section headers, for structured content like roadmaps:

```json
{
  "placeholder_id": "body",
  "type": "bullet_groups",
  "bullet_groups_value": {
    "body": "Product Roadmap",
    "groups": [
      {
        "header": "Phase 1 - Foundation (Q1)",
        "bullets": ["Core platform stabilization", "Performance optimization"]
      },
      {
        "header": "Phase 2 - Growth (Q2-Q3)",
        "bullets": ["New feature releases", "Partner integrations"]
      }
    ],
    "trailing_body": "Timeline subject to board approval."
  }
}
```

| Field           | Required | Description                              |
|-----------------|----------|------------------------------------------|
| `body`          | No       | Optional introductory text               |
| `groups`        | Yes      | Array of bullet group objects            |
| `groups[].header` | No     | Bold section header                      |
| `groups[].body` | No       | Group body text                          |
| `groups[].bullets` | Yes   | Array of bullet strings                  |
| `trailing_body` | No       | Text after all groups                    |

### Table

```json
{
  "placeholder_id": "body",
  "type": "table",
  "table_value": {
    "headers": ["Metric", "Q3 Actual", "Q4 Target"],
    "rows": [
      ["Revenue", "$4.2M", "$5.0M"],
      ["Margin", "62%", "65%"],
      ["Customers", "380", "450"]
    ],
    "style": {
      "header_background": "accent1",
      "borders": "horizontal",
      "striped": true
    },
    "column_alignments": ["left", "right", "right"]
  }
}
```

| Field               | Required | Description                                              |
|---------------------|----------|----------------------------------------------------------|
| `headers`           | Yes      | Array of header cell strings                             |
| `rows`              | Yes      | Array of row arrays (each cell is a string or object)    |
| `style`             | No       | Table styling options                                    |
| `column_alignments` | No       | Array of `"left"`, `"center"`, or `"right"`              |

**Cell format:** Each cell can be a plain string or an object with `content`, `col_span`, and `row_span`:

```json
{"content": "Category", "col_span": 2, "row_span": 1}
```

**Style options:**

| Field                | Values                                          | Default    |
|----------------------|-------------------------------------------------|------------|
| `header_background`  | `accent1`–`accent6`, `none`, or hex color       | `accent1`  |
| `borders`            | `all`, `horizontal`, `outer`, `none`            | `all`      |
| `striped`            | `true`, `false`                                 | `false`    |

### Chart

Charts are rendered as SVG images and embedded in the slide.

```json
{
  "placeholder_id": "body",
  "type": "chart",
  "chart_value": {
    "type": "bar",
    "title": "Quarterly Revenue ($M)",
    "data": [
      {"label": "Q1", "value": 12},
      {"label": "Q2", "value": 14},
      {"label": "Q3", "value": 15},
      {"label": "Q4", "value": 18}
    ],
    "width": 800,
    "height": 600,
    "style": {
      "colors": ["#FF6384", "#36A2EB", "#FFCE56"],
      "font_family": "Arial",
      "show_legend": true,
      "show_values": true,
      "show_grid": true,
      "background": "#FFFFFF"
    }
  }
}
```

#### Chart Types

| Type            | Description                     |
|-----------------|---------------------------------|
| `bar`           | Vertical bar chart              |
| `line`          | Line chart with markers         |
| `pie`           | Pie chart                       |
| `donut`         | Donut chart (pie with hole)     |
| `area`          | Filled area chart               |
| `radar`         | Radar/spider chart              |
| `scatter`       | Scatter plot                    |
| `bubble`        | Bubble chart (scatter + size)   |
| `stacked_bar`   | Stacked bar chart               |
| `grouped_bar`   | Grouped bar chart               |
| `stacked_area`  | Stacked area chart              |
| `waterfall`     | Waterfall chart (financial)     |
| `funnel`        | Funnel chart (conversion)       |
| `gauge`         | Gauge/meter chart               |
| `treemap`       | Treemap chart                   |

#### Chart Data Format

Array-of-objects with `label` and `value` fields:

```json
"data": [
  {"label": "Q1", "value": 100},
  {"label": "Q2", "value": 150}
]
```

#### Optional Chart Fields

| Field      | Default | Description                                |
|------------|---------|--------------------------------------------|
| `title`    | —       | Chart title                                |
| `width`    | 800     | Width in pixels                            |
| `height`   | 600     | Height in pixels                           |
| `fit_mode` | —       | `"stretch"`, `"contain"`, or `"cover"`     |
| `style`    | —       | Styling overrides (colors, fonts, etc.)    |

### Diagram

Diagrams are rendered as SVG images for business visualizations.

```json
{
  "placeholder_id": "body",
  "type": "diagram",
  "diagram_value": {
    "type": "swot",
    "title": "Company SWOT",
    "data": {
      "strengths": ["Strong brand", "Loyal customers"],
      "weaknesses": ["High costs"],
      "opportunities": ["Emerging markets"],
      "threats": ["Competition"]
    }
  }
}
```

#### Diagram Types

**Business Strategy:**

| Type                     | Description                          | Key Data Fields                              |
|--------------------------|--------------------------------------|----------------------------------------------|
| `timeline`               | Horizontal timeline                  | `items[].{date, title, description}`         |
| `process_flow`           | Linear process flow                  | `steps[].{title, description}`               |
| `matrix_2x2`            | 2x2 matrix (effort vs impact)       | `{x_label, y_label, quadrants[]}`            |
| `porters_five_forces`    | Porter's Five Forces                 | `{rivalry, new_entrants, substitutes, ...}`  |
| `business_model_canvas`  | Business Model Canvas                | `{key_partners, key_activities, ...}`        |
| `value_chain`            | Value chain analysis                 | `{primary[], support[]}`                     |
| `pestel`                 | PESTEL analysis                      | `{political[], economic[], social[], ...}`   |

**Organizational:**

| Type              | Description             | Key Data Fields                         |
|-------------------|-------------------------|-----------------------------------------|
| `nine_box_talent` | 9-box talent grid       | `{x_label, y_label, employees[]}`       |
| `org_chart`       | Organizational chart    | `root.{name, title, children[]}`        |

**General:**

| Type            | Description                  | Key Data Fields                                   |
|-----------------|------------------------------|---------------------------------------------------|
| `swot`          | SWOT analysis                | `{strengths[], weaknesses[], opportunities[], threats[]}` |
| `pyramid`       | Pyramid/hierarchy            | `levels[].{label, description}`                   |
| `venn`          | Venn diagram (2-3 circles)   | `{circles[], intersections{}}`                    |
| `house_diagram` | Strategy house               | `{roof, sections[], foundation}`                  |
| `gantt`         | Gantt chart                  | `tasks[].{name, start, end}`                      |
| `kpi_dashboard` | KPI metrics grid             | `metrics[].{label, value, delta, trend}`          |
| `heatmap`       | Heatmap visualization        | `{rows[], columns[], values[][]}`                 |
| `fishbone`      | Fishbone/Ishikawa diagram    | `{problem, categories[].{name, causes[]}}`        |
| `panel_layout`  | Panel layout (columns, rows, stat cards) | `{layout, panels[].{title, body, icon}}` |

#### Diagram Type Aliases

| Alias          | Maps To               |
|----------------|------------------------|
| `process`      | `process_flow`         |
| `flow`         | `process_flow`         |
| `flowchart`    | `process_flow`         |
| `org`          | `org_chart`            |
| `orgchart`     | `org_chart`            |
| `nine-box`     | `nine_box_talent`      |
| `bmc`          | `business_model_canvas`|
| `porter`       | `porters_five_forces`  |
| `porters`      | `porters_five_forces`  |
| `stat_cards`   | `panel_layout`         |
| `panel`        | `panel_layout`         |

### Image

```json
{
  "placeholder_id": "body",
  "type": "image",
  "image_value": {
    "path": "images/architecture.png",
    "alt": "System Architecture"
  }
}
```

| Field  | Required | Description              |
|--------|----------|--------------------------|
| `path` | Yes      | File path or URL         |
| `alt`  | No       | Alt text for the image   |

---

## 6. Shape Grid

The `shape_grid` field on a slide defines a grid of preset geometry shapes for custom layouts. When `shape_grid` is present and no `layout_id` is specified (or `slide_type` is `"blank"` / `"virtual"`), the system automatically selects a suitable blank layout and computes grid bounds from the template's title and footer positions (see [Virtual Layouts](../specs/24-virtual-layouts.md)).

### Grid Definition (ShapeGridInput)

```json
{
  "layout_id": "slideLayout2",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Process Overview"}
  ],
  "shape_grid": {
    "bounds": {"x": 5, "y": 18, "width": 90, "height": 72},
    "gap": 2,
    "columns": 3,
    "rows": [
      {
        "cells": [
          {
            "shape": {
              "geometry": "roundRect",
              "fill": "#4472C4",
              "text": {"content": "Step 1", "size": 14, "bold": true, "color": "#FFFFFF"}
            }
          },
          {
            "shape": {
              "geometry": "roundRect",
              "fill": "#5B9BD5",
              "text": {"content": "Step 2", "size": 14, "color": "#FFFFFF"}
            }
          },
          {
            "shape": {
              "geometry": "roundRect",
              "fill": "#70AD47",
              "text": {"content": "Step 3", "size": 14, "color": "#FFFFFF"}
            }
          }
        ]
      }
    ]
  }
}
```

| Field     | Type                | Required | Default      | Description                                                  |
|-----------|---------------------|----------|--------------|--------------------------------------------------------------|
| `bounds`  | object              | No       | Auto-derived | Bounding rectangle as percentages `{x, y, width, height}`   |
| `gap`     | number              | No       | 0            | Uniform gap between cells (percentage)                       |
| `col_gap` | number              | No       | `gap`        | Column-specific gap override (percentage of grid width)      |
| `row_gap` | number              | No       | `gap`        | Row-specific gap override (percentage of grid height)        |
| `columns` | number or number[]  | No       | Cell count   | Number of equal columns, or array of column width percentages |
| `rows`    | array               | Yes      | —            | Array of row definitions                                     |

**Bounds** (`{x, y, width, height}`): Slide-percentage coordinates (0–100). When omitted, bounds are auto-derived from the selected layout's title and footer placeholders.

### Grid Cells (GridCellInput)

Each cell holds either a **shape** or a **table** (mutually exclusive).

| Field      | Type    | Required | Default | Description                            |
|------------|---------|----------|---------|----------------------------------------|
| `col_span` | integer | No       | 1       | Number of columns this cell spans      |
| `row_span` | integer | No       | 1       | Number of rows this cell spans         |
| `shape`    | object  | No       | —       | Preset geometry shape (ShapeSpecInput) |
| `table`    | object  | No       | —       | Data table (same schema as content type `table`) |

### Shape Specification (ShapeSpecInput)

| Field         | Type              | Required | Default | Description                                             |
|---------------|-------------------|----------|---------|---------------------------------------------------------|
| `geometry`    | string            | Yes      | —       | Preset geometry: `"rect"`, `"roundRect"`, `"ellipse"`, `"diamond"`, `"chevron"`, etc. |
| `fill`        | string or object  | No       | —       | Fill color (`"#hex"`) or gradient object                |
| `line`        | string or object  | No       | —       | Line color (`"#hex"`) or line style object              |
| `text`        | string or object  | No       | —       | Text string shorthand, or full ShapeTextInput object    |
| `rotation`    | number            | No       | 0       | Rotation in degrees                                     |
| `adjustments` | object            | No       | —       | Preset geometry adjustment values                       |

### Shape Text (ShapeTextInput)

When `text` is a plain string, it is equivalent to `{"content": "..."}` with all defaults. The full object form:

```json
{
  "text": {
    "content": "Key Takeaway:\nStrong growth across all segments",
    "size": 14,
    "bold": true,
    "italic": false,
    "align": "ctr",
    "vertical_align": "ctr",
    "color": "#FFFFFF",
    "font": "Arial",
    "inset_left": 12,
    "inset_right": 12,
    "inset_top": 8,
    "inset_bottom": 8
  }
}
```

| Field            | Type    | Required | Default    | Description                                              |
|------------------|---------|----------|------------|----------------------------------------------------------|
| `content`        | string  | Yes      | —          | Text content (supports `<b>`, `<i>`, `<u>` inline tags) |
| `size`           | number  | No       | —          | Font size in points                                      |
| `bold`           | boolean | No       | `false`    | Bold text                                                |
| `italic`         | boolean | No       | `false`    | Italic text                                              |
| `align`          | string  | No       | `"ctr"`    | Horizontal: `"l"` (left), `"ctr"` (center), `"r"` (right) |
| `vertical_align` | string  | No       | `"ctr"`    | Vertical: `"t"` (top), `"ctr"` (middle), `"b"` (bottom) |
| `color`          | string  | No       | —          | Text color: `"#hex"` or scheme name (e.g., `"lt1"`)     |
| `font`           | string  | No       | `"+mn-lt"` | Font name, or `"+mn-lt"` (theme body) / `"+mj-lt"` (theme title) |
| `inset_left`     | number  | No       | 0          | Left text inset in points                                |
| `inset_right`    | number  | No       | 0          | Right text inset in points                               |
| `inset_top`      | number  | No       | 0          | Top text inset in points                                 |
| `inset_bottom`   | number  | No       | 0          | Bottom text inset in points                              |

### Typography Best Practices for Shape Grids

Use a consistent font size hierarchy across shape grid cells to create professional, readable slides. The recommended sizes follow consulting-style (Bain/McKinsey) conventions:

| Role                    | Size (pt) | Weight  | Color                | Use Case                                      |
|-------------------------|-----------|---------|----------------------|-----------------------------------------------|
| Grid header / banner    | 14–18     | Bold    | White on accent fill | Full-width header row spanning all columns     |
| Card title / headline   | 12–14     | Bold    | Dark text or white   | First line of a card (use `\n` to separate)    |
| Card body text          | 9–11      | Regular | Dark text            | Descriptive content within a card              |
| Numbering / step label  | 20–24     | Bold    | White on accent fill | Numbered step indicators in narrow columns     |
| Footnote / source       | 7–8       | Regular | Grey (`#666666`)     | Source citations or disclaimers                |

**Guidelines:**

- **3-4 columns:** Use 11pt body text with 6pt insets
- **5+ columns:** Reduce to 10pt body text with 4pt insets
- **Header rows:** Use `col_span` to span all columns, 18% height, accent fill
- **Card content:** Use `\n` to separate the bold title from body text within a single `content` string
- **Insets:** Always set `inset_top`, `inset_left`, and `inset_right` on body text cells (6–12pt) to prevent text touching shape edges
- **Alignment:** Headers use `"ctr"` / `"ctr"`, body cards use `"ctr"` / `"t"` or `"l"` / `"ctr"` depending on layout

**Example — Consistent typography across a 4-column grid:**

```json
{
  "shape_grid": {
    "gap": 6,
    "rows": [
      {
        "height": 18,
        "cells": [
          {
            "col_span": 4,
            "shape": {
              "geometry": "rect",
              "fill": "accent1",
              "text": { "content": "Strategic Pillars", "size": 18, "bold": true, "color": "#FFFFFF", "align": "ctr", "vertical_align": "ctr" }
            }
          }
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {
            "shape": {
              "geometry": "rect",
              "fill": "lt2",
              "text": { "content": "Innovation\n\nInvest in R&D and emerging technologies", "size": 11, "align": "ctr", "vertical_align": "t", "inset_top": 12, "inset_left": 6, "inset_right": 6 }
            }
          },
          {
            "shape": {
              "geometry": "rect",
              "fill": "lt2",
              "text": { "content": "Growth\n\nExpand into new markets and segments", "size": 11, "align": "ctr", "vertical_align": "t", "inset_top": 12, "inset_left": 6, "inset_right": 6 }
            }
          },
          {
            "shape": {
              "geometry": "rect",
              "fill": "lt2",
              "text": { "content": "Efficiency\n\nStreamline operations via automation", "size": 11, "align": "ctr", "vertical_align": "t", "inset_top": 12, "inset_left": 6, "inset_right": 6 }
            }
          },
          {
            "shape": {
              "geometry": "rect",
              "fill": "lt2",
              "text": { "content": "Talent\n\nAttract and retain top performers", "size": 11, "align": "ctr", "vertical_align": "t", "inset_top": 12, "inset_left": 6, "inset_right": 6 }
            }
          }
        ]
      }
    ]
  }
}
```

### Example: Two-Column Grid with Table and Shape

```json
{
  "slide_type": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Q1 Financial Summary"}
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
                "content": "<b>Key Takeaway</b>\nStrong growth across all segments",
                "size": 14,
                "color": "#FFFFFF",
                "vertical_align": "ctr",
                "inset_left": 12,
                "inset_right": 12
              }
            }
          }
        ]
      }
    ]
  }
}
```

---

## Complete Example

```json
{
  "template": "warm-coral",
  "output_filename": "Q1_Business_Review.pptx",
  "footer": {
    "enabled": true,
    "left_text": "Strategy Team | Confidential"
  },
  "slides": [
    {
      "slide_type": "title",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Q1 2026 Business Review"},
        {"placeholder_id": "subtitle", "type": "text", "text_value": "Strategy Team"}
      ]
    },
    {
      "slide_type": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Agenda"},
        {
          "placeholder_id": "body",
          "type": "bullets",
          "bullets_value": ["Financial performance", "Customer metrics", "Product roadmap", "Next steps"]
        }
      ],
      "speaker_notes": "Keep this to 30 seconds, just an overview."
    },
    {
      "slide_type": "chart",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Revenue by Quarter"},
        {
          "placeholder_id": "body",
          "type": "chart",
          "chart_value": {
            "type": "bar",
            "title": "2025-2026 Revenue ($M)",
            "data": [
              {"label": "Q1 '25", "value": 12},
              {"label": "Q2 '25", "value": 14},
              {"label": "Q3 '25", "value": 15},
              {"label": "Q4 '25", "value": 18},
              {"label": "Q1 '26", "value": 21}
            ]
          }
        }
      ],
      "source": "Internal CRM, Dec 2025"
    },
    {
      "slide_type": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Strategic Priorities"},
        {
          "placeholder_id": "body",
          "type": "bullet_groups",
          "bullet_groups_value": {
            "groups": [
              {
                "header": "Near-term (Q2)",
                "bullets": ["Launch enterprise tier", "Expand partner program"]
              },
              {
                "header": "Medium-term (Q3-Q4)",
                "bullets": ["International expansion", "Platform v2 architecture"]
              }
            ]
          }
        }
      ]
    },
    {
      "slide_type": "section",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Appendix"}
      ]
    },
    {
      "slide_type": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Detailed Financials"},
        {
          "placeholder_id": "body",
          "type": "table",
          "table_value": {
            "headers": ["Metric", "Q4 '25", "Q1 '26", "Change"],
            "rows": [
              ["Revenue", "$18M", "$21M", "+17%"],
              ["COGS", "$5.8M", "$6.7M", "+16%"],
              ["Gross Profit", "$12.2M", "$14.3M", "+17%"],
              ["OpEx", "$9.0M", "$10.1M", "+12%"],
              ["Net Income", "$3.2M", "$4.2M", "+31%"]
            ],
            "style": {"header_background": "accent1", "borders": "horizontal", "striped": true}
          }
        }
      ]
    }
  ]
}
```

---

## Patch Input Format

The system supports incremental modifications to a presentation via a patch input format. When the JSON input contains an `operations` array, it is automatically detected as a patch.

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

### Patch Operations

| Operation | Description | Requires `slide` |
|-----------|-------------|-------------------|
| `replace` | Replace the slide at `slide_index` with the provided slide | Yes |
| `add`     | Insert a new slide at `slide_index` (shifts subsequent slides right) | Yes |
| `remove`  | Remove the slide at `slide_index` (shifts subsequent slides left) | No |

Operations are applied in order. Indices are 0-based.
