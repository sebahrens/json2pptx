package pptx

import (
	"fmt"
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

// The shape is written with a margin that leaves 1.3 em for its one line,
// where every estimate reserves 1.2 em: a renderer sets a line at its face's
// ascent plus descent (1.22 em in Carlito), and LibreOffice tightened the line
// spacing of boxes clamped to exactly 1.2 em (go-slide-creator-bhbtk). The
// written margin is never larger than the estimated one.
func TestGenerateShapeLeavesTheRenderersLineInAClampedBox(t *testing.T) {
	for _, hPt := range []int64{16, 23, 30, 38, 44, 60} {
		tb := insetBody("$120m", 1200, ShapeTextInsets())
		bounds := RectEmu{CX: 3 * 914400, CY: hPt * 12700}
		est := EffectiveTextInsets(tb, bounds)
		got := writtenTextInsets(tb, bounds)
		for side := range got {
			if got[side] > est[side] || got[side] < 0 {
				t.Errorf("%dpt box: written inset %d on side %d, want within [0, %d] (the estimate)", hPt, got[side], side, est[side])
			}
		}
		room := bounds.CY - got[1] - got[3]
		want := min(bounds.CY, int64(1200*127*13/10))
		if room < want {
			t.Errorf("%dpt box: written margins leave %d EMU for a 12pt line, want %d (1.3 em, or the whole box)", hPt, room, want)
		}
		xml, err := GenerateShape(ShapeOptions{ID: 2, Geometry: GeomRect, Bounds: bounds, Text: tb})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(xml), fmt.Sprintf(`tIns="%d"`, got[1])) {
			t.Errorf("%dpt box: shape is not written with the 1.3 em margin %d: %s", hPt, got[1], xml)
		}
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

// LibreOffice 24.2 shrinks rotated text that fits when the insets at the ends
// of its line are large (CI's render-truth check on the matrix-2x2 y-axis
// bar), so rotated text keeps at most 3pt there. The margins across the bar
// and the insets of upright text are untouched.
func TestRotatedTextKeepsSmallLineEndInsets(t *testing.T) {
	bar := RectEmu{CX: 317476, CY: 2823527} // the 25pt x 222pt axis bar
	rotated := insetBody("Market Growth", 1400, [4]int64{38100, 180000, 38100, 180000})
	rotated.Vert = "vert270"
	if got, want := EffectiveTextInsets(rotated, bar), ([4]int64{38100, 38100, 38100, 38100}); got != want {
		t.Errorf("rotated text insets = %v, want %v", got, want)
	}
	small := insetBody("Market Growth", 1400, [4]int64{38100, 12700, 38100, 0})
	small.Vert = "vert270"
	if got, want := EffectiveTextInsets(small, bar), ([4]int64{38100, 12700, 38100, 0}); got != want {
		t.Errorf("line-end insets under the cap = %v, want %v unchanged", got, want)
	}
	upright := insetBody("Market Share", 1400, ShapeTextInsets())
	if got := EffectiveTextInsets(upright, RectEmu{CX: 6224362, CY: 914400}); got != ShapeTextInsets() {
		t.Errorf("upright text insets = %v, want the shape margin %v", got, ShapeTextInsets())
	}
}
