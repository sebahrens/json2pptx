package textfit

import (
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

func TestPhysicalLineWidthEMU(t *testing.T) {
	for _, family := range []string{"Arial", "Poppins Light", "Lora"} {
		t.Run(family, func(t *testing.T) {
			ff, resolved, substituted := fontcache.Resolve(family, "Arial")
			wantResolved, wantSubstituted := family, false
			if family == "Arial" {
				wantResolved, wantSubstituted = "Liberation Sans", true
			}
			if ff == nil || substituted != wantSubstituted || resolved != wantResolved {
				t.Fatalf("native font unavailable: resolved=%q substituted=%v", resolved, substituted)
			}
			for _, pt := range []float64{10, 18, 45} {
				const text = "Illustrative test data for review"
				// Canvas reports millimetres. OOXML defines exactly 36,000 EMU/mm;
				// this oracle deliberately does not use textfit's ptToMM constant.
				mm := canvas.NewTextLine(newFace(ff, pt, canvas.FontRegular), text, canvas.Left).Bounds().W()
				want := int64(math.Ceil(mm * 36000))
				got, err := MeasureLineWidth("Q1\n"+text, family, pt)
				if err != nil || got != want {
					t.Errorf("%.0fpt: width=%d err=%v, want exact physical width %d", pt, got, err, want)
				}
			}
		})
	}
}

func TestPhysicalWrapBoundary(t *testing.T) {
	const family = "Poppins Light"
	const text = "Illustrative test data for review"
	ff, resolved, substituted := fontcache.Resolve(family, "Arial")
	if ff == nil || substituted || resolved != family {
		t.Fatalf("native font unavailable: resolved=%q substituted=%v", resolved, substituted)
	}
	face := newFace(ff, 18, canvas.FontRegular)
	// Match the word/space composition used by wrapping, independently in mm.
	var widthMM float64
	for i, word := range []string{"Illustrative", "test", "data", "for", "review"} {
		if i > 0 {
			widthMM += canvas.NewTextLine(face, " ", canvas.Left).Bounds().W()
		}
		widthMM += canvas.NewTextLine(face, word, canvas.Left).Bounds().W()
	}
	for _, tc := range []struct {
		name  string
		width int64
		lines int
	}{
		{"one_emu_too_narrow", int64(math.Ceil(widthMM*36000)) - 1, 2},
		{"first_fitting_emu", int64(math.Ceil(widthMM * 36000)), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MeasureStyledRuns(StyledMeasureParams{Runs: []StyledRun{{Text: text}}, FontName: family, FontPt: 18, WidthEMU: tc.width, MaxLines: 1})
			if err != nil || got.Lines != tc.lines || got.Fits != (tc.lines == 1) {
				t.Errorf("styled width=%d: %+v err=%v, want %d lines", tc.width, got, err, tc.lines)
			}
			if lines := wrapText(face, text, float64(tc.width)/12700); lines != tc.lines {
				t.Errorf("plain width=%d: %d lines, want %d", tc.width, lines, tc.lines)
			}
			insetWidth := tc.width + 2*91440 // exactly 7.2pt per side
			withInsets, err := MeasureStyledRuns(StyledMeasureParams{Runs: []StyledRun{{Text: text}}, FontName: family, FontPt: 18, WidthEMU: insetWidth, MaxLines: 1, InsetsPt: [4]float64{7.2, 0, 7.2, 0}})
			if err != nil || withInsets.Lines != tc.lines || withInsets.Fits != (tc.lines == 1) {
				t.Errorf("styled insets width=%d: %+v err=%v, want %d lines", insetWidth, withInsets, err, tc.lines)
			}
			plain, err := MeasureRun(text, family, 18, insetWidth, 1)
			if err != nil || plain.Lines != tc.lines || plain.Fits != (tc.lines == 1) {
				t.Errorf("plain insets width=%d: %+v err=%v, want %d lines", insetWidth, plain, err, tc.lines)
			}
		})
	}
	wordMM := canvas.NewTextLine(face, "Illustrative", canvas.Left).Bounds().W()
	tooNarrowFor18pt := int64(math.Ceil(wordMM*36000)) - 1 + 2*91440
	if got := MaxFontForWidth("Illustrative", tooNarrowFor18pt, family); got != 1799 {
		t.Errorf("largest fitting font=%d hundredths pt, want 1799 (18pt exceeds width by one EMU)", got)
	}
}
