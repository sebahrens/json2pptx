package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Heatmap label fitting (go-slide-creator-3rkpt).
//
// The builder divides the available height by the row count with no floor and
// drew each label at full length into a box one cell tall. Past a certain
// density that box holds fewer lines than the label wraps to, so a 14x14 grid
// of "Geschaeftsbereich A Wirtschaftlichkeitsberechnung" rendered every row
// label on top of its neighbours as unreadable mush, and the column headers
// broke mid-word and clipped. Nothing reported it.
//
// Labels are now measured against the box they will be drawn into and
// shortened to fit, and the shortening is reported.

// heatmapGeometry is the label/cell layout of a heatmap. Registration fits the
// labels against it and finalize draws against it, so the two cannot drift.
type heatmapGeometry struct {
	rowLabelW int64
	colLabelH int64
	cellW     int64
	cellH     int64
}

// heatmapGeometryFor computes the layout for a heatmap of numRows x numCols in
// bounds. It is the arithmetic generateHeatmapGroupXML has always done.
func heatmapGeometryFor(bounds types.BoundingBox, numRows, numCols int, hasRowLabels, hasColLabels bool) heatmapGeometry {
	var g heatmapGeometry
	if hasRowLabels {
		g.rowLabelW = heatmapRowLabelWidth
	}
	if hasColLabels {
		g.colLabelH = heatmapColLabelHeight
	}
	if numRows < 1 || numCols < 1 {
		return g
	}
	gridW := bounds.Width - g.rowLabelW
	gridH := bounds.Height - g.colLabelH
	g.cellW = (gridW - int64(numCols-1)*heatmapGap) / int64(numCols)
	g.cellH = (gridH - int64(numRows-1)*heatmapGap) / int64(numRows)
	return g
}

// fitHeatmapLabels shortens row and column labels to the boxes they will be
// drawn into, returning how many were shortened. A sparse heatmap whose labels
// already fit is untouched.
func fitHeatmapLabels(parsed *heatmapParsedData, bounds types.BoundingBox) int {
	if parsed == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return 0
	}
	numRows := len(parsed.values)
	if numRows == 0 {
		return 0
	}
	numCols := len(parsed.values[0])
	g := heatmapGeometryFor(bounds, numRows, numCols, len(parsed.rowLabels) > 0, len(parsed.colLabels) > 0)

	shortened := 0
	// Row labels sit in a (rowLabelW - gap) x cellH box.
	for i, label := range parsed.rowLabels {
		fitted, cut := fitLabelToBox(label, g.rowLabelW-heatmapGap, g.cellH)
		if cut {
			parsed.rowLabels[i] = fitted
			shortened++
		}
	}
	// Column headers sit in a cellW x colLabelH box.
	for i, label := range parsed.colLabels {
		fitted, cut := fitLabelToBox(label, g.cellW, g.colLabelH)
		if cut {
			parsed.colLabels[i] = fitted
			shortened++
		}
	}
	return shortened
}

// heatmapLabelEllipsis is appended to a shortened label so the reader can see
// the text was cut rather than mis-typed.
const heatmapLabelEllipsis = "…"

// fitLabelToBox shortens label so it wraps to at most as many lines as boxH
// holds, returning the result and whether anything was cut. It binary-searches
// the rune count, so a long label costs a handful of measurements rather than
// one per rune.
//
// The budget is a LINE COUNT, not boxH itself. A dense grid can leave a row
// shorter than a single line, and measuring against boxH there finds nothing
// that fits — which used to mean the label was left alone, still wrapping to
// three lines and still overlapping its neighbours. One line always overflows
// less than three, so the floor is one line and the finding carries the rest.
func fitLabelToBox(label string, boxW, boxH int64) (string, bool) {
	if label == "" || boxW <= 0 || boxH <= 0 {
		return label, false
	}
	budget := heatmapLineBudget(boxW, boxH)
	if heatmapLabelHeight(label, boxW) <= budget {
		return label, false
	}
	runes := []rune(label)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if heatmapLabelHeight(string(runes[:mid])+heatmapLabelEllipsis, boxW) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		// The box is narrower than one glyph plus the ellipsis. Replacing the
		// label with a bare ellipsis tells the reader less than the overflow
		// does, and the finding already carries the signal.
		return label, false
	}
	return string(runes[:lo]) + heatmapLabelEllipsis, true
}

// heatmapLabelHeight measures a label's wrapped height in EMU at the heatmap
// header font size.
func heatmapLabelHeight(label string, widthEMU int64) int64 {
	return panelTextHeightEMU([]string{label}, "", widthEMU, heatmapHeaderFontSize, 0)
}

// heatmapLabelFinding reports labels the heatmap had to shorten, naming the
// grid size so the author can see the density that caused it.
func heatmapLabelFinding(numRows, numCols, shortened int, path string) *patterns.FitFinding {
	if shortened <= 0 {
		return nil
	}
	cells := numRows * numCols
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "heatmap",
			Path:    path,
			Code:    patterns.ErrCodeGridDiagramNarrow,
			Message: fmt.Sprintf(
				"heatmap is %dx%d (%d cells), so %d label(s) do not fit their row/column and were shortened with an ellipsis — use fewer rows/columns or shorter labels to show them in full",
				numRows, numCols, cells, shortened,
			),
			Fix: &patterns.FixSuggestion{
				Kind: "reduce_text",
				Params: map[string]any{
					"diagram_type":     "heatmap",
					"rows":             numRows,
					"columns":          numCols,
					"cells":            cells,
					"labels_shortened": shortened,
				},
			},
		},
		Action: "review",
	}
}

// heatmapLineBudget is the wrapped height a label may occupy in a boxW x boxH
// box: whole lines only, and never fewer than one.
func heatmapLineBudget(boxW, boxH int64) int64 {
	lineH := heatmapLabelHeight("X", boxW)
	if lineH <= 0 {
		return boxH
	}
	lines := boxH / lineH
	if lines < 1 {
		lines = 1
	}
	return lineH * lines
}
