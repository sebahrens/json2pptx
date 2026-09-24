package main

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

// cellBudgetEntry is a single cell's text capacity and density in the expand response.
type cellBudgetEntry struct {
	CellIndex   int     `json:"cell_index"`
	Row         int     `json:"row"`
	Col         int     `json:"col"`
	MaxChars    int     `json:"max_chars"`
	ActualChars int     `json:"actual_chars"`
	DensityPct  int     `json:"density_pct"`
	Status      string  `json:"status"`
	FontSizePt  float64 `json:"font_size_pt"`
}

// cellDensityWarning flags a cell that is underfilled or overflowing.
type cellDensityWarning struct {
	CellIndex    int                          `json:"cell_index"`
	Field        string                       `json:"field"`
	Actual       int                          `json:"actual"`
	Budget       int                          `json:"budget"`
	Status       string                       `json:"status"`
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// computeCellBudgets resolves an expanded ShapeGridInput into cell budgets and
// density warnings using the textcapacity package. The layout bounds from the
// ExpandContext determine the content area.
//
// Returns nil slices (not errors) when the grid cannot be resolved — this keeps
// the expand response valid even when budget computation fails.
func computeCellBudgets(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext) ([]cellBudgetEntry, []cellDensityWarning) {
	result, _ := resolveCapacityGrid(grid, ctx)
	if result == nil {
		return nil, nil
	}

	// Compute densities
	densities := textcapacity.ForResolvedGrid(result)
	if len(densities) == 0 {
		return nil, nil
	}

	// Map resolved cells back to row/col positions
	budgets := make([]cellBudgetEntry, 0, len(densities))
	var warnings []cellDensityWarning
	type measuredCell struct {
		density textcapacity.Density
		cell    shapegrid.ResolvedCell
	}
	measured := make(map[[2]int]measuredCell, len(result.Cells))
	for i, cell := range result.Cells {
		// Resolved ColIdx is a physical grid column. A preceding col_span or
		// row_span can make it differ from the authored cells[] index.
		authored := gridCellAtResolved(grid, cell.RowIdx, cell.ColIdx)
		if authored == nil {
			continue
		}
		colIdx := -1
		for j, candidate := range grid.Rows[cell.RowIdx].Cells {
			if candidate == authored {
				colIdx = j
				break
			}
		}
		if colIdx < 0 {
			continue
		}
		key := [2]int{cell.RowIdx, colIdx}
		previous, exists := measured[key]
		// A composite resolves to text and diagram children at the same source
		// coordinate. Its text child owns the authored cell's character budget.
		if !exists || cell.ShapeSpec != nil && previous.cell.ShapeSpec == nil {
			measured[key] = measuredCell{density: densities[i], cell: cell}
		}
	}

	cellIdx := 0
	for rowIdx, row := range grid.Rows {
		for colIdx := range row.Cells {
			m, exists := measured[[2]int{rowIdx, colIdx}]
			if !exists {
				m.density.Status = textcapacity.StatusUnderfilled
			}
			d := m.density
			budgets = append(budgets, cellBudgetEntry{
				CellIndex:   cellIdx,
				Row:         rowIdx,
				Col:         colIdx,
				MaxChars:    d.MaxChars,
				ActualChars: d.ActualChars,
				DensityPct:  d.DensityPct,
				Status:      string(d.Status),
				FontSizePt:  d.FontPt,
			})

			// Emit warning for non-optimal cells that have content
			if d.Status != textcapacity.StatusOptimal && d.ActualChars > 0 {
				field := "body"
				if m.cell.ShapeSpec != nil {
					field = inferCellField(m.cell.ShapeSpec.Text)
				}
				warnings = append(warnings, cellDensityWarning{
					CellIndex: cellIdx,
					Field:     field,
					Actual:    d.ActualChars,
					Budget:    d.MaxChars,
					Status:    string(d.Status),
				})
			}
			cellIdx++
		}
	}

	return budgets, warnings
}

func resolveCapacityGrid(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext) (*shapegrid.ResolveResult, pptx.RectEmu) {
	if grid == nil || len(grid.Rows) == 0 {
		return nil, pptx.RectEmu{}
	}

	// Convert DTO columns to []float64
	colWidths, err := resolveColumnsDTO(grid.Columns, grid.Rows)
	if err != nil {
		return nil, pptx.RectEmu{}
	}

	// Resolve gaps
	colGap := grid.ColGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}

	// Convert DTO rows to shapegrid.Row
	rows := convertGridRows(grid.Rows)

	// Build bounds from ExpandContext layout bounds
	bounds := pptx.RectEmu{
		X:  ctx.LayoutBounds.X,
		Y:  ctx.LayoutBounds.Y,
		CX: ctx.LayoutBounds.Width,
		CY: ctx.LayoutBounds.Height,
	}

	// If grid has explicit bounds, use percentage-based resolution instead
	if grid.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(
			grid.Bounds.X, grid.Bounds.Y,
			grid.Bounds.Width, grid.Bounds.Height,
			ctx.SlideWidth, ctx.SlideHeight,
		)
	}

	vAlign, _ := shapegrid.ParseVerticalAlign(grid.VerticalAlign)
	sgGrid := &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    rows,
		ColGap:  colGap,
		RowGap:  rowGap,
		VAlign:  vAlign,
	}

	// Validate before resolving
	if vErr := shapegrid.Validate(sgGrid); vErr != nil {
		return nil, bounds
	}

	// Resolve with a dummy allocator (we only need cell bounds, not shape IDs)
	alloc := pptx.NewShapeIDAllocator(nil)
	result, err := shapegrid.Resolve(sgGrid, alloc)
	if err != nil || result == nil {
		return nil, bounds
	}
	return result, bounds
}

