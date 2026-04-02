// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/textfit"
)

// applySmartAutofit uses font metrics to determine whether text overflows the
// placeholder and sets OOXML normAutofit with calculated fontScale and
// lnSpcReduction values. Falls back to basic normAutofit if shape dimensions
// are unavailable. When content overflows even at maximum scaling, trims
// trailing paragraphs to prevent clipped text.
// themeFontName is used as fallback when the shape has no explicit typeface.
// autofitOption configures applySmartAutofit behaviour.
type autofitOption func(*autofitConfig)

type autofitConfig struct {
	themeFontName       string
	readabilityMinScale int // override default 62500 (62.5%)
	minFontScalePct     int // override textfit min font scale floor (default 0 = use textfit default 60%)
}

// withThemeFont sets the theme font name for autofit calculations.
func withThemeFont(name string) autofitOption {
	return func(c *autofitConfig) { c.themeFontName = name }
}

// withReadabilityMinScale overrides the minimum font scale threshold below
// which paragraphs are trimmed for readability. Default is 62500 (62.5%).
// Use a lower value (e.g. 45000) for dense content like 4+ bullet groups
// where preserving all content is more important than larger font size.
func withReadabilityMinScale(scale int) autofitOption {
	return func(c *autofitConfig) { c.readabilityMinScale = scale }
}

// withMinFontScalePct overrides the textfit minimum font scale floor (default 60%).
// Use a lower value (e.g. 45) for dense content where fitting all items matters
// more than maintaining a large minimum font size.
func withMinFontScalePct(pct int) autofitOption {
	return func(c *autofitConfig) { c.minFontScalePct = pct }
}

func applySmartAutofit(shape *shapeXML, themeFontName ...string) {
	var opts []autofitOption
	if len(themeFontName) > 0 && themeFontName[0] != "" {
		opts = append(opts, withThemeFont(themeFontName[0]))
	}
	applySmartAutofitWithOptions(shape, opts...)
}

