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
	matrixRectFill   = regexp.MustCompile(`<path d="M[\d.]+ [\d.]+H[\d.]+V[\d.]+H[\d.]+z" fill="(#[0-9a-f]{6})"`)
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

// TestMatrix2x2_NeutralQuadrantsOneInk pins go-slide-creator-njdno and
// go-slide-creator-ckpye: by default the four quadrants share one light field
// tone (the accent's tint, clearly lighter than the accent) and every point
// one ink; highlight_quadrant deepens exactly one quadrant; points naming a
// series get one colour per series.
func TestMatrix2x2_NeutralQuadrantsOneInk(t *testing.T) {
	svg := renderMatrix(t, effortValueRequest(nil))
	quadrantFills := map[string]bool{}
	accent := DefaultStyleGuide().Palette.Accent1
	for _, m := range matrixRectFill.FindAllStringSubmatch(svg, 4) {
		quadrantFills[m[1]] = true
		if c := MustParseColor(m[1]); c.Luminance() < 0.6 || accent.ContrastWith(c) < matrixFieldAccentMin {
			t.Errorf("quadrant fill %s is not a light field the accent stands out on", m[1])
		}
	}
	if len(quadrantFills) != 1 {
		t.Errorf("default quadrants use %d fills %v, want one field tone", len(quadrantFills), quadrantFills)
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
		if fills[0][1] == fills[1][1] {
			t.Errorf("highlight %v: top-left quadrant is not tinted apart from the others", hi)
		}
		if fills[1][1] != fills[2][1] || fills[2][1] != fills[3][1] {
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

// TestMatrix2x2_AxisBars pins go-slide-creator-ckpye: the quadrants are split
// by a gutter, each axis is one dark bar that carries Low, its title and
// High, and a highlighted quadrant holding a list is the solid accent.
func TestMatrix2x2_AxisBars(t *testing.T) {
	svg := renderMatrix(t, effortValueRequest(nil))
	rects := regexp.MustCompile(`<path d="M([\d.]+) ([\d.]+)H([\d.]+)V([\d.]+)H[\d.]+z" fill="#[0-9a-f]{6}"`).FindAllStringSubmatch(svg, 4)
	if len(rects) != 4 {
		t.Fatalf("found %d quadrant rects", len(rects))
	}
	num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	if gap := num(rects[1][1]) - num(rects[0][3]); gap < 2 {
		t.Errorf("gutter between the top quadrants is %.1f, want a visible gap", gap)
	}
	for _, text := range []string{"Implementation effort", "Business value"} {
		if !regexp.MustCompile(">" + text + "<").MatchString(svg) {
			t.Errorf("axis bars do not carry %q", text)
		}
	}
	for _, end := range []string{matrixAxisLow, matrixAxisHigh} {
		if n := len(regexp.MustCompile(">"+end+"<").FindAllStringIndex(svg, -1)); n != 2 {
			t.Errorf("%s drawn %d times, want once per axis", end, n)
		}
	}

	lists := &RequestEnvelope{Type: "matrix_2x2", Output: OutputSpec{Width: 800, Height: 500}, Data: map[string]any{
		"x_axis_label": "Effort", "y_axis_label": "Value",
		"quadrants": []any{
			map[string]any{"position": "top-left", "title": "Quick wins", "items": []any{"A", "B"}, "highlight": true},
			map[string]any{"position": "top-right", "title": "Major projects", "items": []any{"C"}},
			map[string]any{"position": "bottom-left", "title": "Fill-ins", "items": []any{"D"}},
			map[string]any{"position": "bottom-right", "title": "Time sinks", "items": []any{"E"}},
		},
	}}
	fills := matrixRectFill.FindAllStringSubmatch(renderMatrix(t, lists), 4)
	if len(fills) != 4 || MustParseColor(fills[0][1]).Hex() != DefaultStyleGuide().Palette.Accent1.Hex() {
		t.Errorf("highlighted list quadrant fills = %v, want the solid accent first", fills)
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
