# Bubble Chart

Plot data points on two axes with a third dimension represented by bubble size.

## Type Identifier

`bubble_chart`

**Aliases:** `bubble`

## Use Cases

- Market opportunity mapping (size = market value)
- Project portfolio analysis (size = budget)
- Customer segmentation (size = revenue)
- Competitive landscape visualization

## Data Structure

```json
{
  "type": "bubble_chart",
  "title": "Market Opportunity",
  "data": {
    "series": [
      {
        "name": "Markets",
        "x_values": [20, 50, 70, 85],
        "values": [60, 80, 45, 90],
        "bubble_values": [100, 250, 180, 400]
      }
    ],
    "x_label": "Growth Rate (%)",
    "y_label": "Margin (%)"
  },
  "style": {
    "show_legend": true
  }
}
```

## Required Fields

Using arrays:

| Field | Type | Description |
|-------|------|-------------|
| `series` | `object[]` | Data series |
| `series[].name` | `string` | Series label |
| `series[].values` | `number[]` | Y-axis values |
| `series[].x_values` | `number[]` | X-axis values |
| `series[].bubble_values` | `number[]` | Bubble sizes |

Alternatively, use point objects:

| Field | Type | Description |
|-------|------|-------------|
| `series[].points` | `object[]` | Points with `x`, `y`, `size`, and optional `label` |

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
  "type": "bubble_chart",
  "title": "Product Portfolio",
  "data": {
    "series": [
      {
        "name": "Products",
        "points": [
          {"x": 30, "y": 70, "size": 200, "label": "Enterprise"},
          {"x": 60, "y": 50, "size": 120, "label": "SMB"},
          {"x": 80, "y": 85, "size": 350, "label": "Consumer"}
        ]
      }
    ],
    "x_label": "Market Share (%)",
    "y_label": "Growth Rate (%)"
  }
}
```

### Multi-Series

```json
{
  "type": "bubble_chart",
  "title": "Regional Performance",
  "data": {
    "series": [
      {"name": "EMEA", "x_values": [40, 55], "values": [70, 80], "bubble_values": [300, 150]},
      {"name": "APAC", "x_values": [60, 75], "values": [65, 90], "bubble_values": [250, 200]}
    ],
    "x_label": "Revenue ($M)",
    "y_label": "Satisfaction (%)"
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Scatter Chart](./scatter_chart.md) - For two-dimensional plotting without size
- [2x2 Matrix](./matrix_2x2.md) - For quadrant-based positioning
