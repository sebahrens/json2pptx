package svggen

// ChartCapability describes the rendering limits and behavior for a chart type.
// Values are derived from the actual renderer constants and adaptive strategies
// in svggen/core/limits.go, charts.go, waterfall.go, funnel.go, gauge.go, treemap.go.
type ChartCapability struct {
	Type              string  `json:"type"`
	MaxSeries         *int    `json:"max_series"`
	MaxPoints         *int    `json:"max_points"`
	MaxCategories     *int    `json:"max_categories"`
	SupportsNegatives *bool   `json:"supports_negatives"`
	SupportsLogScale  *bool   `json:"supports_log_scale"`
	LabelStrategy     *string `json:"label_strategy"`
	DensityBehavior   *string `json:"density_behavior"`
	Status            string  `json:"status"`
	// AuthoringSurface is the pipeline that owns this type's implementation:
	// "svggen" (rendered through the svggen registry) or "native_ooxml"
	// (rendered through internal/generator as grouped OOXML shapes). All chart
	// types currently render via svggen.
	AuthoringSurface *string `json:"authoring_surface,omitempty"`
	// Aliases lists other accepted names for this chart type. svggen's
	// registry resolves any of these to the same underlying renderer (e.g.,
	// "bar" and "bar_chart" both produce a bar chart). The value in Type is
	// the form documented in this capabilities response; Aliases enumerates
	// the equivalents agents may also send.
	Aliases []string `json:"aliases,omitempty"`
}

// DiagramPlacement describes a supported placement context and its render pipeline.
type DiagramPlacement struct {
	// Context is "placeholder" (standard content placeholder) or "shape_grid" (grid cell).
	Context  string `json:"context"`
	// Pipeline is the render strategy: "native_ooxml" (grouped shapes) or "svg" (SVG/PNG raster).
	Pipeline string `json:"pipeline"`
}

// DiagramCapability describes the rendering limits and behavior for a diagram type.
// Values are derived from the actual pattern validators, shape generators,
// and SVG renderers across internal/patterns/, internal/generator/, and svggen/.
type DiagramCapability struct {
	Type             string             `json:"type"`
	MaxNodes         *int               `json:"max_nodes"`
	MaxDepth         *int               `json:"max_depth"`
	OverflowBehavior *string            `json:"overflow_behavior"`
	RequiredFields   []string           `json:"required_fields"`
	OptionalFields   []string           `json:"optional_fields"`
	Status           string             `json:"status"`
	Placements       []DiagramPlacement `json:"placements,omitempty"`
	GridCellSupport  *bool              `json:"grid_cell_support,omitempty"`
	AuthoringSurface *string            `json:"authoring_surface,omitempty"`
	// Aliases lists other accepted names for this diagram type registered in
	// svggen (e.g., "org" / "orgchart" both resolve to "org_chart"). Omitted
	// for native_ooxml types that don't go through the svggen registry.
	Aliases []string `json:"aliases,omitempty"`
}

// helpers to create pointers for literal values.
func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

