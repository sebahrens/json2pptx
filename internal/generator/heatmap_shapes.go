package generator

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// =============================================================================
// Heatmap Native Shapes — NxM Grid of Rect Cells with Computed Fills
// =============================================================================
//
// Replaces SVG-rendered heatmap diagrams with native OOXML grouped shapes.
// Each cell is a rect with a scheme-colored tint fill (varying lumMod/lumOff
// based on data value) and centered value text. Row/column header text boxes
// along edges. All shapes wrapped in a single p:grpSp.
//
// Layout:
//
//                 Col1      Col2      Col3      Col4
//          ┌─────────┐ gap ┌─────────┐ gap ┌─────────┐ gap ┌─────────┐
//   Row1   │   9     │     │   7     │     │   6     │     │   8     │
//          └─────────┘     └─────────┘     └─────────┘     └─────────┘
//              gap             gap             gap             gap
//          ┌─────────┐ gap ┌─────────┐ gap ┌─────────┐ gap ┌─────────┐
//   Row2   │   5     │     │   9     │     │   8     │     │   7     │
//          └─────────┘     └─────────┘     └─────────┘     └─────────┘
//
// Color strategy: cells use scheme accent1 with lumMod/lumOff interpolated
// based on normalized value (min→max). Low values get light tints (high lumOff),
// high values get saturated color (low lumOff). This is theme-aware — changing
// the template accent1 changes all heatmap cell colors accordingly.

// Heatmap EMU constants.
const (
	// heatmapGap is the gap between cells in EMU.
	// ~0.04" = 36576 EMU — tight gap for dense grids.
	heatmapGap int64 = 36576

	// heatmapCellFontSize is the cell value font size (hundredths of a point).
	// 900 = 9pt — small to fit in dense grid cells.
	heatmapCellFontSize int = 900

	// heatmapCellFontSizeLarge is for sparse grids (≤4 cells per dimension).
	// 1100 = 11pt
	heatmapCellFontSizeLarge int = 1100

	// heatmapHeaderFontSize is the row/column header font size (hundredths of a point).
	// 1000 = 10pt
	heatmapHeaderFontSize int = 1000

	// heatmapCellInset is the text inset for cell shapes (EMU).
	heatmapCellInset int64 = 36000 // ~0.04"

	// heatmapRowLabelWidth is the space reserved for row labels (EMU).
	// ~1.2" = 1097280 EMU
	heatmapRowLabelWidth int64 = 1097280

	// heatmapColLabelHeight is the space reserved for column labels (EMU).
	// ~0.35" = 320040 EMU
	heatmapColLabelHeight int64 = 320040

	// heatmapMinCellDim is the minimum cell dimension before hiding value text (EMU).
	// ~0.3" = 274320 EMU
	heatmapMinCellDim int64 = 274320
)

// heatmapMeta holds metadata for the heatmap layout.
type heatmapMeta struct {
	numRows    int
	numCols    int
	colorScale string // "sequential" or "diverging"
}

// isHeatmapDiagram returns true if the diagram spec is a heatmap type.
func isHeatmapDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "heatmap"
}

// heatmapCellData holds parsed data for a single cell in the NxM grid.
type heatmapCellData struct {
	value float64
}

// heatmapParsedData holds all parsed heatmap data.
type heatmapParsedData struct {
	rowLabels  []string
	colLabels  []string
	values     [][]float64 // [row][col]
	minVal     float64
	maxVal     float64
	colorScale string
}

// processHeatmapNativeShapes parses heatmap data from a DiagramSpec and
// registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processHeatmapNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("heatmap native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("heatmap native shapes: themeOverride is set but scheme color refs will not reflect overrides",
			"slide", slideNum)
	}

	parsed, err := parseHeatmapData(diagramSpec.Data)
	if err != nil {
		slog.Warn("heatmap native shapes: parse failed", "slide", slideNum, "error", err)
		return
	}

	if len(parsed.values) == 0 || len(parsed.values[0]) == 0 {
		slog.Warn("heatmap native shapes: empty values grid", "slide", slideNum)
		return
	}

	numRows := len(parsed.values)
	numCols := len(parsed.values[0])

	// Encode heatmap data into panels for the panelShapeInsert system.
	// Panel 0: metadata (row count, col count, min, max, colorScale, row labels, col labels)
	// Panels 1..N: one per cell [row*numCols+col], value encoded as title
	var panels []nativePanelData

	// Panel 0: metadata
	panels = append(panels, nativePanelData{
		title: "__heatmap_meta__",
		body:  encodeHeatmapMeta(parsed),
	})

	// Panels 1..N: cells
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			v := 0.0
			if col < len(parsed.values[row]) {
				v = parsed.values[row][col]
			}
			panels = append(panels, nativePanelData{
				title: formatHeatmapVal(v),
			})
		}
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native heatmap shapes: registered",
		"slide", slideNum,
		"rows", numRows,
		"cols", numCols,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		heatmapMode:    true,
		heatmapMeta: heatmapMeta{
			numRows:    numRows,
			numCols:    numCols,
			colorScale: parsed.colorScale,
		},
	})
}

