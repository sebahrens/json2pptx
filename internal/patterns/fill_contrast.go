package patterns

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// Fill-aware text colour selection for pattern expanders.
//
// Patterns emit semantic scheme colours ("accent1", "lt2", ...) optionally
// modified by alpha / lumMod / lumOff. The shape-grid contrast pass is
// warn-only, so a pattern that paints lt1 text on a light tint ships an
// unreadable shape. These helpers resolve the effective fill against the
// template theme and pick whichever of the light / dark text roles reads
// better, so the choice holds on every template rather than just the one the
// pattern was tuned on.
// ---------------------------------------------------------------------------

// fillTone describes a pattern fill: a scheme name (or hex) plus the OOXML
// colour modifiers the pattern applies. Zero modifiers mean "unmodified".
type fillTone struct {
	Color  string
	Alpha  float64 // 0-100 percent opacity; 0 = fully opaque
	LumMod int     // OOXML thousandths of a percent (e.g. 20000 = 20%)
	LumOff int     // OOXML thousandths of a percent
	Tint   int     // OOXML thousandths of a percent; mixes toward white
	Shade  int     // OOXML thousandths of a percent; mixes toward black
}

// Light-tint modifiers (PowerPoint's "Lighter 80%" swatch): L' = 0.2·L + 0.8.
const (
	tintLumMod = 20000
	tintLumOff = 80000
)

// inactiveTintTone returns the light tint of color used for de-emphasised
// (inactive / neutral) boxes in place of a dk1 black fill.
func inactiveTintTone(color string) fillTone {
	// The shape-grid fill resolver only honours lumMod/lumOff on scheme
	// colours; a hex accent override falls back to the neutral lt2 surface.
	if isHexColor(color) {
		return fillTone{Color: "lt2"}
	}
	return fillTone{Color: color, LumMod: tintLumMod, LumOff: tintLumOff}
}

// fillJSON renders the tone as a shape-grid fill value: a bare colour string
// when unmodified, otherwise the object form with the modifiers.
func (t fillTone) fillJSON() json.RawMessage {
	if t.Alpha == 0 && t.LumMod == 0 && t.LumOff == 0 && t.Tint == 0 && t.Shade == 0 {
		data, _ := json.Marshal(t.Color)
		return data
	}
	obj := struct {
		Color  string  `json:"color"`
		Alpha  float64 `json:"alpha,omitempty"`
		LumMod int     `json:"lumMod,omitempty"`
		LumOff int     `json:"lumOff,omitempty"`
		Tint   int     `json:"tint,omitempty"`
		Shade  int     `json:"shade,omitempty"`
	}{t.Color, t.Alpha, t.LumMod, t.LumOff, t.Tint, t.Shade}
	data, _ := json.Marshal(obj)
	return data
}

// schemeAliases maps the bg/tx aliases to their underlying theme slots.
var schemeAliases = map[string]string{
	"bg1": "lt1",
	"tx1": "dk1",
	"bg2": "lt2",
	"tx2": "dk2",
}

// resolveThemeColor resolves a scheme colour name (or a hex literal) against
// the expand context's theme. ok is false when the colour cannot be resolved
// (no theme loaded, unknown name).
func resolveThemeColor(ctx ExpandContext, name string) (svggen.Color, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return svggen.Color{}, false
	}
	if alias, isAlias := schemeAliases[name]; isAlias {
		name = alias
	}
	for _, tc := range ctx.Theme.Colors {
		if tc.Name == name {
			c, err := svggen.ParseColor(tc.RGB)
			if err != nil {
				return svggen.Color{}, false
			}
			return c, true
		}
	}
	if isHexColor(name) {
		c, err := svggen.ParseColor(name)
		if err == nil {
			return c, true
		}
	}
	return svggen.Color{}, false
}

