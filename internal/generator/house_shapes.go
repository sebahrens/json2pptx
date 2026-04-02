package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// House Diagram Native Shapes — Triangle Roof + Pillar Columns + Foundation
// =============================================================================
//
// Replaces SVG-rendered house diagrams with native OOXML grouped shapes.
// The "strategy house" diagram has three zones:
//
//              /\
//             /  \           ← Triangle (roof: vision/mission)
//            /    \
//           /______\
//          |  P  P  P |     ← Rect columns (strategic pillars)
//          |  I  I  I |
//          |  L  L  L |
//          |__________|
//          | FOUNDATION |   ← Rect (values/culture)
//
// Multi-floor mode supports alternating single (full-width band) and
// parallel (vertical pillar) floors between roof and foundation.
//
// Color strategy: roof=accent1 (full), pillars=accent1..accent6 (light tint),
// foundation=accent1 (darker tint). All scheme color refs for theme awareness.

// House diagram EMU constants.
const (
	// houseGapEMU is the vertical gap between roof, floors, and foundation.
	// ~0.04" = 36576 EMU
	houseGapEMU int64 = 36576

	// housePillarGapEMU is the horizontal gap between pillar columns.
	// ~0.03" = 27432 EMU
	housePillarGapEMU int64 = 27432

	// houseTextInsetEMU is the text inset for shapes (~0.05").
	houseTextInsetEMU int64 = 45720

	// houseLabelFontSize is the section label font size (hundredths of a point).
	// 1100 = 11pt
	houseLabelFontSize int = 1100

	// houseLabelFontSizeSmall is for houses with many pillars (6+).
	// 900 = 9pt
	houseLabelFontSizeSmall int = 900

	// houseItemFontSize is the bullet item font size (hundredths of a point).
	// 900 = 9pt
	houseItemFontSize int = 900

	// houseItemFontSizeSmall is for houses with many pillars (6+).
	// 700 = 7pt
	houseItemFontSizeSmall int = 700

	// houseRoofHeightRatio is the fraction of total height used by the roof.
	houseRoofHeightRatio float64 = 0.22

	// houseFoundationHeightRatio is the fraction of total height for the foundation.
	houseFoundationHeightRatio float64 = 0.13

	// houseMaxSectionsPerFloor is the max number of pillars per floor.
	houseMaxSectionsPerFloor int = 12
)

// houseSectionData holds parsed data for a single pillar section.
type houseSectionData struct {
	label string
	items []string
}

// houseFloorMeta describes one floor between roof and foundation.
type houseFloorMeta struct {
	floorType    string // "single" or "parallel"
	sectionCount int    // 1 for single, N for parallel
}

// houseDiagramMeta holds structural metadata for the house diagram.
type houseDiagramMeta struct {
	roofLabel       string
	foundationLabel string
	floors          []houseFloorMeta
}

// isHouseDiagram returns true if the diagram spec is a house_diagram type.
func isHouseDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "house_diagram"
}

// processHouseDiagramNativeShapes parses house diagram data from a DiagramSpec
// and registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processHouseDiagramNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("house diagram native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("house diagram native shapes: themeOverride is set but scheme color refs will not reflect overrides",
			"slide", slideNum)
	}

	panels, meta, err := parseHouseDiagramNativeData(diagramSpec.Data)
	if err != nil {
		slog.Warn("house diagram native shapes: parse failed", "slide", slideNum, "error", err)
		return
	}

	if len(panels) == 0 {
		slog.Warn("house diagram native shapes: no panels parsed", "slide", slideNum)
		return
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native house diagram shapes: registered",
		"slide", slideNum,
		"panels", len(panels),
		"floors", len(meta.floors),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx:   shapeIdx,
		bounds:           placeholderBounds,
		panels:           panels,
		houseDiagramMode: true,
		houseDiagramMeta: meta,
	})
}

// parseHouseDiagramNativeData extracts house diagram data from the diagram data map.
// Returns panels encoded as: [roof, floor1_sec1, floor1_sec2, ..., floorN_secM, foundation]
// and metadata describing the floor structure.
func parseHouseDiagramNativeData(data map[string]any) ([]nativePanelData, houseDiagramMeta, error) {
	var meta houseDiagramMeta

	// Parse roof.
	meta.roofLabel = parseHouseRoofLabel(data)

	// Parse foundation.
	meta.foundationLabel = parseHouseFoundationLabel(data)

	// Parse floors (multi-story) or sections (classic single-band).
	floors, floorSections := parseHouseNativeFloors(data)

	// Encode into flat panel list: [roof, ...floor sections..., foundation]
	var panels []nativePanelData

	// Panel 0: roof
	panels = append(panels, nativePanelData{
		title: meta.roofLabel,
	})

	// Floor panels
	for i, floor := range floors {
		meta.floors = append(meta.floors, floor)
		for _, sec := range floorSections[i] {
			body := ""
			if len(sec.items) > 0 {
				bulletLines := make([]string, len(sec.items))
				for j, item := range sec.items {
					bulletLines[j] = "- " + item
				}
				body = strings.Join(bulletLines, "\n")
			}
			panels = append(panels, nativePanelData{
				title: sec.label,
				body:  body,
			})
		}
	}

	// Last panel: foundation
	panels = append(panels, nativePanelData{
		title: meta.foundationLabel,
	})

	return panels, meta, nil
}

