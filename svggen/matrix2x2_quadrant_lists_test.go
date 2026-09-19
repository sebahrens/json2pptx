package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-s27x. data-format-hints promotes `quadrants: [{position,
// title, items}]` as the coordinate-free alternative — what an agent without
// numbers should reach for. It rendered as a scatter plot: every item became a
// dot at an invented position, the quadrant title was drawn as one more centred
// string among them, and with two items per quadrant the strings already
// overlapped ("Data platform ●" over "Major projects" over "ERP upgrade ●").

func quadrantListChart(t *testing.T, w, h float64) string {
	t.Helper()
	b := NewSVGBuilder(w, h)
	cfg := DefaultMatrix2x2Config(w, h)
	cfg.QuadrantLabels = [4]string{"Major projects", "Quick wins", "Deprioritise", "Fill-ins"}
	data := Matrix2x2Data{QuadrantItems: [4][]string{
		{"ERP upgrade", "Data platform"},
		{"Pricing reset", "AI assistant"},
		{"Office move"},
		{"Partner portal refresh"},
	}}
	if err := NewMatrix2x2Chart(b, cfg).Draw(data); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(doc.Content)
}

// TestQuadrantListsDrawNoMarkers is the bead's VERIFY: no dots.
func TestQuadrantListsDrawNoMarkers(t *testing.T) {
	svg := quadrantListChart(t, 1104, 456)
	if strings.Contains(svg, "<circle") {
		t.Error("quadrant lists still draw point markers")
	}
	// The canvas library renders circles as arc paths; there must be none.
	if regexp.MustCompile(`<path d="M[^"]*A`).MatchString(svg) {
		t.Error("quadrant lists still draw arc (marker) paths")
	}
	for _, want := range []string{"Major projects", "ERP upgrade", "Data platform", "Fill-ins", "Partner portal refresh"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG is missing %q", want)
		}
	}
}

// TestQuadrantTitleIsTopmostInItsRectangle is the bead's other VERIFY clause.
func TestQuadrantTitleIsTopmostInItsRectangle(t *testing.T) {
	svg := quadrantListChart(t, 1104, 456)
	lines := parseSVGTextLines(t, svg)

	// Group by the quadrant each label belongs to, using the known contents.
	quadrants := map[string][]string{
		"Major projects": {"ERP upgrade", "Data platform"},
		"Quick wins":     {"Pricing reset", "AI assistant"},
		"Deprioritise":   {"Office move"},
		"Fill-ins":       {"Partner portal refresh"},
	}
	yOf := func(text string) (float64, bool) {
		for _, l := range lines {
			if l.text == text {
				return l.y, true
			}
		}
		return 0, false
	}
	for title, items := range quadrants {
		ty, ok := yOf(title)
		if !ok {
			t.Errorf("quadrant title %q not drawn", title)
			continue
		}
		for _, item := range items {
			iy, ok := yOf(item)
			if !ok {
				t.Errorf("item %q not drawn", item)
				continue
			}
			if ty >= iy {
				t.Errorf("title %q (y=%.2f) is not above its item %q (y=%.2f)", title, ty, item, iy)
			}
		}
	}
}

// TestQuadrantListsStayInsideTheirQuadrant guards the layout: nothing may be
// drawn above the plot area or past the half-way line into the next quadrant.
func TestQuadrantListsStayInsideTheirQuadrant(t *testing.T) {
	const w, h = 1104, 456
	svg := quadrantListChart(t, w, h)
	lines := parseSVGTextLines(t, svg)

	topLeft := map[string]bool{"Major projects": true, "ERP upgrade": true, "Data platform": true}
	bottomLeft := map[string]bool{"Deprioritise": true, "Office move": true}

	var topMaxY, bottomMinY float64 = 0, h
	for _, l := range lines {
		if topLeft[l.text] && l.y > topMaxY {
			topMaxY = l.y
		}
		if bottomLeft[l.text] && l.y < bottomMinY {
			bottomMinY = l.y
		}
	}
	if topMaxY >= bottomMinY {
		t.Errorf("top-left content reaches y=%.2f but bottom-left starts at y=%.2f — the quadrants overlap", topMaxY, bottomMinY)
	}
}

