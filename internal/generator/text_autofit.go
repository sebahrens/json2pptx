// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
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
	readabilityMinScale int                    // override default 62500 (62.5%)
	minFontScalePct     int                    // override textfit min font scale floor (default 0 = use textfit default 60%)
	findings            *[]patterns.FitFinding // optional collector for render-time findings
	findingPath         string                 // JSON path prefix for findings (e.g. "slides[0].content.body")

	// inherited is the text style the placeholder inherits from the slide
	// master (size / caps / line spacing), used when the shape carries no
	// explicit value. nil = unknown (legacy defaults).
	inherited *template.InheritedTextStyle
	// inheritedFontName is the concrete font family for the inherited style
	// (theme tokens already resolved).
	inheritedFontName string
	// isTitle marks a title placeholder: overflow is reported as
	// TITLE_OVERFLOW and no paragraphs are trimmed.
	isTitle bool
	// sectionTitle applies the divider-specific 28pt floor and refuses a
	// title that would require smaller text or compressed line spacing.
	sectionTitle bool
	// overrideNoAutofit allows a content-populated display title to replace a
	// template's noAutofit directive with measured shrink-to-fit protection.
	// Layout placeholder copy is often short, while authored closing statements
	// can wrap upward out of a bottom-anchored box when noAutofit is preserved.
	overrideNoAutofit bool

	// viewingMode / textRole select the readability policy the final fit is
	// judged against (TEXT_BELOW_READABLE_MIN). Empty role = not checked.
	viewingMode tokens.ViewingMode
	textRole    tokens.TextRole
	// bodyTypography normalises template-native body sizes against the
	// paragraph density before measuring fit. Explicit author sizes opt out.
	bodyTypography bool
	bodySourceHPt  int
	bodyPolicy     BodySizePolicy
	bodyBaseHPt    int
	// authoredFontSizeHPt is applied to generated runs before measurement,
	// so autofit uses the same size the output PPTX will actually render.
	authoredFontSizeHPt int
	// leadParagraphs is the number of leading body_and_lead lead paragraphs
	// sized one scale step above the bullets (go-slide-creator-q4zjj).
	leadParagraphs int
}

func withLeadParagraphs(n int) autofitOption {
	return func(c *autofitConfig) { c.leadParagraphs = n }
}

func withBodyTypography() autofitOption {
	return func(c *autofitConfig) { c.bodyTypography = true }
}

func withAuthoredFontSize(sizeHPt int) autofitOption {
	return func(c *autofitConfig) { c.authoredFontSizeHPt = sizeHPt }
}

// withInheritedTextStyle supplies the placeholder's inherited (master) text
// style so measured autofit uses the real rendered size, caps and line
// spacing (go-slide-creator-6cjs).
func withInheritedTextStyle(st template.InheritedTextStyle, fontName string) autofitOption {
	return func(c *autofitConfig) {
		c.inherited = &st
		c.inheritedFontName = fontName
	}
}

// withTitleRole marks the shape as a title placeholder.
func withTitleRole() autofitOption {
	return func(c *autofitConfig) { c.isTitle = true }
}

func withSectionTitleRole() autofitOption {
	return func(c *autofitConfig) {
		c.isTitle = true
		c.sectionTitle = true
		c.overrideNoAutofit = true
	}
}

