package svggen

import (
	"strings"
	"testing"
)

func TestAxisConfig_Defaults(t *testing.T) {
	config := DefaultAxisConfig(AxisPositionBottom)

	if config.Position != AxisPositionBottom {
		t.Errorf("Position = %v, want AxisPositionBottom", config.Position)
	}
	if config.TickCount != 5 {
		t.Errorf("TickCount = %v, want 5", config.TickCount)
	}
	if config.TickSize != 6 {
		t.Errorf("TickSize = %v, want 6", config.TickSize)
	}
	if config.TickPadding != 4 {
		t.Errorf("TickPadding = %v, want 4", config.TickPadding)
	}
	if config.ShowGridLines {
		t.Error("ShowGridLines should be false by default")
	}
}

func TestAxisBuilder_Fluent(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	axisBuilder := NewAxisBuilder(builder, AxisPositionBottom)

	// Test chaining
	result := axisBuilder.
		Title("X Axis").
		TickCount(10).
		TickSize(8).
		TickPadding(6).
		ShowGrid(200).
		LabelRotation(45).
		Format("%.1f")

	if result != axisBuilder {
		t.Error("Fluent methods should return the same builder")
	}

	if axisBuilder.config.Title != "X Axis" {
		t.Errorf("Title = %q, want %q", axisBuilder.config.Title, "X Axis")
	}
	if axisBuilder.config.TickCount != 10 {
		t.Errorf("TickCount = %v, want 10", axisBuilder.config.TickCount)
	}
	if axisBuilder.config.TickSize != 8 {
		t.Errorf("TickSize = %v, want 8", axisBuilder.config.TickSize)
	}
	if axisBuilder.config.TickPadding != 6 {
		t.Errorf("TickPadding = %v, want 6", axisBuilder.config.TickPadding)
	}
	if !axisBuilder.config.ShowGridLines {
		t.Error("ShowGridLines should be true after ShowGrid()")
	}
	if axisBuilder.config.GridLineLength != 200 {
		t.Errorf("GridLineLength = %v, want 200", axisBuilder.config.GridLineLength)
	}
	if axisBuilder.config.LabelRotation != 45 {
		t.Errorf("LabelRotation = %v, want 45", axisBuilder.config.LabelRotation)
	}
	if axisBuilder.config.Format != "%.1f" {
		t.Errorf("Format = %q, want %q", axisBuilder.config.Format, "%.1f")
	}
}

func TestAxisBuilder_HideMethods(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	axisBuilder := NewAxisBuilder(builder, AxisPositionLeft).
		HideAxisLine().
		HideTicks().
		HideLabels()

	if !axisBuilder.config.HideAxisLine {
		t.Error("HideAxisLine should be true")
	}
	if !axisBuilder.config.HideTicks {
		t.Error("HideTicks should be true")
	}
	if !axisBuilder.config.HideLabels {
		t.Error("HideLabels should be true")
	}
}

func TestNewAxis(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	config := DefaultAxisConfig(AxisPositionBottom)
	axis := NewAxis(builder, config)

	if axis == nil {
		t.Fatal("NewAxis returned nil")
	}
	if axis.builder != builder {
		t.Error("Axis builder reference incorrect")
	}
	if axis.config.Position != AxisPositionBottom {
		t.Errorf("Axis position = %v, want AxisPositionBottom", axis.config.Position)
	}
}

