// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// collectParagraphTexts extracts plain text from all paragraph runs.
func collectParagraphTexts(paragraphs []paragraphXML) []string {
	texts := make([]string, 0, len(paragraphs))
	for _, para := range paragraphs {
		var sb strings.Builder
		for _, run := range para.Runs {
			sb.WriteString(run.Text)
		}
		texts = append(texts, sb.String())
	}
	return texts
}

// spcPtsRegexp matches spcPts val="NNN" in OOXML attributes.
var spcPtsRegexp = regexp.MustCompile(`spcPts[^>]*val="(\d+)"`)

// szRegexp matches sz="NNN" in OOXML attributes.
var szRegexp = regexp.MustCompile(`sz="(\d+)"`)

// defRPrBoldRegexp matches b="1" or b="true" attributes inside <a:defRPr> or <defRPr> elements.
// Only matches bold-ON values; b="0" is intentionally preserved since it correctly
// overrides inherited bold from the slide master's bodyStyle.
var defRPrBoldRegexp = regexp.MustCompile(`(<(?:a:)?defRPr\b[^>]*?)\s+b="(?:1|true)"`)

// defRPrTagRegexp matches any <a:defRPr ...> or <defRPr ...> opening/self-closing tag.
// Used with ReplaceAllStringFunc to inject b="0" into tags missing any b= attribute.
var defRPrTagRegexp = regexp.MustCompile(`<(?:a:)?defRPr\b[^>]*/?>`)

// bAttrRegexp checks whether a matched defRPr tag already contains a b= attribute.
var bAttrRegexp = regexp.MustCompile(`\bb="`)

// defRPrNameRegexp finds the end of "defRPr" in a tag to know where to inject attributes.
var defRPrNameRegexp = regexp.MustCompile(`defRPr`)

// stripDefRPrBold replaces b="1" with b="0" in defRPr elements within an XML fragment,
// and also injects b="0" into defRPr elements that don't have any b= attribute.
// This prevents text from inheriting bold from template lstStyle or paragraph defaults,
// ensuring that only explicit inline <b>bold</b> tags produce bold text.
// Using b="0" (not removal) preserves the attribute so downstream renderers don't
// fall back to master bodyStyle b="1" inheritance.
//
// Two cases are handled:
//   - defRPr with b="1" -> replaced to b="0" (explicit bold in template)
//   - defRPr without any b= -> inject b="0" (bold inherited from master bodyStyle)
func stripDefRPrBold(xml string) string {
	if xml == "" {
		return xml
	}
	// Step 1: Replace explicit b="1" or b="true" with b="0"
	result := defRPrBoldRegexp.ReplaceAllString(xml, `${1} b="0"`)
	// Step 2: Inject b="0" into defRPr elements that don't have any b= attribute,
	// preventing bold inheritance from the master bodyStyle.
	result = defRPrTagRegexp.ReplaceAllStringFunc(result, func(tag string) string {
		if bAttrRegexp.MatchString(tag) {
			return tag // already has b= attribute, leave as-is
		}
		loc := defRPrNameRegexp.FindStringIndex(tag)
		if loc == nil {
			return tag
		}
		insertPos := loc[1] // right after "defRPr"
		return tag[:insertPos] + ` b="0"` + tag[insertPos:]
	})
	return result
}

// defRPrCapsRegexp matches cap="all" or cap="small" attributes inside <a:defRPr>
// or <defRPr> elements. These cause text to render in all-caps or small-caps,
// which is undesirable for body content populated by the generator.
var defRPrCapsRegexp = regexp.MustCompile(`(<(?:a:)?defRPr\b[^>]*?)\s+cap="(?:all|small)"`)

// rPrCapsRegexp matches cap="all" or cap="small" in any <a:rPr> or <rPr> element
// within inner XML. Used to strip caps from lstStyle level run properties.
var rPrCapsRegexp = regexp.MustCompile(`(<(?:a:)?rPr\b[^>]*?)\s+cap="(?:all|small)"`)

