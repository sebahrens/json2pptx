// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"strings"
)

// populateShapeText sets text content in a shape based on the content item type.
// Returns an error if the content value is invalid for the specified type.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
// themeFontName is the template's theme font used for text fitting measurements
// (e.g., "Franklin Gothic Book"). Pass "" to fall back to built-in metrics.
func populateShapeText(shape *shapeXML, item ContentItem, masterBulletLevel int, themeFontName string) error {
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
		err = setTextParagraph(shape, item.PlaceholderID, item.Value, 2400, themeFontName) // 24pt cap for body text
	case ContentSectionTitle:
		err = setTextParagraph(shape, item.PlaceholderID, item.Value, 0, themeFontName) // no cap — normAutofit scales to fill
	case ContentTitleSlideTitle:
		err = setTitleSlideTitle(shape, item.PlaceholderID, item.Value) // preserve template styling
	case ContentBullets:
		err = setBulletParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel)
	case ContentBodyAndBullets:
		err = setBodyAndBulletsParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel)
	case ContentBulletGroups:
		err = setBulletGroupsParagraphs(shape, item.PlaceholderID, item.Value, masterBulletLevel)
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
// Pass 0 to skip font capping (used for section titles where normAutofit handles sizing).
// themeFontName is the template's theme font for text fitting measurements.
func setTextParagraph(shape *shapeXML, placeholderID string, value interface{}, maxFontSizeHPt int, themeFontName string) error {
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

	// Cap excessively large inherited font sizes.
	if maxFontSizeHPt > 0 {
		// Regular body text: hard cap at 24pt to prevent overflow.
		capLstStyleFontSize(shape, maxFontSizeHPt)
	} else {
		// Section titles (maxFontSizeHPt=0): cap based on placeholder width so that
		// individual words don't wrap at character boundaries. Templates define large
		// lstStyle fonts (e.g., 96pt) in section divider placeholders that overflow
		// narrow body placeholders, causing "Performance" → "Perform/ance" wrapping.
		//
		// Use a character-based width estimate instead of font metric measurement.
		// The tdewolff/canvas library's metrics underestimate rendered widths by ~2x
		// compared to LibreOffice/PowerPoint, making metric-based caps unreliable.
		// Average advance width for a sans-serif character ≈ 0.55 × em (conservative).
		widthEMU, _ := getShapeDimensions(shape)
		fontSizeHPt := extractFontSizeFromShape(shape)
		if widthEMU > 0 && fontSizeHPt > 0 {
			safeMax := maxFontForWordFit(text, widthEMU)
			if safeMax > 0 && fontSizeHPt > safeMax {
				slog.Info("capping section title font to fit placeholder width",
					slog.Int("original_hpt", fontSizeHPt),
					slog.Int("capped_hpt", safeMax),
					slog.Int64("width_emu", widthEMU))
				capLstStyleFontSize(shape, safeMax)
			}
		}
	}
	floorLstStyleFontSize(shape, 1200) // 12pt min

	// Section titles: boost small fonts so the title is visually prominent
	// in its placeholder. Templates may define a small lstStyle font (e.g., 36pt)
	// in a large placeholder where it looks diminished. normAutofit handles overflow.
	if maxFontSizeHPt == 0 {
		boostSectionTitleFont(shape)
	}

	// Limit title wrapping: truncate extremely long titles to maxTitleLines so they
	// don't shrink to tiny font sizes and crowd the body text below. Section titles
	// (maxFontSizeHPt=0) have their own sizing logic and are not truncated here.
	if maxFontSizeHPt > 0 && isTitlePlaceholder(placeholderID) {
		titleWidthEMU, _ := getShapeDimensions(shape)
		titleFontSizeHPt := extractFontSizeFromShape(shape)
		if titleFontSizeHPt == 0 {
			titleFontSizeHPt = 2000 // 20pt default (typical slide master body lvl1)
		}
		truncated := truncateTextToMaxLines(text, titleWidthEMU, titleFontSizeHPt, maxTitleLines, themeFontName)
		if truncated != text {
			slog.Info("truncated long title to fit max lines",
				slog.String("placeholder", placeholderID),
				slog.Int("max_lines", maxTitleLines),
				slog.Int("original_runes", len([]rune(text))),
				slog.Int("truncated_runes", len([]rune(truncated))))
			// Rebuild paragraphs with the truncated text
			var newParas []paragraphXML
			for _, seg := range strings.Split(truncated, "\n") {
				if seg == "" {
					continue
				}
				runs := createFormattedRuns(seg, templateRProps)
				newParas = append(newParas, paragraphXML{
					Properties: textPProps,
					Runs:       runs,
				})
			}
			if len(newParas) > 0 {
				shape.TextBody.Paragraphs = newParas
			}
		}
	}

	// Replace spAutoFit (grow-box-to-fit-text) with nothing so normAutofit can be applied.
	// Decorative placeholders (e.g. section header layouts with
	// 350pt font) use spAutoFit which causes text to overflow below the slide boundary.
	replaceSpAutoFitWithNorm(shape)

	// Enforce text wrapping within placeholder bounds (prevents overflow into adjacent columns)
	enforceTextWrap(shape)

	// Enable autofit if text overflows (e.g. title routed to large-font body placeholder)
	applySmartAutofit(shape, themeFontName)

	return nil
}

