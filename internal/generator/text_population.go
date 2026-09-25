// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// populateShapeText sets text content in a shape based on the content item type.
// Returns an error if the content value is invalid for the specified type.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
// themeFontName is the template's theme font used for text fitting measurements
// (e.g., "Franklin Gothic Book"). Pass "" to fall back to built-in metrics.
func populateShapeText(shape *shapeXML, item ContentItem, masterBulletLevel int, themeFontName string, autofitOpts ...autofitOption) error {
	if shape.TextBody == nil {
		shape.TextBody = &textBodyXML{
			BodyProperties: &bodyPropertiesXML{},
			ListStyle:      &listStyleXML{},
			Paragraphs:     []paragraphXML{emptyParagraph()},
		}
	} else {
		// Ensure required elements exist even if TextBody was already set
		if shape.TextBody.BodyProperties == nil {
			shape.TextBody.BodyProperties = &bodyPropertiesXML{}
		}
		if shape.TextBody.ListStyle == nil {
			shape.TextBody.ListStyle = &listStyleXML{}
		}
	}

	var err error
	switch item.Type {
	case ContentText:
		if isTitleShape(shape) || isTitlePlaceholder(item.PlaceholderID) {
			autofitOpts = append(autofitOpts, withTitleRole())
		}
		err = setTextParagraph(shape, item.PlaceholderID, item.Value, 2400, themeFontName, autofitOpts...) // 24pt cap (only if layout has no explicit sz)
	case ContentSectionTitle:
		if isTitleShape(shape) || isTitlePlaceholder(item.PlaceholderID) {
			autofitOpts = append(autofitOpts, withSectionTitleRole())
		}
		err = setTextParagraph(shape, item.PlaceholderID, item.Value, 0, themeFontName, autofitOpts...) // no ordinary-text cap; divider titles use the measured 28pt floor
	case ContentTitleSlideTitle:
		err = setTitleSlideTitle(shape, item.PlaceholderID, item.Value, themeFontName, autofitOpts...) // preserve template styling, add overflow protection
	case ContentBullets:
		err = setBulletParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel, autofitOpts...)
	case ContentBodyAndBullets:
		err = setBodyAndBulletsParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel, autofitOpts...)
	case ContentBodyAndLead:
		err = setBodyAndLeadParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel, autofitOpts...)
	case ContentBulletGroups:
		err = setBulletGroupsParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel, autofitOpts...)
	default:
		return fmt.Errorf("unsupported content type %s for placeholder %s", item.Type, item.PlaceholderID)
	}
	if err != nil {
		return err
	}

	// Apply font size override if specified.
	if item.FontSize > 0 {
		applyFontSizeOverride(shape, item.FontSize)
	}
	if item.linkMarker != "" {
		for i := range shape.TextBody.Paragraphs {
			for j := range shape.TextBody.Paragraphs[i].Runs {
				run := &shape.TextBody.Paragraphs[i].Runs[j]
				if run.RunProperties == nil {
					run.RunProperties = &runPropertiesXML{Lang: "en-US"}
				}
				click := `<a:hlinkClick r:id="` + item.linkMarker + `"`
				if item.Link != nil && item.Link.Slide > 0 {
					click += ` action="ppaction://hlinksldjump"`
				}
				run.RunProperties.Inner += click + `/>`
			}
		}
	}

	return nil
}