// stripDefRPrCaps removes cap="all" and cap="small" attributes from defRPr
// and rPr elements within an XML fragment. This prevents body text from
// inheriting all-caps formatting from the template's lstStyle or slide master.
// Some templates define cap="all" in two-column body placeholders for decorative
// headers, which makes all populated body text render as ALL CAPS.
func stripDefRPrCaps(xml string) string {
	if xml == "" {
		return xml
	}
	result := defRPrCapsRegexp.ReplaceAllString(xml, `${1}`)
	result = rPrCapsRegexp.ReplaceAllString(result, `${1}`)
	return result
}

// stripLstStyleCaps removes cap="all" and cap="small" from defRPr and rPr
// elements in the shape's lstStyle. This prevents body text from inheriting
// all-caps formatting from the template's lstStyle definitions.
func stripLstStyleCaps(shape *shapeXML) {
	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return
	}
	lstInner := shape.TextBody.ListStyle.Inner
	if lstInner == "" {
		return
	}
	stripped := stripDefRPrCaps(lstInner)
	if stripped != lstInner {
		shape.TextBody.ListStyle.Inner = stripped
	}
}

// lvlMarLRegexp matches lvlNpPr marL="NNN" in lstStyle XML. Captures the level
// number (1-9) and the marL value in EMU.
var lvlMarLRegexp = regexp.MustCompile(`lvl(\d)pPr[^>]*marL="(\d+)"`)

// extractLevelMargins parses marL values for each level from lstStyle inner XML.
// Returns a map from paragraph level (0-based) to left margin in points.
// In OOXML lstStyle, lvl1pPr corresponds to paragraph level 0, lvl2pPr to level 1, etc.
func extractLevelMargins(lstStyleInner string) map[int]float64 {
	if lstStyleInner == "" {
		return nil
	}
	const emuPerPt = 12700
	margins := make(map[int]float64)
	matches := lvlMarLRegexp.FindAllStringSubmatch(lstStyleInner, -1)
	for _, m := range matches {
		lvlNum, _ := strconv.Atoi(m[1])
		marL, _ := strconv.ParseInt(m[2], 10, 64)
		// lvl1pPr = paragraph level 0, lvl2pPr = level 1, etc.
		margins[lvlNum-1] = float64(marL) / float64(emuPerPt)
	}
	return margins
}

// extractParagraphLeftMargins builds a per-paragraph left margin array from the
// paragraph levels and the shape's lstStyle. This accounts for bullet indentation
// (marL) that reduces the available width for text wrapping.
func extractParagraphLeftMargins(paragraphs []paragraphXML, lstStyle *listStyleXML) []float64 {
	if lstStyle == nil || lstStyle.Inner == "" {
		return nil
	}
	levelMargins := extractLevelMargins(lstStyle.Inner)
	if len(levelMargins) == 0 {
		return nil
	}

	margins := make([]float64, len(paragraphs))
	hasNonZero := false
	for i, para := range paragraphs {
		lvl := 0
		if para.Properties != nil && para.Properties.Level != nil {
			lvl = *para.Properties.Level
		}
		if m, ok := levelMargins[lvl]; ok {
			margins[i] = m
			if m > 0 {
				hasNonZero = true
			}
		}
	}
	if !hasNonZero {
		return nil
	}
	return margins
}

// extractFontSizeFromShape extracts font size in hundredths of a point from shape.
// Checks run properties first, then paragraph default run properties, then lstStyle.
// Returns 0 if no font size is found (textfit will use its default).
func extractFontSizeFromShape(shape *shapeXML) int {
	if shape.TextBody == nil {
		return 0
	}
	// Check first paragraph's run properties
	for _, para := range shape.TextBody.Paragraphs {
		for _, run := range para.Runs {
			if run.RunProperties != nil {
				if sz := parseSzAttr(run.RunProperties.Inner); sz > 0 {
					return sz
				}
			}
		}
		// Check paragraph's default run properties (defRPr) in inner XML
		if para.Properties != nil {
			if sz := parseSzAttr(para.Properties.Inner); sz > 0 {
				return sz
			}
		}
	}
	// Fallback: check lstStyle for inherited font size (e.g. section divider body placeholder)
	if shape.TextBody.ListStyle != nil {
		if sz := parseSzAttr(shape.TextBody.ListStyle.Inner); sz > 0 {
			return sz
		}
	}
	return 0
}

