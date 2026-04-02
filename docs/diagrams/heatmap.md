# Heatmap

Display a matrix of values as color-coded cells, revealing patterns and concentrations.

## Type Identifier

`heatmap`

## Use Cases

- Correlation matrices
- Time-of-day activity patterns
- Risk assessment grids
- Feature usage frequency

## Data Structure

```json
{
  "type": "heatmap",
  "title": "Weekly Activity",
  "data": {
    "row_labels": ["Mon", "Tue", "Wed", "Thu", "Fri"],
    "col_labels": ["9am", "10am", "11am", "12pm", "1pm", "2pm", "3pm", "4pm"],
    "values": [
      [3, 5, 8, 6, 2, 4, 7, 5],
      [4, 6, 9, 7, 3, 5, 8, 4],
      [2, 4, 7, 8, 4, 6, 6, 3],
      [5, 7, 8, 5, 3, 4, 5, 4],
      [3, 5, 6, 4, 2, 3, 4, 2]
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `values` | `number[][]` | 2D array of cell values. Alias: `rows` |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `row_labels` | `string[]` | - | Row headers. Alias: `y_labels` |
| `col_labels` | `string[]` | - | Column headers. Aliases: `x_labels`, `column_labels` |
| `color_scale` | `string` | - | Color scale type (e.g., `"viridis"`) |

## Examples

### Correlation Matrix

```json
{
  "type": "heatmap",
  "title": "Feature Correlation",
  "data": {
    "row_labels": ["Price", "Quality", "Speed", "Support"],
    "col_labels": ["Price", "Quality", "Speed", "Support"],
    "values": [
      [1.0, 0.3, -0.2, 0.1],
      [0.3, 1.0, 0.5, 0.7],
      [-0.2, 0.5, 1.0, 0.4],
      [0.1, 0.7, 0.4, 1.0]
    ]
  }
}
```

### Simple Grid

```json
{
  "type": "heatmap",
  "title": "Server Load",
  "data": {
    "row_labels": ["Server A", "Server B", "Server C"],
    "col_labels": ["CPU", "Memory", "Disk", "Network"],
    "values": [
      [85, 60, 40, 70],
      [45, 80, 55, 30],
      [70, 50, 90, 60]
    ]
  }
}
```

## See Also

- [Nine Box Talent](./nine_box_talent.md) - For 3x3 assessment grids
- [2x2 Matrix](./matrix_2x2.md) - For quadrant positioning