// ChartCapabilities returns capability metadata for all known chart types.
// Limits reflect core/limits.go constants (MaxSeries=50, MaxCategories=200,
// MaxPoints=5000) and per-chart renderer behavior.
//
// Every entry has AuthoringSurface set to "svggen" — chart rendering lives in
// the svggen registry. Use this metadata to predict whether a chart type is
// reachable without round-tripping through render.
func ChartCapabilities() []ChartCapability {
	caps := []ChartCapability{
		{
			Type:              "bar",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(true),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories; auto log-scale at 1000x range"),
			Status:            "ready",
		},
		{
			Type:              "grouped_bar",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(true),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories; auto log-scale at 1000x range"),
			Status:            "ready",
		},
		{
			Type:              "stacked_bar",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories; log-scale disabled for stacked"),
			Status:            "ready",
		},
		{
			Type:              "line",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories"),
			Status:            "ready",
		},
		{
			Type:              "area",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories"),
			Status:            "ready",
		},
		{
			Type:              "stacked_area",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate"),
			DensityBehavior:   strPtr("label thinning at 15+ categories; log-scale disabled for stacked"),
			Status:            "ready",
		},
		{
			Type:              "pie",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(false),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("inside name+pct; outside pct-only; min 2° sweep to show label"),
			DensityBehavior:   strPtr("dynamic radius scaling for label fit; legend overflow capped at 45% height"),
			Status:            "ready",
		},
		{
			Type:              "donut",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(false),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("inside name+pct; outside pct-only; min 2° sweep to show label"),
			DensityBehavior:   strPtr("dynamic radius scaling for label fit; legend overflow capped at 45% height"),
			Status:            "ready",
		},
		{
			Type:              "scatter",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("point labels with smart alignment; font reduced for overlaps"),
			DensityBehavior:   strPtr("labels clamped to viewBox; no truncation"),
			Status:            "ready",
		},
		{
			Type:              "bubble",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("point labels with smart alignment; font reduced for overlaps"),
			DensityBehavior:   strPtr("bubble size range 4-20pt; labels clamped to viewBox"),
			Status:            "ready",
		},
		{
			Type:              "radar",
			MaxSeries:         intPtr(50),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(false),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("radial axis labels; word-split; min 8pt font"),
			DensityBehavior:   strPtr("radius reduced at 8/12/16+ axes (80%/65%/60% of max)"),
			Status:            "ready",
		},
		{
			Type:              "waterfall",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("adapt: shrink→rotate→thin→truncate; total/subtotal labels always shown"),
			DensityBehavior:   strPtr("adaptive font 6+ points; broken-axis zoom; important labels exempt from thinning"),
			Status:            "ready",
		},
		{
			Type:              "funnel",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(false),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("inside labels; overflow to external right-side labels with connectors"),
			DensityBehavior:   strPtr("adaptive font+gap at 6+ segments (floor 7pt); plot area shrunk for external labels"),
			Status:            "ready",
		},
		{
			Type:              "gauge",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(1),
			MaxCategories:     intPtr(1),
			SupportsNegatives: boolPtr(true),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("radial tick labels; center value label"),
			DensityBehavior:   strPtr("single value display; threshold zones for context"),
			Status:            "ready",
		},
		{
			Type:              "treemap",
			MaxSeries:         intPtr(1),
			MaxPoints:         intPtr(5000),
			MaxCategories:     intPtr(200),
			SupportsNegatives: boolPtr(false),
			SupportsLogScale:  boolPtr(false),
			LabelStrategy:     strPtr("centered in cell; omitted if cell < LabelMinSize (12pt)"),
			DensityBehavior:   strPtr("squarify layout; labels hidden for small cells; recursive nesting supported"),
			Status:            "ready",
		},
	}
	for i := range caps {
		caps[i].AuthoringSurface = strPtr("svggen")
		if a := Aliases(caps[i].Type); len(a) > 0 {
			caps[i].Aliases = a
		}
	}
	return caps
}

// DiagramCapabilitiesReady returns capability metadata for diagram types with
// Status "ready". Stub/experimental types are excluded. Use this for
// agent-facing surfaces (MCP tools, skill-info) where advertising non-functional
// types erodes trust.
func DiagramCapabilitiesReady() []DiagramCapability {
	all := DiagramCapabilities()
	ready := make([]DiagramCapability, 0, len(all))
	for _, d := range all {
		if d.Status == "ready" {
			ready = append(ready, d)
		}
	}
	return ready
}

// diagramAuthoringSurface maps each known diagram type to the pipeline that
// owns its implementation. "svggen" types are registered in the svggen
// registry (svggen/init.go) and render as SVG. "native_ooxml" types are
// implemented in internal/generator/ as grouped OOXML shapes.
//
// Keep this map in sync with svggen/init.go (builtinDiagrams) and
// internal/generator/diagram_placement.go (diagramPlacementRegistry). The
// table-driven test in capabilities_test.go enforces the svggen side of this
// invariant: every "svggen" entry must resolve in DefaultRegistry().Get(); no
// "native_ooxml" entry may resolve there.
var diagramAuthoringSurface = map[string]string{
	"timeline":              "svggen",
	"venn":                  "svggen",
	"org_chart":             "svggen",
	"gantt":                 "svggen",
	"matrix_2x2":            "svggen",
	"fishbone":              "svggen",
	"process_flow":          "native_ooxml",
	"pyramid":               "native_ooxml",
	"swot":                  "native_ooxml",
	"porters_five_forces":   "native_ooxml",
	"house_diagram":         "native_ooxml",
	"business_model_canvas": "native_ooxml",
	"value_chain":           "native_ooxml",
	"nine_box_talent":       "native_ooxml",
	"kpi_dashboard":         "native_ooxml",
	"heatmap":               "native_ooxml",
	"pestel":                "native_ooxml",
	"panel_layout":          "native_ooxml",
	// icon_columns / icon_rows / stat_cards are layout-mode aliases for
	// panel_layout. Their implementation lives in internal/generator/panel_shapes.go.
	"icon_columns": "native_ooxml",
	"icon_rows":    "native_ooxml",
	"stat_cards":   "native_ooxml",
}

