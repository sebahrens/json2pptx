package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// =============================================================================
// Nine Box Talent Native Shapes — 3x3 Grid of roundRect Cells
// =============================================================================
//
// Replaces SVG-rendered nine_box_talent diagrams with native OOXML grouped shapes.
// Each cell is a roundRect with a scheme-colored tint fill, a bold label at the
// top, and item names listed below. All 9 cells plus axis label text boxes are
// wrapped in a single p:grpSp with identity child transform.
//
// Layout (3 columns x 3 rows):
//
//                  Low          Medium         High
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   High    │  Enigma   │       │ Growth   │       │  Star    │
//           │ (accent4) │       │ (accent3) │       │ (accent1) │
//           └──────────┘       └──────────┘       └──────────┘
//                 gap                gap                gap
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   Medium  │ Dilemma   │       │  Core    │       │  High    │
//           │ (accent4) │       │ (accent3) │       │ Performer│
//           └──────────┘       └──────────┘       │ (accent1) │
//                 gap                gap           └──────────┘
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   Low     │  Under   │       │ Average  │       │  Solid   │
//           │Performer │       │Performer │       │Performer │
//           │ (accent2) │       │ (accent4) │       │ (accent3) │
//           └──────────┘       └──────────┘       └──────────┘
//
// Color strategy: cells use scheme accent colors with tints based on their
// position in the performance/potential grid (top-right = green/accent1,
// bottom-left = red/accent2, diagonal = yellow/accent3, off-diagonal = amber/accent4).

// Nine Box EMU constants.
const (
	// nineBoxGap is the gap between cells in EMU.
	// Same as SWOT/PESTEL gap for visual consistency.
	nineBoxGap int64 = 73152

	// nineBoxCornerRadius is the roundRect adjustment value.
	nineBoxCornerRadius int64 = 8000

	// nineBoxLabelFontSize is the cell label font size (hundredths of a point).
	// 1100 = 11pt
	nineBoxLabelFontSize int = 1100

	// nineBoxItemFontSize is the item name font size (hundredths of a point).
	// 1000 = 10pt (small to fit multiple names in cells)
	nineBoxItemFontSize int = 1000

	// nineBoxAxisFontSize is the axis label font size (hundredths of a point).
	// 1000 = 10pt
	nineBoxAxisFontSize int = 1000

	// nineBoxAxisTitleFontSize is the axis title font size (hundredths of a point).
	// 1100 = 11pt
	nineBoxAxisTitleFontSize int = 1100

	// nineBoxLabelHeightRatio is the fraction of cell height used for the label area.
	nineBoxLabelHeightRatio = 0.22

	// nineBoxCellInset is the text inset for cell text (EMU).
	nineBoxCellInset int64 = 72000 // ~0.079"

	// nineBoxAxisSpace is the space reserved for axis labels (EMU).
	// Y-axis labels on the left, X-axis labels on the bottom.
	nineBoxAxisLabelSpace int64 = 457200 // ~0.5"

	// nineBoxAxisTitleSpace is additional space for axis titles (EMU).
	nineBoxAxisTitleSpace int64 = 228600 // ~0.25"
)

// nineBoxCellColors maps each cell position [row][col] to a scheme color.
// Uses a traffic-light-inspired pattern:
//   - Top-right (high potential, high performance) = green tones (accent1)
//   - Bottom-left (low potential, low performance) = red tones (accent2)
//   - Diagonal = yellow/neutral tones (accent3)
//   - Off-diagonal = amber tones (accent4)
var nineBoxCellColors = [3][3]struct {
	scheme string
	lumMod int
	lumOff int
}{
	// Row 0 (High Potential): amber, green, green
	{{"accent4", 20000, 80000}, {"accent1", 25000, 75000}, {"accent1", 20000, 80000}},
	// Row 1 (Medium Potential): amber, yellow, green
	{{"accent4", 25000, 75000}, {"accent3", 20000, 80000}, {"accent1", 25000, 75000}},
	// Row 2 (Low Potential): red, amber, yellow
	{{"accent2", 20000, 80000}, {"accent4", 20000, 80000}, {"accent3", 25000, 75000}},
}

