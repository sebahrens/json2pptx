package generator

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
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

	// pyramidLabelFontSize is the level label font size (hundredths of a
	// point), set bold. 1200 = 12pt (it was 11pt, and 9pt from eight levels).
	pyramidLabelFontSize int = tokens.TypeScaleBodyHPt

	// pyramidDescFontSize is the description font size: the 12pt body step,
	// the smallest size a projected slide carries. It was 9pt — 7pt from
	// eight levels — written with no shrink for any check to see
	// (go-slide-creator-6ne1m).
	pyramidDescFontSize int = tokens.TypeScaleBodyHPt

	// pyramidTextInset is the text inset for level shapes (EMU): the uniform
	// 0.5 cm shape text margin.
	pyramidTextInset = pptx.ShapeTextInsetEMU

	// pyramidTopWidthRatio is the width ratio for the top (apex) level.
	// 0.15 = 15% of full width, matching the SVG default.
	pyramidTopWidthRatio float64 = 0.15

	// pyramidMaxApexRatio caps how far pyramidApexRatio widens the apex. At
	// 12pt a description needs more of a narrow column than it did at 9pt:
	// the cap went from 0.30 to 0.40 so "description" stays whole in a third
	// of a slide.
	pyramidMaxApexRatio float64 = 0.40

	// pyramidBoldWidthFactor allows for a bold label measured in the regular
	// face.
	pyramidBoldWidthFactor = 1.08

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

// pyramidPanels encodes a pyramid spec's levels into panels for the
// panelShapeInsert system.
func pyramidPanels(diagramSpec *types.DiagramSpec) ([]nativePanelData, error) {
	levels, err := parsePyramidDiagramData(diagramSpec.Data)
	if err != nil {
		return nil, err
	}
	panels := make([]nativePanelData, 0, len(levels))
	for _, l := range levels {
		panels = append(panels, nativePanelData{title: l.label, body: l.description})
	}
	return panels, nil
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
func generatePyramidGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, fontName string, themeColors ...types.ThemeColor) string {
	numLevels := len(panels)
	if numLevels == 0 {
		return ""
	}

	centerX := bounds.X + bounds.Width/2

	// One size at every level count. A pyramid of eight levels used to drop
	// to 9pt labels over 7pt descriptions; a pyramid too dense for its region
	// now gives up padding, then stores a shrink the readability check
	// reports with the level count that fits.
	labelFontSize := pyramidLabelFontSize
	descFontSize := pyramidDescFontSize
	themeFonts := pptx.ThemeFonts{Major: fontName, Minor: fontName}
	if fontName == "" {
		fontName = "Arial"
	}
	levelHeights, levelGap, pad, _ := pyramidLevelLayout(panels, bounds, labelFontSize, descFontSize, fontName)
	apex := pyramidApexRatio(panels, bounds, labelFontSize, descFontSize, fontName)

	var children [][]byte
	shapeIdx := uint32(0)
	levelY := bounds.Y

	for i, panel := range panels {
		// Compute width ratio: top level uses pyramidTopWidthRatio, bottom uses 1.0.
		widthRatio := pyramidLevelWidthRatioApex(i, numLevels, apex)
		levelWidth := int64(float64(bounds.Width) * widthRatio)

		// Position: centered horizontally, stacked vertically.
		levelX := centerX - levelWidth/2

		// Compute trapezoid adj value: controls how much the top edge is inset.
		// For a true trapezoid look, the top edge should be narrower than bottom.
		// adj value in OOXML is in 1/100000 of shape width from each side.
		// We want the top of each trapezoid to match the width of the level above,
		// and the bottom to be the current level width.
		adjValue := pyramidTrapezoidAdjApex(i, numLevels, widthRatio, apex)

		// Compute fill color: dark (apex) to light (base) using accent1.
		fill := pyramidLevelFill(i, numLevels)

		// Build text paragraphs.
		textColor := pyramidLevelTextColor(i, numLevels, themeColors)
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
			Bounds:   pptx.RectEmu{X: levelX, Y: levelY, CX: levelWidth, CY: levelHeights[i]},
			Geometry: pptx.GeomTrapezoid,
			Adjustments: []pptx.AdjustValue{
				{Name: "adj", Value: adjValue},
			},
			Fill: fill,
			Line: pptx.Line{Width: 0, Fill: pptx.NoFill()},
			Text: &pptx.TextBody{
				Wrap:       "square",
				Anchor:     "ctr",
				Insets:     [4]int64{pyramidTextInset, pad, pyramidTextInset, pad},
				AutoFit:    "normAutofit",
				Paragraphs: paras,
				ThemeFonts: themeFonts,
			},
		})
		if err != nil {
			slog.Warn("pyramid: level shape failed", "error", err, "level", i)
			levelY += levelHeights[i] + levelGap
			continue
		}
		children = append(children, b)
		levelY += levelHeights[i] + levelGap
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

