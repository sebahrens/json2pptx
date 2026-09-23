// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// ContrastSwap records a single text color replacement made by the contrast
// enforcement pass. The generator converts these into FitFindings for the
// response.
type ContrastSwap struct {
	OriginalColor   string  // e.g. "#FFE8D4"
	ReplacedColor   string  // e.g. "#1A1A1A"
	BackgroundColor string  // e.g. "#FFFFFF"
	RatioBefore     float64 // contrast ratio before fix
	RatioAfter      float64 // contrast ratio after fix

	// Location provenance. The low-level fix functions record only the
	// color/ratio evidence above; the enforcement caller that knows the owning
	// surface stamps these via annotateContrastSwaps so contrast_autofixed
	// findings can be mapped back to the offending slide / cell / text.
	SlideIndex int    // 0-based slide index; -1 when unknown
	Path       string // JSON pointer to the surface, e.g. "/slides/3/shape_grid/shapes/2"
	Source     string // surface label: "shape_grid", "lstStyle", "run"
	// Cells counts the sibling shape-grid cells one decision covered. Zero for
	// a single-shape swap; >1 when the colour was chosen once for a group of
	// cells that share a text role (go-slide-creator-tnx3e).
	Cells int
}

// annotateContrastSwaps stamps slide/path/source provenance onto a batch of
// freshly recorded swaps. Swaps are reslices of the caller's accumulator, so
// mutating them in place updates the originals. Callers that know the owning
// surface invoke this immediately after a fix pass that may have appended swaps.
func annotateContrastSwaps(swaps []ContrastSwap, slideIndex int, path, source string) {
	for i := range swaps {
		swaps[i].SlideIndex = slideIndex
		swaps[i].Path = path
		swaps[i].Source = source
	}
}

// =============================================================================
// Layout Background Extraction
// =============================================================================

// layoutBgSolidFillRegexp matches a solid fill sRGB color inside a <p:bg>/<p:bgPr>
// element in a slide layout. Captures the 6-hex-digit color value.
//
// Matches patterns like:
//
//	<p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFE8D4"/></a:solidFill>...
var layoutBgSolidFillRegexp = regexp.MustCompile(
	`<p:bg\b[^>]*>` + // <p:bg>
		`\s*<p:bgPr\b[^>]*>` + // <p:bgPr>
		`\s*<a:solidFill\b[^>]*>` + // <a:solidFill>
		`\s*<a:srgbClr\s+val="([0-9A-Fa-f]{6})"`, // capture hex color
)

// layoutBgSchemeClrRegexp matches a scheme color reference inside a layout background.
var layoutBgSchemeClrRegexp = regexp.MustCompile(
	`<p:bg\b[^>]*>` +
		`\s*<p:bgPr\b[^>]*>` +
		`\s*<a:solidFill\b[^>]*>` +
		`\s*<a:schemeClr\s+val="([^"]+)"`,
)

// extractLayoutBackgroundColor parses the raw layout XML and extracts the
// background solid fill color as a hex string (e.g., "#FFE8D4").
// Returns empty string if no background or non-solid fill.
func extractLayoutBackgroundColor(layoutXML []byte, themeColors []types.ThemeColor) string {
	xmlStr := string(layoutXML)

	// Try sRGB color first (most common for custom backgrounds)
	if m := layoutBgSolidFillRegexp.FindStringSubmatch(xmlStr); len(m) >= 2 {
		return "#" + strings.ToUpper(m[1])
	}

	// Try scheme color reference, resolved through the layout's own color map
	// override. modern-template's section divider fills with schemeClr tx1 under
	// <a:overrideClrMapping tx1="lt1">: read literally that is dk1 (near-black),
	// but the slide renders white (go-slide-creator-hln7).
	if m := layoutBgSchemeClrRegexp.FindStringSubmatch(xmlStr); len(m) >= 2 {
		if rgb := resolveSchemeColorMapped(m[1], parseLayoutColorMapOverride(layoutXML), themeColors); rgb != "" {
			return rgb
		}
	}

	return ""
}

// =============================================================================
// Scheme Color Resolution
// =============================================================================

// schemeToThemeName maps OOXML scheme color names to theme color names.
var schemeToThemeName = map[string]string{
	"tx1":     "dk1",
	"tx2":     "dk2",
	"bg1":     "lt1",
	"bg2":     "lt2",
	"dk1":     "dk1",
	"dk2":     "dk2",
	"lt1":     "lt1",
	"lt2":     "lt2",
	"accent1": "accent1",
	"accent2": "accent2",
	"accent3": "accent3",
	"accent4": "accent4",
	"accent5": "accent5",
	"accent6": "accent6",
}

