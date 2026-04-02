# Radar Chart

Compare multiple variables on a radial grid, useful for profiling and benchmarking.

## Type Identifier

`radar_chart`

**Aliases:** `radar`

## Use Cases

- Skill assessments and competency profiles
- Product feature comparisons
- Performance benchmarking across dimensions
- Team capability mapping

## Data Structure

```json
{
  "type": "radar_chart",
  "title": "Team Skills Assessment",
  "data": {
    "categories": ["Communication", "Technical", "Leadership", "Creativity", "Teamwork"],
    "series": [
      {"name": "Team A", "values": [85, 90, 70, 65, 80]},
      {"name": "Team B", "values": [75, 60, 85, 80, 70]}
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
| `categories` | `string[]` | Axis labels (min 3). Aliases: `labels`, `axes` |
| `series` | `object[]` | Data series with name and values |
| `series[].name` | `string` | Series label for legend |
| `series[].values` | `number[]` | Values per axis (min 3, must match categories length) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `colors` | `string[]` | - | Custom hex color palette |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | `false` | Show value labels |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Single Profile

```json
{
  "type": "radar_chart",
  "title": "Product Evaluation",
  "data": {
    "categories": ["Price", "Quality", "Design", "Features", "Support"],
    "series": [{"name": "Product X", "values": [80, 95, 70, 85, 90]}]
  }
}
```

### Competitive Comparison

```json
{
  "type": "radar_chart",
  "title": "Vendor Comparison",
  "data": {
    "categories": ["Price", "Quality", "Delivery", "Support", "Innovation", "Scale"],
    "series": [
      {"name": "Vendor A", "values": [90, 80, 70, 85, 60, 75]},
      {"name": "Vendor B", "values": [70, 90, 85, 75, 80, 65]}
    ]
  },
  "style": {"show_legend": true}
}
```

## See Also

- [Bar Chart](./bar_chart.md) - For simple category comparisons
- [2x2 Matrix](./matrix_2x2.md) - For two-dimensional positioning
