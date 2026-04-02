# Bar Chart

Compare values across categories with vertical bars.

## Type Identifier

`bar_chart`

## Use Cases

- Comparing sales across regions
- Budget breakdowns by department
- Survey responses by category
- Performance metrics comparison

## Data Structure

```json
{
  "type": "bar_chart",
  "title": "Sales by Region",
  "subtitle": "Q1 2024",
  "data": {
    "categories": ["North", "South", "East", "West"],
    "series": [
      {"name": "Revenue", "values": [100, 80, 120, 90]},
      {"name": "Target", "values": [95, 85, 110, 100]}
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
| `categories` | `string[]` | Category labels for x-axis |
| `series` | `object[]` | Data series with name and values |
| `series[].name` | `string` | Series label for legend |
| `series[].values` | `number[]` | Values (must match categories length) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_legend` | `bool` | `false` | Display legend |
| `show_values` | `bool` | `false` | Show value labels on bars |
| `show_grid` | `bool` | `false` | Display background grid |
| `palette` | `string\|string[]` | `corporate` | Color scheme |

## Examples

### Single Series

```json
{
  "type": "bar_chart",
  "title": "Monthly Sales",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun"],
    "series": [{"name": "Sales", "values": [42, 38, 55, 48, 61, 53]}]
  }
}
```

### Multi-Series Comparison

```json
{
  "type": "bar_chart",
  "title": "Year-over-Year Comparison",
  "data": {
    "categories": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "2023", "values": [100, 120, 115, 140]},
      {"name": "2024", "values": [110, 135, 125, 155]}
    ]
  },
  "style": {"show_legend": true}
}
```

### Custom Colors

```json
{
  "type": "bar_chart",
  "title": "Product Performance",
  "data": {
    "categories": ["Product A", "Product B", "Product C"],
    "series": [{"name": "Revenue", "values": [85, 92, 78]}]
  },
  "style": {
    "palette": ["#2563EB", "#10B981", "#F59E0B"]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Line Chart](./line_chart.md) - For trends over time
- [Waterfall Chart](./waterfall.md) - For cumulative changes
