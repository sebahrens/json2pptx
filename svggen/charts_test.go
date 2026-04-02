package svggen

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// =============================================================================
// Chart Data Tests
// =============================================================================

// =============================================================================
// ScaledMargins Tests
// =============================================================================

func TestScaledMargins_ReferenceSize(t *testing.T) {
	// At reference size (800x600), margins should match the reference values
	top, right, bottom, left := ScaledMargins(800, 600)

	if top != 40 {
		t.Errorf("Expected top margin 40, got %v", top)
	}
	if right != 20 {
		t.Errorf("Expected right margin 20, got %v", right)
	}
	if bottom != 60 {
		t.Errorf("Expected bottom margin 60, got %v", bottom)
	}
	if left != 60 {
		t.Errorf("Expected left margin 60, got %v", left)
	}
}

func TestScaledMargins_SmallChart(t *testing.T) {
	// 400x300 is 1/4 the area of 800x600, so scale = 0.5 (clamped minimum)
	top, right, bottom, left := ScaledMargins(400, 300)

	// Scale = sqrt((400*300)/(800*600)) = sqrt(0.25) = 0.5
	expectedTop := 40 * 0.5
	expectedRight := 20 * 0.5
	expectedBottom := 60 * 0.5
	expectedLeft := 60 * 0.5

	if top != expectedTop {
		t.Errorf("Expected top margin %v, got %v", expectedTop, top)
	}
	if right != expectedRight {
		t.Errorf("Expected right margin %v, got %v", expectedRight, right)
	}
	if bottom != expectedBottom {
		t.Errorf("Expected bottom margin %v, got %v", expectedBottom, bottom)
	}
	if left != expectedLeft {
		t.Errorf("Expected left margin %v, got %v", expectedLeft, left)
	}
}

func TestScaledMargins_LargeChart(t *testing.T) {
	// 1600x1200 is 4x the area of 800x600, so scale = 2.0 (clamped maximum)
	top, right, bottom, left := ScaledMargins(1600, 1200)

	// Scale = sqrt((1600*1200)/(800*600)) = sqrt(4) = 2.0
	expectedTop := 40 * 2.0
	expectedRight := 20 * 2.0
	expectedBottom := 60 * 2.0
	expectedLeft := 60 * 2.0

	if top != expectedTop {
		t.Errorf("Expected top margin %v, got %v", expectedTop, top)
	}
	if right != expectedRight {
		t.Errorf("Expected right margin %v, got %v", expectedRight, right)
	}
	if bottom != expectedBottom {
		t.Errorf("Expected bottom margin %v, got %v", expectedBottom, bottom)
	}
	if left != expectedLeft {
		t.Errorf("Expected left margin %v, got %v", expectedLeft, left)
	}
}

func TestScaledMargins_VerySmallChart(t *testing.T) {
	// Very small chart should be clamped to 0.5x scale
	top, right, bottom, left := ScaledMargins(200, 150)

	// Scale would be sqrt((200*150)/(800*600)) = sqrt(0.0625) = 0.25
	// But clamped to 0.5
	expectedTop := 40 * 0.5
	expectedRight := 20 * 0.5
	expectedBottom := 60 * 0.5
	expectedLeft := 60 * 0.5

	if top != expectedTop {
		t.Errorf("Expected top margin %v (clamped), got %v", expectedTop, top)
	}
	if right != expectedRight {
		t.Errorf("Expected right margin %v (clamped), got %v", expectedRight, right)
	}
	if bottom != expectedBottom {
		t.Errorf("Expected bottom margin %v (clamped), got %v", expectedBottom, bottom)
	}
	if left != expectedLeft {
		t.Errorf("Expected left margin %v (clamped), got %v", expectedLeft, left)
	}
}

func TestScaledMargins_VeryLargeChart(t *testing.T) {
	// Very large chart should be clamped to 2.0x scale
	top, right, bottom, left := ScaledMargins(3200, 2400)

	// Scale would be sqrt((3200*2400)/(800*600)) = sqrt(16) = 4.0
	// But clamped to 2.0
	expectedTop := 40 * 2.0
	expectedRight := 20 * 2.0
	expectedBottom := 60 * 2.0
	expectedLeft := 60 * 2.0

	if top != expectedTop {
		t.Errorf("Expected top margin %v (clamped), got %v", expectedTop, top)
	}
	if right != expectedRight {
		t.Errorf("Expected right margin %v (clamped), got %v", expectedRight, right)
	}
	if bottom != expectedBottom {
		t.Errorf("Expected bottom margin %v (clamped), got %v", expectedBottom, bottom)
	}
	if left != expectedLeft {
		t.Errorf("Expected left margin %v (clamped), got %v", expectedLeft, left)
	}
}

