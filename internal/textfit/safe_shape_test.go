package textfit

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// shaperPanicInputs are rune sequences that make the tdewolff/canvas HarfBuzz
// shaper panic with "slice bounds out of range" (go-slide-creator-7oz3c): a
// bidi control followed by a symbol / emoji and a combining mark. Built from
// code points so no invisible character sits in the source file.
func shaperPanicInputs() []string {
	const emoji, combining = 0x1F600, 0x0301
	return []string{
		string([]rune{0x202E, emoji, combining}) + "Q3 Strategy Review", // RLO + emoji + mark (the reported repro)
		string([]rune{0x2067, emoji, combining}),                        // RLI isolate
		string([]rune{0x202B, emoji, combining}),                        // RLE embedding
		"x " + string([]rune{0x202E, 0x00E9, combining}) + " y",         // RLO mid-string, no emoji
		string([]rune{0x200F, 0x2713, combining}) + " done",             // RLM (kept at the boundary) + check mark
	}
}

func TestShaperPanicInputsReallyPanicUnguarded(t *testing.T) {
	// Guards the regression test itself: if a canvas upgrade fixes the
	// shaper, this fails and the fallback can be revisited.
	face := newFace(fontcache.Get("Arial", ""), 18, canvas.FontRegular)
	s := shaperPanicInputs()[0]
	panicked := func() (p bool) {
		defer func() { p = recover() != nil }()
		canvas.NewTextLine(face, s, canvas.Left)
		return false
	}()
	if !panicked {
		t.Skip("canvas no longer panics on the RLO+emoji+combining repro; the guard is now belt-and-braces")
	}
}

func TestLineWidthMMRecoversFromShaperPanic(t *testing.T) {
	face := newFace(fontcache.Get("Arial", ""), 18, canvas.FontRegular)
	for _, s := range shaperPanicInputs() {
		w := LineWidthMM(face, s)
		if w <= 0 {
			t.Errorf("LineWidthMM(%+q) = %v, want a positive fallback estimate", s, w)
		}
	}
	if got, want := LineWidthMM(face, "Revenue"), canvas.NewTextLine(face, "Revenue", canvas.Left).Bounds().W(); got != want {
		t.Errorf("LineWidthMM must equal the shaped width for ordinary text: got %v want %v", got, want)
	}
	if LineWidthMM(nil, "x") != 0 {
		t.Error("nil face must measure 0")
	}
}

// Every public textfit entry point that measures text must survive the
// shaper panic instead of taking the process down.
func TestTextfitEntryPointsSurviveShaperPanic(t *testing.T) {
	for _, s := range shaperPanicInputs() {
		if _, err := Calculate(Params{WidthEMU: 4_000_000, HeightEMU: 1_000_000, Paragraphs: []string{s, s + " " + s}}); err != nil {
			t.Errorf("Calculate(%+q): %v", s, err)
		}
		if _, err := MeasureHeight(Params{WidthEMU: 4_000_000, HeightEMU: 1_000_000, Paragraphs: []string{s}}); err != nil {
			t.Errorf("MeasureHeight(%+q): %v", s, err)
		}
		if _, err := MeasureRun(s, "Arial", 18, 400_000, 1); err != nil {
			t.Errorf("MeasureRun(%+q): %v", s, err)
		}
		if _, err := MeasureLineWidth(s, "Arial", 18); err != nil {
			t.Errorf("MeasureLineWidth(%+q): %v", s, err)
		}
		if _, err := MeasureStyledLineWidth(s, "Arial", 18, true); err != nil {
			t.Errorf("MeasureStyledLineWidth(%+q): %v", s, err)
		}
		if _, err := MeasureStyledRuns(StyledMeasureParams{Runs: []StyledRun{{Text: s, Bold: true}, {Text: s}}, FontName: "Arial", FontPt: 14, WidthEMU: 2_000_000, MaxLines: 2}); err != nil {
			t.Errorf("MeasureStyledRuns(%+q): %v", s, err)
		}
		_ = MaxFontForWidth(s, 4_000_000, "Arial")
		_, _ = AutofitScale([]AutofitParagraph{{Text: s, FontPt: 18}}, 4_000_000, 40, AutofitOptions{FontName: "Arial"})
	}
}