// pyramidLevelWidthRatioApex is a level's width as a fraction of the full
// width, interpolated from the apex ratio at the top to 1 at the base.
func pyramidLevelWidthRatioApex(levelIndex, numLevels int, apex float64) float64 {
	if numLevels <= 1 {
		return 1
	}
	return apex + (1-apex)*float64(levelIndex)/float64(numLevels-1)
}

// pyramidApexRatio is the apex width ratio: the default 15% point, widened —
// up to pyramidMaxApexRatio — until the apex label and description each fit on
// one line across the trapezoid's midpoint. A 15% apex on a short content area
// wrapped "Self-Actualization" onto three lines, took the height the tiers
// below it needed, and every other tier stored a 74% shrink of its 9pt
// description (go-slide-creator-zbo58).
func pyramidApexRatio(panels []nativePanelData, bounds types.BoundingBox, labelSize, descSize int, fontName string) float64 {
	n := len(panels)
	if n <= 1 || bounds.Width <= 0 {
		return pyramidTopWidthRatio
	}
	need := int64(0)
	for _, line := range []struct {
		text string
		size int
		bold bool
	}{{panels[0].title, labelSize, true}, {panels[0].body, descSize, false}} {
		if strings.TrimSpace(line.text) == "" {
			continue
		}
		w, err := textfit.MeasureLineWidth(line.text, fontName, float64(line.size)/100)
		if err != nil {
			continue
		}
		if line.bold {
			w = int64(float64(w) * pyramidBoldWidthFactor)
		}
		need = max(need, w)
	}
	if need == 0 {
		return pyramidTopWidthRatio
	}
	for ratio := pyramidTopWidthRatio; ratio < pyramidMaxApexRatio; ratio += 0.01 {
		adj := pyramidTrapezoidAdjApex(0, n, ratio, ratio)
		mid := int64(float64(bounds.Width) * ratio * (1 - float64(adj)/100000))
		// measureNativeText (textfit.MeasureRun), which sizes the tiers,
		// removes the OOXML default 0.1in sides from the width it is handed.
		if mid-2*pyramidTextInset-2*91440 >= need {
			return ratio
		}
	}
	return pyramidMaxApexRatio
}

// pyramidLevelHeights gives narrow, text-heavy tiers more of the fixed
// placeholder height. Text is measured at its authored font size against the
// trapezoid's midpoint width, not the much wider bounding rectangle.
func pyramidLevelHeights(panels []nativePanelData, bounds types.BoundingBox, labelSize, descSize int, fontName string) ([]int64, int64) {
	heights, gap, _, _ := pyramidLevelLayout(panels, bounds, labelSize, descSize, fontName)
	return heights, gap
}

// pyramidLevelLayout sizes the tiers at the first top / bottom text margin of
// nativeVerticalPadSteps at which every tier holds its text at the authored
// size: a pyramid gives up the air in its tiers before its type. One that fits
// at no step keeps the last, and its tiers share the shortfall
// (go-slide-creator-6ne1m).
func pyramidLevelLayout(panels []nativePanelData, bounds types.BoundingBox, labelSize, descSize int, fontName string) (heights []int64, gap, pad int64, fits bool) {
	for _, pad = range nativeVerticalPadSteps {
		heights, gap, fits = pyramidLevelHeightsAt(panels, bounds, labelSize, descSize, fontName, pad)
		if fits {
			break
		}
	}
	return heights, gap, pad, fits
}

