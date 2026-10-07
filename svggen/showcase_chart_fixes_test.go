package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Fixtures of the 2026-10-07 showcase: the chart shapes that broke in a
// side-by-side compose zone (about half a slide wide), rendered the way a
// presented deck renders them.

// showcaseZone is a horizontal compose half at slide scale, in points (the
// 50% zones of the showcase rendered at 436 x 320).
var showcaseZone = OutputSpec{Width: 436, Height: 320}

func renderShowcase(t *testing.T, typ string, data map[string]any, out OutputSpec) string {
	t.Helper()
	req := &RequestEnvelope{Type: typ, Title: "Exhibit", Data: data, Output: out}
	req.Style.ViewingMode = "live-presentation"
	// A compose zone tells the renderer its size on the slide and the type
	// floor to hold there, as generation does for a grid cell.
	req.Style.PlacementWidthPt = float64(out.Width) * 0.9
	req.Style.PlacementHeightPt = float64(out.Height) * 0.9
	req.Style.MinReadablePt = 12
	return renderPolishSVG(t, req)
}

// showcaseText is one drawn text run with its anchor position.
type showcaseText struct {
	x, y   float64
	anchor string
	size   float64
	text   string
}

var showcaseTextRe = regexp.MustCompile(`<text x="([\d.]+)"(?: text-anchor="(\w+)")? y="([\d.]+)" style="[^"]*font-size:([\d.]+)px[^"]*"><tspan[^>]*>([^<]*)<`)

