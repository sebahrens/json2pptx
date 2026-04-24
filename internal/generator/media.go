// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// complexDiagramTypes lists diagram types that are inherently complex and
// become illegible when compressed into narrow (half-width) placeholders.
// These diagrams contain dense text labels, hierarchical structures, or
// grid-based layouts that require full-width space to remain readable.
var complexDiagramTypes = map[string]bool{
	"org_chart":             true,
	"fishbone":              true,
	"swot":                  true,
	"heatmap":               true,
	"business_model_canvas": true,
	"nine_box_talent":       true,
	"gantt":                 true,
	"pestel":                true,
	"kpi_dashboard":         true,
	"process_flow":          true,
	"timeline":              true,
	"venn":                  true,
	"matrix_2x2":            true,
	"porters_five_forces":   true,
	"value_chain":           true,
	"funnel":                true,
}

// narrowPlaceholderThreshold is the maximum placeholder width (in EMUs) below
// which a diagram is considered to be in a narrow/half-width layout.
// This is approximately 50% of a standard 16:9 slide width (12192000 EMU),
// which catches two-column layouts where each column is ≤50% of slide width.
// A 60/40 split (60% column ≈ 6.4M EMU) passes this threshold.
const narrowPlaceholderThreshold int64 = 6096000 // 12192000 * 0.50

// complexityItemThreshold is the minimum number of data items/nodes in a
// diagram before it is considered "complex" for narrow-layout warnings.
// Simple diagrams (e.g., an org chart with 3 nodes) render fine at half-width.
const complexityItemThreshold = 8

// getOptimalRenderDimensions determines the best rendering dimensions for a
// diagram/chart based on placeholder size.
//
// The SVG is rendered at the placeholder's point dimensions so that the SVG
// viewBox matches the PPTX placeholder size exactly. This prevents LibreOffice
// from scaling text incorrectly: LibreOffice scales SVG shapes proportionally
// when fitting to a placeholder but does NOT scale text, causing ~1.8x oversized
// text when the viewBox is larger than the placeholder (e.g., 1600px viewBox in
// an 828pt placeholder). By matching dimensions, no scaling is needed.
//
// PNG quality is maintained via the Scale factor (default 2.0×) which produces
// high-resolution raster output independent of the SVG coordinate space.
//
// If the diagramSpec already has explicit dimensions set, they are preserved.
func getOptimalRenderDimensions(diagramSpec *types.DiagramSpec, placeholderBounds types.BoundingBox) (width, height int) {
	// If explicit dimensions are set, use them
	if diagramSpec.Width > 0 && diagramSpec.Height > 0 {
		return diagramSpec.Width, diagramSpec.Height
	}

	// If no placeholder bounds, return zeros (caller should handle)
	if placeholderBounds.Width <= 0 || placeholderBounds.Height <= 0 {
		return 0, 0
	}

	// Convert EMU placeholder dimensions to points. The SVG builder interprets
	// its width/height as points (1pt = 0.3528mm), so using point dimensions
	// makes the SVG viewBox match the placeholder size exactly.
	const emuPerPoint = int64(types.EMUPerPoint) // 12700
	w := int(placeholderBounds.Width / emuPerPoint)
	h := int(placeholderBounds.Height / emuPerPoint)

	// Clamp to reasonable minimums
	if w < 100 {
		w = 100
	}
	if h < 100 {
		h = 100
	}
	return w, h
}

// prepareImages processes image content and prepares media files.
// This method orchestrates image/chart processing for all slides by delegating
// to focused helper methods for each content type.
func (ctx *singlePassContext) prepareImages() error {
	// Sort slide numbers for deterministic media counter allocation
	imgSlideNums := make([]int, 0, len(ctx.templateSlideData))
	for slideNum := range ctx.templateSlideData {
		imgSlideNums = append(imgSlideNums, slideNum)
	}
	sort.Ints(imgSlideNums)

	for _, slideNum := range imgSlideNums {
		slide := ctx.templateSlideData[slideNum]
		slideSpec, hasSpec := ctx.slideContentMap[slideNum]
		if !hasSpec {
			continue
		}

		resolver := newPlaceholderResolver(slide.CommonSlideData.ShapeTree.Shapes)
		ctx.warnings = append(ctx.warnings, resolver.warnings...)

		for _, item := range slideSpec.Content {
			// Check for visual content types
			switch item.Type {
			case ContentImage, ContentDiagram, ContentTable:
				// Continue processing
			default:
				continue
			}

			shapeIdx, tier, found := resolver.ResolveWithFallback(item.PlaceholderID)
			if !found {
				available := resolver.Keys()
				layoutID := slideSpec.LayoutID
				ctx.warnings = append(ctx.warnings, placeholderNotFoundError(item.PlaceholderID, layoutID, available))
				continue
			}

			// Log non-exact resolutions for observability (matches populateTextInSlide behavior).
			if tier != TierExact {
				resolvedName := slide.CommonSlideData.ShapeTree.Shapes[shapeIdx].NonVisualProperties.ConnectionNonVisual.Name
				logFallbackResolution(item.PlaceholderID, resolvedName, tier, slideSpec.LayoutID)
			}

			shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]

			switch item.Type {
			case ContentDiagram:
				ctx.processDiagramContent(slideNum, item, shape, shapeIdx, resolver)
			case ContentImage:
				ctx.processImageContent(slideNum, item, shape, shapeIdx)
			case ContentTable:
				ctx.processTableContent(slideNum, item, shape, shapeIdx)
			}
		}
	}

	return nil
}

