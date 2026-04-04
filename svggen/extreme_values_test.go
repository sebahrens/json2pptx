package svggen_test

import (
	"encoding/json"
	"math"
	"regexp"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// stripBase64Data removes base64-encoded data from SVG content to avoid
// false positives when searching for NaN/Inf.
func stripBase64Data(svg string) string {
	re := regexp.MustCompile(`base64,[A-Za-z0-9+/=]+`)
	return re.ReplaceAllString(svg, "base64,STRIPPED")
}

// stripTextContent removes text content between <tspan>...</tspan> and
// <text>...</text> tags to avoid matching user-provided strings.
func stripTextContent(svg string) string {
	re := regexp.MustCompile(`<tspan[^>]*>[^<]*</tspan>`)
	svg = re.ReplaceAllString(svg, "<tspan/>")
	re2 := regexp.MustCompile(`<text[^>]*>[^<]*</text>`)
	svg = re2.ReplaceAllString(svg, "<text/>")
	return svg
}

// svgHasNaNOrInf checks SVG content for NaN or Inf coordinate values,
// excluding false positives from base64-encoded font data and user text content.
func svgHasNaNOrInf(svg string) (hasNaN, hasInf bool) {
	cleaned := stripBase64Data(svg)
	cleaned = stripTextContent(cleaned)

	nanRe := regexp.MustCompile(`\bNaN\b`)
	infRe := regexp.MustCompile(`[+-]?Inf\b`)

	hasNaN = nanRe.MatchString(cleaned)
	hasInf = infRe.MatchString(cleaned)
	return
}

// TestExtremeValues_WaterfallMaxFloat64 verifies that a waterfall chart with
// float64 max values does not panic or produce NaN in the SVG output.
func TestExtremeValues_WaterfallMaxFloat64(t *testing.T) {
	input := map[string]any{
		"type":  "waterfall",
		"title": "Extreme Values Test",
		"data": map[string]any{
			"points": []any{
				map[string]any{"label": "Start", "value": math.MaxFloat64, "type": "total"},
				map[string]any{"label": "Change", "value": -1e308, "type": "decrease"},
				map[string]any{"label": "End", "value": math.MaxFloat64, "type": "total"},
			},
		},
		"output": map[string]any{
			"width":  800,
			"height": 500,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	req, err := svggen.ParseRequest(data)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	doc, err := svggen.Render(req)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	svg := doc.String()
	hasNaN, hasInf := svgHasNaNOrInf(svg)
	if hasNaN {
		t.Error("SVG output contains NaN coordinates")
	}
	if hasInf {
		t.Error("SVG output contains Inf coordinates")
	}
}

// TestExtremeValues_BarChartNegativeMaxFloat64 verifies that a bar chart with
// negative float64 max values does not produce NaN in SVG paths.
func TestExtremeValues_BarChartNegativeMaxFloat64(t *testing.T) {
	input := map[string]any{
		"type":  "bar_chart",
		"title": "Negative Extreme Values Test",
		"data": map[string]any{
			"categories": []any{"A", "B", "C"},
			"series": []any{
				map[string]any{
					"name":   "Series1",
					"values": []any{-math.MaxFloat64, -1e308, -math.MaxFloat64},
				},
			},
		},
		"output": map[string]any{
			"width":  800,
			"height": 500,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	req, err := svggen.ParseRequest(data)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	doc, err := svggen.Render(req)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	svg := doc.String()
	hasNaN, hasInf := svgHasNaNOrInf(svg)
	if hasNaN {
		t.Error("SVG output contains NaN coordinates")
	}
	if hasInf {
		t.Error("SVG output contains Inf coordinates")
	}
}

// TestExtremeValues_Matrix2x2MaxFloat64 verifies that a matrix_2x2 with
// extreme values in its data does not panic.
func TestExtremeValues_Matrix2x2MaxFloat64(t *testing.T) {
	input := map[string]any{
		"type":  "matrix_2x2",
		"title": "Extreme Matrix Test",
		"data": map[string]any{
			"x_axis": "Risk",
			"y_axis": "Reward",
			"quadrants": []any{
				map[string]any{"label": "Q1", "position": "top_left"},
				map[string]any{"label": "Q2", "position": "top_right"},
				map[string]any{"label": "Q3", "position": "bottom_left"},
				map[string]any{"label": "Q4", "position": "bottom_right"},
			},
			"items": []any{
				map[string]any{"label": "Item A", "x": math.MaxFloat64, "y": math.MaxFloat64},
				map[string]any{"label": "Item B", "x": -math.MaxFloat64, "y": -math.MaxFloat64},
			},
		},
		"output": map[string]any{
			"width":  800,
			"height": 600,
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	req, err := svggen.ParseRequest(data)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	doc, err := svggen.Render(req)
	if err != nil {
		// An error is acceptable (validation failure, etc.) -- a panic is not.
		t.Logf("render returned error (acceptable): %v", err)
		return
	}

	svg := doc.String()
	hasNaN, _ := svgHasNaNOrInf(svg)
	if hasNaN {
		t.Error("SVG output contains NaN coordinates")
	}
}

// TestExtremeValues_PNGRenderRecovery verifies that RenderPNG recovers from
// panics instead of crashing the process. We use RenderMultiFormat with png
// format to exercise this path.
func TestExtremeValues_PNGRenderRecovery(t *testing.T) {
	input := map[string]any{
		"type":  "waterfall",
		"title": "PNG Recovery Test",
		"data": map[string]any{
			"points": []any{
				map[string]any{"label": "Start", "value": 1e15, "type": "total"},
				map[string]any{"label": "Change", "value": -5e14, "type": "decrease"},
				map[string]any{"label": "End", "value": 5e14, "type": "total"},
			},
		},
		"output": map[string]any{
			"width":  800,
			"height": 500,
			"format": "png",
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	req, err := svggen.ParseRequest(data)
	if err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	// This should not panic -- it should either succeed or return an error.
	result, err := svggen.RenderMultiFormat(req, "png")
	if err != nil {
		t.Logf("RenderMultiFormat returned error (acceptable, not a panic): %v", err)
		return
	}

	if result.PNG == nil {
		t.Error("expected PNG bytes, got nil")
	}
}
