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
