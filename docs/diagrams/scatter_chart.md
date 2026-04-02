# Scatter Chart

Plot data points on two axes to show correlation and distribution patterns.

## Type Identifier

`scatter_chart`

**Aliases:** `scatter`

## Use Cases

- Correlation analysis between variables
- Customer segmentation visualization
- Risk vs. return plotting
- Performance distribution analysis

## Data Structure

```json
{
  "type": "scatter_chart",
  "title": "Price vs. Quality",
  "data": {
    "series": [
      {
        "name": "Products",
        "x_values": [20, 35, 50, 65, 80],
        "values": [60, 75, 85, 70, 95]
      }
    ],
    "x_label": "Price ($)",
    "y_label": "Quality Score"
  },
  "style": {
    "show_legend": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `series` | `object[]` | Data series |
| `series[].name` | `string` | Series label |
| `series[].values` | `number[]` | Y-axis values |
| `series[].x_values` | `number[]` | X-axis values |

Alternatively, use point objects:

| Field | Type | Description |
|-------|------|-------------|
| `series[].points` | `object[]` | Points with `x`, `y`, and optional `label` |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `x_label` | `string` | - | X-axis title (alias: `x_axis_title`) |
| `y_label` | `string` | - | Y-axis title (alias: `y_axis_title`) |
| `colors` | `string[]` | - | Custom hex color palette |

## Examples

### Using Points

```json
{
  "type": "scatter_chart",
  "title": "Risk vs. Return",
  "data": {
    "series": [
      {
        "name": "Investments",
        "points": [
          {"x": 10, "y": 5, "label": "Bonds"},
          {"x": 25, "y": 12, "label": "Real Estate"},
          {"x": 40, "y": 18, "label": "Stocks"},
          {"x": 60, "y": 25, "label": "Crypto"}
        ]
      }
    ],
    "x_label": "Risk (%)",
    "y_label": "Return (%)"
  }
}
```

### Multi-Series

```json
{
  "type": "scatter_chart",
  "title": "Employee Performance",
  "data": {
    "series": [
      {"name": "Engineering", "x_values": [3, 5, 4, 7], "values": [80, 90, 85, 95]},
      {"name": "Sales", "x_values": [2, 6, 4, 5], "values": [70, 88, 75, 82]}
    ],
    "x_label": "Years Experience",
    "y_label": "Performance Score"
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Bubble Chart](./bubble_chart.md) - For scatter with a size dimension
- [2x2 Matrix](./matrix_2x2.md) - For quadrant-based positioning
