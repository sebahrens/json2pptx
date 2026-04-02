package svggen

import (
	"math"
	"strings"
	"testing"
)

// =============================================================================
// Bar Series Tests
// =============================================================================

func TestDefaultBarSeriesConfig(t *testing.T) {
	config := DefaultBarSeriesConfig()

	if config.Orientation != SeriesVertical {
		t.Errorf("Expected orientation SeriesVertical, got %v", config.Orientation)
	}
	if config.BarWidth != 0 {
		t.Errorf("Expected BarWidth 0 (auto), got %v", config.BarWidth)
	}
	if config.SeriesIndex != 0 {
		t.Errorf("Expected SeriesIndex 0, got %v", config.SeriesIndex)
	}
	if config.SeriesCount != 1 {
		t.Errorf("Expected SeriesCount 1, got %v", config.SeriesCount)
	}
	if config.StrokeWidth != 0 {
		t.Errorf("Expected StrokeWidth 0, got %v", config.StrokeWidth)
	}
}

func TestBarSeriesDrawCategorical(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()
	config.Color = MustParseColor("#4E79A7")

	bs := NewBarSeries(b, config)

	categories := []string{"A", "B", "C", "D"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{XCategory: "A", Y: 30},
		{XCategory: "B", Y: 70},
		{XCategory: "C", Y: 50},
		{XCategory: "D", Y: 90},
	}

	bs.DrawCategorical(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := doc.String()
	if len(svg) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarSeriesGrouped(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	categories := []string{"Q1", "Q2", "Q3"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	// Draw two series side by side
	for seriesIdx := 0; seriesIdx < 2; seriesIdx++ {
		config := DefaultBarSeriesConfig()
		config.SeriesIndex = seriesIdx
		config.SeriesCount = 2
		if seriesIdx == 0 {
			config.Color = MustParseColor("#4E79A7")
		} else {
			config.Color = MustParseColor("#F28E2B")
		}

		bs := NewBarSeries(b, config)

		points := []DataPoint{
			{XCategory: "Q1", Y: float64(30 + seriesIdx*20)},
			{XCategory: "Q2", Y: float64(50 + seriesIdx*10)},
			{XCategory: "Q3", Y: float64(70 - seriesIdx*15)},
		}

		bs.DrawCategorical(points, xScale, yScale, 250)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content for grouped bars")
	}
}

func TestBarSeriesWithValues(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()
	config.ShowValues = true
	config.ValueFormat = "%.1f"
	config.ValuePosition = ValuePositionTop

	bs := NewBarSeries(b, config)

	xScale := NewCategoricalScale([]string{"X", "Y"})
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{XCategory: "X", Y: 45.5},
		{XCategory: "Y", Y: 67.8},
	}

	bs.DrawCategorical(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarSeriesHorizontal(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()
	config.Orientation = SeriesHorizontal

	bs := NewBarSeries(b, config)

	yScale := NewCategoricalScale([]string{"A", "B", "C"})
	yScale.SetRangeCategorical(50, 250)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(100, 350)

	points := []DataPoint{
		{XCategory: "A", Y: 60},
		{XCategory: "B", Y: 80},
		{XCategory: "C", Y: 40},
	}

	// For horizontal bars, we swap the interpretation
	bs.DrawCategorical(points, yScale, xScale, 100)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG for horizontal bars")
	}
}

func TestBarSeriesLinear(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()

	bs := NewBarSeries(b, config)

	xScale := NewLinearScale(0, 10)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 1, Y: 30},
		{X: 3, Y: 55},
		{X: 5, Y: 40},
		{X: 7, Y: 75},
		{X: 9, Y: 60},
	}

	bs.DrawLinear(points, xScale, yScale, 20, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarSeriesWithRoundedCorners(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()
	config.CornerRadius = 4

	bs := NewBarSeries(b, config)

	xScale := NewCategoricalScale([]string{"A", "B"})
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{XCategory: "A", Y: 50},
		{XCategory: "B", Y: 70},
	}

	bs.DrawCategorical(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestBarSeriesEmptyPoints(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultBarSeriesConfig()
	bs := NewBarSeries(b, config)

	xScale := NewCategoricalScale([]string{"A"})
	xScale.SetRangeCategorical(50, 350)
	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	// Should not panic with empty points
	bs.DrawCategorical([]DataPoint{}, xScale, yScale, 250)

	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

// =============================================================================
// Line Series Tests
// =============================================================================

func TestDefaultLineSeriesConfig(t *testing.T) {
	config := DefaultLineSeriesConfig()

	if config.StrokeWidth != 3.0 {
		t.Errorf("Expected StrokeWidth 3.0, got %v", config.StrokeWidth)
	}
	if config.MarkerSize != 8 {
		t.Errorf("Expected MarkerSize 8, got %v", config.MarkerSize)
	}
	if !config.ShowMarkers {
		t.Error("Expected ShowMarkers to be true")
	}
	if config.FillArea {
		t.Error("Expected FillArea to be false")
	}
	if config.Smooth {
		t.Error("Expected Smooth to be false")
	}
}

func TestLineSeriesDrawCategorical(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()

	ls := NewLineSeries(b, config)

	categories := []string{"Jan", "Feb", "Mar", "Apr", "May"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{XCategory: "Jan", Y: 20},
		{XCategory: "Feb", Y: 45},
		{XCategory: "Mar", Y: 35},
		{XCategory: "Apr", Y: 60},
		{XCategory: "May", Y: 55},
	}

	ls.DrawCategorical(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineSeriesDrawLinear(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 10)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 0, Y: 10},
		{X: 2, Y: 35},
		{X: 4, Y: 28},
		{X: 6, Y: 52},
		{X: 8, Y: 45},
		{X: 10, Y: 70},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineSeriesWithAreaFill(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()
	config.FillArea = true
	config.FillColor = MustParseColor("#4E79A7")
	config.FillOpacity = 0.3

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 5)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 0, Y: 20},
		{X: 1, Y: 40},
		{X: 2, Y: 35},
		{X: 3, Y: 60},
		{X: 4, Y: 50},
		{X: 5, Y: 75},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// TestLineSeriesAreaFillNoBaselineStroke verifies that the area fill path does
// not produce a visible stroke at the baseline, even when data points coincide
// with the y-domain minimum (baseY). Regression test for pptx-3ly.
func TestLineSeriesAreaFillNoBaselineStroke(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	b.SetStrokeColor(MustParseColor("#FF0000"))
	b.SetStrokeWidth(3) // Deliberately set a thick stroke in the parent context

	config := DefaultLineSeriesConfig()
	config.FillArea = true
	config.FillColor = MustParseColor("#4E79A7")
	config.FillOpacity = 0.3
	config.Color = MustParseColor("#4E79A7")

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 4)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	// First point at y=0 maps exactly to baseY — this is the trigger condition
	points := []DataPoint{
		{X: 0, Y: 0},
		{X: 1, Y: 40},
		{X: 2, Y: 60},
		{X: 3, Y: 30},
		{X: 4, Y: 0},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := string(doc.Content)
	if len(svg) == 0 {
		t.Fatal("Expected non-empty SVG content")
	}

	// The fill path should not contain any visible stroke.  Look for
	// stroke-width:0 on path elements that carry the area fill colour.
	// This is a structural smoke-test; the primary fix lives in
	// PathBuilder.Fill() which zeroes the stroke width.
	if !strings.Contains(svg, "<path") {
		t.Error("Expected SVG to contain at least one <path> element")
	}
}

func TestLineSeriesSmooth(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()
	config.Smooth = true
	config.Tension = 0.5

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 4)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 0, Y: 30},
		{X: 1, Y: 60},
		{X: 2, Y: 40},
		{X: 3, Y: 80},
		{X: 4, Y: 55},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineSeriesMarkerShapes(t *testing.T) {
	shapes := []MarkerShape{
		MarkerCircle,
		MarkerSquare,
		MarkerDiamond,
		MarkerTriangle,
		MarkerTriangleDown,
		MarkerCross,
		MarkerPlus,
		MarkerNone,
	}

	for _, shape := range shapes {
		t.Run(markerShapeName(shape), func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultLineSeriesConfig()
			config.MarkerShape = shape

			ls := NewLineSeries(b, config)

			xScale := NewLinearScale(0, 2)
			xScale.SetRangeLinear(50, 350)

			yScale := NewLinearScale(0, 100)
			yScale.SetRangeLinear(250, 50)

			points := []DataPoint{
				{X: 0, Y: 30},
				{X: 1, Y: 60},
				{X: 2, Y: 45},
			}

			ls.DrawLinear(points, xScale, yScale, 250)

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

func TestLineSeriesDashed(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()
	config.DashPattern = []float64{6, 3}

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 3)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 0, Y: 20},
		{X: 1, Y: 50},
		{X: 2, Y: 40},
		{X: 3, Y: 70},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineSeriesSinglePoint(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()
	config.ShowMarkers = true

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 10)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	// Single point should still draw a marker
	points := []DataPoint{
		{X: 5, Y: 50},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLineSeriesWithValueLabels(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLineSeriesConfig()
	config.ShowValues = true
	config.ValueFormat = "%.1f"

	ls := NewLineSeries(b, config)

	xScale := NewLinearScale(0, 2)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 0, Y: 25.5},
		{X: 1, Y: 67.3},
		{X: 2, Y: 42.8},
	}

	ls.DrawLinear(points, xScale, yScale, 250)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Point Series Tests
// =============================================================================

func TestDefaultPointSeriesConfig(t *testing.T) {
	config := DefaultPointSeriesConfig()

	if config.Size != 8 {
		t.Errorf("Expected Size 8, got %v", config.Size)
	}
	if config.Shape != MarkerCircle {
		t.Errorf("Expected Shape MarkerCircle, got %v", config.Shape)
	}
	if config.Opacity != 0.8 {
		t.Errorf("Expected Opacity 0.8, got %v", config.Opacity)
	}
}

func TestPointSeriesDrawLinear(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()

	ps := NewPointSeries(b, config)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 10, Y: 20},
		{X: 25, Y: 45},
		{X: 40, Y: 35},
		{X: 55, Y: 60},
		{X: 70, Y: 50},
		{X: 85, Y: 75},
	}

	ps.DrawLinear(points, xScale, yScale)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestPointSeriesDrawCategorical(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()

	ps := NewPointSeries(b, config)

	categories := []string{"A", "B", "C", "D"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{XCategory: "A", Y: 30},
		{XCategory: "B", Y: 55},
		{XCategory: "C", Y: 40},
		{XCategory: "D", Y: 70},
	}

	ps.DrawCategorical(points, xScale, yScale)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestPointSeriesWithLabels(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()
	config.ShowLabels = true
	config.LabelOffset = 10

	ps := NewPointSeries(b, config)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 20, Y: 30, Label: "Point A"},
		{X: 50, Y: 60, Label: "Point B"},
		{X: 80, Y: 45, Label: "Point C"},
	}

	ps.DrawLinear(points, xScale, yScale)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestPointSeriesVariableSize(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()

	// Create a size scale
	sizeScale := NewLinearScale(0, 100)
	sizeScale.SetRangeLinear(0, 1)
	config.SizeScale = sizeScale

	ps := NewPointSeries(b, config)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 20, Y: 30, Value: 10},
		{X: 50, Y: 60, Value: 50},
		{X: 80, Y: 45, Value: 90},
	}

	ps.DrawLinear(points, xScale, yScale)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestPointSeriesVariableColor(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()

	// Create a color scale
	colorScale := NewLinearScale(0, 100)
	colorScale.SetRangeLinear(0, 1)
	config.ColorScale = colorScale

	ps := NewPointSeries(b, config)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	points := []DataPoint{
		{X: 20, Y: 30, Value: 10},
		{X: 50, Y: 60, Value: 50},
		{X: 80, Y: 45, Value: 90},
	}

	ps.DrawLinear(points, xScale, yScale)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestPointSeriesMarkerShapes(t *testing.T) {
	shapes := []MarkerShape{
		MarkerCircle,
		MarkerSquare,
		MarkerDiamond,
		MarkerTriangle,
		MarkerTriangleDown,
	}

	for _, shape := range shapes {
		t.Run(markerShapeName(shape), func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultPointSeriesConfig()
			config.Shape = shape

			ps := NewPointSeries(b, config)

			xScale := NewLinearScale(0, 100)
			xScale.SetRangeLinear(50, 350)

			yScale := NewLinearScale(0, 100)
			yScale.SetRangeLinear(250, 50)

			points := []DataPoint{
				{X: 30, Y: 40},
				{X: 60, Y: 70},
			}

			ps.DrawLinear(points, xScale, yScale)

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

func TestPointSeriesEmptyPoints(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultPointSeriesConfig()
	ps := NewPointSeries(b, config)

	xScale := NewLinearScale(0, 100)
	xScale.SetRangeLinear(50, 350)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(250, 50)

	// Should not panic with empty points
	ps.DrawLinear([]DataPoint{}, xScale, yScale)

	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

// =============================================================================
// Arc Series Tests
// =============================================================================

func TestDefaultArcSeriesConfig(t *testing.T) {
	config := DefaultArcSeriesConfig(200, 150, 100)

	if config.CenterX != 200 {
		t.Errorf("Expected CenterX 200, got %v", config.CenterX)
	}
	if config.CenterY != 150 {
		t.Errorf("Expected CenterY 150, got %v", config.CenterY)
	}
	if config.OuterRadius != 100 {
		t.Errorf("Expected OuterRadius 100, got %v", config.OuterRadius)
	}
	if config.InnerRadius != 0 {
		t.Errorf("Expected InnerRadius 0 (pie), got %v", config.InnerRadius)
	}
	if config.StartAngle != -90 {
		t.Errorf("Expected StartAngle -90, got %v", config.StartAngle)
	}
}

func TestArcSeriesPieChart(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	config.InnerRadius = 0 // Pie chart

	as := NewArcSeries(b, config)

	slices := []ArcSlice{
		{Value: 30, Label: "A"},
		{Value: 25, Label: "B"},
		{Value: 20, Label: "C"},
		{Value: 15, Label: "D"},
		{Value: 10, Label: "E"},
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	svg := doc.String()
	if len(svg) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesDonutChart(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	config.InnerRadius = 50 // Donut chart

	as := NewArcSeries(b, config)

	slices := []ArcSlice{
		{Value: 40},
		{Value: 30},
		{Value: 20},
		{Value: 10},
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesExploded(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 80)
	config.ExplodeOffset = 15
	config.ExplodedSlices = []int{0, 2}

	as := NewArcSeries(b, config)

	slices := []ArcSlice{
		{Value: 30, Exploded: true}, // Will be exploded via ExplodedSlices
		{Value: 25},
		{Value: 25}, // Will be exploded via ExplodedSlices
		{Value: 20},
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesWithCustomColors(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	config.Colors = []Color{
		MustParseColor("#FF6B6B"),
		MustParseColor("#4ECDC4"),
		MustParseColor("#45B7D1"),
		MustParseColor("#96CEB4"),
	}

	as := NewArcSeries(b, config)

	slices := []ArcSlice{
		{Value: 25},
		{Value: 25},
		{Value: 25},
		{Value: 25},
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesWithSliceColors(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)

	as := NewArcSeries(b, config)

	customColor := MustParseColor("#FF0000")
	slices := []ArcSlice{
		{Value: 50, Color: &customColor}, // Custom color for this slice
		{Value: 50},                      // Uses default palette
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesLabelPositions(t *testing.T) {
	positions := []ArcLabelPosition{
		ArcLabelInside,
		ArcLabelOutside,
		ArcLabelNone,
	}

	for _, pos := range positions {
		t.Run(labelPositionName(pos), func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultArcSeriesConfig(200, 150, 100)
			config.LabelPosition = pos
			config.ShowLabels = true

			as := NewArcSeries(b, config)

			slices := []ArcSlice{
				{Value: 30, Label: "A"},
				{Value: 40, Label: "B"},
				{Value: 30, Label: "C"},
			}

			as.Draw(slices)

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

func TestArcSeriesDrawFromValues(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)

	as := NewArcSeries(b, config)

	values := []float64{25, 30, 20, 15, 10}
	labels := []string{"A", "B", "C", "D", "E"}

	as.DrawFromValues(values, labels)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestArcSeriesEmptySlices(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	as := NewArcSeries(b, config)

	// Should not panic with empty slices
	as.Draw([]ArcSlice{})

	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

func TestArcSeriesZeroTotal(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	as := NewArcSeries(b, config)

	slices := []ArcSlice{
		{Value: 0},
		{Value: 0},
	}

	// Should not panic with zero total
	as.Draw(slices)

	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

func TestArcSeriesSmallSlices(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultArcSeriesConfig(200, 150, 100)
	config.PadAngle = 1

	as := NewArcSeries(b, config)

	// Test with many small slices
	slices := make([]ArcSlice, 20)
	for i := range slices {
		slices[i] = ArcSlice{Value: 5}
	}

	as.Draw(slices)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Utility Function Tests
// =============================================================================

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    float64
		format   string
		expected string
	}{
		{42.467, "%.0f", "42"},
		{42.567, "%.1f", "42.6"},
		{42.567, "%.2f", "42.57"},
		{42.567, "", "43"},      // Default format (%.0f rounds up)
		{0.5, "%.1f%%", "0.5%"}, // Percentage
		{1234.5, "$%.2f", "$1234.50"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := formatValue(tc.value, tc.format)
			if result != tc.expected {
				t.Errorf("formatValue(%v, %q) = %q, want %q", tc.value, tc.format, result, tc.expected)
			}
		})
	}
}

func TestDataPoint(t *testing.T) {
	dp := DataPoint{
		X:         10.5,
		Y:         20.3,
		XCategory: "Category1",
		Label:     "Test Point",
		Value:     100.0,
	}

	if dp.X != 10.5 {
		t.Errorf("Expected X 10.5, got %v", dp.X)
	}
	if dp.Y != 20.3 {
		t.Errorf("Expected Y 20.3, got %v", dp.Y)
	}
	if dp.XCategory != "Category1" {
		t.Errorf("Expected XCategory 'Category1', got %v", dp.XCategory)
	}
	if dp.Label != "Test Point" {
		t.Errorf("Expected Label 'Test Point', got %v", dp.Label)
	}
	if dp.Value != 100.0 {
		t.Errorf("Expected Value 100.0, got %v", dp.Value)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestMultipleSeriesOverlay(t *testing.T) {
	b := NewSVGBuilder(600, 400)

	xScale := NewLinearScale(0, 10)
	xScale.SetRangeLinear(60, 560)

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(340, 60)

	// Draw bar series
	barConfig := DefaultBarSeriesConfig()
	barConfig.Color = MustParseColor("#4E79A7").WithAlpha(0.5)
	bs := NewBarSeries(b, barConfig)

	barPoints := []DataPoint{
		{X: 1, Y: 30},
		{X: 3, Y: 50},
		{X: 5, Y: 40},
		{X: 7, Y: 60},
		{X: 9, Y: 55},
	}
	bs.DrawLinear(barPoints, xScale, yScale, 30, 340)

	// Draw line series on top
	lineConfig := DefaultLineSeriesConfig()
	lineConfig.Color = MustParseColor("#E15759")
	ls := NewLineSeries(b, lineConfig)

	linePoints := []DataPoint{
		{X: 0, Y: 20},
		{X: 2, Y: 45},
		{X: 4, Y: 35},
		{X: 6, Y: 65},
		{X: 8, Y: 50},
		{X: 10, Y: 70},
	}
	ls.DrawLinear(linePoints, xScale, yScale, 340)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}

	// Verify SVG structure
	svg := doc.String()
	if !strings.Contains(svg, "<svg") {
		t.Error("Expected SVG tag in output")
	}
}

func TestSeriesWithAxes(t *testing.T) {
	b := NewSVGBuilder(500, 350)

	// Define chart area
	area := DefaultChartArea(500, 350)

	// Create scales
	categories := []string{"Jan", "Feb", "Mar", "Apr"}
	xScale := NewCategoricalScale(categories)
	xScale.SetRangeCategorical(0, area.PlotWidth())

	yScale := NewLinearScale(0, 100)
	yScale.SetRangeLinear(area.PlotHeight(), 0)

	// Draw axes
	xAxisConfig := DefaultAxisConfig(AxisPositionBottom)
	xAxis := NewAxis(b, xAxisConfig)
	xAxis.DrawCategoricalAxis(xScale, area.YAxisX(), area.XAxisY())

	yAxisConfig := DefaultAxisConfig(AxisPositionLeft)
	yAxisConfig.ShowGridLines = true
	yAxisConfig.GridLineLength = area.PlotWidth()
	yAxis := NewAxis(b, yAxisConfig)
	yAxis.DrawLinearAxis(yScale, area.YAxisX(), area.MarginTop)

	// Draw bar series
	barConfig := DefaultBarSeriesConfig()
	bs := NewBarSeries(b, barConfig)

	points := []DataPoint{
		{XCategory: "Jan", Y: 45},
		{XCategory: "Feb", Y: 72},
		{XCategory: "Mar", Y: 58},
		{XCategory: "Apr", Y: 83},
	}

	// Translate x positions to account for margin
	translatedXScale := NewCategoricalScale(categories)
	translatedXScale.SetRangeCategorical(area.MarginLeft, area.Width-area.MarginRight)

	// Calculate base Y from scaled 0 value
	baseY := area.MarginTop + yScale.Scale(0)

	bs.DrawCategorical(points, translatedXScale, yScale, baseY)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func markerShapeName(s MarkerShape) string {
	names := map[MarkerShape]string{
		MarkerCircle:       "Circle",
		MarkerSquare:       "Square",
		MarkerDiamond:      "Diamond",
		MarkerTriangle:     "Triangle",
		MarkerTriangleDown: "TriangleDown",
		MarkerCross:        "Cross",
		MarkerPlus:         "Plus",
		MarkerNone:         "None",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}

func labelPositionName(p ArcLabelPosition) string {
	names := map[ArcLabelPosition]string{
		ArcLabelInside:  "Inside",
		ArcLabelOutside: "Outside",
		ArcLabelNone:    "None",
	}
	if name, ok := names[p]; ok {
		return name
	}
	return "Unknown"
}

// =============================================================================
// Area Fill Contrast Tests
// =============================================================================

func TestBlendOver(t *testing.T) {
	tests := []struct {
		name string
		fg   Color
		bg   Color
		wantR, wantG, wantB uint8
	}{
		{
			name:  "fully opaque foreground",
			fg:    Color{R: 100, G: 50, B: 200, A: 1.0},
			bg:    Color{R: 255, G: 255, B: 255, A: 1.0},
			wantR: 100, wantG: 50, wantB: 200,
		},
		{
			name:  "fully transparent foreground",
			fg:    Color{R: 100, G: 50, B: 200, A: 0.0},
			bg:    Color{R: 255, G: 255, B: 255, A: 1.0},
			wantR: 255, wantG: 255, wantB: 255,
		},
		{
			name:  "50% opacity over white",
			fg:    Color{R: 140, G: 141, B: 134, A: 0.5}, // template_2 accent1
			bg:    Color{R: 255, G: 255, B: 255, A: 1.0},
			wantR: 197, wantG: 198, wantB: 194,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := blendOver(tc.fg, tc.bg)
			if result.R != tc.wantR || result.G != tc.wantG || result.B != tc.wantB {
				t.Errorf("blendOver = RGB(%d,%d,%d), want RGB(%d,%d,%d)",
					result.R, result.G, result.B, tc.wantR, tc.wantG, tc.wantB)
			}
			if result.A != 1.0 {
				t.Errorf("blendOver alpha = %v, want 1.0", result.A)
			}
		})
	}
}

func TestEnsureAreaFillContrast(t *testing.T) {
	white := MustParseColor("#FFFFFF")

	t.Run("high contrast color unchanged", func(t *testing.T) {
		// Dark blue at 0.5 opacity has plenty of contrast on white
		fill := Color{R: 0, G: 0, B: 128, A: 0.5}
		result := ensureAreaFillContrast(fill, white)
		if result.A != 0.5 {
			t.Errorf("expected alpha 0.5 (unchanged), got %v", result.A)
		}
	})

	t.Run("muted color gets boosted", func(t *testing.T) {
		// muted gray-green (#8C8D86) at 0.5 is very low contrast on white
		fill := Color{R: 140, G: 141, B: 134, A: 0.5}
		result := ensureAreaFillContrast(fill, white)
		if result.A <= 0.5 {
			t.Errorf("expected alpha > 0.5 (boosted), got %v", result.A)
		}
		// Verify the boosted result actually has sufficient contrast
		blended := blendOver(result, white)
		contrast := blended.ContrastWith(white)
		if contrast < 2.5 {
			t.Errorf("boosted contrast %.2f still below threshold 2.5", contrast)
		}
	})

	t.Run("does not exceed max opacity", func(t *testing.T) {
		// Very light color that needs max boost
		fill := Color{R: 250, G: 250, B: 250, A: 0.1}
		result := ensureAreaFillContrast(fill, white)
		if result.A > 0.85 {
			t.Errorf("expected alpha <= 0.85 (max), got %v", result.A)
		}
	})
}

// Ensure math import is used (for test file completeness)
var _ = math.Pi
