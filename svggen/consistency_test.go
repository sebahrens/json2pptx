package svggen

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// Cross-Chart Consistency Tests
// =============================================================================
//
// These tests render all Cartesian chart types with identical data at identical
// dimensions, then assert that structural properties are consistent across
// chart types. Inconsistencies indicate that different chart renderers are
// using different title positioning, margin calculations, or plot area sizing.
// =============================================================================

// cartesianChartTypes returns RequestEnvelopes for all Cartesian chart types
// rendered with the same data and dimensions. This enables cross-type comparison.
func cartesianChartTypes() map[string]*RequestEnvelope {
	// Shared data: 6 categories, 2 series, same dimensions
	categories := []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	series := []any{
		map[string]any{"name": "Series A", "values": []any{100.0, 120.0, 115.0, 135.0, 125.0, 150.0}},
		map[string]any{"name": "Series B", "values": []any{80.0, 95.0, 105.0, 90.0, 110.0, 100.0}},
	}
	width := 800
	height := 600

	return map[string]*RequestEnvelope{
		"bar_chart": {
			Type:  "bar_chart",
			Title: "Consistency Test",
			Data: map[string]any{
				"categories": categories,
				"series":     series,
			},
			Output: OutputSpec{Width: width, Height: height},
			Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
		},
		"line_chart": {
			Type:  "line_chart",
			Title: "Consistency Test",
			Data: map[string]any{
				"categories": categories,
				"series":     series,
			},
			Output: OutputSpec{Width: width, Height: height},
			Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
		},
		"area_chart": {
			Type:  "area_chart",
			Title: "Consistency Test",
			Data: map[string]any{
				"categories": categories,
				"series":     series,
			},
			Output: OutputSpec{Width: width, Height: height},
			Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
		},
		"stacked_bar_chart": {
			Type:  "stacked_bar_chart",
			Title: "Consistency Test",
			Data: map[string]any{
				"categories": categories,
				"series":     series,
			},
			Output: OutputSpec{Width: width, Height: height},
			Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
		},
		"grouped_bar_chart": {
			Type:  "grouped_bar_chart",
			Title: "Consistency Test",
			Data: map[string]any{
				"categories": categories,
				"series":     series,
			},
			Output: OutputSpec{Width: width, Height: height},
			Style:  StyleSpec{ShowGrid: true, ShowLegend: true},
		},
	}
}

// TestConsistency_TitlePosition verifies that the title text y-position is
// consistent (within 2px) across all Cartesian chart types when rendered with
// identical data and dimensions.
func TestConsistency_TitlePosition(t *testing.T) {
	charts := cartesianChartTypes()

	titleYPositions := make(map[string]float64)

	for chartType, req := range charts {
		doc, err := Render(req)
		if err != nil {
			t.Fatalf("failed to render %s: %v", chartType, err)
		}

		svg := string(doc.Content)

		// Extract the y-position of the title text element.
		// The title is typically the first <text> element with a large font-size.
		titleY := extractTitleYPosition(svg)
		if math.IsNaN(titleY) {
			t.Logf("warning: could not extract title Y position from %s", chartType)
			continue
		}
		titleYPositions[chartType] = titleY
	}

	// Compare all pairs. The title Y should be within 2px across chart types.
	const tolerance = 2.0

	types := make([]string, 0, len(titleYPositions))
	for typ := range titleYPositions {
		types = append(types, typ)
	}

	if len(types) < 2 {
		t.Skip("not enough chart types with extractable title positions to compare")
	}

	referenceType := types[0]
	referenceY := titleYPositions[referenceType]

	for _, typ := range types[1:] {
		y := titleYPositions[typ]
		diff := math.Abs(y - referenceY)
		if diff > tolerance {
			t.Errorf("title y-position inconsistency: %s (y=%.1f) vs %s (y=%.1f), diff=%.1f > tolerance=%.1f",
				referenceType, referenceY, typ, y, diff, tolerance)
		}
	}
}

