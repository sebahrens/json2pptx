package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// House Diagram Native Shapes — gabled roof over pillar and band levels
// =============================================================================
//
// The house_diagram is drawn by the same builder as the strategy-house
// pattern (patterns.BuildHouse), so the two cannot drift
// (go-slide-creator-x25dq): one accent for the roof and the pillar rules,
// neutral pillar and band surfaces, a gable pitched from the width, and
// levels sized from their text.
//
//	        /\
//	   ____/  \____      roof (vision / objective)
//	  |____________|
//	  [P1][P2][P3][P4]   sections: the pillar row
//	  [ floor       ]    floors: bands and rows of cells, top to bottom
//	  [ foundation  ]    foundation band
//
// This file only maps the diagram's data onto the shared house model and
// places the builder's grid in the diagram's bounds.

const (
	// houseMaxSectionsPerFloor is the most cells one level may carry.
	houseMaxSectionsPerFloor int = 12
	// houseDenseSections is the row width from which the house switches to
	// the dense type sizes.
	houseDenseSections = 6
)

// houseSectionData holds parsed data for a single pillar section.
type houseSectionData struct {
	label string
	items []string
}

// houseFloorMeta describes one level between roof and foundation.
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

// parseHouseDiagramNativeData extracts house diagram data from the diagram data map.
// Returns panels encoded as: [roof, floor1_sec1, floor1_sec2, ..., floorN_secM, foundation]
// and metadata describing the floor structure.
func parseHouseDiagramNativeData(data map[string]any) ([]nativePanelData, houseDiagramMeta, error) {
	var meta houseDiagramMeta
	meta.roofLabel = parseHouseRoofLabel(data)
	meta.foundationLabel = parseHouseFoundationLabel(data)
	floors, floorSections := parseHouseNativeFloors(data)

	panels := []nativePanelData{{title: meta.roofLabel}}
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
			panels = append(panels, nativePanelData{title: sec.label, body: body})
		}
	}
	panels = append(panels, nativePanelData{title: meta.foundationLabel})
	return panels, meta, nil
}

