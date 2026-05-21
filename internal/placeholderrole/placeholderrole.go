// Package placeholderrole holds the low-level lexical alias and name-hint sets
// used to recognise a placeholder's intent from its ID. It is deliberately a
// leaf package (stdlib-only) so that both internal/template (canonical
// classification) and internal/generator (placeholder resolution) can share a
// single source of truth without internal/template ever importing
// internal/generator.
//
// This package owns only the lexical primitives — the alias IDs, the name
// hints, and the helpers that match them. The intent-level role enums and the
// classifier that combines these signals with OOXML type and font size live in
// internal/template (ClassifyPlaceholderRole), which depends on internal/types.
package placeholderrole

import "strings"

// SectionNumberMinFontSize is the font-size threshold (hundredths of a point)
// above which a body placeholder is treated as a decorative section number.
// Observed across templates: titles top out near 80pt while section numbers run
// 96–208pt, so 90pt cleanly separates the two.
const SectionNumberMinFontSize = 9000

// sectionNumberAliasIDs lists placeholder IDs that resolve to the section number
// placeholder: a large-font shape intended for decorative numbering ("01",
// "02", etc.).
var sectionNumberAliasIDs = map[string]bool{
	"section_number": true,
	"section_no":     true,
	"large_number":   true,
}

// Lower-cased substring hints used to refine a role from a placeholder's name.
var (
	EyebrowHints  = []string{"eyebrow", "kicker"}
	SubtitleHints = []string{"subtitle", "sub title", "sub-title"}
	DateHints     = []string{"date"}
	PageNumHints  = []string{"slide number", "slidenum", "slide_number", "page number", "page_number", "pagenum", "pagenumber"}
	FooterHints   = []string{"footer", "ftr"}
)

// IsSectionNumberAlias reports whether id (case-insensitive, whitespace-trimmed)
// is one of the section-number alias IDs (section_number, section_no,
// large_number).
func IsSectionNumberAlias(id string) bool {
	return sectionNumberAliasIDs[strings.ToLower(strings.TrimSpace(id))]
}

// ContainsAnyHint reports whether id contains any of hints as a substring.
// Callers are expected to pass an already lower-cased id.
func ContainsAnyHint(id string, hints []string) bool {
	for _, h := range hints {
		if strings.Contains(id, h) {
			return true
		}
	}
	return false
}
