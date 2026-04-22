// Package generator provides PPTX file generation from slide specifications.
package generator

import "strings"

// bulletLevelStyle holds paragraph and run properties for a bullet level.
type bulletLevelStyle struct {
	pProps *paragraphPropertiesXML
	rProps *runPropertiesXML
}

// extractTemplateTextStyle extracts paragraph and run properties from the first
// paragraph in a template. These properties contain font, size, color, etc.
// Returns nil for each if no template styling is available.
func extractTemplateTextStyle(paragraphs []paragraphXML) (*paragraphPropertiesXML, *runPropertiesXML) {
	if len(paragraphs) == 0 {
		return nil, defaultRunProperties()
	}

	var pProps *paragraphPropertiesXML
	var rProps *runPropertiesXML

	// Clone paragraph properties if present (check for Level attribute or Inner content)
	firstPara := &paragraphs[0]
	if firstPara.Properties != nil && (firstPara.Properties.Level != nil || firstPara.Properties.Inner != "") {
		pProps = cloneParagraphProperties(firstPara.Properties)
	}

	// Clone run properties from the first run if present
	if len(firstPara.Runs) > 0 && firstPara.Runs[0].RunProperties != nil {
		templateRProps := firstPara.Runs[0].RunProperties
		if templateRProps.Lang != "" || templateRProps.Inner != "" {
			rProps = cloneRunProperties(templateRProps)
		}
	}

	// Provide default if no template run properties found
	if rProps == nil {
		rProps = defaultRunProperties()
	}

	return pProps, rProps
}

// extractBulletTemplateStyles extracts paragraph and run properties for each
// bullet level defined in the template. Body placeholders typically define
// 5 levels (lvl="0" through lvl="4").
func extractBulletTemplateStyles(paragraphs []paragraphXML) []bulletLevelStyle {
	if len(paragraphs) == 0 {
		return nil
	}

	styles := make([]bulletLevelStyle, len(paragraphs))
	for i, para := range paragraphs {
		style := bulletLevelStyle{}

		// Clone paragraph properties (contains lvl, bullet style, etc.)
		// Check for either Level attribute or Inner content (child elements)
		if para.Properties != nil && (para.Properties.Level != nil || para.Properties.Inner != "") {
			style.pProps = cloneParagraphProperties(para.Properties)
		}

		// Clone run properties from the first run
		if len(para.Runs) > 0 && para.Runs[0].RunProperties != nil {
			rProps := para.Runs[0].RunProperties
			if rProps.Lang != "" || rProps.Inner != "" {
				style.rProps = cloneRunProperties(rProps)
			}
		}

		// Provide default if no template run properties found
		if style.rProps == nil {
			style.rProps = defaultRunProperties()
		}

		styles[i] = style
	}

	return styles
}

// hasBulletsDisabled checks if a paragraph style has bullets explicitly disabled.
// This happens when the Inner XML contains <a:buNone/> which is common in
// corporate templates where level 0-1 are for headings without bullets.
func hasBulletsDisabled(pProps *paragraphPropertiesXML) bool {
	if pProps == nil || pProps.Inner == "" {
		return false
	}
	// Check for buNone element which explicitly disables bullets
	return strings.Contains(pProps.Inner, "buNone")
}

