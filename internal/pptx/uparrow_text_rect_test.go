package pptx

import (
	"strings"
	"testing"
)

// TestUpArrowTextRectSize follows ECMA-376's upArrow definition: the text
// rectangle is the shaft, raised into the head as far as the shaft's edges
// reach under the slope.
func TestUpArrowTextRectSize(t *testing.T) {
	bounds := RectEmu{CX: 10000, CY: 2000}
	// A full-width shaft is a gable pentagon: the text rectangle is the band
	// under the gable (head = 2000 × 60% = 1200).
	if w, h := UpArrowTextRectSize(100000, 60000, bounds); w != 10000 || h != 800 {
		t.Errorf("gable text rect = %dx%d, want 10000x800", w, h)
	}
	// The preset default: a half-width shaft under a 1000-high head; the
	// shaft's edges meet the slope halfway up the head.
	if w, h := UpArrowTextRectSize(-1, -1, bounds); w != 5000 || h != 1500 {
		t.Errorf("default text rect = %dx%d, want 5000x1500", w, h)
	}
	// The head cannot be longer than the shape.
	if _, h := UpArrowTextRectSize(100000, 500000, bounds); h != 0 {
		t.Errorf("all-head arrow keeps a %d-high text rect", h)
	}
	if w, h := PresetTextRect("upArrow", map[string]int64{"adj1": 100000, "adj2": 60000}, bounds); w != 10000 || h != 800 {
		t.Errorf("PresetTextRect(upArrow) = %dx%d", w, h)
	}
	if w, h := PresetTextRect("rect", nil, bounds); w != 10000 || h != 2000 {
		t.Errorf("PresetTextRect(rect) = %dx%d", w, h)
	}
	if w, h := PresetTextRect("triangle", nil, bounds); w != 5000 || h != 1000 {
		t.Errorf("PresetTextRect(triangle) = %dx%d", w, h)
	}
}

// TestGableAutofitMeasuresTheBand: a gable's stored autofit shrink is measured
// in the band under the slope, where the text is drawn — not in the whole
// shape, which predicted a fit the band does not have.
func TestGableAutofitMeasuresTheBand(t *testing.T) {
	shape := func(geometry PresetGeometry, adj []AdjustValue) string {
		xml, err := GenerateShape(ShapeOptions{
			ID: 7, Geometry: geometry, Adjustments: adj,
			Bounds: RectEmu{CX: 6000000, CY: 1200000},
			Text: &TextBody{
				Wrap: "square", Anchor: "ctr", AutoFit: "normAutofit", Insets: ShapeTextInsets(),
				Paragraphs: []Paragraph{{Align: "ctr", Runs: []Run{{
					Text: strings.Repeat("Turn pilots into a repeatable operating capability. ", 3), FontSize: 1600, Bold: true,
				}}}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(xml)
	}
	if rect := shape(GeomRect, nil); strings.Contains(rect, "fontScale") {
		t.Fatalf("the text fits the whole shape; the fixture proves nothing: %s", rect)
	}
	// The same box as a gable whose head takes 60% of the height leaves a
	// 480000 EMU band: the text no longer fits and the shrink is stored.
	gable := shape(GeomUpArrow, []AdjustValue{{Name: "adj1", Value: 100000}, {Name: "adj2", Value: 60000}})
	if !strings.Contains(gable, "fontScale") {
		t.Errorf("gable text was measured against the whole shape: %s", gable)
	}
}

// TestSideArrowTextRectSize follows ECMA-376's rightArrow definition: the text
// rectangle is the shaft, run into the head as far as the shaft's edges reach
// under the slope (go-slide-creator-fx48s).
func TestSideArrowTextRectSize(t *testing.T) {
	bounds := RectEmu{CX: 10000, CY: 2000}
	// The preset default: a half-height shaft and a head 1000 long; the
	// shaft's edges meet the slope halfway along the head.
	if w, h := SideArrowTextRectSize(-1, -1, bounds); w != 9500 || h != 1000 {
		t.Errorf("default text rect = %dx%d, want 9500x1000", w, h)
	}
	// A 70% shaft: 300 above and below it, so the rectangle reaches 300/1000
	// of the 1000-long head.
	if w, h := SideArrowTextRectSize(70000, 50000, bounds); w != 9300 || h != 1400 {
		t.Errorf("block arrow text rect = %dx%d, want 9300x1400", w, h)
	}
	for _, geom := range []string{"rightArrow", "leftArrow"} {
		if w, h := PresetTextRect(geom, map[string]int64{"adj1": 70000, "adj2": 50000}, bounds); w != 9300 || h != 1400 {
			t.Errorf("PresetTextRect(%s) = %dx%d, want 9300x1400", geom, w, h)
		}
	}
	if got := SideArrowShaftInsetEMU(70000, bounds); got != 300 {
		t.Errorf("shaft inset = %d, want 300", got)
	}
	// The stored autofit shrink is measured in the shaft, not the shape: a
	// label that fills the shape's height does not fit the shaft unshrunk.
	tb := &TextBody{Wrap: "square", AutoFit: "normAutofit", Paragraphs: []Paragraph{{Runs: []Run{{Text: "Review the case", FontSize: 1600}}}}}
	shape := ShapeOptions{ID: 1, Geometry: GeomRightArrow, Bounds: RectEmu{CX: 200 * 12700, CY: 30 * 12700}, Text: tb}
	if got := autofitBounds(shape); got.CY != 15*12700 {
		t.Errorf("autofit bounds height = %d, want the %d shaft", got.CY, 15*12700)
	}
}
