package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Native framework diagrams inside a shape_grid cell (go-slide-creator-ngbnf).
//
// A SWOT or a five-forces diagram is drawn as OOXML shapes, and until now only
// into a body placeholder: the placeholder path replaced the placeholder shape
// with the diagram group. That pinned both to a layout with a body placeholder,
// while every pattern, table and org slide renders on blank-title — so on a
// template whose content layout styles its title differently (abstract), the
// title jumped mid-deck. A grid cell is just a rectangle, which is all these
// builders ever needed: the same group XML is emitted at the cell's bounds, so
// the diagram can ride in a one-cell grid on blank-title like everything else.

// gridNativeDiagramShapeIDs is how many shape IDs each grid-capable native
// diagram consumes: the group plus its children.
var gridNativeDiagramShapeIDs = map[string]int{
	"swot":                9,  // 1 group + 4 quadrants × (header + body)
	"porters_five_forces": 10, // 1 group + 5 force boxes + 4 connectors
}

// IsGridNativeDiagram reports whether a diagram spec renders as native OOXML
// shapes when it sits in a shape_grid cell, instead of as an svggen picture.
func IsGridNativeDiagram(spec *types.DiagramSpec) bool {
	if spec == nil {
		return false
	}
	_, ok := gridNativeDiagramShapeIDs[spec.Type]
	return ok
}

// GridNativeDiagramShapeIDs returns how many consecutive shape IDs
// GenerateGridNativeDiagramXML needs for spec, so the caller can reserve them.
func GridNativeDiagramShapeIDs(spec *types.DiagramSpec) int {
	if spec == nil {
		return 0
	}
	return gridNativeDiagramShapeIDs[spec.Type]
}

// GenerateGridNativeDiagramXML renders a grid-capable native diagram as one
// <p:grpSp> at bounds. idBase is the first of GridNativeDiagramShapeIDs(spec)
// consecutive shape IDs the caller has reserved. alt becomes the group's
// description. It renders exactly what the placeholder path renders for the
// same spec, sized to the cell instead of to a placeholder.
func GenerateGridNativeDiagramXML(spec *types.DiagramSpec, bounds types.BoundingBox, idBase uint32, themeColors []types.ThemeColor, fontName, alt string) (string, error) {
	if !IsGridNativeDiagram(spec) {
		return "", fmt.Errorf("diagram type %q has no native grid renderer", diagramTypeName(spec))
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return "", fmt.Errorf("%s: empty cell bounds", spec.Type)
	}

	var xml string
	switch spec.Type {
	case "swot":
		panels := swotPanels(spec)
		// Content-size the quadrants the way the placeholder path does, so a
		// sparse SWOT is centred rather than stretched over the whole cell.
		bounds = fitBoundsToFramework(bounds, nativeFrameworkSize("swot", bounds, panels, houseDiagramMeta{}, fontName))
		xml = generateSWOTGroupXML(panels, bounds, idBase, taxonomyPalette(spec, 4, swotDefaultTint))
	case "porters_five_forces":
		panels := porterPanels(spec)
		if len(panels) == 0 {
			return "", fmt.Errorf("porters_five_forces: no forces parsed")
		}
		xml = generatePortersFiveGroupXML(panels, bounds, idBase, themeColors)
	}
	if xml == "" {
		return "", fmt.Errorf("%s: native shape generation failed", spec.Type)
	}
	return pptx.SetGroupDescription(xml, alt), nil
}

func diagramTypeName(spec *types.DiagramSpec) string {
	if spec == nil {
		return ""
	}
	return spec.Type
}