// encodeHeatmapMeta encodes heatmap metadata and labels into a single string.
// Format: "minVal\nmaxVal\ncolorScale\nrowLabel0\n...\nrowLabelN\n---\ncolLabel0\n...\ncolLabelM"
func encodeHeatmapMeta(parsed heatmapParsedData) string {
	result := fmt.Sprintf("%g\n%g\n%s", parsed.minVal, parsed.maxVal, parsed.colorScale)
	for _, rl := range parsed.rowLabels {
		result += "\n" + rl
	}
	result += "\n---"
	for _, cl := range parsed.colLabels {
		result += "\n" + cl
	}
	return result
}

// decodeHeatmapMeta decodes heatmap metadata from the encoded string.
func decodeHeatmapMeta(encoded string) (minVal, maxVal float64, colorScale string, rowLabels, colLabels []string) {
	var lines []string
	start := 0
	for i := 0; i <= len(encoded); i++ {
		if i == len(encoded) || encoded[i] == '\n' {
			lines = append(lines, encoded[start:i])
			start = i + 1
		}
	}

	if len(lines) < 3 {
		return 0, 0, "sequential", nil, nil
	}

	fmt.Sscanf(lines[0], "%g", &minVal)
	fmt.Sscanf(lines[1], "%g", &maxVal)
	colorScale = lines[2]

	// Split remaining lines at "---" separator.
	inCols := false
	for _, line := range lines[3:] {
		if line == "---" {
			inCols = true
			continue
		}
		if inCols {
			colLabels = append(colLabels, line)
		} else {
			rowLabels = append(rowLabels, line)
		}
	}
	return
}

