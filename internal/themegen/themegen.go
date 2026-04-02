// Package themegen generates complete PPTX theme color schemes from a single brand color
// using color theory (HSL color space manipulation).
package themegen

import (
	"fmt"
	"math"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// GenerateThemeOverride creates a full ThemeOverride from a single brand hex color.
// It derives 6 accent colors, dark/light colors, and hyperlink colors using HSL
// color theory: complementary, analogous, triadic, and tint/shade.
func GenerateThemeOverride(brandHex string) (*types.ThemeOverride, error) {
	h, s, l, err := hexToHSL(brandHex)
	if err != nil {
		return nil, fmt.Errorf("invalid brand color %q: %w", brandHex, err)
	}

	// Generate accent colors using color theory:
	// Accent1: brand color (primary)
	// Accent2: complementary (180° rotation)
	// Accent3: analogous (30° clockwise)
	// Accent4: triadic (120° rotation)
	// Accent5: lighter tint of brand
	// Accent6: darker shade of brand
	accent1 := hslToHex(h, s, l)
	accent2 := hslToHex(rotateHue(h, 180), clamp01(s*0.85), clamp01(l*1.05))
	accent3 := hslToHex(rotateHue(h, 30), clamp01(s*0.9), clamp01(l*0.95))
	accent4 := hslToHex(rotateHue(h, 120), clamp01(s*0.8), clamp01(l*1.0))
	accent5 := hslToHex(h, clamp01(s*0.5), clamp01(minF(l+0.25, 0.85)))
	accent6 := hslToHex(h, clamp01(s*0.9), clamp01(maxF(l-0.2, 0.2)))

	// Dark colors: near-black with hint of brand hue
	dk1 := hslToHex(h, clamp01(s*0.15), 0.07) // near-black
	dk2 := hslToHex(h, clamp01(s*0.2), 0.20)  // dark brand tint

	// Light colors: near-white with subtle brand warmth
	lt1 := "#FFFFFF"                                    // pure white background
	lt2 := hslToHex(h, clamp01(s*0.15), clamp01(0.93)) // very light tinted surface

	// Hyperlink colors: brand-adjacent
	hlink := hslToHex(h, clamp01(s*0.9), clamp01(minF(l, 0.45)))
	folHlink := hslToHex(h, clamp01(s*0.4), clamp01(0.55))

	return &types.ThemeOverride{
		Colors: map[string]string{
			"accent1":  accent1,
			"accent2":  accent2,
			"accent3":  accent3,
			"accent4":  accent4,
			"accent5":  accent5,
			"accent6":  accent6,
			"dk1":      dk1,
			"dk2":      dk2,
			"lt1":      lt1,
			"lt2":      lt2,
			"hlink":    hlink,
			"folHlink": folHlink,
		},
	}, nil
}

// ResolveBrandColor generates a ThemeOverride from brandColor, merging any explicit
// overrides on top. Returns nil if brandColor is empty.
func ResolveBrandColor(brandColor string, explicit *types.ThemeOverride) (*types.ThemeOverride, error) {
	if brandColor == "" {
		return explicit, nil
	}
	generated, err := GenerateThemeOverride(brandColor)
	if err != nil {
		return nil, err
	}
	if explicit != nil {
		// Explicit theme_override values win over generated values
		for k, v := range explicit.Colors {
			generated.Colors[k] = v
		}
		if explicit.TitleFont != "" {
			generated.TitleFont = explicit.TitleFont
		}
		if explicit.BodyFont != "" {
			generated.BodyFont = explicit.BodyFont
		}
	}
	return generated, nil
}

// hexToHSL converts a hex color string to HSL values.
// Accepts formats: "#RRGGBB" or "RRGGBB".
func hexToHSL(hex string) (h, s, l float64, err error) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("expected 6 hex digits, got %d", len(hex))
	}

	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hex: %w", err)
	}

	h, s, l = rgbToHSL(r, g, b)
	return h, s, l, nil
}

// rgbToHSL converts RGB (0-255) to HSL (h: 0-360, s: 0-1, l: 0-1).
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	cmax := maxF(maxF(rf, gf), bf)
	cmin := minF(minF(rf, gf), bf)
	delta := cmax - cmin

	l = (cmax + cmin) / 2.0

	if delta == 0 {
		return 0, 0, l // achromatic
	}

	if l < 0.5 {
		s = delta / (cmax + cmin)
	} else {
		s = delta / (2.0 - cmax - cmin)
	}

	switch cmax {
	case rf:
		h = math.Mod((gf-bf)/delta, 6.0) * 60.0
	case gf:
		h = ((bf-rf)/delta + 2.0) * 60.0
	case bf:
		h = ((rf-gf)/delta + 4.0) * 60.0
	}

	if h < 0 {
		h += 360
	}
	return h, s, l
}

// hslToRGB converts HSL to RGB (0-255).
func hslToRGB(h, s, l float64) (r, g, b uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	if s == 0 {
		v := uint8(math.Round(l * 255))
		return v, v, v
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	hNorm := h / 360.0
	rF := hueToRGB(p, q, hNorm+1.0/3.0)
	gF := hueToRGB(p, q, hNorm)
	bF := hueToRGB(p, q, hNorm-1.0/3.0)

	return uint8(math.Round(rF * 255)), uint8(math.Round(gF * 255)), uint8(math.Round(bF * 255))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// hslToHex converts HSL to "#RRGGBB".
func hslToHex(h, s, l float64) string {
	r, g, b := hslToRGB(h, s, l)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func rotateHue(h, degrees float64) float64 {
	h += degrees
	if h >= 360 {
		h -= 360
	}
	if h < 0 {
		h += 360
	}
	return h
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
