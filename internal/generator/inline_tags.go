package generator

import "strings"

// TextRun represents a segment of text with optional formatting.
type TextRun struct {
	Text      string // The text content
	Bold      bool   // Whether this run should be bold
	Italic    bool   // Whether this run should be italic
	Underline bool   // Whether this run should be underlined
}

// ParseInlineTags parses HTML-like inline tags in text and returns a slice of
// TextRun with appropriate formatting.
//
// Supported tags:
//   - <b>...</b> for bold
//   - <i>...</i> for italic
//   - <u>...</u> for underline (mapped to Bold since TextRun has no Underline)
//   - Nesting is supported: <b><i>bold italic</i></b>
//
// Tags are matched case-insensitively. If no tags are found, returns a single
// TextRun with the original text. Unmatched or unknown tags are preserved as
// literal text. Adjacent runs with identical formatting are merged.
func ParseInlineTags(text string) []TextRun {
	if text == "" {
		return nil
	}

	// Fast path: no tags at all
	if !strings.Contains(text, "<") {
		return []TextRun{{Text: text}}
	}

	var runs []TextRun
	bold := 0
	italic := 0
	underline := 0
	pos := 0

	for pos < len(text) {
		// Find the next '<'
		tagStart := strings.IndexByte(text[pos:], '<')
		if tagStart == -1 {
			// No more tags — emit remaining text
			if pos < len(text) {
				runs = appendMergedRun(runs, text[pos:], bold > 0, italic > 0, underline > 0)
			}
			break
		}
		tagStart += pos

		// Emit text before the tag
		if tagStart > pos {
			runs = appendMergedRun(runs, text[pos:tagStart], bold > 0, italic > 0, underline > 0)
		}

		// Find the closing '>'
		tagEnd := strings.IndexByte(text[tagStart:], '>')
		if tagEnd == -1 {
			// No closing '>' — emit rest as literal text
			runs = appendMergedRun(runs, text[tagStart:], bold > 0, italic > 0, underline > 0)
			break
		}
		tagEnd += tagStart

		tag := strings.ToLower(strings.TrimSpace(text[tagStart+1 : tagEnd]))

		switch tag {
		case "b":
			bold++
		case "/b":
			if bold > 0 {
				bold--
			}
		case "i":
			italic++
		case "/i":
			if italic > 0 {
				italic--
			}
		case "u":
			underline++
		case "/u":
			if underline > 0 {
				underline--
			}
		default:
			// Unknown tag — preserve as literal text
			runs = appendMergedRun(runs, text[tagStart:tagEnd+1], bold > 0, italic > 0, underline > 0)
		}

		pos = tagEnd + 1
	}

	if len(runs) == 0 {
		return nil
	}

	return runs
}

// appendMergedRun appends a TextRun to the slice, merging with the last run if
// formatting matches (to avoid fragmented runs with identical styles).
func appendMergedRun(runs []TextRun, text string, bold, italic, underline bool) []TextRun {
	if text == "" {
		return runs
	}
	if len(runs) > 0 {
		last := &runs[len(runs)-1]
		if last.Bold == bold && last.Italic == italic && last.Underline == underline {
			last.Text += text
			return runs
		}
	}
	return append(runs, TextRun{
		Text:      text,
		Bold:      bold,
		Italic:    italic,
		Underline: underline,
	})
}
