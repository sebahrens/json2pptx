# SVG Generation Package (svggen)

Canvas-native SVG generation for charts and business diagrams.

## Scope

This specification covers the `internal/svggen` package which provides:
- SVG diagram rendering using tdewolff/canvas
- Registry-based diagram type system
- Multi-format output (SVG, PNG, PDF)
- Reusable drawing primitives

It does NOT cover:
- Chart data parsing (see `docs/INPUT_FORMAT.md`)
- Chart-to-svggen adapter (see `05-chart-renderer.md`)

## Purpose

Generate high-quality SVG diagrams with support for raster output. Used by the chart renderer and can be extended with custom diagram types.

## Architecture

```
RequestEnvelope
    │
    ▼
Registry.Render()
    │
    ├─▶ Diagram.Validate()
    │
    └─▶ Diagram.Render()
            │
            ▼
        SVGBuilder
            │
            ├─▶ RenderSVG() → SVGDocument
            ├─▶ RenderPNG() → []byte
            └─▶ RenderPDF() → []byte
```

## Core Types

### RequestEnvelope

The top-level input for diagram generation:

```go
type RequestEnvelope struct {
    Type     string         `json:"type"`     // Diagram type (e.g., "bar_chart")
    Title    string         `json:"title"`    // Optional title
    Subtitle string         `json:"subtitle"` // Optional subtitle
    Data     map[string]any `json:"data"`     // Diagram-specific payload
    Output   OutputSpec     `json:"output"`   // Format and dimensions
    Style    StyleSpec      `json:"style"`    // Theming options
}
```

### OutputSpec

Output format and dimensions:

```go
type OutputSpec struct {
    Format string  `json:"format"` // "svg" (default), "png", "pdf"
    Width  int     `json:"width"`  // Pixels/points (default: 800)
    Height int     `json:"height"` // Pixels/points (default: 600)
    Scale  float64 `json:"scale"`  // Raster scale factor (default: 2.0)
}
```

### StyleSpec

Theming and appearance:

```go
type StyleSpec struct {
    Palette    any    `json:"palette"`     // Color scheme name or hex array
    FontFamily string `json:"font_family"` // Font name (default: "Arial")
    Background string `json:"background"`  // Background color
    ShowLegend bool   `json:"show_legend"` // Display legend
    ShowValues bool   `json:"show_values"` // Display value labels
    ShowGrid   bool   `json:"show_grid"`   // Display grid lines
}
```

Built-in palettes: `"corporate"`, `"vibrant"`, `"muted"`, `"monochrome"`

### SVGDocument

Rendered SVG output:

```go
type SVGDocument struct {
    Content []byte  // Raw SVG XML
    Width   float64 // Width in points
    Height  float64 // Height in points
    ViewBox string  // SVG viewBox attribute
}
```

### RenderResult

Multi-format render output:

```go
type RenderResult struct {
    SVG    *SVGDocument // Always populated
    PNG    []byte       // If PNG requested
    PDF    []byte       // If PDF requested
    Format string       // Primary output format
    Width  int
    Height int
}
```

## Diagram Interface

All diagram types implement the `Diagram` interface:

```go
type Diagram interface {
    Type() string                              // Type identifier
    Render(req *RequestEnvelope) (*SVGDocument, error)
    Validate(req *RequestEnvelope) error
}
```

### Optional Interfaces

For multi-format support:

```go
// DiagramWithBuilder exposes the builder for format conversion
type DiagramWithBuilder interface {
    Diagram
    RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error)
}

// MultiFormatRenderer provides native multi-format support
type MultiFormatRenderer interface {
    Diagram
    RenderPNG(req *RequestEnvelope, scale float64) ([]byte, error)
    RenderPDF(req *RequestEnvelope) ([]byte, error)
}
```

## Registry

The registry manages diagram type registrations:

```go
// Register a diagram type
svggen.Register(myDiagram)

// Render a request
doc, err := svggen.Render(req)

// Multi-format render
result, err := svggen.RenderMultiFormat(req, "png", "pdf")

// List registered types
types := svggen.Types()
```

### Built-in Diagram Types (svggen registry)

These types are registered in `svggen/init.go` and rendered as SVG:

| Type | Description |
|------|-------------|
| `bar_chart` | Vertical bar chart |
| `line_chart` | Line chart with markers |
| `pie_chart` | Pie chart with segments |
| `donut_chart` | Donut chart with center hole |
| `area_chart` | Area chart (filled line) |
| `radar_chart` | Radar/spider chart |
| `scatter_chart` | Scatter plot |
| `stacked_bar_chart` | Stacked bar chart |
| `bubble_chart` | Bubble chart (scatter + size) |
| `stacked_area_chart` | Stacked area chart |
| `grouped_bar_chart` | Grouped bar chart |
| `waterfall` | Waterfall/bridge chart |
| `funnel_chart` | Funnel diagram |
| `gauge_chart` | Gauge/meter diagram |
| `treemap_chart` | Hierarchical treemap |
| `matrix_2x2` | 2x2 matrix diagram |
| `timeline` | Timeline diagram |
| `venn` | Venn diagram |
| `org_chart` | Organization chart |
| `gantt` | Gantt chart |
| `fishbone` | Fishbone/Ishikawa diagram |