// TestConsistency_ViewBoxDimensions verifies that all Cartesian chart types
// produce the same viewBox dimensions when given the same output spec.
func TestConsistency_ViewBoxDimensions(t *testing.T) {
	charts := cartesianChartTypes()

	viewBoxes := make(map[string]viewBoxDims)

	for chartType, req := range charts {
		doc, err := Render(req)
		if err != nil {
			t.Fatalf("failed to render %s: %v", chartType, err)
		}

		svg := string(doc.Content)
		vb := extractViewBox(svg)
		if math.IsNaN(vb.width) || math.IsNaN(vb.height) {
			t.Logf("warning: could not extract viewBox from %s", chartType)
			continue
		}
		viewBoxes[chartType] = vb
	}

	types := make([]string, 0, len(viewBoxes))
	for typ := range viewBoxes {
		types = append(types, typ)
	}

	if len(types) < 2 {
		t.Skip("not enough chart types with extractable viewBox to compare")
	}

	referenceType := types[0]
	refVB := viewBoxes[referenceType]

	for _, typ := range types[1:] {
		vb := viewBoxes[typ]
		if vb.width != refVB.width || vb.height != refVB.height {
			t.Errorf("viewBox dimension mismatch: %s (%gx%g) vs %s (%gx%g)",
				referenceType, refVB.width, refVB.height,
				typ, vb.width, vb.height)
		}
	}
}

// TestConsistency_AllTypesProduceValidSVG verifies that every Cartesian chart
// type produces valid SVG with all expected structural elements.
func TestConsistency_AllTypesProduceValidSVG(t *testing.T) {
	charts := cartesianChartTypes()

	for chartType, req := range charts {
		t.Run(chartType, func(t *testing.T) {
			doc, err := Render(req)
			if err != nil {
				t.Fatalf("failed to render %s: %v", chartType, err)
			}

			svg := string(doc.Content)

			// Basic structural checks
			if !strings.Contains(svg, "<svg") {
				t.Error("missing <svg> element")
			}
			if !strings.Contains(svg, "viewBox") {
				t.Error("missing viewBox attribute")
			}
			if !strings.Contains(svg, "Consistency Test") {
				t.Error("title text not found in SVG output")
			}

			// Run quality assertions
			AssertSVGQuality(t, svg)
		})
	}
}

// TestConsistency_SameDataDifferentTypes verifies that rendering the same data
// as different chart types does not produce any quality issues.
func TestConsistency_SameDataDifferentTypes(t *testing.T) {
	// Render bar, line, area, stacked with same data and check quality on all.
	charts := cartesianChartTypes()

	for chartType, req := range charts {
		t.Run("quality_"+chartType, func(t *testing.T) {
			doc, err := Render(req)
			if err != nil {
				t.Fatalf("failed to render %s: %v", chartType, err)
			}

			issues := collectSVGQualityIssues(string(doc.Content))
			for _, issue := range issues {
				t.Errorf("[%s quality: %s] %s", chartType, issue.Category, issue.Message)
			}
		})
	}
}