// applyFontSizeOverride sets the sz attribute on all runs in the shape's text body
// and overrides lstStyle font sizes. fontSizeHPt is in hundredths of a point (e.g., 7200 = 72pt).
func applyFontSizeOverride(shape *shapeXML, fontSizeHPt int) {
	if shape.TextBody == nil {
		return
	}
	sz := fmt.Sprintf("%d", fontSizeHPt)
	for i := range shape.TextBody.Paragraphs {
		for j := range shape.TextBody.Paragraphs[i].Runs {
			r := &shape.TextBody.Paragraphs[i].Runs[j]
			if r.RunProperties == nil {
				r.RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			r.RunProperties.FontSize = sz
		}
	}
	// Also override lstStyle so inherited sizes don't compete.
	if shape.TextBody.ListStyle != nil && shape.TextBody.ListStyle.Inner != "" {
		replacement := fmt.Sprintf(`sz="%d"`, fontSizeHPt)
		shape.TextBody.ListStyle.Inner = szRegexp.ReplaceAllString(shape.TextBody.ListStyle.Inner, replacement)
	}
}

// setTextParagraph sets text paragraph(s) in the shape.
// It preserves paragraph and run properties from the layout template.
// Supports inline tag formatting: <b>bold</b>, <i>italic</i>, <u>underline</u>.
// If text contains "\n" (from blank-line-separated paragraphs),
// each segment is rendered as a separate OOXML paragraph.
//
// maxFontSizeHPt is the maximum allowed lstStyle font size in hundredths of a point.
// Pass 0 to skip the ordinary-text cap (section titles have their own fit policy).
// themeFontName is the template's theme font for text fitting measurements.
func setTextParagraph(shape *shapeXML, placeholderID string, value interface{}, maxFontSizeHPt int, themeFontName string, autofitOpts ...autofitOption) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid text value for placeholder %s", placeholderID)
	}
	if text == "" {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}

	// Extract template styling from existing paragraphs
	templatePProps, templateRProps := extractTemplateTextStyle(shape.TextBody.Paragraphs)

	// For body-only text (ContentText), ensure no bullet marker is shown.
	// Programmatic templates define bullet chars (buChar/buFont) in the body
	// placeholder's lstStyle which get inherited even at level 0. We must add
	// explicit buNone to suppress the bullet, similar to setBulletGroupsParagraphs.
	textPProps := suppressBulletInParagraphProps(templatePProps)

	// Section titles (maxFontSizeHPt=0): don't force left alignment in the inline
	// paragraph properties. Let the lstStyle alignment prevail so templates like
	// some templates can center section divider titles via lstStyle algn="ctr".
	// Other templates that don't specify algn default to left per OOXML spec.
	if maxFontSizeHPt == 0 {
		textPProps.Algn = ""
	}

	// Strip inherited bold from paragraph defRPr so only inline <b>bold</b> renders bold.
	// Some templates may define b="1" in defRPr which makes all body text bold.
	if textPProps != nil && textPProps.Inner != "" {
		textPProps.Inner = stripDefRPrBold(textPProps.Inner)
		textPProps.Inner = stripDefRPrCaps(textPProps.Inner)
	}

	// Create paragraph(s) with preserved template styling.
	// Body text may contain multiple paragraphs separated by "\n".
	var paras []paragraphXML
	for _, seg := range strings.Split(text, "\n") {
		if seg == "" {
			continue
		}
		runs := createFormattedRuns(seg, templateRProps)
		paras = append(paras, paragraphXML{
			Properties: textPProps,
			Runs:       runs,
		})
	}
	if len(paras) == 0 {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}
	shape.TextBody.Paragraphs = paras

	// Strip inherited bold and all-caps from lstStyle so only inline tag formatting applies.
	stripLstStyleBold(shape)
	stripLstStyleCaps(shape)

	// Cap ordinary text only when the layout doesn't explicitly declare a size.
	if maxFontSizeHPt > 0 {
		capLstStyleFontSizeIfUnset(shape, maxFontSizeHPt)
	}
	floorLstStyleFontSize(shape, 1200) // 12pt min

	if maxFontSizeHPt == 0 {
		fitSectionTitle(shape, text, isTitleShape(shape) || isTitlePlaceholder(placeholderID))
	}

	// Replace spAutoFit (grow-box-to-fit-text) with nothing so normAutofit can be applied.
	// Decorative placeholders (e.g. section header layouts with
	// 350pt font) use spAutoFit which causes text to overflow below the slide boundary.
	replaceSpAutoFitWithNorm(shape)

	// Enforce text wrapping within placeholder bounds (prevents overflow into adjacent columns)
	enforceTextWrap(shape)

	// Enable autofit if text overflows (e.g. title routed to large-font body placeholder)
	opts := append([]autofitOption{withThemeFont(themeFontName)}, autofitOpts...)
	if isTitlePlaceholder(placeholderID) {
		// A content-layout title is authored text, not the template's short
		// prompt. Some templates explicitly set noAutofit on that prompt; keeping
		// it lets a long title spill into the pattern even when the boxes do not
		// overlap. Match the title-slide path's overflow protection.
		opts = append(opts, withNoAutofitOverride())
	}
	applySmartAutofitWithOptions(shape, opts...)

	return nil
}

// fitSectionTitle establishes prominence before applying the final width cap.
// Keeping this separate also makes the section-specific sizing order explicit:
// a later prominence boost must never undo the safe fit calculation.
func fitSectionTitle(shape *shapeXML, text string, isTitle bool) {
	boostSectionTitleFont(shape)

	widthEMU, _ := getShapeDimensions(shape)
	fontSizeHPt := extractFontSizeFromShape(shape)
	if widthEMU <= 0 || fontSizeHPt <= 0 {
		return
	}

	safeMax := maxFontForWordFit(text, widthEMU)
	// The shipped divider layouts reserve the other column for a large section
	// number. When the full heading fits at the 32pt readability floor, keep it
	// on one line so LibreOffice does not shift that adjacent number off-slide.
	if singleLine := maxFontForSingleLineFit(text, widthEMU); singleLine >= 3200 && singleLine < safeMax {
		safeMax = singleLine
	}
	if isTitle && safeMax < SectionTitleMinHPt {
		safeMax = SectionTitleMinHPt
	}
	if safeMax <= 0 || fontSizeHPt <= safeMax {
		return
	}

	slog.Info("capping section title font to fit placeholder width",
		slog.Int("original_hpt", fontSizeHPt),
		slog.Int("capped_hpt", safeMax),
		slog.Int64("width_emu", widthEMU))
	capLstStyleFontSize(shape, safeMax)
}

