package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// renderPolishSVG renders a request on a slide-body-shaped canvas.
func renderPolishSVG(t *testing.T, req *RequestEnvelope) string {
	t.Helper()
	if req.Output.Width == 0 {
		req.Output = OutputSpec{Width: 1200, Height: 420}
	}
	result, err := RenderMultiFormat(req)
	if err != nil {
		t.Fatalf("%s: render failed: %v", req.Type, err)
	}
	if result == nil || result.SVG == nil {
		t.Fatalf("%s: render returned no SVG", req.Type)
	}
	return string(result.SVG.Content)
}

// polishTextTag returns the opening <text> tag of the element that prints s.
func polishTextTag(t *testing.T, svg, s string) string {
	t.Helper()
	re := regexp.MustCompile(`<text[^>]*>(?:<tspan[^>]*>)?` + regexp.QuoteMeta(s) + `<`)
	m := re.FindString(svg)
	if m == "" {
		t.Fatalf("no text element prints %q", s)
	}
	return m[:strings.Index(m, ">")+1]
}

// go-slide-creator-9nk6a: a chart title is an exhibit heading at the left
// edge, not a centred line, and the unit line heads the chart on its own when
// the slide title carries the chart title.
func TestChartTitleIsALeftAlignedExhibitHeading(t *testing.T) {
	data := map[string]any{"categories": []any{"Q1", "Q2"}, "series": []any{map[string]any{"name": "Rev", "values": []any{1.0, 2.0}}}}
	for _, typ := range []string{"bar_chart", "line_chart", "pie_chart", "radar_chart", "funnel_chart", "gauge_chart", "treemap_chart", "waterfall", "scatter_chart"} {
		req := &RequestEnvelope{Type: typ, Title: "Exhibit heading", Subtitle: "Unit line", Data: data}
		switch typ {
		case "radar_chart":
			req.Data = map[string]any{"categories": []any{"A", "B", "C"}, "series": []any{map[string]any{"name": "S", "values": []any{1.0, 2.0, 3.0}}}}
		case "pie_chart":
			req.Data = map[string]any{"categories": []any{"A", "B"}, "values": []any{3.0, 1.0}}
		case "funnel_chart", "treemap_chart":
			req.Data = map[string]any{"values": []any{map[string]any{"label": "A", "value": 3.0}, map[string]any{"label": "B", "value": 1.0}}}
		case "gauge_chart":
			req.Data = map[string]any{"value": 50.0}
		case "waterfall":
			req.Data = map[string]any{"points": []any{map[string]any{"label": "A", "value": 3.0, "type": "total"}, map[string]any{"label": "B", "value": 1.0}}}
		case "scatter_chart":
			req.Data = map[string]any{"series": []any{map[string]any{"name": "S", "x_values": []any{1.0, 2.0}, "values": []any{1.0, 2.0}}}}
		}
		svg := renderPolishSVG(t, req)
		for _, line := range []string{"Exhibit heading", "Unit line"} {
			if tag := polishTextTag(t, svg, line); strings.Contains(tag, "text-anchor") {
				t.Errorf("%s: %q is not left-aligned: %s", typ, line, tag)
			}
		}
	}

	req := &RequestEnvelope{Type: "bar_chart", Subtitle: "Unit line", Data: data}
	if svg := renderPolishSVG(t, req); !strings.Contains(svg, "Unit line") {
		t.Errorf("the unit line is dropped with the title")
	}
}

// go-slide-creator-cn8mn: the funnel's first stage does not flare across a
// wide body, and the gauge is a bar unless the dial is asked for.
func TestFunnelAndGaugeDefaults(t *testing.T) {
	stages := []any{
		map[string]any{"label": "Leads", "value": 10000.0},
		map[string]any{"label": "Qualified", "value": 4500.0},
		map[string]any{"label": "Won", "value": 350.0},
	}
	if cfg := DefaultFunnelChartConfig(1200, 420); cfg.Style == FunnelStyleTapered {
		t.Fatalf("the tapered funnel is still the default")
	}
	steps := renderPolishSVG(t, &RequestEnvelope{Type: "funnel_chart", Data: map[string]any{"values": stages}})
	tapered := renderPolishSVG(t, &RequestEnvelope{Type: "funnel_chart", Data: map[string]any{"values": stages, "style": "tapered"}})
	if steps == tapered {
		t.Errorf("style: tapered draws the default funnel")
	}
	for _, want := range []string{">Leads<", ">10,000<", "45% of Leads"} {
		if !strings.Contains(steps, want) {
			t.Errorf("default funnel lost %q", want)
		}
	}
	if !strings.Contains(tapered, "Leads: 10,000") {
		t.Errorf("tapered funnel lost its inside label")
	}

	bullet := renderPolishSVG(t, &RequestEnvelope{Type: "gauge_chart", Data: map[string]any{"value": 72.0}})
	dial := renderPolishSVG(t, &RequestEnvelope{Type: "gauge_chart", Data: map[string]any{"value": 72.0, "style": "dial"}})
	angled := renderPolishSVG(t, &RequestEnvelope{Type: "gauge_chart", Data: map[string]any{"value": 72.0, "start_angle": 135.0, "end_angle": 405.0}})
	// The dial labels its five ticks; the bar labels only the ends of its range.
	for _, tick := range []string{">20<", ">40<", ">60<", ">80<"} {
		if strings.Contains(bullet, tick) {
			t.Errorf("default gauge still draws dial tick %s", tick)
		}
		if !strings.Contains(dial, tick) || !strings.Contains(angled, tick) {
			t.Errorf("the dial (style or arc angles) lost tick %s", tick)
		}
	}
	for _, want := range []string{">72<", ">0<", ">100<"} {
		if !strings.Contains(bullet, want) {
			t.Errorf("default gauge lost %s", want)
		}
	}
}

