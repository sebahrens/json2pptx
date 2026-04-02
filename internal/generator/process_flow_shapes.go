package generator

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Process Flow Native Shapes — Flowchart steps + connectors in OOXML
// =============================================================================
//
// Replaces SVG-rendered process flow diagrams with native OOXML grouped shapes.
// Step types map to OOXML preset geometries:
//   - step/subprocess → flowChartProcess / flowChartPredefinedProcess
//   - decision        → flowChartDecision
//   - start/end       → flowChartTerminator
//
// Connectors use straightConnector1 or bentConnector3 with triangle arrowheads.
// Layout supports horizontal (single/multi-row with zigzag) and vertical flows.
// All shapes wrapped in a single p:grpSp.

// Process flow EMU constants.
const (
	// pfGap is the gap between steps (EMU). ~0.25"
	pfGap int64 = 228600

	// pfCornerRadius is the roundRect adjustment value for process steps.
	pfCornerRadius int64 = 8000

	// pfLabelFontSize is the step label font size (hundredths of a point). 1100 = 11pt
	pfLabelFontSize int = 1100

	// pfDescFontSize is the description font size (hundredths of a point). 900 = 9pt
	pfDescFontSize int = 900

	// pfConnLabelFontSize is the connection label font size. 800 = 8pt
	pfConnLabelFontSize int = 800

	// pfTextInset is the text inset for step shapes (EMU). ~0.06"
	pfTextInset int64 = 54864

	// pfConnectorWidth is the connector line width in EMU. 12700 = 1pt
	pfConnectorWidth int64 = 12700

	// pfMinStepWidth is the minimum step width (EMU). ~1.0"
	pfMinStepWidth int64 = 914400

	// pfMinStepHeight is the minimum step height (EMU). ~0.5"
	pfMinStepHeight int64 = 457200
)

// processFlowStepType identifies the type of a process step.
type processFlowStepType string

const (
	pfStepType       processFlowStepType = "step"
	pfDecisionType   processFlowStepType = "decision"
	pfStartType      processFlowStepType = "start"
	pfEndType        processFlowStepType = "end"
	pfSubprocessType processFlowStepType = "subprocess"
)

// processFlowStep holds parsed data for a single step.
type processFlowStep struct {
	id          string
	label       string
	description string
	stepType    processFlowStepType
}

// processFlowConnection holds parsed data for a connection between steps.
type processFlowConnection struct {
	from  string
	to    string
	label string
	style string // "solid", "dashed"
}

// processFlowMeta holds metadata for process flow layout.
type processFlowMeta struct {
	stepCount       int
	connectionCount int
	direction       string // "horizontal" or "vertical"
}

// isProcessFlowDiagram returns true if the diagram spec is a process_flow type.
func isProcessFlowDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "process_flow"
}

// processProcessFlowNativeShapes parses process flow data and registers a panelShapeInsert.
func (ctx *singlePassContext) processProcessFlowNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("process flow native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	steps, connections, direction := parseProcessFlowDiagramData(diagramSpec.Data)
	if len(steps) == 0 {
		slog.Warn("process flow native shapes: no steps parsed", "slide", slideNum)
		return
	}

	// Encode step+connection data into panels for the panelShapeInsert system.
	var panels []nativePanelData
	for _, s := range steps {
		panels = append(panels, nativePanelData{
			title: s.label,
			body:  s.description,
			value: fmt.Sprintf("%s:%s", s.stepType, s.id),
		})
	}
	// Encode connections as additional panels with a "conn:" prefix in value.
	for _, c := range connections {
		panels = append(panels, nativePanelData{
			title: c.label,
			value: fmt.Sprintf("conn:%s:%s:%s", c.from, c.to, c.style),
		})
	}

	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native process flow shapes: registered",
		"slide", slideNum,
		"steps", len(steps),
		"connections", len(connections),
		"direction", direction,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx:  shapeIdx,
		bounds:          placeholderBounds,
		panels:          panels,
		processFlowMode: true,
		processFlowMeta: processFlowMeta{
			stepCount:       len(steps),
			connectionCount: len(connections),
			direction:       direction,
		},
	})
}

