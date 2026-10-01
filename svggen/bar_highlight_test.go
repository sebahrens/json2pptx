package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// highlightReq builds a single-series bar_chart request on a black-on-white
// theme with an orange accent1, so neutral and accent fills are easy to tell
// apart in the SVG.
func highlightReq(categories []any, values []any, extra map[string]any, showValues bool) *RequestEnvelope {
	data := map[string]any{
		"categories": categories,
		"series":     []any{map[string]any{"name": "Share", "values": values}},
	}
	for k, v := range extra {
		data[k] = v
	}
	return &RequestEnvelope{
		Type:   "bar_chart",
		Data:   data,
		Output: OutputSpec{Width: 800, Height: 500},
		Style: StyleSpec{
			ShowValues: showValues, ShowValuesSet: true,
			ThemeColors: []ThemeColorInput{
				{Name: "dk1", RGB: "#000000"},
				{Name: "lt1", RGB: "#FFFFFF"},
				{Name: "accent1", RGB: "#FD5108"},
				{Name: "accent2", RGB: "#2D6A9F"},
			},
		},
	}
}

// barFills returns the fill of every bar-like rect in render order.
func barFills(t *testing.T, req *RequestEnvelope) []string {
	t.Helper()
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var fills []string
	for _, m := range regexp.MustCompile(`<path d="M[^"]*z" fill="(#[0-9a-f]+)"/>`).FindAllStringSubmatch(string(doc.Content), -1) {
		fills = append(fills, m[1])
	}
	return fills
}

const (
	testAccent  = "#fd5108"
	testNeutral = "#9e9e9e" // dk1 #000 at 38% on white
)

func countFill(fills []string, want string) int {
	n := 0
	for _, f := range fills {
		if f == want {
			n++
		}
	}
	return n
}

func TestBarHighlight_Defaults(t *testing.T) {
	// A time series accents its latest period.
	years := []any{"2023", "2024", "2025", "2026"}
	fills := barFills(t, highlightReq(years, []any{22.0, 35.0, 58.0, 40.0}, nil, true))
	if countFill(fills, testAccent) != 1 || countFill(fills, testNeutral) != 3 {
		t.Fatalf("time series: want 1 accent + 3 neutral bars, got %v", fills)
	}
	if fills[len(fills)-1] != testAccent {
		t.Errorf("time series should accent the last bar, fills %v", fills)
	}

	// Any other chart accents its top bar — first, once the default
	// descending sort has ranked it (go-slide-creator-oocqj).
	funcs := []any{"Marketing", "Engineering", "Legal"}
	fills = barFills(t, highlightReq(funcs, []any{52.0, 71.0, 26.0}, nil, true))
	if len(fills) != 3 || fills[0] != testAccent || fills[1] != testNeutral || fills[2] != testNeutral {
		t.Errorf("ranked chart should accent the top bar only, fills %v", fills)
	}
	// With the author's order kept, the accent stays on the largest bar.
	fills = barFills(t, highlightReq(funcs, []any{52.0, 71.0, 26.0}, map[string]any{"sort": "none"}, true))
	if len(fills) != 3 || fills[1] != testAccent || fills[0] != testNeutral || fills[2] != testNeutral {
		t.Errorf("sort none: the largest (second) bar should be accented, fills %v", fills)
	}
}

func TestBarHighlight_Explicit(t *testing.T) {
	funcs := []any{"Engineering", "Support", "Marketing", "Legal"}
	vals := []any{71.0, 64.0, 52.0, 26.0}

	fills := barFills(t, highlightReq(funcs, vals, map[string]any{"highlight": []any{"engineering", 1.0}}, true))
	if len(fills) != 4 || fills[0] != testAccent || fills[1] != testAccent || fills[2] != testNeutral || fills[3] != testNeutral {
		t.Errorf("highlight by name and index: fills %v", fills)
	}

	fills = barFills(t, highlightReq(funcs, vals, map[string]any{"highlight": []any{}}, true))
	if countFill(fills, testAccent) != 0 || countFill(fills, testNeutral) != 4 {
		t.Errorf("empty highlight should leave every bar neutral, fills %v", fills)
	}

	req := highlightReq(funcs, vals, map[string]any{"highlight": []any{"Finance"}}, true)
	if err := (&BarChartDiagram{NewBaseDiagram("bar_chart")}).Validate(req); err == nil {
		t.Error("a highlight naming no category must fail validation")
	}
	req = highlightReq(funcs, vals, map[string]any{"highlight": []any{4.0}}, true)
	if err := (&BarChartDiagram{NewBaseDiagram("bar_chart")}).Validate(req); err == nil {
		t.Error("an out-of-range highlight index must fail validation")
	}
}