// DiagramCapabilities returns capability metadata for all known diagram types,
// including stubs. For agent-facing surfaces, prefer DiagramCapabilitiesReady.
//
// Every entry has AuthoringSurface populated from diagramAuthoringSurface; an
// unmapped entry triggers a panic at init time via the table-driven test, so
// adding a new diagram type forces a deliberate authoring-surface choice.
func DiagramCapabilities() []DiagramCapability {
	caps := []DiagramCapability{
		{
			Type:             "timeline",
			MaxNodes:         intPtr(7),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("error if <3 or >7 stops; suggest split across slides"),
			RequiredFields:   []string{"values"},
			OptionalFields:   []string{"date", "body", "accent", "connector"},
			Status:           "ready",
		},
		{
			Type:             "process_flow",
			MaxNodes:         intPtr(50),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for density"),
			RequiredFields:   []string{"steps"},
			OptionalFields:   []string{"description", "connections"},
			Status:           "ready",
		},
		{
			Type:             "pyramid",
			MaxNodes:         intPtr(20),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("error if >20 levels; font reduction at 8+"),
			RequiredFields:   []string{"levels"},
			OptionalFields:   []string{"description", "gap", "top_width_ratio"},
			Status:           "ready",
		},
		{
			Type:             "venn",
			MaxNodes:         intPtr(3),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("text wrapping and truncation within circles"),
			RequiredFields:   []string{"circles"},
			OptionalFields:   []string{"intersections", "circle_opacity", "overlap_ratio"},
			Status:           "ready",
		},
		{
			Type:             "swot",
			MaxNodes:         intPtr(4),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for dense bullet lists"),
			RequiredFields:   []string{"strengths", "weaknesses", "opportunities", "threats"},
			OptionalFields:   []string{"footnote"},
			Status:           "ready",
		},
		{
			Type:             "org_chart",
			MaxNodes:         intPtr(50),
			MaxDepth:         intPtr(20),
			OverflowBehavior: strPtr("siblings >9 collapsed to +N more indicator"),
			RequiredFields:   []string{"root"},
			OptionalFields:   []string{"title", "children", "node_width", "node_height", "max_visible_siblings"},
			Status:           "ready",
		},
		{
			Type:             "gantt",
			MaxNodes:         intPtr(50),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction; task name truncation with ellipsis"),
			RequiredFields:   []string{"tasks"},
			OptionalFields:   []string{"milestones", "time_unit", "show_progress", "footnote"},
			Status:           "ready",
		},
		{
			Type:             "matrix_2x2",
			MaxNodes:         intPtr(4),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("fixed 2x2 grid; body text truncated at 200 chars"),
			RequiredFields:   nil,
			OptionalFields:   []string{"x_axis_label", "y_axis_label", "top_left", "top_right", "bottom_left", "bottom_right"},
			Status:           "ready",
		},
		{
			Type:             "porters_five_forces",
			MaxNodes:         intPtr(5),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("fixed 5-force layout; text wrapping within force regions"),
			RequiredFields:   nil,
			OptionalFields:   []string{"industry_name", "forces", "rivalry", "new_entrants", "substitutes", "suppliers", "buyers"},
			Status:           "ready",
		},
		{
			Type:             "house_diagram",
			MaxNodes:         intPtr(10),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for dense sections"),
			RequiredFields:   nil,
			OptionalFields:   []string{"roof", "sections", "foundation", "footnote"},
			Status:           "ready",
		},
		{
			Type:             "business_model_canvas",
			MaxNodes:         intPtr(9),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("fixed 9-cell layout; 1-10 bullets per section; items >200 chars truncated"),
			RequiredFields:   []string{"key_partners", "key_activities", "key_resources", "value_propositions", "customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"},
			OptionalFields:   nil,
			Status:           "ready",
		},
		{
			Type:             "value_chain",
			MaxNodes:         intPtr(20),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for many items"),
			RequiredFields:   nil,
			OptionalFields:   []string{"primary", "support", "margin_label", "show_arrows"},
			Status:           "ready",
		},
		{
			Type:             "nine_box_talent",
			MaxNodes:         intPtr(50),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("dot collision; text truncation for overlapping badges"),
			RequiredFields:   []string{"employees"},
			OptionalFields:   []string{"title", "x_label", "y_label", "cells"},
			Status:           "ready",
		},
		{
			Type:             "kpi_dashboard",
			MaxNodes:         intPtr(12),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("card grid layout; font reduction for many metrics"),
			RequiredFields:   []string{"metrics"},
			OptionalFields:   []string{"label", "value", "unit", "change", "trend"},
			Status:           "ready",
		},
		{
			Type:             "heatmap",
			MaxNodes:         intPtr(200),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("cell labels omitted when cells too small"),
			RequiredFields:   []string{"values", "row_labels", "col_labels"},
			OptionalFields:   nil,
			Status:           "ready",
		},
		{
			Type:             "fishbone",
			MaxNodes:         intPtr(10),
			MaxDepth:         intPtr(2),
			OverflowBehavior: strPtr("categories >10 collapsed to overflow indicator"),
			RequiredFields:   []string{"effect"},
			OptionalFields:   []string{"categories"},
			Status:           "ready",
		},
		{
			Type:             "pestel",
			MaxNodes:         intPtr(6),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("fixed 6-segment layout"),
			RequiredFields:   nil,
			OptionalFields:   []string{"political", "economic", "social", "technological", "environmental", "legal"},
			Status:           "ready",
		},
		{
			Type:             "panel_layout",
			MaxNodes:         intPtr(12),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for many panels"),
			RequiredFields:   []string{"panels"},
			OptionalFields:   []string{"layout", "title", "body", "icon", "color"},
			Status:           "ready",
		},
		{
			Type:             "icon_columns",
			MaxNodes:         intPtr(5),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("error if <3 or >5 items"),
			RequiredFields:   []string{"values"},
			OptionalFields:   []string{"icon", "caption", "accent"},
			Status:           "ready",
		},
		{
			Type:             "icon_rows",
			MaxNodes:         intPtr(5),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("error if <3 or >5 items"),
			RequiredFields:   []string{"values"},
			OptionalFields:   []string{"icon", "caption", "accent"},
			Status:           "ready",
		},
		{
			Type:             "stat_cards",
			MaxNodes:         intPtr(4),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("fixed 3-4 card layout"),
			RequiredFields:   []string{"values"},
			OptionalFields:   []string{"big", "small", "accent"},
			Status:           "ready",
		},
	}
	for i := range caps {
		if surface, ok := diagramAuthoringSurface[caps[i].Type]; ok {
			s := surface
			caps[i].AuthoringSurface = &s
		}
		// Aliases only meaningful for svggen-registered types.
		if caps[i].AuthoringSurface != nil && *caps[i].AuthoringSurface == "svggen" {
			if a := Aliases(caps[i].Type); len(a) > 0 {
				caps[i].Aliases = a
			}
		}
	}
	return caps
}