// parseProcessFlowDiagramData extracts steps, connections, and direction from the diagram data map.
func parseProcessFlowDiagramData(data map[string]any) ([]processFlowStep, []processFlowConnection, string) {
	var steps []processFlowStep

	if stepsRaw, ok := data["steps"].([]any); ok {
		for i, sRaw := range stepsRaw {
			step := processFlowStep{
				id:       fmt.Sprintf("step_%d", i),
				stepType: pfStepType,
			}
			switch s := sRaw.(type) {
			case string:
				step.label = s
			case map[string]any:
				if id, ok := s["id"].(string); ok {
					step.id = id
				}
				if label, ok := s["label"].(string); ok {
					step.label = label
				} else if title, ok := s["title"].(string); ok {
					step.label = title
				} else if name, ok := s["name"].(string); ok {
					step.label = name
				}
				if desc, ok := s["description"].(string); ok {
					step.description = desc
				}
				if typ, ok := s["type"].(string); ok {
					step.stepType = processFlowStepType(typ)
				}
			}
			steps = append(steps, step)
		}
	}

	var connections []processFlowConnection
	if connsRaw, ok := data["connections"].([]any); ok {
		for _, cRaw := range connsRaw {
			if c, ok := cRaw.(map[string]any); ok {
				conn := processFlowConnection{style: "solid"}
				if from, ok := c["from"].(string); ok {
					conn.from = from
				}
				if to, ok := c["to"].(string); ok {
					conn.to = to
				}
				if label, ok := c["label"].(string); ok {
					conn.label = label
				}
				if style, ok := c["style"].(string); ok {
					conn.style = style
				}
				connections = append(connections, conn)
			}
		}
	}

	// Auto-generate sequential connections if none provided.
	if len(connections) == 0 && len(steps) >= 2 {
		connections = generateSequentialFlowConnections(steps)
	}

	direction := "horizontal"
	if d, ok := data["direction"].(string); ok && d != "" {
		direction = d
	}

	return steps, connections, direction
}

// generateSequentialFlowConnections creates default connections for sequential steps.
// Decision steps get "Yes" (next) and "No" (skip one) branches.
func generateSequentialFlowConnections(steps []processFlowStep) []processFlowConnection {
	var conns []processFlowConnection
	for i := 0; i < len(steps)-1; i++ {
		if steps[i].stepType == pfDecisionType {
			conns = append(conns, processFlowConnection{
				from:  steps[i].id,
				to:    steps[i+1].id,
				label: "Yes",
				style: "solid",
			})
			if i+2 < len(steps) {
				conns = append(conns, processFlowConnection{
					from:  steps[i].id,
					to:    steps[i+2].id,
					label: "No",
					style: "dashed",
				})
			}
		} else {
			conns = append(conns, processFlowConnection{
				from:  steps[i].id,
				to:    steps[i+1].id,
				style: "solid",
			})
		}
	}
	return conns
}

// =============================================================================
// Layout Engine — EMU coordinates
// =============================================================================

// pfStepLayout holds the computed position and size for a single step in EMU.
type pfStepLayout struct {
	x, y   int64 // Top-left position
	cx, cy int64 // Width, height
}

// pfLayoutResult holds all computed positions for steps and the flow direction.
type pfLayoutResult struct {
	steps     []pfStepLayout
	direction string
}

// computeProcessFlowLayout calculates EMU positions for all steps within bounds.
func computeProcessFlowLayout(steps []processFlowStep, bounds types.BoundingBox, direction string) pfLayoutResult {
	n := len(steps)
	if n == 0 {
		return pfLayoutResult{direction: direction}
	}

	// Compute step dimensions.
	layouts := make([]pfStepLayout, n)
	for i, s := range steps {
		layouts[i].cx, layouts[i].cy = pfStepDimensions(s, bounds, n)
	}

	// Auto-switch to vertical if horizontal would be too crowded.
	if direction == "horizontal" {
		totalW := int64(0)
		for i, l := range layouts {
			totalW += l.cx
			if i > 0 {
				totalW += pfGap
			}
		}
		if totalW > bounds.Width*2 && n > 4 {
			direction = "vertical"
		}
	}

	if direction == "vertical" {
		return pfLayoutVertical(layouts, bounds)
	}

	// Horizontal layout — check if single row fits.
	totalW := int64(0)
	maxH := int64(0)
	for i, l := range layouts {
		totalW += l.cx
		if i > 0 {
			totalW += pfGap
		}
		if l.cy > maxH {
			maxH = l.cy
		}
	}

	if totalW <= bounds.Width || n <= 4 {
		return pfLayoutSingleRow(layouts, bounds, totalW, maxH)
	}

	return pfLayoutMultiRow(layouts, steps, bounds, maxH, n)
}