// resolveSchemeColorToHex resolves a scheme color name (e.g., "accent1") to a
// hex color string (e.g., "#FD5108") using the provided theme colors.
func resolveSchemeColorToHex(schemeName string, themeColors []types.ThemeColor) string {
	themeName, ok := schemeToThemeName[schemeName]
	if !ok {
		return ""
	}

	for _, tc := range themeColors {
		if tc.Name == themeName {
			hex := tc.RGB
			if !strings.HasPrefix(hex, "#") {
				hex = "#" + hex
			}
			return hex
		}
	}
	return ""
}

// =============================================================================
// Text Contrast Enforcement
// =============================================================================

// schemeClrInFillRegexp matches <a:schemeClr val="..."/> inside <a:solidFill>.
// It captures both the full solidFill element and the scheme color name.
// This handles both self-closing and paired tags.
var schemeClrInFillRegexp = regexp.MustCompile(
	`(<a:solidFill\b[^>]*>\s*<a:schemeClr\s+val=")([^"]+)("\s*(?:/>|>[^<]*</a:schemeClr>)\s*</a:solidFill>)`,
)

// enforceTextContrastInSlide checks all text shapes in a slide for poor contrast
// between the text color (from lstStyle or run properties) and the layout
// background. When a scheme color resolves to a color with contrast below WCAG
// AA normal (4.5:1), it is replaced with a high-contrast sRGB color.
//
// Parameters:
//   - slide: the parsed slide XML to modify
//   - bgHex: the layout background color as hex (e.g., "#FFE8D4")
//   - themeColors: theme colors for resolving scheme color references
//
// Returns a slice of ContrastSwap records for each color replacement made.
// This function mutates the slide's shapes in place. slideIndex is the 0-based
// index into the input slides array, recorded on each swap for finding paths.
func enforceTextContrastInSlide(slide *slideXML, bgHex string, themeColors []types.ThemeColor, slideIndex int, override map[string]string, authorBackground bool) []ContrastSwap {
	if bgHex == "" || slide == nil {
		return nil
	}

	bgColor, err := svggen.ParseColor(bgHex)
	if err != nil {
		slog.Debug("text contrast: failed to parse background color", slog.String("bg", bgHex))
		return nil
	}

	var swaps []ContrastSwap
	for i := range slide.CommonSlideData.ShapeTree.Shapes {
		shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
		swaps = append(swaps, enforceTextContrastInShape(shape, bgColor, bgHex, themeColors, slideIndex, override, authorBackground)...)
	}
	return swaps
}

// enforceTextContrastInShape checks and fixes text color contrast in a single shape.
// It processes both the lstStyle (inherited styling) and individual run properties.
// Recorded swaps are stamped with slideIndex and the slide-level JSON path; the
// source label distinguishes lstStyle-inherited from run-level replacements.
func enforceTextContrastInShape(shape *shapeXML, bgColor svggen.Color, bgHex string, themeColors []types.ThemeColor, slideIndex int, override map[string]string, authorBackground bool) []ContrastSwap {
	if shape.TextBody == nil {
		return nil
	}

	var swaps []ContrastSwap
	slidePath := slidepath.Slide(slideIndex)

	// Fix lstStyle inherited text colors
	if shape.TextBody.ListStyle != nil && shape.TextBody.ListStyle.Inner != "" {
		start := len(swaps)
		lstPt, lstBold := smallestTextPt(shape.TextBody.ListStyle.Inner)
		shape.TextBody.ListStyle.Inner = fixSchemeColorsForContrast(
			shape.TextBody.ListStyle.Inner, bgColor, bgHex, themeColors, &swaps,
			shape.NonVisualProperties.ConnectionNonVisual.Name, "lstStyle", false,
			contrastThresholdFor(lstPt, lstBold), override, authorBackground,
		)
		annotateContrastSwaps(swaps[start:], slideIndex, slidePath, "lstStyle")
	}

	// Fix run-level text colors
	for pi := range shape.TextBody.Paragraphs {
		para := &shape.TextBody.Paragraphs[pi]
		for ri := range para.Runs {
			run := &para.Runs[ri]
			if run.RunProperties != nil && run.RunProperties.Inner != "" {
				start := len(swaps)
				runPt, runBold := runTextSize(run)
				run.RunProperties.Inner = fixSchemeColorsForContrast(
					run.RunProperties.Inner, bgColor, bgHex, themeColors, &swaps,
					shape.NonVisualProperties.ConnectionNonVisual.Name, "run", false,
					contrastThresholdFor(runPt, runBold), override, authorBackground,
				)
				annotateContrastSwaps(swaps[start:], slideIndex, slidePath, "run")
			}
		}
	}
	return swaps
}

// =============================================================================
// White-Text-Safe Allowlist
// =============================================================================