func isHexColor(s string) bool {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

// effectiveFillColor returns the opaque colour a viewer sees for tone,
// applying lumMod/lumOff in HSL space (as PowerPoint does) and compositing
// alpha over the theme's lt1 background.
func effectiveFillColor(ctx ExpandContext, tone fillTone) (svggen.Color, bool) {
	c, ok := resolveThemeColor(ctx, tone.Color)
	if !ok {
		return svggen.Color{}, false
	}
	bg, bgOK := resolveThemeColor(ctx, "lt1")
	if !bgOK {
		bg = svggen.Color{R: 255, G: 255, B: 255, A: 1}
	}
	alpha := 1.0
	if tone.Alpha > 0 && tone.Alpha < 100 {
		alpha = tone.Alpha / 100
	}
	return EffectiveColorMods(c, ColorMods{
		LumMod: tone.LumMod, LumOff: tone.LumOff,
		Tint: tone.Tint, Shade: tone.Shade, Alpha: alpha,
	}, bg), true
}

// EffectiveColor returns the opaque colour produced by applying the OOXML
// lumMod / lumOff modifiers (thousandths of a percent, HSL lightness, as
// PowerPoint does) to base and then compositing it at the given alpha
// (0-1; values <= 0 or >= 1 mean opaque) over bg. The shape-grid contrast
// pass uses it so tinted / translucent fills are judged by what the viewer
// actually sees rather than by their untinted base colour.
func EffectiveColor(base svggen.Color, lumMod, lumOff int, alpha float64, bg svggen.Color) svggen.Color {
	return EffectiveColorMods(base, ColorMods{LumMod: lumMod, LumOff: lumOff, Alpha: alpha}, bg)
}

// ColorMods are the OOXML colour modifiers a fill may carry. LumMod, LumOff,
// Tint and Shade are thousandths of a percent; Alpha is 0-1 (values <= 0 or
// >= 1 mean opaque).
type ColorMods struct {
	LumMod int
	LumOff int
	Tint   int
	Shade  int
	Alpha  float64
}

// EffectiveColorMods returns the opaque colour a viewer sees for base under
// mods, composited over bg.
//
// lumMod / lumOff act on HSL lightness, as PowerPoint does. tint and shade do
// NOT: they are linear-light mixes toward white and black respectively
// (verified against rendered pixels — accent1 #2E5090 under shade 70000
// renders rgb(38,67,122) and under tint 40000 renders rgb(205,208,219), which
// the HSL and sRGB models both miss by 30-50 per channel). Modelling them as
// lumMod was not an option: they were simply ignored, so a pattern that tinted
// its fills was judged on its untinted base colour and shipped white text on a
// near-white bar (go-slide-creator-5qotm).
func EffectiveColorMods(base svggen.Color, mods ColorMods, bg svggen.Color) svggen.Color {
	c := base
	c.A = 1
	if mods.LumMod > 0 || mods.LumOff > 0 {
		c = applyLumModOff(c, mods.LumMod, mods.LumOff)
	}
	if mods.Shade > 0 {
		c = applyLinearMix(c, float64(mods.Shade)/100000, svggen.Color{R: 0, G: 0, B: 0, A: 1})
	}
	if mods.Tint > 0 {
		c = applyLinearMix(c, float64(mods.Tint)/100000, svggen.Color{R: 255, G: 255, B: 255, A: 1})
	}
	if mods.Alpha > 0 && mods.Alpha < 1 {
		c = svggen.Color{
			R: uint8(math.Round(float64(c.R)*mods.Alpha + float64(bg.R)*(1-mods.Alpha))),
			G: uint8(math.Round(float64(c.G)*mods.Alpha + float64(bg.G)*(1-mods.Alpha))),
			B: uint8(math.Round(float64(c.B)*mods.Alpha + float64(bg.B)*(1-mods.Alpha))),
			A: 1,
		}
	}
	return c
}

// applyLinearMix blends c toward target in linear-light space, keeping the
// given fraction of c. This is what OOXML a:tint / a:shade do.
func applyLinearMix(c svggen.Color, keep float64, target svggen.Color) svggen.Color {
	if keep <= 0 {
		return target
	}
	if keep >= 1 {
		return c
	}
	mix := func(a, b uint8) uint8 {
		return linearToSRGB(srgbToLinear(a)*keep + srgbToLinear(b)*(1-keep))
	}
	return svggen.Color{R: mix(c.R, target.R), G: mix(c.G, target.G), B: mix(c.B, target.B), A: 1}
}

// srgbToLinear / linearToSRGB convert one channel between sRGB and linear light.
func srgbToLinear(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

func linearToSRGB(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 1:
		return 255
	}
	var f float64
	if v <= 0.0031308 {
		f = v * 12.92
	} else {
		f = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(math.Max(0, math.Min(1, f)) * 255))
}

// readableTextOn returns "lt1" or "dk1" — whichever has the higher contrast
// ratio against the effective fill. When the theme cannot be resolved it
// returns fallback so behaviour without a template is unchanged.
func readableTextOn(ctx ExpandContext, tone fillTone, fallback string) string {
	return readableInkOn(ctx, tone, fallback, svggen.WCAGAANormal)
}

// readableInkCandidates is the order the generator's own contrast fixer tries
// (internal/generator/text_contrast.go pickThemeTextColor), and the order this
// package must share.
//
// Picking whichever of lt1/dk1 had MORE contrast meant a light accent got dk1 —
// pure black — while the render-time fixer swapped the card's text on the same
// fill to dk2. The off-brand black that go-slide-creator-sis2 removed from text
// was still reachable through every pattern that asked this function, icons
// included (go-slide-creator-1sel).
var readableInkCandidates = []string{"lt1", "dk2", "dk1"}

// readableInkOn returns the first theme ink that clears minContrast against the
// fill, in the fixer's order. When none clears it, the highest-contrast of the
// candidates wins, so the answer is never worse than the old one.
func readableInkOn(ctx ExpandContext, tone fillTone, fallback string, minContrast float64) string {
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return fallback
	}
	best, bestRatio := "", 0.0
	for _, scheme := range readableInkCandidates {
		c, cok := resolveThemeColor(ctx, scheme)
		if !cok {
			continue
		}
		ratio := c.ContrastWith(fill)
		if ratio >= minContrast {
			return scheme
		}
		if ratio > bestRatio {
			best, bestRatio = scheme, ratio
		}
	}
	if best == "" {
		return fallback
	}
	return best
}