// setTitleSlideTitle replaces the text in a title slide's title/ctrTitle or
// subtitle placeholder while preserving the template's font size, alignment, and
// bold styling. Unlike setTextParagraph (used for body text), this function does NOT:
//   - Cap the font size (ctrTitle typically uses 40-60pt by design)
//   - Force left alignment (ctrTitle/subtitle use the template's alignment)
//   - Strip bold/caps from lstStyle (template styling is intentional)
//
// It DOES apply overflow protection so a long title or subtitle scales (and, for
// the subtitle role, truncates) to stay within its placeholder band instead of
// overflowing and overlapping the adjacent title/subtitle:
//   - normAutofit shrink-to-fit, which is non-destructive when the text already
//     fits (fontScale stays at 100%), so the template's large title font is
//     preserved for normal-length titles.
//   - For the subtitle role (a non-title placeholder), wrap-aware truncation to
//     maxSubtitleLines, since subtitle bands are short.
//
// Run-property font sizes are preserved so the autofit pass measures the same
// size that is rendered (see go-slide-creator-waic); extractFontSizeFromShape
// falls back to the lstStyle size when the run carries no explicit sz.
//
// Supports inline tag formatting: <b>bold</b>, <i>italic</i>, <u>underline</u>.
func setTitleSlideTitle(shape *shapeXML, placeholderID string, value interface{}, themeFontName string, autofitOpts ...autofitOption) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid text value for placeholder %s", placeholderID)
	}
	if text == "" {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}

	// Extract template styling from existing paragraphs
	templatePProps, templateRProps := extractTemplateTextStyle(shape.TextBody.Paragraphs)

	// Subtitle role: the placeholder is not the title/ctrTitle. Subtitle bands are
	// short (~1 line tall), so truncate a long subtitle to a small line budget at
	// its own font size so it does not push past its band and collide with the
	// title. The title/ctrTitle is left untruncated; its large font is instead
	// scaled to fit by the autofit pass below.
	if !isTitlePlaceholder(placeholderID) {
		widthEMU, _ := getShapeDimensions(shape)
		fontSizeHPt := extractFontSizeFromShape(shape)
		if fontSizeHPt == 0 {
			fontSizeHPt = 2400 // 24pt typical subtitle default
		}
		truncated := truncateTextToMaxLines(text, widthEMU, fontSizeHPt, maxSubtitleLines, themeFontName)
		if truncated != text {
			slog.Info("truncated long title-slide subtitle to fit max lines",
				slog.String("placeholder", placeholderID),
				slog.Int("max_lines", maxSubtitleLines),
				slog.Int("original_runes", len([]rune(text))),
				slog.Int("truncated_runes", len([]rune(truncated))))
			text = truncated
		}
	}

	// Preserve the template's paragraph properties (alignment, spacing, etc.)
	// without suppressing bullets or forcing left alignment.
	// ctrTitle placeholders don't have bullets, so buNone is unnecessary.
	var paras []paragraphXML
	for _, seg := range strings.Split(text, "\n") {
		if seg == "" {
			continue
		}
		runs := createFormattedRuns(seg, templateRProps)
		paras = append(paras, paragraphXML{
			Properties: templatePProps,
			Runs:       runs,
		})
	}
	if len(paras) == 0 {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}
	shape.TextBody.Paragraphs = paras

	// Do NOT strip bold, caps, or cap font size — preserve template styling.
	//
	// DO add overflow protection: strip grow-to-fit (spAutoFit), enforce wrapping,
	// and apply smart autofit so a long title or subtitle scales down to fit its
	// band instead of overflowing into the adjacent placeholder. normAutofit is
	// non-destructive when the text already fits, and applySmartAutofit respects an
	// explicit template <a:noAutofit/> directive.
	replaceSpAutoFitWithNorm(shape)
	enforceTextWrap(shape)
	opts := append([]autofitOption{withThemeFont(themeFontName)}, autofitOpts...)
	if isTitlePlaceholder(placeholderID) {
		// Closing layouts commonly declare noAutofit for their short prompt text.
		// Authored statements can wrap above a bottom-anchored title box and cross
		// the template's top band, so populated display titles always get measured
		// overflow protection. A short title measures at 100% and stays unchanged.
		opts = append(opts, withNoAutofitOverride())
	}
	applySmartAutofitWithOptions(shape, opts...)

	return nil
}

// maxBulletNestingLevel is the deepest OOXML level an authored indent maps to.
// The slide master defines nine, but past the fourth the glyphs and sizes stop
// being distinguishable on a projected slide, and BULLET_NESTING_DEEP has
// already told the author that two is the readable limit.
const maxBulletNestingLevel = 4

