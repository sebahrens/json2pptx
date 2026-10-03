package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Taxonomy framework palettes (go-slide-creator-w0kj).
//
// business_model_canvas, pestel and swot used to colour every cell with a
// different theme accent — BMC accent1–6 plus repeats, PESTEL accent1–6, SWOT
// accent1–4. Templates define accent3–6 as unrelated hues, so a forest-green
// deck rendered a nine-colour rainbow with a cyan "Customer Relationships"
// box, and warm-coral got a cyan "Threats" quadrant.
//
// Colour carries no information in these frameworks: the cells are named, laid
// out in a fixed grid, and separated by gaps. Rotating hue was therefore pure
// noise, and it overrode whatever accent the deck had chosen. The cells are
// now the shared neutral card with the accent as title ink
// (native_surface_style.go, go-slide-creator-amtkg); accent tints — one hue,
// SWOT's two-tone polarity, or the old rotation — are what style.colors asks
// for.

// taxonomyTint is one cell's fill: a theme color plus its retained-color and
// white-blend percentages. The legacy field names are kept for the existing
// shape-builder call signatures; diagramTintFill emits an RGB tint.
type taxonomyTint struct {
	scheme string
	lumMod int
	lumOff int
	// ink names the scheme colour of the cell's title; empty takes the text
	// role that reads on the fill (see taxonomyTint.titleFill).
	ink string
}

var (
	// taxonomyLight is the tint an authored style.colors scheme colour takes —
	// light enough for dk1 text.
	taxonomyLight = taxonomyTint{scheme: "accent1", lumMod: 20000, lumOff: 80000}
)

// taxonomyPalette returns the n cell tints for a taxonomy diagram.
//
// An author who wants the old rainbow — or any other per-cell colouring — sets
// style.colors, which these diagram types previously ignored entirely. Entries
// are used in cell order; a short list repeats, so style.colors of one colour
// recolours the whole framework. Otherwise the caller's own defaults apply.
func taxonomyPalette(spec *types.DiagramSpec, n int, defaults func(i int) taxonomyTint) []taxonomyTint {
	out := make([]taxonomyTint, n)
	authored := authoredTaxonomyColors(spec)
	for i := range out {
		if len(authored) > 0 {
			out[i] = authored[i%len(authored)]
			continue
		}
		out[i] = defaults(i)
	}
	return out
}

// authoredTaxonomyColors reads style.colors into tints. A scheme name keeps the
// standard cell tint so the box stays a background, not a slab; an explicit hex
// is used at full strength because the author asked for that exact colour.
func authoredTaxonomyColors(spec *types.DiagramSpec) []taxonomyTint {
	if spec == nil || spec.Style == nil || len(spec.Style.Colors) == 0 {
		return nil
	}
	out := make([]taxonomyTint, 0, len(spec.Style.Colors))
	for _, c := range spec.Style.Colors {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.HasPrefix(c, "#") {
			out = append(out, taxonomyTint{scheme: c, lumMod: 0, lumOff: 0})
			continue
		}
		out = append(out, taxonomyTint{scheme: c, lumMod: taxonomyLight.lumMod, lumOff: taxonomyLight.lumOff})
	}
	return out
}

// uniformTaxonomyTint is the template-independent default cell of a taxonomy
// framework: the shared neutral card (native_surface_style.go) with an
// accent1 title. The renderer resolves the title ink against the template
// (nativeSurface.cardTint); this is what a builder falls back to when it is
// handed no tints.
func uniformTaxonomyTint(int) taxonomyTint { return nativeSurface{}.cardTint(0) }
