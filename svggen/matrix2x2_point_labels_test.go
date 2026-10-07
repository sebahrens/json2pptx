package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-995rf: point labels near a quadrant boundary were drawn
// over each other, over other points' markers, over the quadrant heading and
// above the plot (over the diagram's title), with nothing reported.

func useCaseMatrix() []Matrix2x2Point {
	return []Matrix2x2Point{
		{Label: "Service triage", X: 80, Y: 84},
		{Label: "Code changes", X: 62, Y: 78},
		{Label: "IT incidents", X: 53, Y: 66},
		{Label: "Invoice matching", X: 76, Y: 70},
		{Label: "Claims intake", X: 66, Y: 58},
		{Label: "Password resets", X: 80, Y: 53},
		{Label: "Contract review", X: 46, Y: 90},
		{Label: "HR queries", X: 72, Y: 46},
		{Label: "Pricing exceptions", X: 24, Y: 22},
	}
}

func drawUseCaseMatrix(t *testing.T, w, h float64) *SVGBuilder {
	t.Helper()
	b := NewSVGBuilder(w, h)
	config := DefaultMatrix2x2Config(w, h)
	config.ShowPointLabels = true
	chart := NewMatrix2x2Chart(b, config)
	if err := chart.Draw(Matrix2x2Data{Title: "Use cases", Points: useCaseMatrix()}); err != nil {
		t.Fatal(err)
	}
	return b
}

func labelOverlapFindings(b *SVGBuilder) (n int) {
	for _, f := range b.Findings() {
		if f.Code == FindingDiagramTextOverlap && strings.Contains(f.Message, "point label") {
			n++
		}
	}
	return n
}

// On a body-sized canvas every label of the showcase matrix has a clear
// place; on a canvas too small for nine labels the ones that have none are
// reported at warning, not passed in silence.
func TestMatrixPointLabelsAreClearOrReported(t *testing.T) {
	if n := labelOverlapFindings(drawUseCaseMatrix(t, 900, 420)); n != 0 {
		t.Errorf("a 900x420 matrix reports %d point labels without a clear place", n)
	}
	small := drawUseCaseMatrix(t, 360, 220)
	if n := labelOverlapFindings(small); n == 0 {
		t.Error("nine labels on a 360x220 matrix: none was reported as overlapping")
	}
	for _, f := range small.Findings() {
		if f.Code == FindingDiagramTextOverlap && f.Severity != "warning" {
			t.Errorf("overlap finding at severity %q, want warning: %s", f.Severity, f.Message)
		}
	}
}

// A label keeps clear of another point's marker, stays inside the plot and
// off the quadrant heading, and stays beside its own point.
func TestMatrixPointLabelAvoidsMarkersHeadingsAndThePlotEdge(t *testing.T) {
	b := NewSVGBuilder(600, 400)
	config := DefaultMatrix2x2Config(600, 400)
	chart := NewMatrix2x2Chart(b, config)
	plot := Rect{X: 60, Y: 40, W: 500, H: 320}
	const size, fontSize, offset = 12.0, 12.0, 16.0

	// A point in the right half prefers its right side; another point's
	// marker sits exactly there.
	x, y := 360.0, 200.0
	other := placedLabel{x: x + offset + 20 - size/2, y: y - size/2, w: size, h: size}
	chart.markerBoxes = []placedLabel{{x: x - size/2, y: y - size/2, w: size, h: size}, other}
	box := chart.drawPointLabelAvoiding(x, y, size, "Claims intake", plot, nil, fontSize, offset)
	if box.overlaps(other) {
		t.Errorf("label box %+v covers another point's marker %+v", box, other)
	}

	// A point at the top edge under a heading band: not above the plot, not
	// on the heading, and within a line and a half of its point.
	chart.markerBoxes = nil
	heading := placedLabel{x: plot.X, y: plot.Y, w: 200, h: 24}
	x, y = plot.X+230, plot.Y+10
	box = chart.drawPointLabelAvoiding(x, y, size, "Contract review", plot, []placedLabel{heading}, fontSize, offset)
	if box.y < plot.Y || box.y+box.h > plot.Y+plot.H {
		t.Errorf("label box %+v leaves the plot %+v", box, plot)
	}
	if box.overlaps(heading) {
		t.Errorf("label box %+v covers the quadrant heading %+v", box, heading)
	}
	if dy := box.y + box.h/2 - y; dy > 2*fontSize*1.3 || dy < -2*fontSize*1.3 {
		t.Errorf("label sits %.0fpt from its point vertically: it reads as another point's", dy)
	}
	if n := labelOverlapFindings(b); n != 0 {
		t.Errorf("two placeable labels raised %d overlap findings", n)
	}
}
