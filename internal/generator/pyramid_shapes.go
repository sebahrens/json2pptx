package generator

import (
	"fmt"
	"log/slog"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Pyramid Native Shapes — Stacked Trapezoids with Progressive Widths
// =============================================================================
//
// Replaces SVG-rendered pyramid diagrams with native OOXML grouped shapes.
// Each level is a trapezoid preset shape with a computed adj value that
// controls the top-edge inset. Levels stack from top (narrowest) to bottom
// (widest), each filled with a scheme accent color at varying lumMod/lumOff
// to create a gradient from dark (apex) to light (base). Text is centered
// inside each trapezoid. All shapes wrapped in a single p:grpSp.
//
// Layout:
//
//              ┌──────┐           ← Level 0 (narrowest, darkest)
//            ┌──────────┐         ← Level 1
//          ┌──────────────┐       ← Level 2
//        ┌──────────────────┐     ← Level 3
//      ┌──────────────────────┐   ← Level 4 (widest, lightest)
//
// Color strategy: accent1 with lumMod interpolated from full saturation
// (apex) to light tint (base). This matches the SVG renderer's dark-to-light
// gradient but uses scheme colors for theme awareness.

// Pyramid EMU constants.
const (
	// pyramidGapEMU is the gap between levels in EMU.
	// ~0.03" = 27432 EMU — tight gap for compact stacking.
	pyramidGapEMU int64 = 27432

	// pyramidLabelFontSize is the level label font size (hundredths of a point).
	// 1100 = 11pt
	pyramidLabelFontSize int = 1100

	// pyramidLabelFontSizeSmall is for pyramids with many levels (8+).
	// 900 = 9pt
	pyramidLabelFontSizeSmall int = 900

	// pyramidDescFontSize is the description font size (hundredths of a point).
	// 900 = 9pt
	pyramidDescFontSize int = 900

	// pyramidDescFontSizeSmall is for pyramids with many levels (8+).
	// 700 = 7pt
	pyramidDescFontSizeSmall int = 700

	// pyramidTextInset is the text inset for level shapes (EMU). ~0.05"
	pyramidTextInset int64 = 45720

	// pyramidTopWidthRatio is the width ratio for the top (apex) level.
	// 0.15 = 15% of full width, matching the SVG default.
	pyramidTopWidthRatio float64 = 0.15

	// pyramidMaxLevels is the maximum number of levels supported.
	pyramidMaxLevels int = 20
)

// pyramidLevel holds parsed data for a single pyramid level.
type pyramidLevel struct {
	label       string
	description string
}

// isPyramidDiagram returns true if the diagram spec is a pyramid type.
func isPyramidDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "pyramid"
}

// processPyramidNativeShapes parses pyramid data from a DiagramSpec and
// registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processPyramidNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("pyramid native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("pyramid native shapes: themeOverride is set but scheme color refs will not reflect overrides",
			"slide", slideNum)
	}

	levels, err := parsePyramidDiagramData(diagramSpec.Data)
	if err != nil {
		slog.Warn("pyramid native shapes: parse failed", "slide", slideNum, "error", err)
		return
	}

	if len(levels) == 0 {
		slog.Warn("pyramid native shapes: no levels parsed", "slide", slideNum)
		return
	}

	// Encode levels into panels for the panelShapeInsert system.
	var panels []nativePanelData
	for _, l := range levels {
		panels = append(panels, nativePanelData{
			title: l.label,
			body:  l.description,
		})
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native pyramid shapes: registered",
		"slide", slideNum,
		"levels", len(levels),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		pyramidMode:    true,
	})
}

