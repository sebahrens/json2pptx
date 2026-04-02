# SVGGen Diagram Gallery

The `svggen` package provides built-in diagram types for creating professional business graphics. This guide helps you choose the right diagram for your data.

## Quick Reference

| Type | Best For | Data Structure |
|------|----------|----------------|
| `bar_chart` | Comparing categories | Series + Categories |
| `line_chart` | Trends over time | Series + Categories |
| `pie_chart` | Part-to-whole | Series with values |
| `donut_chart` | Part-to-whole (with center) | Series with values |
| `waterfall` | Cumulative change | Points with types |
| `timeline` | Project schedules | Activities with dates |
| `process_flow` | Workflows | Steps + Connections |
| `matrix_2x2` | Priority/analysis | Points with X/Y |
| `porters_five_forces` | Industry analysis | Five forces data |
| `business_model_canvas` | Business strategy | 9 sections |
| `value_chain` | Operations flow | Primary + Support activities |
| `nine_box_talent` | HR assessment | 3x3 grid with items |
| `house_diagram` | Radial relationships | Center + Outer elements |
| `funnel_chart` | Conversion stages | Stages + values |
| `gauge_chart` | KPI thresholds | Value + thresholds |
| `treemap_chart` | Hierarchical breakdown | Nested categories |
| `area_chart` | Filled trends over time | Series + Categories |
| `stacked_bar_chart` | Stacked category bars | Series + Categories |
| `stacked_area_chart` | Cumulative filled areas | Series + Categories |
| `grouped_bar_chart` | Side-by-side bars | Series + Categories |
| `radar_chart` | Multi-axis profiling | Categories + Series |
| `scatter_chart` | Correlation plotting | Series with X/Y values |
| `bubble_chart` | Scatter + size dimension | Series with X/Y/size |
| `swot` | SWOT analysis | Four quadrant lists |
| `pestel` | Macro-environment scan | Six category lists |
| `pyramid` | Hierarchical levels | Levels array |
| `venn` | Set overlap | 2-3 Circles + Intersections |
| `org_chart` | Organization hierarchy | Tree of nodes |
| `gantt` | Project schedule | Tasks + Milestones |
| `fishbone` | Cause-and-effect | Effect + Categories |
| `heatmap` | Color-coded matrix | 2D value array |
| `kpi_dashboard` | Metrics card grid | Metrics array |
| `panel_layout` | Icon/stat card panels | Panels array |

## Decision Tree

Use this flowchart to choose the right diagram type:

```
What are you visualizing?
│
├── Numerical comparisons?
│   ├── Categories vs. values? → bar_chart
│   ├── Grouped comparison? → grouped_bar_chart
│   ├── Stacked composition? → stacked_bar_chart
│   ├── Trends over time? → line_chart or area_chart
│   ├── Cumulative trends? → stacked_area_chart
│   ├── Part of whole? → pie_chart or donut_chart
│   ├── Hierarchical breakdown? → treemap_chart
│   ├── Running total/changes? → waterfall
│   ├── Correlation? → scatter_chart
│   ├── Correlation + size? → bubble_chart
│   ├── Multi-axis profile? → radar_chart
│   ├── Color-coded matrix? → heatmap
│   └── Single KPI dial? → gauge_chart
│
├── Process or timeline?
│   ├── Linear workflow? → process_flow
│   ├── Project schedule? → timeline or gantt
│   ├── Sequential value creation? → value_chain
│   ├── Conversion stages? → funnel_chart
│   └── Cause and effect? → fishbone
│
├── Strategic frameworks?
│   ├── 2x2 prioritization? → matrix_2x2
│   ├── Competitive forces? → porters_five_forces
│   ├── Business model? → business_model_canvas
│   ├── SWOT analysis? → swot
│   ├── Macro environment? → pestel
│   ├── Hierarchy of levels? → pyramid
│   ├── Set overlap? → venn
│   └── Performance/potential? → nine_box_talent
│
├── Organizational?
│   ├── Reporting hierarchy? → org_chart
│   └── KPI summary? → kpi_dashboard
│
├── Content layouts?
│   └── Icon/stat cards? → panel_layout
│
└── Relationships?
    └── Central concept + related items? → house_diagram
```

## Diagram Details

### Charts (Data Visualization)

#### Funnel (`funnel`)
Show conversion stages and drop-off.

```json
{
  "type": "funnel",
  "title": "Sales Funnel",
  "data": {
    "stages": [
      {"label": "Visitors", "value": 1000},
      {"label": "Signups", "value": 250},
      {"label": "Trials", "value": 80},
      {"label": "Customers", "value": 30}
    ]
  }
}
```

#### Gauge (`gauge`)
Show KPI status against thresholds.

```json
{
  "type": "gauge",
  "title": "SLA Uptime",
  "data": {
    "value": 0.92,
    "min": 0.0,
    "max": 1.0,
    "target": 0.99
  }
}
```