// applyLumModOff applies the OOXML lumMod / lumOff transforms (in HSL space).
func applyLumModOff(c svggen.Color, lumMod, lumOff int) svggen.Color {
	h, s, l := toHSL(c)
	mod := 1.0
	if lumMod > 0 {
		mod = float64(lumMod) / 100000
	}
	l = l*mod + float64(lumOff)/100000
	l = math.Max(0, math.Min(1, l))
	return fromHSL(h, s, l)
}

func toHSL(c svggen.Color) (h, s, l float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255
	maxV := math.Max(r, math.Max(g, b))
	minV := math.Min(r, math.Min(g, b))
	l = (maxV + minV) / 2
	if maxV == minV {
		return 0, 0, l
	}
	d := maxV - minV
	if l > 0.5 {
		s = d / (2 - maxV - minV)
	} else {
		s = d / (maxV + minV)
	}
	switch maxV {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func fromHSL(h, s, l float64) svggen.Color {
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return svggen.Color{R: v, G: v, B: v, A: 1}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	conv := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(v * 255))
	}
	return svggen.Color{R: conv(h + 1.0/3), G: conv(h), B: conv(h - 1.0/3), A: 1}
}

// ---------------------------------------------------------------------------
// Fill-vs-fill distinctness (go-slide-creator-ah5s).
//
// A pattern that paints its structure in one scheme slot and its ONE semantic
// signal in another is only as legible as the gap between those two slots in
// whatever template it lands on. value-chain painted steps dk2 and the
// highlighted step accent2: on midnight-blue that is red on navy and reads at
// a glance, on warm-coral it is #5D4037 on #3E2723 — a contrast of 1.48, so
// the highlight simply disappeared. The pattern was tuned on one template and
// the tests could not see the difference.
// ---------------------------------------------------------------------------

// fillDistinctnessMin is the fill-vs-fill contrast a highlight needs to read as
// a highlight. It is the WCAG non-text bar (3:1): below it two fills are the
// same block of colour to anyone past the first row.
const fillDistinctnessMin = 3.0

// fillContrast returns the contrast ratio between two pattern fills as a
// viewer sees them (tints and alpha composited). ok is false when either side
// cannot be resolved against the template — without a theme there is nothing to
// measure and callers keep their defaults.
func fillContrast(ctx ExpandContext, a, b fillTone) (float64, bool) {
	ca, aok := effectiveFillColor(ctx, a)
	cb, bok := effectiveFillColor(ctx, b)
	if !aok || !bok {
		return 0, false
	}
	return ca.ContrastWith(cb), true
}

// pickDistinctFill returns the first candidate whose effective colour clears
// minRatio against base, and ok=false when the theme cannot be resolved or no
// candidate does. Candidates are tried in order, so a caller states its
// preference (the brand accent first, say) and only loses it to a measurement.
func pickDistinctFill(ctx ExpandContext, base fillTone, minRatio float64, candidates ...string) (string, bool) {
	if _, ok := effectiveFillColor(ctx, base); !ok {
		return "", false
	}
	for _, name := range candidates {
		if name == base.Color {
			continue
		}
		ratio, ok := fillContrast(ctx, base, fillTone{Color: name})
		if ok && ratio >= minRatio {
			return name, true
		}
	}
	return "", false
}