// pfStepDimensions returns width and height for a step based on its type and available space.
func pfStepDimensions(step processFlowStep, bounds types.BoundingBox, stepCount int) (cx, cy int64) {
	// Base dimensions scale with available space and step count.
	availPerStep := bounds.Width / int64(stepCount)
	if availPerStep > pfMinStepWidth*3 {
		availPerStep = pfMinStepWidth * 3
	}

	baseW := availPerStep - pfGap
	if baseW < pfMinStepWidth {
		baseW = pfMinStepWidth
	}
	baseH := pfMinStepHeight

	// Give more height when descriptions are present.
	if step.description != "" {
		baseH = pfMinStepHeight * 3 / 2
	}

	switch step.stepType {
	case pfDecisionType:
		// Diamonds: make square-ish for the rotated diamond shape.
		size := baseW
		if baseH > size {
			size = baseH
		}
		// Cap diamond size relative to height.
		maxDiamond := bounds.Height / 3
		if size > maxDiamond {
			size = maxDiamond
		}
		if size < pfMinStepHeight {
			size = pfMinStepHeight
		}
		return size, size
	case pfStartType, pfEndType:
		// Terminators: slightly smaller.
		w := baseW * 85 / 100
		if w < pfMinStepWidth*85/100 {
			w = pfMinStepWidth * 85 / 100
		}
		h := baseH * 80 / 100
		if h < pfMinStepHeight*80/100 {
			h = pfMinStepHeight * 80 / 100
		}
		return w, h
	default:
		return baseW, baseH
	}
}

// pfLayoutVertical positions steps in a single vertical column.
func pfLayoutVertical(layouts []pfStepLayout, bounds types.BoundingBox) pfLayoutResult {
	// Expand widths to fill 80% of available width.
	maxW := bounds.Width * 80 / 100
	for i := range layouts {
		if layouts[i].cx < maxW {
			layouts[i].cx = maxW
		}
	}

	totalH := int64(0)
	for i, l := range layouts {
		totalH += l.cy
		if i > 0 {
			totalH += pfGap
		}
	}

	// Scale down if total exceeds bounds.
	if totalH > bounds.Height {
		scale := float64(bounds.Height) / float64(totalH)
		for i := range layouts {
			layouts[i].cy = int64(float64(layouts[i].cy) * scale)
		}
		totalH = bounds.Height
	}

	startY := bounds.Y + (bounds.Height-totalH)/2
	currentY := startY
	for i := range layouts {
		layouts[i].x = bounds.X + (bounds.Width-layouts[i].cx)/2
		layouts[i].y = currentY
		currentY += layouts[i].cy + pfGap
	}

	return pfLayoutResult{steps: layouts, direction: "vertical"}
}

// pfLayoutSingleRow positions all steps in one horizontal row.
func pfLayoutSingleRow(layouts []pfStepLayout, bounds types.BoundingBox, totalW, maxH int64) pfLayoutResult {
	// Scale down if too wide.
	if totalW > bounds.Width {
		scale := float64(bounds.Width) / float64(totalW)
		totalW = 0
		for i := range layouts {
			layouts[i].cx = int64(float64(layouts[i].cx) * scale)
			layouts[i].cy = int64(float64(layouts[i].cy) * scale)
			totalW += layouts[i].cx
			if i > 0 {
				totalW += pfGap
			}
		}
		maxH = int64(float64(maxH) * scale)
	}

	startX := bounds.X + (bounds.Width-totalW)/2
	centerY := bounds.Y + bounds.Height/2
	currentX := startX
	for i := range layouts {
		layouts[i].x = currentX
		layouts[i].y = centerY - layouts[i].cy/2
		currentX += layouts[i].cx + pfGap
	}

	return pfLayoutResult{steps: layouts, direction: "horizontal"}
}

