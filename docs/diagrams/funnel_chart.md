# Funnel Chart

Show progressive reduction through stages, such as conversion funnels or sales pipelines.

## Type Identifier

`funnel_chart`

**Aliases:** `funnel`

## Use Cases

- Sales pipeline conversion
- Marketing funnel drop-off
- Recruitment process stages
- Customer journey stages

## Data Structure

```json
{
  "type": "funnel_chart",
  "title": "Sales Funnel",
  "data": {
    "stages": [
      {"label": "Visitors", "value": 10000},
      {"label": "Leads", "value": 3000},
      {"label": "Qualified", "value": 800},
      {"label": "Proposals", "value": 200},
      {"label": "Closed", "value": 50}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `stages` | `object[]` | Funnel stages (widest to narrowest). Aliases: `values`, `points` |
| `stages[].label` | `string` | Stage label |
| `stages[].value` | `number` | Stage value |

Alternative flat format:

| Field | Type | Description |
|-------|------|-------------|
| `values` | `number[]` | Values per stage |
| `categories` | `string[]` | Stage labels |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `stages[].color` | `string` | - | Custom hex color per stage |
| `neck_width` | `number` | - | Width of funnel bottom |
| `gap` | `number` | - | Spacing between stages |
| `show_percentage` | `bool` | `false` | Display percentage change |
| `label_position` | `string` | - | Label placement |

## Examples

### Recruitment Funnel

```json
{
  "type": "funnel_chart",
  "title": "Hiring Pipeline",
  "data": {
    "stages": [
      {"label": "Applications", "value": 500},
      {"label": "Phone Screen", "value": 120},
      {"label": "Technical", "value": 40},
      {"label": "Onsite", "value": 15},
      {"label": "Offers", "value": 5}
    ]
  }
}
```

### With Percentages

```json
{
  "type": "funnel_chart",
  "title": "Signup Flow",
  "data": {
    "stages": [
      {"label": "Landing Page", "value": 5000},
      {"label": "Signup Started", "value": 1200},
      {"label": "Email Verified", "value": 800},
      {"label": "Profile Complete", "value": 400},
      {"label": "First Purchase", "value": 100}
    ],
    "show_percentage": true
  }
}
```

## See Also

- [Pyramid](./pyramid.md) - For hierarchical level visualization
- [Bar Chart](./bar_chart.md) - For category comparisons