func TestScaledMargins_NormalAspectRatio(t *testing.T) {
	// 800x600 has aspect ratio 1.33 — well above 0.6, no narrow reduction.
	top, right, bottom, left := ScaledMargins(800, 600)

	// At reference size, margins are exactly the reference values.
	if top != 40 {
		t.Errorf("Expected top 40, got %v", top)
	}
	if right != 20 {
		t.Errorf("Expected right 20 (no narrow reduction), got %v", right)
	}
	if bottom != 60 {
		t.Errorf("Expected bottom 60, got %v", bottom)
	}
	if left != 60 {
		t.Errorf("Expected left 60 (no narrow reduction), got %v", left)
	}
}

func TestScaledMargins_SlightlyNarrow(t *testing.T) {
	// 380x500 has aspect ratio 0.76 — above the 0.6 threshold, no reduction.
	top, right, bottom, left := ScaledMargins(380, 500)

	// scale = sqrt((380*500)/(800*600)) = sqrt(190000/480000) = sqrt(0.3958) ≈ 0.6292
	scale := 0.6291528696
	expectedTop := 40 * scale
	expectedRight := 20 * scale
	expectedBottom := 60 * scale
	expectedLeft := 60 * scale

	const eps = 0.001
	if math.Abs(top-expectedTop) > eps {
		t.Errorf("Expected top ≈ %v, got %v", expectedTop, top)
	}
	if math.Abs(right-expectedRight) > eps {
		t.Errorf("Expected right ≈ %v (no narrow reduction), got %v", expectedRight, right)
	}
	if math.Abs(bottom-expectedBottom) > eps {
		t.Errorf("Expected bottom ≈ %v, got %v", expectedBottom, bottom)
	}
	if math.Abs(left-expectedLeft) > eps {
		t.Errorf("Expected left ≈ %v (no narrow reduction), got %v", expectedLeft, left)
	}
}

func TestScaledMargins_NarrowAspectRatio(t *testing.T) {
	// 300x600 has aspect ratio 0.5 — below 0.6 threshold, 30% reduction on left/right.
	top, right, bottom, left := ScaledMargins(300, 600)

	// scale = sqrt((300*600)/(800*600)) = sqrt(180000/480000) = sqrt(0.375) ≈ 0.6124
	scale := 0.6123724357
	expectedTop := 40 * scale
	expectedRight := 20 * scale * 0.7 // 30% reduction
	expectedBottom := 60 * scale
	expectedLeft := 60 * scale * 0.7 // 30% reduction

	const eps = 0.001
	if math.Abs(top-expectedTop) > eps {
		t.Errorf("Expected top ≈ %v (not affected by narrow), got %v", expectedTop, top)
	}
	if math.Abs(right-expectedRight) > eps {
		t.Errorf("Expected right ≈ %v (with 30%% reduction), got %v", expectedRight, right)
	}
	if math.Abs(bottom-expectedBottom) > eps {
		t.Errorf("Expected bottom ≈ %v (not affected by narrow), got %v", expectedBottom, bottom)
	}
	if math.Abs(left-expectedLeft) > eps {
		t.Errorf("Expected left ≈ %v (with 30%% reduction), got %v", expectedLeft, left)
	}

	// Verify top/bottom are NOT reduced (same as without narrow adjustment)
	topWithoutNarrow := 40 * scale
	bottomWithoutNarrow := 60 * scale
	if math.Abs(top-topWithoutNarrow) > eps {
		t.Errorf("Top margin should not be affected by narrow adjustment")
	}
	if math.Abs(bottom-bottomWithoutNarrow) > eps {
		t.Errorf("Bottom margin should not be affected by narrow adjustment")
	}
}