// setTitleSlideTitle replaces the text in a title slide's ctrTitle placeholder
// while preserving the template's font size, alignment, and bold styling.
// Unlike setTextParagraph (used for body text), this function does NOT:
//   - Cap the font size (ctrTitle typically uses 40-60pt by design)
//   - Force left alignment (ctrTitle uses centered alignment)
//   - Strip bold/caps from lstStyle (template styling is intentional)
//
// Supports inline tag formatting: <b>bold</b>, <i>italic</i>, <u>underline</u>.
func setTitleSlideTitle(shape *shapeXML, placeholderID string, value interface{}) error {
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
	// Do NOT call replaceSpAutoFitWithNorm — ctrTitle uses noAutofit by design.

	return nil
}

// setBulletParagraphs sets multiple bullet paragraphs in the shape.
// It preserves paragraph and run properties from the layout template.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBulletParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int) error {
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

	paragraphs := make([]paragraphXML, len(bullets))
	for i, bullet := range bullets {
		// Use the first bullet-enabled level for all bullets (single-level list)
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletLevel)

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

	// Cap excessively large inherited font sizes (e.g. template_2 body placeholder
	// with 96pt in lstStyle) before calculating autofit.
	capLstStyleFontSize(shape, 2400) // 24pt max for body text

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
	if len(bullets) >= 12 {
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(45000), withMinFontScalePct(45))
	} else if len(bullets) >= 10 {
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(50000), withMinFontScalePct(50))
	} else {
		applySmartAutofit(shape)
	}

	// Vertically center sparse bullet lists to avoid excessive top-floating whitespace.
	centerIfSparse(shape, len(bullets))

	return nil
}

// setBodyAndBulletsParagraphs sets a body paragraph followed by bullet paragraphs.
// The body text appears without a bullet marker, followed by bullet points.
// It preserves paragraph and run properties from the layout template.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBodyAndBulletsParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int) error {
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
			// Body text acts as a section header: render bold in Arial.
			for i := range runs {
				if runs[i].RunProperties == nil {
					runs[i].RunProperties = &runPropertiesXML{Lang: "en-US"}
				}
				runs[i].RunProperties.Bold = "1"
				// Replace any existing latin font with Arial.
				runs[i].RunProperties.Inner = stripSelfClosingElement(runs[i].RunProperties.Inner, "a:latin") +
					`<a:latin typeface="Arial"/>`
			}
			paragraphs = append(paragraphs, paragraphXML{
				// Suppress bullet marker and hanging indent on body text.
				Properties: noBulletParagraphProps(""),
				Runs:       runs,
			})
		}
	}

	// Add bullet paragraphs (using the first bullet-enabled level)
	for _, bullet := range content.Bullets {
		pProps, rProps := getBulletStyleForLevel(templateStyles, bulletLevel)
		// Parse inline tag formatting and create runs
		runs := createFormattedRuns(bullet, rProps)
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

	// Cap excessively large inherited font sizes before calculating autofit.
	capLstStyleFontSize(shape, 2400) // 24pt max for body text

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
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(45000), withMinFontScalePct(45))
	} else if totalBullets >= 10 {
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(50000), withMinFontScalePct(50))
	} else {
		applySmartAutofit(shape)
	}

	// Vertically center sparse body+bullet content to avoid excessive top-floating whitespace.
	centerIfSparse(shape, len(shape.TextBody.Paragraphs))

	return nil
}

// setBulletGroupsParagraphs sets paragraphs for grouped bullets with section headers.
// Each group's header is rendered at level 0 (no bullet marker), and bullets at level 1+.
// This preserves the hierarchical structure where bold text serves as section headers.
// masterBulletLevel specifies the first level with bullets from the slide master (-1 to auto-detect).
func setBulletGroupsParagraphs(shape *shapeXML, placeholderID string, value interface{}, masterBulletLevel int) error {
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

	// Cap excessively large inherited font sizes before calculating autofit.
	capLstStyleFontSize(shape, 2400)   // 24pt max for body text
	floorLstStyleFontSize(shape, 1200) // 12pt min

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
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(20000), withMinFontScalePct(40))
	} else if len(content.Groups) >= 4 {
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(35000), withMinFontScalePct(45))
	} else {
		applySmartAutofit(shape)
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

	// Section header (no bullet marker, forced bold for visual hierarchy)
	if headerText != "" {
		_, rProps := getBulletStyleForLevel(templateStyles, 0)
		runs := createFormattedRuns(headerText, rProps)
		for i := range runs {
			runs[i].RunProperties.Bold = "1"
		}
		spcBef := fmt.Sprintf(`<a:spcBef><a:spcPts val="%s"/></a:spcBef>`, headerSpcBefVal)
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

	// Sub-bullets (indented one level deeper than the base bullet level)
	subBulletLevel := bulletLevel + 1
	for _, bullet := range group.Bullets {
		pProps, rProps := getBulletStyleForLevel(templateStyles, subBulletLevel)
		runs := createFormattedRuns(bullet, rProps)
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
