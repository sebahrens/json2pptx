# Chart Backend Strategy Decision

Decision document for selecting the chart implementation approach for the SVG generation API.

## Decision

**Strategy: Canvas-native implementation using tdewolff/canvas**

For chart rendering in the SVG generation API, we will implement charts directly using canvas drawing primitives rather than adapting an existing charting library.

## Options Evaluated

### Option A: Canvas-native (Selected)

**How it works**: Build chart components (axes, scales, series, legends) using tdewolff/canvas drawing primitives (paths, text, shapes). Output directly to SVG via canvas renderers.

**Pros**:
- Direct SVG output, deterministic and predictable
- No external runtime dependencies (pure Go)
- Full control over rendering and styling
- Integrates with existing StyleGuide architecture
- Consistent with consulting diagram renderers (same primitives)
- Server-side generation without browser

**Cons**:
- More implementation work for chart primitives
- Need to implement scales, axes, legends, tooltips manually
- No pre-built chart types

**Decision**: **Selected** as the primary approach.

### Option B: go-echarts Adapter (Rejected)

**How it works**: Use go-echarts to generate chart definitions, render via ECharts JavaScript, extract SVG from rendered output.

**Pros**:
- 25+ chart types available out of box
- Polished, feature-rich visualizations
- Interactive features (though not needed for PPTX)

**Cons**:
- Outputs HTML+JavaScript, not pure SVG
- Requires headless browser for rendering (Chromium/Puppeteer)
- Non-deterministic (JavaScript execution, timing)
- Heavy runtime dependency
- Complex SVG post-processing needed
- Not suitable for server-side batch generation

**Decision**: Rejected due to architecture mismatch.

### Option C: gonum/plot Integration (Considered for Future)

**How it works**: Use gonum/plot for statistical charts, render via canvas integration.

**Pros**:
- Statistical chart types (scatter, histogram, box plot)
- Canvas integration exists in tdewolff/canvas examples

**Cons**:
- Limited chart types for business visualization
- Separate styling system to integrate

**Decision**: Considered as supplementary option for statistical charts if needed.

## Implementation Plan

### Phase 1: Core Primitives ✅ Complete

Build reusable chart components using canvas:

1. **Scales module** ✅: Linear, categorical scales with tick generation (see `scales.go`)
2. **Axes module** ✅: X/Y axes with labels, gridlines, titles (see `axes.go`)
3. **Series primitives** ✅: Bars, lines, points, areas, arcs (see `series.go`)
4. **Layout module** ✅: Legend, title, margins, padding, footnotes (see `legend.go`, `layout.go`)

### Phase 2: Standard Charts ✅ Complete

Implement common business charts:

1. Bar chart ✅ (vertical, horizontal, stacked, grouped)
2. Line chart ✅ (with/without points, multiple series)
3. Area chart ✅ (stacked, percentage)
4. Pie/donut chart ✅ (with labels, callouts)
5. Scatter plot ✅ (see `charts.go`)

### Phase 3: Extended Charts (Priority: High)

Additional chart types - **prioritized for near-term implementation**:

1. Radar/spider chart ✅ (implemented in `charts.go`)
2. Waterfall/bridge chart ✅ (implemented in `waterfall.go`)
3. Time scale support 📋 (planned - for date/time axes on line/area charts)
4. Funnel chart 📋 (planned - prioritized)
5. Gauge chart 📋 (planned - prioritized)
6. Treemap 📋 (planned - prioritized)

### Bonus: Consulting Diagrams ✅

Additional strategic/consulting diagrams implemented beyond original scope:

- Business Model Canvas (`business_model_canvas.go`)
- 2x2 Matrix (`matrix2x2.go`)
- 9-Box Talent Grid (`nine_box_talent.go`)
- Porter's Five Forces (`porters_five_forces.go`)
- Process Flow (`process_flow.go`)
- Timeline (`timeline.go`)
- Value Chain (`value_chain.go`)
- House Diagram (`house_diagram.go`)

## Architecture

```
RequestEnvelope → Registry → ChartDiagram → SVGBuilder → SVGDocument
                                   ↓
                            [Scale, Axis, Series, Legend]
                                   ↓
                            canvas.Context primitives
                                   ↓
                            svg.Renderer output
```

### Component Responsibilities

| Component | Purpose |
|-----------|---------|
| Scale | Maps data values to pixel coordinates |
| Axis | Draws axis line, ticks, labels, title |
| Series | Renders data points (bars, lines, etc.) |
| Legend | Displays series labels with color keys |
| ChartDiagram | Orchestrates layout and rendering |

## Style Integration

Charts will use the same StyleGuide as consulting diagrams:

- Color palette from theme
- Typography settings (font, sizes)
- Spacing and margins
- Stroke widths and styles

This ensures visual consistency between charts and diagrams.

## Testing Strategy

1. **Unit tests**: Individual components (scales, axes, series)
2. **Golden tests**: Compare rendered SVG against known-good outputs
3. **Visual tests**: Side-by-side comparison with reference images
4. **Integration tests**: Full chart rendering pipeline

## Acceptance Criteria

- [x] Decision documented with rationale
- [x] Scales module implemented (linear, categorical) - `scales.go`
- [x] Axes module implemented with styling - `axes.go`
- [x] Series primitives implemented (bar, line, point, arc) - `series.go`
- [x] At least one chart type fully working (bar chart) - `charts.go`
- [x] Golden tests passing for basic charts - `golden_test.go`, test files in `testdata/`
- [x] Multiple chart types implemented (bar, line, area, scatter, pie, donut, radar)
- [x] Consulting diagrams implemented beyond original scope

## References

- [tdewolff/canvas](https://github.com/tdewolff/canvas) - Vector graphics library
- [go-echarts](https://github.com/go-echarts/go-echarts) - ECharts bindings (not selected)
- [gonum/plot](https://github.com/gonum/plot) - Scientific plotting (future consideration)