// suppressBulletInParagraphProps ensures buNone is present in paragraph properties
// to suppress inherited bullet markers. Body-only text (ContentText) should never
// show a bullet character even when the template defines bullets in lstStyle.
// Also resets marL and indent to zero to remove inherited hanging indent from the
// bodyStyle. Without this, wrapped lines show a hanging indent (second line indented
// further than the first) because the bodyStyle defines marL/indent for bullet
// positioning, but buNone only suppresses the bullet character—not the indentation.
// If props is nil, creates new properties with buNone and zero margins.
// If props already has buNone, still ensures margins are zeroed.
func suppressBulletInParagraphProps(props *paragraphPropertiesXML) *paragraphPropertiesXML {
	buNoneXML := `<a:buNone/>`
	zeroMarL := 0
	zeroIndent := 0
	if props == nil {
		return &paragraphPropertiesXML{
			MarL:   &zeroMarL,
			Indent: &zeroIndent,
			Algn:   "l",
			Inner:  buNoneXML,
		}
	}
	// Clone and ensure buNone + zero margins + explicit left alignment.
	// Explicit algn="l" prevents inheriting center alignment from the
	// slide master's bodyStyle (some templates define algn="ctr" in lvl1pPr).
	cloned := *props
	cloned.MarL = &zeroMarL
	cloned.Indent = &zeroIndent
	cloned.Algn = "l"
	if !hasBulletsDisabled(&cloned) {
		cloned.Inner += buNoneXML
	}
	return &cloned
}

// noBulletParagraphProps returns paragraph properties that suppress bullets and
// reset indentation. Use this for body text paragraphs that should not show
// bullet markers or hanging indents inherited from the bodyStyle.
// Optional extraInner is prepended to the Inner XML (e.g. spcBef for spacing).
func noBulletParagraphProps(extraInner string) *paragraphPropertiesXML {
	zeroMarL := 0
	zeroIndent := 0
	return &paragraphPropertiesXML{
		MarL:   &zeroMarL,
		Indent: &zeroIndent,
		Algn:   "l",
		Inner:  extraInner + `<a:buNone/>`,
	}
}

// findFirstBulletLevel finds the first level that has bullets enabled.
// Some templates disable bullets at levels 0-1 and only enable
// them at level 2+. Returns -1 if no bullet-enabled level is found.
// A level is considered bullet-enabled if:
// - pProps is nil (no explicit styling, inherits from master which typically has bullets)
// - pProps.Inner is empty (no buNone element)
// - pProps.Inner contains bullet styling but not buNone
func findFirstBulletLevel(styles []bulletLevelStyle) int {
	for i, style := range styles {
		// If no explicit paragraph properties, bullets are not disabled
		// If properties exist but no buNone, bullets are enabled
		if !hasBulletsDisabled(style.pProps) {
			return i
		}
	}
	return -1
}

// getBulletStyleForLevel returns the paragraph and run properties for a given
// bullet level. If the level is out of range, returns the last available level's style.
// The returned pProps will have its Level set to the requested level so that bullets
// inherit styling from the correct level in the slide master's bodyStyle.
func getBulletStyleForLevel(styles []bulletLevelStyle, level int) (*paragraphPropertiesXML, *runPropertiesXML) {
	if len(styles) == 0 {
		return defaultBulletParagraphPropertiesWithLevel(level), defaultRunProperties()
	}

	// Use level to index into styles, clamping to available range
	styleIdx := level
	if styleIdx < 0 {
		styleIdx = 0
	}
	if styleIdx >= len(styles) {
		styleIdx = len(styles) - 1
	}

	style := styles[styleIdx]
	pProps := style.pProps
	rProps := style.rProps

	// Ensure we always have paragraph properties for bullets
	if pProps == nil {
		pProps = defaultBulletParagraphPropertiesWithLevel(level)
	} else {
		// Clone and set the level to the requested level
		// This ensures bullets use the correct level from the slide master's bodyStyle
		cloned := *pProps
		cloned.Level = &level
		pProps = &cloned
	}
	if rProps == nil {
		rProps = defaultRunProperties()
	}

	return pProps, rProps
}

// defaultRunProperties returns minimal run properties with just language set.
// This is used as a fallback when the template doesn't define run properties.
func defaultRunProperties() *runPropertiesXML {
	return &runPropertiesXML{Lang: "en-US"}
}