// houseModel maps the parsed panels onto the shared house model: the roof,
// one level per floor, and the foundation band.
//
// A row of sections is a pillar row; a later row whose sections carry labels
// only is a level split into cells.
func houseModel(panels []nativePanelData, meta houseDiagramMeta) patterns.HouseModel {
	m := patterns.HouseModel{RoofOverrideIndex: -1, BadgeOverrideIndex: -1}
	if len(panels) < 2 {
		return m
	}
	m.Roof = panels[0].title
	last := len(panels) - 1
	idx, pillarRows := 1, 0
	for _, floor := range meta.floors {
		level := patterns.HouseLevel{Kind: patterns.HouseBand, OverrideIndex: -1}
		hasItems := false
		for j := 0; j < max(floor.sectionCount, 1) && idx < last; j++ {
			cell := patterns.HouseCell{Title: panels[idx].title}
			for _, line := range strings.Split(panels[idx].body, "\n") {
				if item := strings.TrimSpace(strings.TrimPrefix(line, "- ")); item != "" {
					cell.Body = append(cell.Body, item)
				}
			}
			hasItems = hasItems || len(cell.Body) > 0
			level.Cells = append(level.Cells, cell)
			idx++
		}
		if len(level.Cells) == 0 {
			continue
		}
		if floor.floorType == "parallel" && (pillarRows == 0 || hasItems) {
			level.Kind = patterns.HousePillars
			pillarRows++
		}
		m.Levels = append(m.Levels, level)
	}
	if foundation := panels[last].title; strings.TrimSpace(foundation) != "" {
		m.Levels = append(m.Levels, patterns.HouseLevel{
			Kind: patterns.HouseBand, OverrideIndex: -1,
			Cells: []patterns.HouseCell{{Title: foundation}},
		})
	}
	return m
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

// parseHouseNativeFloors returns the levels between roof and foundation, top
// to bottom, with their sections.
//
// "sections" (alias "pillars" / "columns") is the pillar row. "floors" lists
// further levels: a string or {label, items?} is a full-width band, and
// {sections: [...]} is a row of cells. When both are given the pillar row
// comes first and the floors follow under it; floors alone are drawn in the
// order written. ValidateNativeDiagramData refuses every other floors shape,
// so nothing reaches this parser that it would skip.
func parseHouseNativeFloors(data map[string]any) ([]houseFloorMeta, [][]houseSectionData) {
	var metas []houseFloorMeta
	var allSections [][]houseSectionData

	if sections := parseHouseNativeSections(data); len(sections) > 0 {
		metas = append(metas, houseFloorMeta{floorType: "parallel", sectionCount: len(sections)})
		allSections = append(allSections, sections)
	}
	rawFloors, _ := data["floors"].([]any)
	for _, item := range rawFloors {
		switch v := item.(type) {
		case string:
			metas = append(metas, houseFloorMeta{floorType: "single", sectionCount: 1})
			allSections = append(allSections, []houseSectionData{{label: v}})
		case map[string]any:
			if inferHouseFloorType(v) == "single" {
				label, _ := v["label"].(string)
				metas = append(metas, houseFloorMeta{floorType: "single", sectionCount: 1})
				allSections = append(allSections, []houseSectionData{{label: label, items: houseStringList(v["items"])}})
				continue
			}
			sections := parseHouseNativeSections(v)
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

	// Fallback: outer_elements (hub-and-spoke format).
	if sections := houseSectionList(data["outer_elements"]); len(sections) > 0 {
		return []houseFloorMeta{{floorType: "parallel", sectionCount: len(sections)}},
			[][]houseSectionData{sections}
	}
	return nil, nil
}

// houseSectionKeys are the spellings of a row of sections.
var houseSectionKeys = []string{"sections", "pillars", "columns"}

// parseHouseNativeSections parses sections/pillars/columns from a data map.
func parseHouseNativeSections(dataMap map[string]any) []houseSectionData {
	for _, key := range houseSectionKeys {
		if sections := houseSectionList(dataMap[key]); len(sections) > 0 {
			return sections
		}
	}
	return nil
}

// houseSectionList parses a list of sections: strings or {label, items?}.
func houseSectionList(value any) []houseSectionData {
	raw, _ := value.([]any)
	var sections []houseSectionData
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			sections = append(sections, houseSectionData{label: v})
		case map[string]any:
			label, _ := v["label"].(string)
			sections = append(sections, houseSectionData{label: label, items: houseStringList(v["items"])})
		}
	}
	return sections
}

func houseStringList(value any) []string {
	raw, _ := value.([]any)
	var out []string
	for _, it := range raw {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
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
	for _, key := range houseSectionKeys {
		if _, ok := m[key].([]any); ok {
			return "parallel"
		}
	}
	return "single"
}

// =============================================================================
// Layout and group XML generation
// =============================================================================

// houseStyle is the look of a native house: the template's primary accent,
// neutral surfaces, and the pattern's type sizes — dense sizes once a row
// carries six or more sections.
func houseStyle(meta houseDiagramMeta, env nativeDiagramEnv) patterns.HouseStyle {
	st := patterns.HouseStyle{
		Fonts:  pptx.ThemeFonts{Major: env.fontName, Minor: env.fontName},
		Accent: patterns.PrimaryFill(env.themeColors),
		// A house with room steps its type up, as the pattern's does.
		Grow: true,
	}
	for _, floor := range meta.floors {
		if floor.sectionCount >= houseDenseSections {
			st.HeaderPt, st.BandPt, st.BodyPt = tokens.TypeScaleBodyPt, tokens.TypeScaleBodyPt, tokens.BodyTextMinPt
			st.Grow = false
			break
		}
	}
	// The pattern's tones, measured on this template's colours: tinted caps
	// and bands, a neutral-dark base.
	patterns.HouseTonalStyle(patterns.ExpandContext{Theme: types.ThemeInfo{Colors: env.themeColors}}, &st)
	return st
}

// layoutHouse lays the house out in bounds with the shared builder.
func layoutHouse(panels []nativePanelData, meta houseDiagramMeta, bounds types.BoundingBox, env nativeDiagramEnv) (*patterns.HouseLayout, error) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return nil, fmt.Errorf("house_diagram: empty bounds")
	}
	return patterns.BuildHouse(houseModel(panels, meta), houseStyle(meta, env),
		float64(bounds.Width)/float64(types.EMUPerPoint), float64(bounds.Height)/float64(types.EMUPerPoint))
}

// houseGrid converts the builder's grid — shape cells with optional accent
// bars, one pinned row per level — to the layout engine's grid in bounds.
func houseGrid(in *jsonschema.ShapeGridInput, cols int, bounds types.BoundingBox, fonts pptx.ThemeFonts) (*shapegrid.Grid, error) {
	widths, err := shapegrid.ResolveColumns(cols, nil)
	if err != nil {
		return nil, err
	}
	grid := &shapegrid.Grid{
		Bounds:  pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height},
		Columns: widths,
		ColGap:  in.Gap,
		RowGap:  in.RowGap,
		VAlign:  shapegrid.VAlignTop,
	}
	for _, r := range in.Rows {
		row := shapegrid.Row{MinHeight: r.MinHeight, MaxHeight: r.MaxHeight}
		for _, c := range r.Cells {
			cell := shapegrid.Cell{ColSpan: c.ColSpan}
			if c.Shape != nil {
				cell.Shape = &shapegrid.ShapeSpec{
					Geometry:    c.Shape.Geometry,
					TypeScale:   c.Shape.TypeScale,
					Fill:        c.Shape.Fill,
					Line:        c.Shape.Line,
					Text:        c.Shape.Text,
					Adjustments: c.Shape.Adjustments,
					ThemeFonts:  fonts,
				}
			}
			if c.AccentBar != nil {
				cell.AccentBar = &shapegrid.AccentBarSpec{Position: c.AccentBar.Position, Color: c.AccentBar.Color, Width: c.AccentBar.Width}
			}
			// A pillar's cap is a layer of its cell.
			for _, l := range c.Layers {
				if l.Shape == nil {
					continue
				}
				cell.Layers = append(cell.Layers, shapegrid.Layer{
					Name:  l.Name,
					Frame: shapegrid.LayerFrame{X: l.Frame.X, Y: l.Frame.Y, W: l.Frame.W, H: l.Frame.H},
					Shape: &shapegrid.ShapeSpec{
						Geometry:    l.Shape.Geometry,
						TypeScale:   l.Shape.TypeScale,
						Fill:        l.Shape.Fill,
						Line:        l.Shape.Line,
						Text:        l.Shape.Text,
						Adjustments: l.Shape.Adjustments,
						ThemeFonts:  fonts,
					},
				})
			}
			row.Cells = append(row.Cells, cell)
		}
		grid.Rows = append(grid.Rows, row)
	}
	return grid, nil
}