// TestConsistency_WaterfallVsBar verifies that a waterfall chart at the same
// dimensions as a bar chart produces an SVG with similar structural properties
// (viewBox, title presence).
func TestConsistency_WaterfallVsBar(t *testing.T) {
	barReq := &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Comparison Test",
		Data: map[string]any{
			"categories": []any{"A", "B", "C", "D"},
			"series": []any{
				map[string]any{"name": "S1", "values": []any{100.0, 120.0, 80.0, 150.0}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowValues: true},
	}

	waterfallReq := &RequestEnvelope{
		Type:  "waterfall",
		Title: "Comparison Test",
		Data: map[string]any{
			"title": "Comparison Test",
			"points": []any{
				map[string]any{"label": "A", "value": 100, "type": "increase"},
				map[string]any{"label": "B", "value": 20, "type": "increase"},
				map[string]any{"label": "C", "value": -40, "type": "decrease"},
				map[string]any{"label": "D", "value": 80, "type": "total"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true, ShowValues: true},
	}

	barDoc, err := Render(barReq)
	if err != nil {
		t.Fatalf("failed to render bar chart: %v", err)
	}
	waterfallDoc, err := Render(waterfallReq)
	if err != nil {
		t.Fatalf("failed to render waterfall: %v", err)
	}

	barVB := extractViewBox(string(barDoc.Content))
	wfVB := extractViewBox(string(waterfallDoc.Content))

	// ViewBox dimensions should match since both use 800x600 output
	if barVB.width != wfVB.width || barVB.height != wfVB.height {
		t.Errorf("viewBox mismatch: bar(%gx%g) vs waterfall(%gx%g)",
			barVB.width, barVB.height, wfVB.width, wfVB.height)
	}

	// Both should pass quality
	AssertSVGQuality(t, string(barDoc.Content))
	AssertSVGQuality(t, string(waterfallDoc.Content))
}

// TestConsistency_ScatterVsOtherCartesian verifies that a scatter chart at the
// same dimensions produces structurally consistent SVG.
func TestConsistency_ScatterVsOtherCartesian(t *testing.T) {
	scatterReq := &RequestEnvelope{
		Type:  "scatter_chart",
		Title: "Consistency Test",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":     "Data",
					"values":   []any{100.0, 120.0, 115.0, 135.0, 125.0, 150.0},
					"x_values": []any{1.0, 2.0, 3.0, 4.0, 5.0, 6.0},
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowGrid: true},
	}

	doc, err := Render(scatterReq)
	if err != nil {
		t.Fatalf("failed to render scatter chart: %v", err)
	}

	svg := string(doc.Content)
	AssertSVGQuality(t, svg)

	// Verify viewBox has positive dimensions.
	// Note: scatter chart may auto-scale viewBox dimensions via the canvas
	// library's fit mode, so we do not assert exact 800x600 here.
	vb := extractViewBox(svg)
	if vb.width <= 0 || vb.height <= 0 {
		t.Errorf("scatter viewBox has non-positive dimensions: %gx%g", vb.width, vb.height)
	}
}

// =============================================================================
// Helper functions
// =============================================================================

// extractTitleYPosition attempts to find the y-coordinate of the chart title.
// It looks for a <text> element containing "Consistency Test" and extracts its y.
func extractTitleYPosition(svg string) float64 {
	// Strategy: Find text elements that contain the title string, then extract
	// the y coordinate from the nearest enclosing <text> tag.

	// Look for pattern: <text ...y="N"...>...Consistency Test...</text>
	// We need to handle that the title might be in a <tspan> inside <text>.

	titleIdx := strings.Index(svg, "Consistency Test")
	if titleIdx == -1 {
		return math.NaN()
	}

	// Search backwards from the title text to find the enclosing <text tag
	prefix := svg[:titleIdx]
	textTagStart := strings.LastIndex(prefix, "<text")
	if textTagStart == -1 {
		return math.NaN()
	}

	// Find the end of the <text> opening tag
	tagEnd := strings.Index(svg[textTagStart:], ">")
	if tagEnd == -1 {
		return math.NaN()
	}

	textTag := svg[textTagStart : textTagStart+tagEnd+1]

	// Extract y attribute
	yRe := regexp.MustCompile(`\by\s*=\s*"([^"]+)"`)
	match := yRe.FindStringSubmatch(textTag)
	if match == nil {
		return math.NaN()
	}

	y, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	if err != nil {
		return math.NaN()
	}

	return y
}

type viewBoxDims struct {
	minX, minY, width, height float64
}

// extractViewBox parses the viewBox attribute from SVG content.
func extractViewBox(svg string) viewBoxDims {
	vbRe := regexp.MustCompile(`viewBox\s*=\s*"([\d.\-]+)\s+([\d.\-]+)\s+([\d.\-]+)\s+([\d.\-]+)"`)
	match := vbRe.FindStringSubmatch(svg)
	if match == nil {
		return viewBoxDims{math.NaN(), math.NaN(), math.NaN(), math.NaN()}
	}

	minX, _ := strconv.ParseFloat(match[1], 64)
	minY, _ := strconv.ParseFloat(match[2], 64)
	width, _ := strconv.ParseFloat(match[3], 64)
	height, _ := strconv.ParseFloat(match[4], 64)

	return viewBoxDims{minX, minY, width, height}
}
