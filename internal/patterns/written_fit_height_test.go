package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// writtenFitHeightPt returns the height at which the writer stores no
// autofit shrink — not a degenerate box with no text area, which the writer
// also reports as scale 1 (go-slide-creator-n1muf).
func TestWrittenFitHeightPtMatchesWriterMeasure(t *testing.T) {
	text := buildPhaseRoadmapPlainText("Weeks 1–8", 10, true, "dk1", "ctr")
	const widthPt = 167.24
	h := writtenFitHeightPt(text, widthPt, 0)
	if h < 15 {
		t.Fatalf("fit height %.0fpt is below one 12pt line plus insets", h)
	}
	tb, err := shapegrid.ResolveTextInput(text)
	if err != nil {
		t.Fatal(err)
	}
	box := func(pt float64) pptx.RectEmu {
		return pptx.RectEmu{CX: int64(widthPt * 12700), CY: int64(pt * 12700)}
	}
	if s := pptx.AutofitScaleFor(tb, box(h)); s < 1 {
		t.Fatalf("writer shrinks text at the returned %.0fpt: %.2f", h, s)
	}
	if s := pptx.AutofitScaleFor(tb, box(h-1)); s >= 1 {
		t.Fatalf("%.0fpt is not the smallest fitting height", h)
	}
	if got := writtenFitHeightPt(text, widthPt, h+5); got != h+5 {
		t.Fatalf("minimum not honoured: %.0f", got)
	}
}