// processTableContent handles standalone table content items.
// It generates the OOXML graphicFrame XML for the table and tracks it
// for placeholder replacement during slide writing.
func (ctx *singlePassContext) processTableContent(slideNum int, item ContentItem, shape *shapeXML, shapeIdx int) {
	tableSpec, ok := item.Value.(*types.TableSpec)
	if !ok {
		reason := fmt.Sprintf("invalid table value for placeholder %s", item.PlaceholderID)
		ctx.warnings = append(ctx.warnings, reason)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "table",
			Reason:        reason,
			Fallback:      "skipped",
		})
		return
	}

	placeholderBounds := getPlaceholderBounds(shape, nil)
	placeholder := types.PlaceholderInfo{
		ID: item.PlaceholderID,
		Bounds: types.BoundingBox{
			X:      placeholderBounds.X,
			Y:      placeholderBounds.Y,
			Width:  placeholderBounds.Width,
			Height: placeholderBounds.Height,
		},
	}

	result, err := PopulateTableInShape(tableSpec, placeholder, nil, nil)
	if err != nil {
		reason := fmt.Sprintf("failed to generate table XML: %v", err)
		ctx.warnings = append(ctx.warnings, reason)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "table",
			Reason:        reason,
			Fallback:      "skipped",
		})
		return
	}

	ctx.tableInserts[slideNum] = append(ctx.tableInserts[slideNum], tableInsert{
		placeholderIdx:  shapeIdx,
		graphicFrameXML: result.XML,
	})
}

// minDiagramWidthEMU is the minimum placeholder width (in EMUs) for a diagram
// to render at a visible size. Placeholders narrower than this produce thumbnail
// charts. 2743200 EMU ≈ 3 inches (roughly 25% of a 16:9 slide width).
const minDiagramWidthEMU int64 = 2743200

// minDiagramHeightEMU is the minimum placeholder height (in EMUs) for a diagram
// to render at a visible size. 1828800 EMU ≈ 2 inches.
const minDiagramHeightEMU int64 = 1828800

