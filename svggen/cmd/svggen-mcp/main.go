// Package main provides an MCP (Model Context Protocol) server for svggen diagram rendering.
//
// Usage:
//
//	svggen-mcp              # Start MCP server over stdio
//	svggen-mcp --version    # Print version
//
// The server exposes four tools:
//   - render_diagram: Render a diagram to SVG or PNG
//   - list_diagram_types: List all available diagram types
//   - validate_diagram: Validate diagram input without rendering
//   - get_diagram_schema: Get the data schema for a specific diagram type
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	// Import root package to auto-register all diagram types via init().
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/core"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "version":
			fmt.Println("svggen-mcp " + version)
			return
		case "--help", "-h", "help":
			fmt.Fprintln(os.Stderr, "Usage: svggen-mcp [--version|--help]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Start an MCP server over stdio for SVG diagram rendering.")
			return
		}
	}

	// Logging goes to stderr so stdio transport stays clean.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	s := server.NewMCPServer(
		"svggen",
		version,
		server.WithToolCapabilities(false),
	)

	s.AddTool(renderDiagramTool(), handleRenderDiagram)
	s.AddTool(listDiagramTypesTool(), handleListDiagramTypes)
	s.AddTool(validateDiagramTool(), handleValidateDiagram)
	s.AddTool(getDiagramSchemaTool(), handleGetDiagramSchema)

	slog.Info("starting svggen MCP server", "version", version)

	return server.ServeStdio(s)
}

// --- Tool definitions ---

func renderDiagramTool() mcp.Tool {
	return mcp.NewTool("render_diagram",
		mcp.WithDescription("Render a diagram or chart to SVG or PNG format. Supports 30+ diagram types including bar_chart, line_chart, pie_chart, org_chart, gantt, timeline, funnel, radar, scatter, bubble, waterfall, heatmap, treemap, venn, swot, matrix_2x2, fishbone, and more."),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Diagram type (e.g., bar_chart, line_chart, pie_chart, org_chart, gantt, timeline, funnel, radar_chart, scatter_chart, bubble_chart, waterfall, heatmap, treemap, venn, swot, matrix_2x2, fishbone, pyramid, value_chain, porters_five_forces, pestel, nine_box_talent, business_model_canvas, gauge). Use list_diagram_types to see all available types."),
		),
		mcp.WithObject("data",
			mcp.Required(),
			mcp.Description("Diagram-specific data payload. Structure varies by type. Use get_diagram_schema for the expected format."),
		),
		mcp.WithString("format",
			mcp.Description("Output format: svg (default) or png."),
			mcp.Enum("svg", "png"),
		),
		mcp.WithNumber("width",
			mcp.Description("Output width in pixels (default: 800)."),
		),
		mcp.WithNumber("height",
			mcp.Description("Output height in pixels (default: 600)."),
		),
		mcp.WithString("title",
			mcp.Description("Diagram title (optional)."),
		),
		mcp.WithObject("style",
			mcp.Description("Optional style overrides. Supports palette (name or custom colors), font settings, etc."),
		),
	)
}

func listDiagramTypesTool() mcp.Tool {
	return mcp.NewTool("list_diagram_types",
		mcp.WithDescription("List all available diagram types that can be rendered."),
	)
}

func validateDiagramTool() mcp.Tool {
	return mcp.NewTool("validate_diagram",
		mcp.WithDescription("Validate diagram input data without rendering. Returns whether the input is valid and any validation errors."),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Diagram type to validate against."),
		),
		mcp.WithObject("data",
			mcp.Required(),
			mcp.Description("Diagram data to validate."),
		),
	)
}

func getDiagramSchemaTool() mcp.Tool {
	return mcp.NewTool("get_diagram_schema",
		mcp.WithDescription("Get the expected data schema and a minimal example for a specific diagram type. Use this to understand what data format a diagram type expects."),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Diagram type to get schema for."),
		),
	)
}

// --- Tool handlers ---

