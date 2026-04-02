# Stacked Bar Chart

Compare categories with bars split into stacked segments showing composition.

## Type Identifier

`stacked_bar_chart`

**Aliases:** `stacked_bar`, `bar-stacked`

## Use Cases

- Revenue breakdown by product line per quarter
- Budget allocation across departments
- Market share by segment over time
- Resource utilization by category

## Data Structure

```json
{
  "type": "stacked_bar_chart",
  "title": "Revenue by Product Line",
  "data": {
    "categories": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "Product A", "values": [100, 120, 115, 130]},
      {"name": "Product B", "values": [80, 90, 95, 105]},
      {"name": "Product C", "values": [50, 55, 60, 70]}
    ]
  },
  "style": {
    "show_legend": true,
    "show_values": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `categories` | `string[]` | X-axis labels |
| `series` | `object[]` | Data series (stacked segments) |
| `series[].name` | `string` | Segment label for legend |
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
| `show_values` | `bool` | `false` | Show value labels on segments |
| `show_grid` | `bool` | `false` | Display background grid |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Budget Breakdown

```json
{
  "type": "stacked_bar_chart",
  "title": "Annual Budget Allocation",
  "data": {
    "categories": ["2022", "2023", "2024"],
    "series": [
      {"name": "Engineering", "values": [500, 600, 700]},
      {"name": "Marketing", "values": [200, 250, 300]},
      {"name": "Operations", "values": [150, 160, 180]}
    ]
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Bar Chart](./bar_chart.md) - For side-by-side comparisons
- [Grouped Bar Chart](./grouped_bar_chart.md) - For grouped (non-stacked) multi-series
- [Stacked Area Chart](./stacked_area_chart.md) - For stacked trends over time
