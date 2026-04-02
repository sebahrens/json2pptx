# Treemap Chart

Display hierarchical data as nested rectangles sized by value.

## Type Identifier

`treemap_chart`

**Aliases:** `treemap`

## Use Cases

- Revenue breakdown by segment
- Disk usage visualization
- Portfolio allocation
- Market capitalization by sector

## Data Structure

```json
{
  "type": "treemap_chart",
  "title": "Revenue Mix",
  "data": {
    "nodes": [
      {"label": "Enterprise", "value": 55},
      {"label": "Mid-Market", "value": 30},
      {"label": "SMB", "value": 15}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | `object[]` | Treemap items. Alias: `items` |
| `nodes[].label` | `string` | Item label |
| `nodes[].value` | `number` | Item size |

Alternative flat format:

| Field | Type | Description |
|-------|------|-------------|
| `values` | `number[]` | Values per item |
| `categories` | `string[]` | Item labels |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `nodes[].children` | `object[]` | - | Nested sub-items (hierarchical) |
| `nodes[].color` | `string` | - | Custom hex color |
| `padding` | `number` | - | Cell padding |
| `corner_radius` | `number` | - | Rounded corners |
| `label_position` | `string` | - | Label placement |

## Examples

### Hierarchical

```json
{
  "type": "treemap_chart",
  "title": "Department Budget",
  "data": {
    "nodes": [
      {
        "label": "Engineering",
        "value": 500,
        "children": [
          {"label": "Backend", "value": 250},
          {"label": "Frontend", "value": 150},
          {"label": "DevOps", "value": 100}
        ]
      },
      {
        "label": "Marketing",
        "value": 200,
        "children": [
          {"label": "Digital", "value": 120},
          {"label": "Brand", "value": 80}
        ]
      },
      {"label": "Operations", "value": 150}
    ]
  }
}
```

### Flat List

```json
{
  "type": "treemap_chart",
  "title": "Market Share",
  "data": {
    "nodes": [
      {"label": "Company A", "value": 35},
      {"label": "Company B", "value": 25},
      {"label": "Company C", "value": 20},
      {"label": "Company D", "value": 12},
      {"label": "Others", "value": 8}
    ]
  }
}
```

## See Also

- [Pie Chart](./pie_chart.md) - For simple proportional display
- [Donut Chart](./donut_chart.md) - For part-to-whole with center
