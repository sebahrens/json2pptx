package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// forestGreenTheme mirrors templates/forest-green.pptx's color scheme.
var forestGreenTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"},
	{Name: "lt1", RGB: "#FFFFFF"},
	{Name: "dk2", RGB: "#1A3C34"},
	{Name: "lt2", RGB: "#EDF5F0"},
	{Name: "accent1", RGB: "#2E7D32"},
	{Name: "accent2", RGB: "#FF8F00"},
}

// go-slide-creator-sis2: white text on forest-green accent2 (orange) must be
// fixed to a template color (dk2), not literal #000000.
func TestContrastFix_WhiteOnAccent2PicksThemeColor(t *testing.T) {
	shape := []byte(`<p:sp><p:spPr><a:solidFill><a:schemeClr val="accent2"/></a:solidFill></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>Situation</a:t></a:r></a:p>` +
		`<a:p><a:r><a:rPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:rPr><a:t>Detail</a:t></a:r></a:p></p:txBody></p:sp>`)
	fixed, swaps := fixShapeXMLContrast(shape, forestGreenTheme, nil)
	if len(swaps) != 2 {
		t.Fatalf("expected 2 swaps, got %d: %s", len(swaps), fixed)
	}
	for _, s := range swaps {
		if s.ReplacedColor == "#000000" {
			t.Errorf("replacement is literal black; want a theme color: %+v", s)
		}
		if s.ReplacedColor != "#1A3C34" {
			t.Errorf("replacement = %s, want dk2 #1A3C34", s.ReplacedColor)
		}
		if s.RatioAfter < svggen.WCAGAANormal {
			t.Errorf("ratio after = %.2f, want >= AA 4.5", s.RatioAfter)
		}
	}
	if strings.Contains(string(fixed), `val="000000"`) {
		t.Errorf("output still contains literal black: %s", fixed)
	}
}

func TestPickThemeTextColor_Order(t *testing.T) {
	t.Run("dark_fill_keeps_lt1", func(t *testing.T) {
		c := pickThemeTextColor(svggen.MustParseColor("#1B2A4A"), forestGreenTheme, svggen.WCAGAALarge)
		if c.Scheme != "lt1" {
			t.Errorf("scheme = %q, want lt1", c.Scheme)
		}
	})
	t.Run("dk2_too_close_falls_to_dk1", func(t *testing.T) {
		// Mid-light fill where dk2 (dark green) misses 4.5 but dk1 passes.
		theme := []types.ThemeColor{{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#555555"}}
		c := pickThemeTextColor(svggen.MustParseColor("#9E9E9E"), theme, svggen.WCAGAALarge)
		if c.Scheme != "dk1" {
			t.Errorf("scheme = %q (%s), want dk1", c.Scheme, c.Hex)
		}
	})
	t.Run("shade_when_no_theme_slot_meets_AA", func(t *testing.T) {
		// Only lt1 and a dk2 identical to the fill: no theme slot passes, the
		// tonal shade of the fill does.
		theme := []types.ThemeColor{{Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#FF8F00"}}
		c := pickThemeTextColor(svggen.MustParseColor("#FF8F00"), theme, svggen.WCAGAALarge)
		if c.Scheme != "" {
			t.Errorf("scheme = %q, want derived shade", c.Scheme)
		}
		if c.Color.ContrastWith(svggen.MustParseColor("#FF8F00")) < svggen.WCAGAALarge {
			t.Errorf("shade %s does not meet AA large", c.Hex)
		}
	})
	t.Run("no_theme_falls_back_to_extremes", func(t *testing.T) {
		c := pickThemeTextColor(svggen.MustParseColor("#FFFFFF"), nil, svggen.WCAGAALarge)
		if c.Hex != "#000000" {
			t.Errorf("hex = %s, want #000000", c.Hex)
		}
	})
}
