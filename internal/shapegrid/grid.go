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

	// Auto-height: when no row specifies an explicit height, estimate content
	// heights and shrink grid bounds to fit content instead of stretching to
	// fill the full content zone.
	if allRowHeightsZero(grid.Rows) && numRows > 0 {
		var maxContentEMU int64
		for _, row := range grid.Rows {
			h := estimateRowContentHeightEMU(row)
			if h > maxContentEMU {
				maxContentEMU = h
			}
		}
		if maxContentEMU > 0 {
			totalContentH := maxContentEMU*int64(numRows) + totalRowGap
			if totalContentH < gridH {
				gridH = totalContentH
				availH = gridH - totalRowGap
				grid.Bounds.CY = gridH
			}
			// Set uniform percentage so resolveRowHeights treats them as fixed.
			pctEach := 100.0 / float64(numRows)
			for i := range grid.Rows {
				grid.Rows[i].Height = pctEach
			}
		}
	}

	// Resolve row heights
	rowHeights := resolveRowHeights(grid.Rows, availH)

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
		Cells:      cells,
		Connectors: connectors,
		AccentBars: accentBars,
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

// resolveRowHeights returns percentage heights for each row, summing to 100.
// availHeightEMU is the available grid height in EMU (after gaps), used for
// auto-height estimation.
func resolveRowHeights(rows []Row, availHeightEMU int64) []float64 {
	heights := make([]float64, len(rows))
	var fixedPct float64
	autoIdxs := []int{}
	unspecified := 0

	// Pass 1: assign fixed heights and estimate auto-height rows.
	for i, row := range rows {
		if row.Height > 0 {
			heights[i] = row.Height
			fixedPct += row.Height
		} else if row.AutoHeight {
			est := estimateRowHeightPct(row, availHeightEMU)
			heights[i] = est
			autoIdxs = append(autoIdxs, i)
		} else {
			unspecified++
		}
	}

	// Pass 2: ensure auto-height rows get at least an equal share of the
	// remaining space (after fixed rows). This prevents tiny panels when
	// text content is short but plenty of vertical space is available.
	flexCount := len(autoIdxs) + unspecified
	if flexCount > 0 {
		remainingAfterFixed := 100.0 - fixedPct
		if remainingAfterFixed < 0 {
			remainingAfterFixed = 0
		}
		equalShare := remainingAfterFixed / float64(flexCount)
		for _, idx := range autoIdxs {
			if heights[idx] < equalShare {
				heights[idx] = equalShare
			}
		}
	}

	// Pass 3: distribute leftover space to unspecified rows.
	if unspecified > 0 {
		var usedPct float64
		for _, h := range heights {
			usedPct += h
		}
		remaining := 100.0 - usedPct
		if remaining < 0 {
			remaining = 0
		}
		each := remaining / float64(unspecified)
		for i, row := range rows {
			if heights[i] == 0 && row.Height == 0 && !row.AutoHeight {
				heights[i] = each
			}
		}
	}

	return heights
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

// allRowHeightsZero returns true when no row in the slice has an explicit height.
func allRowHeightsZero(rows []Row) bool {
	for _, row := range rows {
		if row.Height > 0 {
			return false
		}
	}
	return true
}

// estimateRowContentHeightEMU returns the estimated height in EMU needed for a
// row based on the tallest cell's content. It considers text, icons, and images.
func estimateRowContentHeightEMU(row Row) int64 {
	var maxH int64
	for _, cell := range row.Cells {
		h := estimateCellHeightEMU(cell)
		if h > maxH {
			maxH = h
		}
	}
	if maxH == 0 {
		maxH = int64(24 * 12700) // 24pt minimum fallback
	}
	return maxH
}

// estimateCellHeightEMU estimates the total height needed for a cell in EMU,
// including text, icons, and images.
func estimateCellHeightEMU(cell Cell) int64 {
	var h int64

	// Text content height
	textH := estimateCellTextHeightEMU(cell)
	if textH > h {
		h = textH
	}

	// Icon-only cell or icon overlay adds height
	if cell.Icon != nil && cell.Shape == nil {
		// Standalone icon: ~40pt default
		iconH := int64(40 * 12700)
		if iconH > h {
			h = iconH
		}
	} else if cell.Icon != nil && cell.Shape != nil {
		// Icon overlay on shape: icon height + text height
		iconH := int64(28 * 12700) // ~28pt for inline icon
		if textH > 0 {
			h = textH + iconH
		} else if iconH > h {
			h = iconH
		}
	}

	// Image cells: use a reasonable default
	if cell.Image != nil {
		imgH := int64(60 * 12700) // 60pt minimum for images
		if imgH > h {
			h = imgH
		}
	}

	return h
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
