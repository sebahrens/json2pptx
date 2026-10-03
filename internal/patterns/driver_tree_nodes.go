package patterns

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Nodes style of driver-tree (go-slide-creator-xj2sl) — the default.
//
// The legacy tree sized the root to the whole grid and each branch to its
// leaves' rows, so the picture was three columns of slabs with thin brackets
// between them. Here every node is as tall and as wide as its label needs:
//
//	            ┌ leaf
//	   ┌ branch ┤
//	   │        └ leaf
//	root        ┌ leaf
//	   │        ├ leaf
//	   └ branch ┤
//	            └ leaf
//
// The root and each branch still span the rows of their children, but their
// shape is capped at its written height (cell max_height), which centres it
// on those rows. Columns shrink to the widest label of their level and the
// width given back goes into the gaps between the levels, where the elbow
// connectors run; what is still left stays empty to the right of the tree.
// Only the root is a solid accent box. A blank row separates the branches'
// leaf groups, and a branch's annotation sits beside its leaves behind a
// thin rule the height of the group.
//
// The connectors are explicit grid links: a row connector chains the cells
// occupying a row, and a height-capped parent no longer occupies all the rows
// it spans.

const (
	// driverTreeRowGapPt separates two leaves of one branch, and
	// driverTreeGroupGapPt is the blank row between two branches' leaves.
	driverTreeRowGapPt   = 6.0
	driverTreeGroupGapPt = 8.0
	// driverTreeCompactRowGapPt is the row gap of a tree too tall for the
	// content area (the slabs layout's gap).
	driverTreeCompactRowGapPt = 4.0
	// driverTreeMaxColGapPt is the widest gap between two levels: beyond it
	// the connectors read as rules across the slide, not as a tree.
	driverTreeMaxColGapPt = 56.0
	// driverTreeWidthSlack widens a column measured to its label: the
	// renderer's face may draw wider than the one the label was measured in.
	driverTreeWidthSlack = 1.15
	// driverTreeAnnotationRulePt is the rule that ties an annotation to its
	// branch's leaves.
	driverTreeAnnotationRulePt = 1.0
)

// Narrowest columns, as a share of the content width: a one-word tree is not
// drawn as three chips.
var driverTreeMinColFrac = [4]float64{0.14, 0.16, 0.18, 0.14}