// bulletNestingLevels reads each bullet's leading-whitespace depth and returns
// the depths alongside the bullet text with that whitespace removed.
func bulletNestingLevels(bullets []string) ([]int, []string) {
	depths := make([]int, len(bullets))
	stripped := make([]string, len(bullets))
	for i, b := range bullets {
		depths[i], stripped[i] = pptx.BulletIndentDepth(b)
	}
	return depths, stripped
}

// bulletParagraphLevel maps an authored indent depth onto an OOXML paragraph
// level, starting from the first bullet-enabled level of the layout.
func bulletParagraphLevel(baseLevel, depth int) int {
	level := baseLevel + depth
	if level > maxBulletNestingLevel {
		return maxBulletNestingLevel
	}
	if level < 0 {
		return 0
	}
	return level
}

// setBulletParagraphs sets multiple bullet paragraphs in the shape.
// It preserves paragraph and run properties from the layout template.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBulletParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int, autofitOpts ...autofitOption) error {
	bullets, ok := value.([]string)
	if !ok {
		return fmt.Errorf("invalid bullets value for placeholder %s", placeholderID)
	}
	if len(bullets) == 0 {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}

	// Extract template styling from existing paragraphs (one per level)
	templateStyles := extractBulletTemplateStyles(shape.TextBody.Paragraphs)

	// Determine the bullet level to use:
	// 1. If masterBulletLevel is valid (>=0), use it (from slide master)
	// 2. Otherwise try to detect from layout template styles
	// 3. Fall back to level 0
	bulletLevel := masterBulletLevel
	if bulletLevel < 0 {
		bulletLevel = findFirstBulletLevel(templateStyles)
	}
	if bulletLevel < 0 {
		bulletLevel = 0
	}

	// Leading whitespace is the author asking for a sub-bullet. Read the depth
	// and strip the whitespace BEFORE anything else looks at the text, so the
	// numbered-list detector sees "1. Step" rather than "\t1. Step"
	// (go-slide-creator-gyfl).
	depths, stripped := bulletNestingLevels(bullets)

	// An ordered list the author typed as "1. …", "2. …" is rendered with OOXML
	// auto-numbering and the typed prefixes removed, so it shows one marker
	// rather than the layout's glyph beside the author's number
	// (go-slide-creator-6or2).
	texts, numbered := NumberedList(stripped)
	if !numbered {
		texts = stripped
	}

	paragraphs := make([]paragraphXML, len(texts))
	for i, bullet := range texts {
		// Each bullet renders at the level its indent asks for, so a nested
		// bullet picks up the master's smaller size and secondary glyph
		// instead of the parent's.
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletParagraphLevel(bulletLevel, depths[i]))
		if numbered {
			applyAutoNumbering(pProps)
		}

		// Parse inline tag formatting and create runs
		runs := createFormattedRuns(bullet, rProps)

		paragraphs[i] = paragraphXML{
			Properties: pProps,
			Runs:       runs,
		}
	}
	shape.TextBody.Paragraphs = paragraphs

	// Strip inherited bold and all-caps from lstStyle so only inline tag formatting applies.
	stripLstStyleBold(shape)
	stripLstStyleCaps(shape)

	// Cap font sizes only when the layout doesn't explicitly declare one.
	// If lstStyle has an explicit sz, the layout designer intentionally chose that size;
	// normAutofit handles overflow.
	capLstStyleFontSizeIfUnset(shape, 2400) // 24pt max for body text

	// Set a minimum base font size so normAutofit scaling doesn't produce
	// illegibly small text. Dense lists (12+) use 10pt floor instead of 12pt
	// to give the autofit algorithm more room, while keeping the effective
	// minimum (base × fontScale) above the ~10pt readability threshold.
	if len(bullets) >= 12 {
		floorLstStyleFontSize(shape, 1000) // 10pt min for very dense lists
	} else {
		floorLstStyleFontSize(shape, 1200) // 12pt min
	}

	// Replace spAutoFit (grow-box-to-fit-text) with nothing so normAutofit can be applied.
	replaceSpAutoFitWithNorm(shape)

	// Enforce text wrapping within placeholder bounds (prevents overflow into adjacent columns)
	enforceTextWrap(shape)

	// Enable autofit for dense content to prevent overflow.
	// For dense bullet lists (10+ items), use a reduced readability threshold
	// but still enforce a floor that keeps text above ~10pt effective size.
	// Previous thresholds (25%/35% for 12+ items) allowed text to shrink to
	// ~8-9pt which is near the projection readability limit. The raised
	// thresholds trigger trimForReadability to drop trailing bullets with "…"
	// rather than rendering all bullets at an illegibly small size.
	if floor, minPct := AutofitDensityPolicy(len(bullets)); minPct > 0 {
		opts := append([]autofitOption{withReadabilityMinScale(floor), withMinFontScalePct(minPct)}, autofitOpts...)
		applySmartAutofitWithOptions(shape, opts...)
	} else {
		applySmartAutofitWithOptions(shape, autofitOpts...)
	}

	// Vertically center sparse bullet lists to avoid excessive top-floating whitespace.
	centerIfSparse(shape, len(bullets))

	return nil
}

