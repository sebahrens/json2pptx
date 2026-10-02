package pptx

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// go-slide-creator-ohhb2: pattern sizing measures a Calibri theme in its
// metric clone Carlito; the writer measured every body in Liberation Sans.

func TestAutofitMeasureFace(t *testing.T) {
	body := func(fonts ThemeFonts, families ...string) *TextBody {
		tb := &TextBody{AutoFit: "normAutofit", ThemeFonts: fonts}
		for _, f := range families {
			tb.Paragraphs = append(tb.Paragraphs, Paragraph{Runs: []Run{{Text: "Revenue", FontSize: 1400, FontFamily: f}}})
		}
		return tb
	}
	calibri := ThemeFonts{Major: "Calibri Light", Minor: "Calibri"}
	for name, c := range map[string]struct {
		tb   *TextBody
		want autofitFace
	}{
		"no theme fonts":          {body(ThemeFonts{}, "+mn-lt"), autofitFace{name: autofitFontName}},
		"calibri body":            {body(calibri, "+mn-lt", ""), autofitFace{name: "Calibri", exact: true}},
		"calibri light heading":   {body(calibri, "+mj-lt"), autofitFace{name: autofitFontName}},
		"mixed faces":             {body(ThemeFonts{Major: "Lora", Minor: "Calibri"}, "+mn-lt", "+mj-lt"), autofitFace{name: autofitFontName}},
		"arial measures as twin":  {body(ThemeFonts{Minor: "Arial"}, "+mn-lt"), autofitFace{name: autofitFontName, exact: true}},
		"host-dependent face":     {body(ThemeFonts{Minor: "Segoe UI"}, "+mn-lt"), autofitFace{name: autofitFontName}},
		"explicit embedded face":  {body(calibri, "Poppins Light"), autofitFace{name: "Poppins Light", exact: true}},
		"heading font not stated": {body(ThemeFonts{Minor: "Calibri"}, "+mj-lt"), autofitFace{name: autofitFontName}},
	} {
		if got := autofitMeasureFace(c.tb); got != c.want {
			t.Errorf("%s: autofitMeasureFace = %+v, want %+v", name, got, c.want)
		}
	}
}

// A Calibri body that wraps to one more line in Liberation Sans than in
// Carlito is written with no shrink once the writer knows its theme face —
// the face pattern sizing fitted it in.
func TestAutofitMeasuresTheThemeFace(t *testing.T) {
	text := "Stakeholder interviews and current-state mapping across every regional delivery team"
	const pt = 12.0
	lines := func(font string, widthEMU int64) int {
		m, err := textfit.MeasureRun(text, font, pt, widthEMU+2*autofitMeasureSideEMU, 0)
		if err != nil {
			t.Skipf("cannot measure: %v", err)
		}
		return m.Lines
	}
	// The narrowest measure at which Carlito needs fewer lines than
	// Liberation Sans.
	var widthEMU int64
	var carlito, liberation int
	for w := 120; w <= 400 && widthEMU == 0; w++ {
		e := int64(w) * 12700
		if c, l := lines("Calibri", e), lines(autofitFontName, e); c < l {
			widthEMU, carlito, liberation = e, c, l
		}
	}
	if widthEMU == 0 {
		t.Skip("Carlito and Liberation Sans wrap alike at every probed measure")
	}
	tb := func(fonts ThemeFonts) *TextBody {
		return &TextBody{
			AutoFit: "normAutofit", Insets: [4]int64{1, 1, 1, 1}, ThemeFonts: fonts,
			Paragraphs: []Paragraph{{Runs: []Run{{Text: text, FontSize: int(pt * 100), FontFamily: "+mn-lt"}}}},
		}
	}
	// Room for exactly the Carlito line count.
	bounds := RectEmu{CX: widthEMU + 2, CY: int64(float64(carlito)*pt*autofitLineSpacing*12700) + 2 + 12700}
	if s := AutofitScaleFor(tb(ThemeFonts{Minor: "Calibri"}), bounds); s != 1 {
		t.Errorf("Calibri body sized to its Carlito fit is written at %.0f%%", s*100)
	}
	if s := AutofitScaleFor(tb(ThemeFonts{}), bounds); s >= 1 {
		t.Errorf("without theme fonts the Liberation measure needs %d lines and must shrink, got %v", liberation, s)
	}
}

// Rotated text runs its lines along the shape's height: a label whose words
// fit that line is not shrunk for the shape's narrow width.
func TestAutofitMeasuresRotatedTextAlongItsLine(t *testing.T) {
	label := &TextBody{
		AutoFit: "normAutofit", Vert: "vert270",
		Insets:     [4]int64{180000, 180000, 180000, 180000},
		Paragraphs: []Paragraph{{Runs: []Run{{Text: "Market Growth", FontSize: 1400, Bold: true}}}},
	}
	tall := RectEmu{CX: 880392, CY: 2537208} // matrix-2x2 axis column on midnight-blue
	if s := AutofitScaleFor(label, tall); s != 1 {
		t.Errorf("vert270 label written at %.0f%% in a %dx%d EMU column", s*100, tall.CX, tall.CY)
	}
}
