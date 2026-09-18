package patterns

import "errors"

// Deterministic geometry finding codes (go-slide-creator-t5gw). Emitted by the
// preflight fit-report pass from resolved shape_grid geometry — including the
// grids that named patterns expand to — without rendering. All advisory
// (action "review"): they flag slides that validate cleanly but look broken or
// empty once rendered.
//
//   - TextExceedsShape: a word in a shape's text is wider than the text area
//     the shape's preset geometry leaves after insets (chevrons, diamonds,
//     ellipses keep far less than the bounding box), so it is broken
//     mid-word or clipped by the shape outline.
//   - SparseFill: a filled shape covering more than 10% of the slide holds
//     text that occupies under 20% of its area — a large, mostly empty
//     coloured box.
//   - SlideUnderused: the ink bounding box of a slide's grid content (filled
//     shapes, text blocks, media) covers under 45% of the layout's safe
//     content area.
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
		Severity:    "review",
		WhenEmitted: "Preflight measures the widest word of each shape_grid text cell (template body font, rendered size) against the preset geometry's text rectangle (e.g. a chevron keeps width - 2 x notch depth) minus text insets.",
		RemediationSteps: []string{
			"Shorten the label (abbreviate or move detail into the description row).",
			"Lower the text size, or use a geometry with a wider text area (rect / homePlate instead of chevron).",
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
			"Switch to a compact pattern variant (e.g. process-flow-compact, kpi-inline) or an unfilled text layout.",
		},
		RelatedCodes: []string{ErrCodeCellUnderfilled, ErrCodeSlideUnderused, ErrCodeSparseLayout},
	}
	findingMetaRegistry[ErrCodeSlideUnderused] = FindingMeta{
		Code:        ErrCodeSlideUnderused,
		Summary:     "The slide's grid content covers under 45% of the layout's safe content area.",
		Severity:    "review",
		WhenEmitted: "Preflight unions the ink rectangles of every resolved grid cell (filled shapes, estimated text blocks, images, icons, tables, diagrams) and compares the bounding box with the content area below the title.",
		RemediationSteps: []string{
			"Remove an unnecessary bounds / max_height_pct cap so the grid fills the content area.",
			"Add a supporting zone (takeaway, chart, detail row) or merge this slide's content with a neighbour.",
		},
		RelatedCodes: []string{ErrCodeSparseLayout, ErrCodeSparseFill, ErrCodePatternUnderfilled},
	}
}
