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

The default drawing (`style: "steps"`) sets each stage as a centred bar whose width follows its value, joined to the next by a pale connector: the stage name stands bold in a column on the left, the value inside the bar, and the stage-to-stage conversion in a column on the right, level with the connector it describes. The widest bar is capped at 2.6 times the funnel's height, so a wide slide body does not stretch the first stage edge to edge.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `stages[].color` | `string` | - | Custom hex color per stage |
| `style` | `string` | `steps` | `steps`: centred bars with connectors (above). `tapered`: the earlier stack of trapezoids with labels inside; `neck_width` or `label_position` selects it too |
| `neck_width` | `number` | - | Tapered style: width of funnel bottom, as a fraction of the last stage's own top width. Unset in `clamped` mode it becomes 0.55 rather than a point, so the smallest stage keeps room for its label |
| `width_mode` | `string` | `clamped` | How a stage's width follows its value. `clamped` is proportional with a floor: the width that holds the stage's value (steps), or 25% of the plot (tapered); `proportional` is exactly proportional — a 12,400 → 212 funnel ends in a 2px stick, its value set beside it; `equal` gives every stage the full width |
| `show_conversion` | `bool` | `true` | Stage-to-stage conversion ("25% of Visitors") — the number a funnel exists to show |
| `gap` | `number` | - | Spacing between stages |
| `show_percentage` | `bool` | `false` | Append each stage as a percentage of the LARGEST stage (the first stage in a normal narrowing funnel) to its label (distinct from `show_conversion`, which is stage-to-stage) |
| `label_position` | `string` | - | Tapered style: label placement (`inside`, `left`, `right`) |

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
