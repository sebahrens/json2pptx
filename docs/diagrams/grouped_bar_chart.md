# Grouped Bar Chart

Compare multiple series side-by-side within each category using grouped bars.

## Type Identifier

`grouped_bar_chart`

**Aliases:** `grouped_bar`

## Use Cases

- Year-over-year performance comparison
- Multi-product sales by region
- Budget vs. actual by department
- Competitive benchmarking across metrics

## Data Structure

```json
{
  "type": "grouped_bar_chart",
  "title": "Quarterly Performance",
  "data": {
    "categories": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "2023", "values": [100, 120, 115, 140]},
      {"name": "2024", "values": [110, 135, 125, 155]}
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
| `series` | `object[]` | Data series (minimum 2 for grouping) |
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
| `show_values` | `bool` | `false` | Show value labels on bars |
| `show_grid` | `bool` | `false` | Display background grid |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Budget vs. Actual

```json
{
  "type": "grouped_bar_chart",
  "title": "Budget vs. Actual Spending",
  "data": {
    "categories": ["Engineering", "Marketing", "Sales", "Operations"],
    "series": [
      {"name": "Budget", "values": [500, 200, 300, 150]},
      {"name": "Actual", "values": [480, 220, 290, 160]}
    ]
  },
  "style": {"show_legend": true, "show_values": true}
}
```

## See Also

- [Bar Chart](./bar_chart.md) - For single-series bar charts
- [Stacked Bar Chart](./stacked_bar_chart.md) - For stacked (non-grouped) multi-series