// processDiagramContent handles unified diagram content (charts and infographics).
// This is the preferred code path for all visual diagrams.
func (ctx *singlePassContext) processDiagramContent(slideNum int, item ContentItem, shape *shapeXML, shapeIdx int, resolver *placeholderResolver) { //nolint:gocognit,gocyclo
	// Native panel shapes: intercept panel_layout (columns/rows/stat_cards) before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isPanelNativeLayout(diagramSpec) {
		ctx.processPanelNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native SWOT shapes: intercept swot diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isSWOTDiagram(diagramSpec) {
		ctx.processSWOTNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native PESTEL shapes: intercept pestel diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isPESTELDiagram(diagramSpec) {
		ctx.processPESTELNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Nine Box Talent shapes: intercept nine_box_talent diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isNineBoxDiagram(diagramSpec) {
		ctx.processNineBoxNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Value Chain shapes: intercept value_chain diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isValueChainDiagram(diagramSpec) {
		ctx.processValueChainNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native KPI Dashboard shapes: intercept kpi_dashboard diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isKPIDashboardDiagram(diagramSpec) {
		ctx.processKPIDashboardNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Porter's Five Forces shapes: intercept porters_five_forces diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isPortersFiveForcesDiagram(diagramSpec) {
		ctx.processPortersFiveForceNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Business Model Canvas shapes: intercept business_model_canvas diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isBMCDiagram(diagramSpec) {
		ctx.processBMCNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Process Flow shapes: intercept process_flow diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isProcessFlowDiagram(diagramSpec) {
		ctx.processProcessFlowNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Heatmap shapes: intercept heatmap diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isHeatmapDiagram(diagramSpec) {
		ctx.processHeatmapNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native Pyramid shapes: intercept pyramid diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isPyramidDiagram(diagramSpec) {
		ctx.processPyramidNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Native House Diagram shapes: intercept house_diagram diagrams before SVG rendering.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok && isHouseDiagram(diagramSpec) {
		ctx.processHouseDiagramNativeShapes(slideNum, item, shapeIdx)
		return
	}

	// Check if body text also targets the same placeholder. When both text and
	// a diagram share a single placeholder, we split the space: body text occupies
	// the top portion (~25%) and the diagram occupies the remaining space below.
	// Without this, the diagram image replaces the text shape entirely.
	textCollision := ctx.hasTextCollisionForShape(slideNum, shapeIdx, resolver)

	// Get placeholder bounds BEFORE rendering so we can pass dimensions to the renderer
	placeholderBounds := getPlaceholderBounds(shape, nil)

	// Guard against tiny placeholder bounds that produce thumbnail-sized charts.
	// In two-column layouts, shape transforms can inherit incorrect dimensions
	// from the slide master, yielding near-zero cx/cy values.
	if placeholderBounds.Width > 0 && placeholderBounds.Width < minDiagramWidthEMU {
		slog.Warn("diagram placeholder width below minimum, clamping",
			slog.Int("slide_num", slideNum),
			slog.Int64("width_emu", placeholderBounds.Width),
			slog.Int64("min_emu", minDiagramWidthEMU))
		placeholderBounds.Width = minDiagramWidthEMU
	}
	if placeholderBounds.Height > 0 && placeholderBounds.Height < minDiagramHeightEMU {
		slog.Warn("diagram placeholder height below minimum, clamping",
			slog.Int("slide_num", slideNum),
			slog.Int64("height_emu", placeholderBounds.Height),
			slog.Int64("min_emu", minDiagramHeightEMU))
		placeholderBounds.Height = minDiagramHeightEMU
	}

	// When text collides, use adjusted bounds for rendering (lower 75% of the placeholder)
	diagramBounds := placeholderBounds
	if textCollision {
		textHeight := placeholderBounds.Height / 4 // 25% for text
		diagramBounds.Y = placeholderBounds.Y + textHeight
		diagramBounds.Height = placeholderBounds.Height - textHeight
	}

	renderResult, ok := ctx.resolveDiagramWithMetadata(slideNum, item, diagramBounds)
	if !ok {
		// Insert a styled placeholder instead of leaving a blank area.
		diagramType := ""
		if ds, ok := item.Value.(*types.DiagramSpec); ok {
			diagramType = ds.Type
		}
		ctx.insertDiagramPlaceholder(slideNum, diagramBounds, shapeIdx, diagramType)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "diagram",
			DiagramType:   diagramType,
			Reason:        ctx.lastDiagramWarning(item.PlaceholderID),
			Fallback:      "placeholder_image",
		})
		return
	}

	// Calculate embedding dimensions based on fit mode
	// For "contain" mode (all diagrams), preserve content aspect ratio and center within placeholder
	// For "stretch" mode (explicit override only), use full placeholder dimensions
	embedX := diagramBounds.X
	embedY := diagramBounds.Y
	embedW := diagramBounds.Width
	embedH := diagramBounds.Height

	if renderResult.FitMode == "contain" && renderResult.ContentWidth > 0 && renderResult.ContentHeight > 0 {
		// For "contain" mode, fit the content's aspect ratio within the placeholder
		// bounds (in EMUs). The SVG content dimensions define the aspect ratio but
		// must NOT be converted to EMUs directly — doing so produces an image larger
		// than the placeholder because the SVG coordinate system doesn't correspond
		// to physical EMU units.
		contentRatio := renderResult.ContentWidth / renderResult.ContentHeight
		phW := float64(diagramBounds.Width)
		phH := float64(diagramBounds.Height)
		phRatio := phW / phH

		var fitW, fitH float64
		if contentRatio > phRatio {
			// Width-constrained
			fitW = phW
			fitH = phW / contentRatio
		} else {
			// Height-constrained
			fitH = phH
			fitW = phH * contentRatio
		}

		// Center within the diagram area (not the full placeholder)
		offsetXEMU := int64((phW - fitW) / 2)
		offsetYEMU := int64((phH - fitH) / 2)

		embedX = diagramBounds.X + offsetXEMU
		embedY = diagramBounds.Y + offsetYEMU
		embedW = int64(fitW)
		embedH = int64(fitH)
	}

	// Quality check: warn when complex diagrams are placed in narrow placeholders
	// (e.g., two-column layouts where each column is ~50% of slide width).
	// This does not block rendering — it only adds a warning.
	if diagramSpec, ok := item.Value.(*types.DiagramSpec); ok {
		ctx.checkDiagramInNarrowPlaceholder(slideNum, diagramSpec, embedW, item.PlaceholderID)
	}

	// When text shares this placeholder, keep the text shape (don't remove it)
	// and shrink it to make room for the diagram below.
	removeIdx := shapeIdx
	if textCollision {
		removeIdx = -1 // -1 sentinel: don't remove any shape
		ctx.shrinkShapeForTextAboveDiagram(shape, placeholderBounds)
		slog.Info("text+diagram collision resolved: body text kept above diagram",
			slog.Int("slide_num", slideNum),
			slog.String("placeholder_id", item.PlaceholderID))
	}

	if ctx.svgConverter.GetStrategy() == SVGStrategyNative && len(renderResult.SVG) > 0 {
		// Native SVG embedding: embed SVG + 1x1 transparent PNG stub (asvg:svgBlip).
		// PowerPoint 2016+ renders the crisp vector SVG; the PNG is a spec-required
		// placeholder that is never displayed. Using a constant 67-byte PNG avoids
		// expensive per-diagram rasterization.
		svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(fmt.Sprintf("diagram-s%d-x%d", slideNum, removeIdx))

		// Use the rendered PNG if available (e.g., when PNG strategy was used
		// for this diagram), otherwise use the 1x1 transparent constant.
		pngFallback := renderResult.PNG
		if len(pngFallback) == 0 {
			pngFallback = transparentPNG1x1
		}

		ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
			svgData:        renderResult.SVG,
			pngData:        pngFallback,
			svgMediaFile:   svgMediaFile,
			pngMediaFile:   pngMediaFile,
			offsetX:        embedX,
			offsetY:        embedY,
			extentCX:       embedW,
			extentCY:       embedH,
			placeholderIdx: removeIdx,
		})
	} else {
		// PNG-only embedding (for LibreOffice compatibility or when SVG unavailable).
		// The rasterizer (tdewolff/canvas) correctly scales text and shapes
		// identically. PNG is rendered at 2x from placeholder-sized SVG.
		mediaFileName := ctx.allocPNG(fmt.Sprintf("diagram-s%d-x%d", slideNum, removeIdx))

		ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
			mediaFileName:  mediaFileName,
			data:           renderResult.PNG,
			offsetX:        embedX,
			offsetY:        embedY,
			extentCX:       embedW,
			extentCY:       embedH,
			placeholderIdx: removeIdx,
		})
	}
}

// hasTextCollisionForShape checks if any text content item resolves to the
// same physical shape as the diagram. This handles the case where body text
// and a diagram target the same shape via different ID forms (name vs idx:N).
func (ctx *singlePassContext) hasTextCollisionForShape(slideNum int, diagramShapeIdx int, resolver *placeholderResolver) bool {
	slideSpec, ok := ctx.slideContentMap[slideNum]
	if !ok {
		return false
	}
	for _, ci := range slideSpec.Content {
		if ci.Type == ContentText {
			if textShapeIdx, _, found := resolver.ResolveWithFallback(ci.PlaceholderID); found && textShapeIdx == diagramShapeIdx {
				return true
			}
		}
	}
	return false
}

// shrinkShapeForTextAboveDiagram modifies the shape's transform so it only
// occupies the top 25% of the placeholder space, leaving room for the diagram below.
func (ctx *singlePassContext) shrinkShapeForTextAboveDiagram(shape *shapeXML, fullBounds types.BoundingBox) {
	if shape.ShapeProperties.Transform == nil {
		return
	}
	textHeight := fullBounds.Height / 4
	shape.ShapeProperties.Transform.Extent.CY = textHeight
}

// checkDiagramInNarrowPlaceholder emits a quality warning when a complex diagram
// is placed in a narrow (half-width) placeholder such as a two-column layout.
// Complex diagrams compressed to ~50% width become illegible because their text
// labels and structural elements are too small to read.
//
// The check is non-blocking: it adds a warning to ctx.warnings and logs via slog.Warn
// but does not prevent the diagram from being rendered. This allows users to override
// the layout choice while still being informed of the quality issue.
func (ctx *singlePassContext) checkDiagramInNarrowPlaceholder(slideNum int, diagramSpec *types.DiagramSpec, embedW int64, placeholderID string) {
	if embedW > narrowPlaceholderThreshold {
		return
	}
	if !complexDiagramTypes[diagramSpec.Type] {
		return
	}

	complexity := estimateDiagramComplexity(diagramSpec)
	if complexity < complexityItemThreshold {
		return
	}

	// Standard widescreen slide width in EMU (13.33 inches)
	const slideWidthEMU = 12192000
	widthPct := float64(embedW) / float64(slideWidthEMU) * 100

	warning := fmt.Sprintf(
		"slide %d: complex %s diagram (%d items) in narrow placeholder %q (width %.0f%% of slide) may be illegible — consider using a full-width layout",
		slideNum,
		diagramSpec.Type,
		complexity,
		placeholderID,
		widthPct,
	)

	ctx.warnings = append(ctx.warnings, warning)
	slog.Warn("complex diagram in narrow placeholder",
		slog.Int("slide_num", slideNum),
		slog.String("diagram_type", diagramSpec.Type),
		slog.Int("complexity_items", complexity),
		slog.Int64("embed_width_emu", embedW),
		slog.String("placeholder_id", placeholderID),
	)
}

// estimateDiagramComplexity estimates the number of visual elements in a diagram
// based on its type and data payload. This is used to detect diagrams that would
// be illegible when compressed into narrow placeholders.
//
// The function inspects DiagramSpec.Data (a map[string]any) and counts elements
// appropriate to each diagram type:
//   - org_chart: total nodes in the tree (root + all descendants)
//   - fishbone: total causes across all categories
//   - swot: total items across all four quadrants
//   - heatmap: rows x columns
//   - business_model_canvas, nine_box_talent, pestel: total items across all sections
//   - gantt: number of tasks
//   - kpi_dashboard: number of KPI cards
//   - fallback: counts top-level array/map entries in data
func estimateDiagramComplexity(spec *types.DiagramSpec) int {
	if spec == nil || len(spec.Data) == 0 {
		return 0
	}

	switch spec.Type {
	case "org_chart":
		return countOrgChartNodes(spec.Data)
	case "fishbone":
		return countFishboneCauses(spec.Data)
	case "swot":
		return countSWOTItems(spec.Data)
	case "heatmap":
		return countHeatmapCells(spec.Data)
	case "business_model_canvas", "nine_box_talent", "pestel":
		return countSectionItems(spec.Data)
	case "gantt":
		return countArrayItems(spec.Data, "tasks")
	case "kpi_dashboard":
		return countArrayItems(spec.Data, "cards")
	default:
		return countGenericItems(spec.Data)
	}
}

// countOrgChartNodes counts the total number of nodes in an org chart tree.
func countOrgChartNodes(data map[string]any) int {
	rootRaw, ok := data["root"]
	if !ok {
		return 0
	}
	rootMap, ok := rootRaw.(map[string]any)
	if !ok {
		return 0
	}
	return countOrgNodeRecursive(rootMap)
}

// countOrgNodeRecursive recursively counts a node and all its children.
func countOrgNodeRecursive(node map[string]any) int {
	count := 1 // count this node
	childrenRaw, ok := node["children"]
	if !ok {
		return count
	}
	children, ok := childrenRaw.([]any)
	if !ok {
		return count
	}
	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			count++ // count non-map children as a node
			continue
		}
		count += countOrgNodeRecursive(childMap)
	}
	return count
}

// countFishboneCauses counts total causes across all fishbone categories.
func countFishboneCauses(data map[string]any) int {
	categoriesRaw, ok := data["categories"]
	if !ok {
		return 0
	}
	categories, ok := categoriesRaw.([]any)
	if !ok {
		return 0
	}
	total := 0
	for _, catRaw := range categories {
		catMap, ok := catRaw.(map[string]any)
		if !ok {
			continue
		}
		causesRaw, ok := catMap["causes"]
		if !ok {
			total++ // count the category itself
			continue
		}
		causes, ok := causesRaw.([]any)
		if !ok {
			total++
			continue
		}
		total += len(causes)
	}
	return total
}

// countSWOTItems counts total items across all four SWOT quadrants.
func countSWOTItems(data map[string]any) int {
	total := 0
	for _, key := range []string{"strengths", "weaknesses", "opportunities", "threats"} {
		quadRaw, ok := data[key]
		if !ok {
			continue
		}
		switch q := quadRaw.(type) {
		case map[string]any:
			// SWOT quadrants can be {items: [...]}
			if items, ok := q["items"]; ok {
				if arr, ok := items.([]any); ok {
					total += len(arr)
				}
			}
		case []any:
			total += len(q)
		}
	}
	return total
}

// countHeatmapCells counts rows x columns in a heatmap.
func countHeatmapCells(data map[string]any) int {
	// Count by row_labels x col_labels, or by values matrix dimensions
	rows := countStringArray(data, "row_labels")
	if rows == 0 {
		rows = countStringArray(data, "y_labels")
	}
	cols := countStringArray(data, "col_labels")
	if cols == 0 {
		cols = countStringArray(data, "x_labels")
	}
	if rows > 0 && cols > 0 {
		return rows * cols
	}

	// Fallback: count from values matrix
	valuesRaw, ok := data["values"]
	if !ok {
		return 0
	}
	valuesArr, ok := valuesRaw.([]any)
	if !ok {
		return 0
	}
	if len(valuesArr) == 0 {
		return 0
	}
	// Estimate: rows * cols from first row length
	firstRow, ok := valuesArr[0].([]any)
	if !ok {
		return len(valuesArr)
	}
	return len(valuesArr) * len(firstRow)
}

// countSectionItems counts total items across all sections in section-based diagrams
// (business_model_canvas, nine_box_talent, pestel). These diagrams typically have
// a "sections" or similar top-level key containing arrays of items.
func countSectionItems(data map[string]any) int {
	total := 0
	for _, v := range data {
		switch val := v.(type) {
		case []any:
			// Each array element might contain items
			for _, elem := range val {
				if elemMap, ok := elem.(map[string]any); ok {
					if items, ok := elemMap["items"]; ok {
						if arr, ok := items.([]any); ok {
							total += len(arr)
							continue
						}
					}
				}
				total++ // count the element itself
			}
		case map[string]any:
			// Nested sections
			if items, ok := val["items"]; ok {
				if arr, ok := items.([]any); ok {
					total += len(arr)
				}
			}
		}
	}
	return total
}

// countArrayItems counts elements in a named array within data.
func countArrayItems(data map[string]any, key string) int {
	raw, ok := data[key]
	if !ok {
		return 0
	}
	arr, ok := raw.([]any)
	if !ok {
		return 0
	}
	return len(arr)
}

// countStringArray counts elements in a named string array within data.
func countStringArray(data map[string]any, key string) int {
	raw, ok := data[key]
	if !ok {
		return 0
	}
	arr, ok := raw.([]any)
	if !ok {
		return 0
	}
	return len(arr)
}

// countGenericItems provides a rough complexity estimate for unknown diagram types
// by counting top-level data entries and array elements.
func countGenericItems(data map[string]any) int {
	total := 0
	for _, v := range data {
		switch val := v.(type) {
		case []any:
			total += len(val)
		case map[string]any:
			total += len(val)
		default:
			total++
		}
	}
	return total
}

// resolveDiagramBytes renders a DiagramSpec via svggen.
// The placeholderBounds parameter provides the placeholder dimensions for sizing the output.
// Returns the PNG bytes and true if successful, or false with a warning added on failure.
func (ctx *singlePassContext) resolveDiagramBytes(slideNum int, item ContentItem, placeholderBounds types.BoundingBox) ([]byte, bool) {
	result, ok := ctx.resolveDiagramWithMetadata(slideNum, item, placeholderBounds)
	if !ok {
		return nil, false
	}
	return result.PNG, true
}

// resolveDiagramWithMetadata renders a DiagramSpec via svggen and returns full metadata.
// The placeholderBounds parameter provides the placeholder dimensions for sizing the output.
// Returns the render result (PNG bytes + fit metadata) and true if successful,
// or false with a warning added on failure.
func (ctx *singlePassContext) resolveDiagramWithMetadata(slideNum int, item ContentItem, placeholderBounds types.BoundingBox) (*DiagramRenderResult, bool) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("invalid diagram value for placeholder %s: expected *types.DiagramSpec", item.PlaceholderID))
		return nil, false
	}

	// Guard: skip render if diagram data is empty to avoid blank chart frames
	// (axes/grids with no data points).
	if len(diagramSpec.Data) == 0 {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("empty diagram data for placeholder %s (type %s), skipping render", item.PlaceholderID, diagramSpec.Type))
		return nil, false
	}

	// Inject theme colors if diagram doesn't have explicit Colors set
	if diagramSpec.Style == nil {
		diagramSpec.Style = &types.DiagramStyle{}
	}
	if len(diagramSpec.Style.Colors) == 0 && len(ctx.themeColors) > 0 {
		diagramSpec.Style.ThemeColors = ctx.themeColors
	}

	// Use placeholder-aware dimensions if diagram doesn't have explicit dimensions set.
	if diagramSpec.Width == 0 || diagramSpec.Height == 0 {
		diagramSpec.Width, diagramSpec.Height = getOptimalRenderDimensions(diagramSpec, placeholderBounds)
	}

	maxPNGWidth := 0
	if ctx.svgConverter != nil {
		maxPNGWidth = ctx.svgConverter.MaxPNGWidth
	}
	// When using native SVG strategy, skip PNG rasterization entirely.
	// The OOXML blip fallback uses a 1x1 transparent PNG constant instead.
	svgOnly := ctx.svgConverter != nil && ctx.svgConverter.GetStrategy() == SVGStrategyNative
	rendered, err := renderDiagramSpecFull(diagramSpec, ctx.themeColors, maxPNGWidth, svgOnly, ctx.strictFit)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to render diagram for placeholder %s: %v", item.PlaceholderID, err))
		return nil, false
	}
	return rendered, true
}

