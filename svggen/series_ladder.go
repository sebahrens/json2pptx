package svggen

import "math"

// Tint lightens the colour the way the slide engine tints an accent: OOXML
// lumMod / lumOff on HSL lightness, L' = keep·L + (1 − keep), hue and
// saturation unchanged (PowerPoint's "Lighter N%" swatches; keep 0.2 is
// "Lighter 80%"). svggen diagrams and the pattern engine therefore tint one
// accent to one colour (go-slide-creator-7if28): mixing toward white in linear
// light instead pulls a saturated orange to salmon pink.
func (c Color) Tint(keep float64) Color {
	keep = math.Max(0, math.Min(1, keep))
	h, s, l := rgbToHSL(c.R, c.G, c.B)
	r, g, b := hslToRGB(h, s, math.Min(1, l*keep+(1-keep)))
	return Color{R: r, G: g, B: b, A: c.A}
}

// Shade darkens the colour on HSL lightness (OOXML lumMod alone),
// L' = keep·L, hue and saturation unchanged.
func (c Color) Shade(keep float64) Color {
	keep = math.Max(0, math.Min(1, keep))
	h, s, l := rgbToHSL(c.R, c.G, c.B)
	r, g, b := hslToRGB(h, s, l*keep)
	return Color{R: r, G: g, B: b, A: c.A}
}

// Tonal series ladder (go-slide-creator-7bsbd).
//
// A template that declares no data_palette used to hand its raw accent slots
// to the chart. On a single-hue brand theme those are three oranges and three
// pale greys, and the visibility pass darkened the pale ones to a tan, a
// blue-grey and a brown: an arbitrary-looking default that mixes to mud. The
// ladder is built from the one colour the template is sure about, accent1:
//
//	1 accent1            the emphasised series, always the solid accent
//	2 light neutral      the comparison series ("last year", "plan")
//	3 dark neutral
//	4 accent tint        lighter than accent1
//	5 accent shade       darker than accent1 (a tint again on a dark accent)
//	6 mid neutral
//
// Every step is placed by its contrast against the chart background, on the
// grid seriesLadderGrid, so neighbouring series differ in luminance by at
// least seriesLadderStep however the viewer sees hue: a stacked bar stays
// readable in greyscale and for colour-blind readers. Accent steps keep the
// accent's hue and saturation (Tint / Shade, the slide engine's tint family);
// neutral steps are dk1 over the background (NeutralInk), the grey a
// single-series bar chart already uses for its non-highlighted bars.
const (
	// seriesLadderStep is the least luminance-contrast ratio between two
	// neighbouring ladder slots, and between two slots of one family.
	seriesLadderStep = 1.45
	// SeriesLadderLen is the number of slots the ladder has.
	SeriesLadderLen = 6
)

// seriesLadderGrid lists the background-contrast targets a ladder step can
// take, light to dark on a light background. Adjacent targets are a clear
// step (>= 1.5) apart, the lightest still reads as a mark on the background
// (2.1:1) and the darkest stays a colour, not text ink.
var seriesLadderGrid = []float64{2.1, 3.2, 5.0, 8.0}

// TonalSeriesLadder returns the default series colours for accent on
// background, with ink as the text colour neutrals are mixed from. The first
// colour is always accent itself.
func TonalSeriesLadder(accent, background, ink Color) []Color {
	bg := background
	if bg.A < 1 {
		bg = bg.BlendOver(Color{R: 255, G: 255, B: 255, A: 1})
	}
	accent = accent.BlendOver(bg)
	ink.A = 1

	l := &seriesLadder{accent: accent, bg: bg, ink: ink}
	l.steps = []ladderStep{{c: accent, cr: accent.ContrastWith(bg)}}
	g := seriesLadderGrid
	lightFirst := []float64{g[0], g[1], g[2], g[3]}
	// The shade is the nearest step darker than the accent; a dark accent
	// has none and takes a second, mid tint instead.
	shade := make([]float64, 0, len(g))
	for _, target := range g {
		if target > l.steps[0].cr {
			shade = append(shade, target)
		}
	}
	for i := len(g) - 1; i >= 0; i-- {
		if g[i] <= l.steps[0].cr {
			shade = append(shade, g[i])
		}
	}
	l.pick(true, lightFirst)                        // light neutral
	l.pick(true, []float64{g[2], g[3], g[1], g[0]}) // dark neutral: a grey, not a black slab
	l.pick(false, lightFirst)                       // accent tint
	l.pick(false, shade)                            // accent shade
	l.pick(true, []float64{g[1], g[3], g[2], g[0]}) // mid neutral

	out := make([]Color, 0, SeriesLadderLen)
	for _, s := range l.steps {
		out = append(out, s.c)
	}
	// A grey or near-grey accent leaves no room for two families: its tints
	// are the neutrals. Fill the remaining slots with whichever step of a
	// finer grid is furthest from every colour already chosen, so the ladder
	// degrades to an even grey ramp instead of repeating a colour.
	for len(out) < SeriesLadderLen {
		out = append(out, l.furthestFrom(out))
	}
	return out
}

