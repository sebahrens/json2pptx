package patterns

import "errors"

// Layout-balance advisories (go-slide-creator-u9xfy). Emitted by the preflight
// fit pass; both are review-level and never block.
//
//   - VerticalImbalance: a grid / pattern slide's content sits against one
//     edge of the content zone and leaves a large empty band on the other.
//   - SparsePlaceholder: a bullet / text body fills only a small share of a
//     large body placeholder, so a few lines sit at the top of an empty slide.
const (
	ErrCodeVerticalImbalance = "VERTICAL_IMBALANCE"
	ErrCodeSparsePlaceholder = "SPARSE_PLACEHOLDER"
)

var (
	ErrVerticalImbalance = errors.New("slide content leaves a large empty band on one side")
	ErrSparsePlaceholder = errors.New("body text fills a small share of its placeholder")
)

func init() {
	codeSentinel[ErrCodeVerticalImbalance] = ErrVerticalImbalance
	codeSentinel[ErrCodeSparsePlaceholder] = ErrSparsePlaceholder

	patternChoiceCodes[ErrCodeVerticalImbalance] = true
	patternChoiceCodes[ErrCodeSparsePlaceholder] = true
	contentCodes[ErrCodeTitleNotAction] = true
	contentCodes[ErrCodeTitleTooLong] = true

	findingMetaRegistry[ErrCodeVerticalImbalance] = FindingMeta{
		Code:        ErrCodeVerticalImbalance,
		Summary:     "A grid or pattern slide's content hugs one edge of the content zone, leaving a large empty band on the other side.",
		Severity:    "review",
		WhenEmitted: "Preflight resolves the slide's grid geometry and finds the empty band above or below the visible content is at least 1.25in and at least 30% of the content-zone height larger than the band on the opposite side. Slides that also carry body placeholder content, and slides already reported as SLIDE_UNDERUSED, are skipped; on a slide with a takeaway the band below the grid is not counted (the takeaway renders there).",
		RemediationSteps: []string{
			"Centre the block vertically (shape_grid vertical_align) or size the grid to its content.",
			"Use the empty band: add a supporting zone, takeaway or source line.",
			"Or choose a pattern whose rows fill the zone for this amount of content.",
		},
		RelatedCodes: []string{ErrCodeSlideUnderused, ErrCodeSparseFill},
	}
	findingMetaRegistry[ErrCodeSparsePlaceholder] = FindingMeta{
		Code:        ErrCodeSparsePlaceholder,
		Summary:     "A slide's only body text fills under 25% of a large body placeholder.",
		Severity:    "review",
		WhenEmitted: "Preflight measures the text of a slide whose single content item besides the title is a text / bullets body, against the resolved body placeholder (at least 35% of the slide height) at the placeholder font size. The finding fires when the measured text height is under 25% of the placeholder height — e.g. four short bullets stuck to the top of an empty slide.",
		RemediationSteps: []string{
			"Turn the points into a visual pattern (card-grid, icon-row, labeled-rows) that uses the slide.",
			"Add the evidence or takeaway the points are missing.",
			"Or merge the slide with its neighbour.",
		},
		RelatedCodes: []string{ErrCodeSlideUnderused, ErrCodeSlideNearlyEmpty},
	}
	findingMetaRegistry[ErrCodeTitleNotAction] = FindingMeta{
		Code:        ErrCodeTitleNotAction,
		Summary:     "A content slide's title names a topic instead of stating the slide's point.",
		Severity:    "review",
		WhenEmitted: "The title of a content slide (not a cover, section, agenda or next-steps slide, and not a short label on the deck's last slide) is (a) longer than 15 words, (b) a stock label (Overview, Summary, Executive summary, Key metrics, Next steps, Background, Recommendations, ... or \"<topic> overview / analysis / update\"), or (c) has no digit and no verb from the action-title lexicon (an -ed / -s form counts). Check (c) skips titles of six or more words written in sentence case, and slides that state their point in a takeaway (the DeckSpec title + takeaway convention); (a) and (b) apply regardless. fix.params.reason names the check (too_long, stock_label, no_verb_or_number). Review weight (5 points); feeds the quality gate's require_action_titles criterion.",
		RemediationSteps: []string{
			"Rewrite the title as the claim the slide proves — a full sentence of at most 15 words with a verb, carrying its number.",
			"Keep the topic as a section divider or eyebrow if it helps navigation.",
		},
		ExampleBefore: `{"placeholder_id":"title","type":"text","text_value":"Market Overview"}`,
		ExampleAfter:  `{"placeholder_id":"title","type":"text","text_value":"Mid-market demand doubled while enterprise stalled"}`,
		RelatedCodes:  []string{ErrCodeTitleTooLong, ErrCodeDuplicateTitle},
	}
	findingMetaRegistry[ErrCodeTitleTooLong] = FindingMeta{
		Code:        ErrCodeTitleTooLong,
		Summary:     "A content slide's title renders on more than two lines.",
		Severity:    "review",
		WhenEmitted: "The measured title check finds the title fits its placeholder, but at the size generation writes (the template size times any baked autofit scale) it wraps to three or more lines. Content slides only; cover and section titles are exempt. It replaces title_wraps for that title, and TITLE_OVERFLOW / shrink-level title_wraps still own titles that do not fit.",
		RemediationSteps: []string{
			"Tighten the headline to two lines; fix.params.max_chars is the longest word prefix that fits in two lines.",
			"Move supporting detail into the body or a takeaway.",
		},
		RelatedCodes: []string{ErrCodeTitleWraps, ErrCodeTitleOverflow, ErrCodeTitleNotAction},
	}
}