// computeWhiteTextSafeHex derives a set of uppercase hex color strings
// (e.g., "#4472C4") for all accent colors that pass WCAG AA Large (3:1) contrast
// against white. When a shape fill matches one of these colors and the text
// foreground is white/lt1, the contrast auto-fix is skipped — the template
// metadata already certifies that pairing as safe.
func computeWhiteTextSafeHex(themeColors []types.ThemeColor) map[string]bool {
	white := svggen.MustParseColor("#FFFFFF")
	accentNames := []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}
	safe := make(map[string]bool)

	for _, name := range accentNames {
		hex := resolveSchemeColorToHex(name, themeColors)
		if hex == "" {
			continue
		}
		c, err := svggen.ParseColor(hex)
		if err != nil {
			continue
		}
		if c.ContrastWith(white) >= svggen.WCAGAALarge {
			safe[strings.ToUpper(hex)] = true
		}
	}
	return safe
}

// isWhiteOrLt1 returns true if the given color string is white (#FFFFFF) or the
// lt1 scheme name. Used to identify text foregrounds that the white_text_safe
// allowlist should protect from contrast rewriting.
func isWhiteOrLt1(hexOrScheme string) bool {
	upper := strings.ToUpper(strings.TrimSpace(hexOrScheme))
	return upper == "#FFFFFF" || upper == "FFFFFF" || upper == "LT1" || upper == "BG1"
}

// =============================================================================
// Flip-to-extreme helper
// =============================================================================

// themeTextCandidate is a replacement text color considered by
// pickThemeTextColor. Scheme is the theme slot name ("lt1", "dk2", "dk1") when
// the candidate is a theme color; it is empty for derived shades.
type themeTextCandidate struct {
	Scheme string
	Hex    string
	Color  svggen.Color
}

// pickThemeTextColor chooses a template-derived replacement text color for a
// neutral (white/black) foreground that fails contrast against bg.
//
// Candidates are tried in template-fidelity order (go-slide-creator-sis2):
//
//	lt1, dk2, dk1, then a darker/lighter shade of the background hue
//
// The first candidate meeting WCAG AA normal text (4.5:1) wins; if none does,
// the first meeting AA large (3:1) wins; otherwise the highest-contrast
// candidate is returned. Preferring dk2 over dk1 keeps the fix inside the
// template palette — dk1 is literal #000000 in most templates, which reads as
// an off-brand black on accent fills.
func pickThemeTextColor(bg svggen.Color, themeColors []types.ThemeColor, threshold float64) themeTextCandidate {
	if threshold <= 0 {
		threshold = svggen.WCAGAANormal
	}
	var cands []themeTextCandidate
	add := func(scheme, hex string) {
		if hex == "" {
			return
		}
		c, err := svggen.ParseColor(hex)
		if err != nil {
			return
		}
		cands = append(cands, themeTextCandidate{Scheme: scheme, Hex: strings.ToUpper(c.Hex()), Color: c})
	}
	add("lt1", resolveSchemeColorToHex("lt1", themeColors))
	add("dk2", resolveSchemeColorToHex("dk2", themeColors))
	add("dk1", resolveSchemeColorToHex("dk1", themeColors))
	if len(cands) == 0 {
		// No theme at all: fall back to pure extremes.
		add("", "#FFFFFF")
		add("", "#000000")
	}
	// Tonal shade of the background hue (darker on light fills, lighter on
	// dark fills) — keeps the text inside the fill's color family.
	shade := svggen.EnsureContrast(bg, bg, svggen.WCAGAANormal)
	cands = append(cands, themeTextCandidate{Hex: strings.ToUpper(shade.Hex()), Color: shade})

	// The required threshold first; only then relax, so a candidate that merely
	// clears the large-text bar is never chosen for small text when a stricter
	// one exists.
	for _, t := range []float64{threshold, svggen.WCAGAALarge} {
		for _, c := range cands {
			if c.Color.ContrastWith(bg) >= t {
				return c
			}
		}
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.Color.ContrastWith(bg) > best.Color.ContrastWith(bg) {
			best = c
		}
	}
	return best
}

// pickFlippedTextColor returns a clean high-contrast replacement for white/black
// text against bg, chosen from the template palette by pickThemeTextColor.
// Rather than lerping to an intermediate gray, it snaps to a theme text color
// (lt1 / dk2 / dk1) or a tonal shade of the background.
//
// The returned hex string is upper-case with a leading "#".
func pickFlippedTextColor(bg svggen.Color, themeColors []types.ThemeColor, threshold float64) (svggen.Color, string) {
	c := pickThemeTextColor(bg, themeColors, threshold)
	return c.Color, c.Hex
}

// isNeutralExtremeHex reports whether a 6-hex color value is pure white or
// pure black — the two foregrounds for which a clean flip (rather than a
// lerped gray) is the correct contrast fix.
func isNeutralExtremeHex(hex string) bool {
	upper := strings.ToUpper(strings.TrimPrefix(hex, "#"))
	return upper == "FFFFFF" || upper == "000000"
}