// setBodyAndBulletsParagraphs sets a body paragraph followed by bullet paragraphs.
// The body text appears without a bullet marker, followed by bullet points.
// It preserves paragraph and run properties from the layout template.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBodyAndBulletsParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int, autofitOpts ...autofitOption) error {
	content, ok := value.(BodyAndBulletsContent)
	if !ok {
		return fmt.Errorf("invalid body_and_bullets value for placeholder %s", placeholderID)
	}

	// Extract template styling from existing paragraphs
	templateStyles := extractBulletTemplateStyles(shape.TextBody.Paragraphs)

	// Determine the bullet level to use:
	// 1. If masterBulletLevel is valid (>=0), use it (from slide master)
	// 2. Otherwise try to detect from layout template styles
	// 3. Fall back to level 0
	bulletLevel := masterBulletLevel
	if bulletLevel < 0 {
		bulletLevel = findFirstBulletLevel(templateStyles)
	}
	if bulletLevel < 0 {
		// No bullet-enabled level found, use level 0 as fallback
		bulletLevel = 0
	}

	var paragraphs []paragraphXML

	// Add body text as paragraph(s) (using level 0 style without bullet).
	// Body may contain multiple paragraphs separated by "\n" (from blank lines
	// in the source markdown). Each is rendered as a separate OOXML paragraph.
	if content.Body != "" {
		for _, bodyPara := range strings.Split(content.Body, "\n") {
			if bodyPara == "" {
				continue
			}
			_, rProps := getBulletStyleForLevel(templateStyles, 0)
			runs := createFormattedRuns(bodyPara, rProps)
			// Body text acts as a section header: render bold in the
			// template's body (minor) font. Any latin typeface on the
			// template's level-0 run style is kept; otherwise the font is
			// inherited from the placeholder / theme (+mn-lt). Never force a
			// literal font family here — it breaks template fidelity.
			for i := range runs {
				if runs[i].RunProperties == nil {
					runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
				}
				runs[i].RunProperties.Bold = "1"
			}
			paragraphs = append(paragraphs, paragraphXML{
				// Suppress bullet marker and hanging indent on body text.
				Properties: noBulletParagraphProps(""),
				Runs:       runs,
			})
		}
	}

	// Add bullet paragraphs, each at the level its own indent asks for.
	for _, bullet := range content.Bullets {
		depth, text := pptx.BulletIndentDepth(bullet)
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletParagraphLevel(bulletLevel, depth))
		// Parse inline tag formatting and create runs
		runs := createFormattedRuns(text, rProps)
		paragraphs = append(paragraphs, paragraphXML{
			Properties: pProps,
			Runs:       runs,
		})
	}

	// Add trailing body text after bullets (using level 0 style without bullet).
	// TrailingBody may contain multiple paragraphs separated by "\n".
	if content.TrailingBody != "" {
		for j, trailingPara := range strings.Split(content.TrailingBody, "\n") {
			if trailingPara == "" {
				continue
			}
			_, rProps := getBulletStyleForLevel(templateStyles, 0)
			runs := createFormattedRuns(trailingPara, rProps)
			// First trailing paragraph gets extra spcBef for visual separation
			// from preceding bullets. Subsequent paragraphs use no extra spacing.
			extraInner := ""
			if j == 0 {
				extraInner = `<a:spcBef><a:spcPts val="2400"/></a:spcBef>`
			}
			paragraphs = append(paragraphs, paragraphXML{
				Properties: noBulletParagraphProps(extraInner),
				Runs:       runs,
			})
		}
	}

	shape.TextBody.Paragraphs = paragraphs

	// Strip inherited bold and all-caps from lstStyle so only inline tag formatting applies.
	stripLstStyleBold(shape)
	stripLstStyleCaps(shape)

	// Cap font sizes only when the layout doesn't explicitly declare one.
	capLstStyleFontSizeIfUnset(shape, 2400) // 24pt max for body text

	// Set a minimum base font size for dense lists (see setBulletParagraphs).
	totalBullets := len(content.Bullets)
	if totalBullets >= 12 {
		floorLstStyleFontSize(shape, 1000) // 10pt min for very dense lists
	} else {
		floorLstStyleFontSize(shape, 1200) // 12pt min
	}

	// Replace spAutoFit (grow-box-to-fit-text) with nothing so normAutofit can be applied.
	replaceSpAutoFitWithNorm(shape)

	// Enforce text wrapping within placeholder bounds (prevents overflow into adjacent columns)
	enforceTextWrap(shape)

	// Enable autofit for dense content to prevent overflow.
	// Raised readability thresholds (from 25%/35%) to keep effective font
	// size above ~10pt — prefer trimming trailing bullets over tiny text.
	if totalBullets >= 12 {
		opts := append([]autofitOption{withReadabilityMinScale(45000), withMinFontScalePct(45)}, autofitOpts...)
		applySmartAutofitWithOptions(shape, opts...)
	} else if totalBullets >= 10 {
		opts := append([]autofitOption{withReadabilityMinScale(50000), withMinFontScalePct(50)}, autofitOpts...)
		applySmartAutofitWithOptions(shape, opts...)
	} else {
		applySmartAutofitWithOptions(shape, autofitOpts...)
	}

	// Vertically center sparse body+bullet content to avoid excessive top-floating whitespace.
	centerIfSparse(shape, len(shape.TextBody.Paragraphs))

	return nil
}