func applySmartAutofitWithOptions(shape *shapeXML, opts ...autofitOption) {
	cfg := autofitConfig{
		readabilityMinScale: 62500, // 62.5% default
	}
	for _, o := range opts {
		o(&cfg)
	}

	if shape.TextBody == nil || shape.TextBody.BodyProperties == nil {
		return
	}

	bp := shape.TextBody.BodyProperties
	// Respect <a:noAutofit/> — the template explicitly opted out of text scaling.
	if strings.Contains(bp.Inner, "noAutofit") || strings.Contains(bp.Inner, "noAutoFit") {
		return
	}
	// Strip any existing normAutofit from the template. Our content-aware
	// calculation (below) produces a more accurate fontScale for the authored
	// content than the template's generic normAutofit which was calibrated for
	// placeholder text. Without this strip, the old early-return path would
	// skip our calculation and the template's normAutofit thresholds would be
	// applied, ignoring the caller's minFontScalePct / readabilityMinScale
	// options. This fixes dense bullet lists (≥10 items) on templates like
	// templates where the existing normAutofit caused aggressive truncation.
	bp.Inner = normAutofitRegexp.ReplaceAllString(bp.Inner, "")

	// Extract placeholder dimensions from shape transform
	widthEMU, heightEMU := getShapeDimensions(shape)
	if widthEMU <= 0 || heightEMU <= 0 {
		// No dimensions available — estimate a fontScale from paragraph count.
		// A typical body placeholder (~4.5 inches tall) fits ~14 lines at default
		// font size (20pt + 1.2× line spacing + spcBef). When there are more
		// paragraphs, apply proportional scaling to prevent overflow that
		// rendering engines (LibreOffice) may not auto-shrink well enough.
		// The floor is 70% (not 60%) because this heuristic can't account for
		// actual text width/wrapping — being overly aggressive makes dense
		// content like 4-group bullet layouts illegibly small.
		paraCount := len(shape.TextBody.Paragraphs)
		const typicalFitLines = 14
		if paraCount > typicalFitLines {
			scalePct := (typicalFitLines * 100) / paraCount
			if scalePct < 70 {
				scalePct = 70 // conservative floor for dimensionless estimate
			}
			bp.Inner += fmt.Sprintf(`<a:normAutofit fontScale="%d"/>`, scalePct*1000)
		} else {
			bp.Inner += `<a:normAutofit/>`
		}
		return
	}

	// Collect paragraph texts and font info
	texts := collectParagraphTexts(shape.TextBody.Paragraphs)
	if len(texts) == 0 {
		return
	}

	fontSizeHPt := extractFontSizeFromShape(shape)
	fontName := extractFontNameFromShape(shape)
	if fontName == "" && cfg.themeFontName != "" {
		fontName = cfg.themeFontName // Use theme font when shape has no explicit typeface
	}

	// When the font size is inherited from the slide master (not explicit in the shape),
	// the master's bodyStyle typically adds spcBef + spcAft (~10pt + 2pt = 12pt per paragraph).
	// Account for this extra spacing in the height estimate.
	var extraSpacingPt float64
	if fontSizeHPt == 0 {
		extraSpacingPt = 12.0 // typical slide master spcBef + spcAft
	}

	// Extract per-paragraph spacings from explicit spcBef values in paragraph properties.
	// Bullet group headers (spcBef=12pt) and trailing body paragraphs (spcBef=24pt) add
	// extra height that the uniform ExtraSpacingPt doesn't account for.
	perParaSpacings := extractParagraphSpacings(shape.TextBody.Paragraphs, extraSpacingPt)

	// Extract per-paragraph left margins from bullet levels (marL in lstStyle).
	// Bullet paragraphs inherit left margins that reduce the available width for wrapping.
	leftMargins := extractParagraphLeftMargins(shape.TextBody.Paragraphs, shape.TextBody.ListStyle)

	params := textfit.Params{
		WidthEMU:        widthEMU,
		HeightEMU:       heightEMU,
		FontSizeHPt:     fontSizeHPt,
		FontName:        fontName,
		Paragraphs:      texts,
		ExtraSpacingPt:  extraSpacingPt,
		ExtraSpacingsPt: perParaSpacings,
		LeftMarginsPt:   leftMargins,
		MinFontScalePct: cfg.minFontScalePct,
	}

	result := textfit.Calculate(params)
	// Prefer readability over completeness: when font would shrink below the
	// readability threshold and there are enough paragraphs to trim, remove
	// trailing paragraphs to keep text at a legible size.
	// Default threshold is 62500 (62.5% → ~12.5pt for 20pt base font).
	// Callers can lower this for dense content like 4+ bullet groups where
	// preserving all authored content is more important than larger font size.
	readabilityMinFontScale := cfg.readabilityMinScale
	if result.FontScale >= readabilityMinFontScale {
		// Font scale is acceptable, no trimming needed
	} else if result.FontScale > 0 && len(texts) > 6 {
		result = trimForReadability(shape, params, readabilityMinFontScale)
	}

	// When content overflows even at maximum scaling, trim trailing paragraphs
	// so text is never clipped at the bottom of the placeholder.
	if result.Overflow {
		result = trimOverflowParagraphs(shape, params)
	}

	// Always add normAutofit to prevent text clipping. Without it, the empty
	// <a:bodyPr/> overrides the slide master's normAutofit, disabling LibreOffice's
	// built-in shrink-to-fit. This is a safety net for cases where our height
	// estimate is slightly optimistic (e.g., bold text width, inherited marL).
	// Build normAutofit element with calculated values
	var attrs []string
	if result.FontScale > 0 {
		attrs = append(attrs, fmt.Sprintf(`fontScale="%d"`, result.FontScale))
	}
	if result.LnSpcReduction > 0 {
		attrs = append(attrs, fmt.Sprintf(`lnSpcReduction="%d"`, result.LnSpcReduction))
	}

	if len(attrs) > 0 {
		bp.Inner += fmt.Sprintf(`<a:normAutofit %s/>`, strings.Join(attrs, " "))
	} else {
		bp.Inner += `<a:normAutofit/>`
	}
}

