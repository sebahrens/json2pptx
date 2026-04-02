# Stacked Area Chart

Show cumulative trends over time with filled areas stacked on top of each other.

## Type Identifier

`stacked_area_chart`

**Aliases:** `stacked_area`

## Use Cases

- Revenue composition over time
- Resource utilization trends
- Traffic sources breakdown
- Market share evolution

## Data Structure

```json
{
  "type": "stacked_area_chart",
  "title": "Revenue by Channel",
  "data": {
    "categories": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "Direct", "values": [100, 120, 130, 150]},
      {"name": "Partner", "values": [80, 90, 100, 110]},
      {"name": "Online", "values": [50, 70, 90, 120]}
    ]
  },
  "style": {
    "show_legend": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `categories` | `string[]` | X-axis labels |
| `series` | `object[]` | Data series (stacked from bottom to top) |
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

### Traffic Sources

```json
{
  "type": "stacked_area_chart",
  "title": "Website Traffic Sources",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun"],
    "series": [
      {"name": "Organic", "values": [5000, 5500, 6000, 6200, 6800, 7200]},
      {"name": "Paid", "values": [2000, 2200, 2500, 2800, 3000, 3200]},
      {"name": "Social", "values": [1000, 1200, 1100, 1500, 1800, 2000]}
    ]
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Area Chart](./area_chart.md) - For non-stacked area display
- [Stacked Bar Chart](./stacked_bar_chart.md) - For stacked category comparisons
- [Line Chart](./line_chart.md) - For trends without fill
