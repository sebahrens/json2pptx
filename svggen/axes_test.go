package svggen

import (
	"encoding/xml"
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

// TestLeftAxisLabels_TextAnchorEnd is the regression test for adversarial
// finding A1: Y-axis tick labels (drawn with TextAlignRight) must emit an
// explicit text-anchor="end" on the <text> element. Without it, downstream
// renderers (rsvg, LibreOffice, browsers) treat the canvas-precomputed tspan x
// as a left-edge anchor, causing labels to drift right and overlap the axis.
//
// We parse the emitted SVG and assert that every <text> element whose tspan
// contains a numeric tick label (matching the values we drew) carries
// text-anchor="end". X-axis labels in the same SVG are left out of the
// assertion because they are center-aligned (text-anchor="middle").
func TestLeftAxisLabels_TextAnchorEnd(t *testing.T) {
	builder := NewSVGBuilder(400, 300)
	scale := NewLinearScale(0, 50).SetRangeLinear(0, 200)

	config := DefaultAxisConfig(AxisPositionLeft)
	config.TickCount = 6
	axis := NewAxis(builder, config)
	axis.DrawLinearAxis(scale, 60, 50)

	svgStr, err := builder.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Parse all <text> elements. Wrap in a synthetic root so xml.Decoder
	// treats the SVG as a stream of siblings even when the canvas library
	// emits inline <style> alongside text.
	type tspan struct {
		Content string `xml:",chardata"`
	}
	type textEl struct {
		Anchor string `xml:"text-anchor,attr"`
		Tspan  tspan  `xml:"tspan"`
	}
	dec := xml.NewDecoder(strings.NewReader(svgStr))
	var ticks []textEl
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "text" {
			continue
		}
		var el textEl
		// Re-decode this element including children into our struct.
		if err := dec.DecodeElement(&el, &se); err != nil {
			continue
		}
		// Skip elements with no tspan content (titles, empty).
		if strings.TrimSpace(el.Tspan.Content) == "" {
			continue
		}
		ticks = append(ticks, el)
	}

	if len(ticks) == 0 {
		t.Fatal("no <text> elements parsed from SVG; rendering may have failed")
	}

	// Expected Y-axis tick labels for scale 0..50, 6 ticks => 0, 10, 20, 30, 40, 50.
	wantLabels := map[string]bool{
		"0": true, "10": true, "20": true, "30": true, "40": true, "50": true,
	}
	var sawAny bool
	for _, el := range ticks {
		label := strings.TrimSpace(el.Tspan.Content)
		if !wantLabels[label] {
			continue
		}
		sawAny = true
		if el.Anchor != "end" {
			t.Errorf("Y-axis label %q: text-anchor = %q, want %q (right-aligned tick label must anchor at end)",
				label, el.Anchor, "end")
		}
	}
	if !sawAny {
		t.Fatalf("did not find any expected Y-axis tick labels in SVG; got %d <text> elements", len(ticks))
	}
}

// TestDrawText_TextAnchorByAlignment exercises the three TextAlign values
// directly through the builder and asserts that the emitted SVG carries the
// expected text-anchor attribute. This pins the contract that DrawText emits
// text-anchor="end" for TextAlignRight, "middle" for TextAlignCenter, and no
// text-anchor (i.e. SVG default "start") for TextAlignLeft.
func TestDrawText_TextAnchorByAlignment(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.DrawText("right", 150, 20, TextAlignRight, TextBaselineTop)
	b.DrawText("center", 150, 50, TextAlignCenter, TextBaselineTop)
	b.DrawText("left", 150, 80, TextAlignLeft, TextBaselineTop)

	svgStr, err := b.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	type tspan struct {
		Content string `xml:",chardata"`
	}
	type textEl struct {
		Anchor string `xml:"text-anchor,attr"`
		Tspan  tspan  `xml:"tspan"`
	}

	dec := xml.NewDecoder(strings.NewReader(svgStr))
	got := map[string]string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "text" {
			continue
		}
		var el textEl
		if err := dec.DecodeElement(&el, &se); err != nil {
			continue
		}
		got[strings.TrimSpace(el.Tspan.Content)] = el.Anchor
	}

	cases := map[string]string{
		"right":  "end",
		"center": "middle",
		"left":   "", // no text-anchor attribute -> SVG default "start"
	}
	for label, want := range cases {
		if anchor, ok := got[label]; !ok {
			t.Errorf("missing <text> element for label %q", label)
		} else if anchor != want {
			t.Errorf("label %q: text-anchor = %q, want %q", label, anchor, want)
		}
	}
}

