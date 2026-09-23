package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Group-wise contrast for shape-grid cells (go-slide-creator-tnx3e).
//
// The contrast pass decided one cell at a time, so an arch-stack whose tiers are
// progressive tints of one accent came out with three text colours across four
// sibling tiers: white on the darkest, pure black on the next, and dk2 on the
// last two — each defensible alone, none of them a design. The reviewer's crop
// shows a diagram that reads as three unrelated bands.
//
// Cells that start from the same text colour and hold text of the same weight
// are siblings: whatever the grid meant by "white label", it meant it for all of
// them. So the decision is made once per group, against the WORST fill in the
// group, and applied to every member — including the ones that would have
// passed, because a tier keeping white while its neighbours go dark is the
// inconsistency itself.

// gridTextGroup is a set of shape-grid cells that share a text colour and a
// contrast threshold: the same label, drawn on different fills.
type gridTextGroup struct {
	// origHex is the text colour every member starts from.
	origHex string
	// threshold is the WCAG ratio the group's text size demands.
	threshold float64
	// members are the shape indices, in slide order.
	members []int
	// fills are the resolved fill colours of the members, index-aligned.
	fills []svggen.Color
	// fillHex mirrors fills, for the swap record.
	fillHex []string
	// fillSafe is true when EVERY member's fill is certified white-text-safe.
	fillSafe bool
}

// worstFill returns the member fill with the least contrast against c, and its
// hex. It is what a group-wide colour has to clear.
func (g gridTextGroup) worstFill(c svggen.Color) (svggen.Color, string, float64) {
	worst, worstHex, worstRatio := g.fills[0], g.fillHex[0], c.ContrastWith(g.fills[0])
	for i := 1; i < len(g.fills); i++ {
		if r := c.ContrastWith(g.fills[i]); r < worstRatio {
			worst, worstHex, worstRatio = g.fills[i], g.fillHex[i], r
		}
	}
	return worst, worstHex, worstRatio
}

// maxFillSpread is how far apart two fills may be and still count as one
// family. Progressive tints of an accent sit far inside it; a dark navy card
// beside a near-white one does not — those cells look nothing alike, so a colour
// that suits both is a compromise that suits neither.
const maxFillSpread = 3.0

// fillsAreAlike reports whether the group's fills are close enough to be one
// visual family. It is what stops a "shared white label" from grouping cells
// that only happen to start from the same colour.
func (g gridTextGroup) fillsAreAlike() bool {
	for i := range g.fills {
		for j := i + 1; j < len(g.fills); j++ {
			if g.fills[i].ContrastWith(g.fills[j]) > maxFillSpread {
				return false
			}
		}
	}
	return true
}

// textColorRunRegexp matches a text colour inside a run's solidFill, capturing
// the tag (srgbClr / schemeClr) and its value.
var textColorRunRegexp = regexp.MustCompile(`<a:solidFill>\s*<a:(srgbClr|schemeClr)\s+val="([^"]+)"`)

// collectGridTextGroups builds the sibling groups for a slide's shape-grid
// cells. Only cells with a resolvable fill and at least one text colour take
// part; a colour that appears in a single cell forms no group and is left to the
// per-shape pass.
func collectGridTextGroups(shapes [][]byte, themeColors []types.ThemeColor, whiteTextSafeHex map[string]bool, slideBackground ...string) []gridTextGroup {
	type acc struct {
		group gridTextGroup
		order int
	}
	byKey := map[string]*acc{}
	var order int

	for i, shape := range shapes {
		fillHex := effectiveGridShapeFillHex(shape, themeColors, slideBackground...)
		if fillHex == "" {
			continue
		}
		fill, err := svggen.ParseColor(fillHex)
		if err != nil {
			continue
		}
		txBody := shapeTextBody(shape)
		if txBody == "" {
			continue
		}
		threshold := contrastThresholdFor(smallestTextPt(txBody))
		safe := len(whiteTextSafeHex) > 0 && whiteTextSafeHex[strings.ToUpper(fillHex)]

		for _, origHex := range textColorsIn(txBody, themeColors) {
			key := origHex + "|" + fmt.Sprintf("%.2f", threshold)
			a, ok := byKey[key]
			if !ok {
				a = &acc{group: gridTextGroup{origHex: origHex, threshold: threshold, fillSafe: true}, order: order}
				order++
				byKey[key] = a
			}
			a.group.members = append(a.group.members, i)
			a.group.fills = append(a.group.fills, fill)
			a.group.fillHex = append(a.group.fillHex, fillHex)
			a.group.fillSafe = a.group.fillSafe && safe
		}
	}

	out := make([]gridTextGroup, len(byKey))
	for _, a := range byKey {
		out[a.order] = a.group
	}
	return out
}