// extractFontNameFromShape extracts font family name from shape run properties.
// Returns empty string if not found (textfit will use its default).
func extractFontNameFromShape(shape *shapeXML) string {
	if shape.TextBody == nil {
		return ""
	}
	for _, para := range shape.TextBody.Paragraphs {
		for _, run := range para.Runs {
			if run.RunProperties != nil {
				if name := parseTypefaceAttr(run.RunProperties.Inner); name != "" {
					return name
				}
			}
		}
	}
	return ""
}

// typefaceRegexp matches typeface="FontName" in OOXML attributes.
var typefaceRegexp = regexp.MustCompile(`typeface="([^"]+)"`)

// parseSzAttr extracts the sz attribute value from an XML fragment.
func parseSzAttr(xml string) int {
	m := szRegexp.FindStringSubmatch(xml)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return v
}

// parseTypefaceAttr extracts the typeface attribute value from an XML fragment.
// Skips theme font references (names starting with "+").
func parseTypefaceAttr(xml string) string {
	m := typefaceRegexp.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	name := m[1]
	// Skip theme font references like "+mj-lt", "+mn-lt"
	if strings.HasPrefix(name, "+") {
		return ""
	}
	return name
}

// parseSpcBefPt extracts the spcBef value in points from paragraph properties XML.
// Returns 0 if no spcBef is found. The OOXML spcPts val is in hundredths of a point.
func parseSpcBefPt(xml string) float64 {
	idx := strings.Index(xml, "spcBef")
	if idx < 0 {
		return 0
	}
	// Search for spcPts after spcBef
	rest := xml[idx:]
	m := spcPtsRegexp.FindStringSubmatch(rest)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return float64(v) / 100.0
}

// extractParagraphSpacings computes per-paragraph extra spacing by checking for
// explicit spcBef values in paragraph properties. Returns nil if no paragraphs
// have explicit spacing (callers should use the uniform ExtraSpacingPt instead).
func extractParagraphSpacings(paragraphs []paragraphXML, baseSpacing float64) []float64 {
	spacings := make([]float64, len(paragraphs))
	explicit := make([]bool, len(paragraphs))
	hasExplicit := false
	for i, para := range paragraphs {
		if para.Properties != nil {
			if spcBef := parseSpcBefPt(para.Properties.Inner); spcBef > 0 {
				spacings[i] = spcBef
				explicit[i] = true
				hasExplicit = true
				continue
			}
		}
		spacings[i] = baseSpacing
	}
	if !hasExplicit {
		return nil
	}
	// When some paragraphs have explicit spcBef (e.g. bullet group headers),
	// non-explicit paragraphs (bullets, body text) inherit spacing from the
	// slide master which normAutofit already handles. Using the full baseSpacing
	// (12pt) for these over-estimates height and causes false overflow trimming.
	for i := range spacings {
		if !explicit[i] {
			spacings[i] = 0
		}
	}
	return spacings
}

// maxFontForWordFit returns the maximum font size (in hundredths of a point) where
// every word in text fits on one line within widthEMU. Uses a conservative character-
// based estimate (avgCharWidth ≈ 0.55 × em) instead of font metric measurement,
// because the tdewolff/canvas library underestimates rendered glyph widths by ~2x
// compared to LibreOffice/PowerPoint. Returns 0 if estimation is not possible.
func maxFontForWordFit(text string, widthEMU int64) int {
	if text == "" || widthEMU <= 0 {
		return 0
	}

	// Usable width in points after default OOXML margins (7.2pt each side)
	const emuPerPt = 12700
	widthPt := float64(widthEMU) / emuPerPt
	usablePt := widthPt - 2*7.2
	if usablePt <= 0 {
		return 0
	}

	// Find longest word (character count)
	maxChars := 0
	for _, word := range strings.Fields(text) {
		if len([]rune(word)) > maxChars {
			maxChars = len([]rune(word))
		}
	}
	if maxChars == 0 {
		return 0
	}

	// Average advance width for a sans-serif character ≈ 0.55 × em.
	// This is conservative (covers caps, wide chars like 'm', 'w').
	// maxFont = usablePt / (maxChars × 0.55)
	const avgCharRatio = 0.55
	maxFontPt := usablePt / (float64(maxChars) * avgCharRatio)

	// Apply 10% additional margin for kerning, ligatures, and font variation
	maxFontPt *= 0.90

	maxFontHPt := int(maxFontPt * 100)
	if maxFontHPt < 1200 {
		maxFontHPt = 1200 // 12pt floor
	}
	return maxFontHPt
}