func handleRenderDiagram(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError("type is required"), nil
	}

	// Check diagram type exists
	reg := svggen.DefaultRegistry()
	d := reg.Get(diagramType)
	if d == nil {
		return mcp.NewToolResultError(fmt.Sprintf("unknown diagram type %q — use list_diagram_types to see available types", diagramType)), nil
	}

	// Extract data
	args := request.GetArguments()
	dataRaw, ok := args["data"]
	if !ok {
		return mcp.NewToolResultError("data is required"), nil
	}
	dataMap, ok := dataRaw.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("data must be a JSON object"), nil
	}

	// Build request envelope
	req := &svggen.RequestEnvelope{
		Type: diagramType,
		Data: dataMap,
	}

	// Optional title
	if title, err := request.RequireString("title"); err == nil && title != "" {
		req.Title = title
	}

	// Optional dimensions
	if w, ok := args["width"]; ok {
		if wf, ok := w.(float64); ok && wf > 0 {
			req.Output.Width = int(wf)
		}
	}
	if h, ok := args["height"]; ok {
		if hf, ok := h.(float64); ok && hf > 0 {
			req.Output.Height = int(hf)
		}
	}

	// Optional style — reject invalid payloads with a structured error.
	if styleRaw, ok := args["style"]; ok {
		styleMap, ok := styleRaw.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("style must be a JSON object"), nil
		}
		styleJSON, err := json.Marshal(styleMap)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("style: failed to encode: %v", err)), nil
		}
		var style svggen.StyleSpec
		if err := json.Unmarshal(styleJSON, &style); err != nil {
			errResult := finding{
				Pattern: diagramType,
				Path:    "style",
				Code:    "invalid_value",
				Message: fmt.Sprintf("invalid style payload: %v", err),
				Fix:     &fixSuggestion{Kind: "replace_value", Params: map[string]any{"field": "style"}},
			}
			output, _ := json.MarshalIndent(errResult, "", "  ")
			return mcp.NewToolResultError(string(output)), nil
		}
		req.Style = style
	}

	// Determine format
	format := "svg"
	if f, err := request.RequireString("format"); err == nil && f != "" {
		format = f
	}

	// Render
	result, err := svggen.RenderMultiFormat(req, format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("render failed: %v", err)), nil
	}

	switch format {
	case "svg":
		if result.SVG == nil {
			return mcp.NewToolResultError("no SVG output generated"), nil
		}
		return mcp.NewToolResultText(string(result.SVG.Content)), nil

	case "png":
		if result.PNG == nil {
			return mcp.NewToolResultError("no PNG output generated"), nil
		}
		encoded := base64.StdEncoding.EncodeToString(result.PNG)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.ImageContent{
					Type:     "image",
					Data:     encoded,
					MIMEType: "image/png",
				},
			},
		}, nil

	default:
		return mcp.NewToolResultError(fmt.Sprintf("unsupported format %q", format)), nil
	}
}