// shapeTextBody returns a shape's <p:txBody> fragment, or "" when it has none.
func shapeTextBody(shapeXML []byte) string {
	start := strings.Index(string(shapeXML), "<p:txBody>")
	if start < 0 {
		return ""
	}
	body := string(shapeXML)[start:]
	end := strings.Index(body, "</p:txBody>")
	if end < 0 {
		return ""
	}
	return body[:end+len("</p:txBody>")]
}

// textColorsIn returns the distinct text colours of a txBody, resolved to hex
// and in first-seen order.
func textColorsIn(txBody string, themeColors []types.ThemeColor) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range textColorRunRegexp.FindAllStringSubmatch(txBody, -1) {
		hex := m[2]
		if m[1] == "schemeClr" {
			hex = resolveSchemeColorToHex(m[2], themeColors)
		} else {
			hex = "#" + strings.ToUpper(hex)
		}
		if hex == "" || seen[hex] {
			continue
		}
		seen[hex] = true
		out = append(out, hex)
	}
	return out
}

// groupTextColor picks the one colour a group should use, or ok=false when the
// group needs no change.
//
// A group is left alone when its original colour already clears the threshold on
// every fill. Otherwise the candidates are tried in template-fidelity order —
// the palette's own text colours, then a tonal shade of the worst fill, then the
// neutral extremes — and the first that clears the threshold on EVERY fill wins.
// The tonal shade comes before pure black on purpose: when dk2 misses AA by a
// hair on one tint (4.37 against a 4.5 bar, in the reported case), dropping the
// whole diagram to #000000 is a bigger visual change than darkening the fill's
// own hue.
func groupTextColor(g gridTextGroup, themeColors []types.ThemeColor) (svggen.Color, string, bool) {
	orig, err := svggen.ParseColor(g.origHex)
	if err != nil {
		return svggen.Color{}, "", false
	}
	if g.fillSafe && isWhiteOrLt1(g.origHex) {
		return svggen.Color{}, "", false
	}
	if _, _, worst := g.worstFill(orig); worst >= g.threshold {
		return svggen.Color{}, "", false
	}
	if !g.fillsAreAlike() {
		return svggen.Color{}, "", false
	}

	clearsAll := func(c svggen.Color) bool {
		_, _, r := g.worstFill(c)
		return r >= g.threshold
	}

	type candidate struct {
		color  svggen.Color
		scheme string
	}
	var candidates []candidate
	for _, scheme := range []string{"lt1", "dk2", "dk1"} {
		hex := resolveSchemeColorToHex(scheme, themeColors)
		if hex == "" {
			continue
		}
		c, cerr := svggen.ParseColor(hex)
		if cerr != nil {
			continue
		}
		if scheme == "dk1" {
			// Tonal shades of the group's own fills come before the palette's
			// near-black, so a purple stack stays purple. One shade per fill,
			// because the fill that is hardest for a DARK text is the darkest
			// one while the hardest for a LIGHT text is the lightest — and
			// clearsAll then keeps only a shade that works on every cell.
			for _, fill := range g.fills {
				candidates = append(candidates, candidate{svggen.EnsureContrast(fill, fill, g.threshold), ""})
			}
		}
		candidates = append(candidates, candidate{c, scheme})
	}
	for _, fill := range g.fills {
		candidates = append(candidates, candidate{svggen.EnsureContrast(orig, fill, g.threshold), ""})
	}
	candidates = append(candidates,
		candidate{svggen.MustParseColor("#000000"), ""},
		candidate{svggen.MustParseColor("#FFFFFF"), ""},
	)

	for _, c := range candidates {
		if clearsAll(c.color) {
			return c.color, c.scheme, true
		}
	}
	// Nothing clears every fill: this is not one group after all. A row of dark
	// accent cards beside a pale caption box can share a white label and have no
	// single readable colour between them — forcing one would make the dark cards
	// worse to make the pale one better. Leave it to the per-shape pass.
	return svggen.Color{}, "", false
}