// isNeutralExtremeScheme reports whether a scheme color name refers to one of
// the theme's pure-neutral text colors (lt1/bg1 = white, dk1/tx1 = black) for
// which a clean flip is preferable to a lerped gray.
func isNeutralExtremeScheme(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "lt1", "bg1", "dk1", "tx1":
		return true
	}
	return false
}

// Contrast replacement mode tags. They label which algorithm produced a
// replacement color so preflight findings can explain any difference from the
// render-time swap (today the two always agree because both call
// contrastReplacement).
const (
	contrastModeFlip = "flip" // snapped to a template text color (lt1/dk2/dk1) or fill shade
	contrastModeLerp = "lerp" // lerped toward black/white via EnsureContrast
)

// isNeutralForeground reports whether an authored foreground color — supplied
// either as a hex string ("#RRGGBB" / "RRGGBB") or as a scheme name ("lt1",
// "dk1", …) — is a pure neutral (white or black) for which a clean flip to the
// opposite theme extreme is the correct contrast fix. It mirrors the per-form
// checks the render-time pass applies to sRGB (isNeutralExtremeHex) and scheme
// (isNeutralExtremeScheme) text colors, so both the renderer and the preflight
// predictor classify the same authored value identically.
func isNeutralForeground(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "#") || (len(v) == 6 && isHex(v)) {
		return isNeutralExtremeHex(v)
	}
	return isNeutralExtremeScheme(v)
}

// runSizeRegexp captures the sz attribute (hundredths of a point) of every run
// or paragraph default in a text body.
var runSizeRegexp = regexp.MustCompile(`\bsz="(\d+)"`)

// runBoldRegexp captures the b attribute of every run in a text body.
var runBoldRegexp = regexp.MustCompile(`\bb="([01])"`)

// defaultBodyTextPt is assumed when a text body declares no explicit size: the
// inherited body size is the conservative case, and it is below the
// large-text bar either way.
const defaultBodyTextPt = 12.0

// wcagLargeTextPt / wcagLargeBoldTextPt are the WCAG "large text" thresholds:
// 18pt, or 14pt when bold. Only text at or above them may be fixed to the 3:1
// large-text contrast ratio; everything else needs 4.5:1.
const (
	wcagLargeTextPt     = 18.0
	wcagLargeBoldTextPt = 14.0
)

// contrastThresholdFor returns the WCAG AA contrast ratio a run of the given
// size must meet: 3:1 only for genuinely large text, 4.5:1 otherwise.
//
// The contrast pass used to fix every foreground to 3:1 on the stated
// assumption that "presentation text is almost always >= 18pt or >= 14pt
// bold". That is false for the 11pt supporting line a card carries, so swaps
// landed at exactly ratio 3.0 and the rendered text was barely visible
// grey-on-grey (go-slide-creator-9ux4).
func contrastThresholdFor(textPt float64, bold bool) float64 {
	if textPt <= 0 {
		textPt = defaultBodyTextPt
	}
	if textPt >= wcagLargeTextPt || (bold && textPt >= wcagLargeBoldTextPt) {
		return svggen.WCAGAALarge
	}
	return svggen.WCAGAANormal
}

// smallestTextPt returns the smallest declared run size in a text-body fragment
// (in points) and whether every sized run in it is bold. A fragment with no
// explicit size reports defaultBodyTextPt.
//
// The contrast pass rewrites a whole text body against one background, so the
// threshold is chosen from its SMALLEST text: fixing the 28pt KPI value to the
// stricter ratio alongside its 11pt label is harmless, while the reverse would
// leave the label illegible.
func smallestTextPt(fragment string) (float64, bool) {
	matches := runSizeRegexp.FindAllStringSubmatch(fragment, -1)
	if len(matches) == 0 {
		return defaultBodyTextPt, allRunsBold(fragment)
	}
	smallest := 0.0
	for _, m := range matches {
		hundredths, err := strconv.Atoi(m[1])
		if err != nil || hundredths <= 0 {
			continue
		}
		pt := float64(hundredths) / 100.0
		if smallest == 0 || pt < smallest {
			smallest = pt
		}
	}
	if smallest == 0 {
		return defaultBodyTextPt, allRunsBold(fragment)
	}
	return smallest, allRunsBold(fragment)
}

// allRunsBold reports whether every run that declares a b attribute declares
// b="1". A fragment with no b attribute at all is not bold.
func allRunsBold(fragment string) bool {
	matches := runBoldRegexp.FindAllStringSubmatch(fragment, -1)
	if len(matches) == 0 {
		return false
	}
	for _, m := range matches {
		if m[1] != "1" {
			return false
		}
	}
	return true
}

