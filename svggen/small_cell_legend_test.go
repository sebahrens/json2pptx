package svggen

import (
	"regexp"
	"strconv"
	"testing"
)

// textNodeYSize returns the baseline y and font size of the first <text> node
// whose body is exactly label.
func textNodeYSize(svg, label string) (y, size float64, ok bool) {
	re := regexp.MustCompile(`<text[^>]*\by="([0-9.]+)"[^>]*font-size:([0-9.]+)px[^>]*>(?:<tspan[^>]*>)?` + regexp.QuoteMeta(label) + `<`)
	m := re.FindStringSubmatch(svg)
	if m == nil {
		return 0, 0, false
	}
	y, err1 := strconv.ParseFloat(m[1], 64)
	size, err2 := strconv.ParseFloat(m[2], 64)
	return y, size, err1 == nil && err2 == nil
}

// legendSwatchTop returns the top edge of the rect swatch drawn immediately
// before the legend text label, if there is one.
func legendSwatchTop(svg, label string) (float64, bool) {
	re := regexp.MustCompile(`<path d="M[0-9.]+ ([0-9.]+)H[0-9.]+V([0-9.]+)H[0-9.]+z"[^>]*/><text[^>]*>(?:<tspan[^>]*>)?` + regexp.QuoteMeta(label) + `<`)
	m := re.FindStringSubmatch(svg)
	if m == nil {
		return 0, false
	}
	a, err1 := strconv.ParseFloat(m[1], 64)
	b, err2 := strconv.ParseFloat(m[2], 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	if b < a {
		return b, true
	}
	return a, true
}

// go-slide-creator-uptnk: in a small shape_grid cell (template-qa-deck.json
// slide 9) placement typography enlarges the fonts ~2.5x, and the bottom
// legend was placed one bare line height below the tick-label anchor while
// the labels' glyphs reach ~1.8em below it. The Current/Target swatches were
// drawn on top of the Intake/Mobilise category labels. The layout already
// budgeted 1.8em (xLabelGlyphEm); the legend placement did not.
func TestBottomLegendClearsCategoryLabelsInSmallCell(t *testing.T) {
	// Glyph extents in ems of the text's own font size: descenders reach
	// ~0.25em below the baseline, caps/ascenders ~0.75em above it.
	const descentEm, ascentEm = 0.25, 0.75

	categories := []any{"Intake", "Decide", "Mobilise", "Learn"}
	series := []any{
		map[string]any{"name": "Current", "values": []any{2.1, 2.4, 2.8, 1.9}},
		map[string]any{"name": "Target", "values": []any{4.2, 4.5, 4.0, 4.3}},
	}
	sizes := []struct {
		name             string
		w, h             int
		placeW, placeH   float64
		xLabel, withTitl bool
	}{
		// The template-qa-deck slide 9 cell: authored 1000x370 in a ~390x160pt cell.
		{name: "qa_deck_cell", w: 1000, h: 370, placeW: 390, placeH: 160, withTitl: true},
		{name: "qa_deck_cell_x_label", w: 1000, h: 370, placeW: 390, placeH: 160, withTitl: true, xLabel: true},
		// chart-insights-split style panel.
		{name: "insights_panel", w: 800, h: 450, placeW: 420, placeH: 230},
		// Default canvas, no placement scaling.
		{name: "default_canvas", w: 800, h: 600},
	}
	for _, ct := range []string{"bar_chart", "grouped_bar_chart", "stacked_bar_chart", "line_chart", "area_chart"} {
		for _, sz := range sizes {
			t.Run(ct+"/"+sz.name, func(t *testing.T) {
				data := map[string]any{"categories": categories, "series": series}
				if sz.xLabel {
					data["x_label"] = "Stage"
				}
				req := &RequestEnvelope{
					Type:   ct,
					Data:   data,
					Output: OutputSpec{Width: sz.w, Height: sz.h},
				}
				if sz.withTitl {
					req.Title = "svggen operating cadence"
				}
				if sz.placeW > 0 {
					req.Style.PlacementWidthPt = sz.placeW
					req.Style.PlacementHeightPt = sz.placeH
					req.Style.MinReadablePt = 12
				}
				doc, err := Render(req)
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				svg := string(doc.Content)

				// Lowest glyph bottom of anything the x axis draws.
				axisBottom := 0.0
				for _, c := range categories {
					y, fs, ok := textNodeYSize(svg, c.(string))
					if !ok {
						t.Fatalf("category label %q not rendered", c)
					}
					axisBottom = max(axisBottom, y+descentEm*fs)
				}
				if sz.xLabel {
					y, fs, ok := textNodeYSize(svg, "Stage")
					if !ok {
						t.Fatal("x-axis title not rendered")
					}
					axisBottom = max(axisBottom, y+descentEm*fs)
				}

				for _, name := range []string{"Current", "Target"} {
					y, fs, ok := textNodeYSize(svg, name)
					if !ok {
						t.Fatalf("series name %q not rendered", name)
					}
					top := y - ascentEm*fs
					// Direct labels sit above the axis, inside the plot; only a
					// legend below the axis can collide with its labels.
					if y < axisBottom-2*fs {
						continue
					}
					if sw, ok := legendSwatchTop(svg, name); ok {
						top = min(top, sw)
					}
					if top < axisBottom {
						t.Errorf("legend entry %q (top %.1f) overlaps the x-axis labels (bottom %.1f)", name, top, axisBottom)
					}
					if y+descentEm*fs > doc.Height+0.5 {
						t.Errorf("legend entry %q (bottom %.1f) falls below the canvas (%.1f)", name, y+descentEm*fs, doc.Height)
					}
				}
			})
		}
	}
}
