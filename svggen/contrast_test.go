package svggen

import (
	"math"
	"testing"
)

func TestEnsureContrast_AlreadyCompliant(t *testing.T) {
	// Black text on white background has ratio ~21:1, well above any threshold.
	fg := MustParseColor("#000000")
	bg := MustParseColor("#FFFFFF")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	if result.Hex() != fg.Hex() {
		t.Errorf("already-compliant pair should not change: got %s, want %s", result.Hex(), fg.Hex())
	}
}

func TestEnsureContrast_WhiteOnWhite(t *testing.T) {
	// White text on white background should become dark.
	fg := MustParseColor("#FFFFFF")
	bg := MustParseColor("#FFFFFF")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(bg)

	if ratio < WCAGAANormal {
		t.Errorf("contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
	// The result should be dark since white-on-white cannot produce contrast
	// by lightening.
	if result.IsLight() {
		t.Errorf("white-on-white adjustment should produce dark color, got %s (luminance %.3f)",
			result.Hex(), result.Luminance())
	}
}

func TestEnsureContrast_DarkOnDark(t *testing.T) {
	// Dark text on dark background should be lightened to meet contrast.
	fg := MustParseColor("#1A1A1A")
	bg := MustParseColor("#000000")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(bg)

	if ratio < WCAGAANormal {
		t.Errorf("contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
	// The result should be lighter than the original fg.
	if result.Luminance() <= fg.Luminance() {
		t.Errorf("dark-on-dark adjustment should produce lighter color: result luminance %.3f <= original %.3f",
			result.Luminance(), fg.Luminance())
	}
}

func TestEnsureContrast_SameColor(t *testing.T) {
	// Exact same color for fg and bg -- ratio is 1:1, must adjust.
	mid := MustParseColor("#808080")

	result := EnsureContrast(mid, mid, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(mid)

	if ratio < WCAGAANormal {
		t.Errorf("same-color pair: contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
}

func TestEnsureContrast_PreservesAlpha(t *testing.T) {
	// The adjusted color should preserve the original alpha.
	fg := Color{R: 255, G: 255, B: 255, A: 0.7}
	bg := MustParseColor("#FFFFFF")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	if math.Abs(result.A-0.7) > 0.001 {
		t.Errorf("alpha should be preserved: got %.3f, want 0.700", result.A)
	}
}

func TestEnsureContrast_SemiTransparentBackground(t *testing.T) {
	// Semi-transparent background composited over white.
	fg := MustParseColor("#FFFFFF")
	bg := Color{R: 255, G: 255, B: 255, A: 0.5} // composites to ~#FFFFFF over white

	result := EnsureContrast(fg, bg, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(bg.Opaque())

	if ratio < WCAGAANormal {
		t.Errorf("semi-transparent bg: contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
}

func TestEnsureContrast_MinimalAdjustment(t *testing.T) {
	// A pair that is close to but below the threshold should receive a
	// minimal adjustment (not jump to pure black/white).
	bg := MustParseColor("#FFFFFF")
	// Dark gray has contrast ~12:1 with white for normal text, but let's
	// pick something that needs a small bump for AAA (7:1).
	fg := MustParseColor("#767676") // contrast ~4.54:1 with white (just above AA, below AAA)

	result := EnsureContrast(fg, bg, WCAGAAANormal)
	ratio := result.Opaque().ContrastWith(bg)

	if ratio < WCAGAAANormal {
		t.Errorf("AAA enforcement: contrast ratio = %.2f, want >= %.1f", ratio, WCAGAAANormal)
	}

	// The adjustment should not have jumped to pure black; it should be a
	// darker gray somewhere between the original and black.
	if result.Hex() == "#000000" {
		t.Errorf("expected minimal adjustment, got pure black")
	}
	if result.Luminance() >= fg.Luminance() {
		t.Errorf("result should be darker than original: result luminance %.3f >= original %.3f",
			result.Luminance(), fg.Luminance())
	}
}

func TestEnsureContrast_LargeTextThreshold(t *testing.T) {
	// A pair that meets 3:1 but not 4.5:1.
	fg := MustParseColor("#949494") // ~2.98:1 against white
	bg := MustParseColor("#FFFFFF")

	// Should fail normal AA
	ratioOrig := fg.ContrastWith(bg)
	if ratioOrig >= WCAGAANormal {
		t.Skipf("test color already meets AA normal (%.2f), pick a different color", ratioOrig)
	}

	// EnsureContrast with large text threshold should adjust
	result := EnsureContrast(fg, bg, WCAGAALarge)
	ratio := result.Opaque().ContrastWith(bg)
	if ratio < WCAGAALarge {
		t.Errorf("large text contrast ratio = %.2f, want >= %.1f", ratio, WCAGAALarge)
	}
}

func TestEnsureContrast_DarkBackground(t *testing.T) {
	// Light text on dark background that needs adjustment.
	fg := MustParseColor("#333333") // dark fg on dark bg
	bg := MustParseColor("#222222")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(bg)

	if ratio < WCAGAANormal {
		t.Errorf("dark bg contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
	// Result should be lightened since bg is dark.
	if result.Luminance() <= fg.Luminance() {
		t.Errorf("should have lightened fg: result %.3f <= original %.3f",
			result.Luminance(), fg.Luminance())
	}
}

func TestEnsureContrast_MidGrayBackground(t *testing.T) {
	// Mid-gray background is tricky: both extremes have similar contrast.
	fg := MustParseColor("#808080") // Same as bg
	bg := MustParseColor("#808080")

	result := EnsureContrast(fg, bg, WCAGAANormal)
	ratio := result.Opaque().ContrastWith(bg)

	if ratio < WCAGAANormal {
		t.Errorf("mid-gray contrast ratio = %.2f, want >= %.1f", ratio, WCAGAANormal)
	}
}

func TestEnsureWCAGAA(t *testing.T) {
	tests := []struct {
		name string
		fg   string
		bg   string
	}{
		{"white on white", "#FFFFFF", "#FFFFFF"},
		{"black on black", "#000000", "#000000"},
		{"light gray on white", "#CCCCCC", "#FFFFFF"},
		{"dark gray on black", "#333333", "#000000"},
		{"yellow on white", "#FFFF00", "#FFFFFF"},
		{"blue on dark blue", "#0000FF", "#000033"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := MustParseColor(tt.fg)
			bg := MustParseColor(tt.bg)

			result := EnsureWCAGAA(fg, bg)
			ratio := result.Opaque().ContrastWith(bg)

			if ratio < WCAGAANormal {
				t.Errorf("EnsureWCAGAA(%s, %s) = %s, ratio = %.2f, want >= %.1f",
					tt.fg, tt.bg, result.Hex(), ratio, WCAGAANormal)
			}
		})
	}
}

func TestEnsureWCAGAALarge(t *testing.T) {
	tests := []struct {
		name string
		fg   string
		bg   string
	}{
		{"white on white", "#FFFFFF", "#FFFFFF"},
		{"black on black", "#000000", "#000000"},
		{"light gray on white", "#CCCCCC", "#FFFFFF"},
		{"yellow on white", "#FFFF00", "#FFFFFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := MustParseColor(tt.fg)
			bg := MustParseColor(tt.bg)

			result := EnsureWCAGAALarge(fg, bg)
			ratio := result.Opaque().ContrastWith(bg)

			if ratio < WCAGAALarge {
				t.Errorf("EnsureWCAGAALarge(%s, %s) = %s, ratio = %.2f, want >= %.1f",
					tt.fg, tt.bg, result.Hex(), ratio, WCAGAALarge)
			}
		})
	}
}

func TestEnsureWCAGAA_AlreadyCompliantUnchanged(t *testing.T) {
	// Black on white should not be modified.
	fg := MustParseColor("#000000")
	bg := MustParseColor("#FFFFFF")

	result := EnsureWCAGAA(fg, bg)
	if result.Hex() != fg.Hex() {
		t.Errorf("compliant black on white changed: got %s, want %s", result.Hex(), fg.Hex())
	}

	// White on black should not be modified.
	fg2 := MustParseColor("#FFFFFF")
	bg2 := MustParseColor("#000000")

	result2 := EnsureWCAGAA(fg2, bg2)
	if result2.Hex() != fg2.Hex() {
		t.Errorf("compliant white on black changed: got %s, want %s", result2.Hex(), fg2.Hex())
	}
}

func TestEnsureContrast_ZeroRatio(t *testing.T) {
	// A zero min ratio should always pass -- any color pair has ratio >= 1.
	fg := MustParseColor("#808080")
	bg := MustParseColor("#808080")

	result := EnsureContrast(fg, bg, 0)
	if result.Hex() != fg.Hex() {
		t.Errorf("zero ratio should not modify fg: got %s, want %s", result.Hex(), fg.Hex())
	}
}

func TestEnsureContrast_RatioOne(t *testing.T) {
	// A ratio of 1.0 is the minimum possible contrast, so any pair passes.
	fg := MustParseColor("#808080")
	bg := MustParseColor("#808080")

	result := EnsureContrast(fg, bg, 1.0)
	if result.Hex() != fg.Hex() {
		t.Errorf("ratio 1.0 should not modify fg: got %s, want %s", result.Hex(), fg.Hex())
	}
}

func TestEnsureContrast_VeryHighRatio(t *testing.T) {
	// Request a ratio higher than what's achievable (max is 21:1 for black/white).
	fg := MustParseColor("#808080")
	bg := MustParseColor("#808080")

	result := EnsureContrast(fg, bg, 25.0)
	// Should fall back to pure black or white since 25:1 is impossible.
	ratio := result.Opaque().ContrastWith(bg)
	// The best we can get is the max contrast with bg.
	expectedMax := math.Max(
		Color{R: 0, G: 0, B: 0, A: 1.0}.ContrastWith(bg),
		Color{R: 255, G: 255, B: 255, A: 1.0}.ContrastWith(bg),
	)
	if math.Abs(ratio-expectedMax) > 0.1 {
		t.Errorf("impossible ratio should fall back to best extreme: got %.2f, want ~%.2f",
			ratio, expectedMax)
	}
}

func TestEnsureContrast_PaletteAccentColors(t *testing.T) {
	// Test with actual palette accent colors on various backgrounds.
	palette := DefaultPalette()
	accents := palette.AccentColors()

	backgrounds := []struct {
		name string
		bg   Color
	}{
		{"white", MustParseColor("#FFFFFF")},
		{"light gray", MustParseColor("#F8F9FA")},
		{"dark", MustParseColor("#212529")},
		{"mid gray", MustParseColor("#808080")},
	}

	for _, bgCase := range backgrounds {
		for i, accent := range accents {
			t.Run(bgCase.name+"/accent"+string(rune('1'+i)), func(t *testing.T) {
				result := EnsureWCAGAA(accent, bgCase.bg)
				ratio := result.Opaque().ContrastWith(bgCase.bg)

				if ratio < WCAGAANormal {
					t.Errorf("accent%d (%s) on %s (%s): adjusted to %s, ratio %.2f < %.1f",
						i+1, accent.Hex(), bgCase.name, bgCase.bg.Hex(),
						result.Hex(), ratio, WCAGAANormal)
				}
			})
		}
	}
}

func TestLerpColors(t *testing.T) {
	a := Color{R: 0, G: 0, B: 0, A: 1.0}
	b := Color{R: 255, G: 255, B: 255, A: 1.0}

	tests := []struct {
		name string
		t    float64
		want Color
	}{
		{"t=0 returns a", 0.0, Color{R: 0, G: 0, B: 0, A: 1.0}},
		{"t=1 returns b", 1.0, Color{R: 255, G: 255, B: 255, A: 1.0}},
		{"t=0.5 midpoint", 0.5, Color{R: 128, G: 128, B: 128, A: 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lerpColors(a, b, tt.t)
			// Allow +/- 1 for rounding.
			if abs(int(result.R)-int(tt.want.R)) > 1 ||
				abs(int(result.G)-int(tt.want.G)) > 1 ||
				abs(int(result.B)-int(tt.want.B)) > 1 {
				t.Errorf("lerpColors(black, white, %.1f) = (%d,%d,%d), want (%d,%d,%d)",
					tt.t, result.R, result.G, result.B, tt.want.R, tt.want.G, tt.want.B)
			}
		})
	}
}

func TestLerpColors_Clamp(t *testing.T) {
	a := Color{R: 100, G: 100, B: 100, A: 1.0}
	b := Color{R: 200, G: 200, B: 200, A: 1.0}

	// t < 0 should clamp to 0.
	result := lerpColors(a, b, -0.5)
	if result.R != a.R {
		t.Errorf("t < 0 should clamp to a: got R=%d, want %d", result.R, a.R)
	}

	// t > 1 should clamp to 1.
	result = lerpColors(a, b, 1.5)
	if result.R != b.R {
		t.Errorf("t > 1 should clamp to b: got R=%d, want %d", result.R, b.R)
	}
}

func TestEnsureContrast_ConsistencyWithContrastWith(t *testing.T) {
	// Verify that our adjustments produce genuine WCAG-compliant contrast.
	// Use the same Luminance/ContrastWith formulas from style.go.
	pairs := []struct {
		fg, bg string
	}{
		{"#FF0000", "#FFFFFF"}, // Red on white
		{"#00FF00", "#FFFFFF"}, // Green on white
		{"#0000FF", "#FFFFFF"}, // Blue on white
		{"#FFFF00", "#000000"}, // Yellow on black
		{"#FF00FF", "#000000"}, // Magenta on black
		{"#00FFFF", "#000000"}, // Cyan on black
		{"#EDC948", "#FFFFFF"}, // Tableau yellow on white (low contrast)
		{"#F28E2B", "#FFFFFF"}, // Tableau orange on white
	}

	for _, pair := range pairs {
		fg := MustParseColor(pair.fg)
		bg := MustParseColor(pair.bg)

		result := EnsureWCAGAA(fg, bg)

		// Verify using the same ContrastWith method.
		ratio := result.Opaque().ContrastWith(bg)
		if ratio < WCAGAANormal {
			t.Errorf("EnsureWCAGAA(%s, %s) = %s: ContrastWith = %.2f, want >= %.1f",
				pair.fg, pair.bg, result.Hex(), ratio, WCAGAANormal)
		}
	}
}

func TestWCAGConstants(t *testing.T) {
	// Verify the WCAG constant values are correct.
	if WCAGAANormal != 4.5 {
		t.Errorf("WCAGAANormal = %f, want 4.5", WCAGAANormal)
	}
	if WCAGAALarge != 3.0 {
		t.Errorf("WCAGAALarge = %f, want 3.0", WCAGAALarge)
	}
	if WCAGAAANormal != 7.0 {
		t.Errorf("WCAGAAANormal = %f, want 7.0", WCAGAAANormal)
	}
	if WCAGAAALarge != 4.5 {
		t.Errorf("WCAGAAALarge = %f, want 4.5", WCAGAAALarge)
	}
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
