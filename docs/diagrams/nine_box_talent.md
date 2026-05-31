# Nine Box Talent Grid

HR performance/potential assessment using a 3x3 grid.

## Type Identifier

`nine_box_talent`

## Use Cases

- Talent assessment
- Succession planning
- Performance reviews
- Development planning

## Data Structure

```json
{
  "type": "nine_box_talent",
  "title": "Team Assessment",
  "data": {
    "x_axis_label": "Performance",
    "y_axis_label": "Potential",
    "cells": [
      {"position": {"row": 0, "col": 2}, "items": [{"name": "Alice"}]},
      {"position": {"row": 1, "col": 1}, "items": [{"name": "Bob"}, {"name": "Carol"}]},
      {"position": {"row": 2, "col": 0}, "items": [{"name": "Dave"}]}
    ]
  }
}
```

## Grid Layout

```
              LOW         MEDIUM        HIGH
           (col 0)       (col 1)      (col 2)
         ┌───────────┬───────────┬───────────┐
HIGH     │  Enigma   │  Growth   │   Star    │  row 0
(row 0)  │           │  Employee │           │
         ├───────────┼───────────┼───────────┤
MEDIUM   │  Dilemma  │   Core    │   High    │  row 1
(row 1)  │           │  Employee │ Performer │
         ├───────────┼───────────┼───────────┤
LOW      │   Under   │  Average  │   Solid   │  row 2
(row 2)  │ Performer │ Performer │ Performer │
         └───────────┴───────────┴───────────┘
               ← PERFORMANCE →
```

## Required Fields

Provide **one** of two mutually exclusive forms: explicit `cells` (most reliable) or `employees` (auto-routed). If both are present, `cells` wins.

| Field | Type | Description |
|-------|------|-------------|
| `cells` | `object[]` | Cell data |
| `cells[].position` | `object` | Grid position |
| `cells[].position.row` | `number` | Row (0=top/high potential, 2=bottom/low) |
| `cells[].position.col` | `number` | Column (0=left/low performance, 2=right/high) |

> Flat `cells[].row` / `cells[].col` are also accepted as an alternative to the nested `position` object.

### `employees` form (auto-routed)

Instead of placing people in explicit cells, supply a flat `employees` list and let the engine route each person to a cell from their performance/potential rating:

```json
{
  "type": "nine_box_talent",
  "data": {
    "employees": [
      {"name": "Alice", "performance": "high", "potential": "high"},
      {"name": "Bob",   "performance": "medium", "potential": "medium"},
      {"name": "Dave",  "performance": "low",  "potential": "low"}
    ]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `employees[].name` | `string` | Person name (required; blank entries are skipped) |
| `employees[].performance` | `string` | **`"low"` \| `"medium"` \| `"high"`** — picks the column (low=left, medium=center, high=right). **Must be a string, not a number.** |
| `employees[].potential` | `string` | **`"low"` \| `"medium"` \| `"high"`** — picks the row (high=top, medium=center, low=bottom). **Must be a string, not a number.** |

> **Common mistake (do not do this):** numeric ratings like `"performance": 1` or `"potential": 3` are **ignored** — they are not strings, so every employee silently falls through to the center "Core Employee" cell. Always use the words `"low"`, `"medium"`, or `"high"`.

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `x_axis_label` | `string` | `Performance` | X-axis label |
| `y_axis_label` | `string` | `Potential` | Y-axis label |
| `x_axis_labels` | `string[3]` | `[Low, Medium, High]` | Column labels |
| `y_axis_labels` | `string[3]` | `[Low, Medium, High]` | Row labels |
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `footnote` | `string` | - | Footnote text |

## Cell Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `label` | `string` | Override default cell label |
| `description` | `string` | Additional details |
| `color` | `string` | Custom background color |
| `items` | `object[]` | People/items in this cell |

## Item Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Item/person name |
| `subtitle` | `string` | Additional info (role, team) |
| `size` | `number` | Bubble size (relative) |
| `color` | `string` | Custom bubble color |

## Default Cell Labels

| Position | Default Label |
|----------|---------------|
| (0,0) | Enigma |
| (0,1) | Growth Employee |
| (0,2) | Star |
| (1,0) | Dilemma |
| (1,1) | Core Employee |
| (1,2) | High Performer |
| (2,0) | Under Performer |
| (2,1) | Average Performer |
| (2,2) | Solid Performer |

## Examples

### Basic Team Assessment

```json
{
  "type": "nine_box_talent",
  "title": "Engineering Team Assessment",
  "subtitle": "Q4 2024",
  "data": {
    "x_axis_label": "Performance",
    "y_axis_label": "Potential",
    "cells": [
      {
        "position": {"row": 0, "col": 2},
        "items": [{"name": "Sarah", "subtitle": "Tech Lead"}]
      },
      {
        "position": {"row": 0, "col": 1},
        "items": [{"name": "Mike"}, {"name": "Lisa"}]
      },
      {
        "position": {"row": 1, "col": 2},
        "items": [{"name": "Alex"}, {"name": "Jordan"}, {"name": "Chris"}]
      },
      {
        "position": {"row": 1, "col": 1},
        "items": [{"name": "Pat"}, {"name": "Sam"}]
      },
      {
        "position": {"row": 2, "col": 1},
        "items": [{"name": "Taylor"}]
      }
    ]
  }
}
```

### Custom Labels

```json
{
  "type": "nine_box_talent",
  "title": "Leadership Pipeline",
  "data": {
    "x_axis_label": "Current Impact",
    "y_axis_label": "Growth Trajectory",
    "x_axis_labels": ["Developing", "Solid", "Exceptional"],
    "y_axis_labels": ["Steady", "Growing", "Accelerating"],
    "cells": [
      {
        "position": {"row": 0, "col": 2},
        "label": "Future Executives",
        "items": [{"name": "VP Candidate 1"}]
      },
      {
        "position": {"row": 1, "col": 2},
        "label": "Promotion Ready",
        "items": [{"name": "Director 1"}, {"name": "Director 2"}]
      }
    ]
  }
}
```

### With Bubble Sizes

```json
{
  "type": "nine_box_talent",
  "title": "Department Overview",
  "data": {
    "cells": [
      {
        "position": {"row": 0, "col": 2},
        "items": [
          {"name": "Team A", "size": 20, "subtitle": "5 people"},
          {"name": "Team B", "size": 12, "subtitle": "3 people"}
        ]
      },
      {
        "position": {"row": 1, "col": 1},
        "items": [
          {"name": "Team C", "size": 25, "subtitle": "7 people"}
        ]
      }
    ]
  }
}
```

## Color Schemes

| Scheme | Description |
|--------|-------------|
| `traffic_light` | Red/yellow/green gradient (default) |
| `blue_gradient` | Light to dark blue |
| `gray_scale` | Grayscale |

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [2x2 Matrix](./matrix_2x2.md) - Simpler grid
- [Business Model Canvas](./business_model_canvas.md) - 9-section canvas
