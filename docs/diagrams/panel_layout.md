# Panel Layout

Display content in structured card panels with optional icons, values, and callouts.

## Type Identifier

`panel_layout`

**Aliases:** `icon_columns`, `icon_rows`, `stat_cards`, `panel`, `icon_panel`, `number_tiles`, `callout_cards`

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
| `panels[].icon` | `string` | - | Icon name |
| `panels[].body` | `string` | - | Content text |
| `panels[].value` | `string` | - | Numeric display (for stat cards) |
| `layout` | `string` | - | `"columns"`, `"rows"`, or `"stat_cards"` (auto-inferred from alias) |
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
