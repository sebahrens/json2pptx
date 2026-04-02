// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ahrens/svggen"
	"github.com/ahrens/go-slide-creator/internal/types"
)

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

	// Try scheme color reference
	if m := layoutBgSchemeClrRegexp.FindStringSubmatch(xmlStr); len(m) >= 2 {
		if rgb := resolveSchemeColorToHex(m[1], themeColors); rgb != "" {
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
// This function mutates the slide's shapes in place.
func enforceTextContrastInSlide(slide *slideXML, bgHex string, themeColors []types.ThemeColor) {
	if bgHex == "" || slide == nil {
		return
	}

	bgColor, err := svggen.ParseColor(bgHex)
	if err != nil {
		slog.Debug("text contrast: failed to parse background color", slog.String("bg", bgHex))
		return
	}

	for i := range slide.CommonSlideData.ShapeTree.Shapes {
		shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
		enforceTextContrastInShape(shape, bgColor, themeColors)
	}
}

// enforceTextContrastInShape checks and fixes text color contrast in a single shape.
// It processes both the lstStyle (inherited styling) and individual run properties.
func enforceTextContrastInShape(shape *shapeXML, bgColor svggen.Color, themeColors []types.ThemeColor) {
	if shape.TextBody == nil {
		return
	}

	// Fix lstStyle inherited text colors
	if shape.TextBody.ListStyle != nil && shape.TextBody.ListStyle.Inner != "" {
		shape.TextBody.ListStyle.Inner = fixSchemeColorsForContrast(
			shape.TextBody.ListStyle.Inner, bgColor, themeColors,
			shape.NonVisualProperties.ConnectionNonVisual.Name, "lstStyle",
		)
	}

	// Fix run-level text colors
	for pi := range shape.TextBody.Paragraphs {
		para := &shape.TextBody.Paragraphs[pi]
		for ri := range para.Runs {
			run := &para.Runs[ri]
			if run.RunProperties != nil && run.RunProperties.Inner != "" {
				run.RunProperties.Inner = fixSchemeColorsForContrast(
					run.RunProperties.Inner, bgColor, themeColors,
					shape.NonVisualProperties.ConnectionNonVisual.Name, "run",
				)
			}
		}
	}
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
		return "#" + strings.ToUpper(string(m[1]))
	}

	// Try scheme color
	if m := shapeFillSchemeRegexp.FindSubmatch(spPr); len(m) >= 2 {
		return resolveSchemeColorToHex(string(m[1]), themeColors)
	}

	return ""
}

// enforceShapeGridContrast checks text colors within shape_grid raw shape XML
// fragments against each shape's own fill color. When a text color has
// insufficient contrast (below WCAG AA 4.5:1) against the shape fill, a
// warning is logged but the color is NOT replaced. Shape_grid text colors are
// user-specified in JSON input and silently replacing them destroys design
// intent (e.g., gold KPI numbers on accent fills, white text on dark cards).
//
// This is called after the standard enforceTextContrastInSlide pass, which
// handles parsed slide shapes with template-inherited colors.
func enforceShapeGridContrast(shapes [][]byte, themeColors []types.ThemeColor) [][]byte {
	for _, shape := range shapes {
		warnShapeXMLContrast(shape, themeColors)
	}
	return shapes
}

// warnShapeXMLContrast checks text contrast within a single raw shape XML
// fragment against its own fill color. It logs warnings for low-contrast
// text but does NOT modify the XML — shape_grid colors are user-specified.
func warnShapeXMLContrast(shapeXML []byte, themeColors []types.ThemeColor) {
	fillHex := extractShapeFillHex(shapeXML, themeColors)
	if fillHex == "" {
		return // No fill or can't resolve — skip
	}

	bgColor, err := svggen.ParseColor(fillHex)
	if err != nil {
		return
	}

	// Find txBody section
	txStart := bytes.Index(shapeXML, []byte("<p:txBody>"))
	closingTag := []byte("</p:txBody>")
	txEnd := bytes.Index(shapeXML, closingTag)
	if txStart < 0 || txEnd < 0 || txEnd <= txStart {
		return
	}
	txEnd += len(closingTag)

	txBody := string(shapeXML[txStart:txEnd])

	// Check scheme colors
	for _, match := range schemeClrInFillRegexp.FindAllStringSubmatch(txBody, -1) {
		if len(match) < 4 {
			continue
		}
		schemeName := match[2]
		hexColor := resolveSchemeColorToHex(schemeName, themeColors)
		if hexColor == "" {
			continue
		}
		fgColor, err := svggen.ParseColor(hexColor)
		if err != nil {
			continue
		}
		ratio := fgColor.ContrastWith(bgColor)
		if ratio < svggen.WCAGAALarge {
			slog.Warn("shape_grid: low text contrast (user-specified color preserved)",
				slog.String("scheme", schemeName),
				slog.String("resolved", hexColor),
				slog.String("fill", fillHex),
				slog.Float64("contrast_ratio", ratio),
				slog.Float64("wcag_aa_large", svggen.WCAGAALarge),
			)
		}
	}

	// Check sRGB colors
	for _, match := range srgbClrInFillRegexp.FindAllStringSubmatch(txBody, -1) {
		if len(match) < 4 {
			continue
		}
		hexVal := match[2]
		fgColor, err := svggen.ParseColor("#" + hexVal)
		if err != nil {
			continue
		}
		ratio := fgColor.ContrastWith(bgColor)
		if ratio < svggen.WCAGAALarge {
			slog.Warn("shape_grid: low text contrast (user-specified color preserved)",
				slog.String("color", "#"+hexVal),
				slog.String("fill", fillHex),
				slog.Float64("contrast_ratio", ratio),
				slog.Float64("wcag_aa_large", svggen.WCAGAALarge),
			)
		}
	}
}

// srgbClrInFillRegexp matches <a:solidFill><a:srgbClr val="RRGGBB"/></a:solidFill>
// in text run properties. Captures the full element and the hex color.
var srgbClrInFillRegexp = regexp.MustCompile(
	`(<a:solidFill\b[^>]*>\s*<a:srgbClr\s+val=")([0-9A-Fa-f]{6})("\s*(?:/>|>[^<]*</a:srgbClr>)\s*</a:solidFill>)`,
)

// fixSrgbColorsForContrast scans an XML fragment for sRGB color references
// inside solidFill elements. For each sRGB color with insufficient contrast
// against bgColor, it is replaced with a high-contrast color.
func fixSrgbColorsForContrast(xmlFragment string, bgColor svggen.Color) string {
	return srgbClrInFillRegexp.ReplaceAllStringFunc(xmlFragment, func(match string) string {
		submatches := srgbClrInFillRegexp.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		hexVal := submatches[2]

		fgColor, err := svggen.ParseColor("#" + hexVal)
		if err != nil {
			return match
		}

		ratio := fgColor.ContrastWith(bgColor)
		if ratio >= svggen.WCAGAALarge {
			return match // Contrast is adequate (large text threshold: 3:1)
		}

		fixedColor := svggen.EnsureContrast(fgColor, bgColor, svggen.WCAGAALarge)

		slog.Info("text contrast fix: replacing low-contrast sRGB color",
			slog.String("source", "shape_grid"),
			slog.String("original", "#"+hexVal),
			slog.Float64("contrast_ratio", ratio),
			slog.String("replacement", fixedColor.Hex()),
			slog.Float64("new_ratio", fixedColor.ContrastWith(bgColor)),
		)

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
func fixSchemeColorsForContrast(xmlFragment string, bgColor svggen.Color, themeColors []types.ThemeColor, shapeName, source string) string {
	return schemeClrInFillRegexp.ReplaceAllStringFunc(xmlFragment, func(match string) string {
		// Extract the scheme color name from the match
		submatches := schemeClrInFillRegexp.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}
		schemeName := submatches[2]

		// Resolve scheme color to hex
		hexColor := resolveSchemeColorToHex(schemeName, themeColors)
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

		// Compute a high-contrast replacement color
		fixedColor := svggen.EnsureContrast(fgColor, bgColor, svggen.WCAGAALarge)

		slog.Info("text contrast fix: replacing low-contrast scheme color",
			slog.String("shape", shapeName),
			slog.String("source", source),
			slog.String("scheme", schemeName),
			slog.String("resolved", hexColor),
			slog.Float64("contrast_ratio", ratio),
			slog.String("replacement", fixedColor.Hex()),
			slog.Float64("new_ratio", fixedColor.ContrastWith(bgColor)),
		)

		// Replace <a:solidFill><a:schemeClr val="X"/></a:solidFill>
		// with    <a:solidFill><a:srgbClr val="RRGGBB"/></a:solidFill>
		hexVal := strings.TrimPrefix(fixedColor.Hex(), "#")
		return fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexVal)
	})
}
