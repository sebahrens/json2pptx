# Pie Chart

Display proportions of a whole as circular segments.

## Type Identifier

`pie_chart`

## Use Cases

- Market share distribution
- Budget allocation
- Survey response breakdown
- Portfolio composition

## Data Structure

```json
{
  "type": "pie_chart",
  "title": "Market Share",
  "data": {
    "series": [
      {"name": "Company A", "value": 45},
      {"name": "Company B", "value": 30},
      {"name": "Company C", "value": 15},
      {"name": "Others", "value": 10}
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
| `series` | `object[]` | Slice data |
| `series[].name` | `string` | Slice label |
| `series[].value` | `number` | Slice value (absolute or percentage) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | `false` | Show percentage labels |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Basic Pie Chart

```json
{
  "type": "pie_chart",
  "title": "Revenue by Region",
  "data": {
    "series": [
      {"name": "North America", "value": 40},
      {"name": "Europe", "value": 30},
      {"name": "Asia Pacific", "value": 20},
      {"name": "Other", "value": 10}
    ]
  }
}
```

### With Values Displayed

```json
{
  "type": "pie_chart",
  "title": "Budget Allocation",
  "data": {
    "series": [
      {"name": "Engineering", "value": 45},
      {"name": "Marketing", "value": 25},
      {"name": "Sales", "value": 20},
      {"name": "Operations", "value": 10}
    ]
  },
  "style": {
    "show_legend": true,
    "show_values": true,
    "palette": "vibrant"
  }
}
```

## Best Practices

- Limit to 5-7 slices for readability
- Group small values into "Other"
- Use consistent colors across related charts
- Consider donut chart for center text

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Donut Chart](./donut_chart.md) - Pie with center space
- [Bar Chart](./bar_chart.md) - For precise comparisons
