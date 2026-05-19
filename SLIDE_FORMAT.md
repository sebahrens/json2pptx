# JSON Slide Format Reference

This document describes the JSON input format for `json2pptx`. JSON input provides precise control over layout selection and placeholder targeting.

## Top-Level Structure

```json
{
  "template": "midnight-blue",
  "output_filename": "deck.pptx",
  "footer": {
    "enabled": true,
    "left_text": "Acme Corp | Confidential"
  },
  "theme_override": {
    "colors": {"accent1": "#E31837"},
    "title_font": "Helvetica",
    "body_font": "Helvetica"
  },
  "slides": []
}
```

| Field | Required | Description |
|---|---|---|
| `template` | Yes | Template name (without `.pptx` extension) |
| `output_filename` | No | Output filename (default: `output.pptx`) |
| `footer` | No | Footer injection settings |
| `theme_override` | No | Per-deck color and font overrides |
| `slides` | Yes | Array of slide objects (at least one) |

### Theme Override

| Field | Description |
|---|---|
| `colors` | Map of theme color keys to hex values (`accent1`-`accent6`, `dk1`, `dk2`, `lt1`, `lt2`, `hlink`, `folHlink`) |
| `title_font` | Override title font family |
| `body_font` | Override body font family |

## Slide Object

```json
{
  "layout_id": "content",
  "slide_type": "content",
  "content": [],
  "speaker_notes": "Talking points for this slide.",
  "source": "Source: Annual Report 2025",
  "transition": "fade",
  "transition_speed": "medium",
  "build": "bullets"
}
```

| Field | Required | Description |
|---|---|---|
| `layout_id` | No | Canonical layout ID (e.g., `"title"`, `"content"`, `"two-column"`, `"section"`, `"closing"`, `"blank"`). Display names like `"Title Slide"` are **not valid**. |
| `slide_type` | No | Hint for layout auto-selection: `title`, `content`, `section`, `chart`, `two-column`, `diagram`, `image`, `comparison`, `blank` |
| `content` | Yes | Array of content items targeting placeholders |
| `shape_grid` | No | Shape grid definition for custom geometry layouts (see Shape Grid section in README) |
| `background` | No | Slide background: `{"image": "path/to/bg.png"}` or `{"url": "https://..."}` with optional `"fit": "cover"\|"stretch"\|"tile"` |
| `speaker_notes` | No | Speaker notes text |
| `source` | No | Source attribution text |
| `transition` | No | Slide transition: `fade`, `push`, `wipe`, `cover`, `uncover`, `cut`, `dissolve`, `none` |
| `transition_speed` | No | Transition speed: `slow`, `medium`, `fast` (legacy alias `med` is also accepted) |
| `build` | No | Build animation: `bullets` for one-by-one bullet reveal |

Provide `layout_id` for explicit layout selection, or `slide_type` for automatic selection based on template tags. If both are omitted, the generator infers from content.

## Content Items

Each content item targets a placeholder by its **normalized canonical ID**.

```json
{
  "placeholder_id": "body",
  "type": "bullets",
  "bullets_value": ["Revenue +15%", "DAU +22%", "Churn -3%"]
}
```

| Field | Required | Description |
|---|---|---|
| `placeholder_id` | Yes | Target placeholder: `title`, `subtitle`, `body`, `body_2`, `body_3`, `image`, `image_2` |
| `type` | Yes | Content type (see below) |
| Typed value field | Yes | One of the typed value fields matching the `type` discriminator |
| `font_size` | No | Override font size in points (e.g., `72`). Only applies to text-based content types. |

### Placeholder IDs

Use normalized canonical IDs in `placeholder_id`:

| ID | Targets | Used for |
|---|---|---|
| `title` | Title placeholder | Slide title text |
| `subtitle` | Subtitle placeholder | Subtitle on title slides |
| `body` | Primary body (leftmost) | Bullets, text, charts, diagrams, tables |
| `body_2` | Second body placeholder | Right column in two-column layouts |
| `body_3` | Third body placeholder | Additional content areas |
| `image` | Primary image placeholder | Image content |
| `image_2` | Second image placeholder | Additional images |