// processImageContent handles image content items (file-based images).
func (ctx *singlePassContext) processImageContent(slideNum int, item ContentItem, shape *shapeXML, shapeIdx int) {
	imgContent, ok := item.Value.(ImageContent)
	if !ok {
		reason := fmt.Sprintf("invalid image value for placeholder %s", item.PlaceholderID)
		ctx.warnings = append(ctx.warnings, reason)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "image",
			Reason:        reason,
			Fallback:      "skipped",
		})
		return
	}

	imagePath := imgContent.Path
	bounds := imgContent.Bounds

	// Validate path using config-injected allowed paths
	if err := ValidateImagePathWithConfig(imagePath, ctx.allowedImagePaths); err != nil {
		reason := fmt.Sprintf("security: image path validation failed for %s: %v", item.PlaceholderID, err)
		ctx.warnings = append(ctx.warnings, reason)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "image",
			Reason:        reason,
			Fallback:      "skipped",
		})
		return
	}

	// Check if image exists
	if _, err := os.Stat(imagePath); err != nil {
		reason := fmt.Sprintf("image file not found: %s", imagePath)
		ctx.warnings = append(ctx.warnings, reason)
		ctx.mediaFailures = append(ctx.mediaFailures, MediaFailure{
			SlideNum:      slideNum,
			PlaceholderID: item.PlaceholderID,
			ContentType:   "image",
			Reason:        reason,
			Fallback:      "skipped",
		})
		return
	}

	// Get placeholder bounds
	placeholderBounds := getPlaceholderBounds(shape, bounds)

	// Handle SVG files with appropriate strategy
	if IsSVGFile(imagePath) {
		ctx.processSVGImage(slideNum, imagePath, placeholderBounds, shape, shapeIdx)
		return
	}

	// Process regular (non-SVG) image
	ctx.processRegularImage(slideNum, imagePath, placeholderBounds, shape, shapeIdx)
}

