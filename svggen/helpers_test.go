package svggen

import (
	"testing"
)

// colorPtr returns a pointer to the given Color.
func colorPtr(c Color) *Color {
	return &c
}

func TestResolveColors(t *testing.T) {
	style := DefaultStyleGuide()
	accentCount := len(style.Palette.AccentColors())

	tests := []struct {
		name         string
		configColors []Color
		count        int
	}{
		{name: "3 series (under accent count)", count: 3},
		{name: "6 series (equal to accent count)", count: accentCount},
		{name: "7 series (one over accent count)", count: 7},
		{name: "10 series", count: 10},
		{name: "15 series", count: 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			colors := resolveColors(tc.configColors, style, tc.count)
			if len(colors) != tc.count {
				t.Errorf("resolveColors returned %d colors, want %d", len(colors), tc.count)
			}
		})
	}
}

func TestResolveColorsFirstCycleMatchesAccents(t *testing.T) {
	style := DefaultStyleGuide()
	accents := style.Palette.AccentColors()

	// Request more than the accent count so cycling kicks in.
	colors := resolveColors(nil, style, 10)

	// The first len(accents) colors should be identical to the accents.
	for i, a := range accents {
		if colors[i] != a {
			t.Errorf("color[%d] = %v, want accent %v", i, colors[i], a)
		}
	}

	// Colors beyond the accent count should differ from their base.
	n := len(accents)
	for i := n; i < len(colors); i++ {
		base := accents[i%n]
		if colors[i] == base {
			t.Errorf("cycled color[%d] is identical to its base accent[%d] (%v); expected a shifted variant", i, i%n, base)
		}
	}
}

func TestResolveColorsWithConfigColorsExact(t *testing.T) {
	style := DefaultStyleGuide()
	custom := []Color{
		MustParseColor("#FF0000"),
		MustParseColor("#00FF00"),
		MustParseColor("#0000FF"),
	}

	colors := resolveColors(custom, style, 3)
	for i, c := range custom {
		if colors[i] != c {
			t.Errorf("color[%d] = %v, want %v", i, colors[i], c)
		}
	}
}

func TestResolveColorsWithConfigColorsFallsBackToAccents(t *testing.T) {
	style := DefaultStyleGuide()
	// Only 2 config colors but need 5 — should fall back to accent palette.
	custom := []Color{
		MustParseColor("#FF0000"),
		MustParseColor("#00FF00"),
	}

	colors := resolveColors(custom, style, 5)
	if len(colors) != 5 {
		t.Fatalf("got %d colors, want 5", len(colors))
	}
	// Should be the first 5 accent colors, not the custom ones.
	accents := style.Palette.AccentColors()
	for i := 0; i < 5; i++ {
		if colors[i] != accents[i] {
			t.Errorf("color[%d] = %v, want accent %v", i, colors[i], accents[i])
		}
	}
}

func TestResolveColorsZeroCount(t *testing.T) {
	style := DefaultStyleGuide()
	colors := resolveColors(nil, style, 0)
	if colors != nil {
		t.Errorf("expected nil for count=0, got %v", colors)
	}
}

func TestResolveColorsNoDuplicatesAcrossCycles(t *testing.T) {
	style := DefaultStyleGuide()
	count := 15
	colors := resolveColors(nil, style, count)

	// Verify no two colors in the result are exactly the same.
	for i := 0; i < len(colors); i++ {
		for j := i + 1; j < len(colors); j++ {
			if colors[i] == colors[j] {
				t.Errorf("duplicate color at indices %d and %d: %v", i, j, colors[i])
			}
		}
	}
}

func TestRGBToHSLRoundTrip(t *testing.T) {
	testColors := []Color{
		MustParseColor("#4E79A7"),
		MustParseColor("#FF0000"),
		MustParseColor("#00FF00"),
		MustParseColor("#0000FF"),
		MustParseColor("#808080"),
		MustParseColor("#FFFFFF"),
		MustParseColor("#000000"),
	}

	for _, c := range testColors {
		h, s, l := rgbToHSL(c.R, c.G, c.B)
		r, g, b := hslToRGB(h, s, l)
		// Allow +-1 for rounding.
		if diff(c.R, r) > 1 || diff(c.G, g) > 1 || diff(c.B, b) > 1 {
			t.Errorf("round-trip failed for %v: got (%d,%d,%d) from HSL(%.1f,%.3f,%.3f)",
				c.Hex(), r, g, b, h, s, l)
		}
	}
}

func diff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
