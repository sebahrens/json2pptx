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
// noise, and it overrode whatever accent the deck had chosen. These palettes
// keep one hue — the deck's primary accent — and vary only the tint, except
// where the framework encodes a genuine contrast (SWOT's positive/negative
// halves), which earns the second accent.

// taxonomyTint is one cell's fill: a theme color plus its retained-color and
// white-blend percentages. The legacy field names are kept for the existing
// shape-builder call signatures; diagramTintFill emits an RGB tint.
type taxonomyTint struct {
	scheme string
	lumMod int
	lumOff int
}

var (
	// taxonomyLight is the established cell tint — light enough for dk1 text.
	taxonomyLight = taxonomyTint{"accent1", 20000, 80000}
	// taxonomyDeep is the same hue carried a step darker, for the one cell a
	// framework wants to emphasise. Still a tint, so dk1 text stays readable.
	taxonomyDeep = taxonomyTint{"accent1", 35000, 65000}
	// taxonomyNegative is the second accent, reserved for the negative half of
	// a framework that actually has one. Templates that declare
	// semantic_accents.negative are not reachable from the generator context,
	// so this matches the fallback the pattern layer uses when none is
	// declared (negative_accent defaults to accent2).
	taxonomyNegative = taxonomyTint{"accent2", 20000, 80000}
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

// uniformTaxonomyTint is the defaults function for a framework whose cells are
// peers: PESTEL's six forces are not ranked, so nothing should stand out.
func uniformTaxonomyTint(int) taxonomyTint { return taxonomyLight }