#### Treemap (`treemap`)
Show hierarchical breakdowns.

```json
{
  "type": "treemap",
  "title": "Revenue Mix",
  "data": {
    "items": [
      {"label": "Enterprise", "value": 55},
      {"label": "SMB", "value": 30},
      {"label": "Startup", "value": 15}
    ]
  }
}
```


#### Bar Chart (`bar_chart`)
Compare values across categories.

```json
{
  "type": "bar_chart",
  "title": "Sales by Region",
  "data": {
    "categories": ["North", "South", "East", "West"],
    "series": [
      {"name": "Q1", "values": [100, 80, 120, 90]},
      {"name": "Q2", "values": [110, 95, 115, 85]}
    ]
  }
}
```

#### Line Chart (`line_chart`)
Show trends over time or continuous data.

```json
{
  "type": "line_chart",
  "title": "Monthly Revenue",
  "data": {
    "categories": ["Jan", "Feb", "Mar", "Apr"],
    "series": [
      {"name": "2024", "values": [1000, 1200, 1100, 1400]}
    ]
  }
}
```

#### Pie Chart (`pie_chart`)
Display proportions of a whole.

```json
{
  "type": "pie_chart",
  "title": "Market Share",
  "data": {
    "series": [
      {"name": "Product A", "value": 45},
      {"name": "Product B", "value": 30},
      {"name": "Product C", "value": 25}
    ]
  }
}
```

#### Donut Chart (`donut_chart`)
Pie chart with center space (for metrics or text).

```json
{
  "type": "donut_chart",
  "title": "Budget Allocation",
  "data": {
    "series": [
      {"name": "R&D", "value": 35},
      {"name": "Marketing", "value": 25},
      {"name": "Operations", "value": 40}
    ]
  }
}
```

### Business Diagrams

#### Waterfall Chart (`waterfall`)
Show how values add/subtract to reach a total.

```json
{
  "type": "waterfall",
  "title": "Profit Breakdown",
  "data": {
    "points": [
      {"label": "Revenue", "value": 1000, "type": "total"},
      {"label": "COGS", "value": -400, "type": "decrease"},
      {"label": "Marketing", "value": -150, "type": "decrease"},
      {"label": "R&D", "value": -100, "type": "decrease"},
      {"label": "Profit", "value": 350, "type": "total"}
    ]
  }
}
```

#### Timeline (`timeline`)
Visualize project schedules with activities and milestones.

```json
{
  "type": "timeline",
  "title": "Project Roadmap",
  "data": {
    "activities": [
      {"id": "1", "label": "Planning", "type": "activity", "start_date": "2024-01-01", "end_date": "2024-02-15"},
      {"id": "2", "label": "Development", "type": "activity", "start_date": "2024-02-15", "end_date": "2024-06-01"},
      {"id": "3", "label": "Launch", "type": "milestone", "date": "2024-06-15"}
    ]
  }
}
```

#### Process Flow (`process_flow`)
Document workflows with steps and decisions.

```json
{
  "type": "process_flow",
  "title": "Order Processing",
  "data": {
    "steps": [
      {"id": "start", "label": "Order Received", "type": "start"},
      {"id": "check", "label": "Check Inventory", "type": "decision"},
      {"id": "ship", "label": "Ship Order", "type": "step"},
      {"id": "end", "label": "Complete", "type": "end"}
    ],
    "connections": [
      {"from": "start", "to": "check"},
      {"from": "check", "to": "ship", "label": "Yes"},
      {"from": "ship", "to": "end"}
    ]
  }
}
```

### Consulting Frameworks

