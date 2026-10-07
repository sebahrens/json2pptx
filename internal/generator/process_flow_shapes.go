package generator

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
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
//
// Fills, outlines and connectors are the process-flow pattern's tinted look
// (patterns.ProcessFlow*): neutral steps without an outline, decisions
// outlined in the accent, accent connectors on a 2pt line with the large
// arrowhead (go-slide-creator-6shxx).

// Process flow EMU constants.
const (
	// pfGap is the gap between the steps of a horizontal flow (EMU), 0.3":
	// the connector's length, above the process-flow pattern's 20pt minimum.
	pfGap int64 = 274320

	// pfVerticalStepGap is the gap between the steps of a vertical flow
	// (EMU), 0.25", before pfVerticalGap fits it to the frame.
	pfVerticalStepGap int64 = 228600

	// pfCornerRadius is the roundRect adjustment value for process steps.
	pfCornerRadius int64 = 8000

	// Primary process labels must remain readable at slide scale.
	pfLabelFontSize int = 1800

	// Supporting descriptions use a smaller, distinct type role.
	pfDescFontSize int = 1400

	// Connection labels are utility text, not primary step content.
	pfConnLabelFontSize int = 1200

	// pfTextInset is the text inset for step shapes (EMU): the uniform 0.5 cm
	// shape text margin.
	pfTextInset = pptx.ShapeTextInsetEMU

	// pfConnectorWidth is the connector line width in EMU: 1.5pt.
	pfConnectorWidth int64 = 19050

	// pfConnectorInkPct is the neutral ink coverage of a connector: dk1 at
	// 50%, which reads as a line on every template's page.
	pfConnectorInkPct = 50

	// pfMinDescStepWidth is the narrowest step that carries a description
	// (EMU, 1.75"): about eighteen characters of 14pt text per line. Narrower
	// and a one-sentence description becomes a five-line column.
	pfMinDescStepWidth int64 = 1600200

	// pfMaxRows is the most rows a horizontal flow wraps into before it
	// switches to the vertical layout.
	pfMaxRows = 3

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
	// themeColors resolve the tonal roles (content tint, emphasis, inks)
	// against the template; empty draws the template-independent defaults.
	themeColors     []types.ThemeColor
	fontName        string
	stepCount       int
	connectionCount int
	direction       string // "horizontal" or "vertical"
}

// isProcessFlowDiagram returns true if the diagram spec is a process_flow type.
func isProcessFlowDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "process_flow"
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

	// keepWidth marks a decision diamond widened so its label fits at 14pt
	// or more: a too-wide single row narrows the other steps first.
	keepWidth bool
}

// pfLayoutResult holds all computed positions for steps and the flow direction.
type pfLayoutResult struct {
	steps     []pfStepLayout
	direction string
	// band is set when the flow is drawn as interlocking arrows
	// (process_flow_band.go); nil is the flowchart of boxes and connectors.
	band *pfBand
}

