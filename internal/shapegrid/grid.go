package shapegrid

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Resolve converts a Grid into a ResolveResult containing resolved cells and
// connectors with absolute EMU coordinates and allocated shape IDs.
func Resolve(grid *Grid, alloc *pptx.ShapeIDAllocator) (*ResolveResult, error) { //nolint:gocognit,gocyclo
	if grid == nil || len(grid.Rows) == 0 {
		return nil, nil
	}

	numCols := len(grid.Columns)
	numRows := len(grid.Rows)

	// Resolve gaps (default 8pt). Values are in typographic points.
	colGap := grid.ColGap
	if colGap == 0 {
		colGap = 8
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = 8
	}

	gridX := grid.Bounds.X
	gridY := grid.Bounds.Y
	gridW := grid.Bounds.CX
	gridH := grid.Bounds.CY

	// Convert point values to EMU (1pt = 12700 EMU)
	colGapEMU := PtToEMU(colGap)
	rowGapEMU := PtToEMU(rowGap)

	// Available width/height after subtracting gaps
	totalColGap := colGapEMU * int64(numCols-1)
	totalRowGap := rowGapEMU * int64(numRows-1)
	availW := gridW - totalColGap
	availH := gridH - totalRowGap

	// Compute absolute column positions and widths (truncation-safe)
	colWidthsEMU := distributeEMU(grid.Columns, availW)
	colXOffsets := make([]int64, numCols)
	x := gridX
	for c := 0; c < numCols; c++ {
		colXOffsets[c] = x
		x += colWidthsEMU[c] + colGapEMU
	}

	// Bounds are authoritative — never shrink. Row heights are distributed
	// using CSS-flex-like semantics within the available height.

	// Resolve row heights
	rowHeights := resolveRowHeights(grid.Rows, availH)

	// Detect rows whose content exceeds max_height.
	var rowOverflows []RowOverflow
	for i, row := range grid.Rows {
		if row.MaxHeight <= 0 {
			continue
		}
		contentEMU := estimateRowTextHeightEMU(row)
		contentPt := float64(contentEMU) / 12700.0
		if contentPt > row.MaxHeight {
			rowOverflows = append(rowOverflows, RowOverflow{
				RowIndex:    i,
				ContentPt:   contentPt,
				MaxHeightPt: row.MaxHeight,
			})
		}
	}

	// Compute absolute row positions and heights (truncation-safe)
	rowHeightsEMU := distributeEMU(rowHeights, availH)
	rowYOffsets := make([]int64, numRows)
	y := gridY
	for r := 0; r < numRows; r++ {
		rowYOffsets[r] = y
		y += rowHeightsEMU[r] + rowGapEMU
	}

	// Track which cells are occupied by spans
	occupied := make([][]bool, numRows)
	for r := range occupied {
		occupied[r] = make([]bool, numCols)
	}

	var cells []ResolvedCell
	var accentBars []ResolvedAccentBar
	// rowCellIDs tracks resolved cell IDs per row for connector generation
	rowCellIDs := make([][]int, numRows) // index into cells slice

	for r, row := range grid.Rows {
		col := 0
		for _, cell := range row.Cells {
			// Skip occupied cells (from previous row_span)
			for col < numCols && occupied[r][col] {
				col++
			}
			if col >= numCols {
				break
			}

			if cell.Shape == nil && cell.TableSpec == nil && cell.Icon == nil && cell.Image == nil && cell.DiagramSpec == nil {
				col++
				continue
			}

			colSpan := cell.ColSpan
			if colSpan < 1 {
				colSpan = 1
			}
			rowSpan := cell.RowSpan
			if rowSpan < 1 {
				rowSpan = 1
			}

			// Mark spanned cells as occupied
			for dr := 0; dr < rowSpan && r+dr < numRows; dr++ {
				for dc := 0; dc < colSpan && col+dc < numCols; dc++ {
					occupied[r+dr][col+dc] = true
				}
			}

			// Compute cell bounds
			cellX := colXOffsets[col]
			cellY := rowYOffsets[r]

			// Width: sum of spanned columns + gaps between them
			endCol := col + colSpan - 1
			if endCol >= numCols {
				endCol = numCols - 1
			}
			cellW := (colXOffsets[endCol] + colWidthsEMU[endCol]) - cellX

			// Height: sum of spanned rows + gaps between them
			endRow := r + rowSpan - 1
			if endRow >= numRows {
				endRow = numRows - 1
			}
			cellH := (rowYOffsets[endRow] + rowHeightsEMU[endRow]) - cellY

			cellRect := pptx.RectEmu{
				X: cellX, Y: cellY, CX: cellW, CY: cellH,
			}

			// Icons and images default to contain mode to preserve aspect ratio.
			// Shapes and tables keep the default stretch behavior.
			// Exception: shape+icon combos keep shape at stretch and contain the icon separately.
			fitMode := cell.Fit
			hasShapeWithIcon := cell.Shape != nil && cell.Icon != nil
			if fitMode == FitStretch && !hasShapeWithIcon && (cell.Icon != nil || cell.Image != nil) {
				fitMode = FitContain
			}

			rc := ResolvedCell{
				Bounds:     ApplyFitMode(fitMode, cellRect),
				CellBounds: cellRect,
				ID:         alloc.Alloc(),
				RowIdx:     r,
				ColIdx:     col,
				Group:      cell.Group,
			}
			if cell.Image != nil {
				rc.Kind = CellKindImage
				rc.ImageSpec = cell.Image
			} else if cell.DiagramSpec != nil {
				rc.Kind = CellKindDiagram
				rc.DiagramSpec = cell.DiagramSpec
			} else if hasShapeWithIcon {
				// Shape with icon overlay: shape stretches to fill the cell,
				// icon is contained (square) within the shape bounds, scaled down.
				rc.Kind = CellKindShape
				rc.ShapeSpec = cell.Shape
				rc.IconSpec = cell.Icon
				hasText := cell.Shape != nil && hasNonEmptyText(cell.Shape.Text)
				layout := iconOverlayBounds(cell.Icon, rc.Bounds, hasText)
				rc.IconBounds = layout.Bounds
				rc.TextInsets = layout.TextInsets
			} else if cell.Icon != nil {
				rc.Kind = CellKindIcon
				rc.IconSpec = cell.Icon
			} else if cell.TableSpec != nil {
				rc.Kind = CellKindTable
				rc.TableSpec = cell.TableSpec
			} else {
				rc.Kind = CellKindShape
				rc.ShapeSpec = cell.Shape
			}
			rowCellIDs[r] = append(rowCellIDs[r], len(cells))
			cells = append(cells, rc)

			// Generate accent bar if specified
			if cell.AccentBar != nil {
				accentBars = append(accentBars, ResolvedAccentBar{
					Bounds: accentBarBounds(cellRect, cell.AccentBar),
					ID:     alloc.Alloc(),
					Spec:   cell.AccentBar,
				})
			}

			col += colSpan
		}
	}

	// Generate connectors between adjacent cells in rows that have a connector spec
	var connectors []ResolvedConnector
	for r, row := range grid.Rows {
		if row.Connector == nil || len(rowCellIDs[r]) < 2 {
			continue
		}
		for i := 0; i < len(rowCellIDs[r])-1; i++ {
			srcCell := cells[rowCellIDs[r][i]]
			tgtCell := cells[rowCellIDs[r][i+1]]

			// Route connector between actual shape edges (not cell bounds)
			srcOpts := pptx.ShapeOptions{Bounds: srcCell.Bounds}
			tgtOpts := pptx.ShapeOptions{Bounds: tgtCell.Bounds}
			if srcCell.ShapeSpec != nil {
				srcOpts.Geometry = pptx.PresetGeometry(srcCell.ShapeSpec.Geometry)
			}
			if tgtCell.ShapeSpec != nil {
				tgtOpts.Geometry = pptx.PresetGeometry(tgtCell.ShapeSpec.Geometry)
			}
			bounds, startSite, endSite := pptx.RouteBetween(srcOpts, tgtOpts)

			connectors = append(connectors, ResolvedConnector{
				Bounds:    bounds,
				ID:        alloc.Alloc(),
				Spec:      row.Connector,
				SourceID:  srcCell.ID,
				TargetID:  tgtCell.ID,
				StartSite: startSite,
				EndSite:   endSite,
			})
		}
	}

	return &ResolveResult{
		Cells:        cells,
		Connectors:   connectors,
		AccentBars:   accentBars,
		RowOverflows: rowOverflows,
	}, nil
}

