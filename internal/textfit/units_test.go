package textfit

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// Regression for the pt/mm unit bug: canvas faces take their size in points
// and report bounds in millimetres. textfit used to build faces at
// fontPt*ptToMM, measuring every string ~2.83x too narrow.
//
// Liberation Sans (the bundled metric-compatible Arial) has an advance of
// 1708/2048 em for "M", so ten "M" at 100pt are 834pt = 294.2mm wide.
func TestFaceWidthUsesPoints(t *testing.T) {
	m, err := MeasureRun("x", "Arial", 12, 914400, 0) // warm the cache / resolve
	if err != nil || m.FontFamily == "" {
		t.Skipf("font cache unavailable: %v", err)
	}
	ff, _, _ := fontcache.Resolve("Arial", "Arial")
	if ff == nil {
		t.Skip("font cache unavailable")
	}
	face := newFace(ff, 100, canvas.FontRegular)
	gotMM := canvas.NewTextLine(face, strings.Repeat("M", 10), canvas.Left).Bounds().W()
	wantMM := 10 * 100 * (1708.0 / 2048.0) * ptToMM
	if gotMM < wantMM*0.97 || gotMM > wantMM*1.03 {
		t.Fatalf("10xM at 100pt = %.1fmm, want ~%.1fmm (faces must be sized in points)", gotMM, wantMM)
	}
}

// A 40-character capitalised string at 20pt Arial is ~7.4in wide: it must
// wrap in a 4in box and fit on one line in a 9in box.
func TestMeasureRunKnownStringWidth(t *testing.T) {
	text := "QUARTERLY REVENUE GREW ACROSS ALL REGION" // 40 chars
	narrow, err := MeasureRun(text, "Arial", 20, 4*914400, 0)
	if err != nil {
		t.Skipf("font cache unavailable: %v", err)
	}
	if narrow.Lines < 2 {
		t.Errorf("40 caps at 20pt in 4in: lines = %d, want >= 2", narrow.Lines)
	}
	wide, err := MeasureRun(text, "Arial", 20, 9*914400, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wide.Lines != 1 {
		t.Errorf("40 caps at 20pt in 9in: lines = %d, want 1", wide.Lines)
	}
}
