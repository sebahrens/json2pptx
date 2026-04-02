// Package svggen provides SVG diagram generation using canvas primitives.
// This file contains shared helper functions to reduce boilerplate across diagram types.
package svggen

import (
	"fmt"
	"math"
	"strings"
)

// resolveColors returns a color palette of exactly count colors for chart rendering.
// It checks configColors first (user-specified), falling back to the style guide's accent palette.
// When the available base colors are fewer than count, it cycles through the base palette
// using modulo indexing. On each full cycle past the first, colors are shifted slightly
// in hue, saturation and lightness so that cycled entries remain visually distinguishable
// from their base color.
func resolveColors(configColors []Color, style *StyleGuide, count int) []Color {
	if count <= 0 {
		return nil
	}

	// Determine base palette: prefer user-specified colors, else accent colors.
	base := configColors
	if len(base) < count {
		base = style.Palette.AccentColors()
	}

	// If the base already has enough colors, return the slice directly.
	if len(base) >= count {
		return base[:count]
	}

	// Cycle through the base palette, shifting hue/saturation/lightness on each
	// full pass to keep repeated entries visually distinct.
	result := make([]Color, count)
	n := len(base)
	for i := 0; i < count; i++ {
		src := base[i%n]
		cycle := i / n // 0 for the first pass (no shift)
		if cycle == 0 {
			result[i] = src
		} else {
			result[i] = shiftColor(src, cycle)
		}
	}
	return result
}

// shiftColor applies a perceptible hue/saturation/lightness shift to a color.
// The cycle parameter (1, 2, 3 ...) controls how far the color is shifted from
// its base, ensuring each cycle produces a distinct variant.
func shiftColor(c Color, cycle int) Color {
	h, s, l := rgbToHSL(c.R, c.G, c.B)

	// Shift hue by 30 degrees per cycle (wrapping at 360).
	h = math.Mod(h+float64(cycle)*30, 360)

	// Alternate lightness: odd cycles lighten, even cycles darken.
	if cycle%2 == 1 {
		l = math.Min(0.85, l+0.10)
	} else {
		l = math.Max(0.25, l-0.10)
	}

	// Pull saturation toward 0.5 slightly to keep shifted colors within gamut.
	s = s + (0.5-s)*0.10*float64(cycle)
	s = math.Max(0.05, math.Min(1.0, s))

	r, g, b := hslToRGB(h, s, l)
	return Color{R: r, G: g, B: b, A: c.A}
}

// rgbToHSL converts 8-bit RGB values to HSL (hue 0-360, saturation 0-1, lightness 0-1).
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	l = (maxC + minC) / 2

	if maxC == minC {
		return 0, 0, l // achromatic
	}

	d := maxC - minC
	if l > 0.5 {
		s = d / (2 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}

	switch maxC {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60

	return h, s, l
}

// hslToRGB converts HSL (hue 0-360, saturation 0-1, lightness 0-1) to 8-bit RGB.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
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

	hNorm := h / 360

	r := hueToRGB(p, q, hNorm+1.0/3.0)
	g := hueToRGB(p, q, hNorm)
	b := hueToRGB(p, q, hNorm-1.0/3.0)

	return uint8(math.Round(r * 255)), uint8(math.Round(g * 255)), uint8(math.Round(b * 255))
}

// hueToRGB is a helper for the HSL-to-RGB conversion.
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

// truncateText truncates text to fit within maxWidth at the given fontSize,
// using an approximate average character width of 0.55 * fontSize.
func truncateText(text string, maxWidth, fontSize float64) string {
	avgCharWidth := fontSize * 0.55
	maxChars := int(maxWidth / avgCharWidth)
	if maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

// parseStringList coerces a value (typically from JSON-decoded map data) into
// a []string. It handles []any (with string elements) and []string directly.
func parseStringList(v any) []string {
	if v == nil {
		return nil
	}
	switch items := v.(type) {
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return items
	default:
		return nil
	}
}

// validateCategoriesAndSeries validates that a chart request has valid categories and series data.
// chartType is used in error messages (e.g., "bar_chart", "stacked_bar_chart").
// requireCategories controls whether categories are mandatory.
// minSeries is the minimum number of series required (typically 1, but 2 for grouped_bar_chart).
func validateCategoriesAndSeries(data map[string]any, chartType string, requireCategories bool, minSeries int) error {
	// Accept "labels" as alias for "categories" (common in LLM-generated data).
	normalizeCategoryAliases(data)

	if requireCategories {
		categories, hasCats := data["categories"]
		if !hasCats {
			return &ValidationError{Field: "data.categories", Code: ErrCodeRequired, Message: chartType + " requires 'categories' field. Expected format: {\"categories\": [\"Q1\", \"Q2\", \"Q3\"], \"series\": [{\"name\": \"Revenue\", \"values\": [10, 20, 30]}]}"}
		}
		catSlice, ok := toStringSlice(categories)
		if !ok || len(catSlice) == 0 {
			return &ValidationError{Field: "data.categories", Code: ErrCodeInvalidType, Message: chartType + " 'categories' must be a non-empty array of strings, e.g. [\"Q1\", \"Q2\", \"Q3\"]", Value: categories}
		}
	}

	series, hasSeries := data["series"]
	if !hasSeries {
		return &ValidationError{Field: "data.series", Code: ErrCodeRequired, Message: chartType + " requires 'series' field. Expected format: {\"series\": [{\"name\": \"Revenue\", \"values\": [10, 20, 30]}]}"}
	}

	seriesSlice, ok := toSeriesSlice(series)
	if !ok || len(seriesSlice) < minSeries {
		if minSeries > 1 {
			return &ValidationError{Field: "data.series", Code: ErrCodeConstraint, Message: fmt.Sprintf("%s 'series' must have at least %d series, each as {\"name\": \"...\", \"values\": [...]}", chartType, minSeries), Value: series}
		}
		return &ValidationError{Field: "data.series", Code: ErrCodeInvalidType, Message: chartType + " 'series' must be a non-empty array of objects, e.g. [{\"name\": \"Revenue\", \"values\": [10, 20, 30]}]", Value: series}
	}

	return nil
}

// normalizeCategoryAliases promotes common aliases ("labels", "x_labels") to
// the canonical "categories" key so validators and extractors work uniformly.
// The map is mutated in place; if "categories" already exists, aliases are ignored.
func normalizeCategoryAliases(data map[string]any) {
	if _, has := data["categories"]; has {
		return
	}
	for _, alias := range []string{"labels", "x_labels"} {
		if v, ok := data[alias]; ok {
			data["categories"] = v
			return
		}
	}
}