// setBodyAndLeadParagraphs renders a lead-in paragraph (thesis) followed by
// supporting bullets (evidence). The lead renders at 16pt bold with no bullet
// marker; bullets render at 12pt with standard bullet formatting and hanging indent.
func setBodyAndLeadParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int, autofitOpts ...autofitOption) error {
	content, ok := value.(BodyAndLeadContent)
	if !ok {
		return fmt.Errorf("invalid body_and_lead value for placeholder %s", placeholderID)
	}

	// Extract template styling from existing paragraphs
	templateStyles := extractBulletTemplateStyles(shape.TextBody.Paragraphs)

	// Determine the bullet level to use
	bulletLevel := masterBulletLevel
	if bulletLevel < 0 {
		bulletLevel = findFirstBulletLevel(templateStyles)
	}
	if bulletLevel < 0 {
		bulletLevel = 0
	}

	var paragraphs []paragraphXML

	// Lead-in paragraph: 16pt bold, no bullet
	if content.Lead != "" {
		_, rProps := getBulletStyleForLevel(templateStyles, 0)
		runs := createFormattedRuns(content.Lead, rProps)
		for i := range runs {
			if runs[i].RunProperties == nil {
				runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			runs[i].RunProperties.Bold = "1"
			runs[i].RunProperties.FontSize = "1600" // 16pt
		}
		paragraphs = append(paragraphs, paragraphXML{
			Properties: noBulletParagraphProps(`<a:spcAft><a:spcPts val="600"/></a:spcAft>`),
			Runs:       runs,
		})
	}

	// Supporting bullets: 12pt regular with bullet markers, each at the level
	// its own indent asks for.
	for _, bullet := range content.Bullets {
		depth, text := pptx.BulletIndentDepth(bullet)
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletParagraphLevel(bulletLevel, depth))
		runs := createFormattedRuns(text, rProps)
		// Override font size to 12pt for supporting bullets
		for i := range runs {
			if runs[i].RunProperties == nil {
				runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			runs[i].RunProperties.FontSize = "1200" // 12pt
		}
		paragraphs = append(paragraphs, paragraphXML{
			Properties: pProps,
			Runs:       runs,
		})
	}

	shape.TextBody.Paragraphs = paragraphs

	// Strip inherited bold and all-caps from lstStyle
	stripLstStyleBold(shape)
	stripLstStyleCaps(shape)

	// Cap to 24pt max and 12pt floor
	capLstStyleFontSizeIfUnset(shape, 2400)
	floorLstStyleFontSize(shape, 1200)

	// Replace spAutoFit with normAutofit
	replaceSpAutoFitWithNorm(shape)
	enforceTextWrap(shape)
	applySmartAutofitWithOptions(shape, autofitOpts...)

	return nil
}

// setBulletGroupsParagraphs sets paragraphs for grouped bullets with section headers.
// Each group's header is rendered at level 0 (no bullet marker), and bullets at level 1+.
// This preserves the hierarchical structure where bold text serves as section headers.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBulletGroupsParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int, autofitOpts ...autofitOption) error {
	content, ok := value.(BulletGroupsContent)
	if !ok {
		return fmt.Errorf("invalid bullet_groups value for placeholder %s", placeholderID)
	}

	if len(content.Groups) == 0 && content.Body == "" {
		shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}
		return nil
	}

	// Extract template styling from existing paragraphs
	templateStyles := extractBulletTemplateStyles(shape.TextBody.Paragraphs)

	// Determine the bullet level to use:
	// 1. If masterBulletLevel is valid (>=0), use it (from slide master)
	// 2. Otherwise try to detect from layout template styles
	// 3. Fall back to level 0
	bulletLevel := masterBulletLevel
	if bulletLevel < 0 {
		bulletLevel = findFirstBulletLevel(templateStyles)
	}
	if bulletLevel < 0 {
		bulletLevel = 0
	}

	var paragraphs []paragraphXML

	// For dense layouts (≥3 groups), use compact spacing and merge group body
	// into header to reduce paragraph count and prevent excessive font scaling.
	// At 12pt spcBef with 4 groups (19 paragraphs), textfit scales to 60% (12pt),
	// which is illegibly small. Compact mode reduces this to ~70% (14pt).
	// 3-4 groups use moderate spacing (10pt) for visual separation between groups.
	// 5+ groups use tight spacing (6pt) to fit more content.
	denseGroups := len(content.Groups) >= 3
	headerSpcBefVal := "1200" // 12pt
	if len(content.Groups) >= 5 {
		headerSpcBefVal = "600" // 6pt — tight spacing for very dense layouts
	} else if denseGroups {
		headerSpcBefVal = "1000" // 10pt — moderate spacing for 3-4 groups
	}

	// Add intro body paragraph(s) before groups.
	// Body is always the text preceding the first section header/bullets in the
	// markdown source (populated by extractBodySplit), so it renders first.
	// Body may contain multiple paragraphs separated by "\n".
	if content.Body != "" {
		for _, bodyPara := range strings.Split(content.Body, "\n") {
			if bodyPara == "" {
				continue
			}
			_, rProps := getBulletStyleForLevel(templateStyles, 0)
			runs := createFormattedRuns(bodyPara, rProps)
			paragraphs = append(paragraphs, paragraphXML{
				Properties: noBulletParagraphProps(""),
				Runs:       runs,
			})
		}
	}

	for _, group := range content.Groups {
		paragraphs = append(paragraphs, buildGroupParagraphs(group, denseGroups, headerSpcBefVal, bulletLevel, templateStyles)...)
	}

	// Add trailing body paragraph(s) if present (rendered without bullet marker).
	// TrailingBody may contain multiple paragraphs separated by "\n".
	if content.TrailingBody != "" {
		for j, trailingPara := range strings.Split(content.TrailingBody, "\n") {
			if trailingPara == "" {
				continue
			}
			_, rProps := getBulletStyleForLevel(templateStyles, 0)
			runs := createFormattedRuns(trailingPara, rProps)
			extraInner := ""
			if j == 0 {
				extraInner = `<a:spcBef><a:spcPts val="2400"/></a:spcBef>`
			}
			paragraphs = append(paragraphs, paragraphXML{
				Properties: noBulletParagraphProps(extraInner),
				Runs:       runs,
			})
		}
	}

	shape.TextBody.Paragraphs = paragraphs

	// Strip inherited bold and all-caps from lstStyle so only inline tag formatting applies.
	stripLstStyleBold(shape)
	stripLstStyleCaps(shape)

	// Cap font sizes only when the layout doesn't explicitly declare one.
	capLstStyleFontSizeIfUnset(shape, 2400) // 24pt max for body text
	floorLstStyleFontSize(shape, 1200)      // 12pt min

	// Replace spAutoFit (grow-box-to-fit-text) with nothing so normAutofit can be applied.
	replaceSpAutoFitWithNorm(shape)

	// Enforce text wrapping within placeholder bounds (prevents overflow into adjacent columns)
	enforceTextWrap(shape)

	// Enable autofit for dense content to prevent overflow.
	// For bullet groups, use a much lower readability threshold to preserve
	// all authored groups rather than trimming for larger font.
	// The user explicitly created N groups; losing one is worse than smaller text.
	// 5+ groups: effectively disable readability trimming (20%), allow font down to 40%.
	// 4 groups: moderate threshold (35%), allow font down to 45%.
	if len(content.Groups) >= 5 {
		opts := append([]autofitOption{withReadabilityMinScale(20000), withMinFontScalePct(40)}, autofitOpts...)
		applySmartAutofitWithOptions(shape, opts...)
	} else if len(content.Groups) >= 4 {
		opts := append([]autofitOption{withReadabilityMinScale(35000), withMinFontScalePct(45)}, autofitOpts...)
		applySmartAutofitWithOptions(shape, opts...)
	} else {
		applySmartAutofitWithOptions(shape, autofitOpts...)
	}

	return nil
}