// processSVGImage handles SVG files based on the configured conversion strategy.
func (ctx *singlePassContext) processSVGImage(slideNum int, imagePath string, placeholderBounds types.BoundingBox, shape *shapeXML, shapeIdx int) {
	if ctx.svgConverter.GetStrategy() == SVGStrategyNative {
		ctx.processNativeSVG(slideNum, imagePath, placeholderBounds, shapeIdx)
		return
	}

	// PNG/EMF conversion strategy
	convertedPath, ok := ctx.convertSVGToRaster(imagePath, placeholderBounds, slideNum, shapeIdx)
	if !ok {
		// Fallback was already inserted by convertSVGToRaster if needed
		return
	}

	// Process as regular image with converted path
	ctx.processRegularImage(slideNum, convertedPath, placeholderBounds, shape, shapeIdx)
}

// processNativeSVG handles native SVG embedding with PNG fallback.
func (ctx *singlePassContext) processNativeSVG(slideNum int, imagePath string, placeholderBounds types.BoundingBox, shapeIdx int) {
	if !ctx.svgConverter.IsPNGAvailable() {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("SVG file %s: rsvg-convert not available, using placeholder", imagePath))
		ctx.insertSVGFallbackImage(slideNum, placeholderBounds, shapeIdx)
		return
	}

	// Generate PNG fallback
	pngPath, cleanup, err := ctx.svgConverter.ConvertToPNG(ctx.ctx, imagePath, 0)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("SVG PNG fallback generation failed for %s: %v, using placeholder", imagePath, err))
		ctx.insertSVGFallbackImage(slideNum, placeholderBounds, shapeIdx)
		return
	}
	ctx.svgCleanupFuncs = append(ctx.svgCleanupFuncs, cleanup)

	// Scale using PNG dimensions
	scaledBounds, err := scaleImageToFit(pngPath, placeholderBounds)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to scale image %s: %v", imagePath, err))
		return
	}

	// Allocate media filenames
	svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(fmt.Sprintf("nativesvg-s%d-x%d", slideNum, shapeIdx))

	// Track as native SVG insert
	ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
		svgPath:        imagePath,
		pngPath:        pngPath,
		svgMediaFile:   svgMediaFile,
		pngMediaFile:   pngMediaFile,
		offsetX:        scaledBounds.X,
		offsetY:        scaledBounds.Y,
		extentCX:       scaledBounds.Width,
		extentCY:       scaledBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// convertSVGToRaster converts an SVG to PNG/EMF format.
