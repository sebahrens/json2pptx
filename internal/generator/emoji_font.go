package generator

import (
	"strings"
	"unicode"
)

// emojiFontXML is the OOXML fragment injected into run properties for emoji
// characters. It sets latin, East Asian, and Complex Script typefaces to
// "Segoe UI Emoji" so PowerPoint renders pictographs instead of blank spaces.
const emojiFontXML = `<a:latin typeface="Segoe UI Emoji" panose="01000000000000000000"/>` +
	`<a:ea typeface="Segoe UI Emoji"/>` +
	`<a:cs typeface="Segoe UI Emoji"/>`

// isEmoji reports whether r is a Unicode emoji or symbol codepoint that
// typically requires an emoji-capable font for rendering. The ranges cover
// the most common emoji blocks; variation selectors and ZWJ are included so
// they stay attached to surrounding emoji runs.
func isEmoji(r rune) bool {
	// Miscellaneous Symbols
	if r >= 0x2600 && r <= 0x26FF {
		return true
	}
	// Dingbats
	if r >= 0x2700 && r <= 0x27BF {
		return true
	}
	// Variation Selectors (keep with adjacent emoji)
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	// Supplemental Symbols and Pictographs, Emoticons, Transport, etc.
	if r >= 0x1F300 && r <= 0x1FAFF {
		return true
	}
	// Regional Indicator Symbols (flags)
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}
	// Zero Width Joiner (used in composite emoji sequences)
	if r == 0x200D {
		return true
	}
	// Combining Enclosing Keycap
	if r == 0x20E3 {
		return true
	}
	// Miscellaneous Symbols and Arrows
	if r >= 0x2B05 && r <= 0x2B55 {
		return true
	}
	// Copyright, Registered, Trade Mark
	if r == 0x00A9 || r == 0x00AE || r == 0x2122 {
		return true
	}
	return false
}

// containsEmoji reports whether s contains any emoji codepoints.
func containsEmoji(s string) bool {
	for _, r := range s {
		if isEmoji(r) {
			return true
		}
	}
	return false
}

// splitEmojiRuns post-processes a slice of runXML, splitting any run that
// mixes emoji and non-emoji text into separate runs. Emoji-only runs get
// the emoji font injected into their run properties. Runs that contain no
// emoji are returned unchanged.
func splitEmojiRuns(runs []runXML) []runXML {
	var result []runXML
	for _, run := range runs {
		if !containsEmoji(run.Text) {
			result = append(result, run)
			continue
		}

		// Split text into contiguous emoji / non-emoji segments.
		segments := splitTextByEmoji(run.Text)
		for _, seg := range segments {
			r := runXML{
				Text: seg.text,
			}

			// Clone run properties.
			if run.RunProperties != nil {
				rp := &runPropertiesXML{
					Lang:   run.RunProperties.Lang,
					Bold:   run.RunProperties.Bold,
					Italic: run.RunProperties.Italic,
					Inner:  run.RunProperties.Inner,
				}
				if seg.emoji {
					rp.Inner = injectEmojiFont(rp.Inner)
				}
				r.RunProperties = rp
			} else if seg.emoji {
				r.RunProperties = &runPropertiesXML{
					Lang:  "en-US",
					Inner: emojiFontXML,
				}
			}

			result = append(result, r)
		}
	}
	return result
}

// emojiSegment is a contiguous run of either emoji or non-emoji text.
type emojiSegment struct {
	text  string
	emoji bool
}

// splitTextByEmoji splits s into segments where each segment is either all
// emoji or all non-emoji. Whitespace adjacent to emoji is kept with the
// non-emoji segment so that spacing inherits the normal theme font.
func splitTextByEmoji(s string) []emojiSegment {
	if s == "" {
		return nil
	}

	var segments []emojiSegment
	var buf strings.Builder
	currentIsEmoji := false
	first := true

	for _, r := range s {
		re := isEmoji(r)
		// Whitespace stays with non-emoji so theme font metrics apply to spaces.
		if unicode.IsSpace(r) {
			re = false
		}

		if first {
			currentIsEmoji = re
			first = false
		} else if re != currentIsEmoji {
			// Flush segment.
			segments = append(segments, emojiSegment{
				text:  buf.String(),
				emoji: currentIsEmoji,
			})
			buf.Reset()
			currentIsEmoji = re
		}
		buf.WriteRune(r)
	}

	if buf.Len() > 0 {
		segments = append(segments, emojiSegment{
			text:  buf.String(),
			emoji: currentIsEmoji,
		})
	}

	return segments
}

// injectEmojiFont appends the emoji font XML to an existing Inner fragment.
// If the fragment already contains a latin typeface declaration, it is
// replaced to avoid duplicate font specifications.
func injectEmojiFont(inner string) string {
	// Strip existing font declarations that would conflict.
	inner = stripExistingFontElements(inner)
	if inner == "" {
		return emojiFontXML
	}
	return inner + emojiFontXML
}

// stripExistingFontElements removes <a:latin.../>, <a:ea.../>, and <a:cs.../>
// self-closing elements from an XML fragment so emoji font can replace them.
func stripExistingFontElements(xml string) string {
	xml = stripSelfClosingElement(xml, "a:latin")
	xml = stripSelfClosingElement(xml, "a:ea")
	xml = stripSelfClosingElement(xml, "a:cs")
	return xml
}

// stripSelfClosingElement removes a self-closing XML element by tag name.
// E.g., stripSelfClosingElement(`<a:latin typeface="Arial"/>`, "a:latin") returns "".
func stripSelfClosingElement(xml, tag string) string {
	for {
		start := strings.Index(xml, "<"+tag+" ")
		if start == -1 {
			start = strings.Index(xml, "<"+tag+"/>")
		}
		if start == -1 {
			return xml
		}
		end := strings.Index(xml[start:], "/>")
		if end == -1 {
			return xml
		}
		xml = xml[:start] + xml[start+end+2:]
	}
}
