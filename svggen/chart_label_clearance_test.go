package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// svgTextAttrRe captures a <text> element's attributes and its content.
var svgTextAttrRe = regexp.MustCompile(`(?s)<text([^>]*)>(.*?)</text>`)

// svgTextAttr returns attribute name of the first <text> whose content is
// text (tags stripped), or "" when there is none.
func svgTextAttr(svg, text, name string) (string, bool) {
	attrRe := regexp.MustCompile(`\b` + name + `="([^"]*)"`)
	styleRe := regexp.MustCompile(`[;"\s]` + name + `:\s*([^;"]+)`)
	for _, m := range svgTextAttrRe.FindAllStringSubmatch(svg, -1) {
		if strings.TrimSpace(g3TagRe.ReplaceAllString(m[2], "")) != text {
			continue
		}
		if a := attrRe.FindStringSubmatch(m[1]); a != nil {
			return a[1], true
		}
		if s := styleRe.FindStringSubmatch(m[1]); s != nil {
			return strings.TrimSpace(s[1]), true
		}
		return "", true
	}
	return "", false
}

// go-slide-creator-yiznx: a negative horizontal bar's value label sits left
// of the bar end, and the most negative bar ends at the plot edge. The
// category names were anchored to that same edge, so "North" and "−0.5"
// shared one slot (79% overlap). Names and negative values now get separate
// columns across sign mixes, name lengths, chart kinds and label formats.
func TestHorizontalBar_NegativeValueLabelsClearCategoryNames(t *testing.T) {
	short := []any{"East", "South", "North"}
	long := []any{"Enterprise accounts in EMEA", "Mid-market North America", "Public sector Asia Pacific"}
	one := func(cats []any, vals ...any) map[string]any {
		return singleBar(cats, vals, map[string]any{"orientation": "horizontal"})
	}
	two := func(a, b []any) map[string]any {
		return map[string]any{"orientation": "horizontal", "categories": short, "series": []any{
			map[string]any{"name": "FY25", "values": a},
			map[string]any{"name": "FY26", "values": b},
		}}
	}
	cases := []struct {
		name, typ string
		data      map[string]any
		format    string
	}{
		{"mixed", "bar_chart", one(short, 3.0, 2.5, -0.5), ""},
		{"mixed_highlight", "bar_chart", func() map[string]any {
			d := one(short, 3.0, 2.5, -0.5)
			d["highlight"] = []any{"North"}
			return d
		}(), ""},
		{"all_negative", "bar_chart", one(short, -3.0, -2.5, -0.5), ""},
		{"shallow_negative", "bar_chart", one([]any{"East", "South", "North", "West"}, 40.0, 25.0, -0.2, -0.1), ""},
		{"long_names", "bar_chart", one(long, 3.0, 2.5, -1.5), ""},
		{"percent", "bar_chart", one(short, 31.0, 12.0, -7.5), "%.1f%%"},
		{"currency", "bar_chart", one(short, 3.0, 2.5, -0.5), "$%.2fm"},
		{"grouped", "grouped_bar_chart", two([]any{3.0, 2.5, -0.5}, []any{2.0, -1.25, -2.0}), ""},
		{"stacked", "stacked_bar_chart", two([]any{3.0, 2.5, -0.5}, []any{1.0, -1.5, -2.0}), ""},
	}
	sizes := []OutputSpec{{Width: 800, Height: 500}, {Width: 520, Height: 300}}
	for _, tc := range cases {
		for _, size := range sizes {
			if tc.format != "" {
				tc.data["data_labels"] = map[string]any{"format": tc.format}
			}
			req := &RequestEnvelope{Type: tc.typ, Data: tc.data, Output: size}
			findings, err := DryRender(req)
			if err != nil {
				t.Fatalf("%s %vx%v: %v", tc.name, size.Width, size.Height, err)
			}
			for _, f := range findings {
				if f.Code == FindingDiagramTextOverlap {
					t.Errorf("%s %vx%v: %s", tc.name, size.Width, size.Height, f.Message)
				}
			}
		}
	}

	// The control keeps every value with its sign, and the category name
	// sits a clear gap left of the negative label's column.
	svg := renderG3(t, &RequestEnvelope{Type: "bar_chart", Data: one(short, 3.0, 2.5, -0.5)})
	texts := g3Texts(svg)
	for _, want := range []string{"3.0", "2.5", "−0.5", "East", "South", "North"} {
		if indexOfText(texts, want) < 0 {
			t.Errorf("missing %q: %v", want, texts)
		}
	}
	nameX, _ := svgTextAttr(svg, "North", "x")
	valueX, _ := svgTextAttr(svg, "−0.5", "x")
	nx, _ := strconv.ParseFloat(nameX, 64)
	vx, _ := strconv.ParseFloat(valueX, 64)
	// SVG units per point, read off the value label's emitted font size.
	var scale float64
	for _, r := range g3TextRuns(svg) {
		if r.text == "−0.5" {
			scale = r.sizePx / labelledValueFontPt
		}
	}
	m := NewSVGBuilder(800, 500)
	m.SetFontSize(labelledValueFontPt).SetFontWeight(m.StyleGuide().Typography.WeightBold)
	w, _ := m.MeasureText("−0.5")
	// Both are right-anchored: the value's left edge must clear the name's
	// right edge by at least the name-to-bar gap.
	left := vx - w*scale
	if gap := left - nx; scale == 0 || gap < hbarLabelGapPt*scale {
		t.Errorf("North ends at x=%.1f, −0.5 starts at x=%.1f: gap %.1f units, want ≥ %.1f", nx, left, gap, hbarLabelGapPt*scale)
	}
}

