# Waterfall Chart

Show how an initial value is affected by a series of intermediate positive or negative values.

## Type Identifier

`waterfall`

## Use Cases

- Profit and loss breakdowns
- Budget variance analysis
- Step-by-step value changes
- Financial bridge charts

## Data Structure

```json
{
  "type": "waterfall",
  "title": "Profit Breakdown",
  "data": {
    "points": [
      {"label": "Revenue", "value": 1000, "type": "total"},
      {"label": "COGS", "value": -400, "type": "decrease"},
      {"label": "Marketing", "value": -150, "type": "decrease"},
      {"label": "R&D", "value": -100, "type": "decrease"},
      {"label": "Other Income", "value": 50, "type": "increase"},
      {"label": "Net Profit", "value": 400, "type": "total"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `points` | `object[]` | Data points for the waterfall |
| `points[].label` | `string` | Category label |
| `points[].value` | `number` | Value (positive or negative) |
| `points[].type` | `string` | Point type: `total`, `increase`, `decrease`, `subtotal` |

## Point Types

| Type | Description | Color |
|------|-------------|-------|
| `total` | Starting or ending total (from baseline) | Blue |
| `increase` | Positive change (floating bar up) | Green |
| `decrease` | Negative change (floating bar down) | Red |
| `subtotal` | Intermediate subtotal (from baseline) | Blue |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `footnote` | `string` | - | Footnote text |

## Style Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_values` | `bool` | `true` | Show value labels on bars |
| `show_legend` | `bool` | `false` | Display legend |

## Examples

### Revenue to Profit Bridge

```json
{
  "type": "waterfall",
  "title": "Q1 2024 Financials",
  "subtitle": "Revenue to Net Income",
  "data": {
    "points": [
      {"label": "Revenue", "value": 500, "type": "total"},
      {"label": "Cost of Sales", "value": -200, "type": "decrease"},
      {"label": "Gross Profit", "value": 300, "type": "subtotal"},
      {"label": "Operating Exp", "value": -100, "type": "decrease"},
      {"label": "Interest", "value": -20, "type": "decrease"},
      {"label": "Tax Benefit", "value": 15, "type": "increase"},
      {"label": "Net Income", "value": 195, "type": "total"}
    ]
  }
}
```

### Year-over-Year Change

```json
{
  "type": "waterfall",
  "title": "YoY Revenue Change",
  "data": {
    "points": [
      {"label": "2023 Revenue", "value": 800, "type": "total"},
      {"label": "New Customers", "value": 150, "type": "increase"},
      {"label": "Upsells", "value": 75, "type": "increase"},
      {"label": "Churn", "value": -50, "type": "decrease"},
      {"label": "Price Changes", "value": 25, "type": "increase"},
      {"label": "2024 Revenue", "value": 1000, "type": "total"}
    ]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Bar Chart](./bar_chart.md) - For category comparisons
- [Line Chart](./line_chart.md) - For trends
