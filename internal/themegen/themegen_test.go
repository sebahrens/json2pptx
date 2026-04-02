package themegen

import (
	"fmt"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestHexToHSL(t *testing.T) {
	tests := []struct {
		hex        string
		wantH      float64 // degrees
		wantS      float64 // 0-1
		wantL      float64 // 0-1
		tolerance  float64
	}{
		{"#FF0000", 0, 1.0, 0.5, 1.0},       // pure red
		{"#00FF00", 120, 1.0, 0.5, 1.0},      // pure green
		{"#0000FF", 240, 1.0, 0.5, 1.0},      // pure blue
		{"#FFFFFF", 0, 0.0, 1.0, 1.0},        // white (achromatic)
		{"#000000", 0, 0.0, 0.0, 1.0},        // black (achromatic)
		{"#808080", 0, 0.0, 0.502, 1.0},      // gray (achromatic)
		{"#E31837", 350, 0.8, 0.49, 2.0},     // brand red (approximate)
		{"#2E5090", 219, 0.51, 0.37, 2.0},    // midnight-blue accent1 (approximate)
		{"2E7D32", 123, 0.46, 0.34, 2.0},     // forest-green accent1 (no # prefix)
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			h, s, l, err := hexToHSL(tt.hex)
			if err != nil {
				t.Fatalf("hexToHSL(%q) error: %v", tt.hex, err)
			}
			if math.Abs(h-tt.wantH) > tt.tolerance {
				t.Errorf("H: got %.1f, want %.1f (±%.1f)", h, tt.wantH, tt.tolerance)
			}
			if math.Abs(s-tt.wantS) > 0.05 {
				t.Errorf("S: got %.3f, want %.3f", s, tt.wantS)
			}
			if math.Abs(l-tt.wantL) > 0.02 {
				t.Errorf("L: got %.3f, want %.3f", l, tt.wantL)
			}
		})
	}
}

func TestHSLRoundTrip(t *testing.T) {
	// Test that RGB → HSL → RGB round-trips correctly.
	colors := []struct{ r, g, b uint8 }{
		{255, 0, 0},
		{0, 255, 0},
		{0, 0, 255},
		{128, 128, 128},
		{46, 80, 144},    // #2E5090
		{227, 24, 55},    // #E31837
		{46, 125, 50},    // #2E7D32
		{230, 74, 25},    // #E64A19
		{255, 255, 255},
		{0, 0, 0},
	}

	for _, c := range colors {
		t.Run(fmt.Sprintf("%d_%d_%d", c.r, c.g, c.b), func(t *testing.T) {
			h, s, l := rgbToHSL(c.r, c.g, c.b)
			r2, g2, b2 := hslToRGB(h, s, l)
			if absDiff(c.r, r2) > 1 || absDiff(c.g, g2) > 1 || absDiff(c.b, b2) > 1 {
				t.Errorf("round-trip failed: (%d,%d,%d) → (%.1f,%.3f,%.3f) → (%d,%d,%d)",
					c.r, c.g, c.b, h, s, l, r2, g2, b2)
			}
		})
	}
}

