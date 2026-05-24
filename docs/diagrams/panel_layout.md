# Panel Layout

Display content in structured card panels with optional icons, values, and callouts.

## Type Identifier

`panel_layout`

**Aliases:** `icon_columns` (→ `layout: "columns"`), `icon_rows` (→ `layout: "rows"`), `stat_cards` (→ `layout: "stat_cards"`). Each alias is a registered, native-OOXML diagram type that expands to `panel_layout` with the matching layout; an explicit `data.layout` still wins if both are set.

## Use Cases

- Feature highlights with icons
- Stat card grids
- Service descriptions
- Key takeaway callouts

## Data Structure

```json
{
  "type": "panel_layout",
  "title": "Our Services",
  "data": {
    "panels": [
      {"title": "Analytics", "icon": "chart", "body": "Real-time dashboards and reporting"},
      {"title": "Security", "icon": "shield", "body": "Enterprise-grade protection"},
      {"title": "Support", "icon": "headset", "body": "24/7 dedicated support team"},
      {"title": "Scale", "icon": "cloud", "body": "Auto-scaling infrastructure"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `panels` | `object[]` | Panel cards |
| `panels[].title` | `string` | Panel header |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Layout title |
| `subtitle` | `string` | - | Subtitle below title |
| `panels[].icon` | `string` \| `object` | - | Panel icon, embedded as **native SVG** (`asvg:svgBlip`, never rasterized to PNG). Accepts a bundled name (`"rocket"`), inline `<svg>…</svg>`, a `data:image/svg+xml` URI, a `.svg` file path, or an object `{name\|path\|url\|svg_data, fill?, alt?}`. `fill` recolors `currentColor` for bundled **and** external sources. Rendered across all four layout modes (columns: above the title; rows: left column; stat_cards: above the value; stylish_panels: centered on the accent band, white by default). |
| `panels[].body` | `string` | - | Content text |
| `panels[].value` | `string` | - | Numeric display (for stat cards) |
| `layout` | `string` | - | `"columns"`, `"rows"`, `"stat_cards"`, or `"stylish_panels"` (auto-inferred from alias) |
| `gap` | `number` | - | Spacing between panels |
| `corner_radius` | `number` | - | Panel corner radius |
| `icon_size` | `number` | - | Icon size |
| `separator_width` | `number` | - | Separator line width |
| `callout` | `object` | - | Banner with `icon` and `text` |
| `footnote` | `string` | - | Footnote below panels |

## Examples

### Stat Cards

```json
{
  "type": "stat_cards",
  "title": "Key Metrics",
  "data": {
    "panels": [
      {"title": "Users", "value": "12,500"},
      {"title": "Revenue", "value": "$2.1M"},
      {"title": "Growth", "value": "+28%"},
      {"title": "NPS", "value": "74"}
    ]
  }
}
```

### Icon Columns

```json
{
  "type": "icon_columns",
  "title": "How It Works",
  "data": {
    "panels": [
      {"title": "Upload", "icon": "upload", "body": "Drop your files into the portal"},
      {"title": "Process", "icon": "gear", "body": "AI analyzes and categorizes"},
      {"title": "Review", "icon": "check", "body": "Approve the generated output"}
    ]
  }
}
```

### Stylish Panels

```json
{
  "type": "panel_layout",
  "title": "Strategic Priorities",
  "data": {
    "layout": "stylish_panels",
    "panels": [
      {"title": "Growth", "body": "- Market expansion\n- New verticals"},
      {"title": "Efficiency", "body": "- Process automation\n- Cost reduction"},
      {"title": "Innovation", "body": "- R&D investment\n- Patent portfolio"}
    ]
  }
}
```

### With Callout Banner

```json
{
  "type": "panel_layout",
  "title": "Platform Benefits",
  "data": {
    "panels": [
      {"title": "Speed", "body": "10x faster processing"},
      {"title": "Cost", "body": "60% reduction in spend"},
      {"title": "Quality", "body": "99.9% accuracy"}
    ],
    "callout": {"icon": "info", "text": "Results based on Q1 2024 benchmarks"}
  }
}
```

## See Also

- [KPI Dashboard](./kpi_dashboard.md) - For metric cards with trends
- [Process Flow](./process_flow.md) - For sequential workflows