// contrastReplacement computes the high-contrast replacement color for a
// low-contrast foreground, and is the single source of truth shared by the
// render-time contrast pass (fixSrgbColorsForContrast / fixSchemeColorsForContrast)
// and the preflight predictor (DetectContrastPreflight). Sharing it guarantees
// validate-time advice matches generated output.
//
// originalFg is the foreground exactly as authored (a hex string or a scheme
// name); fgColor is its resolved color and bgColor the resolved background.
// Pure-neutral foregrounds (white/black, or scheme lt1/bg1/dk1/tx1) snap to a
// template text color (lt1/dk2/dk1, then a fill shade) via pickThemeTextColor — avoiding the
// muddy mid-gray that EnsureContrast yields when both fg and target sit near one
// extreme (e.g. white on light yellow). Every other foreground is lerped toward
// contrast via EnsureContrast. The returned mode is contrastModeFlip or
// contrastModeLerp.
func contrastReplacement(originalFg string, fgColor, bgColor svggen.Color, themeColors []types.ThemeColor, threshold float64, authorBackground bool) (svggen.Color, string) {
	c, mode, _ := contrastReplacementScheme(originalFg, fgColor, bgColor, themeColors, threshold, authorBackground)
	return c, mode
}

// contrastReplacementScheme is contrastReplacement plus the theme slot name
// ("lt1", "dk2", "dk1") of the chosen color when it came from the template
// palette, or "" for derived colors. Render-time fixes log it so the swap is
// traceable to the template palette.
func contrastReplacementScheme(originalFg string, fgColor, bgColor svggen.Color, themeColors []types.ThemeColor, threshold float64, authorBackground bool) (svggen.Color, string, string) {
	if threshold <= 0 {
		threshold = svggen.WCAGAANormal
	}
	// A background the AUTHOR introduced — background.color, or a scrim over a
	// photo — is not the one the template's text colour was chosen against, so
	// there is no hue intent to preserve and lerping just walks the colour to
	// the WCAG floor: a 60pt section title on midnight-blue came out #4E5A72 at
	// exactly 3.0 against black, where the template's own lt1 would read 21
	// (go-slide-creator-s7wmh). Snap to the palette instead.
	if authorBackground || isNeutralForeground(originalFg) {
		c := pickThemeTextColor(bgColor, themeColors, threshold)
		return c.Color, contrastModeFlip, c.Scheme
	}
	return svggen.EnsureContrast(fgColor, bgColor, threshold), contrastModeLerp, ""
}

// =============================================================================
// Shape Grid Contrast Enforcement
// =============================================================================

// shapeFillSrgbRegexp matches an sRGB solid fill color inside <p:spPr>.
var shapeFillSrgbRegexp = regexp.MustCompile(
	`<a:solidFill[^>]*>\s*<a:srgbClr\s+val="([0-9A-Fa-f]{6})"`,
)

// shapeFillSchemeRegexp matches a scheme color solid fill inside <p:spPr>.
var shapeFillSchemeRegexp = regexp.MustCompile(
	`<a:solidFill[^>]*>\s*<a:schemeClr\s+val="([^"]+)"`,
)

// extractShapeFillHex extracts the fill color from a shape's spPr section as a
// hex string (e.g., "#4472C4"). Returns empty string if no solid fill found.
func extractShapeFillHex(shapeXML []byte, themeColors []types.ThemeColor) string {
	// Isolate the spPr section to avoid matching text colors
	spPrStart := bytes.Index(shapeXML, []byte("<p:spPr>"))
	spPrEnd := bytes.Index(shapeXML, []byte("</p:spPr>"))
	if spPrStart < 0 || spPrEnd < 0 || spPrEnd <= spPrStart {
		return ""
	}
	spPr := shapeXML[spPrStart:spPrEnd]

	// Check for noFill — transparent shape, no contrast to enforce
	if bytes.Contains(spPr, []byte("<a:noFill/>")) {
		return ""
	}

	// Try sRGB first
	if m := shapeFillSrgbRegexp.FindSubmatch(spPr); len(m) >= 2 {
		return applyShapeFillModifiers("#"+strings.ToUpper(string(m[1])), spPr, themeColors)
	}

	// Try scheme color
	if m := shapeFillSchemeRegexp.FindSubmatch(spPr); len(m) >= 2 {
		return applyShapeFillModifiers(resolveSchemeColorToHex(string(m[1]), themeColors), spPr, themeColors)
	}

	return ""
}

// shapeFillBlockRegexp captures the first <a:solidFill>…</a:solidFill> block
// (the shape fill precedes the outline in spPr) so its colour modifiers can
// be read.
var shapeFillBlockRegexp = regexp.MustCompile(`(?s)<a:solidFill[^>]*>(.*?)</a:solidFill>`)

