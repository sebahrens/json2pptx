package pptx

import (
	"strings"
	"testing"
)

func insetBody(text string, sizeHPt int, insets [4]int64) *TextBody {
	return &TextBody{Insets: insets, AutoFit: "normAutofit", Paragraphs: []Paragraph{{Runs: []Run{{Text: text, FontSize: sizeHPt}}}}}
}

func TestShapeTextInsetIsHalfACentimetre(t *testing.T) {
	if ShapeTextInsetEMU != 180000 {
		t.Fatalf("ShapeTextInsetEMU = %d, want 180000 (0.5 cm)", ShapeTextInsetEMU)
	}
	if ShapeTextInsets() != [4]int64{180000, 180000, 180000, 180000} {
		t.Fatalf("ShapeTextInsets = %v", ShapeTextInsets())
	}
}

// A shape with room for one line plus the margin keeps the full margin.
func TestEffectiveTextInsetsKeepsTheMarginWhenThereIsRoom(t *testing.T) {
	tb := insetBody("Roomy card", 1200, ShapeTextInsets())
	got := EffectiveTextInsets(tb, RectEmu{CX: 3 * 914400, CY: 914400})
	if got != ShapeTextInsets() {
		t.Fatalf("roomy shape insets = %v, want the uniform margin", got)
	}
}

// A pill too short for one line plus 2 x 0.5 cm shrinks its vertical margin to
// what still leaves exactly one line, symmetrically; the horizontal margin is
// untouched.
func TestEffectiveTextInsetsClampsADegenerateAxisToOneLine(t *testing.T) {
	tb := insetBody("Pill", 1200, ShapeTextInsets())
	bounds := RectEmu{CX: 3 * 914400, CY: 30 * 12700}
	got := EffectiveTextInsets(tb, bounds)
	left, top, right, bottom := insetSides(got)
	line := int64(1200 * 127 * 12 / 10)
	if left != ShapeTextInsetEMU || right != ShapeTextInsetEMU {
		t.Fatalf("horizontal margin changed on a wide pill: %v", got)
	}
	if room := bounds.CY - top - bottom; room < line || room > line+2 {
		t.Fatalf("clamped pill leaves %d EMU for text, want one %d EMU line (insets %v)", room, line, got)
	}
	if top < 0 || bottom < 0 || top-bottom > 1 || bottom-top > 1 {
		t.Fatalf("clamp must stay symmetric and non-negative: %v", got)
	}
}

// A badge narrower than its widest word loses its side margin (never
// negative) rather than breaking the word.
func TestEffectiveTextInsetsNeverGoesNegative(t *testing.T) {
	tb := insetBody("Transformation", 1400, ShapeTextInsets())
	got := EffectiveTextInsets(tb, RectEmu{CX: 20 * 12700, CY: 10 * 12700})
	for i, v := range got {
		if v != 0 {
			t.Fatalf("side %d of a shape smaller than its text = %d, want 0 (%v)", i, v, got)
		}
	}
}

// Bodies without declared insets (renderer defaults) and empty bodies are
// returned unchanged.
func TestEffectiveTextInsetsLeavesUndeclaredAndEmptyBodies(t *testing.T) {
	if got := EffectiveTextInsets(insetBody("x", 1200, [4]int64{}), RectEmu{CX: 1, CY: 1}); got != [4]int64{} {
		t.Fatalf("undeclared insets changed: %v", got)
	}
	if got := EffectiveTextInsets(insetBody("  ", 1200, ShapeTextInsets()), RectEmu{CX: 1, CY: 1}); got != ShapeTextInsets() {
		t.Fatalf("empty body insets changed: %v", got)
	}
}

// The writer stores the clamped insets, and the autofit measure uses the same
// clamped text area, so a one-line pill is not shrunk.
func TestGenerateShapeWritesTheClampedInsets(t *testing.T) {
	tb := insetBody("Pill", 1200, ShapeTextInsets())
	xml, err := GenerateShape(ShapeOptions{ID: 2, Geometry: GeomRect, Bounds: RectEmu{CX: 3 * 914400, CY: 30 * 12700}, Text: tb})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if strings.Contains(s, `tIns="180000"`) || !strings.Contains(s, `lIns="180000"`) {
		t.Fatalf("pill written without its clamped vertical margin: %s", s)
	}
	if strings.Contains(s, "fontScale") {
		t.Fatalf("one-line pill was shrunk: %s", s)
	}
}

func TestUniformInsetFor(t *testing.T) {
	if got := UniformInsetFor(914400, 12700); got != ShapeTextInsetEMU {
		t.Fatalf("roomy axis inset = %d", got)
	}
	if got := UniformInsetFor(20*12700, 16*12700); got != 2*12700 {
		t.Fatalf("tight axis inset = %d, want %d", got, 2*12700)
	}
	if got := UniformInsetFor(10*12700, 16*12700); got != 0 {
		t.Fatalf("degenerate axis inset = %d, want 0", got)
	}
}

func TestPresetTextRectSize(t *testing.T) {
	b := RectEmu{CX: 100000, CY: 100000}
	if w, h := PresetTextRectSize("rect", -1, b); w != 100000 || h != 100000 {
		t.Fatalf("rect text rect = %dx%d", w, h)
	}
	if w, h := PresetTextRectSize("ellipse", -1, b); w != 70710 || h != 70710 {
		t.Fatalf("ellipse text rect = %dx%d", w, h)
	}
	if w, h := PresetTextRectSize("flowChartDecision", -1, b); w != 50000 || h != 50000 {
		t.Fatalf("decision text rect = %dx%d", w, h)
	}
}

// insetSides unpacks written insets as left, top, right, bottom.
func insetSides(insets [4]int64) (left, top, right, bottom int64) {
	return insets[0], insets[1], insets[2], insets[3]
}
