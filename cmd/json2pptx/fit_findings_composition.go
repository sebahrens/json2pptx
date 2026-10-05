package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Composition faults measured on the resolved grid (go-slide-creator-wwmod):
// text that wraps into a tall column of fragments (TEXT_WRAPS_NARROW) and
// peer cells that render at different sizes (SIBLING_SIZE_MISMATCH). The
// whitespace checks live in fit_findings_balance.go.

const (
	// narrowWrapMinLines is the line count from which a paragraph in a narrow
	// box stops reading as a label: four lines of three words still scan,
	// five or six lines of two do not.
	narrowWrapMinLines = 5
	// narrowWrapMaxWordsPerLine is the average line length (in words) at or
	// below which the box, not the text, is the problem. A six-line paragraph
	// in a wide text column runs eight to twelve words a line.
	narrowWrapMaxWordsPerLine = 3.0
	// narrowWrapFitLines is the line count the suggested word budget fits.
	narrowWrapFitLines = 4

	// siblingSizeMinRatio is how far below the largest sibling the smallest
	// may render before the row reads as two sizes (a 12pt header beside
	// 11pt ones is noise; 12pt beside 10pt is not).
	siblingSizeMinRatio = 0.92
)

// narrowWrapHit records one box whose text wraps into short fragments.
type narrowWrapHit struct {
	path     string
	lines    int
	words    int
	availPt  float64
	maxWords int
	// text is the wrapped paragraph: what locates the authored field behind a
	// cell a pattern generated.
	text string
	// maxBoxes is how many boxes the hit's row holds at a width where this
	// paragraph wraps to narrowWrapFitLines lines; 0 when the row is not known.
	maxBoxes int
}

// rowBoxes describes the boxes of the grid row a cell sits in: how many there
// are, the width they span and the gap between two of them.
type rowBoxes struct {
	n             int
	spanPt, gapPt float64
}

// rowBoxesOf measures the row of cell among the grid's resolved shape cells.
func rowBoxesOf(cells []shapegrid.ResolvedCell, cell shapegrid.ResolvedCell) rowBoxes {
	var left, right, widths int64
	n := 0
	for _, c := range cells {
		if c.RowIdx != cell.RowIdx || c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 || c.Bounds.CX <= 0 {
			continue
		}
		if n == 0 || c.Bounds.X < left {
			left = c.Bounds.X
		}
		if n == 0 || c.Bounds.X+c.Bounds.CX > right {
			right = c.Bounds.X + c.Bounds.CX
		}
		widths += c.Bounds.CX
		n++
	}
	rb := rowBoxes{n: n, spanPt: float64(right-left) / emuPerPt}
	if n > 1 {
		rb.gapPt = math.Max(float64(right-left-widths)/emuPerPt/float64(n-1), 0)
	}
	return rb
}