// shapeFillModRegexp matches the colour modifiers emitted by PPTX fills.
var shapeFillModRegexp = regexp.MustCompile(`<a:(lumMod|lumOff|tint|shade|alpha)\s+val="(\d+)"`)

// applyShapeFillModifiers folds lumMod / lumOff / tint / shade / alpha on the
// shape's solid fill into baseHex so contrast is judged against the colour
// the viewer actually sees. Without this, a light accent tint is evaluated
// as the saturated base accent and correct dark text gets "fixed" to white.
func applyShapeFillModifiers(baseHex string, spPr []byte, themeColors []types.ThemeColor) string {
	if baseHex == "" {
		return ""
	}
	block := shapeFillBlockRegexp.FindSubmatch(spPr)
	if len(block) < 2 {
		return baseHex
	}
	mods := patterns.ColorMods{Alpha: 1}
	found := false
	for _, m := range shapeFillModRegexp.FindAllSubmatch(block[1], -1) {
		v, err := strconv.Atoi(string(m[2]))
		if err != nil {
			continue
		}
		found = true
		switch string(m[1]) {
		case "lumMod":
			mods.LumMod = v
		case "lumOff":
			mods.LumOff = v
		case "tint":
			mods.Tint = v
		case "shade":
			mods.Shade = v
		case "alpha":
			mods.Alpha = float64(v) / 100000
		}
	}
	if !found {
		return baseHex
	}
	base, err := svggen.ParseColor(baseHex)
	if err != nil {
		return baseHex
	}
	bg := svggen.Color{R: 255, G: 255, B: 255, A: 1}
	if bgHex := resolveSchemeColorToHex("lt1", themeColors); bgHex != "" {
		if c, perr := svggen.ParseColor(bgHex); perr == nil {
			bg = c
		}
	}
	return strings.ToUpper(patterns.EffectiveColorMods(base, mods, bg).Hex())
}

// enforceShapeGridContrast checks text colors within shape_grid raw shape XML
// fragments against each shape's own fill color. It auto-fixes low-contrast
// text (below WCAG AA Large 3:1) regardless of whether the fill uses a
// semantic scheme color or an explicit hex value. Users who want to preserve
// exact color choices can opt out via contrast_check: false on the slide.
//
// whiteTextSafeHex is an optional allowlist of uppercase hex colors (e.g.,
// "#4472C4") that the template metadata certifies as safe for white text.
// When a shape's fill matches one of these colors and the text foreground is
// white/lt1, the auto-fix is skipped.
//
// This is called after the standard enforceTextContrastInSlide pass, which
// handles parsed slide shapes with template-inherited colors.
//
// slideIndex is the 0-based index into the input slides array. Each shape's
// swaps are stamped with a per-shape JSON path
// ("/slides/{slideIndex}/shape_grid/shapes/{i}") so contrast_autofixed findings
// point back to the specific rendered cell. The flat shape index is used because
// the original grid row/cell coordinates are not retained on the raw shape XML
// at render time.
func enforceShapeGridContrast(shapes [][]byte, themeColors []types.ThemeColor, whiteTextSafeHex map[string]bool, slideIndex int) ([][]byte, []ContrastSwap) {
	// Sibling cells first: a text colour shared by several cells is decided once,
	// against the worst fill in the group, so a tinted stack does not come out in
	// three colours (go-slide-creator-tnx3e). Whatever the group settled is
	// already readable, so the per-shape pass below finds nothing left to do on
	// those cells.
	shapes, allSwaps := enforceGridGroupContrast(shapes, themeColors, whiteTextSafeHex, slideIndex)
	gridPath := slidepath.ShapeGrid(slideIndex)
	for i, shape := range shapes {
		var swaps []ContrastSwap
		shapes[i], swaps = fixShapeXMLContrast(shape, themeColors, whiteTextSafeHex)
		annotateContrastSwaps(swaps, slideIndex, slidepath.Join(gridPath, fmt.Sprintf("shapes/%d", i)), "shape_grid")
		allSwaps = append(allSwaps, swaps...)
	}
	return shapes, allSwaps
}