Legacy OOXML names (`Title 1`, `Content Placeholder 2`, `Text Placeholder 3`) are accepted as aliases but canonical IDs are preferred for cross-template stability.

## Content Types

### `text`

Plain text content.

```json
{"placeholder_id": "title", "type": "text", "text_value": "Quarterly Review"}
```

Legacy form using `value`:
```json
{"placeholder_id": "title", "type": "text", "value": "Quarterly Review"}
```

### `bullets`

Bulleted list.

```json
{"placeholder_id": "body", "type": "bullets", "bullets_value": ["Point one", "Point two", "Point three"]}
```

### `body_and_bullets`

Paragraph text followed by bullets, with optional trailing text.

```json
{
  "placeholder_id": "body",
  "type": "body_and_bullets",
  "body_and_bullets_value": {
    "body": "Our key achievements this quarter:",
    "bullets": ["Revenue up 25%", "Customer satisfaction at all-time high"],
    "trailing_body": "We expect continued growth in Q2."
  }
}
```

### `bullet_groups`

Grouped bullets with section headers.

```json
{
  "placeholder_id": "body",
  "type": "bullet_groups",
  "bullet_groups_value": {
    "body": "Strategic priorities by phase:",
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
    "trailing_body": "All phases subject to quarterly review."
  }
}
```

### `table`

Tabular data.