// ResolveColumns parses column specifications and returns percentage widths.
// It accepts either a count (equal split) or an explicit array of percentages.
// If columns is nil, it infers the column count from the maximum cell count across rows.
func ResolveColumns(columns interface{}, rowCellCounts []int) ([]float64, error) {
	switch v := columns.(type) {
	case nil:
		maxCols := 0
		for _, n := range rowCellCounts {
			if n > maxCols {
				maxCols = n
			}
		}
		if maxCols == 0 {
			return nil, fmt.Errorf("shape_grid: no cells defined; add cells with a \"shape\", \"table\", \"icon\", or \"image\" key to at least one row")
		}
		return equalSplit(maxCols), nil
	case int:
		if v < 1 {
			return nil, fmt.Errorf("shape_grid: columns must be >= 1, got %d; set columns to a positive integer (e.g. 3) for equal-width columns", v)
		}
		return equalSplit(v), nil
	case []float64:
		if len(v) == 0 {
			return nil, fmt.Errorf("shape_grid: columns array must not be empty; provide percentage widths (e.g. [30, 40, 30]) or use a number for equal columns")
		}
		return v, nil
	default:
		return nil, fmt.Errorf("shape_grid: columns must be a number or array of numbers; use an integer (e.g. 3) for equal columns, or an array of percentages (e.g. [30, 40, 30])")
	}
}