// pfLayoutMultiRow distributes steps across multiple rows with zigzag ordering.
func pfLayoutMultiRow(layouts []pfStepLayout, steps []processFlowStep, bounds types.BoundingBox, maxH int64, n int) pfLayoutResult {
	// Find number of rows where steps fit.
	numRows := 2
	for numRows <= n {
		perRow := (n + numRows - 1) / numRows
		maxRowW := int64(0)
		for start := 0; start < n; start += perRow {
			end := start + perRow
			if end > n {
				end = n
			}
			rowW := int64(0)
			for i := start; i < end; i++ {
				rowW += layouts[i].cx
				if i > start {
					rowW += pfGap
				}
			}
			if rowW > maxRowW {
				maxRowW = rowW
			}
		}
		if maxRowW <= bounds.Width {
			break
		}
		numRows++
	}

	perRow := (n + numRows - 1) / numRows
	rowSpacing := pfGap * 2

	// Scale if needed.
	totalH := int64(numRows)*maxH + int64(numRows-1)*rowSpacing
	if totalH > bounds.Height {
		scale := float64(bounds.Height) / float64(totalH)
		for i := range layouts {
			layouts[i].cx = int64(float64(layouts[i].cx) * scale)
			layouts[i].cy = int64(float64(layouts[i].cy) * scale)
		}
		maxH = int64(float64(maxH) * scale)
		rowSpacing = int64(float64(rowSpacing) * scale)
		totalH = bounds.Height
	}

	scaledTotalH := int64(numRows)*maxH + int64(numRows-1)*rowSpacing
	startY := bounds.Y + (bounds.Height-scaledTotalH)/2

	for rowIdx := 0; rowIdx < numRows; rowIdx++ {
		startIdx := rowIdx * perRow
		endIdx := startIdx + perRow
		if endIdx > n {
			endIdx = n
		}

		// Compute row width.
		rowW := int64(0)
		for i := startIdx; i < endIdx; i++ {
			rowW += layouts[i].cx
			if i > startIdx {
				rowW += pfGap
			}
		}

		rowCenterY := startY + int64(rowIdx)*(maxH+rowSpacing) + maxH/2
		rowStartX := bounds.X + (bounds.Width-rowW)/2

		if rowIdx%2 == 0 {
			// Left to right.
			currentX := rowStartX
			for i := startIdx; i < endIdx; i++ {
				layouts[i].x = currentX
				layouts[i].y = rowCenterY - layouts[i].cy/2
				currentX += layouts[i].cx + pfGap
			}
		} else {
			// Right to left (zigzag).
			currentX := rowStartX
			for i := endIdx - 1; i >= startIdx; i-- {
				layouts[i].x = currentX
				layouts[i].y = rowCenterY - layouts[i].cy/2
				currentX += layouts[i].cx + pfGap
			}
		}
	}

	return pfLayoutResult{steps: layouts, direction: "horizontal"}
}

// =============================================================================
// Group XML Generation
// =============================================================================

