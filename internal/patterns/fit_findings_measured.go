package patterns

import "errors"

// Measured-fit finding codes. These are emitted from real font-metric
// measurement against the resolved (inherited) placeholder geometry and text
// style, not from character-count heuristics. They live in their own file so
// the code, sentinel, and metadata stay together.
const (
	// ErrCodeTitleOverflow fires when a title does not fit its resolved title
	// placeholder even at the minimum autofit font scale and line-spacing
	// reduction (go-slide-creator-6cjs).
	ErrCodeTitleOverflow = "TITLE_OVERFLOW"
)

// ErrTitleOverflow is the sentinel for ErrCodeTitleOverflow.
var ErrTitleOverflow = errors.New("title does not fit its placeholder at the minimum autofit size")

func init() {
	codeSentinel[ErrCodeTitleOverflow] = ErrTitleOverflow

	findingMetaRegistry[ErrCodeTitleOverflow] = FindingMeta{
		Code:        ErrCodeTitleOverflow,
		Summary:     "The title does not fit its title placeholder even at the minimum autofit size; lines collide or spill below the title band.",
		Severity:    "shrink_or_split",
		WhenEmitted: "Generation and validate --fit-report measure the title with the template's inherited title style (master font size, all-caps, line spacing) inside the resolved title placeholder, and the text still overflows at the minimum font scale with maximum line-spacing reduction.",
		RemediationSteps: []string{
			"Shorten the title (repair_slide kind=shorten_title); fix.params.max_chars estimates a length that fits.",
			"Move supporting detail into the body or a takeaway line.",
		},
		ExampleBefore: `{"code":"TITLE_OVERFLOW","path":"/slides/1/content/title","fix":{"kind":"shorten_title","params":{"current_chars":104,"max_chars":70,"font_pt":45,"min_font_pt":27}}}`,
		ExampleAfter:  `Set the title text_value to at most fix.params.max_chars characters.`,
		RelatedCodes:  []string{ErrCodeTitleWraps, ErrCodeHeadlineTooLong},
	}
}