// trimOverflowParagraphs removes trailing paragraphs from the shape until the
// content fits within the placeholder at minimum font scale. Adds a "..." indicator
// to show content was truncated. Returns the recalculated FitResult.
func trimOverflowParagraphs(shape *shapeXML, params textfit.Params) textfit.FitResult {
	paras := shape.TextBody.Paragraphs

	// Need at least 2 paragraphs to trim (keep 1 + add "..." indicator)
	for len(paras) > 2 {
		// Remove last paragraph
		paras = paras[:len(paras)-1]

		// Build trimmed parameter slices
		trimmedTexts := collectParagraphTexts(paras)
		// Add "..." indicator text to height calculation
		trimmedTexts = append(trimmedTexts, "\u2026") // ellipsis character

		var trimmedSpacings []float64
		if params.ExtraSpacingsPt != nil {
			n := len(paras)
			if n < len(params.ExtraSpacingsPt) {
				trimmedSpacings = make([]float64, n+1)
				copy(trimmedSpacings, params.ExtraSpacingsPt[:n])
			} else {
				trimmedSpacings = make([]float64, n+1)
				copy(trimmedSpacings, params.ExtraSpacingsPt)
			}
			trimmedSpacings[n] = params.ExtraSpacingPt // "..." uses base spacing
		}

		var trimmedMargins []float64
		if params.LeftMarginsPt != nil {
			n := len(paras)
			if n < len(params.LeftMarginsPt) {
				trimmedMargins = make([]float64, n+1)
				copy(trimmedMargins, params.LeftMarginsPt[:n])
			} else {
				trimmedMargins = make([]float64, n+1)
				copy(trimmedMargins, params.LeftMarginsPt)
			}
			// "..." indicator has no extra margin
		}

		trimmedParams := params
		trimmedParams.Paragraphs = trimmedTexts
		trimmedParams.ExtraSpacingsPt = trimmedSpacings
		trimmedParams.LeftMarginsPt = trimmedMargins

		result := textfit.Calculate(trimmedParams)
		if !result.Overflow {
			// Content fits after trimming — update shape paragraphs
			shape.TextBody.Paragraphs = paras

			// Add "..." indicator paragraph using first paragraph's run properties
			var rProps *runPropertiesXML
			if len(paras) > 0 && len(paras[0].Runs) > 0 {
				rProps = paras[0].Runs[0].RunProperties
			}
			ellipsisPara := paragraphXML{
				Properties: noBulletParagraphProps(""),
				Runs: []runXML{{
					RunProperties: rProps,
					Text:          "\u2026",
				}},
			}
			shape.TextBody.Paragraphs = append(shape.TextBody.Paragraphs, ellipsisPara)

			slog.Info("trimmed overflow: removed paragraphs to fit placeholder",
				slog.Int("original", len(params.Paragraphs)),
				slog.Int("remaining", len(paras)+1)) // +1 for "..." indicator

			return result
		}
	}

	// Even 2 paragraphs don't fit — apply maximum scaling and accept overflow
	slog.Warn("text overflow: content does not fit even after trimming",
		slog.Int("paragraphs", len(params.Paragraphs)))
	return textfit.FitResult{
		FontScale:      50000,
		LnSpcReduction: 20000,
		Overflow:       true,
	}
}

// trimForReadability removes trailing paragraphs from the shape until the font
// scale meets the desired minimum (e.g., 70000 = 70%). Unlike trimOverflowParagraphs
// which only runs on overflow, this proactively trims dense content to keep text
// at a presentation-legible size. Adds a "…" indicator when paragraphs are removed.
func trimForReadability(shape *shapeXML, params textfit.Params, targetMinFontScale int) textfit.FitResult {
	paras := shape.TextBody.Paragraphs

	// Need at least 4 paragraphs to consider trimming for readability
	// (keep at least 3 visible + 1 "…" indicator)
	for len(paras) > 4 {
		// Remove last paragraph
		paras = paras[:len(paras)-1]

		// Build trimmed parameter slices
		trimmedTexts := collectParagraphTexts(paras)
		trimmedTexts = append(trimmedTexts, "\u2026") // ellipsis

		var trimmedSpacings []float64
		if params.ExtraSpacingsPt != nil {
			n := len(paras)
			trimmedSpacings = make([]float64, n+1)
			if n < len(params.ExtraSpacingsPt) {
				copy(trimmedSpacings, params.ExtraSpacingsPt[:n])
			} else {
				copy(trimmedSpacings, params.ExtraSpacingsPt)
			}
			trimmedSpacings[n] = params.ExtraSpacingPt
		}

		var trimmedMargins []float64
		if params.LeftMarginsPt != nil {
			n := len(paras)
			trimmedMargins = make([]float64, n+1)
			if n < len(params.LeftMarginsPt) {
				copy(trimmedMargins, params.LeftMarginsPt[:n])
			} else {
				copy(trimmedMargins, params.LeftMarginsPt)
			}
		}

		trimmedParams := params
		trimmedParams.Paragraphs = trimmedTexts
		trimmedParams.ExtraSpacingsPt = trimmedSpacings
		trimmedParams.LeftMarginsPt = trimmedMargins

		result := textfit.Calculate(trimmedParams)
		if !result.Overflow && (result.FontScale == 0 || result.FontScale >= targetMinFontScale) {
			// Content fits at an acceptable font scale — update shape
			shape.TextBody.Paragraphs = paras

			// Add "…" indicator paragraph
			var rProps *runPropertiesXML
			if len(paras) > 0 && len(paras[0].Runs) > 0 {
				rProps = paras[0].Runs[0].RunProperties
			}
			ellipsisPara := paragraphXML{
				Properties: noBulletParagraphProps(""),
				Runs: []runXML{{
					RunProperties: rProps,
					Text:          "\u2026",
				}},
			}
			shape.TextBody.Paragraphs = append(shape.TextBody.Paragraphs, ellipsisPara)

			slog.Info("trimmed for readability: removed paragraphs to improve font scale",
				slog.Int("original", len(params.Paragraphs)),
				slog.Int("remaining", len(paras)+1),
				slog.Int("fontScale", result.FontScale))

			return result
		}
	}

	// Couldn't achieve target — return original calculation
	return textfit.Calculate(params)
}