// gridInkHeightPct measures visible content vertically rather than counting
// occupied slots. Parallel cells share a row, so their heights are maxed; rows
// stack. Non-text visuals count as their drawn height, not as empty text.
func gridInkHeightPct(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext) float64 {
	resolved, bounds := resolveCapacityGrid(grid, ctx)
	if resolved == nil || bounds.CY <= 0 {
		return 0
	}
	densities := textcapacity.ForResolvedGrid(resolved)
	rowInkPt := make([]float64, len(grid.Rows))
	for i, cell := range resolved.Cells {
		if cell.RowIdx < 0 || cell.RowIdx >= len(rowInkPt) {
			continue
		}
		inkPt := densities[i].RequiredHeightPt
		if cell.Kind != shapegrid.CellKindShape && cell.Kind != shapegrid.CellKindSubGrid {
			inkPt = float64(cell.Bounds.CY) / 12700
		}
		if cell.IconBounds.CY > 0 {
			inkPt = math.Max(inkPt, float64(cell.IconBounds.CY)/12700)
		}
		rowInkPt[cell.RowIdx] = math.Max(rowInkPt[cell.RowIdx], inkPt)
	}
	var totalInkPt float64
	for _, h := range rowInkPt {
		totalInkPt += h
	}
	return math.Round(math.Min(100, totalInkPt/(float64(bounds.CY)/12700)*100)*10) / 10
}

// inkUnderfillWarning is a grid-level advisory. Low-density patterns are
// intentionally airy, so a low ink ratio is actionable only for multi-cell
// text patterns that have not already been height-constrained by the caller.
func inkUnderfillWarning(occupancy gridOccupancy, budgets []cellBudgetEntry, pat patterns.Pattern, pi *PatternInput) *cellDensityWarning {
	const threshold = 40
	if pi == nil || pi.Bounds != nil || pi.MaxHeightPct > 0 || pat.Taxonomy().DensityClass == "low" || occupancy.InkHeightPct <= 0 || occupancy.InkHeightPct >= threshold {
		return nil
	}
	contentCells := 0
	for _, b := range budgets {
		if b.ActualChars > 0 {
			contentCells++
		}
	}
	if contentCells < 3 {
		return nil
	}
	suggestedPct := int(math.Ceil(occupancy.InkHeightPct / 0.7))
	suggestedPct = max(20, min(90, suggestedPct))
	return &cellDensityWarning{
		CellIndex: -1,
		Field:     "layout",
		Actual:    int(math.Round(occupancy.InkHeightPct)),
		Budget:    threshold,
		Status:    "underfilled_ink",
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool: "expand_pattern",
			ArgsTemplate: map[string]any{
				"name":           pi.Name,
				"max_height_pct": suggestedPct,
			},
		},
	}
}