// resolveRowHeights returns percentage heights for each row, summing to ~100.
// It uses CSS-flex-like semantics:
//  1. Fixed rows (Height > 0): percentage allocated directly.
//  2. Auto-height rows (AutoHeight): estimated from content, clamped by min/max.
//  3. Flex rows (Height == 0, !AutoHeight): remaining space distributed
//     proportionally by Flex factor (default 1).
//
// MinHeight and MaxHeight (in points) are applied after initial allocation.
// availHeightEMU is the available grid height in EMU (after gaps).
func resolveRowHeights(rows []Row, availHeightEMU int64) []float64 { //nolint:gocognit
	n := len(rows)
	heights := make([]float64, n)
	if n == 0 || availHeightEMU <= 0 {
		return heights
	}

	classes := classifyRows(rows, heights, availHeightEMU)

	// Apply min/max constraints to auto-height rows.
	for i, row := range rows {
		if classes[i] == rowClassAuto {
			heights[i] = clampHeightPct(heights[i], row.MinHeight, row.MaxHeight, availHeightEMU)
		}
	}

	// Sum non-flex allocations.
	var nonFlexPct float64
	for i, h := range heights {
		if classes[i] != rowClassFlex {
			nonFlexPct += h
		}
	}

	// Distribute remaining space to flex rows proportionally.
	distributeFlex(rows, classes, heights, 100.0-nonFlexPct)

	// Apply min/max constraints to flex rows with iterative clamping.
	clampFlexRows(rows, classes, heights, availHeightEMU)

	return heights
}

// rowClass identifies how a row's height is determined.
type rowClass int

const (
	rowClassFixed rowClass = iota
	rowClassAuto
	rowClassFlex
)

// classifyRows assigns a class to each row and sets initial heights.
func classifyRows(rows []Row, heights []float64, availHeightEMU int64) []rowClass {
	classes := make([]rowClass, len(rows))
	for i, row := range rows {
		switch {
		case row.Height > 0:
			classes[i] = rowClassFixed
			heights[i] = row.Height
		case row.AutoHeight:
			classes[i] = rowClassAuto
			heights[i] = estimateRowHeightPct(row, availHeightEMU)
		default:
			classes[i] = rowClassFlex
		}
	}
	return classes
}

// effectiveFlex returns the flex factor for a row, defaulting to 1.
func effectiveFlex(row Row) float64 {
	if row.Flex > 0 {
		return row.Flex
	}
	return 1
}

// distributeFlex assigns remaining percentage space to flex rows proportionally.
func distributeFlex(rows []Row, classes []rowClass, heights []float64, remainingPct float64) {
	if remainingPct < 0 {
		remainingPct = 0
	}
	var totalFlex float64
	for i := range rows {
		if classes[i] == rowClassFlex {
			totalFlex += effectiveFlex(rows[i])
		}
	}
	if totalFlex > 0 {
		for i := range rows {
			if classes[i] == rowClassFlex {
				heights[i] = remainingPct * effectiveFlex(rows[i]) / totalFlex
			}
		}
	}
}