// generateProcessFlowGroupXML produces the complete <p:grpSp> XML for a process flow diagram.
func generateProcessFlowGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta processFlowMeta) string {
	if len(panels) == 0 {
		return ""
	}

	// Decode steps and connections back from panel encoding.
	var steps []processFlowStep
	var connections []processFlowConnection

	for _, p := range panels {
		if strings.HasPrefix(p.value, "conn:") {
			// Connection: "conn:fromID:toID:style"
			parts := strings.SplitN(strings.TrimPrefix(p.value, "conn:"), ":", 3)
			conn := processFlowConnection{label: p.title, style: "solid"}
			if len(parts) >= 1 {
				conn.from = parts[0]
			}
			if len(parts) >= 2 {
				conn.to = parts[1]
			}
			if len(parts) >= 3 && parts[2] != "" {
				conn.style = parts[2]
			}
			connections = append(connections, conn)
		} else {
			// Step: "stepType:id"
			step := processFlowStep{label: p.title, description: p.body, stepType: pfStepType}
			if parts := strings.SplitN(p.value, ":", 2); len(parts) == 2 {
				step.stepType = processFlowStepType(parts[0])
				step.id = parts[1]
			}
			steps = append(steps, step)
		}
	}

	if len(steps) == 0 {
		return ""
	}

	// Compute layout.
	layout := computeProcessFlowLayout(steps, bounds, meta.direction)

	var children [][]byte
	nextID := shapeIDBase + 1

	// Track shape options for connector routing.
	stepShapes := make(map[string]pptx.ShapeOptions)
	stepIDs := make(map[string]uint32)

	// Generate step shapes.
	for i, step := range steps {
		if i >= len(layout.steps) {
			break
		}
		sl := layout.steps[i]
		shapeID := nextID
		stepIDs[step.id] = shapeID
		nextID++

		opts := pfGenerateStepShape(step, sl, shapeID, len(steps))
		stepShapes[step.id] = opts

		b, err := pptx.GenerateShape(opts)
		if err != nil {
			slog.Warn("process flow: step shape failed", "error", err, "step", step.label)
			continue
		}
		children = append(children, b)
	}

	// Generate connectors.
	for _, conn := range connections {
		srcOpts, srcOK := stepShapes[conn.from]
		tgtOpts, tgtOK := stepShapes[conn.to]
		if !srcOK || !tgtOK {
			continue
		}

		connXML := pfGenerateConnector(nextID, srcOpts, tgtOpts, stepIDs[conn.from], stepIDs[conn.to], conn, layout.direction)
		if len(connXML) > 0 {
			children = append(children, connXML)
			nextID++
		}

		// Generate connection label as a separate text shape if present.
		if conn.label != "" {
			labelXML := pfGenerateConnLabel(nextID, srcOpts, tgtOpts, conn.label, layout.direction)
			if len(labelXML) > 0 {
				children = append(children, labelXML)
				nextID++
			}
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Process Flow",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateProcessFlowGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// pfGenerateStepShape builds a ShapeOptions for a process flow step.
func pfGenerateStepShape(step processFlowStep, sl pfStepLayout, shapeID uint32, totalSteps int) pptx.ShapeOptions {
	geom := pfGeometryForStepType(step.stepType)
	fill, line := pfColorsForStepType(step.stepType)

	// Build text paragraphs.
	var paras []pptx.Paragraph

	// Label paragraph — bold, centered.
	labelSize := pfLabelFontSize
	if totalSteps >= 10 {
		labelSize = 900 // 9pt for many steps
	} else if totalSteps >= 7 {
		labelSize = 1000 // 10pt
	}

	paras = append(paras, pptx.Paragraph{
		Align:    "ctr",
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     step.label,
			Lang:     "en-US",
			FontSize: labelSize,
			Bold:     true,
			Dirty:    true,
			Color:    pptx.SchemeFill("dk1"),
		}},
	})

	// Description paragraph — smaller, regular weight.
	if step.description != "" {
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     step.description,
				Lang:     "en-US",
				FontSize: pfDescFontSize,
				Dirty:    true,
				Color:    pptx.SchemeFill("dk1"),
			}},
		})
	}

	var adjustments []pptx.AdjustValue
	if geom == pptx.GeomFlowChartAlternateProcess {
		adjustments = append(adjustments, pptx.AdjustValue{Name: "adj", Value: pfCornerRadius})
	}

	return pptx.ShapeOptions{
		ID:          shapeID,
		Name:        fmt.Sprintf("Step %s", step.label),
		Bounds:      pptx.RectEmu{X: sl.x, Y: sl.y, CX: sl.cx, CY: sl.cy},
		Geometry:    geom,
		Adjustments: adjustments,
		Fill:        fill,
		Line:        line,
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{pfTextInset, pfTextInset, pfTextInset, pfTextInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	}
}

// pfGeometryForStepType maps step types to OOXML preset geometries.
func pfGeometryForStepType(st processFlowStepType) pptx.PresetGeometry {
	switch st {
	case pfDecisionType:
		return pptx.GeomFlowChartDecision
	case pfStartType, pfEndType:
		return pptx.GeomFlowChartTerminator
	case pfSubprocessType:
		return pptx.GeomFlowChartPredefinedProcess
	default:
		return pptx.GeomFlowChartProcess
	}
}

// pfColorsForStepType returns fill and line for each step type using scheme colors.
func pfColorsForStepType(st processFlowStepType) (fill pptx.Fill, line pptx.Line) {
	switch st {
	case pfDecisionType:
		// Warning-tinted fill (accent3 light tint).
		return pptx.SchemeFill("accent3", pptx.LumMod(20000), pptx.LumOff(80000)),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent3")}
	case pfStartType, pfEndType:
		// Success-tinted fill (accent6 light tint).
		return pptx.SchemeFill("accent6", pptx.LumMod(20000), pptx.LumOff(80000)),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent6")}
	case pfSubprocessType:
		// Accent2 fill for subprocess.
		return pptx.SchemeFill("accent2", pptx.LumMod(20000), pptx.LumOff(80000)),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent2")}
	default:
		// Primary accent1 fill for regular steps.
		return pptx.SchemeFill("accent1", pptx.LumMod(20000), pptx.LumOff(80000)),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent1")}
	}
}