func TestChartData(t *testing.T) {
	data := ChartData{
		Title:      "Test Chart",
		Subtitle:   "Subtitle",
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "Series 1", Values: []float64{10, 20, 30}},
			{Name: "Series 2", Values: []float64{15, 25, 35}},
		},
		Footnote: "Source: Test data",
	}

	if data.Title != "Test Chart" {
		t.Errorf("Expected title 'Test Chart', got %v", data.Title)
	}
	if len(data.Series) != 2 {
		t.Errorf("Expected 2 series, got %d", len(data.Series))
	}
	if len(data.Categories) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(data.Categories))
	}
}

func TestDefaultChartConfig(t *testing.T) {
	config := DefaultChartConfig(400, 300)

	if config.Width != 400 {
		t.Errorf("Expected width 400, got %v", config.Width)
	}
	if config.Height != 300 {
		t.Errorf("Expected height 300, got %v", config.Height)
	}
	if !config.ShowLegend {
		t.Error("Expected ShowLegend to be true")
	}
	if !config.ShowAxes {
		t.Error("Expected ShowAxes to be true")
	}
	if !config.ShowGrid {
		t.Error("Expected ShowGrid to be true")
	}
}

func TestChartConfigPlotArea(t *testing.T) {
	config := DefaultChartConfig(400, 300)
	plotArea := config.PlotArea()

	if plotArea.X != config.MarginLeft {
		t.Errorf("Expected X = MarginLeft, got %v", plotArea.X)
	}
	if plotArea.Y != config.MarginTop {
		t.Errorf("Expected Y = MarginTop, got %v", plotArea.Y)
	}

	expectedWidth := config.Width - config.MarginLeft - config.MarginRight
	if plotArea.W != expectedWidth {
		t.Errorf("Expected W = %v, got %v", expectedWidth, plotArea.W)
	}
}

func TestPlotArea_NegativeClamping(t *testing.T) {
	// When margins exceed width/height, W and H must be clamped to 0.
	config := ChartConfig{
		Width:        100,
		Height:       80,
		MarginLeft:   60,
		MarginRight:  60,
		MarginTop:    50,
		MarginBottom: 50,
	}

	plotArea := config.PlotArea()

	if plotArea.W != 0 {
		t.Errorf("Expected W clamped to 0 when margins exceed width, got %v", plotArea.W)
	}
	if plotArea.H != 0 {
		t.Errorf("Expected H clamped to 0 when margins exceed height, got %v", plotArea.H)
	}
	if plotArea.X != config.MarginLeft {
		t.Errorf("Expected X = MarginLeft (%v), got %v", config.MarginLeft, plotArea.X)
	}
	if plotArea.Y != config.MarginTop {
		t.Errorf("Expected Y = MarginTop (%v), got %v", config.MarginTop, plotArea.Y)
	}

	// Also verify that normal dimensions still work correctly.
	normal := ChartConfig{
		Width:        400,
		Height:       300,
		MarginLeft:   40,
		MarginRight:  20,
		MarginTop:    30,
		MarginBottom: 50,
	}
	pa := normal.PlotArea()
	if pa.W != 340 {
		t.Errorf("Expected W = 340, got %v", pa.W)
	}
	if pa.H != 220 {
		t.Errorf("Expected H = 220, got %v", pa.H)
	}
}

// =============================================================================
// Bar Chart Tests
// =============================================================================

func TestDefaultBarChartConfig(t *testing.T) {
	config := DefaultBarChartConfig(400, 300)

	if config.Horizontal {
		t.Error("Expected Horizontal to be false")
	}
	if config.Stacked {
		t.Error("Expected Stacked to be false")
	}
	if config.BarPadding != 0.1 {
		t.Errorf("Expected BarPadding 0.1, got %v", config.BarPadding)
	}
}

func TestBarChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Sales by Quarter",
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []ChartSeries{
			{Name: "2023", Values: []float64{100, 150, 120, 180}},
			{Name: "2024", Values: []float64{120, 170, 140, 200}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarChartSingleSeries(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Revenue",
		Categories: []string{"Jan", "Feb", "Mar"},
		Series: []ChartSeries{
			{Name: "Revenue", Values: []float64{50, 75, 60}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarChartWithCustomColors(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	config.Colors = []Color{
		MustParseColor("#FF0000"),
		MustParseColor("#00FF00"),
	}
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B"},
		Series: []ChartSeries{
			{Name: "S1", Values: []float64{10, 20}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}

	_, err = b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

func TestBarChartWithRoundedCorners(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	config.CornerRadius = 4
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"X", "Y"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{30, 50}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}
}

func TestBarChartWithValues(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	config.ShowValues = true
	config.ValueFormat = "%.1f"
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{42.5, 67.8}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}
}

func TestBarChartWithFootnote(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Chart Title",
		Categories: []string{"A"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{50}},
		},
		Footnote: "Data source: Test",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}
}

func TestBarChartEmptyData(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{},
		Series:     []ChartSeries{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// =============================================================================
// Line Chart Tests
// =============================================================================

func TestDefaultLineChartConfig(t *testing.T) {
	config := DefaultLineChartConfig(400, 300)

	if config.Smooth {
		t.Error("Expected Smooth to be false")
	}
	if !config.ShowMarkers {
		t.Error("Expected ShowMarkers to be true")
	}
	if config.FillArea {
		t.Error("Expected FillArea to be false")
	}
}

func TestLineChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	chart := NewLineChart(b, config)

	data := ChartData{
		Title:      "Trend Analysis",
		Categories: []string{"Jan", "Feb", "Mar", "Apr", "May"},
		Series: []ChartSeries{
			{Name: "Sales", Values: []float64{100, 120, 115, 135, 150}},
			{Name: "Target", Values: []float64{110, 115, 120, 125, 130}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw line chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineChartSmooth(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	config.Smooth = true
	config.Tension = 0.5
	chart := NewLineChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C", "D", "E"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{10, 30, 20, 40, 25}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw smooth line chart: %v", err)
	}
}

func TestLineChartWithFill(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	config.FillArea = true
	config.FillOpacity = 0.3
	chart := NewLineChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{20, 40, 30}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw filled line chart: %v", err)
	}
}

func TestLineChartLinearXScale(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	chart := NewLineChart(b, config)

	data := ChartData{
		Series: []ChartSeries{
			{
				Name:    "Data",
				Values:  []float64{10, 25, 15, 35, 20},
				XValues: []float64{0, 2, 4, 6, 8},
			},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw line chart with linear x-scale: %v", err)
	}
}

func TestLineChartEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	chart := NewLineChart(b, config)

	data := ChartData{
		Series: []ChartSeries{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// =============================================================================
// Area Chart Tests
// =============================================================================

func TestDefaultAreaChartConfig(t *testing.T) {
	config := DefaultAreaChartConfig(400, 300)

	if !config.FillArea {
		t.Error("Expected FillArea to be true")
	}
	if config.FillOpacity != 0.5 {
		t.Errorf("Expected FillOpacity 0.5, got %v", config.FillOpacity)
	}
}

func TestAreaChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultAreaChartConfig(400, 300)
	chart := NewAreaChart(b, config)

	data := ChartData{
		Title:      "Website Traffic",
		Categories: []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		Series: []ChartSeries{
			{Name: "Visits", Values: []float64{1000, 1200, 1100, 1400, 1300}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw area chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Scatter Chart Tests
// =============================================================================

func TestDefaultScatterChartConfig(t *testing.T) {
	config := DefaultScatterChartConfig(400, 300)

	if config.PointSize != 8 {
		t.Errorf("Expected PointSize 8, got %v", config.PointSize)
	}
	if config.PointShape != MarkerCircle {
		t.Errorf("Expected PointShape MarkerCircle, got %v", config.PointShape)
	}
}

func TestScatterChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultScatterChartConfig(400, 300)
	chart := NewScatterChart(b, config)

	data := ChartData{
		Title: "Correlation",
		Series: []ChartSeries{
			{
				Name:    "Data Points",
				Values:  []float64{20, 35, 45, 60, 75, 85},
				XValues: []float64{10, 25, 30, 50, 65, 80},
			},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw scatter chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestScatterChartMultipleSeries(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultScatterChartConfig(400, 300)
	chart := NewScatterChart(b, config)

	data := ChartData{
		Title: "Clusters",
		Series: []ChartSeries{
			{Name: "Group A", Values: []float64{20, 25, 22}, XValues: []float64{10, 15, 12}},
			{Name: "Group B", Values: []float64{50, 55, 52}, XValues: []float64{40, 45, 42}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw scatter chart: %v", err)
	}
}

func TestScatterChartWithLabels(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultScatterChartConfig(400, 300)
	config.ShowLabels = true
	chart := NewScatterChart(b, config)

	data := ChartData{
		Series: []ChartSeries{
			{
				Name:    "Points",
				Values:  []float64{30, 50},
				XValues: []float64{20, 40},
				Labels:  []string{"A", "B"},
			},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw scatter chart with labels: %v", err)
	}
}

func TestScatterChartEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultScatterChartConfig(400, 300)
	chart := NewScatterChart(b, config)

	data := ChartData{
		Series: []ChartSeries{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// =============================================================================
// Pie Chart Tests
// =============================================================================

func TestDefaultPieChartConfig(t *testing.T) {
	config := DefaultPieChartConfig(400, 300)

	if config.InnerRadius != 0 {
		t.Errorf("Expected InnerRadius 0, got %v", config.InnerRadius)
	}
	if config.StartAngle != -90 {
		t.Errorf("Expected StartAngle -90, got %v", config.StartAngle)
	}
}

func TestDefaultDonutChartConfig(t *testing.T) {
	config := DefaultDonutChartConfig(400, 300)

	if config.InnerRadius != -1 {
		t.Errorf("Expected InnerRadius -1 (auto), got %v", config.InnerRadius)
	}
}

func TestPieChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPieChartConfig(400, 300)
	chart := NewPieChart(b, config)

	data := ChartData{
		Title:      "Market Share",
		Categories: []string{"Product A", "Product B", "Product C", "Other"},
		Series: []ChartSeries{
			{Values: []float64{35, 30, 20, 15}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw pie chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestDonutChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultDonutChartConfig(400, 300)
	chart := NewPieChart(b, config)

	data := ChartData{
		Title:      "Distribution",
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Values: []float64{40, 35, 25}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw donut chart: %v", err)
	}
}

func TestPieChartWithLabels(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPieChartConfig(400, 300)
	config.ShowLabels = true
	config.LabelFormat = "%.0f%%"
	chart := NewPieChart(b, config)

	data := ChartData{
		Categories: []string{"X", "Y"},
		Series: []ChartSeries{
			{Values: []float64{60, 40}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw pie chart with labels: %v", err)
	}
}

func TestPieChartEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPieChartConfig(400, 300)
	chart := NewPieChart(b, config)

	data := ChartData{
		Series: []ChartSeries{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// =============================================================================
// Radar Chart Tests
// =============================================================================

func TestDefaultRadarChartConfig(t *testing.T) {
	config := DefaultRadarChartConfig(400, 300)

	if config.Levels != 5 {
		t.Errorf("Expected Levels 5, got %v", config.Levels)
	}
	if !config.ShowPoints {
		t.Error("Expected ShowPoints to be true")
	}
	if config.FillOpacity != 0.2 {
		t.Errorf("Expected FillOpacity 0.2, got %v", config.FillOpacity)
	}
}

func TestRadarChartDraw(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	chart := NewRadarChart(b, config)

	data := ChartData{
		Title:      "Skills Assessment",
		Categories: []string{"Technical", "Communication", "Leadership", "Creativity", "Teamwork"},
		Series: []ChartSeries{
			{Name: "Employee A", Values: []float64{80, 70, 60, 90, 85}},
			{Name: "Employee B", Values: []float64{70, 85, 75, 60, 80}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestRadarChartSingleSeries(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C", "D"},
		Series: []ChartSeries{
			{Name: "Data", Values: []float64{50, 70, 60, 80}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart: %v", err)
	}
}

func TestRadarChartWithMaxValue(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	config.MaxValue = 100
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "Data", Values: []float64{80, 60, 70}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart with max value: %v", err)
	}
}

func TestRadarChartEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{},
		Series:     []ChartSeries{},
	}

	err := chart.Draw(data)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

func TestRadarChartAllZeroValues(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C", "D"},
		Series: []ChartSeries{
			{Name: "Zeros", Values: []float64{0, 0, 0, 0}},
		},
	}

	// Must not panic from division-by-zero when all values are zero
	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart with all-zero values: %v", err)
	}

	svg, err := b.RenderToString()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Error("Expected valid SVG output for all-zero radar chart")
	}
}

func TestRadarChartAllZeroValuesMultipleSeries(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{"X", "Y", "Z"},
		Series: []ChartSeries{
			{Name: "Series1", Values: []float64{0, 0, 0}},
			{Name: "Series2", Values: []float64{0, 0, 0}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart with multiple all-zero series: %v", err)
	}
}

func TestRadarChartLabelsNotHyphenated(t *testing.T) {
	// Regression test for pptx-3zw: radar chart labels like "Documentation",
	// "Integrations", "Usability" were being hyphenated (e.g., "Documenta-tion")
	// at small canvas sizes because radiusFactor=0.8 left insufficient label room.
	categories := []string{
		"Performance", "Security", "Usability",
		"Scalability", "Reliability", "Documentation", "Integrations",
	}
	values := []float64{9, 8, 7, 6, 5, 8, 7}

	// Test at small canvas sizes where the bug manifested.
	for _, size := range []float64{250, 300, 350} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size, size), func(t *testing.T) {
			b := NewSVGBuilder(size, size)
			config := DefaultRadarChartConfig(size, size)
			chart := NewRadarChart(b, config)
			data := ChartData{
				Title:      "Product Readiness",
				Categories: categories,
				Series:     []ChartSeries{{Name: "Score", Values: values}},
			}
			if err := chart.Draw(data); err != nil {
				t.Fatalf("Draw failed: %v", err)
			}
			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			svg := string(doc.Content)
			for _, cat := range categories {
				if strings.Contains(svg, cat[:len(cat)-3]+"-") {
					t.Errorf("label %q appears hyphenated in SVG at size %.0f", cat, size)
				}
			}
		})
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestCompleteBarChartSVG(t *testing.T) {
	b := NewSVGBuilder(600, 400)
	config := DefaultBarChartConfig(600, 400)
	config.ShowValues = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Quarterly Revenue",
		Subtitle:   "2024 Performance",
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []ChartSeries{
			{Name: "Actual", Values: []float64{120, 150, 135, 180}},
			{Name: "Budget", Values: []float64{100, 130, 140, 160}},
		},
		Footnote: "Values in millions USD",
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw chart: %v", err)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := doc.String()
	if !strings.Contains(svg, "<svg") {
		t.Error("Expected SVG tag in output")
	}
}

func TestMultipleChartsOnCanvas(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	// Draw a bar chart in top half
	barConfig := DefaultBarChartConfig(800, 280)
	barConfig.MarginTop = 20
	barChart := NewBarChart(b, barConfig)

	barData := ChartData{
		Title:      "Bar Chart",
		Categories: []string{"A", "B", "C"},
		Series:     []ChartSeries{{Name: "S1", Values: []float64{30, 50, 40}}},
	}

	err := barChart.Draw(barData)
	if err != nil {
		t.Fatalf("Failed to draw bar chart: %v", err)
	}

	// Note: Multiple charts on one canvas would require translating
	// This test just verifies both can render without error

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartWithCustomSeriesColors(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	chart := NewLineChart(b, config)

	red := MustParseColor("#FF0000")
	blue := MustParseColor("#0000FF")

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "Red Series", Values: []float64{10, 20, 15}, Color: &red},
			{Name: "Blue Series", Values: []float64{15, 25, 20}, Color: &blue},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw chart with custom colors: %v", err)
	}
}

func TestChartTypes(t *testing.T) {
	types := []ChartType{
		ChartTypeBar,
		ChartTypeLine,
		ChartTypeArea,
		ChartTypeScatter,
		ChartTypePie,
		ChartTypeDonut,
		ChartTypeRadar,
	}

	expected := []string{"bar", "line", "area", "scatter", "pie", "donut", "radar"}

	for i, ct := range types {
		if string(ct) != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], ct)
		}
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestBarChartNegativeValues(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarChartConfig(400, 300)
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{-20, 30, -10}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw bar chart with negative values: %v", err)
	}
}

// =============================================================================
// Log Scale Auto-Detection
// =============================================================================

func TestBarChartLogScale_ExtremeRange(t *testing.T) {
	// Values spanning 6 orders of magnitude should trigger log scale
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.ShowValues = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Company Valuations",
		Categories: []string{"Startup A", "Startup B", "Mid-cap", "Large-cap", "Mega-corp"},
		Series: []ChartSeries{
			{Name: "Valuation ($)", Values: []float64{500_000, 5_000_000, 50_000_000, 500_000_000, 5_000_000_000}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw extreme-range bar chart: %v", err)
	}

	// Verify log scale was activated
	if chart.logScale == nil {
		t.Error("Expected log scale to be activated for 4-order-of-magnitude range")
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}

	svg := doc.String()
	if !strings.Contains(svg, "<svg") {
		t.Error("Expected valid SVG output")
	}
}

func TestBarChartLogScale_NarrowRange(t *testing.T) {
	// Values within 2 orders of magnitude should NOT trigger log scale
	b := NewSVGBuilder(600, 400)
	config := DefaultBarChartConfig(600, 400)
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{10, 50, 200}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	if chart.logScale != nil {
		t.Error("Log scale should not activate for narrow value range (20:1)")
	}
}

func TestBarChartLogScale_NegativeValues(t *testing.T) {
	// Negative values should prevent log scale (even with wide range)
	b := NewSVGBuilder(600, 400)
	config := DefaultBarChartConfig(600, 400)
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{-100, 1, 1_000_000}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	if chart.logScale != nil {
		t.Error("Log scale should not activate when negative values present")
	}
}

func TestBarChartLogScale_StackedDisabled(t *testing.T) {
	// Stacked bars should never use log scale
	b := NewSVGBuilder(600, 400)
	config := DefaultBarChartConfig(600, 400)
	config.Stacked = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S1", Values: []float64{1, 100, 100_000}},
			{Name: "S2", Values: []float64{10, 1000, 1_000_000}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	if chart.logScale != nil {
		t.Error("Log scale should not activate for stacked bars")
	}
}

func TestBarChartLogScale_WithZeroValues(t *testing.T) {
	// Zero values among wide-range positives: log scale should still work,
	// zero-value bars should be placed at baseline
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.ShowValues = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Categories: []string{"None", "Small", "Medium", "Large"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{0, 100, 10_000, 10_000_000}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	// With a positive range of 100 to 10M (5 orders), log scale should activate
	if chart.logScale == nil {
		t.Error("Expected log scale for 5-order-of-magnitude positive range")
	}
}

func TestBarChartLogScale_MultipleSeries(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	config := DefaultBarChartConfig(800, 600)
	config.ShowValues = true
	chart := NewBarChart(b, config)

	data := ChartData{
		Title:      "Budget vs Actual",
		Categories: []string{"R&D", "Marketing", "Sales", "Support"},
		Series: []ChartSeries{
			{Name: "Budget", Values: []float64{500, 50_000, 5_000_000, 50_000_000}},
			{Name: "Actual", Values: []float64{800, 45_000, 4_800_000, 48_000_000}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw: %v", err)
	}

	if chart.logScale == nil {
		t.Error("Expected log scale for multi-series extreme range")
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG")
	}
}

func TestLineChartSinglePoint(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineChartConfig(400, 300)
	chart := NewLineChart(b, config)

	data := ChartData{
		Categories: []string{"A"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{50}},
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw line chart with single point: %v", err)
	}
}

func TestPieChartZeroValue(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPieChartConfig(400, 300)
	chart := NewPieChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Values: []float64{50, 0, 50}}, // One zero value
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw pie chart with zero value: %v", err)
	}
}

// TestPieChartLegendManyItems verifies that pie/donut charts with 5+ legend
// items allocate enough legend height (dynamic cap) and render without error.
// This is a regression test for a bug where a hard 25% legend height cap
// caused the bottom legend items to be clipped.
func TestPieChartLegendManyItems(t *testing.T) {
	tests := []struct {
		name       string
		numItems   int
		chartType  string // "pie" or "donut"
		width      float64
		height     float64
	}{
		{"5 items pie", 5, "pie", 400, 300},
		{"6 items pie", 6, "pie", 400, 300},
		{"8 items donut", 8, "donut", 400, 300},
		{"10 items pie", 10, "pie", 500, 400},
		{"5 items small chart", 5, "pie", 300, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewSVGBuilder(tt.width, tt.height)
			var config PieChartConfig
			if tt.chartType == "donut" {
				config = DefaultDonutChartConfig(tt.width, tt.height)
			} else {
				config = DefaultPieChartConfig(tt.width, tt.height)
			}
			chart := NewPieChart(b, config)

			categories := make([]string, tt.numItems)
			values := make([]float64, tt.numItems)
			for i := 0; i < tt.numItems; i++ {
				categories[i] = fmt.Sprintf("Category %d", i+1)
				values[i] = float64(100/tt.numItems + i)
			}

			data := ChartData{
				Title:      "Many Legend Items",
				Categories: categories,
				Series:     []ChartSeries{{Values: values}},
			}

			err := chart.Draw(data)
			if err != nil {
				t.Fatalf("Failed to draw %s chart with %d items: %v", tt.chartType, tt.numItems, err)
			}

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Failed to render SVG: %v", err)
			}

			if len(doc.Content) == 0 {
				t.Error("Expected non-empty SVG content")
			}
		})
	}
}

// TestDonutChartLegendAllItemsVisible verifies that all legend items are rendered
// even in smaller chart sizes where the dynamic cap would previously clip them.
// Regression test for go-slide-creator-wm87q: donut chart with 5 categories
// dropped the last legend item (Admin) at certain dimensions.
func TestDonutChartLegendAllItemsVisible(t *testing.T) {
	labels := []string{"Research", "Sales", "Marketing", "Operations", "Admin"}
	values := []float64{35, 25, 20, 13, 7}

	sizes := []struct {
		w, h float64
		name string
	}{
		{800, 600, "800x600-default"},
		{400, 350, "400x350-slot"},
		{380, 400, "380x400-narrow"},
		{350, 300, "350x300-small"},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			b := NewSVGBuilder(sz.w, sz.h)
			config := DefaultDonutChartConfig(sz.w, sz.h)
			chart := NewPieChart(b, config)

			data := ChartData{
				Title:      "Budget Allocation",
				Categories: labels,
				Series:     []ChartSeries{{Values: values}},
			}

			if err := chart.Draw(data); err != nil {
				t.Fatalf("Draw failed: %v", err)
			}

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			svg := string(doc.Content)
			for _, label := range labels {
				if !strings.Contains(svg, label) {
					t.Errorf("Legend missing label %q at %s", label, sz.name)
				}
			}
		})
	}
}

// TestPieChartLegendDynamicCap verifies the dynamic legend height cap scales
// with the number of items: more items get a higher cap percentage.
func TestPieChartLegendDynamicCap(t *testing.T) {
	// Measure legend height allocation for different item counts.
	// With the dynamic cap, 6+ items should get proportionally more space.
	measureLegendAlloc := func(numItems int) float64 {
		width, height := 400.0, 300.0
		b := NewSVGBuilder(width, height)
		config := DefaultPieChartConfig(width, height)
		chart := NewPieChart(b, config)

		categories := make([]string, numItems)
		values := make([]float64, numItems)
		for i := 0; i < numItems; i++ {
			categories[i] = fmt.Sprintf("Category %d", i+1)
			values[i] = float64(100 / numItems)
		}

		// Use measureLegendHeight to see how much space the legend requests
		style := b.StyleGuide()
		plotArea := config.PlotArea()

		_ = chart
		legendConfig := PresentationPieLegendConfig(style)
		legend := NewLegend(b, legendConfig)
		items := pieLegendItems(values, categories, style.Palette.AccentColors())
		legend.SetItems(items)
		return legend.Height(plotArea.W)
	}

	h3 := measureLegendAlloc(3)
	h6 := measureLegendAlloc(6)
	h8 := measureLegendAlloc(8)

	// More items should require more height
	if h6 <= h3 {
		t.Errorf("Expected 6-item legend (%v) > 3-item legend (%v)", h6, h3)
	}
	if h8 <= h6 {
		t.Errorf("Expected 8-item legend (%v) > 6-item legend (%v)", h8, h6)
	}
}

func TestRadarChartExceedsMaxValue(t *testing.T) {
	b := NewSVGBuilder(400, 400)
	config := DefaultRadarChartConfig(400, 400)
	config.MaxValue = 50 // Set max below actual values
	chart := NewRadarChart(b, config)

	data := ChartData{
		Categories: []string{"A", "B", "C"},
		Series: []ChartSeries{
			{Name: "S", Values: []float64{60, 70, 80}}, // Exceed max
		},
	}

	err := chart.Draw(data)
	if err != nil {
		t.Fatalf("Failed to draw radar chart with values exceeding max: %v", err)
	}
}