// clampFlexRows iteratively applies min/max constraints to flex rows,
// redistributing freed space among unclamped flex rows.
func clampFlexRows(rows []Row, classes []rowClass, heights []float64, availHeightEMU int64) {
	for iter := 0; iter < len(rows); iter++ {
		clamped := false
		var flexTotal float64
		for i := range rows {
			if classes[i] != rowClassFlex {
				continue
			}
			minPct := ptToPct(rows[i].MinHeight, availHeightEMU)
			maxPct := ptToPct(rows[i].MaxHeight, availHeightEMU)
			if minPct > 0 && heights[i] < minPct {
				heights[i] = minPct
				classes[i] = rowClassFixed
				clamped = true
			} else if maxPct > 0 && heights[i] > maxPct {
				heights[i] = maxPct
				classes[i] = rowClassFixed
				clamped = true
			} else {
				flexTotal += effectiveFlex(rows[i])
			}
		}
		if !clamped {
			break
		}
		// Recalculate remaining space (only count non-flex rows).
		var fixedPct float64
		for i, h := range heights {
			if classes[i] != rowClassFlex {
				fixedPct += h
			}
		}
		distributeFlex(rows, classes, heights, 100.0-fixedPct)
	}
}

// clampHeightPct applies min/max point constraints to a percentage height.
func clampHeightPct(pct, minPt, maxPt float64, availHeightEMU int64) float64 {
	if minPt > 0 {
		minPct := ptToPct(minPt, availHeightEMU)
		if pct < minPct {
			pct = minPct
		}
	}
	if maxPt > 0 {
		maxPct := ptToPct(maxPt, availHeightEMU)
		if pct > maxPct {
			pct = maxPct
		}
	}
	return pct
}

// ptToPct converts a point value to a percentage of available height.
func ptToPct(pt float64, availHeightEMU int64) float64 {
	if pt <= 0 || availHeightEMU <= 0 {
		return 0
	}
	return (pt * 12700) / float64(availHeightEMU) * 100.0
}

// estimateRowTextHeightEMU returns the maximum text height across all cells
// in a row, in EMU. Used for overflow detection.
func estimateRowTextHeightEMU(row Row) int64 {
	var maxH int64
	for _, cell := range row.Cells {
		h := estimateCellTextHeightEMU(cell)
		if h > maxH {
			maxH = h
		}
	}
	return maxH
}

// estimateRowHeightPct estimates the percentage of grid height needed for a row
// based on the text content of its cells. It examines each cell's text to count
// lines and font size, then converts to a percentage of the available grid height.
func estimateRowHeightPct(row Row, availHeightEMU int64) float64 {
	if availHeightEMU <= 0 {
		return 10 // fallback minimum
	}

	var maxHeightEMU int64
	for _, cell := range row.Cells {
		h := estimateCellTextHeightEMU(cell)
		if h > maxHeightEMU {
			maxHeightEMU = h
		}
	}

	if maxHeightEMU == 0 {
		maxHeightEMU = int64(20 * 12700) // 20pt fallback
	}

	pct := float64(maxHeightEMU) / float64(availHeightEMU) * 100.0

	// Clamp to reasonable range
	if pct < 8 {
		pct = 8
	}
	if pct > 80 {
		pct = 80
	}
	return pct
}

// estimateCellTextHeightEMU returns an estimated height in EMU for the text
// content of a cell. It parses the shape's text JSON to count lines and
// determine font size.
func estimateCellTextHeightEMU(cell Cell) int64 {
	if cell.Shape == nil || len(cell.Shape.Text) == 0 {
		return 0
	}

	// Try string shorthand
	var s string
	if err := json.Unmarshal(cell.Shape.Text, &s); err == nil {
		return textHeightEMU(strings.Count(s, "\n")+1, 11, 0, 0)
	}

	// Object form
	var obj struct {
		Content     string  `json:"content"`
		Size        float64 `json:"size"`
		InsetTop    float64 `json:"inset_top"`
		InsetBottom float64 `json:"inset_bottom"`
	}
	if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
		return 0
	}

	fontSize := obj.Size
	if fontSize == 0 {
		fontSize = 11
	}
	lines := strings.Count(obj.Content, "\n") + 1
	return textHeightEMU(lines, fontSize, obj.InsetTop, obj.InsetBottom)
}

// textHeightEMU computes estimated text height in EMU from line count and font metrics.
func textHeightEMU(lines int, fontSizePt, insetTopPt, insetBottomPt float64) int64 {
	lineHeightPt := fontSizePt * 1.4 // standard line spacing factor
	textPt := float64(lines) * lineHeightPt
	totalPt := textPt + insetTopPt + insetBottomPt + 12 // 12pt padding for shape border/margin
	return int64(totalPt * 12700)                        // points to EMU
}