### Aliases

Short names are accepted and resolved to canonical IDs (see `builtinAliases` in `init.go`):

`funnel` -> `funnel_chart`, `gauge` -> `gauge_chart`, `treemap` -> `treemap_chart`, `bar` -> `bar_chart`, `line` -> `line_chart`, `pie` -> `pie_chart`, `donut` -> `donut_chart`, `area` -> `area_chart`, `radar` -> `radar_chart`, `scatter` -> `scatter_chart`, `bubble` -> `bubble_chart`, `stacked_bar` -> `stacked_bar_chart`, `stacked_area` -> `stacked_area_chart`, `grouped_bar` -> `grouped_bar_chart`, `orgchart`/`org` -> `org_chart`, `matrix` -> `matrix_2x2`.

### Native OOXML Diagram Types (not in svggen)

These diagram types bypass svggen and are rendered as native OOXML shapes in `internal/generator/*_shapes.go`:

| Type | File | Description |
|------|------|-------------|
| `swot` | `swot_shapes.go` | SWOT analysis 2x2 grid |
| `pyramid` | `pyramid_shapes.go` | Layered pyramid |
| `process_flow` | `process_flow_shapes.go` | Step-by-step process flow |
| `porters_five_forces` | `porters_shapes.go` | Porter's Five Forces |
| `house_diagram` | `house_shapes.go` | House/temple diagram |
| `business_model_canvas` | `bmc_shapes.go` | Business Model Canvas |
| `value_chain` | `value_chain_shapes.go` | Value chain diagram |
| `nine_box_talent` | `nine_box_shapes.go` | 9-Box talent grid |
| `kpi_dashboard` | `kpi_dashboard_shapes.go` | KPI metric cards |
| `heatmap` | `heatmap_shapes.go` | Color-coded heatmap |
| `pestel` | `pestel_shapes.go` | PESTEL analysis |
| `panel_layout` | `panel_shapes.go` | Flexible panel layout (columns, rows, stat_cards, grid) |

## SVGBuilder

Low-level drawing API built on tdewolff/canvas:

```go
builder := svggen.NewSVGBuilder(width, height)

// Drawing primitives
builder.Rect(x, y, w, h, fill, stroke)
builder.Circle(cx, cy, r, fill, stroke)
builder.Line(x1, y1, x2, y2, stroke, strokeWidth)
builder.Text(x, y, text, size, color, anchor)
builder.Path(path, fill, stroke)

// Multi-format output
svgDoc := builder.RenderSVG()
pngBytes, _ := builder.RenderPNG(2.0)
pdfBytes, _ := builder.RenderPDF()
```

## Drawing Primitives

### Scales

Map data values to pixel coordinates:

```go
// Linear scale for numeric data
scale := svggen.NewLinearScale(0, 100, 0, 400)
y := scale.Scale(50) // → 200

// Categorical scale for labels
catScale := svggen.NewCategoricalScale([]string{"A", "B", "C"}, 0, 300)
x := catScale.Scale("B") // → 150
```

### Series

High-level data visualization:

```go
// Bar series
svggen.DrawBarSeries(builder, rect, data, scale, colors)

// Line series
svggen.DrawLineSeries(builder, rect, data, xScale, yScale, color)

// Arc series (pie/donut)
svggen.DrawArcSeries(builder, center, radius, data, colors)
```

### Layout Components

```go
// Title and subtitle
svggen.DrawTitle(builder, title, subtitle, bounds, style)

// Legend
svggen.DrawLegend(builder, items, bounds, style)

// Axes
svggen.DrawLinearAxis(builder, scale, bounds, config)
svggen.DrawCategoricalAxis(builder, scale, bounds, config)
```

## Extending svggen

### Adding a New Diagram Type

1. Create a struct implementing `Diagram`:

```go
type MyDiagram struct{}

func (d *MyDiagram) Type() string { return "my_diagram" }

func (d *MyDiagram) Validate(req *RequestEnvelope) error {
    // Validate req.Data fields
    return nil
}

func (d *MyDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
    builder := NewSVGBuilder(req.Output.Width, req.Output.Height)
    // Draw diagram using builder
    return builder.RenderSVG(), nil
}
```

2. Register in `init.go`:

