package svggen

import "math"

// MinSeriesDeltaE is the smallest CIE76 colour difference (ΔE*ab) two
// automatically assigned chart series may have. Below it, neighbouring bars,
// lines or slices read as one colour: single-hue templates put two saturated
// oranges (ΔE ≈ 22) or two slate blues (ΔE ≈ 13) in adjacent accent slots.
// Tableau 10's closest pair is ≈ 35, so well-separated palettes are untouched
// (go-slide-creator-iry0p).
const MinSeriesDeltaE = 25.0

// deltaE76 returns the CIE76 colour difference between two opaque colours.
func deltaE76(a, b Color) float64 {
	l1, a1, b1 := a.lab()
	l2, a2, b2 := b.lab()
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// lab converts the colour to CIE L*a*b* (D65 white point).
func (c Color) lab() (l, a, b float64) {
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	r, g, bl := lin(c.R), lin(c.G), lin(c.B)
	x := (r*0.4124 + g*0.3576 + bl*0.1805) / 0.95047
	y := r*0.2126 + g*0.7152 + bl*0.0722
	z := (r*0.0193 + g*0.1192 + bl*0.9505) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

// distinctSeriesPalette orders the palette's accents so that the first count
// series colours are pairwise at least MinSeriesDeltaE apart. Accents keep
// their template order; one too close to an already chosen colour is deferred
// rather than dropped. When the template's own accents cannot supply count
// distinct colours, theme text colours (dk2 / dk1), a shade of accent1 and a
// neutral grey are tried before the deferred accents are used anyway. Every
// returned colour is a template colour or derived from one, so a
// well-separated palette comes back unchanged.
//
// Only a chart that leaves some accents unused is reordered: when every
// palette slot is needed anyway, the template's declared data_palette
// sequence is kept exactly (TestChartPalette_PerTemplate).
func distinctSeriesPalette(p *Palette, count int) []Color {
	accents := p.AccentColors()
	if count <= 1 || count >= len(accents) {
		return accents
	}
	chosen := make([]Color, 0, len(accents)+4)
	var deferred []Color
	farEnough := func(c Color) bool {
		for _, other := range chosen {
			if deltaE76(c, other) < MinSeriesDeltaE {
				return false
			}
		}
		return true
	}
	for _, c := range accents {
		if len(chosen) < count && farEnough(c) {
			chosen = append(chosen, c)
		} else {
			deferred = append(deferred, c)
		}
	}
	if len(chosen) < count {
		fallbacks := []Color{
			p.TextSecondary,
			p.TextPrimary,
			accents[0].Darken(0.45),
			MustParseColor("#8C8C8C"),
		}
		for _, c := range fallbacks {
			if len(chosen) >= count {
				break
			}
			if c.A == 0 || c.ContrastWith(p.Background) < 2 || !farEnough(c) {
				continue
			}
			chosen = append(chosen, c)
		}
	}
	return append(chosen, deferred...)
}