// computePatternCellBudgets keeps card-grid's body-only, pre-authoring budget
// in sync with its BODY_TOO_LONG warning. The generic resolved-grid budget is
// a whole-cell, post-content height estimate; using it for a content-sized card
// can make the "maximum" rise as its body grows.
func computePatternCellBudgets(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, pi *PatternInput) ([]cellBudgetEntry, []cellDensityWarning) {
	budgets, warnings := computeCellBudgets(grid, ctx)
	if pi == nil || pi.Name != "card-grid" {
		return budgets, warnings
	}
	var values patterns.CardGridValues
	if err := json.Unmarshal(pi.Values, &values); err != nil {
		return budgets, warnings
	}
	var overrides patterns.CardGridOverrides
	if len(pi.Overrides) > 0 {
		if err := json.Unmarshal(pi.Overrides, &overrides); err != nil {
			return budgets, warnings
		}
	}
	budgetCtx := ctx
	if b, relative := resolvePatternBounds(pi); b != nil {
		budgetCtx.LayoutBounds = patternExpansionBounds(ctx, b, relative)
	}
	budgetCtx = cardGridBudgetContext(budgetCtx, pi.Callout)
	bodyBudgets := patterns.CardGridBodyBudgets(budgetCtx, &values, &overrides)
	if len(budgets) == 0 {
		// A severely overfull content-sized grid may not resolve at all. The
		// pre-authoring budget still exists and must not disappear with it.
		budgets = make([]cellBudgetEntry, len(bodyBudgets))
		for i := range budgets {
			budgets[i] = cellBudgetEntry{CellIndex: i, Row: i / values.Columns, Col: i % values.Columns}
		}
	}
	limit := min(len(budgets), len(bodyBudgets))
	kept := warnings[:0]
	for _, w := range warnings {
		if w.CellIndex >= limit {
			kept = append(kept, w)
		}
	}
	warnings = kept
	for i := 0; i < limit; i++ {
		b := &budgets[i]
		b.MaxChars = bodyBudgets[i]
		b.ActualChars = len([]rune(values.Cells[i].Body))
		b.FontSizePt = patterns.ResolveSize(overrides.BodySize, 12)
		b.DensityPct = 0
		if b.MaxChars > 0 {
			b.DensityPct = int(float64(b.ActualChars)/float64(b.MaxChars)*100 + 0.5)
		} else if b.ActualChars > 0 {
			// Zero capacity is overfull, not an empty card. Saturate the
			// percentage so downstream ranking cannot mistake it for sparse.
			b.DensityPct = 1000
		}
		switch {
		case b.ActualChars > b.MaxChars:
			b.Status = string(textcapacity.StatusOverflow)
		case b.DensityPct < textcapacity.UnderfilledPct:
			b.Status = string(textcapacity.StatusUnderfilled)
		default:
			b.Status = string(textcapacity.StatusOptimal)
		}
		if b.ActualChars > 0 && b.Status != string(textcapacity.StatusOptimal) {
			warnings = append(warnings, cellDensityWarning{
				CellIndex: b.CellIndex,
				Field:     "body",
				Actual:    b.ActualChars,
				Budget:    b.MaxChars,
				Status:    b.Status,
			})
		}
	}
	return budgets, warnings
}

// cardGridBudgetContext reserves the callout's auto-height row before sizing
// card bodies. The shape-grid resolver gives an auto row at least 8% of the
// grid; its text estimate is 1.2 line heights plus the default 7.2pt vertical
// inset, and the extra row introduces one 10pt gap.
func cardGridBudgetContext(ctx patterns.ExpandContext, callout *patterns.PatternCallout) patterns.ExpandContext {
	if callout == nil || ctx.LayoutBounds.Height <= 0 {
		return ctx
	}
	areaH := float64(ctx.LayoutBounds.Height) / 12700
	lines := strings.Count(callout.Text, "\n") + 1
	reserved := math.Max(areaH*0.08, float64(lines)*14*1.2+7.2) + 10
	remaining := ctx.LayoutBounds.Height - int64(reserved*12700)
	if remaining < 1 {
		remaining = 1
	}
	ctx.LayoutBounds.Height = remaining
	return ctx
}

func postExpandWarningContext(ctx patterns.ExpandContext, pi *PatternInput) patterns.ExpandContext {
	if pi != nil && pi.Name == "card-grid" {
		return cardGridBudgetContext(ctx, pi.Callout)
	}
	return ctx
}