// nineBoxDefaultLabels returns the standard 9-box cell labels indexed [row][col].
// Row 0 = high potential, Col 0 = low performance.
var nineBoxDefaultLabels = [3][3]string{
	{"Enigma", "Growth Employee", "Star"},
	{"Dilemma", "Core Employee", "High Performer"},
	{"Under Performer", "Average Performer", "Solid Performer"},
}

// isNineBoxDiagram returns true if the diagram spec is a nine_box_talent diagram type.
func isNineBoxDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "nine_box_talent"
}

// nineBoxCellData holds parsed data for a single cell in the 3x3 grid.
type nineBoxCellData struct {
	row   int
	col   int
	label string
	items []string // item/person names
}

// processNineBoxNativeShapes parses nine_box_talent data from a DiagramSpec and
// registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processNineBoxNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("nine_box native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	// Warn if themeOverride is set — scheme colors won't reflect overrides.
	if ctx.themeOverride != nil {
		slog.Warn("nine_box native shapes: themeOverride is set but scheme color refs in nine box shapes will not reflect overrides",
			"slide", slideNum)
	}

	// Parse cells and axis info from DiagramSpec.Data.
	cells := parseNineBoxCells(diagramSpec.Data)
	xAxisLabel, _ := diagramSpec.Data["x_axis_label"].(string)
	yAxisLabel, _ := diagramSpec.Data["y_axis_label"].(string)
	xAxisLabels := parseNineBoxAxisLabels(diagramSpec.Data, "x_axis_labels")
	yAxisLabels := parseNineBoxAxisLabels(diagramSpec.Data, "y_axis_labels")

	// Convert cells into nativePanelData with axis info stored in first panel's body.
	// We encode axis metadata as a prefix in the panels slice — the generate function
	// will parse it back. This avoids changing the panelShapeInsert struct.
	var panels []nativePanelData

	// Panel 0: axis metadata encoded as title="__nine_box_axes__"
	axisBody := encodeNineBoxAxes(xAxisLabel, yAxisLabel, xAxisLabels, yAxisLabels)
	panels = append(panels, nativePanelData{
		title: "__nine_box_axes__",
		body:  axisBody,
	})

	// Panels 1-9: one per cell (all 9 cells, even empty ones)
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			label := nineBoxDefaultLabels[row][col]
			var itemNames []string

			// Find cell data if provided
			for _, c := range cells {
				if c.row == row && c.col == col {
					if c.label != "" {
						label = c.label
					}
					itemNames = c.items
					break
				}
			}

			body := ""
			if len(itemNames) > 0 {
				bulletLines := make([]string, len(itemNames))
				for i, name := range itemNames {
					bulletLines[i] = "- " + name
				}
				body = strings.Join(bulletLines, "\n")
			}

			panels = append(panels, nativePanelData{
				title: label,
				body:  body,
			})
		}
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native nine_box shapes: registered",
		"slide", slideNum,
		"cells", len(cells),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		nineBoxMode:    true,
	})
}

// encodeNineBoxAxes encodes axis labels into a single string for transport in nativePanelData.
// Format: "xTitle\nxL0\nxL1\nxL2\nyTitle\nyL0\nyL1\nyL2"
func encodeNineBoxAxes(xTitle, yTitle string, xLabels, yLabels [3]string) string {
	parts := []string{xTitle, xLabels[0], xLabels[1], xLabels[2], yTitle, yLabels[0], yLabels[1], yLabels[2]}
	return strings.Join(parts, "\n")
}

