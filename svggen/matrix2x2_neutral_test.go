package svggen

import (
	"regexp"
	"strconv"
	"testing"
)

func effortValueRequest(extra map[string]any) *RequestEnvelope {
	data := map[string]any{
		"x_axis_label":    "Implementation effort",
		"y_axis_label":    "Business value",
		"quadrant_labels": []any{"Quick wins", "Big bets", "Fill-ins", "Avoid"},
		"points": []any{
			map[string]any{"label": "Self-serve onboarding", "x": 20.0, "y": 80.0},
			map[string]any{"label": "Payments launch", "x": 85.0, "y": 90.0},
			map[string]any{"label": "Billing migration", "x": 75.0, "y": 35.0},
			map[string]any{"label": "Partner portal", "x": 30.0, "y": 70.0},
			map[string]any{"label": "Legacy API sunset", "x": 60.0, "y": 20.0},
		},
	}
	for k, v := range extra {
		data[k] = v
	}
	return &RequestEnvelope{Type: "matrix_2x2", Title: "Effort vs value", Data: data, Output: OutputSpec{Width: 800, Height: 500}}
}

var (
	matrixRectFill   = regexp.MustCompile(`<path d="M[\d.]+ [\d.]+H[\d.]+V[\d.]+H[\d.]+z" fill="rgba\((\d+,\d+,\d+),([\d.]+)\)"`)
	matrixCircleFill = regexp.MustCompile(`<path d="M[\d.]+ [\d.]+A[^"]*" style="fill:(#[0-9a-f]{6})`)
	matrixYAxisTitle = regexp.MustCompile(`translate\(([\d.]+),[\d.]+\) rotate\(-90\)`)
)

func renderMatrix(t *testing.T, req *RequestEnvelope) string {
	t.Helper()
	doc, err := (&Matrix2x2Diagram{NewBaseDiagram("matrix_2x2")}).Render(req)
	if err != nil {
		t.Fatal(err)
	}
	return doc.String()
}

// TestMatrix2x2_NeutralQuadrantsOneInk pins go-slide-creator-njdno: by default
// the four quadrants share one neutral tint and every point one ink;
// highlight_quadrant tints exactly one quadrant; points naming a series get
// one colour per series.
func TestMatrix2x2_NeutralQuadrantsOneInk(t *testing.T) {
	svg := renderMatrix(t, effortValueRequest(nil))
	quadrantFills := map[string]bool{}
	for _, m := range matrixRectFill.FindAllStringSubmatch(svg, 4) {
		quadrantFills[m[1]+"@"+m[2]] = true
		if a, _ := strconv.ParseFloat(m[2], 64); a > 0.1 {
			t.Errorf("quadrant fill %s at opacity %s is not a light neutral wash", m[1], m[2])
		}
	}
	if len(quadrantFills) != 1 {
		t.Errorf("default quadrants use %d fills %v, want one neutral", len(quadrantFills), quadrantFills)
	}
	pointFills := map[string]bool{}
	for _, m := range matrixCircleFill.FindAllStringSubmatch(svg, -1) {
		pointFills[m[1]] = true
	}
	if len(pointFills) != 1 {
		t.Errorf("points use %d colours %v, want one ink", len(pointFills), pointFills)
	}

	for _, hi := range []any{0.0, "top-left", "Quick wins"} {
		svg := renderMatrix(t, effortValueRequest(map[string]any{"highlight_quadrant": hi}))
		fills := matrixRectFill.FindAllStringSubmatch(svg, 4)
		if len(fills) != 4 {
			t.Fatalf("highlight %v: found %d quadrant rects", hi, len(fills))
		}
		if fills[0][1]+fills[0][2] == fills[1][1]+fills[1][2] {
			t.Errorf("highlight %v: top-left quadrant is not tinted apart from the others", hi)
		}
		if fills[1][1]+fills[1][2] != fills[2][1]+fills[2][2] || fills[2][1]+fills[2][2] != fills[3][1]+fills[3][2] {
			t.Errorf("highlight %v: more than one quadrant tinted: %v", hi, fills)
		}
	}

	series := effortValueRequest(nil)
	pts := series.Data["points"].([]any)
	for i, p := range pts {
		p.(map[string]any)["series"] = []string{"Product", "GTM"}[i%2]
	}
	seriesFills := map[string]bool{}
	for _, m := range matrixCircleFill.FindAllStringSubmatch(renderMatrix(t, series), -1) {
		seriesFills[m[1]] = true
	}
	if len(seriesFills) != 2 {
		t.Errorf("two series drew %d point colours, want 2", len(seriesFills))
	}
}

// TestMatrix2x2_YAxisTitleBesideMatrix pins the y-axis title next to the
// matrix's left edge rather than at the canvas edge.
func TestMatrix2x2_YAxisTitleBesideMatrix(t *testing.T) {
	svg := renderMatrix(t, effortValueRequest(nil))
	m := matrixYAxisTitle.FindStringSubmatch(svg)
	if m == nil {
		t.Fatal("rotated y-axis title not found")
	}
	titleX, _ := strconv.ParseFloat(m[1], 64)
	rect := regexp.MustCompile(`<path d="M([\d.]+) [\d.]+H`).FindStringSubmatch(svg)
	if rect == nil {
		t.Fatal("quadrant rect not found")
	}
	plotLeft, _ := strconv.ParseFloat(rect[1], 64)
	if gap := plotLeft - titleX; gap < 0 || gap > 40 {
		t.Errorf("y-axis title centre is %.1fpx from the matrix edge, want it adjacent (0-40px)", gap)
	}
}