// maxTitleLines is the maximum number of wrapped lines allowed for title placeholders.
// Titles exceeding this limit are truncated with an ellipsis to prevent them from
// crowding body text in the placeholder below.
const maxTitleLines = 3

// isTitlePlaceholder returns true if the placeholder ID indicates a regular
// slide title (not subtitle, section title, or title slide ctrTitle — those
// have separate code paths).
func isTitlePlaceholder(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	return lower == "title" || strings.HasPrefix(lower, "title_")
}

// estimateWordWrapLines estimates how many lines text will occupy when
// word-wrapped within a line that can hold charsPerLine characters.
func estimateWordWrapLines(text string, charsPerLine int) int {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 1
	}

	lines := 1
	currentChars := 0
	for i, word := range words {
		wordLen := len([]rune(word))
		if i > 0 {
			if currentChars+1+wordLen > charsPerLine {
				lines++
				currentChars = wordLen
			} else {
				currentChars += 1 + wordLen
			}
		} else {
			currentChars = wordLen
			// Single word wider than the line wraps across multiple lines
			if wordLen > charsPerLine {
				lines = (wordLen + charsPerLine - 1) / charsPerLine
				currentChars = wordLen % charsPerLine
				if currentChars == 0 {
					currentChars = charsPerLine
				}
			}
		}
	}
	return lines
}

// truncateTextToMaxLines truncates text to fit within maxLines at the given
// font size and placeholder width, appending an ellipsis if truncation occurs.
// Uses textfit.MeasureRun for font-metric-aware measurement when fontName is
// available; falls back to the legacy avgCharRatio heuristic when font metrics
// cannot be resolved (e.g. missing font cache).
// Returns the original text unchanged if it already fits.
func truncateTextToMaxLines(text string, widthEMU int64, fontSizeHPt int, maxLines int, fontName string) string {
	if text == "" || widthEMU <= 0 || fontSizeHPt <= 0 || maxLines <= 0 {
		return text
	}

	fontSizePt := float64(fontSizeHPt) / 100.0

	// Try font-metric-aware measurement via textfit.MeasureRun.
	m, err := textfit.MeasureRun(text, fontName, fontSizePt, widthEMU, maxLines)
	if err == nil {
		if m.Fits {
			return text
		}
		// Text overflows maxLines — truncate word by word using MeasureRun.
		words := strings.Fields(text)
		for len(words) > 1 {
			words = words[:len(words)-1]
			candidate := strings.Join(words, " ") + " \u2026"
			cm, cerr := textfit.MeasureRun(candidate, fontName, fontSizePt, widthEMU, maxLines)
			if cerr != nil {
				break // fall through to heuristic
			}
			if cm.Fits {
				return candidate
			}
		}
		// Even one word + ellipsis doesn't fit
		if len(words) == 1 {
			return words[0] + " \u2026"
		}
		return text
	}

	// Fallback: legacy character-based heuristic (no font cache available).
	const emuPerPt = 12700
	widthPt := float64(widthEMU) / float64(emuPerPt)
	usablePt := widthPt - 2*7.2 // default OOXML margins
	if usablePt <= 0 {
		return text
	}

	const avgCharRatio = 0.55
	charsPerLine := int(usablePt / (fontSizePt * avgCharRatio))
	if charsPerLine < 1 {
		charsPerLine = 1
	}

	if estimateWordWrapLines(text, charsPerLine) <= maxLines {
		return text
	}

	words := strings.Fields(text)
	for len(words) > 1 {
		words = words[:len(words)-1]
		candidate := strings.Join(words, " ") + " \u2026"
		if estimateWordWrapLines(candidate, charsPerLine) <= maxLines {
			return candidate
		}
	}

	if len(words) == 1 {
		return words[0] + " \u2026"
	}
	return text
}