// distributeEMU converts percentage slices into absolute EMU values that sum
// exactly to totalEMU. It uses largest-remainder rounding to distribute
// truncation error evenly across entries, preventing cumulative drift that
// causes misaligned rows/columns.
func distributeEMU(pcts []float64, totalEMU int64) []int64 {
	n := len(pcts)
	if n == 0 {
		return nil
	}

	// Compute the sum of percentages to normalise against.
	var pctSum float64
	for _, p := range pcts {
		pctSum += p
	}
	if pctSum == 0 {
		// All zero — equal split.
		each := totalEMU / int64(n)
		result := make([]int64, n)
		for i := range result {
			result[i] = each
		}
		// Distribute remainder to first entries.
		rem := totalEMU - each*int64(n)
		for i := int64(0); i < rem; i++ {
			result[i]++
		}
		return result
	}

	result := make([]int64, n)
	fracs := make([]float64, n)
	var allocated int64
	for i, p := range pcts {
		exact := float64(totalEMU) * p / pctSum
		result[i] = int64(exact)
		fracs[i] = exact - float64(result[i])
		allocated += result[i]
	}

	// Distribute the remaining EMUs to entries with the largest fractional parts.
	remainder := totalEMU - allocated
	for remainder > 0 {
		bestIdx := 0
		bestFrac := fracs[0]
		for i := 1; i < n; i++ {
			if fracs[i] > bestFrac {
				bestIdx = i
				bestFrac = fracs[i]
			}
		}
		result[bestIdx]++
		fracs[bestIdx] = 0
		remainder--
	}

	return result
}

// equalSplit returns n equal percentages summing to 100.
func equalSplit(n int) []float64 {
	each := 100.0 / float64(n)
	result := make([]float64, n)
	for i := range result {
		result[i] = each
	}
	return result
}

// PctToEMU converts a percentage to EMU given the reference dimension.
func PctToEMU(pct float64, refEMU int64) int64 {
	return int64(pct / 100.0 * float64(refEMU))
}

// PtToEMU converts typographic points to EMU (1pt = 12700 EMU).
func PtToEMU(pt float64) int64 {
	return int64(pt * 12700)
}

// accentBarBounds computes the position and size of a decorative accent bar
// relative to the cell bounds. The bar is placed just outside the cell edge.
func accentBarBounds(cellBounds pptx.RectEmu, spec *AccentBarSpec) pptx.RectEmu {
	width := spec.Width
	if width <= 0 {
		width = 4.0 // default 4pt
	}
	widthEMU := int64(width * 12700) // points to EMU

	// Small gap between bar and cell (2pt)
	const gapEMU = 2 * 12700

	pos := spec.Position
	if pos == "" {
		pos = "left"
	}

	switch pos {
	case "right":
		return pptx.RectEmu{
			X:  cellBounds.X + cellBounds.CX + gapEMU,
			Y:  cellBounds.Y,
			CX: widthEMU,
			CY: cellBounds.CY,
		}
	case "top":
		return pptx.RectEmu{
			X:  cellBounds.X,
			Y:  cellBounds.Y - widthEMU - gapEMU,
			CX: cellBounds.CX,
			CY: widthEMU,
		}
	case "bottom":
		return pptx.RectEmu{
			X:  cellBounds.X,
			Y:  cellBounds.Y + cellBounds.CY + gapEMU,
			CX: cellBounds.CX,
			CY: widthEMU,
		}
	default: // "left"
		return pptx.RectEmu{
			X:  cellBounds.X - widthEMU - gapEMU,
			Y:  cellBounds.Y,
			CX: widthEMU,
			CY: cellBounds.CY,
		}
	}
}

// ApplyFitMode adjusts shape bounds within cell bounds according to the fit mode.
// For FitContain, the shape is scaled to the smaller dimension and centered.
// For FitWidth, height equals width and the shape is centered vertically.
// For FitHeight, width equals height and the shape is centered horizontally.
func ApplyFitMode(mode FitMode, cellBounds pptx.RectEmu) pptx.RectEmu {
	w := cellBounds.CX
	h := cellBounds.CY

	switch mode {
	case FitContain:
		// Use the smaller dimension for a 1:1 aspect ratio
		size := w
		if h < w {
			size = h
		}
		return pptx.RectEmu{
			X:  cellBounds.X + (w-size)/2,
			Y:  cellBounds.Y + (h-size)/2,
			CX: size,
			CY: size,
		}
	case FitWidth:
		// Width stays, height = width, centered vertically
		return pptx.RectEmu{
			X:  cellBounds.X,
			Y:  cellBounds.Y + (h-w)/2,
			CX: w,
			CY: w,
		}
	case FitHeight:
		// Height stays, width = height, centered horizontally
		return pptx.RectEmu{
			X:  cellBounds.X + (w-h)/2,
			Y:  cellBounds.Y,
			CX: h,
			CY: h,
		}
	default:
		return cellBounds
	}
}

