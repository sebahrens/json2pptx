package patterns

import (
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// The tonal system for callers outside this package (go-slide-creator-w107j).
//
// The native diagram builders of internal/generator draw the same roles the
// patterns do — panel, content, emphasis — and must resolve them the same
// way: the same collision fallback on a template whose accent tint cannot be
// told from its paper, the same shading of an accent white ink does not read
// on, the same measured ink. They know a theme's colours, not an
// ExpandContext, so these wrappers take the colours.

// TonalFill is a resolved fill: a scheme (or hex) colour with the OOXML
// modifiers the patterns write, in thousandths of a percent.
type TonalFill struct {
	Color  string
	LumMod int
	LumOff int
	Shade  int
}

// Mods are the fill's modifiers in the form EffectiveColorMods takes.
func (f TonalFill) Mods() ColorMods {
	return ColorMods{LumMod: f.LumMod, LumOff: f.LumOff, Shade: f.Shade}
}

func tonalFillOf(t fillTone) TonalFill {
	return TonalFill{Color: t.Color, LumMod: t.LumMod, LumOff: t.LumOff, Shade: t.Shade}
}

func (f TonalFill) tone() fillTone {
	return fillTone{Color: f.Color, LumMod: f.LumMod, LumOff: f.LumOff, Shade: f.Shade}
}

func tonalThemeContext(colors []types.ThemeColor) ExpandContext {
	return ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
}

// TonalContentFill is the fill of a shape that is the content: the accent's
// "Lighter pct%" swatch (TonalLighterContent / Deep / Pale), or the matching
// neutral step on a template whose accent tint collides (tonalRung).
func TonalContentFill(colors []types.ThemeColor, accent string, pct int) TonalFill {
	return tonalFillOf(tonalRung(tonalThemeContext(colors), accent, pct))
}

// TonalPanelFill is the fill of a panel that groups content: the lightest
// neutral surface.
func TonalPanelFill() TonalFill {
	return tonalFillOf(neutralTone(NeutralTint4))
}

// TonalNeutralFill is the neutral surface at pct% ink coverage.
func TonalNeutralFill(pct int) TonalFill {
	return tonalFillOf(neutralTone(pct))
}

// TonalEmphasisFill is the fill and ink of the one emphasised item: the solid
// accent, deepened where white ink would not read on it (tonalEmphasis).
func TonalEmphasisFill(colors []types.ThemeColor, accent string) (TonalFill, string) {
	tone, ink := tonalEmphasis(tonalThemeContext(colors), accent)
	return tonalFillOf(tone), ink
}

// TonalInkOn is the ink of body text on fill, chosen by measurement: the
// first theme ink that reads at WCAG AA (tonalInk).
func TonalInkOn(colors []types.ThemeColor, fill TonalFill) string {
	return tonalInk(tonalThemeContext(colors), fill.tone())
}

// TonalAccentInkOn names the scheme colour of large or bold display text
// (a numeral, a heading) on fill: the accent where it reads at minContrast,
// else the fill's body ink.
func TonalAccentInkOn(colors []types.ThemeColor, fill TonalFill, accent string, minContrast float64) string {
	ctx := tonalThemeContext(colors)
	if ratio, ok := fillContrast(ctx, fill.tone(), fillTone{Color: accent}); ok && ratio >= minContrast {
		return accent
	}
	return tonalInk(ctx, fill.tone())
}

// TonalBadgeFill is the fill and ink of a numbered badge: the neutral dark
// with the page colour as ink (tonalBadge).
func TonalBadgeFill(colors []types.ThemeColor) (TonalFill, string) {
	tone, ink := tonalBadge(tonalThemeContext(colors))
	return tonalFillOf(tone), ink
}

// TonalLargeTextContrast is the contrast bar of large display text.
const TonalLargeTextContrast = svggen.WCAGAALarge