// capLstStyleFontSize reduces the font size in lstStyle if it exceeds maxSizeHPt
// (hundredths of a point). This prevents excessively large inherited fonts from
// causing text overflow (e.g. section divider body placeholder with 96pt lstStyle).
func capLstStyleFontSize(shape *shapeXML, maxSizeHPt int) {
	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return
	}
	lstInner := shape.TextBody.ListStyle.Inner
	if lstInner == "" {
		return
	}
	sz := parseSzAttr(lstInner)
	if sz <= maxSizeHPt {
		return
	}
	// Replace the font size in lstStyle with the capped value
	replacement := fmt.Sprintf(`sz="%d"`, maxSizeHPt)
	shape.TextBody.ListStyle.Inner = szRegexp.ReplaceAllString(lstInner, replacement)
}

// minSectionTitleFontForHeight returns the minimum font size (in hundredths of a
// point) that a section divider title should have given the placeholder height.
// Section titles are meant to be visually prominent; a tiny font in a large
// placeholder looks out of place. The calculation targets 2 lines of text at
// 1.2× line spacing filling ~50% of the placeholder height, clamped to [3200, 5400]
// (32pt–54pt). normAutofit handles overflow if the text is longer.
func minSectionTitleFontForHeight(heightEMU int64) int {
	if heightEMU <= 0 {
		return 3200 // 32pt default floor
	}
	const emuPerPt = 12700
	heightPt := float64(heightEMU) / emuPerPt
	// Target: 2 lines × 1.2 line-spacing fills 50% of height → font = height×0.5/(2×1.2)
	fontPt := heightPt * 0.5 / 2.4
	fontHPt := int(fontPt * 100)
	if fontHPt < 3200 {
		fontHPt = 3200 // 32pt hard floor
	}
	if fontHPt > 5400 {
		fontHPt = 5400 // 54pt hard ceiling
	}
	return fontHPt
}

// boostSectionTitleFont ensures a section divider title font is large enough to
// look visually prominent. If the lstStyle font is below the height-based minimum,
// it is raised. If the lstStyle has no font size at all, a defRPr with the minimum
// size is injected so the title doesn't rely on the (potentially small) master style.
func boostSectionTitleFont(shape *shapeXML) {
	_, heightEMU := getShapeDimensions(shape)
	minFont := minSectionTitleFontForHeight(heightEMU)

	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return
	}
	lstInner := shape.TextBody.ListStyle.Inner
	currentSz := parseSzAttr(lstInner)
	if currentSz >= minFont {
		return // already large enough
	}
	if currentSz > 0 {
		// lstStyle has a font size — raise it
		replacement := fmt.Sprintf(`sz="%d"`, minFont)
		shape.TextBody.ListStyle.Inner = szRegexp.ReplaceAllString(lstInner, replacement)
	} else if lstInner == "" || !strings.Contains(lstInner, "defRPr") {
		// No font in lstStyle — inject a lvl1pPr with defRPr
		shape.TextBody.ListStyle.Inner = fmt.Sprintf(
			`<a:lvl1pPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:defRPr sz="%d"/></a:lvl1pPr>`,
			minFont)
	}
}

// floorLstStyleFontSize raises the font size in lstStyle if it is below minSizeHPt
// (hundredths of a point). This prevents tiny inherited fonts from producing
// unreadable text (e.g. templates with 6-8pt inherited body font).
func floorLstStyleFontSize(shape *shapeXML, minSizeHPt int) {
	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return
	}
	lstInner := shape.TextBody.ListStyle.Inner
	if lstInner == "" {
		return
	}
	sz := parseSzAttr(lstInner)
	if sz <= 0 || sz >= minSizeHPt {
		return
	}
	replacement := fmt.Sprintf(`sz="%d"`, minSizeHPt)
	shape.TextBody.ListStyle.Inner = szRegexp.ReplaceAllString(lstInner, replacement)
}

// stripLstStyleBold removes bold attributes from defRPr elements in the shape's lstStyle.
// This prevents body text from inheriting bold from the template's lstStyle definitions.
// Some templates may define b="1" in lstStyle defRPr which makes all text at that
// level bold by default. Stripping it ensures only explicit inline <b>bold</b> produces bold.
func stripLstStyleBold(shape *shapeXML) {
	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return
	}
	lstInner := shape.TextBody.ListStyle.Inner
	if lstInner == "" {
		return
	}
	stripped := stripDefRPrBold(lstInner)
	if stripped != lstInner {
		shape.TextBody.ListStyle.Inner = stripped
	}
}