// go-slide-creator-vi6uq: a direct series label is text, so it must read at
// WCAG AA on the chart background even when the series colour (a template
// accent or an explicit palette / series colour) is only fit for a stroke.
// The label keeps the series hue and the line keeps the original colour.
func TestLineDirectLabels_ReadableInkKeepsSeriesColour(t *testing.T) {
	data := func() map[string]any {
		return map[string]any{"categories": []any{"Q1", "Q2", "Q3", "Q4"}, "series": []any{
			map[string]any{"name": "Demand", "values": []any{80.0, 90.0, 100.0, 120.0}},
			map[string]any{"name": "Current capacity", "values": []any{100.0, 100.0, 100.0, 100.0}},
		}}
	}
	cases := []struct {
		name, background string
		colors           []string
		highlight        any
	}{
		{"pstyle_accents", "", []string{"#FD5108", "#D89060"}, nil},
		{"explicit_light", "", []string{"#FFD54F", "#9ED6F0"}, nil},
		{"highlighted", "", []string{"#FD5108", "#D89060"}, "Current capacity"},
		{"dark_background", "#1B1F3A", []string{"#2E3A8C", "#4A2C2A"}, nil},
		{"default_palette", "", nil, nil},
	}
	for _, tc := range cases {
		d := data()
		if tc.highlight != nil {
			d["highlight"] = tc.highlight
		}
		style := StyleSpec{Background: tc.background, Palette: PaletteSpec{Colors: tc.colors}}
		svg := strings.ToLower(renderG3(t, &RequestEnvelope{Type: "line_chart", Data: d, Style: style}))
		bg := Color{R: 255, G: 255, B: 255, A: 1}
		if tc.background != "" {
			bg, _ = ParseColor(tc.background)
		}
		for _, name := range []string{"demand", "current capacity"} {
			fill, ok := svgTextAttr(svg, name, "fill")
			if !ok {
				t.Errorf("%s: no direct label %q", tc.name, name)
				continue
			}
			ink, err := ParseColor(fill)
			if err != nil {
				t.Errorf("%s: %q fill %q: %v", tc.name, name, fill, err)
				continue
			}
			if r := ink.ContrastWith(bg); r < WCAGAANormal-0.01 {
				t.Errorf("%s: %q label %s on %s is %.2f:1, want >= %.1f", tc.name, name, fill, bg.Hex(), r, WCAGAANormal)
			}
		}
		// The series strokes keep the palette colour (on white, where no
		// background-contrast pass adjusts the palette itself).
		for _, c := range tc.colors {
			if tc.highlight == nil && tc.background == "" && !strings.Contains(svg, "stroke:"+strings.ToLower(c)) {
				t.Errorf("%s: no line stroked %s", tc.name, c)
			}
		}
	}

	// A programmatic ChartSeries.Color is the label's colour source too.
	b := NewSVGBuilder(400, 300)
	light := Color{R: 0xFF, G: 0xD5, B: 0x4F, A: 1}
	chart := ChartData{Categories: []string{"A", "B"}, Series: []ChartSeries{{Name: "Light", Values: []float64{1, 2}, Color: &light}}}
	x := NewCategoricalScale(chart.Categories)
	x.SetRangeCategorical(0, 300)
	y := NewLinearScale(0, 2)
	y.SetRangeLinear(200, 0)
	if !drawLineDirectSeriesLabels(b, b.StyleGuide(), chart, Rect{X: 20, Y: 20, W: 300, H: 200}, x, y, []Color{light}, 80) {
		t.Fatal("single direct label should fit")
	}
	doc, err := b.Render()
	if err != nil {
		t.Fatal(err)
	}
	fill, _ := svgTextAttr(strings.ToLower(string(doc.Content)), "light", "fill")
	if ink, err := ParseColor(fill); err != nil || ink.ContrastWith(Color{R: 255, G: 255, B: 255, A: 1}) < WCAGAANormal-0.01 {
		t.Errorf("explicit series colour label fill %q is not AA on white", fill)
	}
}
