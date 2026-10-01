package types

import "strings"

// unnumberedSectionPrefixes are divider titles that consulting decks never
// number: back matter (appendix, backup, annex) and the closing Q&A / thanks
// dividers. Matching is case-insensitive on the leading word(s) of the title,
// so "Appendix A: Detailed financials" and "Backup slides" both qualify.
var unnumberedSectionPrefixes = []string{
	"appendix", "appendices", "backup", "back-up", "back up", "annex",
	"q&a", "q & a", "questions", "thank you", "thanks",
}

// AppendixSectionLabel is the running section name (tracker / footer crumb)
// for the slides of an appendix section: the title itself when it already
// says Appendix / Backup / …, otherwise "Appendix: <title>", so a reader
// always sees they are in back matter (go-slide-creator-deb2h).
func AppendixSectionLabel(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Appendix"
	}
	if IsUnnumberedSectionTitle(title) {
		return title
	}
	return "Appendix: " + title
}

// IsUnnumberedSectionTitle reports whether a section divider with this title
// is back matter that must not receive (or consume) an automatic chapter
// number (go-slide-creator-7ldh9). An "Appendix" divider at the end of a
// three-chapter deck used to render as "04" — or "Appendix 01" in the
// template's numeral slot.
func IsUnnumberedSectionTitle(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return false
	}
	for _, p := range unnumberedSectionPrefixes {
		if !strings.HasPrefix(t, p) {
			continue
		}
		rest := t[len(p):]
		if rest == "" {
			return true
		}
		// Whole-word prefix only: "Annexation strategy" is a chapter.
		switch rest[0] {
		case ' ', ':', '-', '?', '!', '.', ',', '/', '(', '\t':
			return true
		}
		if strings.HasPrefix(rest, "—") || strings.HasPrefix(rest, "–") {
			return true
		}
	}
	return false
}