// fixShapeXMLContrast fixes low-contrast text in a raw shape XML fragment.
// It resolves the shape's fill color (scheme or sRGB) to hex, then replaces
// text colors in the txBody that have insufficient contrast.
//
// When the fill matches a white-text-safe accent (per whiteTextSafeHex) and the
// text foreground is white/lt1, the fix is skipped — the template metadata
// certifies that pairing as safe.
func fixShapeXMLContrast(shapeXML []byte, themeColors []types.ThemeColor, whiteTextSafeHex map[string]bool) ([]byte, []ContrastSwap) {
	fillHex := extractShapeFillHex(shapeXML, themeColors)
	if fillHex == "" {
		return shapeXML, nil
	}

	bgColor, err := svggen.ParseColor(fillHex)
	if err != nil {
		return shapeXML, nil
	}

	// Check if this fill is white-text-safe
	fillSafe := len(whiteTextSafeHex) > 0 && whiteTextSafeHex[strings.ToUpper(fillHex)]

	// Find txBody section
	txStart := bytes.Index(shapeXML, []byte("<p:txBody>"))
	closingTag := []byte("</p:txBody>")
	txEnd := bytes.Index(shapeXML, closingTag)
	if txStart < 0 || txEnd < 0 || txEnd <= txStart {
		return shapeXML, nil
	}
	txEnd += len(closingTag)

	txBody := string(shapeXML[txStart:txEnd])

	var swaps []ContrastSwap
	// The whole body is fixed against one background, so its threshold comes
	// from its SMALLEST text: an 11pt supporting line needs 4.5:1 even when it
	// sits beside a 28pt KPI value (go-slide-creator-9ux4).
	bodyPt, bodyBold := smallestTextPt(txBody)
	threshold := contrastThresholdFor(bodyPt, bodyBold)
	// Fix scheme colors in text (with white-text-safe awareness)
	// Shape-grid colors are author-specified on the shape's own fill, not
	// inherited through a layout, so no color map override applies.
	// A shape-grid cell is NOT an author-introduced background for this purpose:
	// the author chose the fill and the text colour together, so the hue they
	// picked is intent worth preserving and the lerp stays (go-slide-creator-s7wmh).
	fixed := fixSchemeColorsForContrast(txBody, bgColor, fillHex, themeColors, &swaps, "shape_grid", "shape_grid", fillSafe, threshold, nil, false)
	// Fix sRGB colors in text (with white-text-safe awareness)
	fixed = fixSrgbColorsForContrast(fixed, bgColor, fillHex, themeColors, &swaps, fillSafe, threshold, false)

	if fixed == txBody {
		return shapeXML, nil // No changes needed
	}

	// Reconstruct the shape XML with the fixed txBody
	result := make([]byte, 0, len(shapeXML))
	result = append(result, shapeXML[:txStart]...)
	result = append(result, []byte(fixed)...)
	result = append(result, shapeXML[txEnd:]...)
	return result, swaps
}

// srgbClrInFillRegexp matches <a:solidFill><a:srgbClr val="RRGGBB"/></a:solidFill>
// in text run properties. Captures the full element and the hex color.
var srgbClrInFillRegexp = regexp.MustCompile(
	`(<a:solidFill\b[^>]*>\s*<a:srgbClr\s+val=")([0-9A-Fa-f]{6})("\s*(?:/>|>[^<]*</a:srgbClr>)\s*</a:solidFill>)`,
)

// fixSrgbColorsForContrast scans an XML fragment for sRGB color references
// inside solidFill elements. For each sRGB color with insufficient contrast
// against bgColor, it is replaced with a high-contrast color.
//
// When fillSafe is true and the foreground color is white (#FFFFFF), the fix
// is skipped — the template metadata certifies that white text on this fill
// is safe.
//
// When the original foreground is a pure neutral (white or black), the fix
// snaps to dk1/lt1 (clean flip) instead of lerping to an intermediate gray.
// This avoids the muddy-gray-on-light-yellow result that the binary-search
// EnsureContrast produces when both fg and target are close to one extreme.
func fixSrgbColorsForContrast(xmlFragment string, bgColor svggen.Color, bgHex string, themeColors []types.ThemeColor, swaps *[]ContrastSwap, fillSafe bool, threshold float64, authorBackground bool) string {
	return srgbClrInFillRegexp.ReplaceAllStringFunc(xmlFragment, func(match string) string {
		submatches := srgbClrInFillRegexp.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		hexVal := submatches[2]

		// Skip fix for white text on white-text-safe fills
		if fillSafe && isWhiteOrLt1(hexVal) {
			return match
		}

		fgColor, err := svggen.ParseColor("#" + hexVal)
		if err != nil {
			return match
		}

		ratio := fgColor.ContrastWith(bgColor)
		if ratio >= threshold {
			return match // Contrast already meets the ratio this text size needs
		}

		fixedColor, _, fixedScheme := contrastReplacementScheme(hexVal, fgColor, bgColor, themeColors, threshold, authorBackground)
		newRatio := fixedColor.ContrastWith(bgColor)

		slog.Warn("text contrast fix: replacing low-contrast sRGB color",
			slog.String("source", "shape_grid"),
			slog.String("background", bgHex),
			slog.String("before", "#"+strings.ToUpper(hexVal)),
			slog.Float64("ratio_before", ratio),
			slog.String("after", fixedColor.Hex()),
			slog.String("after_scheme", fixedScheme),
			slog.Float64("ratio_after", newRatio),
		)

		*swaps = append(*swaps, ContrastSwap{
			OriginalColor:   "#" + strings.ToUpper(hexVal),
			ReplacedColor:   fixedColor.Hex(),
			BackgroundColor: bgHex,
			RatioBefore:     math.Round(ratio*100) / 100,
			RatioAfter:      math.Round(newRatio*100) / 100,
		})

		newHex := strings.TrimPrefix(fixedColor.Hex(), "#")
		return submatches[1] + newHex + submatches[3]
	})
}