// sparseLayoutWarning checks whether the average cell density across all
// content-bearing cells is below the pattern's sparse threshold. When it is,
// the pattern is likely to produce comically tall blocks. Returns nil when
// density is adequate or there are no content cells to measure.
func sparseLayoutWarning(budgets []cellBudgetEntry, pat patterns.Pattern, patternName string, pi *PatternInput) *cellDensityWarning {
	if len(budgets) == 0 {
		return nil
	}
	// Skip if the caller already constrained bounds
	if pi.Bounds != nil || pi.MaxHeightPct > 0 {
		return nil
	}

	// Compute average density across cells that have content
	var totalDensity, contentCells int
	for _, b := range budgets {
		if b.ActualChars > 0 {
			totalDensity += b.DensityPct
			contentCells++
		}
	}
	if contentCells == 0 {
		return nil
	}
	avgDensity := totalDensity / contentCells

	threshold := pat.Taxonomy().EffectiveSparseThreshold(20)
	if avgDensity >= threshold {
		return nil
	}

	// Suggest a max_height_pct that would bring content to ~70% fill
	suggestedPct := int(float64(avgDensity) / 0.7)
	if suggestedPct < 20 {
		suggestedPct = 20
	}
	if suggestedPct > 90 {
		suggestedPct = 90
	}

	return &cellDensityWarning{
		CellIndex: -1, // grid-level, not cell-specific
		Field:     "layout",
		Actual:    avgDensity,
		Budget:    threshold,
		Status:    "sparse_layout",
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool: "expand_pattern",
			ArgsTemplate: map[string]any{
				"name":           patternName,
				"max_height_pct": suggestedPct,
			},
		},
	}
}

// densityClassWarning checks whether the average cell density diverges from
// the pattern's declared DensityClass. When a medium-density pattern has very
// sparse content (<15% avg density) or a high-density pattern is underfilled
// (<30% avg density), it emits a capacity warning with actionable suggestions:
// compact variant, max_height_pct override, or pattern swap.
func densityClassWarning(budgets []cellBudgetEntry, pat patterns.Pattern, patternName string, pi *PatternInput, reg *patterns.Registry) *cellDensityWarning {
	if len(budgets) == 0 {
		return nil
	}
	// Skip if the caller already constrained bounds
	if pi.Bounds != nil || pi.MaxHeightPct > 0 {
		return nil
	}

	tax := pat.Taxonomy()
	densityClass := tax.DensityClass

	// Only medium and high density patterns have divergence thresholds
	var threshold int
	switch densityClass {
	case "medium":
		threshold = 15
	case "high":
		threshold = 30
	default:
		return nil // low-density patterns are expected to be sparse
	}

	// Compute average density across cells that have content
	var totalDensity, contentCells int
	for _, b := range budgets {
		if b.ActualChars > 0 {
			totalDensity += b.DensityPct
			contentCells++
		}
	}
	if contentCells == 0 {
		return nil
	}
	avgDensity := totalDensity / contentCells

	if avgDensity >= threshold {
		return nil
	}

	// Build actionable suggestion
	suggestion := &patterns.ToolCallSuggestion{
		Tool:         "expand_pattern",
		ArgsTemplate: map[string]any{"name": patternName},
	}

	// Check for a compact variant in the registry
	compactName := patternName + "-compact"
	if reg != nil {
		if _, ok := reg.Get(compactName); ok {
			suggestion.ArgsTemplate["name"] = compactName
			return &cellDensityWarning{
				CellIndex:    -1,
				Field:        "density_class",
				Actual:       avgDensity,
				Budget:       threshold,
				Status:       "density_class_divergence",
				NextToolCall: suggestion,
			}
		}
	}

	// Suggest a max_height_pct that would bring content to ~70% fill
	suggestedPct := int(float64(avgDensity) / 0.7)
	if suggestedPct < 20 {
		suggestedPct = 20
	}
	if suggestedPct > 90 {
		suggestedPct = 90
	}
	suggestion.ArgsTemplate["max_height_pct"] = suggestedPct

	return &cellDensityWarning{
		CellIndex:    -1,
		Field:        "density_class",
		Actual:       avgDensity,
		Budget:       threshold,
		Status:       "density_class_divergence",
		NextToolCall: suggestion,
	}
}

// inferCellField examines shape text JSON to determine whether the cell
// contains a "title", "header", or generic "body" text.
func inferCellField(text json.RawMessage) string {
	if len(text) == 0 {
		return "body"
	}
	// Simple heuristic: if it's a short string or has small font, it's likely a label
	var obj struct {
		Paragraphs []struct {
			Size float64 `json:"size,omitempty"`
		} `json:"paragraphs,omitempty"`
		Size float64 `json:"size,omitempty"`
	}
	if err := json.Unmarshal(text, &obj); err == nil {
		if obj.Size >= 18 || (len(obj.Paragraphs) > 0 && obj.Paragraphs[0].Size >= 18) {
			return "header"
		}
	}
	return "body"
}