// Returns the converted path and true if successful. On failure, inserts a fallback
// placeholder image and returns false.
func (ctx *singlePassContext) convertSVGToRaster(imagePath string, placeholderBounds types.BoundingBox, slideNum int, shapeIdx int) (string, bool) {
	if !ctx.svgConverter.IsAvailable() {
		toolName := "rsvg-convert"
		if ctx.svgConverter.GetStrategy() == SVGStrategyEMF {
			toolName = "inkscape"
		}
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("SVG file %s: %s not available for %s conversion, using placeholder", imagePath, toolName, ctx.svgConverter.GetStrategy()))
		ctx.insertSVGFallbackImage(slideNum, placeholderBounds, shapeIdx)
		return "", false
	}

	convertedPath, cleanup, err := ctx.svgConverter.Convert(ctx.ctx, imagePath)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("SVG conversion failed for %s: %v, using placeholder", imagePath, err))
		ctx.insertSVGFallbackImage(slideNum, placeholderBounds, shapeIdx)
		return "", false
	}
	ctx.svgCleanupFuncs = append(ctx.svgCleanupFuncs, cleanup)

	return convertedPath, true
}

// processRegularImage handles non-SVG images (PNG, JPG, etc.) or converted SVGs.
func (ctx *singlePassContext) processRegularImage(slideNum int, imagePath string, placeholderBounds types.BoundingBox, shape *shapeXML, shapeIdx int) {
	scaledBounds, err := scaleImageToFit(imagePath, placeholderBounds)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to scale image %s: %v", imagePath, err))
		return
	}

	// Allocate media slot if not already present
	mediaFileName := ctx.allocateMediaSlot(imagePath)

	// Track relationship update with position for p:pic insertion
	// The placeholder shape will be removed and replaced with a p:pic element
	ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
		imagePath:      imagePath,
		mediaFileName:  mediaFileName,
		offsetX:        scaledBounds.X,
		offsetY:        scaledBounds.Y,
		extentCX:       scaledBounds.Width,
		extentCY:       scaledBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// allocateMediaSlot registers an image file and returns its media filename.
