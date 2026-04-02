# 2x2 Matrix

Position items on two dimensions for prioritization or analysis.

## Type Identifier

`matrix_2x2`

## Use Cases

- Priority/effort matrices
- BCG growth-share matrix
- Ansoff matrix
- Risk assessment
- Feature prioritization

## Data Structure

```json
{
  "type": "matrix_2x2",
  "title": "Priority Matrix",
  "data": {
    "x_axis_label": "Effort",
    "y_axis_label": "Value",
    "quadrant_labels": ["Quick Wins", "Major Projects", "Fill-Ins", "Thankless Tasks"],
    "points": [
      {"label": "Feature A", "x": 20, "y": 80},
      {"label": "Feature B", "x": 70, "y": 90},
      {"label": "Feature C", "x": 30, "y": 20},
      {"label": "Feature D", "x": 85, "y": 15}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `points` | `object[]` | Items to plot |
| `points[].label` | `string` | Point label |
| `points[].x` | `number` | X coordinate (0-100) |
| `points[].y` | `number` | Y coordinate (0-100) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `x_axis_label` | `string` | `Effort` | X-axis label |
| `y_axis_label` | `string` | `Value` | Y-axis label |
| `quadrant_labels` | `string[4]` | See below | Labels for [TL, TR, BL, BR] |
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |

## Default Quadrant Labels

```
[0] Top-Left:     "High Value / Low Effort"
[1] Top-Right:    "High Value / High Effort"
[2] Bottom-Left:  "Low Value / Low Effort"
[3] Bottom-Right: "Low Value / High Effort"
```

## Point Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `size` | `number` | Point radius (default: 12) |
| `color` | `string` | Custom hex color |
| `description` | `string` | Additional details |

## Examples

### Eisenhower Matrix

```json
{
  "type": "matrix_2x2",
  "title": "Eisenhower Matrix",
  "data": {
    "x_axis_label": "Urgency",
    "y_axis_label": "Importance",
    "quadrant_labels": ["Schedule", "Do First", "Delegate", "Eliminate"],
    "points": [
      {"label": "Strategic Planning", "x": 15, "y": 85},
      {"label": "Customer Emergency", "x": 90, "y": 95},
      {"label": "Email Replies", "x": 75, "y": 25},
      {"label": "Social Media", "x": 40, "y": 15}
    ]
  }
}
```

### BCG Matrix

```json
{
  "type": "matrix_2x2",
  "title": "BCG Growth-Share Matrix",
  "data": {
    "x_axis_label": "Market Share",
    "y_axis_label": "Market Growth",
    "quadrant_labels": ["Question Marks", "Stars", "Dogs", "Cash Cows"],
    "points": [
      {"label": "Product A", "x": 75, "y": 80, "size": 20},
      {"label": "Product B", "x": 80, "y": 25, "size": 25},
      {"label": "Product C", "x": 30, "y": 70, "size": 12},
      {"label": "Product D", "x": 20, "y": 20, "size": 8}
    ]
  }
}
```

### Risk Assessment

```json
{
  "type": "matrix_2x2",
  "title": "Risk Assessment",
  "data": {
    "x_axis_label": "Likelihood",
    "y_axis_label": "Impact",
    "quadrant_labels": ["Monitor", "Mitigate", "Accept", "Transfer"],
    "points": [
      {"label": "Data Breach", "x": 40, "y": 95, "color": "#DC2626"},
      {"label": "Server Outage", "x": 60, "y": 80, "color": "#F59E0B"},
      {"label": "Staff Turnover", "x": 70, "y": 40, "color": "#F59E0B"},
      {"label": "Minor Bug", "x": 80, "y": 15, "color": "#10B981"}
    ]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Nine Box Talent](./nine_box_talent.md) - 3x3 grid variant
- [Porter's Five Forces](./porters_five_forces.md) - Industry analysis