// houseColumnCount is the number of grid columns the house's rows span.
func houseColumnCount(in *jsonschema.ShapeGridInput) int {
	cols := 1
	for _, r := range in.Rows {
		n := 0
		for _, c := range r.Cells {
			n += max(c.ColSpan, 1)
		}
		cols = max(cols, n)
	}
	return cols
}

// generateHouseDiagramGroupXML produces the complete <p:grpSp> XML for a house diagram.
func generateHouseDiagramGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, meta houseDiagramMeta, env nativeDiagramEnv) string {
	if len(panels) < 2 {
		return "" // Need at least roof + foundation
	}
	layout, err := layoutHouse(panels, meta, bounds, env)
	if err != nil {
		slog.Warn("house diagram: layout failed", "error", err)
		return ""
	}
	// The pass every expanded pattern gets: an accent too light for the
	// roof's white text is deepened, exactly as the strategy-house's is.
	patterns.ApplyReadableInk(patterns.ExpandContext{Theme: types.ThemeInfo{Colors: env.themeColors}}, layout.Grid)
	fonts := pptx.ThemeFonts{Major: env.fontName, Minor: env.fontName}
	grid, err := houseGrid(layout.Grid, houseColumnCount(layout.Grid), bounds, fonts)
	if err != nil {
		slog.Warn("house diagram: grid failed", "error", err)
		return ""
	}
	alloc := pptx.NewShapeIDAllocator(nil)
	alloc.SetMinID(shapeIDBase + 1)
	resolved, err := shapegrid.Resolve(grid, alloc)
	if err != nil {
		slog.Warn("house diagram: resolve failed", "error", err)
		return ""
	}

	names := houseShapeNames(houseModel(panels, meta), strings.TrimSpace(panels[len(panels)-1].title) != "")
	var children [][]byte
	// Names follow the cells in drawing order; a pillar's cap layer comes
	// right after its cell and takes that cell's name.
	next, last := 0, ""
	for _, cell := range resolved.Cells {
		if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil {
			if !cell.Layer {
				next++
			}
			continue
		}
		name := ""
		if cell.Layer {
			if last != "" {
				name = last + " Cap"
			}
		} else {
			if next < len(names) {
				name = names[next]
			}
			next, last = next+1, name
		}
		xml, err := shapegrid.GenerateCellShapeXML(cell)
		if err != nil {
			slog.Warn("house diagram: shape failed", "error", err, "id", cell.ID)
			continue
		}
		if name != "" {
			xml = []byte(strings.Replace(string(xml),
				fmt.Sprintf(`name="Shape %d"`, cell.ID), fmt.Sprintf(`name="%s"`, pptxEscapeAttr(name)), 1))
		}
		children = append(children, xml)
	}
	for i := range resolved.AccentBars {
		xml, err := shapegrid.GenerateAccentBarXML(&resolved.AccentBars[i])
		if err != nil {
			slog.Warn("house diagram: accent rule failed", "error", err)
			continue
		}
		children = append(children, xml)
	}

	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "House Diagram",
		Bounds:   pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height},
		Children: children,
	})
	if err != nil {
		slog.Warn("generateHouseDiagramGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// houseShapeNames names the house's shapes in drawing order: the roof, then
// every level's cells.
func houseShapeNames(m patterns.HouseModel, foundation bool) []string {
	names := []string{"House Roof"}
	for i, level := range m.Levels {
		for _, c := range level.Cells {
			switch {
			case foundation && i == len(m.Levels)-1:
				names = append(names, "House Foundation")
			case level.Kind == patterns.HousePillars:
				names = append(names, "Pillar "+c.Title)
			default:
				names = append(names, "Floor "+c.Title)
			}
		}
	}
	return names
}

// pptxEscapeAttr escapes a shape name for an XML attribute.
func pptxEscapeAttr(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// houseDiagramEstimateShapeCount returns the estimated number of shapes for ID allocation:
// the group, one shape per panel, and one cap per pillar.
func houseDiagramEstimateShapeCount(panels []nativePanelData) uint32 {
	return uint32(1 + 2*len(panels))
}