// labelFitWidthPt (shared with arch-stack's label column) returns the narrowest width in [minPt, maxPt] at which
// text needs no more height than it does at maxPt, with driverTreeWidthSlack.
func labelFitWidthPt(fonts pptx.ThemeFonts, text json.RawMessage, minPt, maxPt float64) float64 {
	if minPt >= maxPt {
		return maxPt
	}
	base := writtenFitHeightPt(fonts, text, maxPt, 0)
	if base <= 0 {
		return maxPt
	}
	fits := func(w float64) bool {
		h := writtenFitHeightPt(fonts, text, w, 0)
		return h > 0 && h <= base
	}
	lo, hi := minPt, maxPt
	if !fits(lo) {
		for hi-lo > 2 {
			mid := (lo + hi) / 2
			if fits(mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		lo = hi
	}
	return math.Min(maxPt, math.Ceil(lo*driverTreeWidthSlack)+4)
}

// driverTreeNeedPt is the height text is written in at widthPt without
// shrinking, or one line plus the shape margins when it cannot be measured.
func driverTreeNeedPt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt, sizePt float64) float64 {
	if h := writtenFitHeightPt(fonts, text, widthPt, 0); h > 0 {
		return h
	}
	return math.Ceil(sizePt*contentLineHeight + 2*defaultShapeInsetTBPt)
}

// expandNodes renders the tree as label-sized nodes joined by elbow links.
func (dt *driverTree) expandNodes(ctx ExpandContext, vals *DriverTreeValues, ovr *DriverTreeOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	// A small tree — a root, two branches and a handful of one-line leaves —
	// drawn at the dense sizes is a cluster of chips in the middle of the
	// slide. It is laid out one type step up (18 / 14 / 14pt) when the tree at
	// those sizes still needs no more than driverTreeSparseMaxFrac of the
	// content height at one node height per leaf; a tree with real content
	// keeps the sizes its budgets are measured at (go-slide-creator-yhzxt).
	if ovr.HeaderSize == 0 && ovr.BodySize == 0 {
		_, contentH := contentAreaPt(ctx)
		for _, grow := range []float64{driverTreeSparseRowGrow, 1} {
			grid, total, uniform := dt.expandNodesAt(ctx, vals, ovr, cellOverrides, driverTreeSizes{scaleLeadPt, scaleSubheadPt, scaleSubheadPt, grow})
			if uniform && contentH > 0 && total <= contentH*driverTreeSparseMaxFrac {
				return grid, nil
			}
		}
	}
	grid, _, _ := dt.expandNodesAt(ctx, vals, ovr, cellOverrides, driverTreeSizes{
		root:   ResolveSize(ovr.HeaderSize, scaleSubheadPt),
		branch: ResolveSize(ovr.HeaderSize, scaleBodyPt),
		leaf:   ResolveSize(ovr.BodySize, sizeDenseCaptionPt),
		grow:   1,
	})
	return grid, nil
}

// driverTreeSizes are the node text sizes of one layout attempt.
// grow is the node-height growth that goes with them (1 = written fit).
type driverTreeSizes struct{ root, branch, leaf, grow float64 }

// driverTreeSparseRowGrow is the node height of a promoted tree relative to
// its written fit — the row growth the placement policy pairs with a type
// step (shapegrid composeRowScales), well inside the 1.6x content cap.
const driverTreeSparseRowGrow = 1.2

// driverTreeSparseMaxFrac is the share of the content height a tree may need
// at the promoted sizes and still count as small.
const driverTreeSparseMaxFrac = 0.85

// expandNodesAt lays the tree out at the given sizes. It also returns the
// height of the rows with their gaps and whether the first row plan (every
// leaf at one node height) fits the content area.
//
//nolint:gocognit,gocyclo // one pass builds the nodes, a second sizes columns and rows
func (dt *driverTree) expandNodesAt(ctx ExpandContext, vals *DriverTreeValues, ovr *DriverTreeOverrides, cellOverrides map[int]any, sizes driverTreeSizes) (*jsonschema.ShapeGridInput, float64, bool) {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	rootSize, branchSize, leafSize := sizes.root, sizes.branch, sizes.leaf
	fonts := ctx.themeFonts()

	totalLeaves, hasAnnotation := 0, false
	for _, b := range vals.Branches {
		totalLeaves += len(b.Leaves)
		hasAnnotation = hasAnnotation || strings.TrimSpace(b.Annotation) != ""
	}
	nBranches := len(vals.Branches)
	// Cell indices for cell_overrides: root, branches, leaves (flat), then
	// the annotations present.
	leafIdx0 := 1 + nBranches
	annotIdx0 := leafIdx0 + totalLeaves

	// --- Nodes -----------------------------------------------------------
	rootCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"` + baseAccent + `"`),
		Line:     noLine,
		Text:     buildDriverTreeNodeText(vals.Root.Label, vals.Root.Unit, rootSize, readableTextOn(ctx, fillTone{Color: baseAccent}, "lt1")),
	}}
	applyDriverTreeOverride(rootCell, cellOverrides, 0, baseAccent)

	type group struct {
		branch, annot *jsonschema.GridCellInput
		leaves        []*jsonschema.GridCellInput
	}
	groups := make([]group, nBranches)
	leafCounter, annotCounter := 0, 0
	for i, branch := range vals.Branches {
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		tone := inactiveTintTone(accent)
		g := group{branch: &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     tone.fillJSON(),
			Line:     noLine,
			Text:     buildDriverTreeNodeText(branch.Label, branch.Unit, branchSize, readableTextOn(ctx, tone, "dk1")),
		}}}
		applyDriverTreeOverride(g.branch, cellOverrides, 1+i, accent)
		for _, leaf := range branch.Leaves {
			cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     neutralFillJSON(NeutralTint8),
				Line:     noLine,
				Text:     buildDriverTreeLeafText(pptx.ConvertMarkdownEmphasis(leaf), leafSize),
			}}
			applyDriverTreeOverride(cell, cellOverrides, leafIdx0+leafCounter, accent)
			leafCounter++
			g.leaves = append(g.leaves, cell)
		}
		if note := strings.TrimSpace(branch.Annotation); note != "" {
			g.annot = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"none"`), // unboxed note: not a connector anchor
					Text:     buildDriverTreeAnnotationText(pptx.ConvertMarkdownEmphasis(note), leafSize),
				},
				AccentBar: &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: driverTreeAnnotationRulePt},
			}
			applyDriverTreeOverride(g.annot, cellOverrides, annotIdx0+annotCounter, accent)
			annotCounter++
		}
		groups[i] = g
	}

	// --- Columns ---------------------------------------------------------
	// The widest a column gets is its share in the slabs layout, so the
	// character budgets measured there still hold.
	contentW, contentH := contentAreaPt(ctx)
	maxPct := []float64{22, 28, 50}
	if hasAnnotation {
		maxPct = []float64{20, 24, 36, 20}
	}
	levels := len(maxPct)
	minGap := ctx.Gap(driverTreeColGapPt)
	usable := contentW - float64(levels-1)*minGap
	texts := make([][]json.RawMessage, levels)
	texts[0] = []json.RawMessage{rootCell.Shape.Text}
	for _, g := range groups {
		texts[1] = append(texts[1], g.branch.Shape.Text)
		for _, leaf := range g.leaves {
			texts[2] = append(texts[2], leaf.Shape.Text)
		}
		if g.annot != nil {
			texts[3] = append(texts[3], g.annot.Shape.Text)
		}
	}
	widths := make([]float64, levels)
	sumW := 0.0
	for c := range widths {
		maxW := usable * maxPct[c] / 100
		minW := math.Min(contentW*driverTreeMinColFrac[c], maxW)
		widths[c] = minW
		for _, text := range texts[c] {
			widths[c] = math.Max(widths[c], labelFitWidthPt(fonts, text, minW, maxW))
		}
		sumW += widths[c]
	}
	// The width the columns gave back widens the gaps, then stays empty in a
	// trailing column.
	colGap := math.Min((contentW-sumW)/float64(levels-1), ctx.Gap(driverTreeMaxColGapPt))
	colGap = math.Max(colGap, minGap)
	cols := append([]float64(nil), widths...)
	if rest := contentW - sumW - float64(levels)*colGap; rest > 1 {
		cols = append(cols, rest)
	}
	colsJSON, _ := json.Marshal(cols)

	// --- Rows ------------------------------------------------------------
	// A leaf row is as tall as its text needs, and a branch whose own label
	// (or annotation) needs more than its leaves stretches its rows. Three
	// plans, the first that fits the content area:
	//   1. every leaf row as tall as the tallest leaf — one node height;
	//   2. every leaf row at its own height;
	//   3. compact: no blank rows between the branches, the tighter row gap,
	//      and the rows sharing the whole height in proportion to their needs
	//      — the slabs layout's row geometry, so its character budgets hold;
	//      the writer shrinks what still does not fit and PostExpandWarnings
	//      names the content to cut.
	leafNeed := make([][]float64, nBranches)
	tallest := 0.0
	branchNeed := make([]float64, nBranches)
	groupNeed := make([]float64, nBranches)
	for i, g := range groups {
		for _, leaf := range g.leaves {
			h := driverTreeNeedPt(fonts, leaf.Shape.Text, widths[2], leafSize)
			leafNeed[i] = append(leafNeed[i], h)
			tallest = math.Max(tallest, h)
		}
		branchNeed[i] = driverTreeNeedPt(fonts, g.branch.Shape.Text, widths[1], branchSize)
		groupNeed[i] = branchNeed[i]
		if g.annot != nil {
			groupNeed[i] = math.Max(groupNeed[i], driverTreeNeedPt(fonts, g.annot.Shape.Text, widths[3], leafSize))
		}
	}
	// plan returns the row heights (by branch, by leaf) and their sum with
	// the gaps, for leaves at least floorPt tall.
	plan := func(floorPt, gap, groupGap float64) ([][]float64, float64) {
		heights := make([][]float64, nBranches)
		total := float64(nBranches-1) * groupGap
		for i := range groups {
			n := float64(len(leafNeed[i]))
			sum := (n - 1) * gap
			for _, h := range leafNeed[i] {
				h = math.Ceil(math.Max(h, floorPt))
				heights[i] = append(heights[i], h)
				sum += h
			}
			if short := groupNeed[i] - sum; short > 0 {
				for j := range heights[i] {
					heights[i][j] += math.Ceil(short / n)
				}
				sum += n * math.Ceil(short/n)
			}
			total += sum
		}
		return heights, total
	}
	rowGap := ctx.Gap(driverTreeRowGapPt)
	groupGap := driverTreeGroupGapPt + 2*rowGap
	fits := func(total float64) bool { return contentH <= 0 || total <= contentH+1 }
	rowH, total := plan(tallest*sizes.grow, rowGap, groupGap)
	uniform := fits(total)
	blockPt := total + float64(max(totalLeaves+nBranches-2, 0))*rowGap
	if !fits(total) {
		rowH, total = plan(0, rowGap, groupGap)
	}
	compact := !fits(total)
	if compact {
		rowGap = ctx.Gap(driverTreeCompactRowGapPt)
		rowH, _ = plan(0, rowGap, rowGap)
	}

	var rows []jsonschema.GridRowInput
	var links []jsonschema.GridLinkInput
	link := func(fromRow, fromCol, toRow, toCol int) {
		links = append(links, jsonschema.GridLinkInput{
			From:      [2]int{fromRow, fromCol},
			To:        [2]int{toRow, toCol},
			Connector: &jsonschema.ConnectorSpecInput{Style: "line", Color: baseAccent, Width: 1.0},
		})
	}
	spacers := nBranches - 1
	if compact {
		spacers = 0
	}
	for i, g := range groups {
		if i > 0 && !compact {
			rows = append(rows, jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{{}}, MinHeight: driverTreeGroupGapPt, MaxHeight: driverTreeGroupGapPt})
		}
		first := len(rows)
		for j, leaf := range g.leaves {
			var cells []*jsonschema.GridCellInput
			if i == 0 && j == 0 {
				rootCell.RowSpan = totalLeaves + spacers
				rootCell.MaxHeight = math.Ceil(sizes.grow * driverTreeNeedPt(fonts, rootCell.Shape.Text, widths[0], rootSize))
				cells = append(cells, rootCell)
			}
			if j == 0 {
				g.branch.RowSpan = len(g.leaves)
				g.branch.MaxHeight = math.Ceil(sizes.grow * branchNeed[i])
				cells = append(cells, g.branch)
				link(0, 0, first, 1)
			}
			cells = append(cells, leaf)
			link(first, 1, len(rows), 2)
			if j == 0 && g.annot != nil {
				g.annot.RowSpan = len(g.leaves)
				cells = append(cells, g.annot)
			}
			row := jsonschema.GridRowInput{Cells: cells, MinHeight: rowH[i][j], MaxHeight: rowH[i][j]}
			if compact {
				row = jsonschema.GridRowInput{Cells: cells, Flex: rowH[i][j]}
			}
			rows = append(rows, row)
		}
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        colGap,
		RowGap:        rowGap,
		Rows:          rows,
		Links:         links,
		VerticalAlign: GridVerticalAlignDefault,
	}, blockPt, uniform
}