// getShapeDimensions returns the width and height of a shape in EMUs.
// Returns (0, 0) if the shape has no transform.
func getShapeDimensions(shape *shapeXML) (width, height int64) {
	if shape.ShapeProperties.Transform == nil {
		return 0, 0
	}
	return shape.ShapeProperties.Transform.Extent.CX, shape.ShapeProperties.Transform.Extent.CY
}

// spAutoFitRegexp matches <a:spAutoFit/> or <spAutoFit/> elements in bodyPr inner XML,
// including variants with namespace declarations (e.g., xmlns:a="...").
// This element tells renderers to grow the text box to fit text (used by decorative
// placeholders like section numbers). We strip it when populating body text so that
// normAutofit (shrink text to fit box) can be applied instead.
var spAutoFitRegexp = regexp.MustCompile(`<(?:a:)?spAutoFit\b[^>]*/>|<(?:a:)?spAutoFit\b[^>]*>.*?</(?:a:)?spAutoFit>`)

// normAutofitRegexp matches <a:normAutofit .../> elements in bodyPr inner XML.
// We strip existing normAutofit before computing our own content-aware autofit
// values, since the template's normAutofit was calibrated for placeholder text,
// not for the authored content we are populating.
var normAutofitRegexp = regexp.MustCompile(`<(?:a:)?normAutofit\b[^>]*/>|<(?:a:)?normAutofit\b[^>]*>.*?</(?:a:)?normAutofit>`)

// replaceSpAutoFitWithNorm strips <a:spAutoFit/> from the shape's bodyPr Inner XML.
// In OOXML, spAutoFit means "grow the text box to fit the text" — the opposite of what
// we want for content text. When a layout placeholder (like a section header
// layout's decorative placeholder) has spAutoFit, populating it with long body text causes overflow
// because the text box tries to grow beyond the slide boundary. By stripping spAutoFit,
// the subsequent applySmartAutofit call can add normAutofit (which shrinks text to fit).
func replaceSpAutoFitWithNorm(shape *shapeXML) {
	if shape.TextBody == nil || shape.TextBody.BodyProperties == nil {
		return
	}
	bp := shape.TextBody.BodyProperties
	if bp.Inner == "" {
		return
	}
	cleaned := spAutoFitRegexp.ReplaceAllString(bp.Inner, "")
	if cleaned != bp.Inner {
		slog.Info("replaced spAutoFit with normAutofit-eligible bodyPr",
			slog.String("original_inner", bp.Inner))
		bp.Inner = cleaned
	}
}

// enforceTextWrap ensures the shape's bodyPr has wrap="square" to constrain text
// within the placeholder boundary. Without this, text in two-column or narrow
// placeholders can visually overflow into adjacent shapes (e.g., a chart column).
//
// In OOXML, wrap="square" is the default, but when <a:bodyPr/> is empty and the
// layout is round-tripped through XML parsing, the attribute can be lost.
// Setting it explicitly guarantees rendering engines (PowerPoint, LibreOffice)
// wrap text at the placeholder width defined in <a:xfrm>.
func enforceTextWrap(shape *shapeXML) {
	if shape.TextBody == nil || shape.TextBody.BodyProperties == nil {
		return
	}
	bp := shape.TextBody.BodyProperties
	// Only enforce wrap="square" if not already explicitly set.
	// Respect wrap="none" if explicitly configured by the template.
	if bp.Wrap == "" {
		bp.Wrap = "square"
	}
}

// fontScaleRe extracts the fontScale value from normAutofit XML.
var fontScaleRe = regexp.MustCompile(`fontScale="(\d+)"`)

// centerIfSparse sets vertical anchor to "ctr" when bullet content is sparse
// (few items with no significant font scaling). This prevents small bullet lists
// from floating at the top of the placeholder with excessive whitespace below,
// which looks unprofessional in consulting-style presentations.
func centerIfSparse(shape *shapeXML, paragraphCount int) {
	if shape.TextBody == nil || shape.TextBody.BodyProperties == nil {
		return
	}
	bp := shape.TextBody.BodyProperties

	// Only center sparse content (≤ 8 paragraphs)
	if paragraphCount > 8 {
		return
	}

	// Don't override if already centered or bottom-aligned
	if bp.Anchor == "ctr" || bp.Anchor == "b" {
		return
	}

	// Check if font scaling was applied — if content needed significant
	// shrinking, it's dense and should stay top-aligned to avoid overflow
	// at the bottom when centered.
	if m := fontScaleRe.FindStringSubmatch(bp.Inner); len(m) > 1 {
		if scale, err := strconv.Atoi(m[1]); err == nil && scale < 85000 {
			return // Dense content needed shrinking, keep top-aligned
		}
	}

	bp.Anchor = "ctr"
}
