# Line Chart

Show trends over time or continuous data with connected points.

## Type Identifier

`line_chart`

## Use Cases

- Revenue trends over months/quarters
- Stock price movements
- Temperature or sensor data
- Growth metrics over time

## Data Structure

```json
{
  "type": "line_chart",
  "title": "Revenue Trend",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun"],
    "series": [
      {"name": "2024", "values": [100, 120, 115, 140, 155, 180]},
      {"name": "2023", "values": [90, 105, 100, 125, 140, 160]}
    ]
  },
  "style": {
    "show_legend": true,
    "show_grid": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `categories` | `string[]` | X-axis labels (time periods, etc.) |
| `series` | `object[]` | Data series to plot |
| `series[].name` | `string` | Series name for legend |
| `series[].values` | `number[]` | Y-values for each category |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | `false` | Show value labels at points |
| `show_grid` | `bool` | `false` | Display background grid |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Single Trend Line

```json
{
  "type": "line_chart",
  "title": "Monthly Active Users",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun"],
    "series": [{"name": "MAU", "values": [10000, 12500, 15000, 18000, 22000, 28000]}]
  }
}
```

### Multi-Series Comparison

```json
{
  "type": "line_chart",
  "title": "Website Traffic Sources",
  "data": {
    "categories": ["Week 1", "Week 2", "Week 3", "Week 4"],
    "series": [
      {"name": "Organic", "values": [5000, 5500, 6200, 7100]},
      {"name": "Paid", "values": [3000, 4500, 4200, 5000]},
      {"name": "Social", "values": [1500, 2000, 2800, 3500]}
    ]
  },
  "style": {"show_legend": true, "show_grid": true}
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Bar Chart](./bar_chart.md) - For category comparisons
- [Timeline](./timeline.md) - For project schedules
