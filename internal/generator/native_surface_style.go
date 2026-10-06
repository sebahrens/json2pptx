package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// The one surface style of the native framework diagrams
// (go-slide-creator-amtkg).
//
// swot, business_model_canvas, pestel, nine_box_talent, porters_five_forces,
// value_chain, kpi_dashboard and the panel family used to choose their own
// corner radius, tint ladder and header treatment: rounded cards in accent
// pastels with centred tab headers, beside patterns that draw square neutral
// surfaces with one accent. A deck mixing the two looked assembled from two
// tools. Every one of those builders now takes its surface from here, and
// this file takes it from internal/patterns:
//
//   - corner radius: none (patterns.SurfaceGeometry);
//   - surface: the dk1 neutral ladder (patterns.NeutralTint4 for a card,
//     NeutralTint8 for its header band or an emphasised cell), with no
//     outline; a shape that is the content itself (a value-chain step) takes
//     the accent's Lighter 80% swatch instead (contentTint), as the patterns'
//     tonal system does;
//   - header: a bold, left-aligned title in the template's primary accent
//     where that reads on the surface, else in the first theme ink that does
//     (patterns.AccentInkOnNeutral) — the bmc-canvas pattern's cell header;
//   - accent count: one, patterns.PrimaryFill.
//
// Colour beyond that is kept only where it is data: nine-box score bands,
// Porter intensity steps, heatmap scales and KPI trend direction.

// nativeSurfaceGeometry is the preset of every native card.
const nativeSurfaceGeometry = pptx.PresetGeometry(patterns.SurfaceGeometry)

// nativeHeaderAlign is the alignment of a native card's title.
const nativeHeaderAlign = "l"

// nativeSurface resolves the shared style against one template's theme. The
// zero value (no theme colours) is the template-independent default: accent1
// as the accent and as the title ink.
type nativeSurface struct {
	colors []types.ThemeColor
	// authored are the diagram's style.colors as cell tints (see
	// authoredTaxonomyColors): the documented way back to accent-tinted
	// cards. Empty means the neutral default.
	authored []taxonomyTint
}

// accent is the diagram's one accent: the template's primary fill.
func (s nativeSurface) accent() string {
	return patterns.PrimaryFill(s.colors)
}

// titleInk names the scheme colour of a bold title of the given size
// (hundredths of a point) on the neutral surface at pct% coverage.
func (s nativeSurface) titleInk(pct, sizeHundredths int) string {
	return patterns.AccentInkOnNeutral(s.colors, s.accent(), pct,
		patterns.TextContrastThreshold(float64(sizeHundredths)/100, true))
}

// cardTint is the default card of a taxonomy framework: the neutral surface
// with an accent title sized titleHundredths.
func (s nativeSurface) cardTint(titleHundredths int) taxonomyTint {
	t := nativeNeutralTint(patterns.NeutralTint4)
	t.ink = s.titleInk(patterns.NeutralTint4, titleHundredths)
	return t
}

// tint is the surface of cell i: the neutral step pct under an accent title
// sized titleHundredths, or — when the diagram sets style.colors — the i-th
// authored colour (a short list repeats) under the text role that reads on it.
func (s nativeSurface) tint(i, pct, titleHundredths int) taxonomyTint {
	if len(s.authored) > 0 {
		return s.authored[i%len(s.authored)]
	}
	t := nativeNeutralTint(pct)
	t.ink = s.titleInk(pct, titleHundredths)
	return t
}

// contentTint is the fill of a shape that IS the diagram's content (a value
// chain's primary activity, its margin tip): the accent's "Lighter pct%"
// swatch of the patterns' tonal system (patterns.TonalLighterContent), under
// the text role that reads on it — or, when the diagram sets style.colors,
// the i-th authored colour.
func (s nativeSurface) contentTint(i, pct int) taxonomyTint {
	if len(s.authored) > 0 {
		t := s.authored[i%len(s.authored)]
		t.ink = ""
		return t
	}
	return taxonomyTint{scheme: s.accent(), lumMod: (100 - pct) * 1000, lumOff: pct * 1000}
}

// nativeNeutralTint is the neutral surface at pct% coverage as a cell tint.
func nativeNeutralTint(pct int) taxonomyTint {
	lumMod, lumOff := patterns.NeutralSurfaceMods(pct)
	return taxonomyTint{scheme: patterns.NeutralSurfaceColor, lumMod: lumMod, lumOff: lumOff}
}

// nativeNeutralFill is the neutral surface at pct% coverage: the fill a
// pattern writes as {"color":"dk1","lumMod":…,"lumOff":…}.
func nativeNeutralFill(pct int) pptx.Fill {
	return nativeNeutralTint(pct).fill()
}

// fill renders the tint. The neutral ladder is ink coverage in HSL terms
// (lumMod / lumOff), exactly as the patterns write it, and so is an accent
// tint (diagramTintFill): one tint family across both engines.
func (t taxonomyTint) fill() pptx.Fill {
	if t.scheme == patterns.NeutralSurfaceColor && t.lumOff != 0 {
		return pptx.SchemeFill(t.scheme, pptx.LumMod(t.lumMod), pptx.LumOff(t.lumOff))
	}
	return diagramTintFill(t.scheme, t.lumMod, t.lumOff)
}

// mods are the tint's colour modifiers in the form the contrast helpers take.
func (t taxonomyTint) mods() patterns.ColorMods {
	if t.scheme == patterns.NeutralSurfaceColor && t.lumOff != 0 {
		return patterns.ColorMods{LumMod: t.lumMod, LumOff: t.lumOff}
	}
	if !pptx.IsSchemeColor(t.scheme) || t.lumOff == 0 {
		return patterns.ColorMods{}
	}
	return diagramTintMods(t.lumMod, t.lumOff)
}

// titleFill is the colour of the card's title: the tint's own ink when it
// names one, else the text role that reads on the fill.
func (t taxonomyTint) titleFill() pptx.Fill {
	if t.ink != "" {
		return pptx.SchemeFill(t.ink)
	}
	return diagramPanelTextFill(t.scheme)
}
