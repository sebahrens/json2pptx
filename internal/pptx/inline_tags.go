package pptx

import (
	"strings"
)

// formatState tracks the open nesting depth of each supported inline tag.
// Counting rather than flagging keeps nested and repeated tags
// (<b>a<b>b</b>c</b>) well-behaved.
type formatState struct {
	bold, italic, underline int
	superscript, subscript  int
}

// counterFor maps a tag name (with or without a leading "/") to the depth
// counter it controls, or nil when the tag is not one the renderer supports.
func (s *formatState) counterFor(tag string) *int {
	switch strings.TrimPrefix(tag, "/") {
	case "b":
		return &s.bold
	case "i":
		return &s.italic
	case "u":
		return &s.underline
	case "sup":
		return &s.superscript
	case "sub":
		return &s.subscript
	default:
		return nil
	}
}

// SplitInlineTags parses <b>, <i>, <u>, <sup> and <sub> inline formatting tags
// in a Run's text and returns multiple Runs with the matching Bold / Italic /
// Underline / Baseline properties.
//
// <sup> and <sub> exist because footnote markers are near-universal in
// consulting decks: without them an author writing "+210bps<sup>1</sup>" got
// the tag printed literally on the slide (go-slide-creator-510u).
// The template Run provides base styling (font size, color, font family, etc.).
// If no tags are found, returns a single-element slice with the original Run.
func SplitInlineTags(base Run) []Run {
	text := base.Text
	if !strings.Contains(text, "<") {
		return []Run{base}
	}

	var state formatState
	if base.Bold {
		state.bold = 1
	}
	if base.Italic {
		state.italic = 1
	}
	if base.Underline {
		state.underline = 1
	}

	var runs []Run
	pos := 0

	emit := func(s string) {
		if s == "" {
			return
		}
		r := base
		r.Text = s
		r.Bold = state.bold > 0
		r.Italic = state.italic > 0
		r.Underline = state.underline > 0
		switch {
		case state.superscript > 0:
			r.Baseline = BaselineSuperscript
		case state.subscript > 0:
			r.Baseline = BaselineSubscript
		default:
			r.Baseline = base.Baseline
		}

		// Merge with previous run if formatting matches
		if len(runs) > 0 {
			last := &runs[len(runs)-1]
			if last.Bold == r.Bold && last.Italic == r.Italic && last.Underline == r.Underline && last.Baseline == r.Baseline {
				last.Text += s
				return
			}
		}
		runs = append(runs, r)
	}

	for pos < len(text) {
		tagStart := strings.IndexByte(text[pos:], '<')
		if tagStart == -1 {
			emit(text[pos:])
			break
		}
		tagStart += pos

		// Emit text before tag
		if tagStart > pos {
			emit(text[pos:tagStart])
		}

		tagEnd := strings.IndexByte(text[tagStart:], '>')
		if tagEnd == -1 {
			emit(text[tagStart:])
			break
		}
		tagEnd += tagStart

		// A '<' inside the candidate means the first '<' is a bare
		// less-than, not a tag opener: emit it literally and rescan from the
		// next byte so a real tag after it is not swallowed
		// (go-slide-creator-s1uvj.25).
		if strings.IndexByte(text[tagStart+1:tagEnd], '<') != -1 {
			emit("<")
			pos = tagStart + 1
			continue
		}

		tag := strings.ToLower(strings.TrimSpace(text[tagStart+1 : tagEnd]))

		if counter := state.counterFor(tag); counter != nil {
			if strings.HasPrefix(tag, "/") {
				if *counter > 0 {
					*counter--
				}
			} else {
				*counter++
			}
		} else {
			// Unknown tag — preserve as literal text
			emit(text[tagStart : tagEnd+1])
		}

		pos = tagEnd + 1
	}

	if len(runs) == 0 {
		// All tags, no text content — return empty run
		r := base
		r.Text = ""
		return []Run{r}
	}

	return runs
}

// ConvertMarkdownEmphasis converts markdown-style bold (**text**) and italic
// (*text*) to inline HTML tags (<b>text</b>, <i>text</i>) that SplitInlineTags
// already understands.
//
// Supported syntax:
//   - **bold** → <b>bold</b>
//   - *italic* → <i>italic</i>
//   - ***bold-italic*** → <b><i>bold-italic</i></b>
//   - \* → literal asterisk (escape)
//
// Unmatched asterisks are left as-is. The function is safe for text that
// already contains <b>/<i> tags (they pass through unchanged).
func ConvertMarkdownEmphasis(text string) string {
	if !strings.Contains(text, "*") {
		return text
	}

	var buf strings.Builder
	buf.Grow(len(text) + 20) // small extra for tags

	i := 0
	for i < len(text) {
		// Handle escaped asterisks: \* → literal *
		if text[i] == '\\' && i+1 < len(text) && text[i+1] == '*' {
			buf.WriteByte('*')
			i += 2
			continue
		}

		if text[i] != '*' {
			buf.WriteByte(text[i])
			i++
			continue
		}

		// Count consecutive asterisks at position i
		totalStars := 0
		for i+totalStars < len(text) && text[i+totalStars] == '*' {
			totalStars++
		}

		// Clamp to 3 — more than *** treated as literal extra stars + bold-italic
		stars := totalStars
		if stars > 3 {
			stars = 3
		}

		// Look for matching closing delimiter
		closeStart := findMatchingClose(text, i+totalStars, stars)
		if closeStart < 0 {
			// No match — emit all stars as literal text
			for k := 0; k < totalStars; k++ {
				buf.WriteByte('*')
			}
			i += totalStars
			continue
		}

		// Emit excess stars as literal before the tag
		for k := 0; k < totalStars-stars; k++ {
			buf.WriteByte('*')
		}

		inner := text[i+totalStars : closeStart]
		// Don't match empty spans like ** ** or * *
		if len(strings.TrimSpace(inner)) == 0 {
			for k := 0; k < totalStars; k++ {
				buf.WriteByte('*')
			}
			i += totalStars
			continue
		}

		switch stars {
		case 1:
			buf.WriteString("<i>")
			buf.WriteString(inner)
			buf.WriteString("</i>")
		case 2:
			buf.WriteString("<b>")
			buf.WriteString(inner)
			buf.WriteString("</b>")
		case 3:
			buf.WriteString("<b><i>")
			buf.WriteString(inner)
			buf.WriteString("</i></b>")
		}

		i = closeStart + stars
	}

	return buf.String()
}

// findMatchingClose finds the position of the matching closing asterisk
// sequence of length n, starting the search at pos. Returns -1 if not found.
// A closing delimiter must not be preceded by whitespace (standard markdown
// rule: right-flanking).
func findMatchingClose(text string, pos, n int) int {
	for j := pos; j <= len(text)-n; j++ {
		// Skip escaped asterisks
		if text[j] == '\\' && j+1 < len(text) && text[j+1] == '*' {
			j++ // skip the escaped char
			continue
		}
		if text[j] != '*' {
			continue
		}

		// Count consecutive asterisks
		count := 0
		for j+count < len(text) && text[j+count] == '*' {
			count++
		}

		if count == n {
			// Right-flanking check: character before closing delimiter must not
			// be whitespace (and must exist — j > pos guarantees content).
			if j > pos && text[j-1] != ' ' && text[j-1] != '\t' {
				return j
			}
		}

		// Skip past this asterisk run to avoid re-matching
		j += count - 1
	}
	return -1
}