// decodeNineBoxAxes decodes axis labels from the encoded string.
func decodeNineBoxAxes(encoded string) (xTitle string, yTitle string, xLabels, yLabels [3]string) {
	parts := strings.Split(encoded, "\n")
	if len(parts) >= 8 {
		xTitle = parts[0]
		xLabels = [3]string{parts[1], parts[2], parts[3]}
		yTitle = parts[4]
		yLabels = [3]string{parts[5], parts[6], parts[7]}
	}
	return
}

// generateNineBoxGroupXML produces the complete <p:grpSp> XML for a 3x3 nine box grid
// with axis labels.
func generateNineBoxGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	// panels[0] = axis metadata, panels[1..9] = cells [row*3+col]
	if len(panels) != 10 {
		slog.Warn("generateNineBoxGroupXML: expected 10 panels (1 axis + 9 cells)", "got", len(panels))
		return ""
	}

	// Decode axis info from first panel.
	xAxisTitle, yAxisTitle, xAxisLabels, yAxisLabels := decodeNineBoxAxes(panels[0].body)

	// Determine space needed for axes.
	hasYAxis := yAxisTitle != "" || yAxisLabels[0] != ""
	hasXAxis := xAxisTitle != "" || xAxisLabels[0] != ""

	yAxisWidth := int64(0)
	if hasYAxis {
		yAxisWidth = nineBoxAxisLabelSpace
		if yAxisTitle != "" {
			yAxisWidth += nineBoxAxisTitleSpace
		}
	}

	xAxisHeight := int64(0)
	if hasXAxis {
		xAxisHeight = nineBoxAxisLabelSpace
		if xAxisTitle != "" {
			xAxisHeight += nineBoxAxisTitleSpace
		}
	}

	// Grid bounds (after reserving space for axes).
	gridX := bounds.X + yAxisWidth
	gridY := bounds.Y
	gridW := bounds.Width - yAxisWidth
	gridH := bounds.Height - xAxisHeight

	// 3x3 grid cell dimensions.
	cellW := (gridW - 2*nineBoxGap) / 3
	cellH := (gridH - 2*nineBoxGap) / 3

	// Label height within each cell.
	labelCY := int64(float64(cellH) * nineBoxLabelHeightRatio)
	bodyCY := cellH - labelCY

	var children [][]byte
	shapeIdx := uint32(0)

	// Generate 9 cells.
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			panelIdx := 1 + row*3 + col // skip axis panel at index 0
			panel := panels[panelIdx]
			colors := nineBoxCellColors[row][col]

			cellX := gridX + int64(col)*(cellW+nineBoxGap)
			cellY := gridY + int64(row)*(cellH+nineBoxGap)

			shapeIdx++
			labelID := shapeIDBase + shapeIdx
			shapeIdx++
			bodyID := shapeIDBase + shapeIdx

			// Label shape: roundRect with scheme fill, centered bold text
			labelXML := generateNineBoxCellLabelXML(
				panel.title, cellX, cellY, cellW, labelCY,
				labelID, colors.scheme, colors.lumMod, colors.lumOff,
			)
			children = append(children, []byte(labelXML))

			// Body shape: roundRect with same scheme fill, top-aligned bulleted text
			bodyXML := generateNineBoxCellBodyXML(
				panel.body, cellX, cellY+labelCY, cellW, bodyCY,
				bodyID, colors.scheme, colors.lumMod, colors.lumOff,
			)
			children = append(children, []byte(bodyXML))
		}
	}

	// Generate axis labels as text box shapes.
	if hasXAxis {
		// X-axis value labels (bottom of grid, centered under each column).
		xLabelY := gridY + gridH + nineBoxGap/2
		xLabelH := nineBoxAxisLabelSpace - nineBoxGap/2

		// Default x-axis labels.
		if xAxisLabels[0] == "" {
			xAxisLabels = [3]string{"Low", "Medium", "High"}
		}

		for col := 0; col < 3; col++ {
			if xAxisLabels[col] == "" {
				continue
			}
			shapeIdx++
			labelX := gridX + int64(col)*(cellW+nineBoxGap)
			xlbl := generateNineBoxAxisLabelXML(
				xAxisLabels[col], labelX, xLabelY, cellW, xLabelH,
				shapeIDBase+shapeIdx, nineBoxAxisFontSize, "ctr", false,
			)
			children = append(children, []byte(xlbl))
		}

		// X-axis title (centered below value labels).
		if xAxisTitle != "" {
			shapeIdx++
			titleY := xLabelY + xLabelH
			xlbl := generateNineBoxAxisLabelXML(
				xAxisTitle, gridX, titleY, gridW, nineBoxAxisTitleSpace,
				shapeIDBase+shapeIdx, nineBoxAxisTitleFontSize, "ctr", true,
			)
			children = append(children, []byte(xlbl))
		}
	}

	if hasYAxis {
		// Y-axis value labels (left of grid, centered beside each row).
		// Y-axis labels are in ascending order [low, medium, high] by convention,
		// but rows are top-to-bottom [high, medium, low], so reverse them.
		if yAxisLabels[0] == "" {
			yAxisLabels = [3]string{"High", "Medium", "Low"}
		} else {
			yAxisLabels = [3]string{yAxisLabels[2], yAxisLabels[1], yAxisLabels[0]}
		}

		yLabelX := bounds.X
		if yAxisTitle != "" {
			yLabelX += nineBoxAxisTitleSpace
		}
		yLabelW := nineBoxAxisLabelSpace
		if yAxisTitle != "" {
			yLabelW -= nineBoxAxisTitleSpace
		}

		for row := 0; row < 3; row++ {
			if yAxisLabels[row] == "" {
				continue
			}
			shapeIdx++
			labelY := gridY + int64(row)*(cellH+nineBoxGap)
			ylbl := generateNineBoxAxisLabelXML(
				yAxisLabels[row], yLabelX, labelY, yLabelW, cellH,
				shapeIDBase+shapeIdx, nineBoxAxisFontSize, "ctr", false,
			)
			children = append(children, []byte(ylbl))
		}

		// Y-axis title (rotated 90° CCW, centered to the left of value labels).
		if yAxisTitle != "" {
			shapeIdx++
			// For rotated text, we create a text box rotated -90° (270°).
			// Position: left edge of bounds, vertically centered.
			ylbl := generateNineBoxAxisTitleVerticalXML(
				yAxisTitle, bounds.X, gridY, nineBoxAxisTitleSpace, gridH,
				shapeIDBase+shapeIdx,
			)
			children = append(children, []byte(ylbl))
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Nine Box Talent",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateNineBoxGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxCellLabelXML produces a roundRect label shape for a nine box cell.
func generateNineBoxCellLabelXML(label string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "NineBox " + label,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: nineBoxCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{nineBoxCellInset, 0, nineBoxCellInset, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     label,
					Lang:     "en-US",
					FontSize: nineBoxLabelFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateNineBoxCellLabelXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxCellBodyXML produces a roundRect body shape for a nine box cell.
func generateNineBoxCellBodyXML(body string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	paras := panelBulletsParagraphs(body, nineBoxItemFontSize)

	// Use the same accent color for bullets but with full strength.
	bulletColor := pptx.SchemeFill(schemeColor)
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "NineBox Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: nineBoxCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{nineBoxCellInset, nineBoxCellInset, nineBoxCellInset, nineBoxCellInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateNineBoxCellBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxAxisLabelXML produces a text box shape for an axis label.
func generateNineBoxAxisLabelXML(text string, x, y, cx, cy int64, shapeID uint32, fontSize int, align string, bold bool) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Axis Label",
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
		slog.Warn("generateNineBoxAxisLabelXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxAxisTitleVerticalXML produces a rotated (-90°) text box for the Y-axis title.
func generateNineBoxAxisTitleVerticalXML(text string, x, y, cx, cy int64, shapeID uint32) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "Y-Axis Title",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRect,
		Rotation: 16200000, // 270° in 60,000ths of a degree (= -90° = reads bottom-to-top)
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{0, 0, 0, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     text,
					Lang:     "en-US",
					FontSize: nineBoxAxisTitleFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateNineBoxAxisTitleVerticalXML failed", "error", err)
		return ""
	}
	return string(b)
}

// parseNineBoxCells parses cell data from the nine_box_talent diagram data map.
// Supports two formats:
//  1. "cells" array with position, label, and items
//  2. "employees" array with name, performance, potential
func parseNineBoxCells(data map[string]any) []nineBoxCellData {
	// Try "cells" array first.
	if cellsRaw, ok := data["cells"].([]any); ok {
		return parseNineBoxCellsArray(cellsRaw)
	}

	// Try "employees" format.
	if employees, ok := data["employees"].([]any); ok {
		return parseNineBoxEmployees(employees)
	}

	return nil
}

// parseNineBoxCellsArray parses the "cells" format.
func parseNineBoxCellsArray(raw []any) []nineBoxCellData {
	var cells []nineBoxCellData
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}

		cell := nineBoxCellData{}

		// Parse position from nested "position" object.
		if posMap, ok := m["position"].(map[string]any); ok {
			if row, ok := posMap["row"].(float64); ok {
				cell.row = int(row)
			}
			if col, ok := posMap["col"].(float64); ok {
				cell.col = int(col)
			}
		}
		// Also support flat row/col.
		if row, ok := m["row"].(float64); ok {
			cell.row = int(row)
		}
		if col, ok := m["col"].(float64); ok {
			cell.col = int(col)
		}

		// Parse label.
		if label, ok := m["label"].(string); ok {
			cell.label = label
		}

		// Parse items (array of strings or objects with "name").
		if items, ok := m["items"].([]any); ok {
			for _, item := range items {
				switch v := item.(type) {
				case string:
					cell.items = append(cell.items, v)
				case map[string]any:
					if name, ok := v["name"].(string); ok {
						cell.items = append(cell.items, name)
					}
				}
			}
		}

		cells = append(cells, cell)
	}
	return cells
}

// parseNineBoxEmployees parses the "employees" format, grouping by performance/potential.
func parseNineBoxEmployees(employees []any) []nineBoxCellData {
	// Group employees by grid position.
	type posKey struct{ row, col int }
	cellMap := make(map[posKey][]string)

	for _, e := range employees {
		emp, ok := e.(map[string]any)
		if !ok {
			continue
		}

		name, _ := emp["name"].(string)
		if name == "" {
			continue
		}

		performance, _ := emp["performance"].(string)
		potential, _ := emp["potential"].(string)

		row, col := employeeToGridPos(performance, potential)
		key := posKey{row, col}
		cellMap[key] = append(cellMap[key], name)
	}

	var cells []nineBoxCellData
	for pos, names := range cellMap {
		cells = append(cells, nineBoxCellData{
			row:   pos.row,
			col:   pos.col,
			items: names,
		})
	}
	return cells
}

// employeeToGridPos converts performance/potential strings to grid position.
// Performance -> column: low=0, medium=1, high=2
// Potential -> row: high=0, medium=1, low=2 (inverted — high at top)
func employeeToGridPos(performance, potential string) (row, col int) {
	switch performance {
	case "low":
		col = 0
	case "high":
		col = 2
	default:
		col = 1
	}

	switch potential {
	case "high":
		row = 0
	case "low":
		row = 2
	default:
		row = 1
	}
	return
}

// parseNineBoxAxisLabels parses axis labels from the data map.
func parseNineBoxAxisLabels(data map[string]any, key string) [3]string {
	var labels [3]string
	if arr, ok := data[key].([]any); ok {
		for i, l := range arr {
			if i < 3 {
				if s, ok := l.(string); ok {
					labels[i] = s
				}
			}
		}
	}
	return labels
}