// parseHouseRoofLabel extracts the roof label from data.
func parseHouseRoofLabel(data map[string]any) string {
	if roofStr, ok := data["roof"].(string); ok {
		return roofStr
	}
	if roofMap, ok := data["roof"].(map[string]any); ok {
		if label, ok := roofMap["label"].(string); ok {
			return label
		}
	}
	// Fallback: center_element
	if ce, ok := data["center_element"].(map[string]any); ok {
		if label, ok := ce["label"].(string); ok {
			return label
		}
	}
	return ""
}

// parseHouseFoundationLabel extracts the foundation label from data.
func parseHouseFoundationLabel(data map[string]any) string {
	if foundStr, ok := data["foundation"].(string); ok {
		return foundStr
	}
	if foundMap, ok := data["foundation"].(map[string]any); ok {
		if label, ok := foundMap["label"].(string); ok {
			return label
		}
	}
	return ""
}

// parseHouseNativeFloors parses floors or sections from data.
// Returns a list of floor metadata and their corresponding section data.
func parseHouseNativeFloors(data map[string]any) ([]houseFloorMeta, [][]houseSectionData) { //nolint:gocognit
	// Try "floors" key first (multi-story layout).
	if rawFloors, ok := data["floors"].([]any); ok && len(rawFloors) > 0 {
		var metas []houseFloorMeta
		var allSections [][]houseSectionData

		for _, item := range rawFloors {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			floorType := inferHouseFloorType(m)
			if floorType == "single" {
				label, _ := m["label"].(string)
				var items []string
				if rawItems, ok := m["items"].([]any); ok {
					for _, it := range rawItems {
						if s, ok := it.(string); ok {
							items = append(items, s)
						}
					}
				}
				metas = append(metas, houseFloorMeta{floorType: "single", sectionCount: 1})
				allSections = append(allSections, []houseSectionData{{label: label, items: items}})
			} else {
				// Parallel floor: parse sections.
				sections := parseHouseNativeSections(m)
				if len(sections) == 0 {
					continue
				}
				metas = append(metas, houseFloorMeta{floorType: "parallel", sectionCount: len(sections)})
				allSections = append(allSections, sections)
			}
		}

		if len(metas) > 0 {
			return metas, allSections
		}
	}

	// Fallback: "sections"/"pillars"/"columns" as a single parallel floor.
	sections := parseHouseNativeSections(data)
	if len(sections) > 0 {
		return []houseFloorMeta{{floorType: "parallel", sectionCount: len(sections)}},
			[][]houseSectionData{sections}
	}

	// Fallback: outer_elements (hub-and-spoke format).
	if oe, ok := data["outer_elements"].([]any); ok && len(oe) > 0 {
		var sections []houseSectionData
		for _, item := range oe {
			switch v := item.(type) {
			case string:
				sections = append(sections, houseSectionData{label: v})
			case map[string]any:
				sec := houseSectionData{}
				if label, ok := v["label"].(string); ok {
					sec.label = label
				}
				if items, ok := v["items"].([]any); ok {
					for _, it := range items {
						if s, ok := it.(string); ok {
							sec.items = append(sec.items, s)
						}
					}
				}
				sections = append(sections, sec)
			}
		}
		if len(sections) > 0 {
			return []houseFloorMeta{{floorType: "parallel", sectionCount: len(sections)}},
				[][]houseSectionData{sections}
		}
	}

	return nil, nil
}

// parseHouseNativeSections parses sections/pillars/columns from a data map.
func parseHouseNativeSections(dataMap map[string]any) []houseSectionData {
	for _, key := range []string{"sections", "pillars", "columns"} {
		raw, ok := dataMap[key].([]any)
		if !ok || len(raw) == 0 {
			continue
		}

		var sections []houseSectionData
		for _, item := range raw {
			switch v := item.(type) {
			case string:
				sections = append(sections, houseSectionData{label: v})
			case map[string]any:
				sec := houseSectionData{}
				if label, ok := v["label"].(string); ok {
					sec.label = label
				}
				if items, ok := v["items"].([]any); ok {
					for _, it := range items {
						if s, ok := it.(string); ok {
							sec.items = append(sec.items, s)
						}
					}
				}
				sections = append(sections, sec)
			}
		}
		return sections
	}
	return nil
}