func handleListDiagramTypes(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	types := svggen.Types()
	sort.Strings(types)

	output, err := json.MarshalIndent(types, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal types: %v", err)), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

func handleValidateDiagram(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError("type is required"), nil
	}

	reg := svggen.DefaultRegistry()
	d := reg.Get(diagramType)
	if d == nil {
		return mcp.NewToolResultError(fmt.Sprintf("unknown diagram type %q", diagramType)), nil
	}

	args := request.GetArguments()
	dataRaw, ok := args["data"]
	if !ok {
		return mcp.NewToolResultError("data is required"), nil
	}
	dataMap, ok := dataRaw.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("data must be a JSON object"), nil
	}

	req := &svggen.RequestEnvelope{
		Type: diagramType,
		Data: dataMap,
	}

	type validationResult struct {
		Valid  bool      `json:"valid"`
		Errors []finding `json:"errors,omitempty"`
	}

	// Validate envelope
	if err := req.Validate(); err != nil {
		errs := svggen.GetValidationErrors(err)
		var findings []finding
		if len(errs) > 0 {
			findings = convertValidationErrors(diagramType, errs)
		} else {
			// Non-structured error from envelope validation — wrap it.
			findings = []finding{{
				Pattern: diagramType,
				Path:    "data",
				Code:    "parse_failed",
				Message: err.Error(),
			}}
		}
		output, _ := json.MarshalIndent(validationResult{
			Valid:  false,
			Errors: findings,
		}, "", "  ")
		return mcp.NewToolResultText(string(output)), nil
	}

	// Validate against diagram-specific rules
	if err := d.Validate(req); err != nil {
		errs := svggen.GetValidationErrors(err)
		var findings []finding
		if len(errs) > 0 {
			findings = convertValidationErrors(diagramType, errs)
		} else {
			// Non-structured error — wrap it.
			findings = []finding{{
				Pattern: diagramType,
				Path:    "data",
				Code:    "invalid_value",
				Message: err.Error(),
			}}
		}
		output, _ := json.MarshalIndent(validationResult{
			Valid:  false,
			Errors: findings,
		}, "", "  ")
		return mcp.NewToolResultText(string(output)), nil
	}

	output, _ := json.MarshalIndent(validationResult{Valid: true}, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleGetDiagramSchema(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("type")
	if err != nil {
		return mcp.NewToolResultError("type is required"), nil
	}

	reg := svggen.DefaultRegistry()
	d := reg.Get(diagramType)
	if d == nil {
		return mcp.NewToolResultError(fmt.Sprintf("unknown diagram type %q — use list_diagram_types to see available types", diagramType)), nil
	}

	// Build a minimal example by looking up known schemas.
	schema := getSchemaForType(diagramType)

	type schemaResult struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Example     any    `json:"example,omitempty"`
		DataSchema  any    `json:"data_schema,omitempty"`
	}

	result := schemaResult{
		Type:        diagramType,
		Description: schema.description,
		Example:     schema.example,
	}

	// Include the machine-readable data schema when the diagram provides one.
	if ds, ok := d.(svggen.DiagramWithSchema); ok {
		result.DataSchema = ds.DataSchema()
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

// diagramSchema holds description and example data for a diagram type.
type diagramSchema struct {
	description string
	example     any
}

// getSchemaForType returns a human-readable schema with example for known types.
func getSchemaForType(typ string) diagramSchema {
	schemas := map[string]diagramSchema{
		"bar_chart": {
			description: "Bar chart with categories and series. Supports grouped, stacked, and horizontal variants.",
			example: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{"name": "Revenue", "values": []any{100, 150, 120, 180}},
				},
			},
		},
		"line_chart": {
			description: "Line chart with categories and series. Supports multiple lines, area fill, and smooth curves.",
			example: map[string]any{
				"categories": []any{"Jan", "Feb", "Mar", "Apr"},
				"series": []any{
					map[string]any{"name": "Sales", "values": []any{40, 55, 70, 65}},
				},
			},
		},
		"pie_chart": {
			description: "Pie or donut chart with labeled slices.",
			example: map[string]any{
				"slices": []any{
					map[string]any{"label": "Product A", "value": 40},
					map[string]any{"label": "Product B", "value": 30},
					map[string]any{"label": "Product C", "value": 30},
				},
			},
		},
		"radar_chart": {
			description: "Radar/spider chart with axes and series.",
			example: map[string]any{
				"axes": []any{"Speed", "Power", "Range", "Defense", "Accuracy"},
				"series": []any{
					map[string]any{"name": "Player A", "values": []any{80, 70, 90, 60, 85}},
				},
			},
		},
		"scatter_chart": {
			description: "Scatter plot with x/y data points.",
			example: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Group A",
						"points": []any{
							map[string]any{"x": 10, "y": 20},
							map[string]any{"x": 30, "y": 40},
						},
					},
				},
			},
		},
		"bubble_chart": {
			description: "Bubble chart with x/y/size data points.",
			example: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Markets",
						"points": []any{
							map[string]any{"x": 10, "y": 20, "size": 30, "label": "US"},
							map[string]any{"x": 40, "y": 50, "size": 20, "label": "EU"},
						},
					},
				},
			},
		},
		"waterfall": {
			description: "Waterfall chart showing incremental changes to a total.",
			example: map[string]any{
				"items": []any{
					map[string]any{"label": "Revenue", "value": 500},
					map[string]any{"label": "COGS", "value": -200},
					map[string]any{"label": "OpEx", "value": -150},
					map[string]any{"label": "Profit", "value": 0, "is_total": true},
				},
			},
		},
		"org_chart": {
			description: "Organizational chart with hierarchical nodes.",
			example: map[string]any{
				"nodes": []any{
					map[string]any{"id": "ceo", "label": "CEO", "title": "John Smith"},
					map[string]any{"id": "vp1", "label": "VP Engineering", "parent": "ceo"},
					map[string]any{"id": "vp2", "label": "VP Sales", "parent": "ceo"},
				},
			},
		},
		"gantt": {
			description: "Gantt chart for project timelines with tasks and dependencies.",
			example: map[string]any{
				"tasks": []any{
					map[string]any{"id": "t1", "name": "Design", "start": "2024-01-01", "end": "2024-01-15"},
					map[string]any{"id": "t2", "name": "Develop", "start": "2024-01-15", "end": "2024-02-15", "depends_on": []any{"t1"}},
				},
			},
		},
		"timeline": {
			description: "Timeline with dated events.",
			example: map[string]any{
				"events": []any{
					map[string]any{"date": "2024-01", "title": "Project Start", "description": "Kicked off development"},
					map[string]any{"date": "2024-06", "title": "Beta Release"},
					map[string]any{"date": "2024-12", "title": "Launch"},
				},
			},
		},
		"funnel": {
			description: "Funnel chart showing progressive narrowing stages.",
			example: map[string]any{
				"stages": []any{
					map[string]any{"label": "Visitors", "value": 10000},
					map[string]any{"label": "Leads", "value": 5000},
					map[string]any{"label": "Qualified", "value": 2000},
					map[string]any{"label": "Closed", "value": 500},
				},
			},
		},
		"pyramid": {
			description: "Pyramid diagram with hierarchical layers.",
			example: map[string]any{
				"layers": []any{
					map[string]any{"label": "Self-Actualization"},
					map[string]any{"label": "Esteem"},
					map[string]any{"label": "Belonging"},
					map[string]any{"label": "Safety"},
					map[string]any{"label": "Physiological"},
				},
			},
		},
		"venn": {
			description: "Venn diagram with 2-4 overlapping circles.",
			example: map[string]any{
				"circles": []any{
					map[string]any{"label": "Set A", "items": []any{"a", "b", "c"}},
					map[string]any{"label": "Set B", "items": []any{"c", "d", "e"}},
				},
			},
		},
		"swot": {
			description: "SWOT analysis matrix (Strengths, Weaknesses, Opportunities, Threats).",
			example: map[string]any{
				"strengths":     []any{"Strong brand", "Loyal customers"},
				"weaknesses":    []any{"High costs", "Limited reach"},
				"opportunities": []any{"New markets", "Partnerships"},
				"threats":       []any{"Competition", "Regulation"},
			},
		},
		"matrix_2x2": {
			description: "2x2 matrix/quadrant diagram with labeled axes and items.",
			example: map[string]any{
				"x_axis": "Effort",
				"y_axis": "Impact",
				"items": []any{
					map[string]any{"label": "Quick Win", "x": 0.2, "y": 0.8},
					map[string]any{"label": "Major Project", "x": 0.8, "y": 0.9},
					map[string]any{"label": "Fill In", "x": 0.3, "y": 0.3},
				},
			},
		},
		"fishbone": {
			description: "Fishbone (Ishikawa) cause-and-effect diagram.",
			example: map[string]any{
				"effect": "Production Delays",
				"categories": []any{
					map[string]any{"name": "People", "causes": []any{"Training", "Staffing"}},
					map[string]any{"name": "Process", "causes": []any{"Bottleneck", "Handoffs"}},
					map[string]any{"name": "Technology", "causes": []any{"Downtime", "Legacy systems"}},
				},
			},
		},
		"heatmap": {
			description: "Heatmap with rows, columns, and values.",
			example: map[string]any{
				"rows":    []any{"Mon", "Tue", "Wed"},
				"columns": []any{"Morning", "Afternoon", "Evening"},
				"values":  []any{[]any{3, 7, 2}, []any{5, 9, 4}, []any{1, 6, 8}},
			},
		},
		"treemap": {
			description: "Treemap showing hierarchical data as nested rectangles.",
			example: map[string]any{
				"items": []any{
					map[string]any{"label": "Category A", "value": 60},
					map[string]any{"label": "Category B", "value": 30},
					map[string]any{"label": "Category C", "value": 10},
				},
			},
		},
		"gauge": {
			description: "Gauge/dial chart showing a single metric against a range.",
			example: map[string]any{
				"value":     75,
				"min":       0,
				"max":       100,
				"label":     "Performance",
				"unit":      "%",
				"thresholds": []any{30, 70},
			},
		},
		"value_chain": {
			description: "Porter's Value Chain diagram with primary and support activities.",
			example: map[string]any{
				"primary": []any{
					map[string]any{"label": "Inbound Logistics"},
					map[string]any{"label": "Operations"},
					map[string]any{"label": "Outbound Logistics"},
					map[string]any{"label": "Marketing & Sales"},
					map[string]any{"label": "Service"},
				},
				"support": []any{
					map[string]any{"label": "Infrastructure"},
					map[string]any{"label": "HR Management"},
					map[string]any{"label": "Technology"},
					map[string]any{"label": "Procurement"},
				},
			},
		},
		"porters_five_forces": {
			description: "Porter's Five Forces competitive analysis diagram.",
			example: map[string]any{
				"center": "Industry Rivalry",
				"forces": []any{
					map[string]any{"position": "top", "label": "Threat of New Entrants", "level": "high"},
					map[string]any{"position": "bottom", "label": "Threat of Substitutes", "level": "medium"},
					map[string]any{"position": "left", "label": "Supplier Power", "level": "low"},
					map[string]any{"position": "right", "label": "Buyer Power", "level": "high"},
				},
			},
		},
		"pestel": {
			description: "PESTEL analysis diagram covering Political, Economic, Social, Technological, Environmental, Legal factors.",
			example: map[string]any{
				"factors": []any{
					map[string]any{"category": "Political", "items": []any{"Regulation", "Trade policy"}},
					map[string]any{"category": "Economic", "items": []any{"GDP growth", "Inflation"}},
					map[string]any{"category": "Social", "items": []any{"Demographics", "Culture"}},
					map[string]any{"category": "Technological", "items": []any{"AI", "Automation"}},
					map[string]any{"category": "Environmental", "items": []any{"Climate", "Sustainability"}},
					map[string]any{"category": "Legal", "items": []any{"IP law", "Labor law"}},
				},
			},
		},
		"nine_box_talent": {
			description: "9-box talent grid with performance and potential axes.",
			example: map[string]any{
				"x_axis": "Performance",
				"y_axis": "Potential",
				"people": []any{
					map[string]any{"name": "Alice", "performance": "high", "potential": "high"},
					map[string]any{"name": "Bob", "performance": "medium", "potential": "high"},
				},
			},
		},
		"business_model_canvas": {
			description: "Business Model Canvas with 9 building blocks.",
			example: map[string]any{
				"key_partners":      []any{"Suppliers", "Distributors"},
				"key_activities":    []any{"Production", "Marketing"},
				"key_resources":     []any{"IP", "Staff"},
				"value_proposition": []any{"Quality", "Speed"},
				"customer_segments": []any{"B2B", "B2C"},
				"channels":          []any{"Online", "Retail"},
				"customer_relationships": []any{"Self-service", "Community"},
				"revenue_streams":   []any{"Subscriptions", "Licensing"},
				"cost_structure":    []any{"Fixed costs", "Variable costs"},
			},
		},
	}

	if s, ok := schemas[typ]; ok {
		return s
	}

	return diagramSchema{
		description: fmt.Sprintf("Diagram type %q. Use validate_diagram to check your data format.", typ),
	}
}

