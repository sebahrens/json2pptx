# Chart Renderer

Render data visualizations as images for PPTX embedding.

## Scope

This specification covers ONLY chart rendering to images. It does NOT cover:
- Chart data parsing (handled by JSON input, see `docs/INPUT_FORMAT.md`)
- PPTX embedding (handled by `04-pptx-generator.md`)

## Purpose

Convert chart specifications into PNG images suitable for PowerPoint embedding.

## Critical Design Decision: Image Output

This renderer produces **PNG images**, not SVG or embedded Excel charts.

Rationale:
- SVG support in PowerPoint is inconsistent across versions
- SVG-to-EMF conversion requires external tools (Inkscape, LibreOffice)
- Embedded Excel charts require complex OOXML manipulation
- PNG images work universally and render predictably

Trade-off: Charts are not editable in PowerPoint. This is acceptable for automated generation.

## Input

```go
type ChartSpec struct {
    Type   ChartType          // bar, line, pie, donut
    Title  string             // Chart title
    Data   map[string]float64 // Label -> Value pairs
    Width  int                // Pixels (default: 800)
    Height int                // Pixels (default: 600)
    Style  *ChartStyle        // Optional styling
}

type ChartType string
const (
    ChartBar   ChartType = "bar"
    ChartLine  ChartType = "line"
    ChartPie   ChartType = "pie"
    ChartDonut ChartType = "donut"
)

type ChartStyle struct {
    Colors      []string // Hex colors for data series
    FontFamily  string   // Font for labels
    FontSize    int      // Label font size
    Background  string   // Background color (default: transparent)
    ShowLegend  bool     // Display legend
    ShowValues  bool     // Display values on chart
}
```

## Output

```go
type ChartResult struct {
    ImageData   []byte    // PNG image bytes
    Width       int       // Actual rendered width
    Height      int       // Actual rendered height
    ContentType string    // "image/png"
}
```

## Supported Chart Types

### Bar Chart
- Vertical bars
- One bar per data entry
- Y-axis shows values
- X-axis shows labels

### Line Chart
- Connected data points
- Single series support
- Points marked with circles

### Pie Chart
- Circular chart with segments
- Segments proportional to values
- Labels outside segments

### Donut Chart
- Pie chart with center hole
- Center can display total or title

## Rendering Implementation

Use Go's `image` package with a charting library.

Recommended approach:
1. **github.com/wcharczuk/go-chart** - Pure Go, produces PNG directly
2. Falls back to basic shapes if library unavailable

### go-chart Integration

```go
import "github.com/wcharczuk/go-chart/v2"

func RenderBar(spec ChartSpec) ([]byte, error) {
    bars := []chart.Value{}
    for label, value := range spec.Data {
        bars = append(bars, chart.Value{
            Label: label,
            Value: value,
        })
    }

    graph := chart.BarChart{
        Title:  spec.Title,
        Width:  spec.Width,
        Height: spec.Height,
        Bars:   bars,
    }

    buffer := bytes.Buffer{}
    err := graph.Render(chart.PNG, &buffer)
    return buffer.Bytes(), err
}
```

## ChartStyle Reference

The `ChartStyle` struct provides fine-grained control over chart appearance. All fields are optional.

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Colors` | `[]string` | DefaultChartColors | Hex color array for data series. First color used for first series, etc. |
| `ThemeColors` | `[]ThemeColor` | (internal) | Theme colors extracted from PPTX template. Internal use. |
| `FontFamily` | `string` | `"Arial"` | Font for labels. Inherits from template when using style templates. |
| `FontSize` | `int` | `12` | Font size in points for labels. |
| `Background` | `string` | `"transparent"` | Background color as hex (e.g., `"#FFFFFF"`) or `"transparent"`. |
| `ShowLegend` | `bool` | `false` | Display chart legend with series labels and colors. |
| `ShowValues` | `bool` | `false` | Display numeric values on chart elements. |

### Color Priority

Colors are resolved in the following priority order:

1. **Style.Colors** - Explicit colors in ChartStyle
2. **Style.ThemeColors** - Theme colors from PPTX template
3. **DefaultChartColors** - Built-in palette

## Theme Integration

The chart package includes a style template system for theme-aligned styling.

### Style Templates

Predefined templates that map to PPTX theme colors:

| Template | Description | Show Legend | Show Values |
|----------|-------------|-------------|-------------|
| corporate | Professional, accent colors in order | true | false |
| vibrant | High-contrast for emphasis | true | true |
| subtle | Muted colors for supporting data | false | false |
| monochrome | Single-hue using accent1 variations | true | false |
| dark | Inverted colors for dark backgrounds | true | false |

### Theme Application Functions

```go
// Get a template by name (defaults to corporate)
func GetStyleTemplate(name string) ChartStyleTemplate