// inferHouseFloorType determines the floor type from a floor map entry.
func inferHouseFloorType(m map[string]any) string {
	if t, ok := m["type"].(string); ok {
		if t == "single" {
			return "single"
		}
		return "parallel"
	}
	// Infer: if sections/pillars/columns present, it's parallel.
	for _, key := range []string{"sections", "pillars", "columns"} {
		if _, ok := m[key].([]any); ok {
			return "parallel"
		}
	}
	return "single"
}

// =============================================================================
// Group XML Generation
// =============================================================================

// generateHouseDiagramGroupXML produces the complete <p:grpSp> XML for a house diagram.
func generateHouseDiagramGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta houseDiagramMeta) string {
	if len(panels) < 2 {
		return "" // Need at least roof + foundation
	}

	// Layout calculation: roof, floors, foundation with gaps.
	roofH := int64(float64(bounds.Height) * houseRoofHeightRatio)
	foundH := int64(float64(bounds.Height) * houseFoundationHeightRatio)

	nFloors := len(meta.floors)
	if nFloors == 0 {
		nFloors = 1
	}

	// Gaps: roof-to-floor + between-floors + floor-to-foundation
	totalGaps := int64(nFloors+1) * houseGapEMU
	totalFloorH := bounds.Height - roofH - foundH - totalGaps
	if totalFloorH < 0 {
		totalFloorH = bounds.Height / 4
	}
	floorH := totalFloorH / int64(nFloors)

	// Count total pillars to choose font size.
	totalSections := 0
	for _, f := range meta.floors {
		totalSections += f.sectionCount
	}
	labelFont := houseLabelFontSize
	itemFont := houseItemFontSize
	if totalSections >= 6 {
		labelFont = houseLabelFontSizeSmall
		itemFont = houseItemFontSizeSmall
	}

	var children [][]byte
	shapeIdx := uint32(0)

	// --- Roof (triangle) ---
	roofY := bounds.Y
	roofTitle := panels[0].title // safe: len(panels) >= 2 checked above
	shapeIdx++
	roofShape, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeIDBase + shapeIdx,
		Name:     "House Roof",
		Bounds:   pptx.RectEmu{X: bounds.X, Y: roofY, CX: bounds.Width, CY: roofH},
		Geometry: pptx.GeomTriangle,
		Fill:     pptx.SchemeFill("accent1"),
		Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{houseTextInsetEMU, houseTextInsetEMU * 3, houseTextInsetEMU, houseTextInsetEMU},
			AutoFit: "normAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     roofTitle,
					Lang:     "en-US",
					FontSize: labelFont,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("lt1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("house diagram: roof shape failed", "error", err)
	} else {
		children = append(children, roofShape)
	}

	// --- Floor sections ---
	curY := roofY + roofH + houseGapEMU
	panelIdx := 1 // Skip panel 0 (roof)

	for _, floor := range meta.floors {
		if floor.floorType == "single" {
			// Full-width band
			if panelIdx < len(panels)-1 {
				shapeIdx++
				singleShape := generateHouseSingleFloorShape(
					shapeIDBase+shapeIdx, panels[panelIdx],
					bounds.X, curY, bounds.Width, floorH,
					labelFont, itemFont,
				)
				if singleShape != nil {
					children = append(children, singleShape)
				}
				panelIdx++
			}
		} else {
			// Parallel pillars
			n := floor.sectionCount
			if n > houseMaxSectionsPerFloor {
				n = houseMaxSectionsPerFloor
			}
			totalPillarGaps := int64(n-1) * housePillarGapEMU
			pillarW := (bounds.Width - totalPillarGaps) / int64(n)

			for j := 0; j < n && panelIdx < len(panels)-1; j++ {
				px := bounds.X + int64(j)*(pillarW+housePillarGapEMU)
				accentIdx := j % 6

				shapeIdx++
				pillarShape := generateHousePillarShape(
					shapeIDBase+shapeIdx, panels[panelIdx],
					px, curY, pillarW, floorH,
					accentIdx, labelFont, itemFont,
				)
				if pillarShape != nil {
					children = append(children, pillarShape)
				}
				panelIdx++
			}
		}
		curY += floorH + houseGapEMU
	}

	// --- Foundation ---
	foundY := curY
	if panelIdx < len(panels) {
		shapeIdx++
		foundShape, err := pptx.GenerateShape(pptx.ShapeOptions{
			ID:       shapeIDBase + shapeIdx,
			Name:     "House Foundation",
			Bounds:   pptx.RectEmu{X: bounds.X, Y: foundY, CX: bounds.Width, CY: foundH},
			Geometry: pptx.GeomRect,
			Fill:     pptx.SchemeFill("accent1", pptx.LumMod(75000), pptx.LumOff(0)),
			Line:     pptx.Line{Width: 0, Fill: pptx.NoFill()},
			Text: &pptx.TextBody{
				Wrap:    "square",
				Anchor:  "ctr",
				Insets:  [4]int64{houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU},
				AutoFit: "normAutofit",
				Paragraphs: []pptx.Paragraph{{
					Align:    "ctr",
					NoBullet: true,
					Runs: []pptx.Run{{
						Text:     panels[panelIdx].title,
						Lang:     "en-US",
						FontSize: labelFont,
						Bold:     true,
						Dirty:    true,
						Color:    pptx.SchemeFill("lt1"),
					}},
				}},
			},
		})
		if err != nil {
			slog.Warn("house diagram: foundation shape failed", "error", err)
		} else {
			children = append(children, foundShape)
		}
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "House Diagram",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateHouseDiagramGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateHouseSingleFloorShape generates a full-width band shape for a single floor.
func generateHouseSingleFloorShape(id uint32, panel nativePanelData, x, y, w, h int64, labelFont, itemFont int) []byte {
	var paras []pptx.Paragraph
	paras = append(paras, pptx.Paragraph{
		Align:    "ctr",
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     panel.title,
			Lang:     "en-US",
			FontSize: labelFont,
			Bold:     true,
			Dirty:    true,
			Color:    pptx.SchemeFill("dk1"),
		}},
	})

	// Add bullet items from body.
	if panel.body != "" {
		for _, line := range strings.Split(panel.body, "\n") {
			text := strings.TrimPrefix(line, "- ")
			paras = append(paras, pptx.Paragraph{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     text,
					Lang:     "en-US",
					FontSize: itemFont,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			})
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       id,
		Name:     fmt.Sprintf("Floor %s", panel.title),
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: w, CY: h},
		Geometry: pptx.GeomRect,
		Fill:     pptx.SchemeFill("accent1", pptx.LumMod(20000), pptx.LumOff(80000)),
		Line:     pptx.Line{Width: 6350, Fill: pptx.SchemeFill("accent1", pptx.LumMod(40000), pptx.LumOff(60000))},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("house diagram: single floor shape failed", "error", err, "id", id)
		return nil
	}
	return b
}

