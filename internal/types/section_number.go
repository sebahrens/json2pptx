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

// appendixSectionPrefixes are the back-matter divider titles that open an
// appendix: the unnumbered titles minus the closing Q&A / thanks dividers.
var appendixSectionPrefixes = []string{
	"appendix", "appendices", "backup", "back-up", "back up", "annex",
}

// IsAppendixSectionTitle reports whether a divider title (or a running
// section label such as "Appendix: Detailed financials") names back matter:
// Appendix, Appendices, Backup or Annex as a whole leading word. Q&A and
// thank-you dividers are unnumbered but are not an appendix.
func IsAppendixSectionTitle(title string) bool {
	if !IsUnnumberedSectionTitle(title) {
		return false
	}
	t := strings.ToLower(strings.TrimSpace(title))
	for _, p := range appendixSectionPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// BackMatterProbe describes one slide for BackMatterMask.
type BackMatterProbe struct {
	// Divider is true for a section divider slide.
	Divider bool
	// AppendixDivider is true for a divider that opens back matter: one
	// titled Appendix / Backup / Annex, or one from a section marked
	// appendix: true.
	AppendixDivider bool
	// Crumb is the slide's running section label (structure decks carry
	// "Appendix: <title>" on appendix slides).
	Crumb string
}

// BackMatterMask reports, per slide, whether it is appendix back matter
// (go-slide-creator-khzni): everything from an appendix divider up to the
// next ordinary divider, and any slide whose running section names an
// appendix. Deck-rhythm and monotony checks skip these slides: backup pages
// are reference material, not part of the argument's visual rhythm.
func BackMatterMask(probes []BackMatterProbe) []bool {
	mask := make([]bool, len(probes))
	in := false
	for i, p := range probes {
		if p.Divider {
			in = p.AppendixDivider
		}
		mask[i] = in || (p.Crumb != "" && IsAppendixSectionTitle(p.Crumb))
	}
	return mask
}