#### 2x2 Matrix (`matrix_2x2`)
Position items on two dimensions (e.g., Value vs. Effort).

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
      {"label": "Feature C", "x": 30, "y": 20}
    ]
  }
}
```

#### Porter's Five Forces (`porters_five_forces`)
Analyze industry competitive dynamics.

```json
{
  "type": "porters_five_forces",
  "title": "Industry Analysis",
  "data": {
    "industry_name": "Cloud Computing",
    "forces": [
      {"type": "rivalry", "label": "Competitive Rivalry", "intensity": 0.8},
      {"type": "new_entrants", "label": "New Entrants", "intensity": 0.4},
      {"type": "substitutes", "label": "Substitutes", "intensity": 0.3},
      {"type": "suppliers", "label": "Supplier Power", "intensity": 0.5},
      {"type": "buyers", "label": "Buyer Power", "intensity": 0.7}
    ]
  }
}
```

#### Business Model Canvas (`business_model_canvas`)
Map out a complete business model in 9 sections.

```json
{
  "type": "business_model_canvas",
  "title": "SaaS Startup",
  "data": {
    "boxes": {
      "value_proposition": {"items": ["Cloud-native", "Pay-per-use", "Easy integration"]},
      "customer_segments": {"items": ["SMBs", "Enterprise", "Startups"]},
      "channels": {"items": ["Website", "Partners", "Direct sales"]},
      "key_partners": {"items": ["AWS", "Stripe", "Consultants"]},
      "key_activities": {"items": ["Development", "Support", "Sales"]},
      "key_resources": {"items": ["Engineering team", "IP", "Brand"]},
      "customer_relationships": {"items": ["Self-service", "Account managers"]},
      "cost_structure": {"items": ["Infrastructure", "Salaries", "Marketing"]},
      "revenue_streams": {"items": ["Subscriptions", "Usage fees", "Services"]}
    }
  }
}
```

#### Value Chain (`value_chain`)
Map Porter's Value Chain activities.

```json
{
  "type": "value_chain",
  "title": "Manufacturing Value Chain",
  "data": {
    "primary_activities": [
      {"label": "Inbound Logistics", "items": ["Warehousing", "Inventory"]},
      {"label": "Operations", "items": ["Assembly", "QC"]},
      {"label": "Outbound Logistics", "items": ["Distribution", "Shipping"]},
      {"label": "Marketing & Sales", "items": ["Advertising", "Sales team"]},
      {"label": "Service", "items": ["Support", "Warranties"]}
    ],
    "support_activities": [
      {"label": "Infrastructure", "items": ["Finance", "Legal"]},
      {"label": "HR", "items": ["Recruiting", "Training"]},
      {"label": "Technology", "items": ["R&D", "IT"]},
      {"label": "Procurement", "items": ["Sourcing", "Contracts"]}
    ]
  }
}
```

#### Nine Box Talent Grid (`nine_box_talent`)
HR performance/potential assessment.

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

#### House Diagram (`house_diagram`)
Radial relationship diagram (e.g., McKinsey 7S).

```json
{
  "type": "house_diagram",
  "title": "McKinsey 7S Framework",
  "data": {
    "center_element": {"label": "Shared Values"},
    "outer_elements": [
      {"label": "Strategy"},
      {"label": "Structure"},
      {"label": "Systems"},
      {"label": "Style"},
      {"label": "Staff"},
      {"label": "Skills"}
    ],
    "show_connectors": true
  }
}
```

## Styling Options

All diagrams support common styling:

```json
{
  "style": {
    "palette": "corporate",
    "font_family": "Arial",
    "background": "white",
    "show_legend": true,
    "show_values": true
  },
  "output": {
    "format": "svg",
    "width": 800,
    "height": 600
  }
}
```

### Built-in Palettes

- `corporate` - Professional blues and grays
- `vibrant` - Bright, saturated colors
- `muted` - Soft, pastel tones
- `monochrome` - Grayscale

### Custom Colors

```json
{
  "style": {
    "palette": ["#336699", "#993366", "#669933", "#996633"]
  }
}
```

## API Usage

### Go Package

```go
import "github.com/ahrens/svggen"

req := &svggen.RequestEnvelope{
    Type:  "bar_chart",
    Title: "Sales",
    Data:  map[string]any{...},
}

doc, err := svggen.Render(req)
svg := doc.String()
```

### HTTP API

```bash
curl -X POST http://localhost:3001/render \
  -H "Content-Type: application/json" \
  -d '{"type": "bar_chart", "data": {...}}'
```

## See Also

### Charts
- [Bar Chart](./bar_chart.md)
- [Grouped Bar Chart](./grouped_bar_chart.md)
- [Stacked Bar Chart](./stacked_bar_chart.md)
- [Line Chart](./line_chart.md)
- [Area Chart](./area_chart.md)
- [Stacked Area Chart](./stacked_area_chart.md)
- [Pie Chart](./pie_chart.md)
- [Donut Chart](./donut_chart.md)
- [Scatter Chart](./scatter_chart.md)
- [Bubble Chart](./bubble_chart.md)
- [Radar Chart](./radar_chart.md)
- [Waterfall Chart](./waterfall.md)
- [Funnel Chart](./funnel_chart.md)
- [Gauge Chart](./gauge_chart.md)
- [Treemap Chart](./treemap_chart.md)
- [Heatmap](./heatmap.md)

### Business & Strategy
- [2x2 Matrix](./matrix_2x2.md)
- [Porter's Five Forces](./porters_five_forces.md)
- [Business Model Canvas](./business_model_canvas.md)
- [SWOT Analysis](./swot.md)
- [PESTEL Analysis](./pestel.md)
- [Value Chain](./value_chain.md)
- [Pyramid](./pyramid.md)
- [Venn Diagram](./venn.md)
- [Nine Box Talent](./nine_box_talent.md)
- [House Diagram](./house.md)
- [Fishbone Diagram](./fishbone.md)

### Process & Organization
- [Process Flow](./process_flow.md)
- [Timeline](./timeline.md)
- [Gantt Chart](./gantt.md)
- [Org Chart](./org_chart.md)
- [KPI Dashboard](./kpi_dashboard.md)
- [Panel Layout](./panel_layout.md)