var showcaseViewBoxRe = regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`)

// showcaseCanvasWidth is the SVG's own width, in the units its text is placed in.
func showcaseCanvasWidth(t *testing.T, svg string) float64 {
	t.Helper()
	m := showcaseViewBoxRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatal("SVG has no viewBox")
	}
	w, _ := strconv.ParseFloat(m[1], 64)
	return w
}

func showcaseTexts(svg string) []showcaseText {
	var out []showcaseText
	for _, m := range showcaseTextRe.FindAllStringSubmatch(svg, -1) {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[3], 64)
		size, _ := strconv.ParseFloat(m[4], 64)
		out = append(out, showcaseText{x: x, y: y, anchor: m[2], size: size, text: m[5]})
	}
	return out
}

// go-slide-creator-47rss: a radar in a half-width zone lists every series in
// its key, not the first one alone.
func TestRadarKeyListsEverySeriesInAHalfWidthZone(t *testing.T) {
	data := map[string]any{
		"categories": []any{"Time to value", "Process fit", "Governance", "Unit cost", "Portability", "Talent need"},
		"series": []any{
			map[string]any{"name": "Embedded in SaaS", "values": []any{5.0, 2.0, 4.0, 3.0, 1.0, 5.0}},
			map[string]any{"name": "Agent platform", "values": []any{4.0, 4.0, 5.0, 4.0, 3.0, 3.0}},
			map[string]any{"name": "Custom build", "values": []any{2.0, 5.0, 3.0, 3.0, 5.0, 1.0}},
		},
	}
	for _, out := range []OutputSpec{showcaseZone, {Width: 520, Height: 320}, {Width: 330, Height: 320}} {
		svg := renderShowcase(t, "radar_chart", data, out)
		// Every name is drawn, and drawn on the canvas: the old legend wrote
		// the second and third rows below the SVG's bottom edge.
		canvasH := 0.0
		if m := showcaseViewBoxRe.FindStringSubmatch(svg); m != nil {
			canvasH, _ = strconv.ParseFloat(m[2], 64)
		}
		drawn := map[string]float64{}
		for _, tx := range showcaseTexts(svg) {
			drawn[tx.text] = tx.y
		}
		for _, name := range []string{"Embedded in SaaS", "Agent platform", "Custom build"} {
			y, ok := drawn[name]
			if !ok {
				t.Errorf("%dx%d: the key does not list %q", out.Width, out.Height, name)
			} else if y > canvasH {
				t.Errorf("%dx%d: %q is drawn at y=%.0f, below the %.0f-high canvas", out.Width, out.Height, name, y, canvasH)
			}
		}
	}
}

// go-slide-creator-fchaq: a point label that would cross the right edge is set
// on another side of its point, in full.
func TestScatterLabelAtTheRightEdgeStaysOnTheCanvas(t *testing.T) {
	point := func(x, y float64, label string) map[string]any {
		return map[string]any{"x": x, "y": y, "label": label}
	}
	data := map[string]any{
		"x_label": "Steps without a human checkpoint", "y_label": "Task success (%)",
		"series": []any{map[string]any{"name": "Workflows", "points": []any{
			point(3, 97, "FAQ answer"), point(12, 90, "Invoice match"), point(25, 81, "Code change"),
			point(34, 74, "Incident fix"), point(49, 66, "Vendor onboarding"),
		}}},
	}
	for _, out := range []OutputSpec{showcaseZone, {Width: 900, Height: 320}} {
		svg := renderShowcase(t, "scatter_chart", data, out)
		canvasW := showcaseCanvasWidth(t, svg)
		found := false
		for _, tx := range showcaseTexts(svg) {
			if tx.text != "Vendor onboarding" {
				continue
			}
			found = true
			// An average glyph is about half an em wide: enough to tell a
			// label that starts 20px from the edge from one that fits.
			width := float64(len(tx.text)) * tx.size * 0.5
			right := tx.x + width
			switch tx.anchor {
			case "end":
				right = tx.x
			case "middle":
				right = tx.x + width/2
			}
			if right > canvasW {
				t.Errorf("%dx%d: label runs to x=%.0f on a %.0f-wide canvas (anchor %q at %.0f)", out.Width, out.Height, right, canvasW, tx.anchor, tx.x)
			}
		}
		if !found {
			t.Errorf("%dx%d: the edge label is not drawn in full", out.Width, out.Height)
		}
	}
}

// go-slide-creator-phenw: long names that do not fit beside a half-width pie
// go to a legend instead of across the slices.
func TestPieWithLongNamesUsesALegendInAHalfWidthZone(t *testing.T) {
	names := []string{"Customer service", "Software engineering", "IT operations", "Finance and procurement", "Sales and marketing", "HR and other"}
	cats := make([]any, len(names))
	for i, n := range names {
		cats[i] = n
	}
	data := map[string]any{"categories": cats, "values": []any{27.0, 24.0, 16.0, 13.0, 11.0, 9.0}}
	for _, typ := range []string{"pie_chart", "donut_chart"} {
		half := renderShowcase(t, typ, data, showcaseZone)
		for _, n := range names {
			if !strings.Contains(half, ">"+n+"<") {
				t.Errorf("%s at half width: %q is not a legend entry of its own", typ, n)
			}
		}
		// A full-width body still has the room for direct labels.
		full := renderShowcase(t, typ, data, OutputSpec{Width: 900, Height: 320})
		if strings.Contains(full, ">Customer service<") {
			t.Errorf("%s at full width fell back to a legend", typ)
		}
	}
}

// go-slide-creator-u3nl6: every zone bound of a gauge is ticked and numbered,
// and a narrow last zone keeps its whole label.
func TestGaugeTicksEveryZoneBoundAndKeepsNarrowZoneLabels(t *testing.T) {
	data := func(style string) map[string]any {
		d := map[string]any{"value": 87.0, "min": 0.0, "max": 100.0, "unit": "%", "thresholds": []any{
			map[string]any{"value": 80.0, "label": "Not ready"},
			map[string]any{"value": 95.0, "label": "Pilot only"},
			map[string]any{"value": 100.0, "label": "Release gate"},
		}}
		if style != "" {
			d["style"] = style
		}
		return d
	}
	for _, out := range []OutputSpec{{Width: 330, Height: 320}, {Width: 900, Height: 320}} {
		bullet := renderShowcase(t, "gauge_chart", data(""), out)
		for _, want := range []string{">80<", ">95<", ">100<", ">Not ready<", ">Pilot only<", ">Release gate<"} {
			if !strings.Contains(bullet, want) {
				t.Errorf("bullet %dx%d: missing %s", out.Width, out.Height, want)
			}
		}
		dial := renderShowcase(t, "gauge_chart", data("dial"), out)
		for _, want := range []string{">80<", ">95<", ">100<"} {
			if !strings.Contains(dial, want) {
				t.Errorf("dial %dx%d: missing tick %s", out.Width, out.Height, want)
			}
		}
	}
}

// go-slide-creator-dd817: eight period labels under a half-width stacked area
// keep a gutter between them, and long series names do not take the plot.
func TestStackedAreaXLabelsKeepAGutterInAHalfWidthZone(t *testing.T) {
	data := map[string]any{
		"categories": []any{"Q1 25", "Q2 25", "Q3 25", "Q4 25", "Q1 26", "Q2 26", "Q3 26", "Q4 26"},
		"series": []any{
			map[string]any{"name": "Human approves each task", "values": []any{0.35, 0.55, 0.8, 1.1, 1.4, 1.7, 1.9, 2.1}},
			map[string]any{"name": "Completed autonomously", "values": []any{0.05, 0.15, 0.4, 0.8, 1.4, 2.4, 3.7, 5.3}},
		},
	}
	svg := renderShowcase(t, "stacked_area_chart", data, OutputSpec{Width: 400, Height: 310})
	var first, last, second *showcaseText
	texts := showcaseTexts(svg)
	for i := range texts {
		switch texts[i].text {
		case "Q1 25", "Q1":
			if first == nil {
				first = &texts[i]
			}
		case "Q2 25", "Q2":
			if second == nil {
				second = &texts[i]
			}
		case "Q4 26", "Q4":
			last = &texts[i]
		}
	}
	if first == nil || second == nil || last == nil {
		t.Fatalf("x labels not found in the SVG")
	}
	// The plot keeps most of the zone: the names are in a legend, not in a
	// margin that took 35% of the width.
	canvasW := showcaseCanvasWidth(t, svg)
	if span := last.x - first.x; span < 0.7*canvasW {
		t.Errorf("x axis spans %.0f of a %.0f-wide zone: the series names still squeeze the plot", span, canvasW)
	}
	// One-line labels need their width plus the gutter inside one pitch.
	if first.text == "Q1 25" {
		pitch := second.x - first.x
		width := float64(len(first.text)) * first.size * 0.5
		if pitch-width < first.size*xLabelMinGutterEm*0.9 {
			t.Errorf("x labels %.0fpx wide on a %.0fpx pitch: no gutter", width, pitch)
		}
	}

	// The rule itself: a label that fills 95% of its band is not "fitting".
	b := NewSVGBuilder(400, 300)
	cats := []string{"Q1 25", "Q2 25", "Q3 25", "Q4 25", "Q1 26", "Q2 26", "Q3 26", "Q4 26"}
	b.SetFontSize(12)
	w, _ := b.MeasureText("Q1 25")
	tight := AdaptXLabels(b, cats, 8*w*1.1/0.95, 12, true)
	if tight.Rotation == 0 && tight.MaxLines < 2 && tight.FontSize >= 12 {
		t.Errorf("labels at 95%% of their band were left on one line at full size: %+v", tight)
	}
}