// generateHeatmapGroupXML produces the complete <p:grpSp> XML for a heatmap grid.
func generateHeatmapGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta heatmapMeta) string {
	if len(panels) < 2 {
		slog.Warn("generateHeatmapGroupXML: insufficient panels", "got", len(panels))
		return ""
	}

	numRows := meta.numRows
	numCols := meta.numCols
	expectedPanels := 1 + numRows*numCols
	if len(panels) != expectedPanels {
		slog.Warn("generateHeatmapGroupXML: panel count mismatch",
			"expected", expectedPanels, "got", len(panels))
		return ""
	}

	// Decode metadata.
	minVal, maxVal, colorScale, rowLabels, colLabels := decodeHeatmapMeta(panels[0].body)

	// Determine if we have row/col labels.
	hasRowLabels := len(rowLabels) > 0
	hasColLabels := len(colLabels) > 0

	// Reserve space for labels.
	rowLabelW := int64(0)
	if hasRowLabels {
		rowLabelW = heatmapRowLabelWidth
	}
	colLabelH := int64(0)
	if hasColLabels {
		colLabelH = heatmapColLabelHeight
	}

	// Grid bounds.
	gridX := bounds.X + rowLabelW
	gridY := bounds.Y + colLabelH
	gridW := bounds.Width - rowLabelW
	gridH := bounds.Height - colLabelH

	// Cell dimensions.
	hGapTotal := int64(numCols-1) * heatmapGap
	vGapTotal := int64(numRows-1) * heatmapGap
	cellW := (gridW - hGapTotal) / int64(numCols)
	cellH := (gridH - vGapTotal) / int64(numRows)

	// Determine if cells are large enough for value text.
	showValues := cellW >= heatmapMinCellDim && cellH >= heatmapMinCellDim

	// Choose font size based on grid density.
	cellFontSize := heatmapCellFontSize
	if numRows <= 4 && numCols <= 4 {
		cellFontSize = heatmapCellFontSizeLarge
	}

	var children [][]byte
	shapeIdx := uint32(0)

	// Generate column label text boxes (top of grid).
	if hasColLabels {
		for col := 0; col < numCols; col++ {
			label := ""
			if col < len(colLabels) {
				label = colLabels[col]
			}
			if label == "" {
				continue
			}
			shapeIdx++
			labelX := gridX + int64(col)*(cellW+heatmapGap)
			lbl := generateHeatmapLabelXML(
				label, labelX, bounds.Y, cellW, colLabelH,
				shapeIDBase+shapeIdx, heatmapHeaderFontSize, "ctr", true,
			)
			children = append(children, []byte(lbl))
		}
	}

	// Generate row label text boxes (left of grid).
	if hasRowLabels {
		for row := 0; row < numRows; row++ {
			label := ""
			if row < len(rowLabels) {
				label = rowLabels[row]
			}
			if label == "" {
				continue
			}
			shapeIdx++
			labelY := gridY + int64(row)*(cellH+heatmapGap)
			lbl := generateHeatmapLabelXML(
				label, bounds.X, labelY, rowLabelW-heatmapGap, cellH,
				shapeIDBase+shapeIdx, heatmapHeaderFontSize, "r", true,
			)
			children = append(children, []byte(lbl))
		}
	}

	// Generate NxM grid cells.
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			panelIdx := 1 + row*numCols + col
			panel := panels[panelIdx]

			cellX := gridX + int64(col)*(cellW+heatmapGap)
			cellY := gridY + int64(row)*(cellH+heatmapGap)

			// Parse the value from panel title.
			var val float64
			fmt.Sscanf(panel.title, "%g", &val)

			// Compute fill based on value.
			fill := heatmapCellFill(val, minVal, maxVal, colorScale)

			shapeIdx++
			cellXML := generateHeatmapCellXML(
				panel.title, cellX, cellY, cellW, cellH,
				shapeIDBase+shapeIdx, fill, showValues, cellFontSize,
			)
			children = append(children, []byte(cellXML))
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Heatmap",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateHeatmapGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// heatmapCellFill computes a scheme-based fill for a heatmap cell.
// Sequential: accent1 with lumMod interpolated from 20000 (lightest) to 100000 (full saturation).
// Diverging: accent2 for low values, lt1 for midpoint, accent1 for high values.
func heatmapCellFill(value, minVal, maxVal float64, colorScale string) pptx.Fill {
	if maxVal == minVal {
		// All values the same — use 50% tint.
		return pptx.SchemeFill("accent1", pptx.LumMod(50000), pptx.LumOff(50000))
	}

	t := (value - minVal) / (maxVal - minVal)
	t = math.Max(0, math.Min(1, t))

	if colorScale == "diverging" {
		// Diverging: accent2 (low) → lt1 (mid) → accent1 (high)
		if t < 0.5 {
			// Low half: accent2 with decreasing saturation toward midpoint.
			// t=0 → full accent2, t=0.5 → very light accent2
			satT := 1.0 - t*2 // 1.0 → 0.0
			lumMod := 20000 + int(satT*80000)
			lumOff := 100000 - lumMod
			return pptx.SchemeFill("accent2", pptx.LumMod(lumMod), pptx.LumOff(lumOff))
		}
		// High half: accent1 with increasing saturation from midpoint.
		// t=0.5 → very light accent1, t=1.0 → full accent1
		satT := (t - 0.5) * 2 // 0.0 → 1.0
		lumMod := 20000 + int(satT*80000)
		lumOff := 100000 - lumMod
		return pptx.SchemeFill("accent1", pptx.LumMod(lumMod), pptx.LumOff(lumOff))
	}

	// Sequential: accent1 from light tint (t=0) to full saturation (t=1).
	// lumMod ranges from 20000 (very light) to 100000 (full color).
	lumMod := 20000 + int(t*80000)
	lumOff := 100000 - lumMod
	return pptx.SchemeFill("accent1", pptx.LumMod(lumMod), pptx.LumOff(lumOff))
}

// heatmapTextColor returns the text fill for a cell value based on luminance.
// Light cells (low lumMod) get dark text, dark cells (high lumMod) get light text.
func heatmapTextColor(value, minVal, maxVal float64) pptx.Fill {
	if maxVal == minVal {
		return pptx.SchemeFill("dk1")
	}
	t := (value - minVal) / (maxVal - minVal)
	t = math.Max(0, math.Min(1, t))

	// Crossover at t=0.6 — cells darker than 60% get light text.
	if t > 0.6 {
		return pptx.SchemeFill("lt1")
	}
	return pptx.SchemeFill("dk1")
}

// generateHeatmapCellXML produces a single rect cell shape for the heatmap grid.
func generateHeatmapCellXML(valueText string, x, y, cx, cy int64, shapeID uint32, fill pptx.Fill, showValue bool, fontSize int) string {
	var text *pptx.TextBody
	if showValue && valueText != "" {
		var val float64
		fmt.Sscanf(valueText, "%g", &val)
		// Re-format to clean display.
		displayText := valueText

		text = &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{heatmapCellInset, heatmapCellInset, heatmapCellInset, heatmapCellInset},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     displayText,
					Lang:     "en-US",
					FontSize: fontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Heatmap Cell",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRect,
		Fill:     fill,
		Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
		Text:     text,
	})
	if err != nil {
		slog.Warn("generateHeatmapCellXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateHeatmapLabelXML produces a text box shape for a row or column label.
func generateHeatmapLabelXML(text string, x, y, cx, cy int64, shapeID uint32, fontSize int, align string, bold bool) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Heatmap Label",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{0, 0, 0, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    align,
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     text,
					Lang:     "en-US",
					FontSize: fontSize,
					Bold:     bold,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateHeatmapLabelXML failed", "error", err)
		return ""
	}
	return string(b)
}

// formatHeatmapVal formats a heatmap cell value for display.
func formatHeatmapVal(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}

// parseHeatmapData extracts heatmap data from a diagram data map.
// Supports the same keys as the SVG renderer: row_labels/y_labels,
// col_labels/x_labels/column_labels, values/rows.
func parseHeatmapData(data map[string]any) (heatmapParsedData, error) {
	parsed := heatmapParsedData{
		colorScale: "sequential",
	}

	// Parse row labels.
	if rl, ok := data["row_labels"]; ok {
		if labels, ok := toStringSliceHeatmap(rl); ok {
			parsed.rowLabels = labels
		}
	} else if rl, ok := data["y_labels"]; ok {
		if labels, ok := toStringSliceHeatmap(rl); ok {
			parsed.rowLabels = labels
		}
	}

	// Parse column labels.
	if cl, ok := data["col_labels"]; ok {
		if labels, ok := toStringSliceHeatmap(cl); ok {
			parsed.colLabels = labels
		}
	} else if cl, ok := data["x_labels"]; ok {
		if labels, ok := toStringSliceHeatmap(cl); ok {
			parsed.colLabels = labels
		}
	} else if cl, ok := data["column_labels"]; ok {
		if labels, ok := toStringSliceHeatmap(cl); ok {
			parsed.colLabels = labels
		}
	}

	// Parse values.
	valuesRaw, ok := data["values"]
	if !ok {
		valuesRaw, ok = data["rows"]
	}
	if !ok {
		return parsed, fmt.Errorf("heatmap requires 'values' or 'rows' field")
	}

	grid, ok := toNestedFloat64SliceHeatmap(valuesRaw)
	if !ok {
		return parsed, fmt.Errorf("heatmap 'values' must be a 2D array of numbers")
	}
	parsed.values = grid

	// Auto-range.
	if len(grid) > 0 && len(grid[0]) > 0 {
		parsed.minVal = grid[0][0]
		parsed.maxVal = grid[0][0]
		for _, row := range grid {
			for _, v := range row {
				if v < parsed.minVal {
					parsed.minVal = v
				}
				if v > parsed.maxVal {
					parsed.maxVal = v
				}
			}
		}
	}

	// Parse color scale.
	if cs, ok := data["color_scale"].(string); ok {
		parsed.colorScale = cs
	}

	return parsed, nil
}

// toStringSliceHeatmap converts an interface{} to []string.
func toStringSliceHeatmap(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result[i] = s
	}
	return result, true
}

// toNestedFloat64SliceHeatmap converts an interface{} to [][]float64.
func toNestedFloat64SliceHeatmap(v any) ([][]float64, bool) {
	outer, ok := v.([]any)
	if !ok {
		return nil, false
	}
	result := make([][]float64, len(outer))
	for i, row := range outer {
		rowSlice, ok := row.([]any)
		if !ok {
			return nil, false
		}
		result[i] = make([]float64, len(rowSlice))
		for j, val := range rowSlice {
			f, ok := val.(float64)
			if !ok {
				return nil, false
			}
			result[i][j] = f
		}
	}
	return result, true
}