// Create ChartStyle from template + theme
func ApplyThemeToChartStyle(template ChartStyleTemplate, theme types.ThemeInfo) *types.ChartStyle

// List available templates
func ListStyleTemplates() []string
```

### Theme Color Extraction

Theme colors are extracted from PPTX templates during template analysis:

1. **Color Names**: `accent1`-`accent6`, `dk1`, `dk2`, `lt1`, `lt2`
2. **Source**: `ppt/theme/themeN.xml` in the PPTX archive
3. **Font Inheritance**: When `UseThemeFont=true`, the template's body font is used

### Default Color Palette

When no theme or style is provided:
```go
var DefaultChartColors = []string{
    "#4E79A7", // Blue
    "#F28E2B", // Orange
    "#E15759", // Red
    "#76B7B2", // Teal
    "#59A14F", // Green
    "#EDC948", // Yellow
    "#B07AA1", // Purple
    "#FF9DA7", // Pink
}
```

These colors are from the Tableau 10 palette, designed for visual distinction.

## Acceptance Criteria

### AC1: Bar Chart Rendering
- Given ChartSpec with type "bar" and 4 data points
- When rendered
- Then produces valid PNG showing 4 bars

### AC2: Line Chart Rendering
- Given ChartSpec with type "line" and 6 data points
- When rendered
- Then produces PNG with connected line through 6 points

### AC3: Pie Chart Rendering
- Given ChartSpec with type "pie" and 3 data points
- When rendered
- Then produces PNG with 3 proportional segments

### AC4: Donut Chart Rendering
- Given ChartSpec with type "donut"
- When rendered
- Then produces pie chart with center hole

### AC5: Custom Dimensions
- Given ChartSpec with Width=1200, Height=800
- When rendered
- Then output image has those exact dimensions

### AC6: Title Display
- Given ChartSpec with Title="Sales Data"
- When rendered
- Then chart displays title

### AC7: Theme Colors
- Given ChartStyle with custom Colors array
- When rendered
- Then chart uses those colors

### AC8: Default Colors
- Given ChartSpec without style
- When rendered
- Then uses default color palette

### AC9: Legend Display
- Given ChartStyle with ShowLegend=true
- When rendered
- Then legend appears on chart

### AC10: Value Labels
- Given ChartStyle with ShowValues=true
- When rendered
- Then values appear on/near chart elements

### AC11: Empty Data
- Given ChartSpec with empty Data map
- When rendered
- Then returns error (cannot render empty chart)

### AC12: Single Data Point
- Given ChartSpec with single data entry
- When rendered
- Then produces valid chart (single bar, full pie, etc.)

### AC13: Negative Values
- Given bar chart with negative values
- When rendered
- Then bars extend below axis appropriately

### AC14: Large Values
- Given data with values > 1 million
- When rendered
- Then axis labels use appropriate formatting (1M, 2M)

### AC15: Long Labels
- Given data with labels > 20 characters
- When rendered
- Then labels are truncated or wrapped (not overlapping)

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Unsupported chart type | Error |
| Empty data | Error |
| Invalid dimensions (< 100px) | Error |
| Invalid color format | Warning, use default |
| Too many data points (>50) | Warning, render anyway |

## Performance

- Target: < 100ms per chart render
- Charts should be rendered in parallel when multiple exist
- Consider caching for identical specs

## Testing Requirements

Test each chart type with:
- Minimal data (2 points)
- Typical data (5-10 points)
- Maximum recommended data (20 points)
- Edge case data (negative, zero, very large)

Visual verification:
- Compare rendered output to reference images
- Check text legibility at standard sizes
- Verify color accuracy

Fixtures needed:
- Reference PNG outputs for each chart type
- Test data sets with known visual output
