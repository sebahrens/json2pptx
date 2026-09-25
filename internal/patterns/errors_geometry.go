package patterns

import "errors"

// Deterministic geometry finding codes (go-slide-creator-t5gw). Emitted by the
// preflight fit-report pass from resolved shape_grid geometry — including the
// grids that named patterns expand to — without rendering. Most are advisory;
// predicted TEXT_EXCEEDS_SHAPE at >=2x width overflow is a warning.
//
//   - TextExceedsShape: a word in a shape's text is wider than the text area
//     the shape's preset geometry leaves after insets (chevrons, diamonds,
//     ellipses keep far less than the bounding box), so it is broken
//     mid-word or clipped by the shape outline.
//   - SparseFill: a filled shape covering more than 10% of the slide holds
//     text that occupies under 20% of its area — a large, mostly empty
//     coloured box.
//   - SlideUnderused: the union of visible grid ink (filled shapes, text
//     blocks, media) covers too little of the layout's safe content area, or
//     a KPI row leaves an excessive vertical band empty.
const (
	ErrCodeTextExceedsShape = "TEXT_EXCEEDS_SHAPE"
	ErrCodeSparseFill       = "SPARSE_FILL"
	ErrCodeSlideUnderused   = "SLIDE_UNDERUSED"
)

// Sentinels for errors.Is matching.
var (
	ErrTextExceedsShape = errors.New("text is wider than the shape's text area")
	ErrSparseFill       = errors.New("large filled shape is mostly empty")
	ErrSlideUnderused   = errors.New("slide content covers too little of the safe area")
)

func init() {
	codeSentinel[ErrCodeTextExceedsShape] = ErrTextExceedsShape
	codeSentinel[ErrCodeSparseFill] = ErrSparseFill
	codeSentinel[ErrCodeSlideUnderused] = ErrSlideUnderused

	findingMetaRegistry[ErrCodeTextExceedsShape] = FindingMeta{
		Code:        ErrCodeTextExceedsShape,
		Summary:     "A word in a shape's text is wider than the text area left by the shape's geometry and insets.",
		Severity:    "shrink_or_split",
		WhenEmitted: "Preflight measures the widest word of each shape_grid text cell (template body font, rendered size) against the preset geometry's text rectangle (e.g. a chevron keeps width - 2 x notch depth) minus text insets. The actual action is review below 2x overflow and shrink_or_split at 2x or more.",
		RemediationSteps: []string{
			"When even one glyph cannot fit, patch the pattern's max_height_pct downward or change pointed step types to step; for a raw grid, use wider geometry or fewer columns. Shortening alone cannot work.",
			"For smaller deficits, shorten the label or lower the text size, then rerun fit preflight.",
			"Give the row more width (fewer columns) so each shape is wider.",
		},
		ExampleBefore: `{"geometry":"chevron","text":{"paragraphs":[{"content":"01"},{"content":"Onboarding"}]}}  // notch leaves ~0pt of text width`,
		ExampleAfter:  `{"geometry":"homePlate","text":{"paragraphs":[{"content":"01"},{"content":"Onboard"}]}}`,
		RelatedCodes:  []string{ErrCodeFitOverflow, ErrCodeSparseFill},
	}
	findingMetaRegistry[ErrCodeSparseFill] = FindingMeta{
		Code:        ErrCodeSparseFill,
		Summary:     "A large filled shape (>10% of the slide) holds text covering under 20% of its area.",
		Severity:    "review",
		WhenEmitted: "Preflight estimates the wrapped text block area of each filled shape_grid shape and compares it with the shape area.",
		RemediationSteps: []string{
			"Add supporting detail to the shape, or cap the grid height (bounds / max_height_pct) so the box shrinks to its text.",
			"For a DeckSpec slide, add relevant detail, choose a denser slide kind, or merge the slide with a related one; raw grid bounds are not DeckSpec fields.",
			"Switch to a compact pattern variant (e.g. process-flow-compact, kpi-inline) or an unfilled text layout.",
		},
		RelatedCodes: []string{ErrCodeCellUnderfilled, ErrCodeSlideUnderused, ErrCodeSparseLayout},
	}
	findingMetaRegistry[ErrCodeSlideUnderused] = FindingMeta{
		Code:        ErrCodeSlideUnderused,
		Summary:     "The slide's grid content covers too little of the layout's safe content area.",
		Severity:    "review",
		WhenEmitted: "Preflight measures the union area of content rectangles from resolved grid cells (filled shapes with content, estimated text blocks, images, icons, tables, diagrams) inside the safe content area; an explicitly text-empty filled card is excluded, while fill-only chrome remains visible. The threshold is 45% for restrictive author bounds / max_height_pct and 29% for content-sized patterns or uncapped grids. A KPI row also fires for a vertical empty band of at least 0.75in. Explicit full-area bounds are not a cap. fix.params.band_capped_by identifies author, pattern, or none.",
		RemediationSteps: []string{
			"band_capped_by \"author\": raise or remove restrictive bounds / max_height_pct so the grid fills more of the content area.",
			"band_capped_by \"pattern\": the height comes from the content, not from a cap — add detail to the block, pair it with a supporting zone using compose, or choose a denser pattern.",
			"band_capped_by \"none\": no restrictive cap or content-sized pattern set the height — add detail to the raw grid, pair it with supporting content, or merge slides.",
			"For a DeckSpec slide, act on semantic_path: add relevant detail, choose a denser kind, or merge slides rather than editing generated grid bounds.",
			"Or merge this slide's content with a neighbour.",
		},
		RelatedCodes: []string{ErrCodeSparseLayout, ErrCodeSparseFill, ErrCodePatternUnderfilled, ErrCodeSlideNearlyEmpty},
	}
}
