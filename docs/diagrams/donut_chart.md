# Donut Chart

Pie chart variant with a center hole, useful for displaying a central metric.

## Type Identifier

`donut_chart`

## Use Cases

- Progress indicators with total in center
- Category breakdowns with key metric
- Budget allocation with total spend
- Portfolio distribution

## Data Structure

```json
{
  "type": "donut_chart",
  "title": "Portfolio Allocation",
  "data": {
    "series": [
      {"name": "Stocks", "value": 60},
      {"name": "Bonds", "value": 25},
      {"name": "Cash", "value": 10},
      {"name": "Real Estate", "value": 5}
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
| `series` | `object[]` | Segment data |
| `series[].name` | `string` | Segment label |
| `series[].value` | `number` | Segment value |

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

### Basic Donut

```json
{
  "type": "donut_chart",
  "title": "Time Allocation",
  "data": {
    "series": [
      {"name": "Development", "value": 50},
      {"name": "Meetings", "value": 20},
      {"name": "Planning", "value": 15},
      {"name": "Admin", "value": 15}
    ]
  }
}
```

### With Legend and Values

```json
{
  "type": "donut_chart",
  "title": "Customer Segments",
  "data": {
    "series": [
      {"name": "Enterprise", "value": 35},
      {"name": "SMB", "value": 40},
      {"name": "Consumer", "value": 25}
    ]
  },
  "style": {
    "show_legend": true,
    "show_values": true,
    "palette": ["#4F46E5", "#10B981", "#F59E0B"]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Pie Chart](./pie_chart.md) - Full circle version
- [Bar Chart](./bar_chart.md) - For exact comparisons
