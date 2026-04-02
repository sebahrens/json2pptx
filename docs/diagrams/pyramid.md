# Pyramid

Display hierarchical levels as stacked trapezoids, widening from top to bottom.

## Type Identifier

`pyramid`

## Use Cases

- Maslow's hierarchy of needs
- Organizational hierarchy tiers
- Priority frameworks
- Strategy cascades (vision → execution)

## Data Structure

```json
{
  "type": "pyramid",
  "title": "Strategic Priorities",
  "data": {
    "levels": [
      {"label": "Vision", "description": "Long-term aspiration"},
      {"label": "Strategy", "description": "3-year plan"},
      {"label": "Objectives", "description": "Annual goals"},
      {"label": "Initiatives", "description": "Key projects"},
      {"label": "Tasks", "description": "Day-to-day execution"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `levels` | `object[]` | Pyramid levels (top to bottom) |
| `levels[].label` | `string` | Level name |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |
| `levels[].description` | `string` | - | Additional text for the level |
| `levels[].color` | `string` | - | Custom hex color for this level |
| `gap` | `number` | - | Spacing between levels |
| `top_width_ratio` | `number` | - | Relative width of top level (0-1) |
| `label_position` | `string` | `"inside"` | `"inside"` or `"right"` |
| `footnote` | `string` | - | Footnote text below diagram |

## Examples

### Maslow's Hierarchy

```json
{
  "type": "pyramid",
  "title": "Maslow's Hierarchy of Needs",
  "data": {
    "levels": [
      {"label": "Self-actualization"},
      {"label": "Esteem"},
      {"label": "Love/Belonging"},
      {"label": "Safety"},
      {"label": "Physiological"}
    ]
  }
}
```

### With Custom Colors

```json
{
  "type": "pyramid",
  "title": "Customer Segments",
  "data": {
    "levels": [
      {"label": "Enterprise", "description": "$1M+ ARR", "color": "#1E40AF"},
      {"label": "Mid-Market", "description": "$100K-$1M", "color": "#2563EB"},
      {"label": "SMB", "description": "$10K-$100K", "color": "#3B82F6"},
      {"label": "Self-Serve", "description": "<$10K", "color": "#60A5FA"}
    ]
  }
}
```

## See Also

- [Funnel Chart](./funnel_chart.md) - For conversion/drop-off stages
- [Process Flow](./process_flow.md) - For sequential workflows
