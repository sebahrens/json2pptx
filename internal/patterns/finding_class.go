package patterns

// FindingClass categorizes a finding code so a visual-QA report can separate a
// "poor pattern choice" from a "rendering" problem (and from a "content"
// authoring problem). The split a reviewer cares about most is pattern-choice
// vs rendering:
//
//   - pattern_choice — the author picked a layout family ill-suited to the
//     content (a single-row flow stretched to fill the slide, an agenda drawn
//     as a flowchart). The engine rendered exactly what it was asked to; the
//     fix is to swap to a better-suited pattern, not to patch the renderer.
//   - rendering — the engine could not fit, had to adjust, or produced a
//     geometry artifact for what was authored (overflow, contrast auto-fix,
//     clamping, a rotated band that renders mis-shaped). These are engine-side
//     signals: shrink/split, or a genuine rendering bug to fix.
//   - content — an authoring-text problem independent of pattern or render
//     (headline/body too long, missing alt text, duplicate title).
//
// Codes not explicitly mapped default to "rendering". A new code therefore
// lands in the conservative bucket until it is classified; the per-code tests
// pin the QA-relevant codes so the default cannot silently swallow a
// pattern-choice smell.
const (
	FindingClassPatternChoice = "pattern_choice"
	FindingClassRendering     = "rendering"
	FindingClassContent       = "content"
)

// patternChoiceCodes are the finding codes that signal a layout/pattern
// mismatch — "you chose the wrong pattern for this content", not a render bug.
var patternChoiceCodes = map[string]bool{
	ErrCodeSparseSingleRowFlow:  true,
	ErrCodeOvertallFlowLane:     true,
	ErrCodeFlowDiamondNoContent: true,
	ErrCodeTocFlowchartVocab:    true,
	ErrCodeWrongPattern:         true,
	ErrCodePatternOvercrowded:   true,
	ErrCodePatternUnderfilled:   true,
	ErrCodeSparseLayout:         true,
	ErrCodeCellUnderfilled:      true,
}

// contentCodes are authoring-text findings that are neither a pattern-choice
// mismatch nor a render-side adjustment.
var contentCodes = map[string]bool{
	ErrCodeHeadlineTooLong:   true,
	ErrCodeBodyTooLong:       true,
	ErrCodeBulletNestingDeep: true,
	ErrCodeMissingAltText:    true,
	ErrCodeDuplicateTitle:    true,
	ErrCodeTakeawayMissing:   true,
}

// FindingClass returns the QA class for a finding code: one of
// FindingClassPatternChoice, FindingClassContent, or (default)
// FindingClassRendering. It lets a QA report state, per finding, whether the
// problem is a poor pattern choice or a rendering issue.
func FindingClass(code string) string {
	switch {
	case patternChoiceCodes[code]:
		return FindingClassPatternChoice
	case contentCodes[code]:
		return FindingClassContent
	default:
		return FindingClassRendering
	}
}
