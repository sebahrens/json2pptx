# Area Chart

Display trends over time with filled areas beneath the line.

An area joins neighbouring values with a slope, so it is drawn only over a sequence: periods (`Q1`, `Jan`, `2024`), numbers, or an ordinal scale. Categories that are not a sequence (regions, products) are drawn as a bar chart instead; `as_area: true` keeps the area.

## Type Identifier

`area_chart`

**Aliases:** `area`

## Use Cases

- Revenue trends over time
- Market share evolution
- Traffic volume patterns
- Cumulative metrics visualization

## Data Structure

```json
{
  "type": "area_chart",
  "title": "Monthly Revenue",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun"],
    "series": [
      {"name": "Revenue", "values": [120, 135, 148, 162, 155, 170]}
    ]
  },
  "style": {
    "show_legend": true,
    "show_values": true,
    "show_grid": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `categories` | `string[]` | X-axis labels |
| `series` | `object[]` | Data series with name and values |
| `series[].name` | `string` | Series label for legend |
| `series[].values` | `number[]` | Values (must match categories length) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `x_label` | `string` | - | X-axis title (alias: `x_axis_title`) |
| `y_label` | `string` | - | Y-axis title (alias: `y_axis_title`) |
| `colors` | `string[]` | - | Hex colors or template scheme names (e.g. `accent1`) |
| `as_area` | `bool` | `false` | Keep the area for categories that are not a sequence (drawn as bars by default) |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | labelled for one series up to 12 points | Show value labels; omitted, a single-series area with at most 12 points is labelled, `false` opts out |
| `show_grid` | `bool` | `false` | Display background grid |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Single Series

```json
{
  "type": "area_chart",
  "title": "Website Traffic",
  "data": {
    "categories": ["Mon", "Tue", "Wed", "Thu", "Fri"],
    "series": [{"name": "Visitors", "values": [1200, 1500, 1350, 1800, 1650]}]
  }
}
```

### Multi-Series

```json
{
  "type": "area_chart",
  "title": "Revenue by Channel",
  "data": {
    "categories": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "Online", "values": [200, 250, 280, 320]},
      {"name": "Retail", "values": [150, 160, 170, 180]}
    ]
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Line Chart](./line_chart.md) - For trends without fill
- [Stacked Area Chart](./stacked_area_chart.md) - For cumulative area display