func withNoAutofitOverride() autofitOption {
	return func(c *autofitConfig) { c.overrideNoAutofit = true }
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

// withFindingsCollector sets a collector for render-time FitFindings.
func withFindingsCollector(findings *[]patterns.FitFinding, path string) autofitOption {
	return func(c *autofitConfig) {
		c.findings = findings
		c.findingPath = path
	}
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
	if cfg.authoredFontSizeHPt > 0 {
		applyFontSizeOverride(shape, cfg.authoredFontSizeHPt)
	}
	normalizeBodyTypography(shape, &cfg)
	applyLeadParagraphSizes(shape, &cfg)

	bp := shape.TextBody.BodyProperties
	if handleNoAutofitDirective(bp, shape, &cfg) {
		emitBodySizeFinding(&cfg, 100000)
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
		applyZeroDimensionAutofit(bp, shape, &cfg)
		emitBodySizeFinding(&cfg, 100000)
		return
	}

	// Collect paragraph texts and font info
	texts := collectParagraphTexts(shape.TextBody.Paragraphs)
	if len(texts) == 0 {
		return
	}

	params := buildTextfitParams(shape, widthEMU, heightEMU, texts, &cfg)
	params.ViewingMode = cfg.viewingMode
	params.TextRole = cfg.textRole
	if cfg.sectionTitle && params.FontSizeHPt > 0 {
		params.MinFontScalePct = int(math.Ceil(float64(SectionTitleMinHPt) / float64(params.FontSizeHPt) * 100))
	}

	result, err := textfit.Calculate(params)
	if err != nil {
		slog.Warn("textfit: font cache unavailable, skipping autofit", slog.String("err", err.Error()))
		bp.Inner += `<a:normAutofit/>`
		emitBodySizeFinding(&cfg, 100000)
		return
	}
	// Titles are a single statement: never trim paragraphs. When the title
	// cannot fit even at the minimum scale, keep the maximum reduction and
	// report TITLE_OVERFLOW so the author shortens it.
	if cfg.isTitle {
		var brokenWord string
		if cfg.sectionTitle {
			result, brokenWord = capScaleForLongestWord(shape, params, result, &cfg)
		}
		if cfg.findings != nil {
			if cfg.sectionTitle && (result.Overflow || result.LnSpcReduction > 0) {
				*cfg.findings = append(*cfg.findings, newSectionTitleFloorFinding(cfg.findingPath, strings.Join(texts, " "), params, brokenWord))
			} else if result.Overflow {
				*cfg.findings = append(*cfg.findings, newTitleOverflowFinding(cfg.findingPath, strings.Join(texts, " "), params, result))
			}
		}
		// Bake the fit into explicit sizes / line spacing. Ordinary titles
		// retain normAutofit as a safety net; divider titles disable further
		// renderer shrink so the 28pt floor remains stable.
		bakeTitleFit(shape, params, result)
		balanceTitleLines(shape, params, result, &cfg)
		emitReadabilityFinding(&cfg, shape, params, result, len(shape.TextBody.Paragraphs))
		if cfg.sectionTitle {
			// Keep the measured font size stable across renderers. A bare
			// normAutofit can silently shrink below the floor on re-open.
			bp.Inner += `<a:noAutofit/>`
		} else {
			bp.Inner += `<a:normAutofit/>`
		}
		return
	}

	// Prefer readability over completeness: when font would shrink below the
	// readability threshold and there are enough paragraphs to trim, remove
	// trailing paragraphs to keep text at a legible size.
	// Default threshold is 62500 (62.5% → ~12.5pt for 20pt base font).
	// Callers can lower this for dense content like 4+ bullet groups where
	// preserving all authored content is more important than larger font size.
	if result.FontScale > 0 && result.FontScale < cfg.readabilityMinScale && len(texts) > 6 {
		result = trimForReadability(shape, params, cfg.readabilityMinScale, &cfg)
	}

	// When content overflows even at maximum scaling, trim trailing paragraphs
	// so text is never clipped at the bottom of the placeholder.
	if result.Overflow {
		result = trimOverflowParagraphs(shape, params, &cfg)
	}

	// Always add normAutofit to prevent text clipping. Without it, the empty
	// <a:bodyPr/> overrides the slide master's normAutofit, disabling LibreOffice's
	// built-in shrink-to-fit. This is a safety net for cases where our height
	// estimate is slightly optimistic (e.g., bold text width, inherited marL).
	emitReadabilityFinding(&cfg, shape, params, result, len(shape.TextBody.Paragraphs))
	emitBodySizeFinding(&cfg, result.FontScale)
	bp.Inner += buildNormAutofitElement(result)
}

// letterSpacingRegexp finds the first spc (character spacing) attribute in a
// list style.
var letterSpacingRegexp = regexp.MustCompile(`\bspc="(-?\d+)"`)

// lstBoldRegexp finds the b attribute of a list style's run defaults.
var lstBoldRegexp = regexp.MustCompile(`<a:defRPr\b[^>]*\bb="([01]|true|false)"`)

// capScaleForLongestWord shrinks a divider title until its longest word fits
// on one line. textfit counts a word wider than the box as wrapping by
// character, so a single long all-caps word ("PERFORMANCE") "fitted" as
// PERFORMAN / CE (go-slide-creator-csclk.96). The word is measured as it
// renders — inherited all-caps, weight and letter spacing — against the box's
// real insets, with the renderer slack the shape writer leaves its widest
// word (pptx.WordLineNeedEMU): Abstract's Tenorite is measured in a stand-in,
// and a 3% margin there still broke BOTTLENEC / K at 43.5pt
// (go-slide-creator-akues). When even the divider floor cannot hold the word,
// the result is marked as overflowing and the word that breaks is returned
// for the section-title floor finding.
func capScaleForLongestWord(shape *shapeXML, p textfit.Params, res textfit.FitResult, cfg *autofitConfig) (textfit.FitResult, string) {
	if p.FontSizeHPt <= 0 || len(p.Paragraphs) == 0 {
		return res, ""
	}
	ws := sectionTitleWordStyleFor(shape, cfg)
	text := strings.Join(p.Paragraphs, " ")
	if ws.caps {
		text = strings.ToUpper(text)
	}
	var marginPt float64
	for _, m := range p.LeftMarginsPt {
		marginPt = max(marginPt, m)
	}
	avail := p.WidthEMU - titleInsetsEMU(shape.TextBody.BodyProperties) - int64(marginPt*12700)
	maxHPt := maxHPtForWholeWords(text, avail, p.FontName, ws)
	if maxHPt <= 0 {
		return res, ""
	}
	scale := res.FontScale
	if scale == 0 {
		scale = 100000
	}
	if p.FontSizeHPt*scale/100000 <= maxHPt {
		return res, ""
	}
	newScale := maxHPt * 100000 / p.FontSizeHPt
	var broken string
	if p.MinFontScalePct > 0 && newScale < p.MinFontScalePct*1000 {
		newScale = p.MinFontScalePct * 1000
		res.Overflow = true
		broken = wordBrokenAt(text, p.FontSizeHPt*newScale/100000, avail, p.FontName, ws)
	}
	res.FontScale = newScale
	return res, broken
}

// titleWordStyle is what a title's words render with beyond face and size.
type titleWordStyle struct {
	caps, bold bool
	spcHPt     int
}

// sectionTitleWordStyleFor resolves the divider title's all-caps, weight and
// letter spacing: the shape's own list style over the inherited master style.
func sectionTitleWordStyleFor(shape *shapeXML, cfg *autofitConfig) titleWordStyle {
	var ws titleWordStyle
	if cfg.inherited != nil {
		ws = titleWordStyle{caps: cfg.inherited.CapsAll, bold: cfg.inherited.Bold, spcHPt: cfg.inherited.SpcHPt}
	}
	if shape.TextBody.ListStyle == nil {
		return ws
	}
	lst := shape.TextBody.ListStyle.Inner
	ws.caps = template.InheritedTextStyle{CapsAll: ws.caps}.OverrideFromListStyle(lst).CapsAll
	if m := lstBoldRegexp.FindStringSubmatch(lst); m != nil {
		ws.bold = m[1] == "1" || m[1] == "true"
	}
	if m := letterSpacingRegexp.FindStringSubmatch(lst); m != nil {
		if spc, err := strconv.Atoi(m[1]); err == nil {
			ws.spcHPt = spc
		}
	}
	return ws
}

// titleInsetsEMU is the width a body's left and right insets take (the OOXML
// 0.1" default on a side that declares none).
func titleInsetsEMU(bp *bodyPropertiesXML) int64 {
	const defaultInsetEMU = 91440
	l, r := int64(defaultInsetEMU), int64(defaultInsetEMU)
	if bp != nil && bp.LIns != nil {
		l = *bp.LIns
	}
	if bp != nil && bp.RIns != nil {
		r = *bp.RIns
	}
	return l + r
}

// wordBrokenAt returns the first word of text that does not fit one line of
// availEMU at hpt (hundredths of a point), or "" when all do or none can be
// measured.
func wordBrokenAt(text string, hpt int, availEMU int64, fontName string, ws titleWordStyle) string {
	if ws.caps {
		text = strings.ToUpper(text)
	}
	for _, w := range strings.Fields(text) {
		if need, ok := pptx.WordLineNeedEMU(w, fontName, float64(hpt)/100, ws.bold, ws.spcHPt); ok && need > availEMU {
			return w
		}
	}
	return ""
}

// maxHPtForWholeWords is the largest size (hundredths of a point) at which
// every word of text keeps to one line of availEMU, measured as
// pptx.WordLineNeedEMU measures it. 0 means it cannot be measured.
func maxHPtForWholeWords(text string, availEMU int64, fontName string, ws titleWordStyle) int {
	words := strings.Fields(text)
	if len(words) == 0 || availEMU <= 0 {
		return 0
	}
	fits := func(hpt int) (bool, bool) {
		for _, w := range words {
			need, ok := pptx.WordLineNeedEMU(w, fontName, float64(hpt)/100, ws.bold, ws.spcHPt)
			if !ok {
				return false, false
			}
			if need > availEMU {
				return false, true
			}
		}
		return true, true
	}
	lo, hi := 100, 50000 // 1pt .. 500pt
	if ok, measured := fits(lo); !measured {
		return 0
	} else if !ok {
		return lo
	}
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if ok, _ := fits(mid); ok {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// handleNoAutofitDirective respects an explicit <a:noAutofit/> in the template
// bodyPr. Returns true when the caller should bail out (the directive was set).
// Emits an ErrCodeNoAutofitOverflow finding when paragraph count is high enough
// that overflow is likely under the suppressed autofit.
func handleNoAutofitDirective(bp *bodyPropertiesXML, shape *shapeXML, cfg *autofitConfig) bool {
	if !strings.Contains(bp.Inner, "noAutofit") && !strings.Contains(bp.Inner, "noAutoFit") {
		return false
	}
	if cfg.overrideNoAutofit {
		bp.Inner = noAutofitRegexp.ReplaceAllString(bp.Inner, "")
		return false
	}
	if cfg.findings == nil {
		return true
	}
	paraCount := len(shape.TextBody.Paragraphs)
	if paraCount <= 6 {
		return true
	}
	*cfg.findings = append(*cfg.findings, patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    cfg.findingPath,
			Code:    patterns.ErrCodeNoAutofitOverflow,
			Message: fmt.Sprintf("noAutofit active: %d paragraphs may overflow placeholder (smart autofit suppressed by template)", paraCount),
			Fix:     &patterns.FixSuggestion{Kind: "reduce_text"},
		},
		Action: "review",
	})
	return true
}

// applyZeroDimensionAutofit handles the case where the shape has no usable
// width/height extent. Falls back to a paragraph-count heuristic, applies a
// conservative 70% floor (and an absolute 10pt floor at 20pt base size), and
// emits an ErrCodePlaceholderOverflow finding for dense content.
func applyZeroDimensionAutofit(bp *bodyPropertiesXML, shape *shapeXML, cfg *autofitConfig) {
	// A typical body placeholder (~4.5 inches tall) fits ~14 lines at default
	// font size (20pt + 1.2× line spacing + spcBef). When there are more
	// paragraphs, apply proportional scaling to prevent overflow that
	// rendering engines (LibreOffice) may not auto-shrink well enough.
	// The floor is 70% (not 60%) because this heuristic can't account for
	// actual text width/wrapping — being overly aggressive makes dense
	// content like 4-group bullet layouts illegibly small.
	paraCount := len(shape.TextBody.Paragraphs)
	const typicalFitLines = 14
	if paraCount <= typicalFitLines {
		bp.Inner += `<a:normAutofit/>`
		return
	}
	scalePct := (typicalFitLines * 100) / paraCount
	// Enforce absolute 10pt floor for dimensionless estimate.
	// Assume 20pt base font (typical body level 1) when unknown.
	absFloorPct := int(textfit.AbsMinFontPt / 20.0 * 100) // = 50%
	if scalePct < 70 {
		scalePct = 70 // conservative floor for dimensionless estimate
	}
	if scalePct < absFloorPct {
		scalePct = absFloorPct
	}
	bp.Inner += fmt.Sprintf(`<a:normAutofit fontScale="%d"/>`, scalePct*1000)
	if cfg.findings == nil {
		return
	}
	*cfg.findings = append(*cfg.findings, patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    cfg.findingPath,
			Code:    patterns.ErrCodePlaceholderOverflow,
			Message: fmt.Sprintf("placeholder has no dimensions: %d paragraphs estimated via heuristic (fits ~%d), fontScale=%d%%", paraCount, typicalFitLines, scalePct),
			Fix:     &patterns.FixSuggestion{Kind: "reduce_text"},
		},
		Action: "review",
	})
}