// defaultBulletParagraphPropertiesWithLevel returns paragraph properties for bullet points
// with the specified level. The level determines which lvlXpPr from the slide master's
// bodyStyle is used for bullet styling.
func defaultBulletParagraphPropertiesWithLevel(level int) *paragraphPropertiesXML {
	return &paragraphPropertiesXML{Level: &level}
}

// cloneParagraphProperties creates a deep copy of paragraph properties.
// Returns nil if the input is nil or has no meaningful content.
func cloneParagraphProperties(props *paragraphPropertiesXML) *paragraphPropertiesXML {
	if props == nil {
		return nil
	}
	// Check if there's anything to clone (Level attribute, Inner content, margin overrides, or alignment)
	if props.Level == nil && props.Inner == "" && props.MarL == nil && props.Indent == nil && props.Algn == "" {
		return nil
	}
	cloned := &paragraphPropertiesXML{Inner: strings.Clone(props.Inner), Algn: props.Algn}
	if props.Level != nil {
		level := *props.Level
		cloned.Level = &level
	}
	if props.MarL != nil {
		v := *props.MarL
		cloned.MarL = &v
	}
	if props.Indent != nil {
		v := *props.Indent
		cloned.Indent = &v
	}
	return cloned
}

// cloneRunProperties creates a deep copy of run properties.
// Returns nil if the input is nil or has no meaningful content.
func cloneRunProperties(props *runPropertiesXML) *runPropertiesXML {
	if props == nil {
		return nil
	}
	// Check if there's anything to clone (Lang attribute or Inner content)
	if props.Lang == "" && props.Inner == "" {
		return nil
	}
	return &runPropertiesXML{
		Lang:  props.Lang,
		Inner: strings.Clone(props.Inner),
	}
}

// createFormattedRuns parses inline tag formatting in text and creates XML runs.
// Supports <b>bold</b>, <i>italic</i>, and <u>underline</u> formatting.
// Returns a slice of runXML with appropriate b and i attributes.
func createFormattedRuns(text string, templateRProps *runPropertiesXML) []runXML {
	// Parse inline tags to get text runs with formatting
	textRuns := ParseInlineTags(text)
	if len(textRuns) == 0 {
		// Empty text
		return nil
	}

	// Convert TextRun to runXML
	runs := make([]runXML, len(textRuns))
	for i, tr := range textRuns {
		// Clone template run properties as base, but do NOT copy Bold.
		// Inline tags are the sole source of bold — template rPr bold is
		// intentionally dropped to prevent body text from inheriting bold from
		// the placeholder's lstStyle or slide master bodyStyle.
		// Italic IS preserved from the template since it doesn't suffer from
		// the same inheritance issue (master styles rarely set italic).
		var rProps *runPropertiesXML
		if templateRProps != nil {
			rProps = &runPropertiesXML{
				Lang:   templateRProps.Lang,
				Italic: templateRProps.Italic,
				Inner:  templateRProps.Inner,
			}
		} else {
			rProps = &runPropertiesXML{Lang: "en-US"}
		}

		// Only set b="1" when inline tags explicitly request bold (<b>bold</b>).
		// For non-bold text, omit the b attribute entirely so font properties
		// (family, size) continue to inherit from the defRPr/lstStyle chain.
		// Bold inheritance from the master bodyStyle is handled by
		// stripDefRPrBold() which sets b="0" on the defRPr level.
		// Setting b="0" directly on <a:rPr> can trigger a PowerPoint quirk
		// where explicit attributes break inheritance for unspecified properties.
		if tr.Bold {
			rProps.Bold = "1"
		}
		if tr.Italic {
			rProps.Italic = "1"
		}
		if tr.Underline {
			rProps.Underline = "sng"
		}

		runs[i] = runXML{
			RunProperties: rProps,
			Text:          tr.Text,
		}
	}

	// Split runs at emoji boundaries and inject emoji-capable font so
	// PowerPoint renders pictographs instead of blank spaces.
	runs = splitEmojiRuns(runs)

	return runs
}