// naturalDiagramAspects maps non-chart diagram types to their natural width/height
// ratio (W/H) as defined by the renderer's intrinsic canvas. Only diagrams that
// use a fixed natural aspect via RenderWithHelperDimensions are listed; chart
// types that fit their container (via RenderWithHelper) intentionally have no
// entry because they have no intrinsic aspect to conflict with.
//
// Values are sourced from the renderer constants:
//   - timeline:  RenderWithHelperDimensions(req, 800, 400, ...) → 2.0
//   - gantt:     RenderWithHelperDimensions(req, 900, 500, ...) → 1.8
//   - org_chart: RenderWithHelperDimensions(req, 1100, 700, ...) → ~1.57 (data adjusts)
//
// Keep this in sync with the per-diagram defaults in svggen/timeline.go,
// svggen/gantt.go, and svggen/org_chart.go.
var naturalDiagramAspects = map[string]float64{
	"timeline":  2.0,
	"gantt":     1.8,
	"org_chart": 1100.0 / 700.0,
}

// NaturalAspect returns the natural width-to-height aspect ratio of a known
// non-chart diagram type. Returns 0 when the type either fits its container
// (most charts) or is not recognized. Aliases are resolved before lookup, so
// "org" and "orgchart" both resolve to the org_chart aspect.
//
// Callers use this at validate / preview time to predict whether a target cell
// or placeholder shape will conflict with a diagram's intrinsic aspect, which
// lets agents fix the layout before invoking resvg-convert or rasterising.
func NaturalAspect(diagramType string) float64 {
	canonical := diagramType
	if alias, ok := builtinAliases[diagramType]; ok {
		canonical = alias
	}
	return naturalDiagramAspects[canonical]
}
