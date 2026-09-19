package svggen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-s5ur. Every node drew its name and its job title at almost
// the same baseline: in the reported chart the name sat at y=103.23 in 12.08px
// and the title at y=113.53 in 10.44px — a 10.31px gap for glyph boxes whose
// half-heights sum to 11.26, so "Anna Becker" and "CEO" touched on all four
// templates. The cause was `titleY = nameY + nameFontSize*0.9`: a fraction of
// ONE font size, which says nothing about how tall the other line is.

// svgTextLine is one rendered <text> element: its baseline, size and content.
type svgTextLine struct {
	y    float64
	size float64
	text string
}

var svgTextRe = regexp.MustCompile(`(?s)<text\b([^>]*)>(.*?)</text>`)
var svgTextYRe = regexp.MustCompile(`\by="([-\d.]+)"`)
var svgTextSizeRe = regexp.MustCompile(`font:\s*(?:\d+\s+)?([\d.]+)px`)
var svgTagRe = regexp.MustCompile(`<[^>]+>`)

// parseSVGTextLines extracts every text element in document order.
func parseSVGTextLines(t *testing.T, svg string) []svgTextLine {
	t.Helper()
	var out []svgTextLine
	for _, m := range svgTextRe.FindAllStringSubmatch(svg, -1) {
		attrs, body := m[1], m[2]
		y := svgTextYRe.FindStringSubmatch(attrs)
		size := svgTextSizeRe.FindStringSubmatch(attrs)
		if y == nil || size == nil {
			continue
		}
		yv, err1 := strconv.ParseFloat(y[1], 64)
		sv, err2 := strconv.ParseFloat(size[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, svgTextLine{y: yv, size: sv, text: strings.TrimSpace(svgTagRe.ReplaceAllString(body, ""))})
	}
	return out
}

// drawOrgChartSVG renders a chart at the given canvas size and returns its SVG.
func drawOrgChartSVG(t *testing.T, data OrgChartData, w, h float64) string {
	t.Helper()
	b := NewSVGBuilder(w, h)
	if err := NewOrgChartRenderer(b, DefaultOrgChartConfig(w, h)).Draw(data); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(doc.Content)
}

// leadershipTree is the nine-person tree the reviewers used.
func leadershipTree() OrgChartData {
	return OrgChartData{
		Root: OrgNode{Name: "Anna Becker", Title: "CEO", Children: []OrgNode{
			{Name: "Jonas Weber", Title: "CFO", Children: []OrgNode{
				{Name: "Lea Hoffmann", Title: "Head of FP&A"},
				{Name: "Tim Krause", Title: "Head of Treasury"},
			}},
			{Name: "Marta Nowak", Title: "COO", Children: []OrgNode{
				{Name: "Felix Braun", Title: "VP Supply Chain"},
				{Name: "Sara Lind", Title: "VP Operations"},
			}},
			{Name: "David Klein", Title: "CTO", Children: []OrgNode{
				{Name: "Nora Vogel", Title: "Head of Platform"},
			}},
		}},
	}
}

// TestOrgChartLabelsDoNotOverlap is the bead's VERIFY, as an invariant rather
// than a pixel: consecutive label lines inside a node must be at least
// (upperSize + lowerSize) / 2 * 1.05 apart, so their glyph boxes cannot touch.
func TestOrgChartLabelsDoNotOverlap(t *testing.T) {
	// Canvas sizes spanning the placeholder shapes the four bundled templates
	// give a diagram, including the letterboxed wide one the reviewers hit.
	for _, size := range [][2]float64{{656, 456}, {900, 500}, {1200, 600}, {560, 380}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size[0], size[1]), func(t *testing.T) {
			lines := parseSVGTextLines(t, drawOrgChartSVG(t, leadershipTree(), size[0], size[1]))
			if len(lines) < 4 {
				t.Fatalf("expected label lines, got %d", len(lines))
			}
			checked := 0
			for i := 0; i+1 < len(lines); i++ {
				a, b := lines[i], lines[i+1]
				gap := b.y - a.y
				// Lines in different nodes are far apart or ordered arbitrarily;
				// only consecutive lines within one node are close enough to
				// collide, and those are the pairs worth asserting on.
				if gap <= 0 || gap > 3*(a.size+b.size) {
					continue
				}
				checked++
				need := (a.size + b.size) / 2 * 1.05
				if gap < need {
					t.Errorf("%q (%.2fpx) and %q (%.2fpx) are %.2f apart, need >= %.2f — the glyphs touch",
						a.text, a.size, b.text, b.size, gap, need)
				}
			}
			if checked == 0 {
				t.Error("no name/title pairs were checked — the test is not exercising the layout")
			}
		})
	}
}

// TestOrgChartPruningKeepsTheRealTitle: depth pruning used to overwrite a
// manager's job title with "+3 reports", so the chart lost data it was never
// asked to lose. The count is now its own line.
func TestOrgChartPruningKeepsTheRealTitle(t *testing.T) {
	var directs []OrgNode
	for i := 1; i <= 6; i++ {
		var leads []OrgNode
		for j := 1; j <= 3; j++ {
			leads = append(leads, OrgNode{Name: fmt.Sprintf("Lead %d%d", i, j), Title: "Head"})
		}
		directs = append(directs, OrgNode{
			Name: fmt.Sprintf("Director %d", i), Title: fmt.Sprintf("VP Area %d", i), Children: leads,
		})
	}
	data := OrgChartData{Root: OrgNode{Name: "Anna Becker", Title: "CEO", Children: directs}}

	svg := drawOrgChartSVG(t, data, 656, 456)
	if !strings.Contains(svg, "+3 reports") {
		t.Fatal("pruning did not mark the hidden reports")
	}
	for i := 1; i <= 6; i++ {
		title := fmt.Sprintf("VP Area %d", i)
		if !strings.Contains(svg, title) {
			t.Errorf("director %d lost its real title %q to the pruning marker", i, title)
		}
	}

	// And the three lines must still clear each other.
	lines := parseSVGTextLines(t, svg)
	for i := 0; i+1 < len(lines); i++ {
		a, b := lines[i], lines[i+1]
		gap := b.y - a.y
		if gap <= 0 || gap > 3*(a.size+b.size) {
			continue
		}
		if need := (a.size + b.size) / 2 * 1.05; gap < need {
			t.Errorf("%q and %q are %.2f apart, need >= %.2f", a.text, b.text, gap, need)
		}
	}
}

func TestOrgLabelLeading(t *testing.T) {
	// The leading must clear the touching threshold for any pair of sizes.
	for _, sizes := range [][2]float64{{12, 10}, {8, 7}, {7, 12}, {10, 10}} {
		got := orgLabelLeading(sizes[0], sizes[1])
		need := (sizes[0] + sizes[1]) / 2 * 1.05
		if got < need {
			t.Errorf("orgLabelLeading(%.0f, %.0f) = %.2f, below the touching threshold %.2f", sizes[0], sizes[1], got, need)
		}
	}
}