// buildTextfitParams assembles the textfit.Params for the autofit calculation,
// including font lookup with theme fallback and master-spacing compensation.
func buildTextfitParams(shape *shapeXML, widthEMU, heightEMU int64, texts []string, cfg *autofitConfig) textfit.Params {
	fontSizeHPt := extractFontSizeFromShape(shape)
	if n := cfg.leadParagraphs; n > 0 && n < len(shape.TextBody.Paragraphs) {
		// Measure at the bullets' size, not the larger lead's: the lead is
		// one paragraph and normAutofit scales both uniformly.
		body := *shape.TextBody
		body.Paragraphs = body.Paragraphs[n:]
		rest := *shape
		rest.TextBody = &body
		fontSizeHPt = extractFontSizeFromShape(&rest)
	}
	fontName := extractFontNameFromShape(shape)
	if strings.HasPrefix(fontName, "+") {
		fontName = "" // theme token, not a family — resolve below
	}

	// Inherited (master) style: the shape's own lstStyle overrides the master.
	var style *template.InheritedTextStyle
	if cfg.inherited != nil {
		st := *cfg.inherited
		if shape.TextBody.ListStyle != nil {
			st = st.OverrideFromListStyle(shape.TextBody.ListStyle.Inner)
		}
		style = &st
		if fontName == "" && cfg.inheritedFontName != "" {
			fontName = cfg.inheritedFontName
		}
	}
	if fontName == "" && cfg.themeFontName != "" {
		fontName = cfg.themeFontName // Use theme font when shape has no explicit typeface
	}

	// When the font size is inherited from the slide master (not explicit in the shape),
	// the master's bodyStyle typically adds spcBef + spcAft (~10pt + 2pt = 12pt per paragraph).
	// Account for this extra spacing in the height estimate. When the inherited
	// style is known, use its real size and space-before instead.
	var extraSpacingPt float64
	if fontSizeHPt == 0 {
		if style != nil && style.SizeHPt > 0 {
			fontSizeHPt = style.SizeHPt
		}
		// A known style speaks for itself, including when it declares no
		// space-before. Only an unknown style falls back to the typical value.
		if style != nil {
			extraSpacingPt = style.SpcBefPt
		} else {
			extraSpacingPt = defaultParagraphSpacingPt
		}
	}
	// Normalising the runs makes their size explicit, but must not discard the
	// inherited paragraph spacing that still renders from the master.
	if cfg.bodyTypography && style != nil && extraSpacingPt == 0 {
		extraSpacingPt = style.SpcBefPt
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
	if style != nil {
		applyInheritedStyleToParams(&params, *style)
	}
	return params
}

// defaultParagraphSpacingPt is the per-paragraph space a slide master
// typically adds (spcBef + spcAft) when it declares none we can read.
const defaultParagraphSpacingPt = 12.0

// InheritedParagraphSpacingPt is the per-paragraph extra spacing a measured fit
// budgets: the master's declared space-before when there is one, else the
// typical value. Exported so the preflight predictor applies the same rule —
// the two sides disagreeing on this is what made validate predict a 70% font
// scale where the render applied 50% (go-slide-creator-nlrg).
func InheritedParagraphSpacingPt(declaredSpcBefPt float64) float64 {
	if declaredSpcBefPt > 0 {
		return declaredSpcBefPt
	}
	return defaultParagraphSpacingPt
}

// buildNormAutofitElement renders the OOXML <a:normAutofit/> element from a
// textfit result, omitting attributes that have a zero (default) value.
func buildNormAutofitElement(result textfit.FitResult) string {
	var attrs []string
	if result.FontScale > 0 {
		attrs = append(attrs, fmt.Sprintf(`fontScale="%d"`, result.FontScale))
	}
	if result.LnSpcReduction > 0 {
		attrs = append(attrs, fmt.Sprintf(`lnSpcReduction="%d"`, result.LnSpcReduction))
	}
	if len(attrs) == 0 {
		return `<a:normAutofit/>`
	}
	return fmt.Sprintf(`<a:normAutofit %s/>`, strings.Join(attrs, " "))
}

// trimmedFitParams returns params measuring the first keep paragraphs of texts
// followed by the "…" truncation indicator, which uses the base paragraph
// spacing and no extra left margin.
func trimmedFitParams(params textfit.Params, texts []string, keep int) textfit.Params {
	trimmedTexts := make([]string, keep+1)
	copy(trimmedTexts, texts[:keep])
	trimmedTexts[keep] = "\u2026" // ellipsis character

	var trimmedSpacings []float64
	if params.ExtraSpacingsPt != nil {
		trimmedSpacings = make([]float64, keep+1)
		copy(trimmedSpacings, params.ExtraSpacingsPt[:min(keep, len(params.ExtraSpacingsPt))])
		trimmedSpacings[keep] = params.ExtraSpacingPt // "…" uses base spacing
	}

	var trimmedMargins []float64
	if params.LeftMarginsPt != nil {
		trimmedMargins = make([]float64, keep+1)
		copy(trimmedMargins, params.LeftMarginsPt[:min(keep, len(params.LeftMarginsPt))])
		// "…" indicator has no extra margin
	}

	trimmed := params
	trimmed.Paragraphs = trimmedTexts
	trimmed.ExtraSpacingsPt = trimmedSpacings
	trimmed.LeftMarginsPt = trimmedMargins
	return trimmed
}

// largestFittingPrefix returns the largest keep in [minKeep, maxKeep] for
// which accept(Calculate(first keep paragraphs + "…")) holds, with that
// result. Keeping fewer paragraphs never makes the text harder to fit, so the
// predicate is monotone in keep and a binary search finds the same answer the
// old drop-one-paragraph-per-iteration loop did, in O(log n) measurements
// instead of O(n). The linear loop re-measured every remaining paragraph on
// each iteration, so a 2000-bullet placeholder took minutes
// (go-slide-creator-8hg02).
//
// ok is false when no prefix is accepted or the font cache is unavailable.
func largestFittingPrefix(params textfit.Params, texts []string, minKeep, maxKeep int, accept func(textfit.FitResult) bool) (keep int, result textfit.FitResult, ok bool) {
	lo, hi := minKeep, maxKeep
	for lo <= hi {
		mid := lo + (hi-lo)/2
		r, err := textfit.Calculate(trimmedFitParams(params, texts, mid))
		if err != nil {
			return 0, textfit.FitResult{}, false // font cache unavailable
		}
		if accept(r) {
			keep, result, ok = mid, r, true
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return keep, result, ok
}

// appendEllipsisParagraph truncates the shape to paras and appends the "…"
// indicator paragraph, reusing the first paragraph's run properties.
func appendEllipsisParagraph(shape *shapeXML, paras []paragraphXML) {
	shape.TextBody.Paragraphs = paras
	var rProps *runPropertiesXML
	if len(paras) > 0 && len(paras[0].Runs) > 0 {
		rProps = paras[0].Runs[0].RunProperties
	}
	shape.TextBody.Paragraphs = append(shape.TextBody.Paragraphs, paragraphXML{
		Properties: noBulletParagraphProps(""),
		Runs: []runXML{{
			RunProperties: rProps,
			Text:          "\u2026",
		}},
	})
}

// trimOverflowParagraphs removes trailing paragraphs from the shape until the
// content fits within the placeholder at minimum font scale. Adds a "..." indicator
// to show content was truncated. Returns the recalculated FitResult.
func trimOverflowParagraphs(shape *shapeXML, params textfit.Params, cfg *autofitConfig) textfit.FitResult {
	paras := shape.TextBody.Paragraphs

	// Need at least 2 paragraphs to trim (keep >= 2, drop >= 1, add "...").
	if len(paras) > 2 {
		texts := collectParagraphTexts(paras)
		keep, result, ok := largestFittingPrefix(params, texts, 2, len(paras)-1, func(r textfit.FitResult) bool {
			return !r.Overflow
		})
		if ok {
			// Content fits after trimming — update shape paragraphs
			paras = paras[:keep]
			appendEllipsisParagraph(shape, paras)

			slog.Info("trimmed overflow: removed paragraphs to fit placeholder",
				slog.Int("original", len(params.Paragraphs)),
				slog.Int("remaining", len(paras)+1)) // +1 for "..." indicator

			// Site 2: emit finding when paragraphs are trimmed to fit.
			if cfg.findings != nil {
				removed := len(params.Paragraphs) - len(paras)
				*cfg.findings = append(*cfg.findings, patterns.FitFinding{
					ValidationError: patterns.ValidationError{
						Path:    cfg.findingPath,
						Code:    patterns.ErrCodeTextTrimmed,
						Message: fmt.Sprintf("trimmed %d trailing paragraphs to fit placeholder (%d remaining including ellipsis); removed source content is absent from the deck, split it across slides", removed, len(paras)+1),
					},
					Action: "refuse",
				})
			}

			return result
		}
	}

	// Even 2 paragraphs don't fit — apply maximum scaling and accept overflow.
	// Enforce the absolute 10pt floor: never emit a fontScale that would
	// produce text smaller than 10pt.
	fontSizePt := float64(params.FontSizeHPt) / 100.0
	if fontSizePt <= 0 {
		fontSizePt = 20.0
	}
	floorScale := int(textfit.AbsMinFontPt / fontSizePt * 100)
	if floorScale > 100 {
		floorScale = 100
	}
	if floorScale < 50 {
		floorScale = 50 // never go below 50% in the hard-overflow path
	}

	slog.Warn("text overflow: content does not fit even after trimming",
		slog.Int("paragraphs", len(params.Paragraphs)))

	// Site 3: emit error when content overflows even at maximum scaling.
	if cfg.findings != nil {
		*cfg.findings = append(*cfg.findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    cfg.findingPath,
				Code:    patterns.ErrCodeTextOverflow,
				Message: fmt.Sprintf("text overflow: %d paragraphs do not fit even after trimming and maximum font scaling", len(params.Paragraphs)),
				Fix:     &patterns.FixSuggestion{Kind: "split_at_row"},
			},
			Action: "refuse",
		})
	}

	return textfit.FitResult{
		FontScale:      floorScale * 1000,
		LnSpcReduction: 20000,
		Overflow:       true,
	}
}

// trimForReadability removes trailing paragraphs from the shape until the font
// scale meets the desired minimum (e.g., 70000 = 70%). Unlike trimOverflowParagraphs
// which only runs on overflow, this proactively trims dense content to keep text
// at a presentation-legible size. Adds a "…" indicator when paragraphs are removed.
func trimForReadability(shape *shapeXML, params textfit.Params, targetMinFontScale int, cfg *autofitConfig) textfit.FitResult {
	paras := shape.TextBody.Paragraphs

	// Need at least 4 paragraphs to consider trimming for readability
	// (keep at least 4 visible, drop at least 1, add the "…" indicator).
	if len(paras) > 4 {
		texts := collectParagraphTexts(paras)
		keep, result, ok := largestFittingPrefix(params, texts, 4, len(paras)-1, func(r textfit.FitResult) bool {
			return !r.Overflow && (r.FontScale == 0 || r.FontScale >= targetMinFontScale)
		})
		if ok {
			// Content fits at an acceptable font scale — update shape
			paras = paras[:keep]
			appendEllipsisParagraph(shape, paras)

			slog.Info("trimmed for readability: removed paragraphs to improve font scale",
				slog.Int("original", len(params.Paragraphs)),
				slog.Int("remaining", len(paras)+1),
				slog.Int("fontScale", result.FontScale))

			// Site 4: emit hint when paragraphs trimmed for readability.
			if cfg.findings != nil {
				removed := len(params.Paragraphs) - len(paras)
				*cfg.findings = append(*cfg.findings, patterns.FitFinding{
					ValidationError: patterns.ValidationError{
						Path:    cfg.findingPath,
						Code:    patterns.ErrCodeReadabilityTrimmed,
						Message: fmt.Sprintf("trimmed %d paragraphs for readability (fontScale improved to %d%%); removed source content is absent from the deck, split it across slides", removed, result.FontScale/1000),
					},
					Action: "refuse",
				})
			}

			return result
		}
	}

	// Couldn't achieve target — return original calculation
	result, _ := textfit.Calculate(params)
	return result
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

var noAutofitRegexp = regexp.MustCompile(`(?i)<(?:a:)?noautofit\b[^>]*/>|<(?:a:)?noautofit\b[^>]*>.*?</(?:a:)?noautofit>`)

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
