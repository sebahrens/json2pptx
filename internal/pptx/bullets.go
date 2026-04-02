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
}

// ParseBulletText splits text on newlines and detects bullet prefixes.
// Lines starting with "- " or "• " become bulleted paragraphs. If DetectNumbered
// is true, lines matching "N. " (digits + dot + space) also become bullets.
// Plain lines become regular paragraphs with the same run styling.
func ParseBulletText(text string, opts BulletTextOptions) []Paragraph {
	bulletChar := opts.BulletChar
	if bulletChar == "" {
		bulletChar = "\u2022"
	}
	bulletFont := opts.BulletFont
	if bulletFont == "" {
		bulletFont = "Arial"
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
			}
		}

		if isBullet {
			para.MarginL = BulletMarginLeft
			para.Indent = BulletIndent
			para.SpaceAfter = opts.SpaceAfter
			para.Bullet = &BulletDef{
				Char:  bulletChar,
				Font:  bulletFont,
				Color: opts.BulletColor,
			}
		}

		para.Runs = []Run{run}
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
