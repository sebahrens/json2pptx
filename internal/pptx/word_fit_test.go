package pptx

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// go-slide-creator-v74wv: a narrow timeline column clamped its margin to
// exactly the measured width of "Design", and the renderer broke the last
// glyph onto a second line ("Desig / n"). The clamp now leaves WordFitSlack of
// room when the shape has it — StandInWordFitSlack when the word is measured
// in the Liberation Sans stand-in rather than its own face.
func TestEffectiveTextInsetsLeavesRendererSlack(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fonts ThemeFonts
		slack float64
	}{
		{"own face (Calibri)", ThemeFonts{Minor: "Calibri"}, WordFitSlack},
		{"host-dependent face (Segoe UI)", ThemeFonts{Minor: "Segoe UI"}, StandInWordFitSlack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := insetBody("Design", 1400, ShapeTextInsets())
			tb.Paragraphs[0].Runs[0].Bold = true
			tb.ThemeFonts = tc.fonts
			word := widestWordEMU(tb)
			bounds := RectEmu{CX: word + word/2, CY: 914400} // room for the slack, not the full margin
			in := EffectiveTextInsets(tb, bounds)
			avail := bounds.CX - in[0] - in[2]
			if float64(avail) < float64(word)*tc.slack-1 {
				t.Fatalf("clamp leaves %d EMU for a %d EMU word, want at least %.2fx", avail, word, tc.slack)
			}
			if in[0] >= ShapeTextInsetEMU {
				t.Fatalf("the margin was not clamped: %v", in)
			}
		})
	}
}

// A bulleted item's words start at marL: the clamp gives the margin back for
// the word plus the paragraph's indent, not the word alone ("Warehousi / ng"
// in a value-chain chevron).
func TestEffectiveTextInsetsCountsParagraphMargins(t *testing.T) {
	tb := insetBody("Warehousing", 1000, ShapeTextInsets())
	tb.Paragraphs[0].MarginL = 177800
	bare := insetBody("Warehousing", 1000, ShapeTextInsets())
	if got, want := widestWordEMU(tb), widestWordEMU(bare)+177800; got != want {
		t.Fatalf("widest word with marL = %d, want %d", got, want)
	}
	bounds := RectEmu{CX: widestWordEMU(tb) * 13 / 10, CY: 914400}
	in := EffectiveTextInsets(tb, bounds)
	if line := bounds.CX - in[0] - in[2] - 177800; line < widestWordEMU(bare) {
		t.Fatalf("bulleted line keeps %d EMU for a %d EMU word", line, widestWordEMU(bare))
	}
}

// ECMA-376 trapezoid: il = wd3·a/maxAdj, it = hd3·a/maxAdj with
// maxAdj = 50000·w/ss — the text rectangle a pyramid tier really has.
func TestPresetTextRectSizeTrapezoid(t *testing.T) {
	w, h := PresetTextRectSize("trapezoid", 30000, RectEmu{CX: 100000, CY: 50000})
	if w != 80000 || h != 45000 {
		t.Fatalf("trapezoid text rect = %dx%d, want 80000x45000", w, h)
	}
	if w, _ := PresetTextRectSize("trapezoid", -1, RectEmu{CX: 100000, CY: 100000}); w != 66666 {
		t.Fatalf("default-adj trapezoid text width = %d, want 66666", w)
	}
}

// A shape whose margin the clamp takes to zero on both axes writes explicit
// zero insets. Unwritten, all-zero insets mean the 0.1" renderer default: the
// figure got that margin back, and the autofit measure took the same default
// off its width and stored a 66% shrink for a figure that fits.
func TestGenerateShapeWritesClampedZeroInsets(t *testing.T) {
	tb := insetBody("$4.2M", 2400, ShapeTextInsets())
	tb.Paragraphs[0].Runs[0].Bold = true
	word := widestWordEMU(tb)
	xml, err := GenerateShape(ShapeOptions{ID: 2, Geometry: GeomRect, Bounds: RectEmu{CX: word * 21 / 20, CY: 28 * 12700}, Text: tb})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `lIns="0" tIns="0" rIns="0" bIns="0"`) {
		t.Fatalf("clamped-to-zero insets not written: %s", s)
	}
	if tb.AutoFitFontScale != 0 && tb.AutoFitFontScale < 95000 {
		t.Fatalf("a figure that fits its width was shrunk to %d: %s", tb.AutoFitFontScale, s)
	}
}

func TestUnfitWords(t *testing.T) {
	shape := func(text string, sizeHPt int, cx int64) string {
		tb := insetBody(text, sizeHPt, ShapeTextInsets())
		xml, err := GenerateShape(ShapeOptions{ID: 7, Geometry: GeomRect, Bounds: RectEmu{CX: cx, CY: 914400}, Text: tb})
		if err != nil {
			t.Fatal(err)
		}
		return string(xml)
	}
	word, err := textfit.MeasureStyledLineWidth("Transformation", "Liberation Sans", 11, false)
	if err != nil {
		t.Fatal(err)
	}
	// The bare shape is narrower than the word at the 11pt it is written at.
	narrow := shape("Transformation programme", 1100, word*8/10)
	got := UnfitWords(narrow, "")
	if len(got) != 1 || got[0].Word != "Transformation" || got[0].ShapeID != "7" || got[0].NeedEMU <= got[0].AvailEMU {
		t.Fatalf("narrow shape: got %+v", got)
	}
	// Wide enough once the margin is clamped: nothing breaks.
	if got := UnfitWords(shape("Transformation programme", 1100, word*11/10), ""); len(got) != 0 {
		t.Fatalf("clamped shape that holds its word reported: %+v", got)
	}
	// A 14pt word the writer shrinks onto one line is written whole.
	if got := UnfitWords(shape("Transformation", 1400, word*13/10), ""); len(got) != 0 {
		t.Fatalf("word rescued by the stored shrink reported: %+v", got)
	}
	// Placeholders are the template's, not the writer's.
	if got := UnfitWords(strings.Replace(narrow, "<p:nvPr/>", `<p:nvPr><p:ph type="body"/></p:nvPr>`, 1), ""); len(got) != 0 {
		t.Fatalf("placeholder reported: %+v", got)
	}
}

func TestWordShrinkRescues(t *testing.T) {
	body := insetBody("Transformation", 1600, ShapeTextInsets())
	if !WordShrinkRescues(body, 100, 90) {
		t.Fatal("a 16pt word 1.1x its line is shrunk onto it")
	}
	if WordShrinkRescues(body, 100, 60) {
		t.Fatal("a word needing a shrink below the 12pt floor is not rescued")
	}
	if WordShrinkRescues(insetBody("Transformation", 1200, ShapeTextInsets()), 100, 95) {
		t.Fatal("12pt text does not shrink for a word")
	}
	noAutofit := insetBody("Transformation", 1600, ShapeTextInsets())
	noAutofit.AutoFit = ""
	if WordShrinkRescues(noAutofit, 100, 95) {
		t.Fatal("a body without normAutofit is not shrunk")
	}
}
