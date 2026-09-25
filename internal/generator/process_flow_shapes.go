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

	// pfStepHeightDivisor sizes a step against the space it has: a quarter of
	// the available height, which leaves room for a second row of steps and for
	// the descriptions under them.
	pfStepHeightDivisor int64 = 4

	// pfMaxStepHeightFactor caps the step at twice the minimum (~1.0"). A flow
	// reads as a strip; a box tall enough to fill a body placeholder would be
	// worse than a short one.
	pfMaxStepHeightFactor int64 = 2

	// Vertical flows need whitespace on both sides of the spine for decision
	// branches. Steps are content-sized but never consume more than 40% of the
	// frame width; branch targets sit on the 25% / 75% lanes.
	pfVerticalMaxWidthPct    int64 = 40
	pfVerticalLeftCenterPct  int64 = 25
	pfVerticalRightCenterPct int64 = 75

	// Approximate text widths used only to choose a sensible native-shape width.
	// PowerPoint still performs the authoritative normAutofit inside the box.
	pfLabelGlyphWidthEMU int64 = 7 * 12700
	pfBodyGlyphWidthEMU  int64 = 5 * 12700
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
		altText:         diagramAltText(item),
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
func parseProcessFlowDiagramData(data map[string]any) ([]processFlowStep, []processFlowConnection, string) { //nolint:gocognit
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
func computeProcessFlowLayout(steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox, direction string) pfLayoutResult {
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
		return pfLayoutVertical(layouts, steps, connections, bounds)
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

// pfStepHeight is the height of one process step, scaled to the space it has.
//
// The step height used to be the 0.5" minimum whatever the placeholder was, so
// a five-step flow drew a 0.5" strip of boxes in the middle of a 4.75" body and
// read as an unfinished slide (go-slide-creator-rkq0). It scales with the
// available height now, capped at twice the minimum: a flow is a strip, and a
// 3"-tall box would be worse than a short one, so the box grows to a readable
// size and stops.
func pfStepHeight(bounds types.BoundingBox) int64 {
	if bounds.Height <= 0 {
		return pfMinStepHeight
	}
	h := bounds.Height / pfStepHeightDivisor
	if h < pfMinStepHeight {
		return pfMinStepHeight
	}
	if max := pfMinStepHeight * pfMaxStepHeightFactor; h > max {
		return max
	}
	return h
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
	baseH := pfStepHeight(bounds)

	// Give more height when descriptions are present.
	if step.description != "" {
		baseH = baseH * 3 / 2
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

// pfLayoutVertical positions steps on a central spine, with the direct targets
// of a decision placed on left/right lanes. This makes Yes/No paths visually
// distinct while preserving the authored step order and merge connections.
func pfLayoutVertical(layouts []pfStepLayout, steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox) pfLayoutResult {
	for i := range layouts {
		layouts[i].cx = pfVerticalStepWidth(steps[i], layouts[i], bounds)
	}

	gap := pfVerticalGap(len(layouts), bounds.Height)
	availableForSteps := bounds.Height - int64(len(layouts)-1)*gap
	if availableForSteps < 1 {
		availableForSteps = 1
	}
	stepHeight := int64(0)
	for _, layout := range layouts {
		stepHeight += layout.cy
	}
	if stepHeight > availableForSteps {
		scale := float64(availableForSteps) / float64(stepHeight)
		stepHeight = 0
		for i := range layouts {
			layouts[i].cy = pfMax64(1, int64(float64(layouts[i].cy)*scale))
			stepHeight += layouts[i].cy
		}
	}
	pfConstrainVerticalDecisionAspect(layouts, steps)
	totalH := stepHeight + int64(len(layouts)-1)*gap
	startY := bounds.Y + pfMax64(0, bounds.Height-totalH)/2
	currentY := startY
	for i := range layouts {
		layouts[i].x = bounds.X + (bounds.Width-layouts[i].cx)/2
		layouts[i].y = currentY
		currentY += layouts[i].cy + gap
	}
	pfPlaceVerticalDecisionBranches(layouts, steps, connections, bounds)

	return pfLayoutResult{steps: layouts, direction: "vertical"}
}

func pfConstrainVerticalDecisionAspect(layouts []pfStepLayout, steps []processFlowStep) {
	for i := range layouts {
		if steps[i].stepType != pfDecisionType {
			continue
		}
		maxW := layouts[i].cy * 2
		if maxW < pfMinStepWidth {
			maxW = pfMinStepWidth
		}
		if layouts[i].cx > maxW {
			layouts[i].cx = maxW
		}
	}
}

func pfVerticalStepWidth(step processFlowStep, layout pfStepLayout, bounds types.BoundingBox) int64 {
	capW := bounds.Width * pfVerticalMaxWidthPct / 100
	if capW < pfMinStepWidth {
		capW = bounds.Width
	}
	w := layout.cx
	labelW := int64(len([]rune(step.label)))*pfLabelGlyphWidthEMU + 2*pfTextInset
	if step.stepType == pfDecisionType {
		// Only the central portion of a diamond is usable for text.
		labelW = labelW * 3 / 2
	}
	if labelW > w {
		w = labelW
	}
	if step.description != "" {
		// Descriptions should wrap at roughly 36 characters rather than turn a
		// vertical process into a row of slide-wide banners.
		chars := len([]rune(step.description))
		if chars > 36 {
			chars = 36
		}
		bodyW := int64(chars)*pfBodyGlyphWidthEMU + 2*pfTextInset
		if bodyW > w {
			w = bodyW
		}
	}
	if w < pfMinStepWidth {
		w = pfMinStepWidth
	}
	if w > capW {
		w = capW
	}
	return w
}

func pfVerticalGap(stepCount int, height int64) int64 {
	if stepCount <= 1 {
		return 0
	}
	gap := pfGap
	// Connectors and their 0.2" labels get at most one third of the frame;
	// dense flows retain visible gaps without pushing the final step outside.
	if maxGap := height / (3 * int64(stepCount-1)); gap > maxGap {
		gap = maxGap
	}
	if gap < 1 {
		return 1
	}
	return gap
}

func pfPlaceVerticalDecisionBranches(layouts []pfStepLayout, steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox) {
	index := make(map[string]int, len(steps))
	for i, step := range steps {
		index[step.id] = i
	}
	for i, step := range steps {
		if step.stepType != pfDecisionType {
			continue
		}
		var outgoing []processFlowConnection
		for _, conn := range connections {
			if conn.from == step.id {
				outgoing = append(outgoing, conn)
			}
		}
		if len(outgoing) < 2 {
			continue
		}
		usedLeft, usedRight := false, false
		for branchIndex, conn := range outgoing {
			target, ok := index[conn.to]
			if !ok || target == i {
				continue
			}
			side := pfVerticalBranchSide(conn.label, branchIndex, usedLeft, usedRight)
			if side < 0 {
				usedLeft = true
				pfCenterLayoutAt(&layouts[target], bounds, pfVerticalLeftCenterPct)
			} else {
				usedRight = true
				pfCenterLayoutAt(&layouts[target], bounds, pfVerticalRightCenterPct)
			}
		}
	}
}

func pfVerticalBranchSide(label string, index int, usedLeft, usedRight bool) int {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "yes", "y", "true":
		return -1
	case "no", "n", "false":
		return 1
	}
	if !usedLeft {
		return -1
	}
	if !usedRight {
		return 1
	}
	if index%2 == 0 {
		return -1
	}
	return 1
}

func pfCenterLayoutAt(layout *pfStepLayout, bounds types.BoundingBox, centerPct int64) {
	center := bounds.X + bounds.Width*centerPct/100
	x := center - layout.cx/2
	minX, maxX := bounds.X, bounds.X+bounds.Width-layout.cx
	if x < minX {
		x = minX
	}
	if x > maxX {
		x = maxX
	}
	layout.x = x
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
func pfLayoutMultiRow(layouts []pfStepLayout, steps []processFlowStep, bounds types.BoundingBox, maxH int64, n int) pfLayoutResult { //nolint:gocognit
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
	layout := computeProcessFlowLayout(steps, connections, bounds, meta.direction)

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
		return diagramTintFill("accent3", 20000, 80000),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent3")}
	case pfStartType, pfEndType:
		// Success-tinted fill (accent6 light tint).
		return diagramTintFill("accent6", 20000, 80000),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent6")}
	case pfSubprocessType:
		// Accent2 fill for subprocess.
		return diagramTintFill("accent2", 20000, 80000),
			pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("accent2")}
	default:
		// Primary accent1 fill for regular steps.
		return diagramTintFill("accent1", 20000, 80000),
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
	labelBounds := pfConnLabelBounds(src.Bounds, tgt.Bounds, direction)

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Conn Label %s", label),
		Bounds:   labelBounds,
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

func pfConnLabelBounds(src, tgt pptx.RectEmu, direction string) pptx.RectEmu {
	const (
		labelW    int64 = 457200 // ~0.5"
		labelH    int64 = 182880 // ~0.2"
		offsetAmt int64 = 91440  // ~0.1"
	)
	srcCX, srcCY := src.X+src.CX/2, src.Y+src.CY/2
	tgtCX, tgtCY := tgt.X+tgt.CX/2, tgt.Y+tgt.CY/2
	midX, midY := (srcCX+tgtCX)/2, (srcCY+tgtCY)/2
	if direction == "vertical" {
		// Centre the label in the actual edge-to-edge gap, not halfway between
		// shape centres (which can put it on top of a tall target).
		if tgtCY >= srcCY {
			midY = (src.Y + src.CY + tgt.Y) / 2
		} else {
			midY = (tgt.Y + tgt.CY + src.Y) / 2
		}
		x := midX + offsetAmt
		if tgtCX < srcCX {
			x = midX - labelW - offsetAmt
		}
		return pptx.RectEmu{X: x, Y: midY - labelH/2, CX: labelW, CY: labelH}
	}
	return pptx.RectEmu{X: midX - labelW/2, Y: midY - labelH/2 - offsetAmt, CX: labelW, CY: labelH}
}

func pfMax64(a, b int64) int64 {
	if a > b {
		return a
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