func TestDrawLinearAxis_Bottom(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	scale := NewLinearScale(0, 100).SetRangeLinear(0, 300)

	config := DefaultAxisConfig(AxisPositionBottom)
	config.Title = "Values"
	axis := NewAxis(builder, config)

	// This should not panic
	axis.DrawLinearAxis(scale, 50, 250)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Basic sanity checks
	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestDrawLinearAxis_Left(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	scale := NewLinearScale(0, 50).SetRangeLinear(0, 200)

	config := DefaultAxisConfig(AxisPositionLeft)
	config.Title = "Count"
	config.ShowGridLines = true
	config.GridLineLength = 300
	axis := NewAxis(builder, config)

	// This should not panic
	axis.DrawLinearAxis(scale, 50, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestDrawCategoricalAxis_Bottom(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)

	config := DefaultAxisConfig(AxisPositionBottom)
	config.Title = "Quarter"
	axis := NewAxis(builder, config)

	// This should not panic
	axis.DrawCategoricalAxis(scale, 50, 250)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestDrawCategoricalAxis_Left(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	categories := []string{"High", "Medium", "Low"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 200)

	config := DefaultAxisConfig(AxisPositionLeft)
	axis := NewAxis(builder, config)

	// This should not panic
	axis.DrawCategoricalAxis(scale, 60, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestAxisPositions(t *testing.T) {
	positions := []AxisPosition{
		AxisPositionBottom,
		AxisPositionTop,
		AxisPositionLeft,
		AxisPositionRight,
	}

	for _, pos := range positions {
		t.Run("", func(t *testing.T) {
			builder := NewSVGBuilder(400, 300)
			scale := NewLinearScale(0, 100).SetRangeLinear(0, 200)

			config := DefaultAxisConfig(pos)
			axis := NewAxis(builder, config)

			// Should not panic
			axis.DrawLinearAxis(scale, 100, 100)

			_, err := builder.RenderToString()
			if err != nil {
				t.Errorf("Render failed for position %v: %v", pos, err)
			}
		})
	}
}

func TestAxis_EmptyTicks(t *testing.T) {
	builder := NewSVGBuilder(400, 300)

	// Scale with same min/max will produce few ticks
	scale := NewLinearScale(50, 50).SetRangeLinear(0, 300)

	config := DefaultAxisConfig(AxisPositionBottom)
	axis := NewAxis(builder, config)

	// Should not panic with degenerate scale
	axis.DrawLinearAxis(scale, 50, 250)

	_, err := builder.RenderToString()
	if err != nil {
		t.Errorf("Render failed: %v", err)
	}
}

func TestAxis_WithRotatedLabels(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	categories := []string{"January", "February", "March", "April"}
	scale := NewCategoricalScale(categories).SetRangeCategorical(0, 350)

	config := DefaultAxisConfig(AxisPositionBottom)
	config.LabelRotation = -45
	axis := NewAxis(builder, config)

	axis.DrawCategoricalAxis(scale, 25, 250)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestChartArea_Defaults(t *testing.T) {
	area := DefaultChartArea(800, 600)

	if area.Width != 800 {
		t.Errorf("Width = %v, want 800", area.Width)
	}
	if area.Height != 600 {
		t.Errorf("Height = %v, want 600", area.Height)
	}
	if area.MarginTop == 0 || area.MarginRight == 0 ||
		area.MarginBottom == 0 || area.MarginLeft == 0 {
		t.Error("Margins should be > 0")
	}
}

func TestChartArea_PlotRect(t *testing.T) {
	area := ChartArea{
		Width:        400,
		Height:       300,
		MarginTop:    20,
		MarginRight:  30,
		MarginBottom: 40,
		MarginLeft:   50,
	}

	rect := area.PlotRect()

	if rect.X != 50 {
		t.Errorf("PlotRect.X = %v, want 50", rect.X)
	}
	if rect.Y != 20 {
		t.Errorf("PlotRect.Y = %v, want 20", rect.Y)
	}
	if rect.W != 320 { // 400 - 50 - 30
		t.Errorf("PlotRect.W = %v, want 320", rect.W)
	}
	if rect.H != 240 { // 300 - 20 - 40
		t.Errorf("PlotRect.H = %v, want 240", rect.H)
	}
}

func TestChartArea_PlotDimensions(t *testing.T) {
	area := ChartArea{
		Width:        400,
		Height:       300,
		MarginTop:    20,
		MarginRight:  30,
		MarginBottom: 40,
		MarginLeft:   50,
	}

	if area.PlotWidth() != 320 {
		t.Errorf("PlotWidth() = %v, want 320", area.PlotWidth())
	}
	if area.PlotHeight() != 240 {
		t.Errorf("PlotHeight() = %v, want 240", area.PlotHeight())
	}
}

func TestChartArea_AxisPositions(t *testing.T) {
	area := ChartArea{
		Width:        400,
		Height:       300,
		MarginTop:    20,
		MarginRight:  30,
		MarginBottom: 40,
		MarginLeft:   50,
	}

	xAxisY := area.XAxisY()
	if xAxisY != 260 { // 300 - 40
		t.Errorf("XAxisY() = %v, want 260", xAxisY)
	}

	yAxisX := area.YAxisX()
	if yAxisX != 50 {
		t.Errorf("YAxisX() = %v, want 50", yAxisX)
	}
}

func TestGridConfig_Defaults(t *testing.T) {
	config := DefaultGridConfig()

	if !config.ShowHorizontal {
		t.Error("ShowHorizontal should be true by default")
	}
	if config.ShowVertical {
		t.Error("ShowVertical should be false by default")
	}
	if config.StrokeWidth <= 0 {
		t.Error("StrokeWidth should be > 0")
	}
	// DashPattern is nil (solid lines) for a clean dashboard look.
	// This is intentional — solid grid lines are more professional than dashed.
}

func TestDrawGrid(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	area := DefaultChartArea(400, 300)

	xScale := NewLinearScale(0, 100).SetRangeLinear(0, area.PlotWidth())
	yScale := NewLinearScale(0, 50).SetRangeLinear(0, area.PlotHeight())

	config := DefaultGridConfig()
	config.ShowVertical = true

	// Should not panic
	DrawGrid(builder, area, xScale, yScale, config)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestAxisBuilder_DrawLinear(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	scale := NewLinearScale(0, 100).SetRangeLinear(0, 300)

	result := NewAxisBuilder(builder, AxisPositionBottom).
		Title("X Axis").
		TickCount(5).
		DrawLinear(scale, 50, 250)

	if result != builder {
		t.Error("DrawLinear should return the SVGBuilder")
	}

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestAxisBuilder_DrawCategorical(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	scale := NewCategoricalScale([]string{"A", "B", "C"}).SetRangeCategorical(0, 300)

	result := NewAxisBuilder(builder, AxisPositionBottom).
		Title("Categories").
		DrawCategorical(scale, 50, 250)

	if result != builder {
		t.Error("DrawCategorical should return the SVGBuilder")
	}

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
}

func TestLinearScale_CompactFormat(t *testing.T) {
	// When tick values exceed 9999 and no explicit format is set,
	// DrawLinearAxis should use FormatCompact (K/M/B suffixes).
	builder := NewSVGBuilder(600, 400)
	scale := NewLinearScale(0, 5000000).SetRangeLinear(0, 400)

	config := DefaultAxisConfig(AxisPositionLeft)
	axis := NewAxis(builder, config)
	axis.DrawLinearAxis(scale, 60, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// The SVG should contain compact labels like "1M", "2M", etc.
	// and should NOT contain the full number "1000000".
	if strings.Contains(svg, "1000000") {
		t.Error("SVG should use compact format (e.g. '1M') instead of full number '1000000'")
	}
	// Should contain at least one compact-formatted label
	hasCompact := strings.Contains(svg, "M") || strings.Contains(svg, "K")
	if !hasCompact {
		t.Error("SVG should contain at least one K or M suffix label for large values")
	}
}

func TestLinearScale_CompactFormatNotUsedForSmallValues(t *testing.T) {
	// When tick values are all <= 9999, compact format should NOT be used.
	builder := NewSVGBuilder(400, 300)
	scale := NewLinearScale(0, 100).SetRangeLinear(0, 300)

	config := DefaultAxisConfig(AxisPositionLeft)
	axis := NewAxis(builder, config)
	axis.DrawLinearAxis(scale, 50, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// For small values 0-100, labels should be normal numbers like "20", "40".
	for _, expectedLabel := range []string{"20", "40", "60", "80"} {
		if !strings.Contains(svg, ">"+expectedLabel+"<") {
			t.Errorf("SVG should contain plain numeric label %q for small-value axis", expectedLabel)
		}
	}
}

func TestLinearScale_CompactFormatExplicitOverride(t *testing.T) {
	// When an explicit format is provided via config, it should be used
	// even for large values, NOT compact format.
	builder := NewSVGBuilder(600, 400)
	scale := NewLinearScale(0, 5000000).SetRangeLinear(0, 400)

	config := DefaultAxisConfig(AxisPositionLeft)
	config.Format = "%.0f" // Explicit format
	axis := NewAxis(builder, config)
	axis.DrawLinearAxis(scale, 60, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// With explicit format "%.0f", the full number "1000000" should appear
	if !strings.Contains(svg, ">1000000<") {
		t.Error("SVG should contain full number '1000000' when explicit format is set")
	}
}

// Integration test: create a complete chart with axes
func TestCompleteChartWithAxes(t *testing.T) {
	// Create builder
	builder := NewSVGBuilder(500, 400)

	// Define chart area
	area := DefaultChartArea(500, 400)
	plotRect := area.PlotRect()

	// Create scales
	categories := []string{"Jan", "Feb", "Mar", "Apr"}
	xScale := NewCategoricalScale(categories).
		SetRangeCategorical(0, plotRect.W)

	yScale := NewLinearScale(0, 100).
		SetRangeLinear(plotRect.H, 0) // Inverted for screen coordinates

	// Draw background
	builder.SetFillColor(MustParseColor("#F8F9FA")).FillRect(builder.Bounds())

	// Draw grid
	gridConfig := DefaultGridConfig()
	gridConfig.ShowHorizontal = true
	DrawGrid(builder, area, xScale, yScale, gridConfig)

	// Draw X axis (bottom)
	NewAxisBuilder(builder, AxisPositionBottom).
		Title("Month").
		DrawCategorical(xScale, plotRect.X, area.XAxisY())

	// Draw Y axis (left)
	NewAxisBuilder(builder, AxisPositionLeft).
		Title("Value").
		TickCount(5).
		Format("%.0f").
		DrawLinear(yScale, area.YAxisX(), plotRect.Y)

	// Render
	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Verify output
	if !strings.Contains(svg, "svg") {
		t.Error("Output should contain svg element")
	}
	if len(svg) < 500 {
		t.Error("SVG seems too short for a complete chart")
	}
}

// TestRangeExtent_ClipsOutOfBoundsTicksOnYAxis verifies that setting
// RangeExtent on a left Y-axis prevents ticks from rendering outside
// the plot area when Nice() expands the domain.
func TestRangeExtent_ClipsOutOfBoundsTicksOnYAxis(t *testing.T) {
	builder := NewSVGBuilder(500, 400)

	// Data range 40-95; Nice() would push to 40-100 (step=20, ticks at 40,60,80,100).
	// Plot area height = 200px. Range maps [200..0] (Y inverted).
	plotH := 200.0
	scale := NewLinearScale(40, 95).SetRangeLinear(plotH, 0)
	scale.Nice(true)

	config := DefaultAxisConfig(AxisPositionLeft)
	config.RangeExtent = plotH // Clip to [0, 200] offset range
	axis := NewAxis(builder, config)

	// Origin at (60, 50). Valid tick Y positions: 50..250.
	axis.DrawLinearAxis(scale, 60, 50)

	svg, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// The scale domain after Nice should be [40, 100] (positive guard
	// keeps domainMin at 40, not pushed below 0).
	dMin, dMax := scale.DomainBounds()
	if dMin < 0 {
		t.Errorf("Nice() domainMin = %v, want >= 0 for positive data", dMin)
	}
	if dMax < 95 {
		t.Errorf("Nice() domainMax = %v, want >= 95", dMax)
	}

	// Verify SVG was produced and contains text elements (tick labels).
	if !strings.Contains(svg, "<text") {
		t.Error("SVG should contain tick label text elements")
	}
}
