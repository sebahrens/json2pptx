package textfit

import "strings"

// Balanced line breaks (go-slide-creator-58dhw).
//
// DrawingML has no balanced wrapping, so a heading or title that ends in a
// one-word line ("… not the / exception") is corrected by narrowing the
// paragraph's measure with a right margin (a:pPr marR). BalancedMarginEMU
// finds that margin; callers own the measurement so it matches the font,
// weight and insets their renderer assumes.

// balanceToleranceFrac is how far, as a share of the full measure, the chosen
// measure stays from either edge of the balanced range: narrower adds a line,
// wider brings the one-word last line back. A renderer whose metrics differ by
// less than this wraps the same way; a narrower range is left alone.
const balanceToleranceFrac = 0.04

// LineCounter reports how many lines text wraps to at widthEMU, or a negative
// value when it cannot measure.
type LineCounter func(text string, widthEMU int64) int

// BalancedMarginEMU returns the right margin (EMU) that removes a one-word
// last line from text wrapped at widthEMU without changing its line count, or
// 0 when there is no such widow, the text is far shorter than the measure, or
// the balanced range is too narrow to hold across renderers.
func BalancedMarginEMU(lines LineCounter, text string, widthEMU int64) int64 {
	text = strings.Join(strings.Fields(text), " ")
	cut := strings.LastIndex(text, " ")
	if widthEMU <= 0 || cut < 0 {
		return 0
	}
	head := text[:cut]
	n := lines(text, widthEMU)
	if n < 2 || lines(head, widthEMU) != n-1 {
		return 0 // one line, or the last line already holds several words
	}
	if lines(text, widthEMU/4) == n {
		return 0 // cannot bracket the narrowest measure; leave it alone
	}
	// narrowest: the smallest measure that still wraps text to n lines.
	narrowest := bisectWidth(widthEMU/4, widthEMU, func(w int64) bool { return lines(text, w) == n })
	// widow: the smallest measure at which the head fits on n-1 lines again,
	// i.e. the one-word last line returns.
	widow := bisectWidth(narrowest, widthEMU, func(w int64) bool { return lines(head, w) <= n-1 })
	tol := int64(float64(widthEMU) * balanceToleranceFrac)
	if widow-narrowest < 2*tol {
		return 0
	}
	target := (narrowest + widow) / 2
	if lines(text, target) != n || lines(head, target) != n {
		return 0
	}
	return widthEMU - target
}

// bisectWidth returns the smallest width in (lo, hi] for which ok holds,
// assuming ok is false at lo, true at hi and monotone in between.
func bisectWidth(lo, hi int64, ok func(int64) bool) int64 {
	step := max(hi/400, 1)
	for hi-lo > step {
		mid := (lo + hi) / 2
		if ok(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}