```json
{
  "placeholder_id": "body",
  "type": "table",
  "table_value": {
    "headers": ["Metric", "Q4 Actual", "Q1 Target"],
    "rows": [
      ["Revenue", "$4.2M", "$5.0M"],
      ["Margin", "62%", "65%"]
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

Table cells support both string shorthand and object form for merged cells:
```json
{"content": "Merged cell", "col_span": 2, "row_span": 1}
```

### `chart`

SVG chart rendered into the placeholder.

```json
{
  "placeholder_id": "body",
  "type": "chart",
  "chart_value": {
    "type": "bar_chart",
    "title": "Revenue by Quarter",
    "data": [
      {"label": "Q1", "value": 100},
      {"label": "Q2", "value": 150}
    ]
  }
}
```

Supported chart types: `bar_chart`, `line_chart`, `pie_chart`, `donut_chart`, `area_chart`, `radar_chart`, `scatter_chart`, `bubble_chart`, `stacked_bar_chart`, `stacked_area_chart`, `grouped_bar_chart`, `waterfall`, `funnel_chart`, `gauge_chart`, `treemap_chart`.

### `diagram`

SVG diagram rendered into the placeholder.

```json
{
  "placeholder_id": "body",
  "type": "diagram",
  "diagram_value": {
    "type": "timeline",
    "title": "Roadmap",
    "data": {
      "items": [
        {"date": "2026 Q1", "title": "Phase 1", "description": "Discovery"},
        {"date": "2026 Q2", "title": "Phase 2", "description": "Development"}
      ]
    }
  }
}
```

Supported diagram types: `timeline`, `process_flow`, `pyramid`, `venn`, `swot`, `org_chart`, `gantt`, `matrix_2x2`, `porters_five_forces`, `house_diagram`, `business_model_canvas`, `value_chain`, `nine_box_talent`, `kpi_dashboard`, `heatmap`, `fishbone`, `pestel`, `panel_layout`.

### `image`

Image file or URL.

```json
{"placeholder_id": "image", "type": "image", "image_value": {"path": "images/photo.png", "alt": "Team photo"}}
```

## Icons (IconInput)

Icons may appear inside `shape_grid` cells, inside shapes, and inside pattern fields that accept icons (e.g. `card-grid` cells, `icon-row` items, `herodetail` icon). All icon fields use the same `IconInput` shape and the same source rules.

### No-emoji policy (hard rule)

**Never emit emoji codepoints in any icon field.** Glyphs in the Unicode emoji range (`🚀`, `📈`, `✅`, `⚡`, `🎯`, `🔥`, etc.) are rejected by pattern validators (`card-grid`, `icon-row`, `herodetail`) and produce broken or off-brand output if they slip through. This rule also applies to text fields: do not use emoji in titles, bullets, headers, captions, table cells, or speaker notes. Plain Unicode symbols outside the emoji range (e.g. arrows `→` / `←`, the bullet `•`, en/em dashes) are still allowed inside text.

To represent a concept visually, use one of the four accepted sources below.

### Accepted icon sources (exactly one per icon)

| Source | Field | Example |
|---|---|---|
| Bundled SVG icon | `name` | `{"icon": {"name": "chart-pie", "fill": "#FFFFFF"}}` |
| Local file (SVG / image) | `path` | `{"icon": {"path": "icons/custom.svg", "fill": "#FF6600"}}` |
| Remote SVG / image | `url` | `{"icon": {"url": "https://example.com/logo.svg"}}` |
| Inline SVG markup | `svg_data` | `{"icon": {"svg_data": "<svg ...>...</svg>", "alt": "Pie chart"}}` |

Run `json2pptx icons list` (CLI) or call `list_icons` (MCP) for the bundled icon catalog (e.g. `rocket`, `chart-pie`, `users`, `alert-circle`, `filled:circle-check`, etc.).

| Field | Description |
|---|---|
| `name` | Bundled icon name (preferred). Resolves through the bundled SVG registry; never an emoji glyph. |
| `path` | File path to a custom SVG or raster image, relative to the JSON input directory. |
| `url` | HTTP/HTTPS URL to fetch an SVG or image. |
| `svg_data` | Inline SVG markup (e.g. the output of `svggen-mcp.render_diagram`). No filesystem or network access. |
| `alt` | Accessibility description. Falls back to `name`/`path`/`"icon"` when omitted. |
| `fill` | Optional fill color override. Use semantic theme colors (`accent1`–`accent6`, `dk1`, `lt1`) when possible. Ignored for `svg_data`. |
| `position` | Icon position relative to text in a shape: `"left"`, `"top"`, `"center"`. Auto-detected when empty. |

Exactly one of `name`, `path`, `url`, or `svg_data` must be set per icon. Setting an emoji codepoint in `name` (or any pattern field that accepts an icon) is rejected.

## Complete Example

```json
{
  "template": "midnight-blue",
  "output_filename": "q1-review.pptx",
  "slides": [
    {
      "layout_id": "title",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Q1 2026 Business Review"},
        {"placeholder_id": "subtitle", "type": "text", "text_value": "Strategy Team"}
      ]
    },
    {
      "layout_id": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Agenda"},
        {"placeholder_id": "body", "type": "bullets", "bullets_value": [
          "Financial performance",
          "Customer metrics",
          "Product roadmap",
          "Next steps"
        ]}
      ],
      "speaker_notes": "Keep this to 30 seconds, just an overview."
    },
    {
      "layout_id": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Revenue by Quarter"},
        {
          "placeholder_id": "body",
          "type": "chart",
          "chart_value": {
            "type": "bar_chart",
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
      ]
    },
    {
      "layout_id": "two-column",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Key Metrics"},
        {
          "placeholder_id": "body",
          "type": "bullet_groups",
          "bullet_groups_value": {
            "groups": [
              {
                "header": "Financial",
                "bullets": ["Revenue: $21M (+75% YoY)", "Gross Margin: 68%", "Net Income: $3.2M"]
              }
            ]
          }
        },
        {
          "placeholder_id": "body_2",
          "type": "bullet_groups",
          "bullet_groups_value": {
            "groups": [
              {
                "header": "Operational",
                "bullets": ["Headcount: 145", "Customer NPS: 72", "Uptime: 99.95%"]
              }
            ]
          }
        }
      ]
    },
    {
      "layout_id": "section",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Appendix"}
      ]
    }
  ]
}
```

## JSON Output

When run with `--json-output`, the generator returns:

```json
{
  "success": true,
  "output_path": "/path/to/q1-review.pptx",
  "slide_count": 5,
  "duration_ms": 850,
  "warnings": [],
  "quality": {
    "score": 0.92,
    "slide_scores": [
      {"slide_number": 1, "score": 1.0},
      {"slide_number": 2, "score": 0.95}
    ],
    "issues": []
  }
}
```