// ladderStep is one chosen ladder colour: its family and its contrast
// against the chart background.
type ladderStep struct {
	c       Color
	neutral bool
	cr      float64
}

// seriesLadder builds a tonal series ladder step by step.
type seriesLadder struct {
	accent, bg, ink Color
	steps           []ladderStep
}

// at returns the family's colour at a background-contrast target.
func (l *seriesLadder) at(neutral bool, target float64) Color {
	if neutral {
		return neutralAtContrast(l.ink, l.bg, target)
	}
	return accentAtContrast(l.accent, l.bg, target)
}

// pick appends the first valid grid target in the given order; a second,
// lenient pass keeps the ladder full on awkward accents.
func (l *seriesLadder) pick(neutral bool, order []float64) {
	for _, strict := range []bool{true, false} {
		for _, target := range order {
			c := l.at(neutral, target)
			s := ladderStep{c: c, neutral: neutral, cr: c.ContrastWith(l.bg)}
			if l.valid(s, strict) {
				l.steps = append(l.steps, s)
				return
			}
		}
	}
}

// valid reports whether s is a clear luminance step from the previous slot
// and from every chosen step of its own family, and a colour of its own.
func (l *seriesLadder) valid(s ladderStep, strict bool) bool {
	if contrastStep(s.cr, l.steps[len(l.steps)-1].cr) < seriesLadderStep {
		return false
	}
	for _, o := range l.steps {
		if o.neutral == s.neutral && contrastStep(s.cr, o.cr) < seriesLadderStep {
			return false
		}
		// A neutral beside a greyish accent is the same colour to the eye.
		dist := deltaE76(s.c, o.c)
		if dist < MinSeriesDeltaE/2 {
			return false
		}
		if strict && s.neutral != o.neutral && contrastStep(s.cr, o.cr) < 1.2 && dist < MinSeriesDeltaE {
			return false
		}
	}
	// A neutral that lands on the text ink reads as text.
	return !s.neutral || deltaE76(s.c, l.ink) >= MinSeriesDeltaE
}

// furthestFrom returns the step of a fine contrast grid, of either family,
// that is furthest (CIE76) from the ink and from every colour in chosen.
func (l *seriesLadder) furthestFrom(chosen []Color) Color {
	var best Color
	bestDist := -1.0
	for target := seriesLadderGrid[0]; target <= 10; target *= 1.12 {
		for _, neutral := range []bool{true, false} {
			c := l.at(neutral, target)
			dist := deltaE76(c, l.ink)
			for _, o := range chosen {
				dist = math.Min(dist, deltaE76(c, o))
			}
			if dist > bestDist {
				best, bestDist = c, dist
			}
		}
	}
	return best
}

// contrastStep is the ratio between two background contrasts, >= 1.
func contrastStep(a, b float64) float64 {
	if a < b {
		a, b = b, a
	}
	return a / b
}

// accentAtContrast returns accent with its HSL lightness moved until it has
// the target contrast against bg; hue and saturation are kept.
func accentAtContrast(accent, bg Color, target float64) Color {
	h, s, _ := rgbToHSL(accent.R, accent.G, accent.B)
	return searchContrast(bg, target, func(t float64) Color {
		r, g, b := hslToRGB(h, s, t)
		return Color{R: r, G: g, B: b, A: 1}
	})
}

// neutralAtContrast returns ink over bg at the share that has the target
// contrast against bg. An ink that does not itself read on bg (a chart drawn
// on a dark panel of a light theme) is replaced by black or white.
func neutralAtContrast(ink, bg Color, target float64) Color {
	if ink.ContrastWith(bg) < WCAGAANormal {
		ink = Color{A: 1}
		if white := (Color{R: 255, G: 255, B: 255, A: 1}); white.ContrastWith(bg) > ink.ContrastWith(bg) {
			ink = white
		}
	}
	at := func(share float64) Color {
		c := ink
		c.A = share
		return c.BlendOver(bg)
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 24; i++ {
		mid := (lo + hi) / 2
		if at(mid).ContrastWith(bg) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return at((lo + hi) / 2)
}

// searchContrast bisects the lightness parameter t in [0, 1] (0 dark, 1
// light) of at for the colour whose contrast against bg is target. On a light
// background contrast grows as t falls; on a dark one as t rises.
func searchContrast(bg Color, target float64, at func(t float64) Color) Color {
	lightBg := bg.Luminance() >= 0.18
	lo, hi := 0.0, 1.0
	for i := 0; i < 24; i++ {
		mid := (lo + hi) / 2
		tooFaint := at(mid).ContrastWith(bg) < target
		// tooFaint: move away from the background.
		if tooFaint == lightBg {
			hi = mid
		} else {
			lo = mid
		}
	}
	return at((lo + hi) / 2)
}
