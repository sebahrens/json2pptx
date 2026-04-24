package svggen

// ChartCapability describes the capability shape for a chart type.
// Values are initially nil / "tbd" — the shape ships first; concrete
// values are a separate P3 epic requiring per-chart design calls.
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

// DiagramCapability describes the capability shape for a diagram type.
// Values are initially nil / "tbd" — the shape ships first; concrete
// values are a separate P3 epic requiring per-diagram design calls.
type DiagramCapability struct {
	Type             string   `json:"type"`
	MaxNodes         *int     `json:"max_nodes"`
	MaxDepth         *int     `json:"max_depth"`
	OverflowBehavior *string  `json:"overflow_behavior"`
	RequiredFields   []string `json:"required_fields"`
	OptionalFields   []string `json:"optional_fields"`
	Status           string   `json:"status"`
}

// ChartCapabilities returns the capability shape for all known chart types.
// All capability values are nil (TBD); the struct shape is the deliverable.
func ChartCapabilities() []ChartCapability {
	types := []string{
		"bar", "line", "pie", "donut", "area", "radar", "scatter",
		"stacked_bar", "bubble", "stacked_area", "grouped_bar",
		"waterfall", "funnel", "gauge", "treemap",
	}
	caps := make([]ChartCapability, len(types))
	for i, t := range types {
		caps[i] = ChartCapability{
			Type:   t,
			Status: "tbd",
		}
	}
	return caps
}

// DiagramCapabilities returns the capability shape for all known diagram types.
// All capability values are nil (TBD); the struct shape is the deliverable.
func DiagramCapabilities() []DiagramCapability {
	types := []string{
		"timeline", "process_flow", "pyramid", "venn", "swot",
		"org_chart", "gantt", "matrix_2x2", "porters_five_forces",
		"house_diagram", "business_model_canvas", "value_chain",
		"nine_box_talent", "kpi_dashboard", "heatmap", "fishbone",
		"pestel", "panel_layout", "icon_columns", "icon_rows", "stat_cards",
	}
	caps := make([]DiagramCapability, len(types))
	for i, t := range types {
		caps[i] = DiagramCapability{
			Type:   t,
			Status: "tbd",
		}
	}
	return caps
}
