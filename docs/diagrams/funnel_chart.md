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
| `neck_width` | `number` | - | Width of funnel bottom, as a fraction of the last stage's own top width. Unset in `clamped` mode it becomes 0.55 rather than a point, so the smallest stage keeps room for its label |
| `width_mode` | `string` | `clamped` | How a stage's width follows its value. `clamped` interpolates between a 25%-of-plot floor and the full width (ordering preserved, every stage still a shape); `proportional` is exactly proportional — a 12,400 → 212 funnel ends in a 2px stick; `equal` gives every stage the full width |
| `show_conversion` | `bool` | `true` | Stage-to-stage conversion under each stage's label ("25% of Visitors") — the number a funnel exists to show |
| `gap` | `number` | - | Spacing between stages |
| `show_percentage` | `bool` | `false` | Append each stage as a percentage of the FIRST stage to its label (distinct from `show_conversion`, which is stage-to-stage) |
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