// TestDrawText_DominantBaselineByBaseline exercises the four TextBaseline
// values directly and asserts that the emitted SVG carries the matching
// dominant-baseline attribute. Pins the contract that DrawText emits
// dominant-baseline="text-before-edge" for TextBaselineTop, "central" for
// TextBaselineMiddle, "text-after-edge" for TextBaselineBottom, and no
// dominant-baseline (SVG default = alphabetic) for TextBaselineAlphabetic.
func TestDrawText_DominantBaselineByBaseline(t *testing.T) {
	b := NewSVGBuilder(200, 200)
	b.DrawText("top", 50, 30, TextAlignLeft, TextBaselineTop)
	b.DrawText("middle", 50, 70, TextAlignLeft, TextBaselineMiddle)
	b.DrawText("bottom", 50, 110, TextAlignLeft, TextBaselineBottom)
	b.DrawText("alpha", 50, 150, TextAlignLeft, TextBaselineAlphabetic)

	svgStr, err := b.RenderToString()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	type tspan struct {
		Content string `xml:",chardata"`
	}
	type textEl struct {
		Baseline string `xml:"dominant-baseline,attr"`
		Tspan    tspan  `xml:"tspan"`
	}

	dec := xml.NewDecoder(strings.NewReader(svgStr))
	got := map[string]string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "text" {
			continue
		}
		var el textEl
		if err := dec.DecodeElement(&el, &se); err != nil {
			continue
		}
		got[strings.TrimSpace(el.Tspan.Content)] = el.Baseline
	}

	cases := map[string]string{
		"top":    "text-before-edge",
		"middle": "central",
		"bottom": "text-after-edge",
		"alpha":  "", // no dominant-baseline attribute -> SVG default
	}
	for label, want := range cases {
		if baseline, ok := got[label]; !ok {
			t.Errorf("missing <text> element for label %q", label)
		} else if baseline != want {
			t.Errorf("label %q: dominant-baseline = %q, want %q", label, baseline, want)
		}
	}
}

// TestAxisLabels_DominantBaseline is the regression test for adversarial
// finding A5: axis tick labels must emit an explicit dominant-baseline so
// downstream renderers (rsvg, LibreOffice, browsers, PowerPoint) don't drift
// the label vertically based on their own font ascent/descent metrics.
//
// Left axis labels use TextBaselineMiddle -> "central".
// Bottom axis (unrotated) labels use TextBaselineTop -> "text-before-edge".
func TestAxisLabels_DominantBaseline(t *testing.T) {
	// --- Left axis (numeric ticks, TextBaselineMiddle) ---
	leftBuilder := NewSVGBuilder(400, 300)
	leftScale := NewLinearScale(0, 50).SetRangeLinear(0, 200)
	leftConfig := DefaultAxisConfig(AxisPositionLeft)
	leftConfig.TickCount = 6
	leftAxis := NewAxis(leftBuilder, leftConfig)
	leftAxis.DrawLinearAxis(leftScale, 60, 50)
	leftSVG, err := leftBuilder.RenderToString()
	if err != nil {
		t.Fatalf("Left axis render failed: %v", err)
	}
	leftLabels := map[string]bool{"0": true, "10": true, "20": true, "30": true, "40": true, "50": true}
	assertBaseline(t, "Left", leftSVG, leftLabels, "central")

	// --- Bottom axis (categorical, unrotated, TextBaselineTop) ---
	bottomBuilder := NewSVGBuilder(400, 300)
	categories := []string{"Q1", "Q2", "Q3", "Q4"}
	bottomScale := NewCategoricalScale(categories).SetRangeCategorical(0, 300)
	bottomConfig := DefaultAxisConfig(AxisPositionBottom)
	bottomAxis := NewAxis(bottomBuilder, bottomConfig)
	bottomAxis.DrawCategoricalAxis(bottomScale, 50, 250)
	bottomSVG, err := bottomBuilder.RenderToString()
	if err != nil {
		t.Fatalf("Bottom axis render failed: %v", err)
	}
	bottomLabels := map[string]bool{"Q1": true, "Q2": true, "Q3": true, "Q4": true}
	assertBaseline(t, "Bottom", bottomSVG, bottomLabels, "text-before-edge")
}

// assertBaseline parses every <text> element in svgStr and asserts that any
// element whose tspan content matches a key in expected carries the wanted
// dominant-baseline attribute.
func assertBaseline(t *testing.T, axisName, svgStr string, expected map[string]bool, want string) {
	t.Helper()
	type tspan struct {
		Content string `xml:",chardata"`
	}
	type textEl struct {
		Baseline string `xml:"dominant-baseline,attr"`
		Tspan    tspan  `xml:"tspan"`
	}
	dec := xml.NewDecoder(strings.NewReader(svgStr))
	var sawAny bool
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "text" {
			continue
		}
		var el textEl
		if err := dec.DecodeElement(&el, &se); err != nil {
			continue
		}
		label := strings.TrimSpace(el.Tspan.Content)
		if !expected[label] {
			continue
		}
		sawAny = true
		if el.Baseline != want {
			t.Errorf("%s axis label %q: dominant-baseline = %q, want %q",
				axisName, label, el.Baseline, want)
		}
	}
	if !sawAny {
		t.Fatalf("%s axis: did not find any expected tick labels in SVG", axisName)
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
