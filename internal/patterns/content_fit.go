package patterns

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// Content-fit helpers shared by patterns that size cards / rows to their
// content instead of stretching them over the whole content area
// (go-slide-creator-5lbo, go-slide-creator-7km8, go-slide-creator-3i7c).
// ---------------------------------------------------------------------------

const (
	// defaultShapeInsetLRPt is the OOXML default left/right text inset (0.1").
	defaultShapeInsetLRPt = 7.2
	// defaultShapeInsetTBPt is the OOXML default top/bottom text inset (0.05").
	defaultShapeInsetTBPt = 3.6
	// contentLineHeight is the line-height factor used for content estimates.
	contentLineHeight = 1.2
)

// contentAreaPt returns the pattern's content-area width and height in points,
// falling back to the default 16:9 grid bounds when the context carries no
// layout geometry (e.g. unit tests or expand_pattern without a template).
func contentAreaPt(ctx ExpandContext) (w, h float64) {
	we, he := expandContentSize(ctx)
	if we <= 0 || he <= 0 {
		db := shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU)
		we, he = db.CX, db.CY
	}
	return float64(we) / 12700.0, float64(he) / 12700.0
}

// equalColumnWidthPt returns the width of one of n equal columns separated by
// gapPt across totalPt.
func equalColumnWidthPt(totalPt float64, n int, gapPt float64) float64 {
	if n < 1 {
		n = 1
	}
	w := (totalPt - float64(n-1)*gapPt) / float64(n)
	if w < 1 {
		return 1
	}
	return w
}

// measuredLines returns how many lines text wraps to at sizePt inside a text
// box widthPt wide (insets already removed), using real font metrics for the
// theme body font. When no font can be resolved it falls back to an average
// glyph-advance estimate (0.55 em regular, 0.6 em bold).
func measuredLines(text, font string, bold bool, sizePt, widthPt float64) int {
	if text == "" {
		return 0
	}
	if widthPt <= 0 || sizePt <= 0 {
		return 1
	}
	m, err := textfit.MeasureStyledRuns(textfit.StyledMeasureParams{
		Runs:     []textfit.StyledRun{{Text: text, Bold: bold}},
		FontName: font,
		FontPt:   sizePt,
		WidthEMU: int64(widthPt * 12700),
	})
	if err == nil && m.Lines > 0 {
		return m.Lines
	}
	em := 0.55
	if bold {
		em = 0.6
	}
	return estimateWrappedLines(text, sizePt*em/0.5, widthPt)
}

// fitSingleLineSize returns the largest font size <= sizePt (in 1pt steps,
// floored at minPt) at which text renders on ONE line within widthPt. Used for
// big-number values that must never break mid-token ("$4 / .2 / M") or even
// at a space ("12 / days"): the value shrinks instead of wrapping.
func fitSingleLineSize(text, font string, bold bool, sizePt, minPt, widthPt float64) float64 {
	if text == "" || sizePt <= minPt {
		return sizePt
	}
	for s := sizePt; s > minPt; s-- {
		if measuredLines(text, font, bold, s, widthPt) <= 1 {
			return s
		}
	}
	// Even the floor does not fit on one line in practice; keep the floor.
	return minPt
}

// GridVerticalAlignDefault is the vertical_align every expanded pattern grid
// gets unless the pattern sets its own (go-slide-creator-7km8): content-sized
// blocks (rows capped by max_height, height-capped bounds) are centred in the
// content area instead of stretching to fill it or hugging the top.
const GridVerticalAlignDefault = "center"

// ApplyGridDefaults applies the pattern-expansion defaults shared by every
// caller of Pattern.Expand (generation, budgets, previews, the HTTP API) so
// all of them resolve the same geometry.
func ApplyGridDefaults(grid *jsonschema.ShapeGridInput) {
	if grid == nil {
		return
	}
	if grid.VerticalAlign == "" {
		grid.VerticalAlign = GridVerticalAlignDefault
	}
}
// textParagraph is a sized paragraph used for height estimates.
type textParagraph struct {
	text string
	size float64
	bold bool
}

// textBlockHeightPt estimates the height (points) of stacked paragraphs
// wrapped in a text box widthPt wide (insets already removed).
func textBlockHeightPt(font string, widthPt float64, paras ...textParagraph) float64 {
	var h float64
	for _, p := range paras {
		if p.text == "" {
			continue
		}
		size := shapegrid.EffectiveTextSizePt(p.size)
		lines := measuredLines(p.text, font, p.bold, size, widthPt)
		h += float64(lines) * size * contentLineHeight
	}
	return h
}

// clampPt clamps v into [lo, hi]; hi <= 0 means "no upper bound".
func clampPt(v, lo, hi float64) float64 {
	if hi > 0 {
		v = math.Min(v, hi)
	}
	return math.Max(v, lo)
}