// buildGroupParagraphs converts a single BulletGroup into paragraphs with
// optional dense-mode header merging, body text, and indented sub-bullets.
func buildGroupParagraphs(group BulletGroup, denseGroups bool, headerSpcBefVal string, bulletLevel int, templateStyles []bulletLevelStyle) []paragraphXML {
	var paragraphs []paragraphXML

	// Build header text, optionally merging group body for dense layouts.
	headerText := group.Header
	groupBodyRendered := false
	if denseGroups && group.Body != "" && headerText != "" {
		bodyLines := strings.Split(group.Body, "\n")
		if len(bodyLines) > 0 && bodyLines[0] != "" {
			headerText += " — " + bodyLines[0]
			groupBodyRendered = len(bodyLines) == 1
		}
	}

	// Group label (small-caps accent text above the header)
	if group.GroupLabel != "" {
		_, rProps := getBulletStyleForLevel(templateStyles, 0)
		runs := createFormattedRuns(group.GroupLabel, rProps)
		for i := range runs {
			if runs[i].RunProperties == nil {
				runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
			}
			runs[i].RunProperties.FontSize = "1000" // 10pt
			// Add cap="small" for small-caps rendering
			// Font family is inherited from the template (theme minor font).
			runs[i].RunProperties.Caps = "small"
		}
		spcBef := fmt.Sprintf(`<a:spcBef><a:spcPts val="%s"/></a:spcBef>`, headerSpcBefVal)
		paragraphs = append(paragraphs, paragraphXML{
			Properties: noBulletParagraphProps(spcBef),
			Runs:       runs,
		})
	}

	// Section header (no bullet marker, forced bold for visual hierarchy)
	if headerText != "" {
		_, rProps := getBulletStyleForLevel(templateStyles, 0)
		runs := createFormattedRuns(headerText, rProps)
		for i := range runs {
			runs[i].RunProperties.Bold = "1"
		}
		// When there's a group label above, use tighter spacing before the header
		hSpcBef := headerSpcBefVal
		if group.GroupLabel != "" {
			hSpcBef = "200" // 2pt — tight coupling between label and header
		}
		spcBef := fmt.Sprintf(`<a:spcBef><a:spcPts val="%s"/></a:spcBef>`, hSpcBef)
		paragraphs = append(paragraphs, paragraphXML{
			Properties: noBulletParagraphProps(spcBef),
			Runs:       runs,
		})
	}

	// Group-level body text (skipped if already merged into header)
	if group.Body != "" && !groupBodyRendered {
		bodyText := group.Body
		if denseGroups && headerText != group.Header {
			bodyLines := strings.Split(group.Body, "\n")
			if len(bodyLines) > 1 {
				bodyText = strings.Join(bodyLines[1:], "\n")
			} else {
				bodyText = ""
			}
		}
		for _, bodyPara := range strings.Split(bodyText, "\n") {
			if bodyPara == "" {
				continue
			}
			_, rProps := getBulletStyleForLevel(templateStyles, 0)
			runs := createFormattedRuns(bodyPara, rProps)
			paragraphs = append(paragraphs, paragraphXML{
				Properties: noBulletParagraphProps(""),
				Runs:       runs,
			})
		}
	}

	// Sub-bullets (indented one level deeper than the base bullet level), plus
	// whatever depth the bullet's own leading whitespace asks for.
	subBulletLevel := bulletLevel + 1
	for _, bullet := range group.Bullets {
		depth, text := pptx.BulletIndentDepth(bullet)
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletParagraphLevel(subBulletLevel, depth))
		runs := createFormattedRuns(text, rProps)
		if pProps.MarL != nil && *pProps.MarL == 0 {
			fallbackMarL := 360000
			pProps.MarL = &fallbackMarL
		}
		paragraphs = append(paragraphs, paragraphXML{
			Properties: pProps,
			Runs:       runs,
		})
	}

	return paragraphs
}

