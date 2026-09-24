package slides

import (
	"strings"
	"unicode"
)

// FoldConclusion keeps two authored conclusions in one visual callout. Repeated
// wording is shown only once; distinct wording is retained in reading order.
func FoldConclusion(primary, takeaway string) string {
	primary = strings.TrimSpace(primary)
	takeaway = strings.TrimSpace(takeaway)
	if primary == "" {
		return takeaway
	}
	if takeaway == "" || DuplicateConclusion(primary, takeaway) {
		return primary
	}
	return primary + " — " + takeaway
}

// DuplicateConclusion tolerates case, punctuation and minor whitespace edits,
// without treating a short shared phrase as the same decision.
func DuplicateConclusion(a, b string) bool {
	a, b = normalizeConclusion(a), normalizeConclusion(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(short) < 24 || float64(len(short))/float64(len(long)) < 0.85 || !strings.Contains(" "+long+" ", " "+short+" ") {
		return false
	}
	// A near-duplicate with an added negation is a different decision, not
	// disposable repetition ("fund the pod" versus "do not fund the pod").
	extra := strings.TrimSpace(strings.Replace(" "+long+" ", " "+short+" ", " ", 1))
	for _, word := range strings.Fields(extra) {
		switch word {
		case "no", "not", "never", "without", "avoid", "stop", "dont":
			return false
		}
	}
	return true
}

func normalizeConclusion(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}
