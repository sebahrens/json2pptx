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
}

// DiagramCapability describes the rendering limits and behavior for a diagram type.
// Values are derived from the actual pattern validators, shape generators,
// and SVG renderers across internal/patterns/, internal/generator/, and svggen/.
type DiagramCapability struct {
	Type             string   `json:"type"`
	MaxNodes         *int     `json:"max_nodes"`
	MaxDepth         *int     `json:"max_depth"`
	OverflowBehavior *string  `json:"overflow_behavior"`
	RequiredFields   []string `json:"required_fields"`
	OptionalFields   []string `json:"optional_fields"`
	Status           string   `json:"status"`
}

// CapabilitiesTBD returns true if any chart or diagram capability still
// has Status "tbd", indicating values have not been researched yet.
func CapabilitiesTBD() bool {
	for _, c := range ChartCapabilities() {
		if c.Status == "tbd" {
			return true
		}
	}
	for _, d := range DiagramCapabilities() {
		if d.Status == "tbd" {
			return true
		}
	}
	return false
}

// helpers to create pointers for literal values.
func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

// ChartCapabilities returns capability metadata for all known chart types.
// Limits reflect core/limits.go constants (MaxSeries=50, MaxCategories=200,
// MaxPoints=5000) and per-chart renderer behavior.
func ChartCapabilities() []ChartCapability {
	return []ChartCapability{
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
}

// DiagramCapabilities returns capability metadata for all known diagram types.
// Limits reflect pattern validators, native shape generators, and SVG renderers.
func DiagramCapabilities() []DiagramCapability {
	return []DiagramCapability{
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
			Status:           "stub",
		},
		{
			Type:             "heatmap",
			MaxNodes:         intPtr(200),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("cell labels omitted when cells too small"),
			RequiredFields:   []string{"values", "row_labels", "col_labels"},
			OptionalFields:   nil,
			Status:           "stub",
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
			Status:           "stub",
		},
		{
			Type:             "panel_layout",
			MaxNodes:         intPtr(12),
			MaxDepth:         intPtr(1),
			OverflowBehavior: strPtr("font reduction for many panels"),
			RequiredFields:   []string{"panels"},
			OptionalFields:   []string{"layout", "title", "body", "icon", "color"},
			Status:           "stub",
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
}