// pfGenerateConnector produces a connector between two step shapes.
func pfGenerateConnector(connID uint32, src, tgt pptx.ShapeOptions, srcShapeID, tgtShapeID uint32, conn processFlowConnection, direction string) []byte {
	connBounds, startSite, endSite := pptx.RouteBetween(src, tgt)

	lineOpts := pptx.Line{
		Width: pfConnectorWidth,
		Fill:  pptx.SchemeFill("tx1", pptx.LumMod(50000), pptx.LumOff(50000)),
	}
	if conn.style == "dashed" {
		lineOpts.Dash = "dash"
	}

	// Determine if we need a bent connector for non-aligned shapes.
	geom := pptx.GeomStraightConnector1
	srcCY := src.Bounds.Y + src.Bounds.CY/2
	tgtCY := tgt.Bounds.Y + tgt.Bounds.CY/2
	srcCX := src.Bounds.X + src.Bounds.CX/2
	tgtCX := tgt.Bounds.X + tgt.Bounds.CX/2

	if direction == "horizontal" {
		// Use bent connector if shapes are on different rows.
		if abs64(srcCY-tgtCY) > src.Bounds.CY/2 {
			geom = pptx.GeomBentConnector3
		}
	} else {
		// Vertical: bent connector if not directly above/below.
		if abs64(srcCX-tgtCX) > src.Bounds.CX/2 {
			geom = pptx.GeomBentConnector3
		}
	}

	// Determine flip flags for the connector.
	flipH := connBounds.CX < 0
	flipV := connBounds.CY < 0
	if flipH {
		connBounds.CX = -connBounds.CX
	}
	if flipV {
		connBounds.CY = -connBounds.CY
	}

	b, err := pptx.GenerateConnector(pptx.ConnectorOptions{
		ID:       connID,
		Name:     fmt.Sprintf("Flow Connector %d", connID),
		Geometry: geom,
		Bounds:   connBounds,
		Line:     lineOpts,
		TailEnd: &pptx.ArrowHead{
			Type: "triangle",
			W:    "med",
			Len:  "med",
		},
		StartConn: &pptx.ConnectionRef{
			ShapeID: srcShapeID,
			SiteIdx: startSite,
		},
		EndConn: &pptx.ConnectionRef{
			ShapeID: tgtShapeID,
			SiteIdx: endSite,
		},
		FlipH: flipH,
		FlipV: flipV,
	})
	if err != nil {
		slog.Warn("process flow connector failed", "error", err)
		return nil
	}
	return b
}

// pfGenerateConnLabel produces a small text shape for a connection label,
// positioned at the midpoint between two shapes.
func pfGenerateConnLabel(shapeID uint32, src, tgt pptx.ShapeOptions, label, direction string) []byte {
	// Compute midpoint between shape centers.
	srcCX := src.Bounds.X + src.Bounds.CX/2
	srcCY := src.Bounds.Y + src.Bounds.CY/2
	tgtCX := tgt.Bounds.X + tgt.Bounds.CX/2
	tgtCY := tgt.Bounds.Y + tgt.Bounds.CY/2

	midX := (srcCX + tgtCX) / 2
	midY := (srcCY + tgtCY) / 2

	// Small text box for the label.
	labelW := int64(457200) // ~0.5"
	labelH := int64(182880) // ~0.2"

	// Offset perpendicular to the flow direction.
	offsetAmt := int64(91440) // ~0.1"
	x := midX - labelW/2
	y := midY - labelH/2

	if direction == "horizontal" {
		y -= offsetAmt // Shift above the connector line
	} else {
		x += offsetAmt // Shift to the right of the connector line
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Conn Label %s", label),
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: labelW, CY: labelH},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "ctr",
			Insets: [4]int64{0, 0, 0, 0},
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     label,
					Lang:     "en-US",
					FontSize: pfConnLabelFontSize,
					Italic:   true,
					Dirty:    true,
					Color:    pptx.SchemeFill("tx1", pptx.LumMod(65000), pptx.LumOff(35000)),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("process flow conn label failed", "error", err)
		return nil
	}
	return b
}

// abs64 returns the absolute value of an int64.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// pfEstimateShapeCount returns the estimated number of shapes for ID allocation.
// 1 (group) + N (step shapes) + M (connectors) + L (connection labels)
func pfEstimateShapeCount(panels []nativePanelData) uint32 {
	steps := 0
	conns := 0
	labels := 0
	for _, p := range panels {
		if strings.HasPrefix(p.value, "conn:") {
			conns++
			if p.title != "" {
				labels++
			}
		} else {
			steps++
		}
	}
	return uint32(1 + steps + conns + labels)
}

// Ensure math import is used.
var _ = math.Min