// parsePyramidDiagramData extracts pyramid levels from the diagram data map.
func parsePyramidDiagramData(data map[string]any) ([]pyramidLevel, error) {
	levelsRaw, ok := data["levels"]
	if !ok {
		return nil, fmt.Errorf("pyramid requires 'levels' array")
	}

	levelsSlice, ok := levelsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("pyramid 'levels' must be an array")
	}

	if len(levelsSlice) == 0 {
		return nil, fmt.Errorf("pyramid requires at least one level")
	}

	if len(levelsSlice) > pyramidMaxLevels {
		return nil, fmt.Errorf("pyramid supports at most %d levels, got %d", pyramidMaxLevels, len(levelsSlice))
	}

	var levels []pyramidLevel
	for _, lRaw := range levelsSlice {
		switch l := lRaw.(type) {
		case string:
			levels = append(levels, pyramidLevel{label: l})
		case map[string]any:
			level := pyramidLevel{}
			if label, ok := l["label"].(string); ok {
				level.label = label
			}
			if desc, ok := l["description"].(string); ok {
				level.description = desc
			}
			levels = append(levels, level)
		default:
			return nil, fmt.Errorf("invalid pyramid level format")
		}
	}

	return levels, nil
}

// =============================================================================
// Group XML Generation
// =============================================================================

// generatePyramidGroupXML produces the complete <p:grpSp> XML for a pyramid diagram.
func generatePyramidGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	numLevels := len(panels)
	if numLevels == 0 {
		return ""
	}

	// Calculate level dimensions.
	totalGaps := int64(numLevels-1) * pyramidGapEMU
	levelHeight := (bounds.Height - totalGaps) / int64(numLevels)
	if levelHeight < 0 {
		levelHeight = bounds.Height / int64(numLevels)
	}

	centerX := bounds.X + bounds.Width/2

	// Choose font sizes based on level count.
	labelFontSize := pyramidLabelFontSize
	descFontSize := pyramidDescFontSize
	if numLevels >= 8 {
		labelFontSize = pyramidLabelFontSizeSmall
		descFontSize = pyramidDescFontSizeSmall
	}

	var children [][]byte
	shapeIdx := uint32(0)

	for i, panel := range panels {
		// Compute width ratio: top level uses pyramidTopWidthRatio, bottom uses 1.0.
		var widthRatio float64
		if numLevels == 1 {
			widthRatio = 1.0
		} else {
			widthRatio = pyramidTopWidthRatio + (1.0-pyramidTopWidthRatio)*float64(i)/float64(numLevels-1)
		}
		levelWidth := int64(float64(bounds.Width) * widthRatio)

		// Position: centered horizontally, stacked vertically.
		levelX := centerX - levelWidth/2
		levelY := bounds.Y + int64(i)*(levelHeight+pyramidGapEMU)

		// Compute trapezoid adj value: controls how much the top edge is inset.
		// For a true trapezoid look, the top edge should be narrower than bottom.
		// adj value in OOXML is in 1/100000 of shape width from each side.
		// We want the top of each trapezoid to match the width of the level above,
		// and the bottom to be the current level width.
		adjValue := pyramidTrapezoidAdj(i, numLevels, widthRatio)

		// Compute fill color: dark (apex) to light (base) using accent1.
		fill := pyramidLevelFill(i, numLevels)

		// Build text paragraphs.
		textColor := pyramidLevelTextColor(i, numLevels)
		var paras []pptx.Paragraph

		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     panel.title,
				Lang:     "en-US",
				FontSize: labelFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    textColor,
			}},
		})

		if panel.body != "" {
			paras = append(paras, pptx.Paragraph{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     panel.body,
					Lang:     "en-US",
					FontSize: descFontSize,
					Dirty:    true,
					Color:    textColor,
				}},
			})
		}

		shapeIdx++
		b, err := pptx.GenerateShape(pptx.ShapeOptions{
			ID:       shapeIDBase + shapeIdx,
			Name:     fmt.Sprintf("Pyramid Level %d", i+1),
			Bounds:   pptx.RectEmu{X: levelX, Y: levelY, CX: levelWidth, CY: levelHeight},
			Geometry: pptx.GeomTrapezoid,
			Adjustments: []pptx.AdjustValue{
				{Name: "adj", Value: adjValue},
			},
			Fill: fill,
			Line: pptx.Line{Width: 0, Fill: pptx.NoFill()},
			Text: &pptx.TextBody{
				Wrap:       "square",
				Anchor:     "ctr",
				Insets:     [4]int64{pyramidTextInset, pyramidTextInset, pyramidTextInset, pyramidTextInset},
				AutoFit:    "normAutofit",
				Paragraphs: paras,
			},
		})
		if err != nil {
			slog.Warn("pyramid: level shape failed", "error", err, "level", i)
			continue
		}
		children = append(children, b)
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Pyramid",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generatePyramidGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// pyramidTrapezoidAdj computes the OOXML trapezoid adj value for a given level.
// The trapezoid preset has the bottom edge at full width and the top edge
// indented by adj/100000 of the shape width from each side.
//
// For level i in a pyramid of n levels:
//   - The top edge should visually align with the width of the level above (i-1).
//   - The bottom edge is the current level's full width.
//
// adj = ((bottomWidth - topWidth) / (2 * bottomWidth)) * 100000
//
// For the topmost level (i=0), adj creates a narrow top (approaching a triangle).
// For the bottommost level (i=n-1), adj=0 (rectangle).
func pyramidTrapezoidAdj(levelIndex, numLevels int, currentWidthRatio float64) int64 {
	if numLevels <= 1 {
		return 0 // Single level: rectangle
	}

	// Top width ratio for this trapezoid (the level above's width ratio).
	var topWidthRatio float64
	if levelIndex == 0 {
		// Apex: top edge is very narrow (half the current width ratio for a pointed look).
		topWidthRatio = currentWidthRatio * 0.4
	} else {
		// The top edge should match the bottom edge of the level above.
		topWidthRatio = pyramidTopWidthRatio + (1.0-pyramidTopWidthRatio)*float64(levelIndex-1)/float64(numLevels-1)
	}

	// The adj value is the fraction of shape width that each side indents at the top.
	// topEdgeWidth = shapeWidth * (1 - 2*adj/100000)
	// So: adj = (1 - topEdgeWidth/shapeWidth) * 100000 / 2
	//        = (1 - topWidthRatio/currentWidthRatio) * 100000 / 2
	if currentWidthRatio <= 0 {
		return 0
	}

	adj := (1.0 - topWidthRatio/currentWidthRatio) * 100000.0 / 2.0
	if adj < 0 {
		adj = 0
	}
	if adj > 50000 {
		adj = 50000 // Maximum: triangle shape
	}
	return int64(adj)
}

