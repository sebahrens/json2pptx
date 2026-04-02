package svggen

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"testing"
)

func TestScatterChartContentCoverage(t *testing.T) {
	// Test at various dimensions to find which ones have poor coverage
	dims := []struct {
		w, h int
		desc string
	}{
		{800, 600, "standard 4:3"},
		{1200, 400, "very wide"},
		{1200, 900, "large"},
		{960, 540, "16:9"},
		{700, 500, "typical placeholder"},
		{720, 540, "standard slide"},
	}

	for _, dim := range dims {
		t.Run(fmt.Sprintf("%dx%d_%s", dim.w, dim.h, dim.desc), func(t *testing.T) {
			d := &ScatterChartDiagram{NewBaseDiagram("scatter_chart")}
			req := &RequestEnvelope{
				Type:  "scatter_chart",
				Title: "Initiative Prioritization Matrix",
				Data: map[string]any{
					"series": []any{
						map[string]any{
							"name":   "Initiatives",
							"points": []any{
								map[string]any{"x": 9.0, "y": 4.6, "label": "Legacy Migration"},
								map[string]any{"x": 8.0, "y": 5.7, "label": "Reporting"},
								map[string]any{"x": 6.0, "y": 6.7, "label": "Admin Portal"},
								map[string]any{"x": 5.0, "y": 7.7, "label": "Integrations"},
								map[string]any{"x": 4.5, "y": 8.6, "label": "Analytics"},
								map[string]any{"x": 7.0, "y": 9.7, "label": "Mobile App"},
								map[string]any{"x": 3.0, "y": 10.7, "label": "API Platform"},
							},
						},
					},
				},
				Output: OutputSpec{
					Width:  dim.w,
					Height: dim.h,
				},
				Style: StyleSpec{ShowValues: true},
			}

			doc, err := d.Render(req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			svg := string(doc.Content)

			// Find all Y coordinates in the SVG to determine content bounds
			yCoords := extractYCoordinates(svg)
			if len(yCoords) == 0 {
				t.Fatal("No Y coordinates found in SVG")
			}

			minY := math.MaxFloat64
			maxY := 0.0
			for _, y := range yCoords {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}

			contentH := maxY - minY
			coverage := contentH / doc.Height * 100

			t.Logf("SVG dimensions: %.0fx%.0f", doc.Width, doc.Height)
			t.Logf("Content Y range: %.1f to %.1f (span=%.1f)", minY, maxY, contentH)
			t.Logf("Content vertical coverage: %.1f%%", coverage)

			if coverage < 60 {
				t.Errorf("Content only covers %.1f%% of canvas height (expected >60%%)", coverage)
			}
		})
	}
}

// TestScatterAxisTicksAlignWithGrid verifies that the Y-axis tick labels
// appear at the same Y positions as the horizontal grid lines.  A previous
// bug had the scatter chart's axis ticks shifted because the shared axis
// drawing code added plotArea.Y to already-absolute scale positions.
func TestScatterAxisTicksAlignWithGrid(t *testing.T) {
	d := &ScatterChartDiagram{NewBaseDiagram("scatter_chart")}
	req := &RequestEnvelope{
		Type:  "scatter_chart",
		Title: "Axis Alignment Test",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":   "A",
					"points": []any{
						map[string]any{"x": 10.0, "y": 25.0},
						map[string]any{"x": 30.0, "y": 75.0},
						map[string]any{"x": 50.0, "y": 50.0},
						map[string]any{"x": 70.0, "y": 90.0},
						map[string]any{"x": 90.0, "y": 15.0},
					},
				},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	doc, err := d.Render(req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(doc.Content)

	// Horizontal grid lines are <path> elements with horizontal moves
	// (same Y for start and end): M<x1> <y>H<x2> or M<x1>,<y> ... <x2>,<y>
	gridLineRe := regexp.MustCompile(`M([\d.]+)\s+([\d.]+)H([\d.]+)`)
	gridMatches := gridLineRe.FindAllStringSubmatch(svg, -1)
	if len(gridMatches) == 0 {
		t.Fatal("No horizontal grid lines found in SVG")
	}

	gridYs := make(map[float64]bool)
	for _, m := range gridMatches {
		y, _ := strconv.ParseFloat(m[2], 64)
		gridYs[y] = true
	}

	// Y-axis tick labels are <text><tspan> elements to the left of the grid
	// (small X), with numeric content and varying Y coordinates.
	// Extract all numeric <tspan> elements with their coordinates.
	tspanRe := regexp.MustCompile(`<tspan[^>]*\bx="([\d.]+)"[^>]*\by="([\d.]+)"[^>]*>(-?\d+\.?\d*)</tspan>`)
	tspanMatches := tspanRe.FindAllStringSubmatch(svg, -1)

	// The grid's left edge (plotArea.X) is the start X of horizontal grid lines.
	plotX := 0.0
	if len(gridMatches) > 0 {
		plotX, _ = strconv.ParseFloat(gridMatches[0][1], 64)
	}

	// Y-axis labels sit to the left of plotArea.X.
	var tickYs []float64
	for _, m := range tspanMatches {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		if x < plotX {
			tickYs = append(tickYs, y)
		}
	}

	if len(tickYs) == 0 {
		t.Fatal("No Y-axis tick labels found in SVG")
	}

	// Each Y-axis tick label should have a grid line at the same Y (within
	// tolerance).  Tick labels have a text-baseline offset (~4-6px for
	// TextBaselineMiddle at typical font sizes), so we allow up to 8px.
	// The pre-fix double-offset bug produced shifts of plotArea.Y (~88px),
	// so 8px still catches regressions.
	const tol = 8.0
	for _, ty := range tickYs {
		found := false
		for gy := range gridYs {
			if math.Abs(ty-gy) < tol {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Y-axis tick at y=%.1f has no matching grid line (grid Ys: %v)", ty, gridYKeys(gridYs))
		}
	}

	// Grid lines and tick labels should have equal counts.  Before the fix,
	// the double-offset caused some ticks to be filtered by RangeExtent,
	// producing a mismatch (e.g. 11 grid lines but only 5 tick labels).
	if len(gridYs) != len(tickYs) {
		t.Errorf("grid line count (%d) != tick label count (%d)", len(gridYs), len(tickYs))
	}

	t.Logf("Grid lines: %d, Tick labels: %d", len(gridYs), len(tickYs))
}

func gridYKeys(m map[float64]bool) []float64 {
	keys := make([]float64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractYCoordinates extracts all Y coordinates from SVG path data and attributes
func extractYCoordinates(svg string) []float64 {
	var coords []float64

	// Extract Y from M/L/C commands in path d= attributes
	pathRe := regexp.MustCompile(`[MLCmlcHVhvSsQqTtAa][\d\s.,eE+-]+`)
	moveRe := regexp.MustCompile(`[ML]\s*([\d.eE+-]+)\s*[,\s]\s*([\d.eE+-]+)`)
	matches := moveRe.FindAllStringSubmatch(svg, -1)
	for _, m := range matches {
		if y, err := strconv.ParseFloat(m[2], 64); err == nil {
			coords = append(coords, y)
		}
	}

	// Also look for y= attributes
	yAttrRe := regexp.MustCompile(`\by="([\d.eE+-]+)"`)
	yMatches := yAttrRe.FindAllStringSubmatch(svg, -1)
	for _, m := range yMatches {
		if y, err := strconv.ParseFloat(m[1], 64); err == nil {
			coords = append(coords, y)
		}
	}

	// Also cy= for circles
	cyRe := regexp.MustCompile(`\bcy="([\d.eE+-]+)"`)
	cyMatches := cyRe.FindAllStringSubmatch(svg, -1)
	for _, m := range cyMatches {
		if y, err := strconv.ParseFloat(m[1], 64); err == nil {
			coords = append(coords, y)
		}
	}

	_ = pathRe
	return coords
}