```go
func RegisterMyDiagram() {
    Register(&MyDiagram{})
}
```

3. Call registration in `init()`:

```go
func init() {
    // ... existing registrations ...
    RegisterMyDiagram()
}
```

### Multi-Format Support

Implement `DiagramWithBuilder` to enable PNG/PDF output:

```go
func (d *MyDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
    builder := NewSVGBuilder(req.Output.Width, req.Output.Height)
    // Draw diagram
    return builder, builder.RenderSVG(), nil
}
```

## Typography Scaling

### Overview

Font sizes must be calibrated for the full rendering pipeline: SVG canvas → PNG raster → PPTX placeholder. The target is professional-grade presentation typography where chart titles render at 14-16pt and axis labels at 10-12pt in the final PPTX.

### Typography Tiers

```go
// DefaultTypography: optimized for standard charts (bar, line, pie, scatter)
// Reference canvas: 800x600. Base sizes: Title=40, Body=22, Small=20, Caption=18

// CompactTypography: optimized for dense diagrams (SWOT, Gantt, KPI, BMC, PESTEL, Timeline)
// Base sizes: Title=24, Body=14, Small=14, Caption=12
```

Dense diagram types MUST use `CompactTypography` instead of `DefaultTypography`. The following types are classified as dense:
- SWOT, Business Model Canvas, PESTEL (multi-section text layouts)
- Gantt (dense swimlane labels)
- KPI Dashboard (grid of metric cards)
- Timeline (date labels with potential overlap)

### Scaling Formula

`ScaleForDimensions(width, height)` adjusts font sizes for the output canvas:

```
scale = sqrt((width / ReferenceWidth) * (height / ReferenceHeight))
```

Clamped to [0.5, 5.0]. Applied uniformly to all 6 size tiers.

### Size Bounds

Both minimum floors AND maximum caps are enforced after scaling:

| Tier | Min Floor | Max Cap | Purpose |
|------|-----------|---------|---------|
| Title | 28 | configurable | Chart titles |
| Subtitle | 22 | configurable | Subtitles |
| Heading | 20 | configurable | Legend text, section headers |
| Body | 18 | configurable | Descriptions, pie labels |
| Small | 16 | configurable | Axis labels, data labels |
| Caption | 14 | configurable | Footnotes, badges |

Max caps prevent oversized text on large canvases (e.g., 1600x900 produces scale=1.73 without caps).

### PPTX Pipeline Math

The relationship between SVG font sizes and final PPTX point sizes:

```
scaleFactor = (placeholderWidthInches * 72) / (canvasWidthPx * dpiScale)
pptxPt = svgFontSize * scaleFactor
svgFontSize = targetPptxPt / scaleFactor
```

For full-width 16:9 (1600px canvas, 8" placeholder, 2x DPI): scaleFactor ≈ 0.18

### Future: Preset-Based Typography

The current geometric mean scaling will be replaced with layout preset-based typography tables, where each `LayoutPreset` (full-width, half-width, third-width) maps to hand-tuned font sizes derived from PPTX point targets. This provides cross-slide consistency — all charts at the same layout size use identical typography.

### Future: Element Clamping

Individual text elements (SWOT quadrant bullets, funnel tier labels, KPI hero numbers) will be able to shrink below the preset size when they exceed their container bounds. The preset size is the ceiling; `CompactTypography` sizes are the floor. This replaces ad-hoc `truncateText()` calls with principled font size reduction.

## Performance

- Font loading is cached to avoid repeated parsing
- PNG rasterization is serialized due to canvas library race condition
- Target: <100ms per diagram render

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Unknown diagram type | Error: "unknown diagram type" |
| Missing required data | Error from `Validate()` |
| Invalid output format | Fallback to SVG |
| Font not found | Fallback to system font |

## Testing

Each diagram type should have:
- Unit tests for `Validate()`
- Golden tests comparing output to reference SVGs
- Tests for edge cases (empty data, single item, max items)

Run tests:
```bash
go test ./internal/svggen/... -v
```

Golden test updates:
```bash
UPDATE_GOLDEN=1 go test ./internal/svggen/... -v
```

## Package Split: svggen vs svggen/core

The `svggen` package is split into two import paths:

| Package | Purpose |
|---------|---------|
| `svggen/core` | Foundational types, interfaces, registry, and validation — no diagram implementations linked |
| `svggen` | Full package with all built-in diagrams auto-registered via `init()` |

Import `svggen/core` for lightweight access to `RequestEnvelope`, `Diagram`, `SVGDocument` types and the registry API without pulling in all diagram implementations. Import the parent `svggen` package when you need all built-in diagrams available.

## Related Specifications

- `05-chart-renderer.md` - Chart package that uses svggen
- `16-chart-backend-decision.md` - Design decision choosing svggen