// If the image is already registered, returns the existing filename.
func (ctx *singlePassContext) allocateMediaSlot(imagePath string) string {
	return ctx.allocPNGForFile(imagePath)
}

// insertSVGFallbackImage inserts a placeholder image indicating SVG rendering is unavailable.
// This is used when SVG conversion tools are not available or conversion fails.
//
// When native SVG strategy is active, generates a minimal SVG placeholder instead
// of a rasterized PNG, keeping the output PNG-free.
func (ctx *singlePassContext) insertSVGFallbackImage(slideNum int, placeholderBounds types.BoundingBox, shapeIdx int) {
	if ctx.svgConverter != nil && ctx.svgConverter.GetStrategy() == SVGStrategyNative {
		ctx.insertSVGFallbackImageSVG(slideNum, placeholderBounds, shapeIdx)
		return
	}

	// Calculate image dimensions in pixels from EMUs
	// EMU = English Metric Unit, 914400 EMUs = 1 inch, at 96 DPI: 1 pixel = 9525 EMUs
	const emuPerPixel = int64(types.EMUPerPixel)
	width := int(placeholderBounds.Width / emuPerPixel)
	height := int(placeholderBounds.Height / emuPerPixel)

	// Generate fallback image
	imgBytes, err := SVGFallbackImage(width, height)
	if err != nil {
		// If we can't even generate a fallback, just log and skip
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to generate SVG fallback image: %v", err))
		return
	}

	mediaFileName := ctx.allocPNG(fmt.Sprintf("svgfallback-s%d-x%d", slideNum, shapeIdx))

	// Track relationship with byte data for p:pic insertion
	ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
		mediaFileName:  mediaFileName,
		data:           imgBytes,
		offsetX:        placeholderBounds.X,
		offsetY:        placeholderBounds.Y,
		extentCX:       placeholderBounds.Width,
		extentCY:       placeholderBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// insertSVGFallbackImageSVG inserts a minimal SVG placeholder when SVG conversion
// tools are unavailable and native SVG strategy is active.
func (ctx *singlePassContext) insertSVGFallbackImageSVG(slideNum int, placeholderBounds types.BoundingBox, shapeIdx int) {
	const emuPerPoint = int64(types.EMUPerPoint)
	w := int(placeholderBounds.Width / emuPerPoint)
	h := int(placeholderBounds.Height / emuPerPoint)
	if w < 100 {
		w = 100
	}
	if h < 50 {
		h = 50
	}

	svgData := []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<rect width="100%%" height="100%%" fill="#F0F0F0" stroke="#C8C8C8" stroke-width="1"/>`+
			`<text x="50%%" y="45%%" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#808080">SVG Unavailable</text>`+
			`<text x="50%%" y="55%%" text-anchor="middle" font-family="sans-serif" font-size="12" fill="#808080">Install rsvg-convert</text>`+
			`</svg>`,
		w, h, w, h,
	))

	svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(fmt.Sprintf("svgfallbacksvg-s%d-x%d", slideNum, shapeIdx))

	ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
		svgData:        svgData,
		pngData:        transparentPNG1x1,
		svgMediaFile:   svgMediaFile,
		pngMediaFile:   pngMediaFile,
		offsetX:        placeholderBounds.X,
		offsetY:        placeholderBounds.Y,
		extentCX:       placeholderBounds.Width,
		extentCY:       placeholderBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// insertDiagramPlaceholder inserts a styled placeholder image when a chart/diagram