// pyramidFitBudget measures what the pyramid holds at the authored size in
// bounds: how many levels of a one-line label and a one-line description, and
// how many characters a line takes in the apex, the narrowest tier.
func pyramidFitBudget(panels []nativePanelData, bounds types.BoundingBox, fontName string) nativeFitBudget {
	if fontName == "" {
		fontName = "Arial"
	}
	levels := 0
	for n := 1; n <= pyramidMaxLevels; n++ {
		probe := make([]nativePanelData, n)
		for i := range probe {
			probe[i] = nativePanelData{title: "Level", body: "Detail"}
		}
		if _, _, _, fits := pyramidLevelLayout(probe, bounds, pyramidLabelFontSize, pyramidDescFontSize, fontName); !fits {
			break
		}
		levels = n
	}
	// The apex of a pyramid of that many levels, at its widest.
	n := max(levels, 2)
	adj := pyramidTrapezoidAdjApex(0, n, pyramidMaxApexRatio, pyramidMaxApexRatio)
	mid := int64(float64(bounds.Width) * pyramidMaxApexRatio * (1 - float64(adj)/100000))
	chars := 0
	sample := []rune(nativeBudgetSample)
	for k := 1; k <= len(sample); k++ {
		w, err := textfit.MeasureLineWidth(strings.TrimSpace(string(sample[:k])), fontName, float64(pyramidDescFontSize)/100)
		if err != nil || w > mid-2*pyramidTextInset-2*91440 {
			break
		}
		chars = k
	}
	return nativeFitBudget{
		item: "level", container: "pyramid", maxItems: levels, maxChars: chars,
		sizePt: float64(pyramidDescFontSize) / 100,
	}
}

// pyramidLevelHeightsAt is the tier heights at a top / bottom text margin of
// pad, and whether every tier then holds its text unshrunk.
func pyramidLevelHeightsAt(panels []nativePanelData, bounds types.BoundingBox, labelSize, descSize int, fontName string, pad int64) ([]int64, int64, bool) {
	n := len(panels)
	if n == 0 {
		return nil, 0, true
	}
	gap := pyramidGapEMU
	apex := pyramidApexRatio(panels, bounds, labelSize, descSize, fontName)
	minimumTierHeight := 2*pad + int64(math.Ceil(float64(labelSize)/100*1.2*float64(types.EMUPerPoint)))
	if bounds.Height <= int64(n-1)*gap+int64(n)*minimumTierHeight {
		gap = 0
	}
	available := bounds.Height - int64(n-1)*gap
	if available < 0 {
		available = 0
	}
	weights := make([]int64, n)
	var totalWeight int64
	for i, panel := range panels {
		widthRatio := pyramidLevelWidthRatioApex(i, n, apex)
		levelWidth := int64(float64(bounds.Width) * widthRatio)
		adj := pyramidTrapezoidAdjApex(i, n, widthRatio, apex)
		// At half height the sides have expanded halfway from the top edge.
		midWidth := int64(float64(levelWidth) * (1 - float64(adj)/100000))
		textWidth := midWidth - 2*pyramidTextInset
		if textWidth < 1 {
			textWidth = 1
		}
		need := 2 * pad
		if strings.TrimSpace(panel.title) != "" {
			need += measureNativeText(panel.title, fontName, float64(labelSize)/100, textWidth)
		}
		if strings.TrimSpace(panel.body) != "" {
			need += measureNativeText(panel.body, fontName, float64(descSize)/100, textWidth)
		}
		if need < 1 {
			need = 1
		}
		weights[i] = need
		totalWeight += need
	}
	heights := make([]int64, n)
	if totalWeight <= available {
		// Distribute extra breathing room equally; measured differences remain.
		extra := (available - totalWeight) / int64(n)
		for i := range heights {
			heights[i] = weights[i] + extra
		}
	} else {
		// A dense diagram cannot fit at the authored sizes. Reserve enough
		// space for one label line in every tier, then allocate the rest by
		// unmet need. Otherwise an extreme apex could collapse the base.
		floor := minimumTierHeight
		if evenShare := available / int64(n); floor > evenShare {
			floor = evenShare
		}
		remaining := available - int64(n)*floor
		var totalSurplus int64
		for _, weight := range weights {
			if weight > floor {
				totalSurplus += weight - floor
			}
		}
		for i := range heights {
			heights[i] = floor
			if totalSurplus > 0 && weights[i] > floor {
				heights[i] += int64(math.Floor(float64(remaining) * float64(weights[i]-floor) / float64(totalSurplus)))
			}
		}
	}
	var assigned int64
	for _, height := range heights {
		assigned += height
	}
	heights[n-1] += available - assigned
	return heights, gap, totalWeight <= available
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
	return pyramidTrapezoidAdjApex(levelIndex, numLevels, currentWidthRatio, pyramidTopWidthRatio)
}

