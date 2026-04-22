package pptx

import "strings"

// SplitInlineTags parses <b>, <i>, <u> inline formatting tags in a Run's text
// and returns multiple Runs with appropriate Bold/Italic/Underline flags.
// The template Run provides base styling (font size, color, font family, etc.).
// If no tags are found, returns a single-element slice with the original Run.
func SplitInlineTags(base Run) []Run {
	text := base.Text
	if !strings.Contains(text, "<") {
		return []Run{base}
	}

	type formatState struct {
		bold, italic, underline int
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

		// Merge with previous run if formatting matches
		if len(runs) > 0 {
			last := &runs[len(runs)-1]
			if last.Bold == r.Bold && last.Italic == r.Italic && last.Underline == r.Underline {
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

		tag := strings.ToLower(strings.TrimSpace(text[tagStart+1 : tagEnd]))

		switch tag {
		case "b":
			state.bold++
		case "/b":
			if state.bold > 0 {
				state.bold--
			}
		case "i":
			state.italic++
		case "/i":
			if state.italic > 0 {
				state.italic--
			}
		case "u":
			state.underline++
		case "/u":
			if state.underline > 0 {
				state.underline--
			}
		default:
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