func TestGenerateThemeOverride(t *testing.T) {
	tests := []struct {
		name      string
		brandHex  string
		wantErr   bool
	}{
		{"red brand", "#E31837", false},
		{"blue brand", "#2E5090", false},
		{"green brand", "#2E7D32", false},
		{"coral brand", "#E64A19", false},
		{"no hash prefix", "4E79A7", false},
		{"invalid hex", "#GGGGGG", true},
		{"too short", "#FFF", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ov, err := GenerateThemeOverride(tt.brandHex)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Must produce all required OOXML color slots
			requiredColors := []string{
				"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
				"dk1", "dk2", "lt1", "lt2", "hlink", "folHlink",
			}
			for _, name := range requiredColors {
				hex, ok := ov.Colors[name]
				if !ok {
					t.Errorf("missing color %q", name)
					continue
				}
				if len(hex) != 7 || hex[0] != '#' {
					t.Errorf("color %q has invalid hex: %q", name, hex)
				}
			}

			// Accent1 must be the brand color (or very close to it)
			if ov.Colors["accent1"] != normalizeHex(tt.brandHex) {
				t.Errorf("accent1: got %q, want %q", ov.Colors["accent1"], normalizeHex(tt.brandHex))
			}

			// dk1 should be very dark
			_, _, dk1L, _ := hexToHSL(ov.Colors["dk1"])
			if dk1L > 0.15 {
				t.Errorf("dk1 lightness too high: %.3f (want < 0.15)", dk1L)
			}

			// lt1 should be white
			if ov.Colors["lt1"] != "#FFFFFF" {
				t.Errorf("lt1: got %q, want #FFFFFF", ov.Colors["lt1"])
			}

			// lt2 should be very light
			_, _, lt2L, _ := hexToHSL(ov.Colors["lt2"])
			if lt2L < 0.85 {
				t.Errorf("lt2 lightness too low: %.3f (want > 0.85)", lt2L)
			}

			// Fonts should be empty (not overriding template fonts)
			if ov.TitleFont != "" || ov.BodyFont != "" {
				t.Errorf("fonts should be empty, got title=%q body=%q", ov.TitleFont, ov.BodyFont)
			}
		})
	}
}

func TestAccentColorDistinctness(t *testing.T) {
	// Generated accent colors should be visually distinct from each other.
	ov, err := GenerateThemeOverride("#2E5090")
	if err != nil {
		t.Fatal(err)
	}

	accents := []string{
		ov.Colors["accent1"],
		ov.Colors["accent2"],
		ov.Colors["accent3"],
		ov.Colors["accent4"],
		ov.Colors["accent5"],
		ov.Colors["accent6"],
	}

	// Check that no two accents are identical
	for i := 0; i < len(accents); i++ {
		for j := i + 1; j < len(accents); j++ {
			if accents[i] == accents[j] {
				t.Errorf("accent%d and accent%d are identical: %s", i+1, j+1, accents[i])
			}
		}
	}
}

func TestResolveBrandColor(t *testing.T) {
	t.Run("empty brand returns explicit", func(t *testing.T) {
		explicit := &types.ThemeOverride{Colors: map[string]string{"accent1": "#FF0000"}}
		result, err := ResolveBrandColor("", explicit)
		if err != nil {
			t.Fatal(err)
		}
		if result != explicit {
			t.Error("expected explicit override returned as-is")
		}
	})

	t.Run("brand only", func(t *testing.T) {
		result, err := ResolveBrandColor("#E31837", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Colors) != 12 {
			t.Errorf("expected 12 colors, got %d", len(result.Colors))
		}
	})

	t.Run("brand with explicit merge", func(t *testing.T) {
		explicit := &types.ThemeOverride{
			Colors:    map[string]string{"accent2": "#CUSTOM"},
			TitleFont: "Impact",
		}
		result, err := ResolveBrandColor("#2E5090", explicit)
		if err != nil {
			t.Fatal(err)
		}
		// accent2 should be the explicit value, not generated
		if result.Colors["accent2"] != "#CUSTOM" {
			t.Errorf("accent2: got %q, want #CUSTOM", result.Colors["accent2"])
		}
		// accent1 should still be the generated brand color
		if result.Colors["accent1"] == "" {
			t.Error("accent1 should be generated")
		}
		if result.TitleFont != "Impact" {
			t.Errorf("TitleFont: got %q, want Impact", result.TitleFont)
		}
	})

	t.Run("invalid brand returns error", func(t *testing.T) {
		_, err := ResolveBrandColor("#GGGGGG", nil)
		if err == nil {
			t.Error("expected error for invalid brand color")
		}
	})
}

// normalizeHex ensures "#" prefix and uppercase.
func normalizeHex(hex string) string {
	if hex[0] != '#' {
		hex = "#" + hex
	}
	return hslToHex(mustHSL(hex))
}

func mustHSL(hex string) (float64, float64, float64) {
	h, s, l, _ := hexToHSL(hex)
	return h, s, l
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
