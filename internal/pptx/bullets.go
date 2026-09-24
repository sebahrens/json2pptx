package pptx

import "strings"

// Standard bullet formatting constants shared across all text rendering pipelines.
const (
	BulletMarginLeft int64 = 177800  // paragraph left margin for bullet indentation
	BulletIndent     int64 = -177800 // hanging indent (negative = bullet hangs left)
)

// BulletTextOptions configures how ParseBulletText builds paragraphs.
type BulletTextOptions struct {
	FontSize   int    // Font size in hundredths of a point (e.g. 1400 = 14pt)
	Bold       bool   // Bold text
	Italic     bool   // Italic text
	Align      string // Paragraph alignment ("l", "ctr", "r", "just")
	Color      Fill   // Text color fill
	FontFamily string // Font typeface (e.g. "+mn-lt", "Arial")
	Lang       string // Language tag (e.g. "en-US")
	Dirty      bool   // Emit dirty="0" spell-check flag

	// Bullet styling
	BulletChar  string // Bullet character (default "•")
	BulletFont  string // Bullet font family (default "Arial")
	BulletColor Fill   // Bullet color (zero value = inherit)
	SpaceAfter  int    // Space after bullet paragraphs in hundredths of a point

	// Detection controls
	DetectNumbered bool // Also detect "N. " numbered list prefixes
	InlineTags     bool // Parse <b>, <i>, <u> inline formatting tags
}

// ParseBulletText splits text on newlines and detects bullet prefixes.
// Lines starting with "- " or "• " become bulleted paragraphs. If DetectNumbered
// is true, lines matching "N. " (digits + dot + space) also become bullets.
// Plain lines become regular paragraphs with the same run styling.
func ParseBulletText(text string, opts BulletTextOptions) []Paragraph {
	bulletChar := opts.BulletChar
	if bulletChar == "" {
		bulletChar = DefaultBulletChar
	}
	bulletFont := opts.BulletFont
	if bulletFont == "" {
		bulletFont = DefaultBulletFont
	}

	lines := strings.Split(text, "\n")
	paragraphs := make([]Paragraph, 0, len(lines))

	for _, line := range lines {
		run := Run{
			Text:       line,
			FontSize:   opts.FontSize,
			Bold:       opts.Bold,
			Italic:     opts.Italic,
			Color:      opts.Color,
			FontFamily: opts.FontFamily,
			Lang:       opts.Lang,
			Dirty:      opts.Dirty,
		}
		para := Paragraph{Align: opts.Align}

		isBullet := false
		isNumberedLine := false
		if strings.HasPrefix(line, "- ") {
			run.Text = strings.TrimPrefix(line, "- ")
			isBullet = true
		} else if strings.HasPrefix(line, "\u2022 ") {
			run.Text = strings.TrimPrefix(line, "\u2022 ")
			isBullet = true
		} else if opts.DetectNumbered {
			if _, rest, ok := ParseNumberedPrefix(line); ok {
				run.Text = rest
				isBullet = true
				isNumberedLine = true
			}
		}

		if isBullet {
			para.MarginL = BulletMarginLeft
			para.Indent = BulletIndent
			para.SpaceAfter = opts.SpaceAfter
			if opts.DetectNumbered && isNumberedLine {
				// Use OOXML auto-numbering for numbered lists.
				// The hanging indent from MarginL+Indent ensures multi-line
				// wraps align under the text, not under the number.
				para.Bullet = &BulletDef{
					AutoNum: "arabicPeriod",
					Font:    bulletFont,
					Color:   opts.BulletColor,
				}
			} else {
				para.Bullet = &BulletDef{
					Char:  bulletChar,
					Font:  bulletFont,
					Color: opts.BulletColor,
				}
			}
		}

		if opts.InlineTags && strings.Contains(run.Text, "<") {
			para.Runs = SplitInlineTags(run)
		} else {
			para.Runs = []Run{run}
		}
		paragraphs = append(paragraphs, para)
	}

	return paragraphs
}

// ParseNumberedPrefix checks if line starts with a numbered list prefix like "1. ", "12. ".
// Returns the number, the remaining text, and whether the pattern matched.
func ParseNumberedPrefix(line string) (int, string, bool) {
	dotIdx := strings.Index(line, ". ")
	if dotIdx < 1 {
		return 0, "", false
	}
	prefix := line[:dotIdx]
	num := 0
	for _, ch := range prefix {
		if ch < '0' || ch > '9' {
			return 0, "", false
		}
		num = num*10 + int(ch-'0')
	}
	return num, line[dotIdx+2:], true
}

// DefaultBulletChar and DefaultBulletFont are the bullet glyph and the buFont
// it must be resolved in. A <a:buChar> without a <a:buFont> is looked up in the
// theme font, which may not carry U+2022 at all — renderers then substitute a
// different glyph in the bullet colour (go-slide-creator-2zej). Every caller
// that sets a bullet character should name this font.
const (
	DefaultBulletChar = "\u2022"
	DefaultBulletFont = "Arial"
)

// BulletIndentDepth measures the nesting depth a bullet's leading whitespace
// asks for, and returns the bullet text with that whitespace removed.
//
// One tab is one level; two leading spaces are one level. A markdown marker
// ("- ", "* ", "• ") after the indent is left in the text — the indent that
// precedes it is what conveys nesting, and the caller decides what to do with
// the marker.
//
// The fit-report lint measured this depth to report BULLET_NESTING_DEEP while
// the renderer emitted every paragraph at lvl="0", so an authored sub-bullet
// rendered with the parent's glyph and size and the text pushed right — it
// read as a typo (go-slide-creator-gyfl). Both sides read the depth here now.
func BulletIndentDepth(s string) (int, string) {
	level := 0
	spaces := 0
	for i, r := range s {
		switch r {
		case '\t':
			level++
			spaces = 0
		case ' ':
			spaces++
			if spaces >= 2 {
				level++
				spaces = 0
			}
		default:
			return level, s[i:]
		}
	}
	// Whitespace only: there is no bullet to indent.
	return 0, ""
}