// applyGroupTextColor rewrites a group's text colour in every member shape and
// returns the ONE swap that records the decision. Members already carrying the
// chosen colour are left untouched but still counted: the finding is about the
// group, not about one cell.
func applyGroupTextColor(shapes [][]byte, g gridTextGroup, replacement svggen.Color, themeColors []types.ThemeColor, slideIndex int) (ContrastSwap, bool) {
	toHex := strings.ToUpper(replacement.Hex())
	if toHex == strings.ToUpper(g.origHex) {
		return ContrastSwap{}, false
	}
	changed := 0
	for _, idx := range g.members {
		body := shapeTextBody(shapes[idx])
		if body == "" {
			continue
		}
		fixed := replaceTextColor(body, g.origHex, toHex, themeColors)
		if fixed == body {
			continue
		}
		shapes[idx] = []byte(strings.Replace(string(shapes[idx]), body, fixed, 1))
		changed++
	}
	if changed == 0 {
		return ContrastSwap{}, false
	}

	orig, err := svggen.ParseColor(g.origHex)
	if err != nil {
		return ContrastSwap{}, false
	}
	_, worstHex, before := g.worstFill(orig)
	_, _, after := g.worstFill(replacement)
	return ContrastSwap{
		OriginalColor:   strings.ToUpper(g.origHex),
		ReplacedColor:   toHex,
		BackgroundColor: worstHex,
		RatioBefore:     before,
		RatioAfter:      after,
		SlideIndex:      slideIndex,
		Path:            slidepath.ShapeGrid(slideIndex),
		Source:          gridGroupSwapSource,
		Cells:           changed,
	}, true
}

// gridGroupSwapSource labels a swap made for a whole group of sibling cells, so
// an agent reading the finding knows it covers more than one shape.
const gridGroupSwapSource = "shape_grid_group"

// replaceTextColor rewrites every run colour equal to fromHex with toHex,
// covering both the sRGB and the scheme spelling. The scheme form is replaced by
// an explicit sRGB value: the point is to pin one colour across cells, and a
// scheme name would resolve per shape again.
func replaceTextColor(txBody, fromHex, toHex string, themeColors []types.ThemeColor) string {
	bare := strings.TrimPrefix(strings.ToUpper(toHex), "#")
	want := strings.ToUpper(fromHex)
	return textColorRunRegexp.ReplaceAllStringFunc(txBody, func(match string) string {
		m := textColorRunRegexp.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		current := "#" + strings.ToUpper(m[2])
		if m[1] == "schemeClr" {
			// Compare what the scheme name RESOLVES to, the same way the group
			// was built: a cell whose label is schemeClr accent1 belongs to the
			// group of that accent's hex, not to a group named "accent1".
			current = strings.ToUpper(resolveSchemeColorToHex(m[2], themeColors))
		}
		if current != want {
			return match
		}
		return `<a:solidFill><a:srgbClr val="` + bare + `"`
	})
}

// enforceGridGroupContrast harmonises sibling shape-grid cells before the
// per-shape pass runs. It returns the shapes with group decisions applied, the
// swaps recording them, and the set of (shape index, colour) pairs the per-shape
// pass must leave alone — they have already been decided as a group.
func enforceGridGroupContrast(shapes [][]byte, themeColors []types.ThemeColor, whiteTextSafeHex map[string]bool, slideIndex int, slideBackground ...string) ([][]byte, []ContrastSwap) {
	var swaps []ContrastSwap
	for _, g := range collectGridTextGroups(shapes, themeColors, whiteTextSafeHex, slideBackground...) {
		if len(g.members) < 2 {
			continue
		}
		replacement, _, ok := groupTextColor(g, themeColors)
		if !ok {
			continue
		}
		if swap, applied := applyGroupTextColor(shapes, g, replacement, themeColors, slideIndex); applied {
			swaps = append(swaps, swap)
		}
	}
	return shapes, swaps
}
