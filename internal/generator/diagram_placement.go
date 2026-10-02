package generator

import (
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// DiagramPlacementInfo describes the render pipeline and placement support for
// a diagram type. It is derived from the renderer dispatch itself — the
// native-diagram set the placeholder and region paths both route through the
// bounded adapter, and the svggen registry — so get_diagram_capabilities,
// recommend_visual's placement guidance and validate's grid checks cannot
// advertise a placement generation does not draw (go-slide-creator-6x9bt).
type DiagramPlacementInfo struct {
	// PlaceholderPipeline is the render strategy when the diagram is placed in a
	// standard content placeholder: "native_ooxml" or "svg".
	PlaceholderPipeline string

	// GridCellPipeline is the render strategy when the diagram is placed inside a
	// shape_grid cell or compose segment. Empty string means the diagram is not
	// supported in grid cells.
	GridCellPipeline string

	// AuthoringSurface describes which pipeline owns the implementation:
	// "native_ooxml" (rendered through internal/generator as grouped OOXML
	// shapes) or "svggen" (rendered through the svggen registry as SVG).
	// This mirrors the values exposed on svggen.DiagramCapability /
	// svggen.ChartCapability so agents see a consistent vocabulary across
	// list_templates, get_diagram_capabilities, and get_chart_capabilities.
	AuthoringSurface string
}

// DiagramGridPipeline is the pipeline a shape_grid cell or compose segment
// dispatches spec to: "native_ooxml" for the native types (drawn through the
// same bounded adapter as the placeholder path), "svg" for a type registered
// in svggen, or "" when no renderer owns it.
func DiagramGridPipeline(spec *types.DiagramSpec) string {
	if spec == nil || spec.Type == "" {
		return ""
	}
	if IsGridNativeDiagram(spec) {
		return "native_ooxml"
	}
	if svggen.DefaultRegistry().Get(spec.Type) != nil {
		return "svg"
	}
	return ""
}

// diagramPlaceholderPipeline is the pipeline processDiagramContent dispatches
// spec to in a body placeholder, or "" when no renderer owns it.
func diagramPlaceholderPipeline(spec *types.DiagramSpec) string {
	if spec == nil || spec.Type == "" {
		return ""
	}
	if IsNativeDiagramType(spec) {
		return "native_ooxml"
	}
	if svggen.DefaultRegistry().Get(spec.Type) != nil {
		return "svg"
	}
	return ""
}

// DiagramPlacementFor returns the placement info for a diagram type, derived
// from the renderer dispatch, or nil when no renderer owns the type.
func DiagramPlacementFor(diagramType string) *DiagramPlacementInfo {
	spec := &types.DiagramSpec{Type: diagramType}
	placeholder := diagramPlaceholderPipeline(spec)
	if placeholder == "" {
		return nil
	}
	info := &DiagramPlacementInfo{
		PlaceholderPipeline: placeholder,
		GridCellPipeline:    DiagramGridPipeline(spec),
		AuthoringSurface:    "svggen",
	}
	if placeholder == "native_ooxml" {
		info.AuthoringSurface = "native_ooxml"
	}
	return info
}

// ApplyPlacementMetadata enriches a slice of DiagramCapability with placement-aware
// fields derived from the renderer dispatch. A type no renderer owns gets no
// placements and grid_cell_support false. This is called by the MCP handler to
// merge type-level limits (from svggen) with placement truth (from
// internal/generator).
func ApplyPlacementMetadata(caps []svggen.DiagramCapability) []svggen.DiagramCapability {
	result := make([]svggen.DiagramCapability, len(caps))
	copy(result, caps)
	for i := range result {
		info := DiagramPlacementFor(result[i].Type)
		if info == nil {
			result[i].Placements = nil
			result[i].GridCellSupport = boolP(false)
			continue
		}
		result[i].Placements = []svggen.DiagramPlacement{
			{Context: "placeholder", Pipeline: info.PlaceholderPipeline},
		}
		if info.GridCellPipeline != "" {
			result[i].Placements = append(result[i].Placements, svggen.DiagramPlacement{
				Context:  "shape_grid",
				Pipeline: info.GridCellPipeline,
			})
			result[i].GridCellSupport = boolP(true)
		} else {
			result[i].GridCellSupport = boolP(false)
		}
		result[i].AuthoringSurface = strP(info.AuthoringSurface)
	}
	return result
}

func boolP(v bool) *bool    { return &v }
func strP(v string) *string { return &v }
