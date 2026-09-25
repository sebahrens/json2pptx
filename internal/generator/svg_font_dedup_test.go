package generator

import (
	"strings"
	"testing"
)

func TestStripTemplateSystemFontFaces(t *testing.T) {
	input := `<svg><text style="font-family:Calibri, Helvetica, sans-serif">A</text><style>` +
		`@font-face{font-family:'Calibri';src:url('data:font/ttf;base64,AAEAAAAA');}` +
		`@font-face{font-family:'Calibri';font-weight:700;src:url('data:font/ttf;base64,AAEAAAAA');}` +
		`@font-face{font-family:'Custom Serif';src:url('data:font/otf;base64,T1RUTwAA');}` +
		`</style></svg>`
	for _, tc := range []struct {
		name, template string
		wantFaces      int
	}{
		{"matching system font", "Calibri", 1},
		{"spaced system font", " Calibri ", 1},
		{"different system font", "Arial", 3},
		{"custom template font", "Custom Serif", 3},
		{"missing template font", "", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := string(StripTemplateSystemFontFaces([]byte(input), tc.template))
			if n := strings.Count(got, "@font-face"); n != tc.wantFaces {
				t.Errorf("font face count = %d, want %d: %s", n, tc.wantFaces, got)
			}
			if !strings.Contains(got, `font-family:Calibri, Helvetica, sans-serif`) {
				t.Error("chart text lost its font-family fallback")
			}
			if !strings.Contains(got, "data:font/otf;base64,T1RUTwAA") {
				t.Error("custom non-template font lost its embedded face")
			}
		})
	}
}

func TestStripTemplateSystemFontFacesOnRenderedChart(t *testing.T) {
	svg := renderPlaceholderDiagramSVG(t, "Arial", "")
	if !strings.Contains(svg, "@font-face") {
		t.Skip("no embeddable Arial face on this platform")
	}
	stripped := StripTemplateSystemFontFaces([]byte(svg), "Arial")
	t.Logf("rendered chart SVG bytes: embedded=%d template-font-reference=%d", len(svg), len(stripped))
	if len(stripped) >= len(svg) || strings.Contains(string(stripped), "@font-face{font-family:'Arial'") {
		t.Fatalf("Arial chart SVG did not shed its embedded template face: before=%d after=%d", len(svg), len(stripped))
	}
	if !strings.Contains(string(stripped), "font-family:Arial") {
		t.Error("Arial chart text lost its family declaration")
	}
}
