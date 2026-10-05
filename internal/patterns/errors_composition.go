package patterns

import "errors"

// Composition faults (go-slide-creator-wwmod). The score saturated at 100 on
// slides an expert would send back: content in the left half only, flow boxes
// wrapped to six lines of two words, column headers at two sizes. Each is
// measured on the resolved geometry — where the ink sits, how the text wraps,
// what size the writer stores — not looked up by pattern name.
//
//   - HorizontalImbalance: the content sits against one side of the content
//     zone and leaves a wide empty band on the other.
//   - TextWrapsNarrow: a box so narrow that a paragraph breaks into a tall
//     column of two- and three-word lines.
//   - SiblingSizeMismatch: cells that are peers in one row (column headers,
//     card titles) render at visibly different font sizes.
const (
	ErrCodeHorizontalImbalance = "HORIZONTAL_IMBALANCE"
	ErrCodeTextWrapsNarrow     = "TEXT_WRAPS_NARROW"
	ErrCodeSiblingSizeMismatch = "SIBLING_SIZE_MISMATCH"
)

var (
	ErrHorizontalImbalance = errors.New("slide content leaves a wide empty band on one side")
	ErrTextWrapsNarrow     = errors.New("text wraps into a tall column of very short lines")
	ErrSiblingSizeMismatch = errors.New("sibling cells render at different font sizes")
)

func init() {
	codeSentinel[ErrCodeHorizontalImbalance] = ErrHorizontalImbalance
	codeSentinel[ErrCodeTextWrapsNarrow] = ErrTextWrapsNarrow
	codeSentinel[ErrCodeSiblingSizeMismatch] = ErrSiblingSizeMismatch

	patternChoiceCodes[ErrCodeHorizontalImbalance] = true
	patternChoiceCodes[ErrCodeTextWrapsNarrow] = true

	findingMetaRegistry[ErrCodeHorizontalImbalance] = FindingMeta{
		Code:        ErrCodeHorizontalImbalance,
		Summary:     "A grid or pattern slide's content fills one side of the content zone and leaves the other side empty.",
		Severity:    "review",
		WhenEmitted: "Preflight resolves the slide's grid geometry and measures where the visible content (filled shapes, text blocks at their measured width, images, charts) sits. The finding fires when the empty band left or right of the content is at least 40% of the content-zone width and at least 30% of that width larger than the band on the other side — rows of short text that end mid-slide. Slides that carry body placeholder content, and slides already reported as SLIDE_UNDERUSED or VERTICAL_IMBALANCE, are skipped. Costs 25 points on its slide; it does not block by itself.",
		RemediationSteps: []string{
			"Use the empty side: add the evidence, a chart, an image or a so-what column (compose, chart-insights-split, image-text-split, text-sidebar).",
			"Or choose a pattern that spreads this content across the width (card-grid, icon-row, kpi-3up for short items).",
			"Or lengthen the row text so each row states its point in a full sentence.",
		},
		RelatedCodes: []string{ErrCodeVerticalImbalance, ErrCodeSlideUnderused},
	}
	findingMetaRegistry[ErrCodeTextWrapsNarrow] = FindingMeta{
		Code:        ErrCodeTextWrapsNarrow,
		Summary:     "A paragraph in a narrow box wraps into five or more lines of two or three words.",
		Severity:    "review",
		WhenEmitted: "Preflight wraps every shape's text at the width its geometry and insets leave, with the same font metrics as TEXT_EXCEEDS_SHAPE. The finding fires when one paragraph needs 5 or more lines and averages at most 3 words per line — eight flow boxes across a slide, each a column of fragments. It is reported once per slide; fix.params.cells lists every box, paths the authored value behind each (the pattern value on a pattern slide), max_lines the tallest, max_words the paragraph length that fits in 4 lines at that width and max_boxes how many boxes the row holds once each is wide enough for its text to fit in 4 lines. Costs 25 points on its slide and counts toward the gate's problem-slide share.",
		RemediationSteps: []string{
			"Cut each box to fix.params.max_words words or fewer (a label, not a sentence).",
			"Or use fewer boxes so each is wider: at most fix.params.max_boxes on the row (process-flow puts 7-8 steps on two rows unless overrides.rows is 1).",
			"Or switch to a layout that gives each step a full-width row (numbered-step-strip, labeled-rows) and keep the sentences.",
		},
		RelatedCodes: []string{ErrCodeTextExceedsShape, ErrCodeBodyTooLong},
	}
	findingMetaRegistry[ErrCodeSiblingSizeMismatch] = FindingMeta{
		Code:        ErrCodeSiblingSizeMismatch,
		Summary:     "Cells that are peers in one row render at visibly different font sizes.",
		Severity:    "review",
		WhenEmitted: "Preflight reads the autofit scale generation stores on each shape and compares the cells of one grid row that were authored alike (same paragraph sizes and weights — column headers, card titles). The finding fires when the smallest renders more than 8% below the largest: the longer labels were shrunk to fit and the short ones were not. A one-line label in a box that holds one line and carries no stored scale is also measured for a renderer whose font runs 12% wider: within that margin of its box it wraps there and is shrunk by the renderer, and the message then says the size it \"is written at\". fix.params.cells lists the row's cells with written_pt (the size in the slide XML) and rendered_pt (the size expected of a renderer). Costs 25 points on its slide and counts toward the gate's problem-slide share.",
		RemediationSteps: []string{
			"Shorten the longest labels in the row to the length of the shortest (fix.params.longest names it).",
			"Or use fewer columns so every label fits at the authored size.",
			"Or set one explicit text size on all of the row's cells.",
			"For a label written at full size (written_pt above rendered_pt) on a raw grid: make the row tall enough for two lines, so a wrap costs a line and not the size.",
		},
		RelatedCodes: []string{ErrCodeTextBelowReadableMin, ErrCodeTextExceedsShape},
	}
}