// pyramidTrapezoidAdjApex is pyramidTrapezoidAdj for an apex of the given
// width ratio.
func pyramidTrapezoidAdjApex(levelIndex, numLevels int, currentWidthRatio, apex float64) int64 {
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
		topWidthRatio = apex + (1.0-apex)*float64(levelIndex-1)/float64(numLevels-1)
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
// Gradient goes from full accent1 (apex, dark) to an RGB tint (base).
//
// Level 0 (apex): 100% accent, 0% white.
// Level n-1 (base): 20% accent, 80% white.
func pyramidLevelFill(levelIndex, numLevels int) pptx.Fill {
	if numLevels <= 1 {
		return pptx.SchemeFill("accent1")
	}

	// t goes from 0.0 (apex) to 1.0 (base)
	t := float64(levelIndex) / float64(numLevels-1)

	// The retained accent fraction ranges from 100000 to 20000.
	lumMod := 100000 - int(t*80000)
	lumOff := 100000 - lumMod

	if lumOff <= 0 {
		return pptx.SchemeFill("accent1")
	}
	return diagramTintFill("accent1", lumMod, lumOff)
}

// pyramidLevelTone is a level's fill as scheme colour and lumMod / lumOff:
// what pyramidLevelFill renders and what the ink is measured against.
func pyramidLevelTone(levelIndex, numLevels int) heatmapTone {
	if numLevels <= 1 {
		return heatmapTone{scheme: "accent1"}
	}
	t := float64(levelIndex) / float64(numLevels-1)
	lumMod := 100000 - int(t*80000)
	if lumMod >= 100000 {
		return heatmapTone{scheme: "accent1"}
	}
	return heatmapTone{scheme: "accent1", lumMod: lumMod, lumOff: 100000 - lumMod}
}

// pyramidLevelTextColor returns the text fill for a pyramid level: the text
// role (lt1 or dk1) with the higher measured contrast on the tier's own fill.
//
// The ink used to follow the level index: lt1 on the upper half of the tiers,
// dk1 below. On a bright accent the second tier of five is already a light
// tint (p-style FD5108 at lumMod 80 / lumOff 20) and white on it measured
// about 2.6:1 (go-slide-creator-3fct0). Without theme colours there is
// nothing to measure and the index rule stands.
func pyramidLevelTextColor(levelIndex, numLevels int, themeColors []types.ThemeColor) pptx.Fill {
	if len(themeColors) > 0 {
		return pptx.SchemeFill(heatmapValueColor(pyramidLevelTone(levelIndex, numLevels), themeColors))
	}
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

// pyramidMeasureFont is the face a pyramid's tiers are measured in: the
// template's body face, or Arial when the template names none.
func pyramidMeasureFont(fontName string) string {
	if fontName == "" {
		return "Arial"
	}
	return fontName
}