// --- Structured validation finding types (matches internal/patterns/errors.go shape) ---

// finding is a structured validation finding matching the patterns.ValidationError shape.
// It lives at the MCP boundary only — svggen internals are unchanged.
type finding struct {
	Pattern string         `json:"pattern"`           // diagram type, e.g. "bar_chart"
	Path    string         `json:"path"`              // JSON path, e.g. "data.series[0].values"
	Code    string         `json:"code"`              // lowercase_snake code, e.g. "required"
	Message string         `json:"message"`           // human-readable description
	Fix     *fixSuggestion `json:"fix,omitempty"`     // optional structured fix
}

// fixSuggestion matches patterns.FixSuggestion shape.
type fixSuggestion struct {
	Kind   string         `json:"kind"`            // e.g. "replace_value", "align_series"
	Params map[string]any `json:"params,omitempty"`
}

// codeMap converts svggen UPPER_SNAKE codes to lowercase_snake codes
// matching internal/patterns/errors.go conventions.
var codeMap = map[string]string{
	core.ErrCodeRequired:       "required",
	core.ErrCodeInvalidType:    "invalid_type",
	core.ErrCodeInvalidFormat:  "invalid_format",
	core.ErrCodeInvalidValue:   "invalid_value",
	core.ErrCodeUnknownField:   "unknown_field",
	core.ErrCodeParseFailed:    "parse_failed",
	core.ErrCodeConstraint:     "constraint",
	core.ErrCodeUnknownDiagram: "unknown_diagram",
}