// computeProcessFlowLayout calculates EMU positions for all steps within bounds.
func computeProcessFlowLayout(steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox, direction string, fonts ...string) pfLayoutResult {
	n := len(steps)
	if n == 0 {
		return pfLayoutResult{direction: direction}
	}

	// Compute step dimensions.
	font := defaultFontFamily
	if len(fonts) > 0 && fonts[0] != "" {
		font = fonts[0]
	}
	// A plain sequence is a band of interlocking arrows.
	if band, ok := computeProcessFlowBand(steps, connections, bounds, direction, font); ok {
		return band
	}
	// A horizontal flow holds as many steps in a row as stay readable; the
	// rest wrap. More rows than pfMaxRows read better as a vertical flow.
	perRow := n
	if direction == "horizontal" {
		perRow = pfStepsPerRow(steps, bounds, font)
		if rows := (n + perRow - 1) / perRow; rows > pfMaxRows && n > 4 {
			direction = "vertical"
			perRow = n
		}
	}
	layouts := make([]pfStepLayout, n)
	for i, s := range steps {
		layouts[i].cx, layouts[i].cy = pfStepDimensions(s, bounds, perRow)
		pfGrowTextHeight(&layouts[i], s, font)
		pfWidenDecision(&layouts[i], s, font, bounds)
	}

	if direction == "vertical" {
		return pfLayoutVertical(layouts, steps, connections, bounds, font)
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

// pfReadableStepWidth is the narrowest step in which every label keeps its
// widest word whole at the label size inside the uniform text margin, and a
// description gets a readable line length.
func pfReadableStepWidth(steps []processFlowStep, font string) int64 {
	minW := pfMinStepWidth
	for _, s := range steps {
		if s.stepType == pfDecisionType {
			// A decision sizes itself (pfWidenDecision).
			continue
		}
		// Half the uniform margin on each side: the writer gives a word the
		// rest of the margin back before it lets it break
		// (pptx.EffectiveTextInsets), so the full margin is not the floor.
		word := int64(math.Ceil(float64(pfWidestLabelWordEMU(s, font)) * pptx.StandInWordFitSlack))
		wPct, _ := pfTextAreaPercent(s.stepType)
		minW = max(minW, (word+pfTextInset)*100/wPct)
		if s.description != "" {
			minW = max(minW, pfMinDescStepWidth)
		}
	}
	return minW
}

// pfStepsPerRow is how many steps a horizontal flow puts in one row: all of
// them when each still gets a readable width (and always for four or fewer),
// otherwise the rows are balanced — seven steps wrap 4 + 3, not 6 + 1. Small
// outlined boxes in one strip left most of a body placeholder empty and the
// text below body size (go-slide-creator-6shxx).
func pfStepsPerRow(steps []processFlowStep, bounds types.BoundingBox, font string) int {
	n := len(steps)
	if n <= 4 || bounds.Width <= 0 {
		return max(n, 1)
	}
	fit := int((bounds.Width + pfGap) / (pfReadableStepWidth(steps, font) + pfGap))
	if fit < 1 {
		fit = 1
	}
	if n <= fit {
		return n
	}
	rows := (n + fit - 1) / fit
	return (n + rows - 1) / rows
}

// Preset text rectangles are narrower than the exterior geometry. In
// particular, terminators must not measure their text as full-width boxes.
func pfTextAreaPercent(kind processFlowStepType) (width, height int64) {
	switch kind {
	case pfDecisionType:
		return 50, 50
	case pfStartType, pfEndType:
		// flowChartTerminator's text rectangle: 90.6% x 70.7% of the box.
		return 90, 70
	default:
		return 100, 100
	}
}

// pfTextArea is the text area the writer leaves in a step box: the preset's
// own text rectangle (pfTextAreaPercent of the box) minus the uniform shape
// text margin, which the writer clamps on an axis too short for one label
// line or for the label's widest word (pptx.EffectiveTextInsets).
func pfTextArea(step processFlowStep, font string, width, height int64) (int64, int64) {
	w, h := pfTextAreaPercent(step.stepType)
	rectW, rectH := width*w/100, height*h/100
	lineH := int64(pfLabelFontSize) * 127 * 12 / 10
	// Native bodies carry no theme fonts, so the writer's clamp measures them
	// in its stand-in face and leaves the stand-in slack.
	wordNeed := int64(math.Ceil(float64(pfWidestLabelWordEMU(step, font)) * pptx.StandInWordFitSlack))
	return max(1, rectW-2*pptx.UniformInsetFor(rectW, wordNeed)),
		max(1, rectH-2*pptx.UniformInsetFor(rectH, lineH))
}

// pfWidestLabelWordEMU is the width of the step label's widest word, bold at
// the label size — the horizontal room the writer's clamp keeps.
func pfWidestLabelWordEMU(step processFlowStep, font string) int64 {
	var widest int64
	for _, word := range strings.Fields(step.label) {
		if w, err := textfit.MeasureStyledLineWidth(word, font, float64(pfLabelFontSize)/100, true); err == nil {
			widest = max(widest, w)
		}
	}
	return widest
}

func pfRequiredTextHeight(step processFlowStep, font string, width int64) int64 {
	m, err := textfit.MeasureStyledRuns(textfit.StyledMeasureParams{
		Runs:     []textfit.StyledRun{{Text: step.label, Bold: true}},
		FontName: font, FontPt: float64(pfLabelFontSize) / 100, WidthEMU: width,
	})
	if err != nil {
		return 0
	}
	return m.RequiredEMU + measureNativeText(step.description, font, float64(pfDescFontSize)/100, width)
}

func pfGrowTextHeight(layout *pfStepLayout, step processFlowStep, font string) {
	if step.stepType == pfDecisionType && step.description == "" {
		// The label is fitted to the diamond at draw time
		// (pfDecisionLabelLines). Growing the height for a word broken inside
		// the half-width text rectangle only made the diamond tall and narrow.
		return
	}
	layout.cy = max(layout.cy, pfMinimumTextHeight(*layout, step, font))
}

func pfMinimumTextHeight(layout pfStepLayout, step processFlowStep, font string) int64 {
	w, _ := pfTextArea(step, font, layout.cx, layout.cy)
	required := pfRequiredTextHeight(step, font, w)
	_, heightPct := pfTextAreaPercent(step.stepType)
	return ((required+2*pfTextInset)*100 + heightPct - 1) / heightPct
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

// pfStepDimensions returns width and height for a step based on its type and
// available space; perRow is the number of steps sharing its row.
func pfStepDimensions(step processFlowStep, bounds types.BoundingBox, perRow int) (cx, cy int64) {
	// A row's steps share its width, gaps included, up to 3" each.
	if perRow < 1 {
		perRow = 1
	}
	availPerStep := (bounds.Width + pfGap) / int64(perRow)
	if availPerStep > pfMinStepWidth*3+pfGap {
		availPerStep = pfMinStepWidth*3 + pfGap
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
func pfLayoutVertical(layouts []pfStepLayout, steps []processFlowStep, connections []processFlowConnection, bounds types.BoundingBox, font string) pfLayoutResult {
	for i := range layouts {
		layouts[i].cx = pfVerticalStepWidth(steps[i], layouts[i], bounds, font)
		// The vertical lane can be narrower than the initial horizontal box.
		// Measure again at that actual width before allocating its height.
		pfGrowTextHeight(&layouts[i], steps[i], font)
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
		minimums := make([]int64, len(layouts))
		minimumHeight := int64(0)
		for i, layout := range layouts {
			minimums[i] = pfMinimumTextHeight(layout, steps[i], font)
			minimumHeight += minimums[i]
		}
		scale := float64(availableForSteps) / float64(stepHeight)
		if minimumHeight <= availableForSteps && stepHeight > minimumHeight {
			// Remove decorative whitespace before sacrificing readable text.
			scale = float64(availableForSteps-minimumHeight) / float64(stepHeight-minimumHeight)
		}
		stepHeight = 0
		for i := range layouts {
			if minimumHeight <= availableForSteps {
				layouts[i].cy = minimums[i] + int64(float64(layouts[i].cy-minimums[i])*scale)
			} else {
				layouts[i].cy = pfMax64(1, int64(float64(layouts[i].cy)*scale))
			}
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

func pfVerticalStepWidth(step processFlowStep, layout pfStepLayout, bounds types.BoundingBox, font string) int64 {
	capW := bounds.Width * pfVerticalMaxWidthPct / 100
	if capW < pfMinStepWidth {
		capW = bounds.Width
	}
	w := layout.cx
	// The label's one-line width, measured bold, plus the uniform shape
	// margin; the glyph estimate stands in when no font can be measured.
	textW := int64(len([]rune(step.label))) * pfLabelGlyphWidthEMU
	if m, err := textfit.MeasureStyledLineWidth(step.label, font, float64(pfLabelFontSize)/100, true); err == nil && m > 0 {
		textW = m + m/50
	}
	labelW := textW + 2*pfTextInset
	// A preset's text rectangle is only part of its box: half of a diamond,
	// nine tenths of a terminator.
	switch step.stepType {
	case pfDecisionType:
		labelW *= 2
	case pfStartType, pfEndType:
		labelW = labelW * 100 / 90
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
	gap := pfVerticalStepGap
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
	// Scale down if too wide. A widened decision keeps its width while the
	// other steps can absorb the squeeze (to at most half their width);
	// otherwise everything scales together.
	if totalW > bounds.Width {
		var keptW, flexW int64
		for _, l := range layouts {
			if l.keepWidth {
				keptW += l.cx
			} else {
				flexW += l.cx
			}
		}
		gaps := totalW - keptW - flexW
		flexScale := 0.0
		if flexW > 0 && keptW > 0 {
			flexScale = float64(bounds.Width-gaps-keptW) / float64(flexW)
		}
		uniform := flexScale < 0.5
		scale := float64(bounds.Width) / float64(totalW)
		totalW = 0
		for i := range layouts {
			s := scale
			if !uniform {
				s = flexScale
				if layouts[i].keepWidth {
					s = 1
				}
			}
			layouts[i].cx = int64(float64(layouts[i].cx) * s)
			layouts[i].cy = int64(float64(layouts[i].cy) * s)
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
		// Long descriptions must not grow a centered box into footer chrome.
		// Preflight reports the remaining text-capacity deficit at this height.
		if bounds.Height > 0 {
			layouts[i].cy = min(layouts[i].cy, bounds.Height)
		}
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

	// The widest row sets the block every row aligns to.
	gridW := int64(0)
	for start := 0; start < n; start += perRow {
		rowW := int64(0)
		for i := start; i < min(start+perRow, n); i++ {
			rowW += layouts[i].cx
			if i > start {
				rowW += pfGap
			}
		}
		gridW = max(gridW, rowW)
	}
	gridX := bounds.X + (bounds.Width-gridW)/2

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

		// Rows share one left and one right edge, so the turn from a row's
		// last step to the next row's first is a straight drop: a short last
		// row hangs from the side the flow arrives on.
		rowCenterY := startY + int64(rowIdx)*(maxH+rowSpacing) + maxH/2
		rowStartX := gridX
		if rowIdx%2 == 1 {
			rowStartX = gridX + gridW - rowW
		}

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

// pfDecodePanels decodes steps and connections back from the panel encoding:
// a step rides as "stepType:id" in value, a connection as
// "conn:fromID:toID:style".
func pfDecodePanels(panels []nativePanelData) ([]processFlowStep, []processFlowConnection) {
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

	return steps, connections
}

// generateProcessFlowGroupXML produces the complete <p:grpSp> XML for a process flow diagram.
func generateProcessFlowGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta processFlowMeta) string {
	if len(panels) == 0 {
		return ""
	}

	steps, connections := pfDecodePanels(panels)
	if len(steps) == 0 {
		return ""
	}

	// Compute layout.
	layout := computeProcessFlowLayout(steps, connections, bounds, meta.direction, meta.fontName)

	var children [][]byte
	nextID := shapeIDBase + 1

	// Track shape options for connector routing.
	stepShapes := make(map[string]pptx.ShapeOptions)
	stepIDs := make(map[string]uint32)

	surface := nativeSurface{colors: meta.themeColors}
	var bandTones pfBandTones
	if layout.band != nil {
		bandTones = pfBandTonesFor(steps, surface)
		// The interlocking arrows are the sequence: no connectors.
		connections = nil
	}

	// Generate step shapes.
	for i, step := range steps {
		if i >= len(layout.steps) {
			break
		}
		sl := layout.steps[i]
		shapeID := nextID
		stepIDs[step.id] = shapeID
		nextID++

		var opts pptx.ShapeOptions
		if layout.band != nil {
			opts = pfBandStepShape(steps, i, sl, layout.band, bandTones, shapeID, meta.fontName)
		} else {
			opts = pfGenerateStepShape(step, sl, shapeID, len(steps), meta.fontName)
			pfApplyFlowchartTone(&opts, step.stepType, surface)
		}
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
			labelXML := pfGenerateConnLabel(nextID, srcOpts, tgtOpts, conn.label, layout.direction, meta.fontName)
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
func pfGenerateStepShape(step processFlowStep, sl pfStepLayout, shapeID uint32, totalSteps int, fonts ...string) pptx.ShapeOptions {
	geom := pfGeometryForStepType(step.stepType)
	fill, line := pfColorsForStepType(step.stepType)
	font := defaultFontFamily
	if len(fonts) > 0 && fonts[0] != "" {
		font = fonts[0]
	}

	// Build text paragraphs.
	var paras []pptx.Paragraph

	// More steps must not silently lower the primary text role.
	labelSize := pfLabelFontSize
	labelLines := []string{step.label}
	wrap := "square"
	autoFit := "normAutofit"
	insets := pptx.ShapeTextInsets()
	if step.stepType == pfDecisionType {
		// A diamond's preset text rectangle is only the middle half of its
		// width, so "Approved?" at 18pt was force-broken mid-word
		// (go-slide-creator-acydi). The label is broken at spaces only, at the
		// largest size in [14pt, 18pt] whose lines fit the diamond's visible
		// width at their height, and drawn unwrapped so no renderer can split
		// a word.
		if step.description == "" {
			labelSize, labelLines = pfDecisionLabelLines(step.label, font, sl.cx, sl.cy)
			// Unwrapped and not autofit: LibreOffice's shrink-on-overflow
			// re-wraps an unwrapped body inside the preset text rectangle.
			wrap = "none"
			autoFit = ""
			insets = [4]int64{}
		} else {
			labelSize = pfDecisionWrappedLabelSize(step, font, sl.cx)
		}
	}

	for _, text := range labelLines {
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     text,
				Lang:     "en-US",
				FontSize: labelSize,
				Bold:     true,
				Dirty:    true,
				Color:    pptx.SchemeFill("dk1"),
			}},
		})
	}

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
			Wrap:       wrap,
			Anchor:     "ctr",
			Insets:     insets,
			AutoFit:    autoFit,
			Paragraphs: paras,
		},
	}
}

// Decision label sizes, hundredths of a point: never above the step label
// role, never below the 14pt the review set as the floor for a decision.
const (
	pfDecisionMinLabelSize = 1400
	pfDecisionSizeStep     = 100
	pfDecisionMaxLines     = 3
)

// pfDecisionLabelLines breaks a decision label at spaces into the fewest lines
// at the largest size in [14pt, 18pt] where every line fits the diamond. A
// line block of height H centred in a diamond of cx x cy has cx*(1-H/cy) of
// visible width at its top and bottom edge; a margin keeps the text off the
// outline. When nothing fits, the label is drawn at 14pt in as few lines as
// its words allow — still never split inside a word.
func pfDecisionLabelLines(label, font string, cx, cy int64) (int, []string) {
	words := strings.Fields(label)
	if len(words) == 0 || cx <= 0 || cy <= 0 {
		return pfLabelFontSize, []string{label}
	}
	const margin int64 = 2 * 45720 // 0.05" each side
	for size := pfLabelFontSize; size >= pfDecisionMinLabelSize; size -= pfDecisionSizeStep {
		lineH := int64(size) * 127 * 12 / 10
		for n := 1; n <= pfDecisionMaxLines && n <= len(words); n++ {
			blockH := int64(n) * lineH
			if blockH >= cy {
				break
			}
			allowed := cx*(cy-blockH)/cy - margin
			if lines, ok := pfGreedyLines(words, font, size, allowed); ok && len(lines) <= n {
				return size, lines
			}
		}
	}
	lines, _ := pfGreedyLines(words, font, pfDecisionMinLabelSize, cx/2)
	return pfDecisionMinLabelSize, lines
}

// pfWidenDecision widens a decision diamond (height unchanged) until its
// label fits at 14pt or more, up to a third of the frame width, so a single
// long word is never broken inside the diamond (go-slide-creator-acydi).
func pfWidenDecision(layout *pfStepLayout, step processFlowStep, font string, bounds types.BoundingBox) {
	if step.stepType != pfDecisionType || step.description != "" || layout.cy <= 0 {
		return
	}
	if pfDecisionLabelFits(step.label, font, layout.cx, layout.cy) {
		return
	}
	limit := max(layout.cx, bounds.Width/3)
	for w := layout.cx + layout.cx/20; w <= limit; w += max(1, layout.cx/20) {
		if pfDecisionLabelFits(step.label, font, w, layout.cy) {
			layout.cx = w
			layout.keepWidth = true
			return
		}
	}
	layout.cx = limit
	layout.keepWidth = true
}

// pfDecisionLabelFits reports whether pfDecisionLabelLines finds a size in
// [14pt, 18pt] at which the label fits the diamond without the fallback.
func pfDecisionLabelFits(label, font string, cx, cy int64) bool {
	size, lines := pfDecisionLabelLines(label, font, cx, cy)
	if size > pfDecisionMinLabelSize {
		return true
	}
	lineH := int64(size) * 127 * 12 / 10
	blockH := int64(len(lines)) * lineH
	if blockH >= cy {
		return false
	}
	allowed := cx*(cy-blockH)/cy - 2*45720
	_, ok := pfGreedyLines(lines, font, size, allowed)
	return ok
}

// pfGreedyLines packs words into lines no wider than maxW; ok is false when
// a single word is wider on its own.
func pfGreedyLines(words []string, font string, size int, maxW int64) ([]string, bool) {
	width := func(s string) int64 {
		w, err := textfit.MeasureStyledLineWidth(s, font, float64(size)/100, true)
		if err != nil || w <= 0 {
			return int64(len([]rune(s))) * pfLabelGlyphWidthEMU * int64(size) / int64(pfLabelFontSize)
		}
		return w + w/50
	}
	ok := true
	var lines []string
	cur := ""
	for _, word := range words {
		if width(word) > maxW {
			ok = false
		}
		next := word
		if cur != "" {
			next = cur + " " + word
		}
		if cur != "" && width(next) > maxW {
			lines = append(lines, cur)
			cur = word
			continue
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines, ok
}

// pfDecisionWrappedLabelSize is the size for a decision that also carries a
// description, whose body must wrap: the largest size in [14pt, 18pt] at
// which the label's widest word fits the preset's half-width text rectangle.
func pfDecisionWrappedLabelSize(step processFlowStep, font string, cx int64) int {
	rectW := cx / 2
	for size := pfLabelFontSize; size > pfDecisionMinLabelSize; size -= pfDecisionSizeStep {
		var widest int64
		for _, word := range strings.Fields(step.label) {
			if w, err := textfit.MeasureStyledLineWidth(word, font, float64(size)/100, true); err == nil {
				widest = max(widest, w)
			}
		}
		if widest <= rectW {
			return size
		}
	}
	return pfDecisionMinLabelSize
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

// pfColorsForStepType returns fill and line for each step type of the
// flowchart look: every step sits on the accent's content tint (the tonal
// system's fill of a shape that IS the content) and the preset shape says
// what kind of step it is. A decision is outlined in the accent, a subprocess
// keeps a hairline so its side bars draw, and plain steps and terminators
// have no outline. The steps used to be equal neutral-grey boxes
// (go-slide-creator-6shxx, go-slide-creator-av25u).
func pfColorsForStepType(st processFlowStepType, surfaces ...nativeSurface) (fill pptx.Fill, line pptx.Line) {
	var surface nativeSurface
	if len(surfaces) > 0 {
		surface = surfaces[0]
	}
	fill = surface.content(patterns.TonalLighterContent).fill()
	switch st {
	case pfDecisionType:
		return fill, pptx.Line{Width: int64(patterns.ProcessFlowDecisionLinePt * 12700), Fill: pptx.SchemeFill(surface.accent())}
	case pfSubprocessType:
		return fill, pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill("dk1", pptx.LumMod(50000), pptx.LumOff(50000))}
	default:
		return fill, pptx.Line{Width: 0, Fill: pptx.NoFill()}
	}
}

// pfApplyFlowchartTone resolves a flowchart step's fill, outline and ink
// against the template: the content tint and the ink measured on it.
func pfApplyFlowchartTone(opts *pptx.ShapeOptions, st processFlowStepType, surface nativeSurface) {
	opts.Fill, opts.Line = pfColorsForStepType(st, surface)
	ink := surface.content(patterns.TonalLighterContent).inkFill()
	if opts.Text == nil {
		return
	}
	for p := range opts.Text.Paragraphs {
		for r := range opts.Text.Paragraphs[p].Runs {
			opts.Text.Paragraphs[p].Runs[r].Color = ink
		}
	}
}

// pfConnectorFill is the neutral ink of a connector.
func pfConnectorFill() pptx.Fill {
	return pptx.SchemeFill("dk1", pptx.LumMod(pfConnectorInkPct*1000), pptx.LumOff(100000-pfConnectorInkPct*1000))
}

// pfGenerateConnector produces a connector between two step shapes.
func pfGenerateConnector(connID uint32, src, tgt pptx.ShapeOptions, srcShapeID, tgtShapeID uint32, conn processFlowConnection, direction string) []byte {
	connBounds, startSite, endSite := pptx.RouteBetween(src, tgt)

	// A neutral 1.5pt line: a connector links two steps, it is not the
	// content, so it never takes the accent (go-slide-creator-av25u).
	lineOpts := pptx.Line{
		Width: pfConnectorWidth,
		Fill:  pfConnectorFill(),
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
			W:    patterns.ProcessFlowConnectorHead,
			Len:  patterns.ProcessFlowConnectorHead,
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

// Connection label geometry (EMU).
const (
	pfConnLabelPad       int64 = 27432 // 0.03" text padding inside the knock-out
	pfConnLabelVPad      int64 = 9144  // 0.01" above and below the line box
	pfConnLabelClearance int64 = 45720 // 0.05" between the label and its connector
)

// pfGenerateConnLabel produces a small text shape for a connection label. On
// a horizontal row it sits above the connector, clear of the line and its
// arrowhead, on a background-coloured knock-out; it used to be centred on the
// line, where the 11pt italic "Yes" read as a smudge on the arrowhead
// (go-slide-creator-acydi).
func pfGenerateConnLabel(shapeID uint32, src, tgt pptx.ShapeOptions, label, direction string, fonts ...string) []byte {
	font := defaultFontFamily
	if len(fonts) > 0 && fonts[0] != "" {
		font = fonts[0]
	}
	labelW, labelH := pfConnLabelSize(label, font)
	labelBounds := pfConnLabelBounds(src.Bounds, tgt.Bounds, direction, labelW, labelH, src.Geometry == pptx.GeomFlowChartDecision)

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Conn Label %s", label),
		Bounds:   labelBounds,
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill("bg1"),
		Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "none",
			Anchor: "ctr",
			Insets: [4]int64{pfConnLabelPad, pfConnLabelVPad, pfConnLabelPad, pfConnLabelVPad},
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     label,
					Lang:     "en-US",
					FontSize: pfConnLabelFontSize,
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

// pfConnLabelSize is the knock-out box a connection label needs: its measured
// one-line width plus padding, by one line of the label size.
func pfConnLabelSize(label, font string) (int64, int64) {
	textW, err := textfit.MeasureStyledLineWidth(label, font, float64(pfConnLabelFontSize)/100, false)
	if err != nil || textW <= 0 {
		textW = int64(len([]rune(label))) * pfBodyGlyphWidthEMU
	}
	return textW + textW/10 + 2*pfConnLabelPad, int64(pfConnLabelFontSize)*127*12/10 + 2*pfConnLabelVPad
}

// pfRectHitsStep reports whether rect touches the visible step shape: the
// diamond itself for a decision (its bounding box corners are empty), the
// box for every other step.
func pfRectHitsStep(rect, step pptx.RectEmu, kind processFlowStepType) bool {
	if !nativeRectsOverlap(rect, step) {
		return false
	}
	if kind != pfDecisionType || step.CX <= 0 || step.CY <= 0 {
		return true
	}
	// The rect point closest to the diamond centre in the diamond's own
	// L1 metric is the per-axis clamp of the centre into the rect.
	cx, cy := step.X+step.CX/2, step.Y+step.CY/2
	px := min(max(cx, rect.X), rect.X+rect.CX)
	py := min(max(cy, rect.Y), rect.Y+rect.CY)
	return float64(abs64(px-cx))/float64(step.CX/2)+float64(abs64(py-cy))/float64(step.CY/2) < 1
}

// pfConnLabelBounds places a labelW x labelH connection label. Vertical flows
// put it beside the connector in the gap between the boxes. On a horizontal
// row it sits above the connector line with pfConnLabelClearance to spare,
// centred on the edge-to-edge gap; a label wider than the gap is right-aligned
// to the target's left edge, so it extends back over the source (a decision
// diamond's corner is empty there) instead of over the target's box.
func pfConnLabelBounds(src, tgt pptx.RectEmu, direction string, labelW, labelH int64, srcDiamond bool) pptx.RectEmu {
	const offsetAmt int64 = 91440 // ~0.1"
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
	if abs64(srcCY-tgtCY) > src.CY/2 {
		// A bent connector between rows: keep the label beside its midpoint.
		return pptx.RectEmu{X: midX - labelW/2, Y: midY - labelH - pfConnLabelClearance, CX: labelW, CY: labelH}
	}
	// Straight connector on one row: the line runs at the source's centre.
	lineY := srcCY
	gapL, gapR := src.X+src.CX, tgt.X
	if tgt.X < src.X {
		gapL, gapR = tgt.X+tgt.CX, src.X
	}
	x := (gapL+gapR)/2 - labelW/2
	lift := pfConnLabelClearance
	if overhang := labelW - (gapR - gapL - 2*pfConnLabelPad); overhang > 0 {
		if tgt.X >= src.X {
			x = gapR - pfConnLabelPad - labelW
		} else {
			x = gapL + pfConnLabelPad
		}
		// Over a diamond's empty corner the label clears the sloped edge
		// once its bottom is overhang*cy/cx above the vertex.
		if srcDiamond && src.CX > 0 {
			lift = max(lift, overhang*src.CY/src.CX+pfConnLabelPad)
		}
	}
	return pptx.RectEmu{X: x, Y: lineY - lift - labelH, CX: labelW, CY: labelH}
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