// go-slide-creator-n978t: an area over unordered categories is drawn as bars.
func TestAreaFallsBackToBarsForUnorderedCategories(t *testing.T) {
	series := []ChartSeries{{Name: "Rev", Values: []float64{3, 2, 1}}}
	cases := []struct {
		name       string
		categories []string
		data       map[string]any
		want       bool
	}{
		{"regions", []string{"North America", "Europe", "Asia Pacific"}, map[string]any{}, true},
		{"regions kept", []string{"North America", "Europe", "Asia Pacific"}, map[string]any{"as_area": true}, false},
		{"quarters", []string{"Q1", "Q2", "Q3"}, map[string]any{}, false},
		{"months", []string{"Jan", "Feb", "Mar"}, map[string]any{}, false},
		{"years", []string{"2023", "2024", "2025"}, map[string]any{}, false},
		{"scale", []string{"Low", "Medium", "High"}, map[string]any{}, false},
	}
	for _, c := range cases {
		got := areaFallsBackToBars(&RequestEnvelope{Data: c.data}, ChartData{Categories: c.categories, Series: series})
		if got != c.want {
			t.Errorf("%s: areaFallsBackToBars = %v, want %v", c.name, got, c.want)
		}
	}
}

// go-slide-creator-n978t: a radar's yardstick series is an outline, and the
// treemap spends the accent on its largest tile only.
func TestRadarReferenceSeriesAndTreemapTiles(t *testing.T) {
	ref := radarReferenceSeries([]ChartSeries{{Name: "Team"}, {Name: "Target"}})
	if ref[0] || !ref[1] {
		t.Errorf("radarReferenceSeries(Team, Target) = %v, want [false true]", ref)
	}
	if all := radarReferenceSeries([]ChartSeries{{Name: "Target"}, {Name: "Plan"}}); all[0] || all[1] {
		t.Errorf("a radar of references only must keep a web: %v", all)
	}

	style := DefaultStyleGuide()
	tc := NewTreemapChart(NewSVGBuilder(800, 600), DefaultTreemapChartConfig(800, 600))
	nodes := []*TreemapNode{{Label: "Small", Value: 5}, {Label: "Large", Value: 45}, {Label: "Mid", Value: 25}}
	colors := tc.tileColors(style, nodes)
	accent := style.Palette.AccentColors()[0]
	if colors[1] != accent {
		t.Errorf("largest tile = %s, want the accent %s", colors[1].Hex(), accent.Hex())
	}
	for _, i := range []int{0, 2} {
		if d := deltaE76(colors[i], accent); d < MinSeriesDeltaE {
			t.Errorf("tile %d = %s is not apart from the accent (ΔE %.1f)", i, colors[i].Hex(), d)
		}
	}
	if colors[2].Luminance() >= colors[0].Luminance() {
		t.Errorf("the neutral ladder must lighten with size: mid %s, small %s", colors[2].Hex(), colors[0].Hex())
	}
}

// go-slide-creator-9nk6a: a stack of a few series names its segments beside
// the last column instead of in a legend row.
func TestStackedBarNamesItsSeriesBesideTheLastColumn(t *testing.T) {
	data := ChartData{
		Categories: []string{"Q1", "Q2", "Q3"},
		Series: []ChartSeries{
			{Name: "North", Values: []float64{12, 14, 15}},
			{Name: "South", Values: []float64{8, 9, 11}},
		},
	}
	cfg := DefaultBarChartConfig(1200, 420)
	cfg.Stacked = true
	cfg.PreferDirectLabels = true
	bc := NewBarChart(NewSVGBuilder(1200, 420), cfg)
	if !bc.stackedDirectLabels(data) {
		t.Fatalf("a two-series positive stack must take direct labels")
	}
	data.Series[1].Values[2] = -1
	if bc.stackedDirectLabels(data) {
		t.Errorf("a negative segment in the last column must keep the legend")
	}
	horizontal := cfg
	horizontal.Horizontal = true
	data.Series[1].Values[2] = 11
	if NewBarChart(NewSVGBuilder(1200, 420), horizontal).stackedDirectLabels(data) {
		t.Errorf("a horizontal stack must keep the legend")
	}
}
