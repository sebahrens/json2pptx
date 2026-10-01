package deckplan

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxDerivedTitleLen is the length past which TitleFromClause looks for a
// clause boundary to stop at. A brief fact is already one clause, so most
// titles are the whole fact.
const maxDerivedTitleLen = 90

// TitleFromClause turns a brief clause (a routed fact, the deck topic) into a
// slide title (go-slide-creator-tu35a): sentence-cased, and never cut in the
// middle of a clause. make_deck used to lower-case nothing and cut every title
// at 60 runes, shipping "revenue grew 18% YoY to $42M. gross margin expanded
// to 61..." as a headline. A clause longer than maxDerivedTitleLen stops at
// its last inner boundary (", ", "; ", " — ", " (") before the limit; one with
// no such boundary is kept whole up to maxFactLen.
func TitleFromClause(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, " .;:,")
	if utf8.RuneCountInString(s) > maxDerivedTitleLen {
		cut := -1
		limit := len(string([]rune(s)[:maxDerivedTitleLen]))
		for _, sep := range []string{"; ", " — ", " – ", ", ", " ("} {
			if i := strings.LastIndex(s[:limit], sep); i > cut && i > 20 {
				cut = i
			}
		}
		if cut > 0 {
			s = strings.TrimRight(s[:cut], " .;:,")
		} else {
			s = TruncateBrief(s, maxFactLen)
		}
		s = balanceFactBrackets(s)
	}
	return sentenceCase(s)
}

// sentenceCase upper-cases the first letter, leaving the rest as written so
// acronyms and names survive.
func sentenceCase(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || !unicode.IsLower(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// BriefTopic is the brief's first clause: the deck's subject.
func BriefTopic(brief string) string {
	return deckTopic(brief)
}
