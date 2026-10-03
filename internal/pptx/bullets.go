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
	DetectNumbered bool // Also detect "N. " numbered lists (two or more consecutive lines counting 1, 2, 3 …)
	InlineTags     bool // Parse <b>, <i>, <u> inline formatting tags
}

// ParseBulletText splits text on newlines and detects bullet prefixes.
// Lines starting with "- " or "• " become bulleted paragraphs. If DetectNumbered
// is true, a numbered list — two or more consecutive "N. " lines counting 1, 2,
// 3 … — becomes auto-numbered paragraphs; any other line that merely opens
// with a number keeps its text as written (see NumberedRuns). Plain lines become regular paragraphs with the
// same run styling.
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
	var inList []bool
	if opts.DetectNumbered {
		inList = NumberedRuns(lines)
	}

	for i, line := range lines {
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
		} else if inList != nil && inList[i] {
			_, run.Text, _ = ParseNumberedPrefix(line)
			isBullet = true
			isNumberedLine = true
		}

		if isBullet {
			para.MarginL = BulletMarginLeft
			para.Indent = BulletIndent
			para.SpaceAfter = opts.SpaceAfter
			if isNumberedLine {
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

// NumberedRuns finds the numbered lists in a block of lines that the renderer
// can number itself. The result has one entry per line: true when the line
// belongs to such a list, false when it must be written as authored.
//
// A list is two or more consecutive lines that each open with "N. " and count
// 1, 2, 3 …. Auto-numbering counts from 1 at every list, so a line can only
// hand its number to the renderer when the renderer will count to that number
// again: a lone "2. Enabler bar: funded first" used to be stripped to an
// auto-numbered paragraph and rendered "1. Enabler bar: funded first", and a
// list typed 3, 4, 5 rendered 1, 2, 3 (go-slide-creator-zdzk2). Those lines
// keep their literal number.
//
// A list that starts above 1 is deliberately not auto-numbered with startAt:
// PowerPoint continues a list only when every paragraph repeats the same
// startAt, and LibreOffice — which draws the thumbnails an author checks —
// restarts at startAt on every such paragraph ("3. 3. 3."). The typed numbers
// are the one form both draw the same.
func NumberedRuns(lines []string) []bool {
	numbered := make([]bool, len(lines))
	for i := 0; i < len(lines); {
		first, rest, ok := ParseNumberedPrefix(lines[i])
		if !ok || first != 1 || strings.TrimSpace(rest) == "" {
			i++
			continue
		}
		end := i + 1
		for end < len(lines) {
			n, rest, ok := ParseNumberedPrefix(lines[end])
			if !ok || n != 1+(end-i) || strings.TrimSpace(rest) == "" {
				break
			}
			end++
		}
		if end-i >= 2 {
			for j := i; j < end; j++ {
				numbered[j] = true
			}
		}
		i = end
	}
	return numbered
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