// iconOverlayLayout holds the resolved icon position and extra text insets
// needed to prevent text from overlapping the icon.
type iconOverlayLayout struct {
	Bounds     pptx.RectEmu // Icon position and size
	TextInsets [4]int64     // Extra text insets [L,T,R,B] in EMU
}

// iconOverlayGapEMU is the gap between icon and text (3pt).
const iconOverlayGapEMU = 3 * 12700

// hasNonEmptyText checks whether a json.RawMessage text field contains actual
// non-empty text content (not just an empty string or object with empty content).
func hasNonEmptyText(text json.RawMessage) bool {
	if len(text) == 0 || string(text) == "null" {
		return false
	}
	// If it's a plain string, check if non-empty.
	var s string
	if json.Unmarshal(text, &s) == nil {
		return strings.TrimSpace(s) != ""
	}
	// If it's an object, check the "content" field.
	var obj struct {
		Content string `json:"content"`
	}
	if json.Unmarshal(text, &obj) == nil {
		return strings.TrimSpace(obj.Content) != ""
	}
	return true
}

// resolveIconPosition determines the effective icon position. If the icon spec
// has an explicit position, it is used. Otherwise, auto-detect based on shape
// aspect ratio and whether the cell contains text. Standalone icon cells (no
// text) default to "center"; wide shapes use "left"; otherwise "top".
func resolveIconPosition(icon *IconSpec, shapeBounds pptx.RectEmu, hasText bool) string {
	if icon != nil && icon.Position != "" {
		return icon.Position
	}
	// Standalone icon on a shape with no text — center it.
	if !hasText {
		return "center"
	}
	// Auto-detect based on aspect ratio (1.2:1 threshold for landscape shapes).
	if shapeBounds.CX > int64(float64(shapeBounds.CY)*1.2) {
		return "left"
	}
	return "top"
}

// iconOverlayBounds computes the icon bounds and text insets for an icon
// overlaid on a shape cell. The position controls the layout:
//   - "left":   icon on the left, text shifted right
//   - "top":    icon centered at the top, text shifted down
//   - "center": icon centered over text (legacy behavior, no text adjustment)
func iconOverlayBounds(icon *IconSpec, shapeBounds pptx.RectEmu, hasText bool) iconOverlayLayout {
	scale := 0.6
	if icon != nil && icon.Scale > 0 && icon.Scale <= 1.0 {
		scale = icon.Scale
	}

	w := shapeBounds.CX
	h := shapeBounds.CY
	minDim := w
	if h < w {
		minDim = h
	}
	size := int64(float64(minDim) * scale)

	pos := resolveIconPosition(icon, shapeBounds, hasText)

	switch pos {
	case "left":
		// Icon on the left side, sized to 60% of cell height, vertically centered.
		iconH := int64(float64(h) * scale)
		if iconH > size {
			iconH = size // keep square
		}
		return iconOverlayLayout{
			Bounds: pptx.RectEmu{
				X:  shapeBounds.X + iconOverlayGapEMU,
				Y:  shapeBounds.Y + (h-iconH)/2,
				CX: iconH,
				CY: iconH,
			},
			TextInsets: [4]int64{iconH + 2*iconOverlayGapEMU, 0, 0, 0}, // extra left inset
		}
	case "top":
		// Icon centered horizontally and vertically within the top icon zone.
		iconZoneH := size + 2*iconOverlayGapEMU
		return iconOverlayLayout{
			Bounds: pptx.RectEmu{
				X:  shapeBounds.X + (w-size)/2,
				Y:  shapeBounds.Y + (iconZoneH-size)/2,
				CX: size,
				CY: size,
			},
			TextInsets: [4]int64{0, iconZoneH, 0, 0}, // extra top inset
		}
	default: // "center" — legacy behavior, no text adjustment
		return iconOverlayLayout{
			Bounds: pptx.RectEmu{
				X:  shapeBounds.X + (w-size)/2,
				Y:  shapeBounds.Y + (h-size)/2,
				CX: size,
				CY: size,
			},
		}
	}
}
