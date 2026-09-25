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
	// ErrCodeTitleTruncated refuses a divider title that cannot fit at 28pt.
	ErrCodeTitleTruncated = "TITLE_TRUNCATED"

	// ErrCodeTextBelowReadableMin fires when text renders (after autofit or
	// predicted renderer shrink) below the deck viewing_mode's readability
	// floor for its text role (go-slide-creator-vbic).
	ErrCodeTextBelowReadableMin = "TEXT_BELOW_READABLE_MIN"
	// ErrCodeTextSizeOffTarget reports template body-size normalization or a
	// fitted result outside the range selected for the content density.
	ErrCodeTextSizeOffTarget = "TEXT_SIZE_OFF_TARGET"
)

// ErrTitleOverflow is the sentinel for ErrCodeTitleOverflow.
var ErrTitleOverflow = errors.New("title does not fit its placeholder at the minimum autofit size")
var ErrTitleTruncated = errors.New("section title cannot fit its divider at the readable floor")

// ErrTextBelowReadableMin is the sentinel for ErrCodeTextBelowReadableMin.
var ErrTextBelowReadableMin = errors.New("text renders below the viewing-mode readability minimum")
var ErrTextSizeOffTarget = errors.New("body text size differs from its density target")

func init() {
	codeSentinel[ErrCodeTitleOverflow] = ErrTitleOverflow
	codeSentinel[ErrCodeTitleTruncated] = ErrTitleTruncated
	codeSentinel[ErrCodeTextBelowReadableMin] = ErrTextBelowReadableMin
	codeSentinel[ErrCodeTextSizeOffTarget] = ErrTextSizeOffTarget

	findingMetaRegistry[ErrCodeTextSizeOffTarget] = FindingMeta{
		Code:             ErrCodeTextSizeOffTarget,
		Summary:          "Body text inherited from a template was normalised to a density target, or autofit shrank it outside that target range.",
		Severity:         "review",
		WhenEmitted:      "Generation and fit-report preflight: after template body size is resolved and content-aware autofit is measured or predicted.",
		RemediationSteps: []string{"For a review finding, shorten or split the body content so it fits within the reported size range.", "An info finding documents an automatic template-size correction; no action is needed."},
		ExampleBefore:    `{"code":"TEXT_SIZE_OFF_TARGET","action":"review","fix":{"kind":"reduce_text","params":{"strategy":"split","actual_pt":13,"min_pt":16}}}`,
		ExampleAfter:     "Split the dense slide until the body text fits at 16pt or larger.",
		RelatedCodes:     []string{ErrCodeTextBelowReadableMin, ErrCodeTextTrimmed},
	}

	findingMetaRegistry[ErrCodeTextBelowReadableMin] = FindingMeta{
		Code:        ErrCodeTextBelowReadableMin,
		Summary:     "Text renders below the readability floor for its role in the deck's viewing_mode (present: 12pt body, 10pt captions, 20pt titles).",
		Severity:    "review",
		WhenEmitted: "Generation: a placeholder's measured autofit shrinks its text below the floor. Preflight (fit report): a shape_grid cell's text would be shrunk by the renderer's autofit below the floor because it exceeds the cell's capacity.",
		RemediationSteps: []string{
			"Shorten the text (fix.params.strategy=shorten) or split the content across slides/cells (strategy=split).",
			"Use a pattern with larger cells, or fewer items per slide.",
			"Set viewing_mode: \"read\" only if the deck is read on screen or printed, not projected.",
		},
		ExampleBefore: `{"code":"TEXT_BELOW_READABLE_MIN","path":"/slides/2/shape_grid/rows/0/cells/1/shape/text","fix":{"kind":"reduce_text","params":{"strategy":"shorten","role":"caption","actual_pt":6.5,"min_pt":10,"viewing_mode":"present"}}}`,
		ExampleAfter:  `Shorten the cell text until the predicted size is at or above min_pt.`,
		RelatedCodes:  []string{ErrCodeFitOverflow, ErrCodeTextTrimmed, ErrCodeReadabilityTrimmed},
	}

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
	findingMetaRegistry[ErrCodeTitleTruncated] = FindingMeta{
		Code:             ErrCodeTitleTruncated,
		Summary:          "A section-divider title cannot fit at the 28pt readability floor without losing text.",
		Severity:         "refuse",
		WhenEmitted:      "Generation and title-fit preflight measure a section title against its divider box at a 28pt floor and normal line spacing.",
		RemediationSteps: []string{"Shorten the title to about fix.params.max_chars characters, or use a divider with a larger title box."},
		ExampleBefore:    `{"code":"TITLE_TRUNCATED","action":"refuse","fix":{"kind":"shorten_title","params":{"max_chars":36,"min_font_pt":28}}}`,
		ExampleAfter:     "Use a shorter section title that fits at 28pt or larger.",
		RelatedCodes:     []string{ErrCodeTitleOverflow, ErrCodeTitleWraps},
	}
}