// prependEyebrowParagraph inserts a small-caps eyebrow paragraph before the
// existing title text in a shape. Its alignment and horizontal offsets follow
// the title paragraph (including a template lstStyle when values are omitted).
func prependEyebrowParagraph(shape *shapeXML, eyebrow string, fontSizeHPt int) {
	if shape.TextBody == nil || len(shape.TextBody.Paragraphs) == 0 {
		return
	}
	if fontSizeHPt < 1200 {
		fontSizeHPt = 1200
	}
	alignment := ""
	var marginLeft, indent *int
	if props := shape.TextBody.Paragraphs[0].Properties; props != nil {
		alignment = props.Algn
		marginLeft = props.MarL
		indent = props.Indent
	}

	eyebrowRun := runXML{
		Text: eyebrow,
		RunProperties: &runPropertiesXML{
			Lang:     "en-US",
			FontSize: fmt.Sprintf("%d", fontSizeHPt),
			Caps:     "small",
			// Theme minor (body) font: the eyebrow sits inside a title
			// placeholder whose default is the major font.
			Inner: `<a:latin typeface="+mn-lt"/>`,
		},
	}

	eyebrowPara := paragraphXML{
		Properties: &paragraphPropertiesXML{
			MarL:   marginLeft,
			Indent: indent,
			Algn:   alignment,
			Inner:  `<a:buNone/><a:spcAft><a:spcPts val="200"/></a:spcAft>`,
		},
		Runs: []runXML{eyebrowRun},
	}

	// Prepend eyebrow before existing paragraphs
	shape.TextBody.Paragraphs = append([]paragraphXML{eyebrowPara}, shape.TextBody.Paragraphs...)
}
