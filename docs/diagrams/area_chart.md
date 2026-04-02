# Area Chart

Display trends over time with filled areas beneath the line.

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
| `colors` | `string[]` | - | Custom hex color palette |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | `false` | Show value labels |
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