// generateHousePillarShape generates a pillar rectangle for a parallel floor.
func generateHousePillarShape(id uint32, panel nativePanelData, x, y, w, h int64, accentIdx, labelFont, itemFont int) []byte {
	// Cycle through accent1..accent6 with light tint fills.
	accentName := fmt.Sprintf("accent%d", (accentIdx%6)+1)
	fill := pptx.SchemeFill(accentName, pptx.LumMod(20000), pptx.LumOff(80000))
	lineFill := pptx.SchemeFill(accentName, pptx.LumMod(50000), pptx.LumOff(50000))
	textColor := pptx.SchemeFill("dk1")

	var paras []pptx.Paragraph

	// Title paragraph.
	paras = append(paras, pptx.Paragraph{
		Align:    "ctr",
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     panel.title,
			Lang:     "en-US",
			FontSize: labelFont,
			Bold:     true,
			Dirty:    true,
			Color:    textColor,
		}},
	})

	// Bullet items from body.
	if panel.body != "" {
		for _, line := range strings.Split(panel.body, "\n") {
			text := strings.TrimPrefix(line, "- ")
			paras = append(paras, pptx.Paragraph{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     "\u2022 " + text,
					Lang:     "en-US",
					FontSize: itemFont,
					Dirty:    true,
					Color:    textColor,
				}},
			})
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       id,
		Name:     fmt.Sprintf("Pillar %s", panel.title),
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: w, CY: h},
		Geometry: pptx.GeomRect,
		Fill:     fill,
		Line:     pptx.Line{Width: 6350, Fill: lineFill},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU, houseTextInsetEMU},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("house diagram: pillar shape failed", "error", err, "id", id)
		return nil
	}
	return b
}

// houseDiagramEstimateShapeCount returns the estimated number of shapes for ID allocation.
// 1 (group) + 1 (roof) + N (floor sections) + 1 (foundation)
func houseDiagramEstimateShapeCount(panels []nativePanelData) uint32 {
	return uint32(1 + len(panels))
}