// TestQuadrantCaptionsSitInTheCorner: in POINTS mode the captions used to be
// centred in each quadrant, which is exactly where a point cluster lands — the
// reported chart printed "ERP upgrade" through "Major projects".
func TestQuadrantCaptionsSitInTheCorner(t *testing.T) {
	const w, h = 1104, 456
	b := NewSVGBuilder(w, h)
	cfg := DefaultMatrix2x2Config(w, h)
	cfg.QuadrantLabels = [4]string{"Major projects", "Quick wins", "Deprioritise", "Fill-ins"}
	data := Matrix2x2Data{Points: []Matrix2x2Point{
		{Label: "ERP upgrade", X: 25, Y: 75},
		{Label: "Pricing reset", X: 80, Y: 80},
	}}
	if err := NewMatrix2x2Chart(b, cfg).Draw(data); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lines := parseSVGTextLines(t, string(doc.Content))

	var caption, point *svgTextLine
	for i := range lines {
		switch lines[i].text {
		case "Major projects":
			caption = &lines[i]
		case "ERP upgrade":
			point = &lines[i]
		}
	}
	if caption == nil || point == nil {
		t.Fatalf("expected both the caption and the point label; got %+v", lines)
	}
	// The caption must be clear of the point label's line.
	if gap := point.y - caption.y; gap < (caption.size+point.size)/2 {
		t.Errorf("caption %q (y=%.2f) and point label %q (y=%.2f) are %.2f apart — they collide",
			caption.text, caption.y, point.text, point.y, gap)
	}
}

// TestPointsOutsideTheAxisRangeAreClampedAndReported: x=-10 / x=120 used to be
// plotted outside the frame with nothing reported.
func TestPointsOutsideTheAxisRangeAreClampedAndReported(t *testing.T) {
	const w, h = 1104, 456

	render := func(points []Matrix2x2Point) (string, []Finding) {
		b := NewSVGBuilder(w, h)
		if err := NewMatrix2x2Chart(b, DefaultMatrix2x2Config(w, h)).Draw(Matrix2x2Data{Points: points}); err != nil {
			t.Fatalf("Draw: %v", err)
		}
		doc, err := b.Render()
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return string(doc.Content), b.Findings()
	}

	svgOut, findings := render([]Matrix2x2Point{
		{Label: "Beyond", X: 120, Y: 50},
		{Label: "Before", X: -10, Y: 50},
	})

	var outOfRange int
	for _, f := range findings {
		if f.Code != FindingPointOutOfRange {
			continue
		}
		outOfRange++
		if !strings.Contains(f.Message, "clamped") {
			t.Errorf("finding does not say what happened: %q", f.Message)
		}
	}
	if outOfRange != 2 {
		t.Errorf("got %d out-of-range findings, want 2 (one per offending coordinate)", outOfRange)
	}

	// Clamping must land the points exactly where the axis ends — identical
	// geometry to the same labels at the axis extremes.
	svgClamped, clampedFindings := render([]Matrix2x2Point{
		{Label: "Beyond", X: 100, Y: 50},
		{Label: "Before", X: 0, Y: 50},
	})
	for _, f := range clampedFindings {
		if f.Code == FindingPointOutOfRange {
			t.Error("in-range points must not report an out-of-range finding")
		}
	}
	if labelX(t, svgOut, "Beyond") != labelX(t, svgClamped, "Beyond") {
		t.Errorf("x=120 landed at %.2f but x=100 lands at %.2f — the point was not clamped to the axis",
			labelX(t, svgOut, "Beyond"), labelX(t, svgClamped, "Beyond"))
	}
	if labelX(t, svgOut, "Before") != labelX(t, svgClamped, "Before") {
		t.Errorf("x=-10 landed at %.2f but x=0 lands at %.2f — the point was not clamped to the axis",
			labelX(t, svgOut, "Before"), labelX(t, svgClamped, "Before"))
	}
}

// labelX returns the x of the <text> element carrying the given content.
func labelX(t *testing.T, svg, text string) float64 {
	t.Helper()
	re := regexp.MustCompile(`<text\b[^>]*\bx="(-?[\d.]+)"[^>]*>(?:<tspan[^>]*>)?` + regexp.QuoteMeta(text) + `<`)
	m := re.FindStringSubmatch(svg)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}
