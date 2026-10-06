package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The chart is the hero of its slide (go-slide-creator-9nk6a, -7bsbd).

var heroFontSizeRe = regexp.MustCompile(`font-size:([0-9.]+)px`)

// drawnFontSizesPt returns every font size an SVG draws, in points. The
// builder writes CSS pixels at 96 dpi, 0.75pt each.
func drawnFontSizesPt(t *testing.T, svg string) []float64 {
	t.Helper()
	var out []float64
	for _, m := range heroFontSizeRe.FindAllStringSubmatch(svg, -1) {
		px, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("font size %q: %v", m[1], err)
		}
		out = append(out, px*0.75)
	}
	return out
}

func heroSeriesData() map[string]any {
	return map[string]any{
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series": []any{
			map[string]any{"name": "North", "values": []any{12.0, 14.0, 15.0, 18.0}},
			map[string]any{"name": "South", "values": []any{8.0, 9.0, 11.0, 12.0}},
			map[string]any{"name": "East", "values": []any{5.0, 7.0, 6.0, 9.0}},
		},
	}
}

// TestPresentationChartTextAtBodyStep renders the charts of a slide body
// placeholder (a wide, short canvas in points) in live-presentation mode: no
// label - tick, data label, legend, treemap or funnel caption - may be under
// the 12pt slide body step. A dense report keeps its smaller label floor.
func TestPresentationChartTextAtBodyStep(t *testing.T) {
	charts := map[string]map[string]any{
		"bar_chart":          heroSeriesData(),
		"stacked_bar_chart":  heroSeriesData(),
		"line_chart":         heroSeriesData(),
		"area_chart":         heroSeriesData(),
		"stacked_area_chart": heroSeriesData(),
		"radar_chart":        heroSeriesData(),
		"pie_chart": {
			"categories": []any{"A", "B", "C"},
			"values":     []any{50.0, 30.0, 20.0},
		},
		"treemap_chart": {"values": []any{
			map[string]any{"label": "Enterprise", "value": 45.0},
			map[string]any{"label": "SMB", "value": 25.0},
			map[string]any{"label": "API", "value": 15.0},
			map[string]any{"label": "Services", "value": 10.0},
			map[string]any{"label": "Training", "value": 5.0},
		}},
		"funnel_chart": {"values": []any{
			map[string]any{"label": "Leads", "value": 10000.0},
			map[string]any{"label": "Qualified", "value": 4500.0},
			map[string]any{"label": "Proposals", "value": 2000.0},
			map[string]any{"label": "Won", "value": 350.0},
		}},
	}
	for chartType, data := range charts {
		t.Run(chartType, func(t *testing.T) {
			render := func(mode string) string {
				doc, err := Render(&RequestEnvelope{
					Type:   chartType,
					Data:   data,
					Output: OutputSpec{Width: 899, Height: 315},
					Style:  StyleSpec{ViewingMode: mode},
				})
				if err != nil {
					t.Fatalf("render %s: %v", chartType, err)
				}
				return string(doc.Content)
			}
			sizes := drawnFontSizesPt(t, render("live-presentation"))
			if len(sizes) == 0 {
				t.Fatal("chart drew no text")
			}
			for _, pt := range sizes {
				if pt < ChartStepBodyPt-0.05 {
					t.Errorf("%s draws %.1fpt text in live-presentation mode, below the %.0fpt body step", chartType, pt, ChartStepBodyPt)
				}
			}
			for _, pt := range drawnFontSizesPt(t, render("dense-report")) {
				if pt < 7 {
					t.Errorf("%s draws %.1fpt text in a dense report", chartType, pt)
				}
			}
		})
	}
}

// TestViewingFloorRaisesEveryTextRole pins the floor itself: live-presentation
// lifts every role under the title to the body step, a dense report only
// lifts small labels to the caption step.
func TestViewingFloorRaisesEveryTextRole(t *testing.T) {
	small := func() *Typography {
		return &Typography{SizeTitle: 14, SizeSubtitle: 11, SizeHeading: 11, SizeBody: 11, SizeSmall: 9, SizeCaption: 10}
	}
	live := small()
	applySmallTextFloor(live, "live-presentation")
	for role, got := range map[string]float64{
		"subtitle": live.SizeSubtitle, "heading": live.SizeHeading, "body": live.SizeBody,
		"small": live.SizeSmall, "caption": live.SizeCaption, "floor": live.ReadableFloor,
	} {
		if got != ChartStepBodyPt {
			t.Errorf("live-presentation %s = %gpt, want %gpt", role, got, ChartStepBodyPt)
		}
	}
	if live.SizeTitle != 14 {
		t.Errorf("title = %g, want it untouched", live.SizeTitle)
	}
	report := small()
	applySmallTextFloor(report, "dense-report")
	if report.SizeSmall != 10 || report.SizeBody != 11 || report.SizeCaption != 10 || report.ReadableFloor != 0 {
		t.Errorf("dense-report typography = %+v, want only small labels lifted to 10pt", *report)
	}
}

// TestOverlappingAreasAreOpaque: several filled areas must not mix where they
// overlap (orange over grey reads as brown), so each fill is flattened onto
// the background and the areas are painted tallest first.
func TestOverlappingAreasAreOpaque(t *testing.T) {
	for _, chartType := range []string{"area_chart", "stacked_area_chart"} {
		doc, err := Render(&RequestEnvelope{Type: chartType, Data: heroSeriesData()})
		if err != nil {
			t.Fatalf("render %s: %v", chartType, err)
		}
		svg := string(doc.Content)
		for _, path := range regexp.MustCompile(`<path[^>]*>`).FindAllString(svg, -1) {
			if !strings.Contains(path, "z\"") && !strings.Contains(path, "Z\"") {
				continue // an open path is a line, not an area
			}
			if strings.Contains(path, "fill-opacity") || strings.Contains(path, `fill="rgba(`) || strings.Contains(path, "fill:rgba(") {
				t.Errorf("%s: a filled area is translucent: %s", chartType, path)
			}
		}
	}
	if got := areaDrawOrder(ChartData{Series: []ChartSeries{
		{Name: "low", Values: []float64{1, 2}}, {Name: "high", Values: []float64{9, 3}}, {Name: "mid", Values: []float64{4, 5}},
	}}); got[0] != 1 || got[1] != 2 || got[2] != 0 {
		t.Errorf("areaDrawOrder = %v, want tallest first [1 2 0]", got)
	}
}

// TestFixedSeriesPaletteKeepsItsOrder: the tonal ladder's neighbours are
// separated by luminance, not hue, so a chart with fewer series than slots
// must take it in order rather than reorder it for hue distance.
func TestFixedSeriesPaletteKeepsItsOrder(t *testing.T) {
	ladder := TonalSeriesLadder(MustParseColor("#FD5108"), MustParseColor("#FFFFFF"), MustParseColor("#000000"))
	hexes := make([]string, len(ladder))
	for i, c := range ladder {
		hexes[i] = c.Hex()
	}
	spec := StyleSpec{
		ThemeColors: []ThemeColorInput{
			{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
			{Name: "accent1", RGB: "#FD5108"}, {Name: "accent2", RGB: "#FE7C39"},
		},
		DataPalette:               hexes,
		DataPaletteFixed:          true,
		DisablePaletteEnforcement: true,
	}
	for count := 2; count <= SeriesLadderLen; count++ {
		got := resolveColors(nil, StyleGuideFromSpec(spec), count)
		for i := range got {
			if got[i] != ladder[i] {
				t.Errorf("%d series: colour %d = %s, want ladder step %s", count, i+1, got[i].Hex(), ladder[i].Hex())
			}
		}
	}
}