// convertValidationError maps a svggen core.ValidationError to a structured
// finding at the MCP boundary. The svggen internal type is unchanged.
func convertValidationError(diagramType string, ve core.ValidationError) finding {
	code := strings.ToLower(ve.Code)
	if mapped, ok := codeMap[ve.Code]; ok {
		code = mapped
	}

	path := ve.Field
	if path == "" {
		path = "data"
	}

	f := finding{
		Pattern: diagramType,
		Path:    path,
		Code:    code,
		Message: ve.Message,
		Fix:     inferFix(ve),
	}
	return f
}

// inferFix derives a structured fix suggestion from the svggen validation error
// code and field context. Returns nil when no actionable fix can be inferred.
func inferFix(ve core.ValidationError) *fixSuggestion {
	switch ve.Code {
	case core.ErrCodeRequired:
		return &fixSuggestion{
			Kind:   "replace_value",
			Params: map[string]any{"field": ve.Field, "message": "provide the required field"},
		}
	case core.ErrCodeInvalidType, core.ErrCodeInvalidFormat, core.ErrCodeInvalidValue:
		params := map[string]any{"field": ve.Field}
		if ve.Value != nil {
			params["invalid_value"] = ve.Value
		}
		return &fixSuggestion{Kind: "replace_value", Params: params}
	case core.ErrCodeConstraint:
		// Constraint violations on series/values fields suggest alignment.
		if strings.Contains(ve.Field, "series") || strings.Contains(ve.Field, "values") {
			return &fixSuggestion{
				Kind:   "align_series",
				Params: map[string]any{"field": ve.Field},
			}
		}
		// Constraint on item counts suggest reducing.
		if strings.Contains(ve.Field, "items") || strings.Contains(ve.Field, "stages") ||
			strings.Contains(ve.Field, "slices") || strings.Contains(ve.Field, "layers") {
			return &fixSuggestion{
				Kind:   "reduce_items",
				Params: map[string]any{"field": ve.Field},
			}
		}
		return nil
	case core.ErrCodeUnknownField:
		return nil // agent should remove the field
	default:
		return nil
	}
}

// convertValidationErrors converts a slice of core.ValidationError into findings.
func convertValidationErrors(diagramType string, errs []core.ValidationError) []finding {
	findings := make([]finding, len(errs))
	for i, ve := range errs {
		findings[i] = convertValidationError(diagramType, ve)
	}
	return findings
}
