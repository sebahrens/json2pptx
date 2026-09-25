package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// heatmapThemes are the bundled templates' palettes. The heatmap paints its
// cells in tints of accent1 (sequential) or accent1/accent2 (diverging), so
// those and the two text roles are what decide legibility.
var heatmapThemes = map[string][]types.ThemeColor{
	"forest-green": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "2E7D32"}, {Name: "accent2", RGB: "FF8F00"},
	},
	"midnight-blue": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "2E5090"}, {Name: "accent2", RGB: "D4463A"},
	},
	"modern-template": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "B5485A"}, {Name: "accent2", RGB: "4A7AB5"},
	},
	"warm-coral": {
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "E64A19"}, {Name: "accent2", RGB: "5D4037"},
	},
}

// schemeHex resolves a scheme name against a test palette.
func schemeHex(t *testing.T, scheme string, colors []types.ThemeColor) svggen.Color {
	t.Helper()
	c, err := svggen.ParseColor(resolveSchemeColorToHex(scheme, colors))
	if err != nil {
		t.Fatalf("resolve %q: %v", scheme, err)
	}
	return c
}

// Every cell value was written in dk1, so "92" on the darkest tile of a
// sequential scale was near-black on dark green or dark blue — invisible, on an
// engine that advertises WCAG AA enforcement everywhere else. The diagram's own
// text never passed through the contrast pass (go-slide-creator-vdvs).
func TestHeatmapValueContrastOnEveryTheme(t *testing.T) {
	// The value range of the reported fixture (heatmap__real.json).
	values := []float64{92, 85, 78, 71, 64, 52, 41, 33, 20, 12}
	minVal, maxVal := 12.0, 92.0

	for name, colors := range heatmapThemes {
		t.Run(name, func(t *testing.T) {
			white := schemeHex(t, "lt1", colors)
			for _, scale := range []string{"sequential", "diverging"} {
				for _, v := range values {
					tone := heatmapCellFill(v, minVal, maxVal, scale)
					textScheme := heatmapValueColor(tone, colors)

					base := schemeHex(t, tone.scheme, colors)
					cell := patterns.EffectiveColorMods(base, patterns.ColorMods{Tint: tone.lumMod}, white)
					text := schemeHex(t, textScheme, colors)

					if ratio := text.ContrastWith(cell); ratio < svggen.WCAGAANormal {
						t.Errorf("%s %s value %.0f: %s on %s reads at %.2f:1, want >= %.1f",
							name, scale, v, textScheme, cell.Hex(), ratio, svggen.WCAGAANormal)
					}
				}
			}
		})
	}
}

// The saturated end of the scale must flip to light text and the pale end must
// stay dark: a rule that answered "lt1" everywhere would pass the ratio check
// above while making the light tiles unreadable.
func TestHeatmapValueColorFlipsWithTheTint(t *testing.T) {
	colors := heatmapThemes["forest-green"]
	darkest := heatmapCellFill(92, 12, 92, "sequential")
	palest := heatmapCellFill(12, 12, 92, "sequential")

	if got := heatmapValueColor(darkest, colors); got != "lt1" {
		t.Errorf("the darkest tile prints its value in %q, want lt1", got)
	}
	if got := heatmapValueColor(palest, colors); got != "dk1" {
		t.Errorf("the palest tile prints its value in %q, want dk1", got)
	}
}

// Without a theme there is nothing to measure, so the historical dk1 stands.
func TestHeatmapValueColorWithoutTheme(t *testing.T) {
	tone := heatmapCellFill(92, 12, 92, "sequential")
	if got := heatmapValueColor(tone, nil); got != "dk1" {
		t.Errorf("with no theme the value colour is %q, want dk1", got)
	}
}

func TestHeatmapTintAndValueContrastOnSaturatedAccents(t *testing.T) {
	colors := []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "0097A7"}, {Name: "accent2", RGB: "E60000"},
	}
	white := schemeHex(t, "lt1", colors)
	for _, scale := range []string{"sequential", "diverging"} {
		for _, value := range []float64{0, 25, 50, 75, 100} {
			tone := heatmapCellFill(value, 0, 100, scale)
			var xml bytes.Buffer
			tone.fill().WriteTo(&xml)
			if tone.lumOff > 0 && !strings.Contains(xml.String(), `<a:tint val="`) {
				t.Errorf("%s %.0f: expected RGB tint, got %s", scale, value, xml.String())
			}
			if strings.Contains(xml.String(), "lumMod") || strings.Contains(xml.String(), "lumOff") {
				t.Errorf("%s %.0f: HSL modifier still present: %s", scale, value, xml.String())
			}
			base := schemeHex(t, tone.scheme, colors)
			cell := patterns.EffectiveColorMods(base, patterns.ColorMods{Tint: tone.lumMod}, white)
			text := schemeHex(t, heatmapValueColor(tone, colors), colors)
			if ratio := text.ContrastWith(cell); ratio < svggen.WCAGAANormal {
				t.Errorf("%s %.0f: value contrast %.2f:1 on %s", scale, value, ratio, cell.Hex())
			}
		}
	}
}