func TestBarHighlight_MultiSeriesKeepsPalette(t *testing.T) {
	req := highlightReq([]any{"A", "B"}, []any{1.0, 2.0}, nil, true)
	req.Data["series"] = []any{
		map[string]any{"name": "X", "values": []any{1.0, 2.0}},
		map[string]any{"name": "Y", "values": []any{3.0, 4.0}},
	}
	fills := barFills(t, req)
	if countFill(fills, testNeutral) != 0 {
		t.Errorf("multi-series charts stay on the series palette, fills %v", fills)
	}
}

func TestBarLabelledMode_DropsValueAxis(t *testing.T) {
	cats := []any{"2023", "2024", "2025", "2026"}
	vals := []any{22.0, 35.0, 58.0, 78.0}
	render := func(show bool) string {
		doc, err := Render(highlightReq(cats, vals, nil, show))
		if err != nil {
			t.Fatal(err)
		}
		return string(doc.Content)
	}
	tickRe := regexp.MustCompile(`>(0|20|40|60|80)</tspan>`)
	if !tickRe.MatchString(render(false)) {
		t.Fatal("an unlabelled chart keeps its value axis")
	}
	labelled := render(true)
	if tickRe.MatchString(labelled) {
		t.Error("a labelled chart must not draw value-axis ticks")
	}
	if !strings.Contains(labelled, "stroke-width:1.00") { // 0.75pt in px
		t.Error("a labelled chart draws a 0.75pt baseline")
	}
	if !strings.Contains(labelled, "font-weight:700") {
		t.Error("the highlighted bar's label is bold")
	}

	// Bars take 60% of their category slot: width / (next left - this left).
	rects := regexp.MustCompile(`<path d="M([0-9.]+) [0-9.]+H([0-9.]+)V[0-9.]+H[0-9.]+z" fill`).FindAllStringSubmatch(labelled, -1)
	if len(rects) < 2 {
		t.Fatalf("bar geometry not found")
	}
	x0, x1, next := atof(t, rects[0][1]), atof(t, rects[0][2]), atof(t, rects[1][1])
	if share := (x1 - x0) / (next - x0); share < 0.58 || share > 0.62 {
		t.Errorf("bar width is %.2f of its slot, want 0.60", share)
	}
}

func atof(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTrueMinus(t *testing.T) {
	cases := map[string]string{
		"-12":     "−12",
		"-$4.5M":  "−$4.5M",
		"-€3":     "−€3",
		"12":      "12",
		"y-o-y 3": "y-o-y 3",
		"-.5":     "−.5",
	}
	for in, want := range cases {
		if got := TrueMinus(in); got != want {
			t.Errorf("TrueMinus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLooksLikeTimeCategories(t *testing.T) {
	yes := [][]string{
		{"2023", "2024", "2025"},
		{"Q1 2025", "Q2 2025", "Q3 2025"},
		{"FY24", "FY25", "FY26E"},
		{"Jan", "Feb", "Mar"},
		{"H1", "H2"},
	}
	for _, c := range yes {
		if !looksLikeTimeCategories(c) {
			t.Errorf("%v should read as a time series", c)
		}
	}
	no := [][]string{
		{"Engineering", "Support"},
		{"10", "20", "30"},
		{"2024", "Other"},
	}
	for _, c := range no {
		if looksLikeTimeCategories(c) {
			t.Errorf("%v should not read as a time series", c)
		}
	}
}
