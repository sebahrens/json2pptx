package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// nativeVerticalPadSteps are the top / bottom text margins a native diagram's
// boxes step down when their text does not fit at the uniform 0.5 cm margin:
// the margin itself, then the 10 / 7 / 5pt row padding the ruled patterns
// give up before type (patterns.rowPadStepsPt). Left and right margins are
// never tightened, and a diagram that fits at the uniform margin never takes
// a step (go-slide-creator-6ne1m).
var nativeVerticalPadSteps = []int64{
	pptx.ShapeTextInsetEMU,
	10 * int64(types.EMUPerPoint),
	7 * int64(types.EMUPerPoint),
	5 * int64(types.EMUPerPoint),
}

// nativeBudgetSample is the running text one-line budgets are measured with:
// ordinary words, so the count is what an author's own label would reach.
const nativeBudgetSample = "Switching costs decrease as enterprise buyers and regional partner networks consolidate around open standards"

// nativeOneLineChars returns how many characters of ordinary text stay on one
// line in the body build returns, set in a shape of the given width. It is
// measured with the writer's own estimator, so it is the length at which the
// writer starts a second line.
func nativeOneLineChars(build func(text string) pptx.TextBody, width int64) int {
	limit := 100 * int64(types.EMUPerInch)
	oneLine := nativeTextNeedAtMarginEMU(build("x"), width, limit)
	if oneLine <= 0 || oneLine >= limit {
		return 0
	}
	sample := []rune(nativeBudgetSample)
	fits := func(n int) bool {
		// Trailing spaces never start a line, so a sample cut after one is
		// measured without it.
		text := strings.TrimSpace(string(sample[:n]))
		return nativeTextNeedAtMarginEMU(build(text), width, limit) <= oneLine+int64(types.EMUPerPoint)
	}
	lo, hi := 0, len(sample)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// nativeFitBudget is what a native diagram's region holds at the authored
// size: how many items each container takes and how long a one-line item may
// be. It rides on the TEXT_BELOW_READABLE_MIN finding of a diagram whose text
// had to be shrunk, so the finding says what to cut to rather than only that
// something is too long (go-slide-creator-6ne1m, go-slide-creator-35rsq).
type nativeFitBudget struct {
	// item and container name what is counted: "factor" per "force", "name"
	// per "cell", "bullet" per "section", "level" per "pyramid".
	item, container string
	// maxItems is how many one-line items every container holds at once;
	// maxChars how many characters one of those lines takes.
	maxItems int
	maxChars int
	sizePt   float64
}

// sentence states the budget: what the diagram holds at the authored size.
func (b *nativeFitBudget) sentence(diagramType string) string {
	if b.maxItems <= 0 {
		return fmt.Sprintf("at %.0fpt this %s has no room for a %s in every %s: give it a larger region",
			b.sizePt, diagramType, b.item, b.container)
	}
	return fmt.Sprintf("at %.0fpt this %s holds %d one-line %ss per %s of about %d characters each",
		b.sizePt, diagramType, b.maxItems, b.item, b.container, b.maxChars)
}

// apply adds the budget to a readability finding raised on one of the
// diagram's shapes: the measured counts as fix params and a sentence in the
// message.
func (b *nativeFitBudget) apply(f *patterns.FitFinding, diagramType string) {
	if b == nil || b.item == "" {
		return
	}
	f.Pattern = diagramType
	f.Message += "; " + b.sentence(diagramType)
	if f.Fix == nil {
		return
	}
	f.Fix.Params["diagram_type"] = diagramType
	f.Fix.Params["budget_item"] = b.item
	f.Fix.Params["max_items_per_"+b.container] = b.maxItems
	f.Fix.Params["max_chars_per_item"] = b.maxChars
	f.Fix.Params["budget_pt"] = b.sizePt
}

// Sizing native diagram cells to their text (go-slide-creator-zbo58).
//
// The native diagram builders used to cut their cells by fixed ratios (a BMC
// top row of 60%, SWOT headers of 18%, Porter's peripheral boxes of 25%) and
// let the stored autofit shrink whatever did not fit. On a short content area
// (abstract, modern) that shrink reached 20-60%: four BMC bullets written at
// 10pt and stored at 2pt, which generation then correctly refused. The layouts
// below measure each cell's text with the writer's own estimator and hand the
// space to the cells that need it, before any shrink is stored.

// nativeCardSeamInsetEMU is the text inset on the side of a card body that
// meets its own header. A BMC / SWOT / nine-box card is drawn as a header shape
// stacked directly on a body shape of the same fill, so that seam is not an
// edge a reader sees: the uniform 0.5 cm margin still holds on every visible
// edge of the card, but repeating it at the seam put 1 cm of dead space between
// a header and its first bullet. The seam keeps the OOXML default top inset
// (0.05in).
const nativeCardSeamInsetEMU int64 = 45720

// nativeCardBodyInsets are the insets of a card body under a same-fill header:
// the uniform margin on the three visible edges, the seam inset on top.
func nativeCardBodyInsets() [4]int64 {
	return [4]int64{pptx.ShapeTextInsetEMU, nativeCardSeamInsetEMU, pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU}
}

// nativeCardHeaderInsets are the insets of a card header above a same-fill
// body: the uniform margin on the three visible edges, the seam inset below.
func nativeCardHeaderInsets() [4]int64 {
	return [4]int64{pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU, nativeCardSeamInsetEMU}
}

// nativeTextNeedEMU returns the smallest shape height at which tb, set in a
// shape of the given width, needs no autofit shrink. It measures with
// pptx.AutofitScaleFor after the EffectiveTextInsets clamp GenerateShape
// applies, so the height it reports is the one the writer will not shrink.
// It returns limit when the text does not fit even there, and 0 for a body
// with no text.
func nativeTextNeedEMU(tb pptx.TextBody, width, limit int64) int64 {
	return nativeTextNeed(tb, width, limit, false)
}

// nativeTextNeedAtMarginEMU is nativeTextNeedEMU for a body that keeps the
// margins it declares. The writer gives up a shape's top and bottom margin
// before it lets a line of text go (EffectiveTextInsets), so the smallest
// height a one-line label "fits" is the bare line: a cell sized to that need
// set its label against the edge of its card. A layout that sizes boxes from
// their text asks for this height instead, and steps the margin down itself
// (nativeVerticalPadSteps) when the boxes do not fit.
//
// It also leaves each line the slack the writer leaves a word
// (pptx.WordFitSlackFor): 5% in a face it measures exactly, 20% when the
// template's face is host-dependent and Liberation Sans stands in for it. A
// line that fits its box by a hair in the writer's measure wraps in a renderer
// whose face runs wider — LibreOffice draws Segoe UI as Verdana — and the box
// sized for three lines then holds four: LibreOffice shrank the one BMC cell
// on modern-template whose longest bullet filled 98% of its line. The writer
// still measures the full width, so the room is never stored as a shrink.
func nativeTextNeedAtMarginEMU(tb pptx.TextBody, width, limit int64) int64 {
	return nativeTextNeed(tb, width, limit, true)
}

// nativeHeaderNeedEMU is the height of a card's header band in a cell cellH
// tall: what its title needs at its declared margins, and never more than half
// the cell. A title that does not fit there — a long word in a narrow cell —
// keeps half the cell and is left to the writer's shrink, so the body under it
// still has a box to be measured in.
func nativeHeaderNeedEMU(tb pptx.TextBody, width, cellH int64) int64 {
	return min(nativeTextNeedAtMarginEMU(tb, width, cellH), cellH/2)
}

func nativeTextNeed(tb pptx.TextBody, width, limit int64, keepMargins bool) int64 {
	if width <= 0 || limit <= 0 || len(tb.Paragraphs) == 0 {
		return 0
	}
	fits := func(h int64) bool {
		probe := tb
		bounds := pptx.RectEmu{CX: width, CY: h}
		probe.Insets = pptx.EffectiveTextInsets(&probe, bounds)
		if keepMargins {
			if probe.Insets[1] != tb.Insets[1] || probe.Insets[3] != tb.Insets[3] {
				return false
			}
			if line := width - probe.Insets[0] - probe.Insets[2]; line > 0 {
				probe.Insets[2] += int64(float64(line) * (1 - 1/pptx.WordFitSlackFor(&probe)))
			}
		}
		return pptx.AutofitFitsFor(&probe, bounds)
	}
	if !fits(limit) {
		return limit
	}
	lo, hi := int64(0), limit
	for hi-lo > int64(types.EMUPerPoint)/2 {
		mid := lo + (hi-lo)/2
		if fits(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// splitByNeed divides total between two stacked parts in proportion to what
// each needs, keeping each part's share within [minShare, 1-minShare] so a
// sparse part never collapses. With no measured need it splits evenly.
func splitByNeed(total, needA, needB int64, minShare float64) (int64, int64) {
	share := 0.5
	if needA+needB > 0 {
		share = float64(needA) / float64(needA+needB)
	}
	share = min(max(share, minShare), 1-minShare)
	a := int64(float64(total) * share)
	return a, total - a
}

// Setting a card's list in columns (go-slide-creator-35rsq).
//
// A landscape content area gives a wide card several times the width of a
// short item and little height, so a list that does not fit in one column is
// set in two or three before anything is given up vertically. The card body's
// own shape carries the first column, held to it by its right inset; each
// further column is an unfilled text box over the body.

// nativeColumnGutter keeps an item clear of the column beside it.
const nativeColumnGutter int64 = 6 * 12700

// nativeSplitColumns sets items in c columns, filled column by column and as
// evenly as the count allows.
func nativeSplitColumns(items []string, c int) [][]string {
	if len(items) == 0 {
		return nil
	}
	c = max(1, min(c, len(items)))
	rows := (len(items) + c - 1) / c
	var cols [][]string
	for i := 0; i < len(items); i += rows {
		cols = append(cols, items[i:min(i+rows, len(items))])
	}
	return cols
}

// nativeColumnRect is the rectangle of column j of n in a card body: the
// body's own shape for the first column, a text box over it for the others.
// The columns share the width inside the uniform side margins.
func nativeColumnRect(body pptx.RectEmu, j, n int) pptx.RectEmu {
	if j == 0 {
		return body
	}
	colW := (body.CX - 2*pptx.ShapeTextInsetEMU) / int64(n)
	return pptx.RectEmu{X: body.X + pptx.ShapeTextInsetEMU + int64(j)*colW, Y: body.Y, CX: colW, CY: body.CY}
}

// nativeColumnInsets turns a card body's insets into those of column j of n
// in a body bodyW wide: the first column keeps the left margin and gives the
// other columns' width to its right inset; the others start at their own left
// edge.
func nativeColumnInsets(insets [4]int64, j, n int, bodyW int64) [4]int64 {
	if n <= 1 {
		return insets
	}
	colW := (bodyW - 2*pptx.ShapeTextInsetEMU) / int64(n)
	switch {
	case j == 0:
		insets[2] = bodyW - pptx.ShapeTextInsetEMU - colW + nativeColumnGutter
	case j == n-1:
		insets[0], insets[2] = 0, 0
	default:
		insets[0], insets[2] = 0, nativeColumnGutter
	}
	return insets
}