// fixSchemeColorsForContrast scans an XML fragment for scheme color references
// inside solidFill elements. For each scheme color that resolves to a color
// with insufficient contrast against bgColor, the scheme color reference is
// replaced with an sRGB color that meets WCAG AA normal (4.5:1).
//
// The replacement color is computed by the existing EnsureContrast algorithm,
// which darkens or lightens the resolved color just enough to meet the threshold
// while preserving the hue.
//
// When fillSafe is true and the foreground scheme color is lt1/bg1 (white),
// the fix is skipped — the template metadata certifies that white text on
// this fill is safe.
func fixSchemeColorsForContrast(xmlFragment string, bgColor svggen.Color, bgHex string, themeColors []types.ThemeColor, swaps *[]ContrastSwap, shapeName, source string, fillSafe bool, threshold float64, override map[string]string, authorBackground bool) string {
	return schemeClrInFillRegexp.ReplaceAllStringFunc(xmlFragment, func(match string) string {
		// Extract the scheme color name from the match
		submatches := schemeClrInFillRegexp.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		schemeName := submatches[2]

		// Skip fix for white/lt1 text on white-text-safe fills
		if fillSafe && isWhiteOrLt1(schemeName) {
			return match
		}

		// Resolve scheme color to hex the way the renderer will: through the
		// layout's color map override. On an inverted layout tx1 means lt1, and
		// reading it literally makes the pass reason about a color the slide will
		// never show (go-slide-creator-hln7).
		hexColor := resolveSchemeColorMapped(schemeName, override, themeColors)
		if hexColor == "" {
			return match // Cannot resolve, leave as-is
		}

		// Parse the resolved color
		fgColor, err := svggen.ParseColor(hexColor)
		if err != nil {
			return match
		}

		// Check contrast ratio — use large text threshold (3:1) since
		// presentation text is almost always >= 18pt or >= 14pt bold.
		ratio := fgColor.ContrastWith(bgColor)
		if ratio >= svggen.WCAGAALarge {
			return match // Contrast is adequate (large text threshold: 3:1)
		}

		// Compute a high-contrast replacement color. For pure-neutral scheme
		// foregrounds (lt1/bg1/dk1/tx1) contrastReplacement snaps to the opposite
		// theme extreme rather than lerping into a muddy gray — this fixes the
		// white-on-light-yellow case where EnsureContrast otherwise produces
		// ~#606060.
		fixedColor, _, fixedScheme := contrastReplacementScheme(schemeName, fgColor, bgColor, themeColors, threshold, authorBackground)
		newRatio := fixedColor.ContrastWith(bgColor)

		slog.Warn("text contrast fix: replacing low-contrast scheme color",
			slog.String("shape", shapeName),
			slog.String("source", source),
			slog.String("background", bgHex),
			slog.String("before", schemeName+" "+hexColor),
			slog.Float64("ratio_before", ratio),
			slog.String("after", fixedColor.Hex()),
			slog.String("after_scheme", fixedScheme),
			slog.Float64("ratio_after", newRatio),
		)

		*swaps = append(*swaps, ContrastSwap{
			OriginalColor:   hexColor,
			ReplacedColor:   fixedColor.Hex(),
			BackgroundColor: bgHex,
			RatioBefore:     math.Round(ratio*100) / 100,
			RatioAfter:      math.Round(newRatio*100) / 100,
		})

		// Replace <a:solidFill><a:schemeClr val="X"/></a:solidFill>
		// with    <a:solidFill><a:srgbClr val="RRGGBB"/></a:solidFill>
		// (RRGGBB is the chosen theme slot's value, e.g. dk2).
		hexVal := strings.TrimPrefix(fixedColor.Hex(), "#")
		return fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexVal)
	})
}

// runTextSize returns a run's declared size in points and whether it is bold,
// falling back to the inherited body default when the run declares no size.
func runTextSize(run *runXML) (float64, bool) {
	if run == nil || run.RunProperties == nil {
		return defaultBodyTextPt, false
	}
	pt := defaultBodyTextPt
	if run.RunProperties.FontSize != "" {
		if hundredths, err := strconv.Atoi(run.RunProperties.FontSize); err == nil && hundredths > 0 {
			pt = float64(hundredths) / 100.0
		}
	}
	return pt, run.RunProperties.Bold == "1"
}