// pyramidLevelFill returns a scheme-based fill for a pyramid level.
// Gradient goes from full accent1 saturation (apex, dark) to light tint (base).
//
// Level 0 (apex): lumMod=100000, lumOff=0 (full color)
// Level n-1 (base): lumMod=20000, lumOff=80000 (light tint)
func pyramidLevelFill(levelIndex, numLevels int) pptx.Fill {
	if numLevels <= 1 {
		return pptx.SchemeFill("accent1")
	}

	// t goes from 0.0 (apex) to 1.0 (base)
	t := float64(levelIndex) / float64(numLevels-1)

	// lumMod ranges from 100000 (full color at apex) to 20000 (light tint at base).
	lumMod := 100000 - int(t*80000)
	lumOff := 100000 - lumMod

	if lumOff <= 0 {
		return pptx.SchemeFill("accent1")
	}
	return pptx.SchemeFill("accent1", pptx.LumMod(lumMod), pptx.LumOff(lumOff))
}

// pyramidLevelTextColor returns the text fill for a pyramid level.
// Dark levels (apex) get light text, light levels (base) get dark text.
func pyramidLevelTextColor(levelIndex, numLevels int) pptx.Fill {
	if numLevels <= 1 {
		return pptx.SchemeFill("lt1")
	}

	t := float64(levelIndex) / float64(numLevels-1)
	// Crossover at t=0.5 — upper half (dark fills) gets light text.
	if t < 0.5 {
		return pptx.SchemeFill("lt1")
	}
	return pptx.SchemeFill("dk1")
}

// pyramidEstimateShapeCount returns the estimated number of shapes for ID allocation.
// 1 (group) + N (level shapes)
func pyramidEstimateShapeCount(panels []nativePanelData) uint32 {
	return uint32(1 + len(panels))
}