// fails validation or rendering. Instead of leaving a blank area, this shows the
// diagram type name and "Data unavailable" in a professional placeholder.
//
// When native SVG strategy is active, generates a minimal SVG placeholder instead
// of a rasterized PNG, keeping the output PNG-free.
func (ctx *singlePassContext) insertDiagramPlaceholder(slideNum int, placeholderBounds types.BoundingBox, shapeIdx int, diagramType string) {
	if ctx.svgConverter != nil && ctx.svgConverter.GetStrategy() == SVGStrategyNative {
		ctx.insertDiagramPlaceholderSVG(slideNum, placeholderBounds, shapeIdx, diagramType)
		return
	}

	const emuPerPixel = int64(types.EMUPerPixel)
	width := int(placeholderBounds.Width / emuPerPixel)
	height := int(placeholderBounds.Height / emuPerPixel)

	imgBytes, err := DiagramPlaceholderImage(width, height, diagramType)
	if err != nil {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to generate diagram placeholder image: %v", err))
		return
	}

	mediaFileName := ctx.allocPNG(fmt.Sprintf("diagramph-s%d-x%d", slideNum, shapeIdx))

	ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
		mediaFileName:  mediaFileName,
		data:           imgBytes,
		offsetX:        placeholderBounds.X,
		offsetY:        placeholderBounds.Y,
		extentCX:       placeholderBounds.Width,
		extentCY:       placeholderBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// insertDiagramPlaceholderSVG inserts a minimal SVG placeholder for failed diagrams
// when native SVG strategy is active. The SVG shows the diagram type and
// "Data unavailable" text. Uses the 1x1 transparent PNG as the OOXML blip fallback.
func (ctx *singlePassContext) insertDiagramPlaceholderSVG(slideNum int, placeholderBounds types.BoundingBox, shapeIdx int, diagramType string) {
	const emuPerPoint = int64(types.EMUPerPoint)
	w := int(placeholderBounds.Width / emuPerPoint)
	h := int(placeholderBounds.Height / emuPerPoint)
	if w < 100 {
		w = 100
	}
	if h < 50 {
		h = 50
	}

	label := formatDiagramType(diagramType)
	svgData := []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<rect width="100%%" height="100%%" fill="#F5F3F0" stroke="#D2CDC8" stroke-width="1"/>`+
			`<text x="50%%" y="45%%" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#8C8782">%s</text>`+
			`<text x="50%%" y="55%%" text-anchor="middle" font-family="sans-serif" font-size="12" fill="#8C8782">Data unavailable</text>`+
			`</svg>`,
		w, h, w, h, label,
	))

	svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(fmt.Sprintf("diagramphsvg-s%d-x%d", slideNum, shapeIdx))

	ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
		svgData:        svgData,
		pngData:        transparentPNG1x1,
		svgMediaFile:   svgMediaFile,
		pngMediaFile:   pngMediaFile,
		offsetX:        placeholderBounds.X,
		offsetY:        placeholderBounds.Y,
		extentCX:       placeholderBounds.Width,
		extentCY:       placeholderBounds.Height,
		placeholderIdx: shapeIdx,
	})
}

// lastDiagramWarning returns the most recent warning that mentions the given placeholder ID.
// Used to populate MediaFailure.Reason with the specific error message.
func (ctx *singlePassContext) lastDiagramWarning(placeholderID string) string {
	for i := len(ctx.warnings) - 1; i >= 0; i-- {
		if strings.Contains(ctx.warnings[i], placeholderID) {
			return ctx.warnings[i]
		}
	}
	return "diagram rendering failed"
}
