package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// stackedAreaSeries is the review specimen: two series whose cumulative
// boundaries (77, 68, 52, 34) differ from every authored value.
func stackedAreaSeries(extra ...map[string]any) map[string]any {
	series := []any{
		map[string]any{"name": "Current year", "values": []any{42.0, 36.0, 28.0, 18.0}},
		map[string]any{"name": "Prior year", "values": []any{35.0, 32.0, 24.0, 16.0}},
	}
	for _, e := range extra {
		series = append(series, e)
	}
	return map[string]any{
		"categories": []any{"North America", "Europe", "Asia Pacific", "Latin America"},
		"series":     series,
	}
}

// svgTexts returns the text content of every tspan in an SVG.
func svgTexts(svg string) []string {
	re := regexp.MustCompile(`<tspan[^>]*>([^<]*)</tspan>`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		out = append(out, m[1])
	}
	return out
}

// go-slide-creator-b7qqg.18: stacked_area labelled each band with its
// cumulative boundary (Prior year 35 printed as 77) while the legend named the
// individual series. Labels must show the authored contribution.
func TestStackedArea_ValueLabelsShowAuthoredValues(t *testing.T) {
	cases := []struct {
		name   string
		data   map[string]any
		want   []string
		absent []string
	}{
		{
			name:   "two series",
			data:   stackedAreaSeries(),
			want:   []string{"42", "36", "28", "18", "35", "32", "24", "16"},
			absent: []string{"77", "68", "52", "34"},
		},
		{
			name: "three series with a zero contribution",
			data: stackedAreaSeries(map[string]any{"name": "Forecast", "values": []any{7.0, 0.0, 5.0, 3.0}}),
			// Cumulative tops of the third band: 84, 68, 57, 37.
			want:   []string{"42", "35", "7", "0", "5", "3"},
			absent: []string{"77", "84", "57", "37", "52"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Render(&RequestEnvelope{
				Type:  "stacked_area_chart",
				Style: StyleSpec{ShowValues: true},
				Data:  tc.data,
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			texts := map[string]int{}
			for _, s := range svgTexts(string(doc.Content)) {
				texts[s]++
			}
			for _, w := range tc.want {
				if texts[w] == 0 {
					t.Errorf("label %q missing; texts = %v", w, texts)
				}
			}
			for _, a := range tc.absent {
				if texts[a] > 0 {
					t.Errorf("cumulative total %q rendered as a series label; texts = %v", a, texts)
				}
			}
		})
	}
}

var svgColorRE = regexp.MustCompile(`(?:fill|stroke)(?::|=")(#[0-9a-fA-F]{6}|#[0-9a-fA-F]{3})\b`)

// normHex lower-cases a colour and expands the minified #rgb form.
func normHex(s string) string {
	if c, err := ParseColor(s); err == nil {
		return strings.ToLower(c.Hex())
	}
	return strings.ToLower(s)
}

// textElement returns the SVG before the <text> element whose body is name,
// and that element's own fill colour.
func textElement(t *testing.T, svg, name string) (before, fill string) {
	t.Helper()
	idx := strings.LastIndex(svg, ">"+name+"<")
	if idx < 0 {
		t.Fatalf("no text %q in SVG", name)
	}
	before = svg[:idx]
	start := strings.LastIndex(before, "<text")
	if start < 0 {
		t.Fatalf("text %q is not inside a <text> element", name)
	}
	if m := svgColorRE.FindAllStringSubmatch(before[start:], -1); len(m) > 0 {
		fill = normHex(m[len(m)-1][1])
	}
	return before[:start], fill
}

// seriesIdentityColor returns the colour a chart gives the series called name:
// a direct label's own ink when it is coloured, else the legend swatch / line
// drawn immediately before the legend text.
func seriesIdentityColor(t *testing.T, svg, name, neutralLabel string) string {
	t.Helper()
	_, neutral := textElement(t, svg, neutralLabel)
	before, fill := textElement(t, svg, name)
	if fill != "" && fill != neutral {
		return fill
	}
	matches := svgColorRE.FindAllStringSubmatch(before, -1)
	if len(matches) == 0 {
		t.Fatalf("no colour before legend text %q", name)
	}
	return normHex(matches[len(matches)-1][1])
}

// go-slide-creator-b7qqg.19: stacked_area reversed its paint order BEFORE the
// palette was allocated, so the first series took the last series' colour —
// the same deck coloured "Current year" blue in a bar chart and orange in a
// stacked area. Identity colours must match the other Cartesian types.
func TestStackedArea_SeriesColorsMatchOtherCartesianTypes(t *testing.T) {
	for _, colors := range [][]any{nil, {"#112233", "#aa5500", "#339966"}} {
		data := func() map[string]any {
			d := stackedAreaSeries(map[string]any{"name": "Forecast", "values": []any{7.0, 4.0, 5.0, 3.0}})
			if colors != nil {
				d["colors"] = colors
			}
			return d
		}
		render := func(ct string) string {
			doc, err := Render(&RequestEnvelope{
				Type: ct,
				Data: data(),
			})
			if err != nil {
				t.Fatalf("render %s: %v", ct, err)
			}
			return string(doc.Content)
		}
		ref := render("grouped_bar_chart")
		for _, ct := range []string{"line_chart", "area_chart", "stacked_bar_chart", "stacked_area_chart"} {
			got := render(ct)
			for _, name := range []string{"Current year", "Prior year", "Forecast"} {
				want, have := seriesIdentityColor(t, ref, name, "Europe"), seriesIdentityColor(t, got, name, "Europe")
				// A direct label may carry a readable shade of the series
				// colour (go-slide-creator-vi6uq).
				shade := want
				if c, err := ParseColor(want); err == nil {
					shade = strings.ToLower(directLabelInk(DefaultStyleGuide().Palette, c).Hex())
				}
				if have != want && have != shade {
					t.Errorf("colors=%v %s: %q is %s, grouped_bar draws it %s", colors, ct, name, have, want)
				}
			}
		}
	}
}