// widthForLines is the narrowest text width (whole points) at which p wraps to
// at most maxLines lines.
func (m *geomMeasurer) widthForLines(p geomParagraph, fromPt float64, maxLines int) float64 {
	lo, hi := math.Floor(fromPt), math.Floor(fromPt)
	for range 12 {
		hi *= 2
		if lines, _ := m.paragraphLines(p, hi); lines <= maxLines {
			break
		}
	}
	for hi-lo > 1 {
		mid := math.Floor((lo + hi) / 2)
		if lines, _ := m.paragraphLines(p, mid); lines <= maxLines {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// maxBoxesFor is how many boxes the row holds once each is wide enough for p
// to wrap to narrowWrapFitLines lines: the row's span, shared between boxes
// that keep their text margin (boxPt - availPt) and the gap between them.
func (m *geomMeasurer) maxBoxesFor(p geomParagraph, availPt, boxPt float64, row rowBoxes) int {
	if row.n < 2 || row.spanPt <= 0 || boxPt <= 0 {
		return 0
	}
	need := m.widthForLines(p, availPt, narrowWrapFitLines) + math.Max(boxPt-availPt, 0)
	boxes := int(math.Floor((row.spanPt + row.gapPt) / (need + row.gapPt)))
	return min(max(boxes, 1), row.n-1)
}

// paragraphLines greedily wraps one paragraph at availPt and returns its line
// count and word count, with the font metrics textBlockPt uses.
func (m *geomMeasurer) paragraphLines(p geomParagraph, availPt float64) (lines, words int) {
	space := m.widthPt(" ", p.sizePt, p.bold)
	lines, lineW := 1, 0.0
	for _, w := range strings.Fields(p.text) {
		words++
		ww := m.widthPt(w, p.sizePt, p.bold)
		switch {
		case lineW == 0:
			lineW = ww
		case lineW+space+ww <= availPt:
			lineW += space + ww
		default:
			lines++
			lineW = ww
		}
	}
	return lines, words
}

// wordsFittingLines is the longest word prefix of p that wraps to at most
// maxLines at availPt.
func (m *geomMeasurer) wordsFittingLines(p geomParagraph, availPt float64, maxLines int) int {
	fields := strings.Fields(p.text)
	for n := len(fields); n > 0; n-- {
		q := p
		q.text = strings.Join(fields[:n], " ")
		if lines, _ := m.paragraphLines(q, availPt); lines <= maxLines {
			return n
		}
	}
	return 0
}

// noteNarrowWrap records the cell when one of its paragraphs wraps into
// narrowWrapMinLines or more lines of at most narrowWrapMaxWordsPerLine
// words. A list of short bullets is many paragraphs of one line each and is
// not a hit; neither is a long paragraph in a wide column.
func (a *geomAccumulator) noteNarrowWrap(path string, txt geomText, availPt float64, cell shapegrid.ResolvedCell) {
	if a.m == nil || a.m.family == nil || availPt <= 0 {
		return
	}
	for _, p := range txt.paragraphs {
		lines, words := a.m.paragraphLines(p, availPt)
		if lines < narrowWrapMinLines || float64(words)/float64(lines) > narrowWrapMaxWordsPerLine {
			continue
		}
		a.narrow = append(a.narrow, narrowWrapHit{
			path: path, lines: lines, words: words, availPt: availPt, text: p.text,
			maxWords: a.m.wordsFittingLines(p, availPt, narrowWrapFitLines),
			maxBoxes: a.m.maxBoxesFor(p, availPt, float64(cell.Bounds.CX)/emuPerPt, rowBoxesOf(a.gridCells, cell)),
		})
		return
	}
}

// narrowWrapFinding aggregates the slide's narrow boxes into one finding.
// slide is the authored slide: its pattern values locate the field behind
// each box (params.paths), where a cell path names only the expanded grid.
func (a *geomAccumulator) narrowWrapFinding(patternName string, slide *SlideInput, slideIdx int) *patterns.FitFinding {
	if len(a.narrow) == 0 {
		return nil
	}
	worst := a.narrow[0]
	maxWords, maxBoxes := worst.maxWords, 0
	cells := make([]string, len(a.narrow))
	texts := make([]string, len(a.narrow))
	for i, h := range a.narrow {
		cells[i], texts[i] = h.path, h.text
		if h.lines > worst.lines {
			worst = h
		}
		if h.maxWords > 0 && (maxWords == 0 || h.maxWords < maxWords) {
			maxWords = h.maxWords
		}
		if h.maxBoxes > 0 && (maxBoxes == 0 || h.maxBoxes < maxBoxes) {
			maxBoxes = h.maxBoxes
		}
	}
	msg := fmt.Sprintf("a %d-word paragraph wraps to %d lines in a %.0fpt-wide text area (about %.1f words per line) — a column of fragments, not a label",
		worst.words, worst.lines, worst.availPt, float64(worst.words)/float64(worst.lines))
	if len(a.narrow) > 1 {
		msg = fmt.Sprintf("%d boxes wrap their text into columns of two- and three-word lines; worst: %s", len(a.narrow), msg)
	}
	params := map[string]any{
		"cells":     cells,
		"max_lines": worst.lines,
		"fit_lines": narrowWrapFitLines,
		"hint":      "cut each box to a label, use fewer boxes so each is wider, or give each item a full-width row (numbered-step-strip, labeled-rows)",
		// paths names the authored field behind each box, in the order of
		// cells; texts is the paragraph each holds.
		"paths": narrowWrapAuthoredPaths(slide, slideIdx, cells, texts),
		"texts": texts,
	}
	if maxWords > 0 {
		params["max_words"] = maxWords
	}
	if maxBoxes > 0 {
		// The box count of a row at which the text reads as a label again.
		params["max_boxes"] = maxBoxes
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    a.narrow[0].path,
			Code:    patterns.ErrCodeTextWrapsNarrow,
			Message: msg,
			Fix:     &patterns.FixSuggestion{Kind: "shorten_or_restructure", Params: params},
		},
		Action: "review",
	}
}

// narrowWrapAuthoredPaths is where the author edits each wrapped box: on a
// pattern slide the pattern value that carries the box's text
// (/slides/N/pattern/values/steps/3/label), and the cell itself on a slide
// whose grid the author wrote, or when no single value carries the text.
func narrowWrapAuthoredPaths(slide *SlideInput, slideIdx int, cells, texts []string) []string {
	out := append([]string(nil), cells...)
	if slide == nil || slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return out
	}
	var values any
	if json.Unmarshal(slide.Pattern.Values, &values) != nil {
		return out
	}
	base := slidepath.SlideField(slideIdx, "pattern") + "/values"
	for i, text := range texts {
		if matches := findValueStrings(values, "", strings.TrimSpace(text)); len(matches) == 1 {
			out[i] = base + specPointer(matches[0].path)
		}
	}
	return out
}

// siblingCell is one text cell of a grid row with the size it renders at.
type siblingCell struct {
	path     string
	label    string
	authored float64
	// written is the size the slide XML carries for the cell's first
	// paragraph: its run size times the autofit scale stored on the shape.
	written float64
	// rendered is the size a renderer is expected to draw it at: written,
	// or less when the renderer re-fits the shape (refit).
	rendered float64
	// shrunk says the renderer scales this cell's text down: the writer
	// stored an autofit scale, or the text needs more lines than the box
	// holds once a renderer's font runs a little wider than the measured one.
	shrunk bool
	// refit says the shrink is not in the file: the cell is written at its
	// full size and the check expects a renderer with a wider face to wrap
	// and shrink it (siblingRenderSlack).
	refit bool
}

// siblingRenderSlack is how much wider a renderer's font may run than the
// face the text was measured with. Generation stores an autofit scale from
// its own metrics; a shape left at normAutofit with no stored scale is
// re-fitted by the renderer with the installed font, and a label within this
// margin of its box wraps there and is shrunk (go-slide-creator-wwmod E15:
// "Deployment speed" and "Commercial traction" beside "Energy use").
const siblingRenderSlack = shapegrid.RenderFaceSlack

// siblingSizeFindings compares the cells of each grid row that were authored
// alike — the same paragraph sizes and weights, as column headers and card
// titles are — and reports rows whose cells render at visibly different
// sizes: the long labels are shrunk to fit and the short ones are not. One
// finding per slide, on the row with the widest spread.
func siblingSizeFindings(m *geomMeasurer, grid *ShapeGridInput, result *shapegrid.ResolveResult, basePath, patternName string, slideWidth, slideHeight int64) []patterns.FitFinding {
	type group struct {
		cells []shapegrid.ResolvedCell
		paths []string
	}
	groups := map[string]*group{}
	var order []string
	for _, rc := range readabilityGridCells(grid, result, basePath, slideWidth, slideHeight, 0) {
		cell := rc.cell
		if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil || cell.Bounds.CX <= 0 || cell.Bounds.CY <= 0 {
			continue
		}
		paras := parseCellParagraphs(cell.ShapeSpec.Text)
		if len(paras) == 0 {
			continue
		}
		cut := strings.LastIndex(rc.path, "/cells/")
		if cut < 0 {
			continue
		}
		var sig strings.Builder
		sig.WriteString(rc.path[:cut])
		for _, p := range paras {
			fmt.Fprintf(&sig, "|%.1f/%t", p.sizePt, p.bold)
		}
		key := sig.String()
		g, ok := groups[key]
		if !ok {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.cells = append(g.cells, cell)
		g.paths = append(g.paths, rc.path)
	}

	var worst []siblingCell
	worstRatio := 1.0
	for _, key := range order {
		g := groups[key]
		if len(g.cells) < 2 {
			continue
		}
		row := make([]siblingCell, 0, len(g.cells))
		lo, hi := math.Inf(1), 0.0
		for i, cell := range g.cells {
			sc, ok := siblingRenderedSize(m, cell, g.paths[i])
			if !ok {
				row = nil
				break
			}
			lo, hi = math.Min(lo, sc.rendered), math.Max(hi, sc.rendered)
			row = append(row, sc)
		}
		if len(row) < 2 || hi <= 0 {
			continue
		}
		if ratio := lo / hi; ratio < siblingSizeMinRatio && ratio < worstRatio {
			worst, worstRatio = row, ratio
		}
	}
	if worst == nil {
		return nil
	}
	sort.SliceStable(worst, func(i, j int) bool { return worst[i].rendered < worst[j].rendered })
	smallest, largest := worst[0], worst[len(worst)-1]
	cells := make([]any, len(worst))
	for i, c := range worst {
		cells[i] = map[string]any{"path": c.path + "/shape/text", "text": c.label, "written_pt": round1(c.written), "rendered_pt": round1(c.rendered), "shrunk": c.shrunk}
	}
	// A shrink the writer stored is a fact of the file; one the check expects
	// of a renderer is a risk, and the message says which
	// (go-slide-creator-bhoo3: "is shrunk to about 16.2pt" on a cell written
	// at 18pt that LibreOffice drew at 18pt).
	message := fmt.Sprintf("%d cells of one row are authored alike (%.0fpt) but %q is shrunk to about %.1fpt to fit its box while %q stays at %.1fpt — the row reads as two sizes",
		len(worst), smallest.authored, smallest.label, smallest.rendered, largest.label, largest.rendered)
	if smallest.refit {
		message = fmt.Sprintf("%d cells of one row are authored alike (%.0fpt): %q is written at %.1fpt on one line of a box that holds one line, within %.0f%% of its width — a renderer whose font runs wider wraps it and shrinks it to about %.1fpt while %q stays at %.1fpt, and the row reads as two sizes",
			len(worst), smallest.authored, smallest.label, smallest.written, (siblingRenderSlack-1)*100, smallest.rendered, largest.label, largest.rendered)
	}
	return []patterns.FitFinding{{
		ValidationError: patterns.ValidationError{
			Pattern: patternName,
			Path:    smallest.path + "/shape/text",
			Code:    patterns.ErrCodeSiblingSizeMismatch,
			Message: message,
			Fix: &patterns.FixSuggestion{
				Kind: "shorten_or_restructure",
				Params: map[string]any{
					"cells":       cells,
					"longest":     smallest.label,
					"authored_pt": round1(smallest.authored),
					"min_pt":      round1(smallest.rendered),
					"max_pt":      round1(largest.rendered),
					"hint":        siblingSizeHint(smallest.refit),
				},
			},
		},
		Action: "review",
	}}
}

// siblingRenderedSize estimates the size a cell's first paragraph renders
// at: the authored size times the autofit scale generation stores, or — for
// a shape left to the renderer's own autofit — times the scale that makes
// the text fit its box when the renderer's font runs siblingRenderSlack
// wider than the measured one.
func siblingRenderedSize(m *geomMeasurer, cell shapegrid.ResolvedCell, path string) (siblingCell, bool) {
	paras := parseCellParagraphs(cell.ShapeSpec.Text)
	scale, err := writtenCellAutofitScale(cell)
	if err != nil || scale <= 0 || len(paras) == 0 {
		return siblingCell{}, false
	}
	sc := siblingCell{path: path, label: truncateForMessage(paras[0].text, 32), authored: paras[0].sizePt, written: paras[0].sizePt * scale, rendered: paras[0].sizePt * scale, shrunk: scale < 1}
	if sc.shrunk || m == nil || m.family == nil {
		return sc, true
	}
	txt := parseGeomText(cell.ShapeSpec.Text)
	if txt.rotated() || len(txt.paragraphs) == 0 {
		return sc, true
	}
	tw, th := pptx.PresetTextRect(cell.ShapeSpec.Geometry, cell.ShapeSpec.Adjustments, cell.Bounds)
	in := txt.writtenInsets(pptx.RectEmu{CX: tw, CY: th}, cell.TextInsets)
	availW := float64(geometryTextWidthEMU(cell.ShapeSpec, cell.Bounds)-in[0]-in[2]) / emuPerPt
	availH := float64(th-in[1]-in[3]) / emuPerPt
	if availW <= 0 || availH <= 0 {
		return sc, true
	}
	// Only a one-line label in a box that holds one line is at risk: there a
	// wrap the renderer adds cannot be absorbed, while a body paragraph in a
	// content-sized box has the line the engine measured for it.
	if len(txt.paragraphs) != 1 {
		return sc, true
	}
	p := txt.paragraphs[0]
	lineH := p.sizePt * geometryLineHeight
	if lines, _ := m.paragraphLines(p, availW); lines != 1 || availH >= 2*lineH {
		return sc, true
	}
	if lines, _ := m.paragraphLines(p, availW/siblingRenderSlack); lines == 1 {
		return sc, true
	}
	// Shrink in the renderer's steps until the label is back on one line at
	// the pessimistic width.
	for fit := 0.95; fit >= 0.5; fit -= 0.05 {
		q := p
		q.sizePt = p.sizePt * fit
		if lines, _ := m.paragraphLines(q, availW/siblingRenderSlack); lines == 1 {
			sc.rendered, sc.shrunk, sc.refit = sc.authored*fit, true, true
			return sc, true
		}
	}
	sc.rendered, sc.shrunk, sc.refit = sc.authored*0.5, true, true
	return sc, true
}

// siblingSizeHint is the advice of a SIBLING_SIZE_MISMATCH finding. A label
// the renderer may re-fit is also cured by a row tall enough for its second
// line: a wrap then costs a line, not the size.
func siblingSizeHint(refit bool) string {
	if refit {
		return "shorten the label that fills its box to the length of the others, use fewer columns, or make the row tall enough for two lines so a wrap costs a line and not the size"
	}
	return "shorten the shrunk labels to the length of the others, use fewer columns, or set one explicit size on every cell of the row"
}
