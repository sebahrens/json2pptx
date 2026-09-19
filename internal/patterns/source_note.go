package patterns

import "strings"

// One source convention for the whole engine (go-slide-creator-7eib).
//
// slide.source used to render as "Source: <text>" at 8pt in a raw #888888,
// right-aligned in the chrome band, while chart-insights-split drew its own
// values.source verbatim at 9pt dk1 italic on the left, and stat-hero at 10pt
// centred. One deck therefore showed sources in three sizes, two alignments
// and two corners depending on which pattern happened to own the slide — and
// an agent that wrote "Source: …" into the slide-level field got
// "Source: Source: …" on a pattern slide.

const (
	// SourceNoteSizePt is the one size a source line is set in.
	//
	// The number is decided by the tighter of the two surfaces: shape_grid
	// text is floored at 12pt by the renderer (shapegrid.MinTextSizePt), so a
	// pattern that asks for 9 draws 12 anyway. Setting the chrome band to
	// anything smaller would leave the two surfaces at different sizes in the
	// same deck, which is the thing this convention exists to prevent — and
	// the old 8pt band was reported as barely legible at 100% in any case.
	SourceNoteSizePt = 12.0
	// SourceNoteScheme is the source line's colour. The slide-level note used
	// a raw #888888, which is off-palette on every template; dk2 is the
	// template's own structural dark, muted against the body text without
	// leaving the theme.
	SourceNoteScheme = "dk2"
	// SourceNoteAlign is the alignment of a source line: left, under whatever
	// it attributes.
	SourceNoteAlign = "l"
)

// SourceNoteText labels a source line without stuttering when the author
// already wrote the label. It is the one place that decides whether a source
// carries the "Source: " prefix, so the two surfaces cannot disagree
// (go-slide-creator-xg48 fixed the stutter for the slide-level note;
// go-slide-creator-7eib made the pattern surfaces use the same rule).
func SourceNoteText(sourceText string) string {
	trimmed := strings.TrimSpace(sourceText)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "source:") {
		return trimmed
	}
	return "Source: " + trimmed
}
