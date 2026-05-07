package generator

import "github.com/sebahrens/json2pptx/svggen"

// DiagramPlacementInfo describes the render pipeline and placement support for
// a diagram type. This is the single canonical source of truth for placement-aware
// render behavior, derived from the native-intercept checks in media.go and the
// grid-cell SVG path in shapegrid/.
type DiagramPlacementInfo struct {
	// PlaceholderPipeline is the render strategy when the diagram is placed in a
	// standard content placeholder: "native_ooxml" or "svg".
	PlaceholderPipeline string

	// GridCellPipeline is the render strategy when the diagram is placed inside a
	// shape_grid cell. Empty string means the diagram is not supported in grid cells.
	GridCellPipeline string

	// AuthoringSurface describes where the primary implementation lives:
	// "internal/generator" for native OOXML, "svggen" for SVG renderers.
	AuthoringSurface string
}

// diagramPlacementRegistry is the canonical static mapping from diagram type to
// placement-aware render truth. Each entry reflects the actual code path in
// processDiagramContent (media.go) and the shape_grid diagram cell path.
//
// Native-intercepted types: the is*Diagram() guards in media.go route these to
// process*NativeShapes() before the SVG path. In shape_grid cells, these same
// types fall through to the SVG renderer because grid cells don't support native
// OOXML grouped shapes.
//
// SVG-only types: these always render via svggen, both in placeholders and grid cells.
var diagramPlacementRegistry = map[string]DiagramPlacementInfo{
	// --- Native-intercepted in placeholder, SVG in grid cells ---
	"panel_layout":          {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"swot":                  {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"pestel":                {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"nine_box_talent":       {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"value_chain":           {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"kpi_dashboard":         {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"porters_five_forces":   {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"business_model_canvas": {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"process_flow":          {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"heatmap":               {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"pyramid":               {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},
	"house_diagram":         {PlaceholderPipeline: "native_ooxml", GridCellPipeline: "svg", AuthoringSurface: "internal/generator"},

	// --- SVG-only in both placements ---
	"timeline":     {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"venn":         {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"org_chart":    {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"gantt":        {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"matrix_2x2":   {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"fishbone":     {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"icon_columns": {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"icon_rows":    {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
	"stat_cards":   {PlaceholderPipeline: "svg", GridCellPipeline: "svg", AuthoringSurface: "svggen"},
}

// DiagramPlacementFor returns the placement info for a diagram type, or nil if unknown.
func DiagramPlacementFor(diagramType string) *DiagramPlacementInfo {
	info, ok := diagramPlacementRegistry[diagramType]
	if !ok {
		return nil
	}
	return &info
}

// ApplyPlacementMetadata enriches a slice of DiagramCapability with placement-aware
// fields from the canonical registry. Capabilities without a registry entry get
// a default SVG-only placement. This is called by the MCP handler to merge type-level
// limits (from svggen) with placement truth (from internal/generator).
func ApplyPlacementMetadata(caps []svggen.DiagramCapability) []svggen.DiagramCapability {
	result := make([]svggen.DiagramCapability, len(caps))
	copy(result, caps)
	for i := range result {
		info := DiagramPlacementFor(result[i].Type)
		if info == nil {
			// Unknown type: assume SVG-only, grid-cell supported.
			result[i].Placements = []svggen.DiagramPlacement{
				{Context: "placeholder", Pipeline: "svg"},
				{Context: "shape_grid", Pipeline: "svg"},
			}
			result[i].GridCellSupport = boolP(true)
			result[i].AuthoringSurface = strP("svggen")
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
