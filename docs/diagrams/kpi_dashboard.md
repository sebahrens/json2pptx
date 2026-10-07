# KPI Dashboard

Display multiple key performance indicators as a grid of big numbers with optional trends and deltas.

By default the dashboard is open, like the `kpi-Nup` patterns: each value (28pt bold, in the accent) stands on the page over its label (14pt, as authored — not capitalised) and its delta (12pt, in the trend ink), with a hairline divider between the columns of a row and no tile behind the numbers. `style.colors` brings back tinted cards, one colour per metric in order.

## Type Identifier

`kpi_dashboard`

## Use Cases

- Executive summary dashboards
- Monthly performance snapshots
- SLA monitoring displays
- Financial highlights

## Data Structure

```json
{
  "type": "kpi_dashboard",
  "title": "Q1 Performance",
  "data": {
    "metrics": [
      {"label": "Revenue", "value": "$2.4M", "delta": "+12%", "trend": "up"},
      {"label": "Customers", "value": "1,250", "delta": "+85", "trend": "up"},
      {"label": "Churn Rate", "value": "3.2%", "delta": "-0.5%", "trend": "down"},
      {"label": "NPS Score", "value": "72", "delta": "+4", "trend": "up"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `metrics` | `object[]` | KPI cards. Alias: `kpis` |
| `metrics[].label` | `string` | Metric name |
| `metrics[].value` | `string\|number` | Current value |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Dashboard title |
| `subtitle` | `string` | - | Subtitle below title |
| `metrics[].delta` | `string` | - | Change indicator (e.g., `"+5%"`) |
| `metrics[].trend` | `string` | - | Trend direction: `"up"`, `"down"` or `"flat"`. Sets the arrow before the change |
| `metrics[].good_direction` | `string` | `"up"` | The direction in which the metric improves: `"up"` or `"down"`. Sets the colour of the change: a trend in the good direction takes the positive ink, a trend against it the negative ink. Set `"down"` for a cost, a churn rate, an escalation rate or a latency, so that `{"change": "-12%", "trend": "down", "good_direction": "down"}` reads as good news. The arrow still shows which way the metric moved |
| `gap` | `number` | - | Spacing between cards |
| `max_columns` | `number` | - | Cards per row |
| `corner_radius` | `number` | - | Card corner radius |

## Examples

### Financial Summary

```json
{
  "type": "kpi_dashboard",
  "title": "Monthly Financials",
  "data": {
    "metrics": [
      {"label": "MRR", "value": "$850K", "delta": "+8%", "trend": "up"},
      {"label": "ARR", "value": "$10.2M", "delta": "+15%", "trend": "up"},
      {"label": "Burn Rate", "value": "$120K/mo", "delta": "-5%", "trend": "down"},
      {"label": "Runway", "value": "18 months"}
    ]
  }
}
```

### Operational Metrics

```json
{
  "type": "kpi_dashboard",
  "title": "System Health",
  "data": {
    "metrics": [
      {"label": "Uptime", "value": "99.97%"},
      {"label": "P95 Latency", "value": "142ms", "delta": "-18ms", "trend": "down"},
      {"label": "Error Rate", "value": "0.02%", "delta": "-0.01%", "trend": "down"},
      {"label": "Deploys/Week", "value": "12", "delta": "+3", "trend": "up"}
    ],
    "max_columns": 4
  }
}
```

## See Also

- [Gauge Chart](./gauge_chart.md) - For single KPI with thresholds
- [Panel Layout](./panel_layout.md) - For custom card layouts with icons
