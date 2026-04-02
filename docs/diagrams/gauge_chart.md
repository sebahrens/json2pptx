# Gauge Chart

Display a single KPI value against thresholds on a semicircular dial.

## Type Identifier

`gauge_chart`

**Aliases:** `gauge`

## Use Cases

- SLA uptime monitoring
- KPI dashboards
- Performance score display
- Budget utilization meters

## Data Structure

```json
{
  "type": "gauge_chart",
  "title": "SLA Uptime",
  "data": {
    "value": 99.2,
    "min": 0,
    "max": 100,
    "unit": "%",
    "thresholds": [
      {"value": 95, "color": "#EF4444", "label": "Critical"},
      {"value": 99, "color": "#F59E0B", "label": "Warning"},
      {"value": 100, "color": "#10B981", "label": "Healthy"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `value` | `number` | Current gauge value |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `min` | `number` | `0` | Minimum scale value |
| `max` | `number` | `100` | Maximum scale value (auto-detects 0-1 range) |
| `label` | `string` | - | Value label |
| `unit` | `string` | - | Unit suffix (e.g., `"%"`, `"ms"`) |
| `start_angle` | `number` | - | Gauge arc start angle |
| `end_angle` | `number` | - | Gauge arc end angle |
| `thresholds` | `object[]` | - | Color bands on the gauge |
| `thresholds[].value` | `number` | - | Upper bound of this band |
| `thresholds[].color` | `string` | - | Hex color for this band |
| `thresholds[].label` | `string` | - | Band label |

## Examples

### Simple Percentage

```json
{
  "type": "gauge_chart",
  "title": "CPU Utilization",
  "data": {
    "value": 72,
    "unit": "%"
  }
}
```

### Decimal Range (0-1)

```json
{
  "type": "gauge_chart",
  "title": "Model Accuracy",
  "data": {
    "value": 0.94,
    "min": 0.0,
    "max": 1.0
  }
}
```

### With Thresholds

```json
{
  "type": "gauge_chart",
  "title": "Response Time",
  "data": {
    "value": 180,
    "min": 0,
    "max": 500,
    "unit": "ms",
    "thresholds": [
      {"value": 100, "color": "#10B981", "label": "Fast"},
      {"value": 250, "color": "#F59E0B", "label": "OK"},
      {"value": 500, "color": "#EF4444", "label": "Slow"}
    ]
  }
}
```

## See Also

- [KPI Dashboard](./kpi_dashboard.md) - For multiple metrics at once
- [Bar Chart](./bar_chart.md) - For comparing values across categories
