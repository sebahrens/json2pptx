// Package main provides an MCP (Model Context Protocol) server for svggen diagram rendering.
//
// Usage:
//
//	svggen-mcp              # Start MCP server over stdio
//	svggen-mcp --version    # Print version
//
// The server exposes five tools:
//   - render_diagram: Render a diagram to SVG or PNG
//   - list_diagram_types: List all available diagram types
//   - validate_diagram: Validate diagram input without rendering
//   - get_diagram_schema: Get the data schema for a specific diagram type
//   - get_capabilities: Returns schema_version, tool list, registered chart
//     and diagram types, deprecations, and feature flags for drift detection
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

// version is the MCP server version, sourced from the svggen library so the
// server, the get_capabilities response, and the library all report a single
// value.
const version = svggen.Version

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
	s.AddTool(getCapabilitiesTool(), handleGetCapabilities)

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
		mcp.WithBoolean("dry_run",
			mcp.Description("When true, run the diagram's layout and labeling pass and return ONLY structured findings (chart.tick_thinned, chart.label_clipped, chart.legend_overflow_dropped, chart.label_truncated, chart.scatter_label_skipped, etc.) without producing SVG or PNG bytes. Useful for preview-time visual feedback before committing to a full render. Output is JSON {valid:bool, findings:[...]}."),
		),
	)
}

func listDiagramTypesTool() mcp.Tool {
	return mcp.NewTool("list_diagram_types",
		mcp.WithDescription("List all available diagram types that can be rendered. Returns an array of {name, aliases?} entries — name is the canonical registered ID (e.g., \"bar_chart\"); aliases enumerates other accepted names (e.g., [\"bar\"]) that resolve to the same renderer."),
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

func getCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_capabilities",
		mcp.WithDescription("Returns this svggen-mcp server's schema version, the live tool list, registered chart and diagram types, deprecations, and feature flags. Use this once per session to detect contract drift without re-reading SKILL.md. Compare schema_version across sessions — a change means the rendering or validation contract may have shifted."),
	)
}

// --- Tool handlers ---

//nolint:gocyclo // structured error envelopes per parameter inflate branch count
func handleRenderDiagram(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("type")
	if err != nil {
		return emitErrorResult(diagnostic{
			Code:     "required",
			Message:  "type is required",
			Path:     "type",
			Severity: "error",
			Fix:      &fix{Kind: "provide_value", Params: map[string]any{"path": "type"}},
			NextToolCall: &toolCallSugg{
				Tool:         "list_diagram_types",
				ArgsTemplate: map[string]any{},
			},
		})
	}

	// Check diagram type exists
	reg := svggen.DefaultRegistry()
	d := reg.Get(diagramType)
	if d == nil {
		return emitErrorResult(diagnostic{
			Code:     "unknown_diagram_type",
			Message:  fmt.Sprintf("unknown diagram type %q — use list_diagram_types to see available types", diagramType),
			Path:     "type",
			Severity: "error",
			Fix: &fix{
				Kind:   "replace_value",
				Params: map[string]any{"path": "type", "invalid_value": diagramType},
			},
			NextToolCall: &toolCallSugg{
				Tool:         "list_diagram_types",
				ArgsTemplate: map[string]any{},
			},
			Details: map[string]any{"type": diagramType},
		})
	}

	// Extract data
	args := request.GetArguments()
	dataRaw, ok := args["data"]
	if !ok {
		return emitErrorResult(diagnostic{
			Code:     "required",
			Message:  "data is required",
			Path:     "data",
			Severity: "error",
			Fix:      &fix{Kind: "provide_value", Params: map[string]any{"path": "data"}},
			NextToolCall: &toolCallSugg{
				Tool:         "get_diagram_schema",
				ArgsTemplate: map[string]any{"type": diagramType},
			},
			Details: map[string]any{"pattern": diagramType},
		})
	}
	dataMap, ok := dataRaw.(map[string]any)
	if !ok {
		return emitErrorResult(diagnostic{
			Code:     "invalid_type",
			Message:  "data must be a JSON object",
			Path:     "data",
			Severity: "error",
			Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "data"}},
			NextToolCall: &toolCallSugg{
				Tool:         "get_diagram_schema",
				ArgsTemplate: map[string]any{"type": diagramType},
			},
			Details: map[string]any{"pattern": diagramType},
		})
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
			return emitErrorResult(diagnostic{
				Code:     "invalid_type",
				Message:  "style must be a JSON object",
				Path:     "style",
				Severity: "error",
				Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "style"}},
				Details:  map[string]any{"pattern": diagramType},
			})
		}
		styleJSON, err := json.Marshal(styleMap)
		if err != nil {
			return emitErrorResult(diagnostic{
				Code:     "invalid_value",
				Message:  fmt.Sprintf("failed to encode style: %v", err),
				Path:     "style",
				Severity: "error",
				Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "style"}},
				Details:  map[string]any{"pattern": diagramType, "go_error": err.Error()},
			})
		}
		var style svggen.StyleSpec
		if err := json.Unmarshal(styleJSON, &style); err != nil {
			return emitErrorResult(diagnostic{
				Code:     "invalid_value",
				Message:  fmt.Sprintf("invalid style payload: %v", err),
				Path:     "style",
				Severity: "error",
				Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "style"}},
				Details:  map[string]any{"pattern": diagramType, "go_error": err.Error()},
			})
		}
		req.Style = style
	}

	// Determine format
	format := "svg"
	if f, err := request.RequireString("format"); err == nil && f != "" {
		format = f
	}

	// Dry-run path: run layout/labeling but return only findings.
	if dryRun, _ := args["dry_run"].(bool); dryRun {
		return runDryRender(req)
	}

	// Render
	result, err := svggen.RenderMultiFormat(req, format)
	if err != nil {
		// Prefer structured validation errors when the renderer surfaces them.
		if vErrs := svggen.GetValidationErrors(err); len(vErrs) > 0 {
			return emitErrorResult(convertValidationErrors(diagramType, vErrs)...)
		}
		return emitErrorResult(diagnostic{
			Code:     "render_failed",
			Message:  fmt.Sprintf("render failed: %v", err),
			Path:     "data",
			Severity: "error",
			Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "data"}},
			NextToolCall: &toolCallSugg{
				Tool:         "get_diagram_schema",
				ArgsTemplate: map[string]any{"type": diagramType},
			},
			Details: map[string]any{
				"pattern":  diagramType,
				"go_error": err.Error(),
			},
		})
	}

	switch format {
	case "svg":
		if result.SVG == nil {
			return emitErrorResult(diagnostic{
				Code:     "render_failed",
				Message:  "no SVG output generated",
				Path:     "format",
				Severity: "error",
				Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "data"}},
				Details:  map[string]any{"pattern": diagramType, "format": "svg"},
			})
		}
		return mcp.NewToolResultText(string(result.SVG.Content)), nil

	case "png":
		if result.PNG == nil {
			return emitErrorResult(diagnostic{
				Code:     "render_failed",
				Message:  "no PNG output generated",
				Path:     "format",
				Severity: "error",
				Fix:      &fix{Kind: "replace_value", Params: map[string]any{"path": "data"}},
				Details:  map[string]any{"pattern": diagramType, "format": "png"},
			})
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
		return emitErrorResult(diagnostic{
			Code:     "invalid_value",
			Message:  fmt.Sprintf("unsupported format %q", format),
			Path:     "format",
			Severity: "error",
			Fix: &fix{
				Kind: "replace_value",
				Params: map[string]any{
					"path":          "format",
					"invalid_value": format,
					"allowed":       []string{"svg", "png"},
				},
			},
			Details: map[string]any{"pattern": diagramType},
		})
	}
}

// diagramTypeEntry is the per-type record returned by list_diagram_types.
// Name is the canonical registered ID; Aliases enumerates other names the
// registry will also accept (e.g., "bar" for "bar_chart"). Agents should
// prefer Name in new code; Aliases is provided so existing JSON payloads
// using the short form continue to be recognized as valid.
type diagramTypeEntry struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

// runDryRender executes svggen's layout/labeling pass and returns the
// findings as JSON {valid, findings, error?}. It is the dry_run handler for
// render_diagram, factored out so handleRenderDiagram stays within gocognit
// limits.
func runDryRender(req *svggen.RequestEnvelope) (*mcp.CallToolResult, error) {
	findings, dryErr := svggen.DryRender(req)
	type dryRunResult struct {
		Valid    bool             `json:"valid"`
		Findings []svggen.Finding `json:"findings"`
		Error    string           `json:"error,omitempty"`
	}
	res := dryRunResult{
		Valid:    dryErr == nil,
		Findings: findings,
	}
	if dryErr != nil {
		res.Error = dryErr.Error()
	}
	out, mErr := json.MarshalIndent(res, "", "  ")
	if mErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal dry_run result: %v", mErr)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func handleListDiagramTypes(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	types := svggen.Types()
	sort.Strings(types)

	entries := make([]diagramTypeEntry, 0, len(types))
	for _, t := range types {
		entry := diagramTypeEntry{Name: t}
		if aliases := svggen.Aliases(t); len(aliases) > 0 {
			entry.Aliases = aliases
		}
		entries = append(entries, entry)
	}

	output, err := json.MarshalIndent(entries, "", "  ")
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

	// validationResult envelope matches SKILL.md: {valid, errors?: [...]}.
	// The field is named "errors" (not "diagnostics") to align with
	// validate_pattern's envelope and avoid collision with the render-time
	// "diagnostics" channel inside json2pptx.
	type validationResult struct {
		Valid  bool         `json:"valid"`
		Errors []diagnostic `json:"errors,omitempty"`
	}

	// Validate envelope
	if err := req.Validate(); err != nil {
		errs := svggen.GetValidationErrors(err)
		var diags []diagnostic
		if len(errs) > 0 {
			diags = convertValidationErrors(diagramType, errs)
		} else {
			// Non-structured error from envelope validation — wrap it.
			diags = []diagnostic{{
				Code:     "parse_failed",
				Message:  err.Error(),
				Path:     "data",
				Severity: "error",
				Details:  map[string]any{"pattern": diagramType},
			}}
		}
		output, _ := json.MarshalIndent(validationResult{
			Valid:  false,
			Errors: diags,
		}, "", "  ")
		return mcp.NewToolResultText(string(output)), nil
	}

	// Validate against diagram-specific rules
	if err := d.Validate(req); err != nil {
		errs := svggen.GetValidationErrors(err)
		var diags []diagnostic
		if len(errs) > 0 {
			diags = convertValidationErrors(diagramType, errs)
		} else {
			// Non-structured error — wrap it.
			diags = []diagnostic{{
				Code:     "invalid_value",
				Message:  err.Error(),
				Path:     "data",
				Severity: "error",
				Details:  map[string]any{"pattern": diagramType},
			}}
		}
		output, _ := json.MarshalIndent(validationResult{
			Valid:  false,
			Errors: diags,
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

// --- Structured diagnostic types (unified contract with internal/diagnostics) ---
//
// The diagnostic struct mirrors internal/diagnostics.Diagnostic so that agents
// see an identical JSON envelope from svggen and json2pptx surfaces:
//
//   { "code", "message", "path", "severity", "fix", "next_tool_call", "details" }
//
// Because svggen is a separate Go module that cannot import internal/, the
// struct is duplicated here at the MCP boundary.

// errorResult is the unified envelope returned by handlers when one or more
// structured diagnostics must be surfaced via mcp.NewToolResultError. It is
// the same shape as the validation result returned by handleValidateDiagram
// so agents can parse error payloads uniformly across all svggen-mcp tools.
type errorResult struct {
	Valid       bool         `json:"valid"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

// emitErrorResult marshals one or more diagnostics into the unified envelope
// and returns it as an MCP tool error. The single-string payload format is
// the JSON envelope itself — never a doubly-encoded JSON-in-a-string.
func emitErrorResult(diags ...diagnostic) (*mcp.CallToolResult, error) {
	output, _ := json.MarshalIndent(errorResult{Valid: false, Diagnostics: diags}, "", "  ")
	return mcp.NewToolResultError(string(output)), nil
}

// diagnostic is a single machine-readable issue matching the
// internal/diagnostics.Diagnostic JSON shape.
type diagnostic struct {
	Code         string          `json:"code"`                    // lowercase_snake code, e.g. "required"
	Message      string          `json:"message"`                 // human-readable description
	Path         string          `json:"path,omitempty"`          // JSON path, e.g. "data.series[0].values"
	Severity     string          `json:"severity"`                // "error", "warning", "info"
	Fix          *fix            `json:"fix,omitempty"`           // optional structured remediation
	NextToolCall *toolCallSugg   `json:"next_tool_call,omitempty"` // machine-readable next MCP tool call
	Details      map[string]any  `json:"details,omitempty"`       // additional context (pattern, value, etc.)
}

// fix matches internal/diagnostics.Fix / patterns.FixSuggestion JSON shape.
type fix struct {
	Kind   string         `json:"kind"`            // e.g. "provide_value", "align_series"
	Params map[string]any `json:"params,omitempty"`
}

// toolCallSugg matches patterns.ToolCallSuggestion JSON shape.
type toolCallSugg struct {
	Tool         string         `json:"tool"`
	ArgsTemplate map[string]any `json:"args_template"`
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

// convertValidationError maps a svggen core.ValidationError to a diagnostic
// at the MCP boundary, producing the same JSON shape as
// internal/diagnostics.Diagnostic. The svggen internal type is unchanged.
func convertValidationError(diagramType string, ve core.ValidationError) diagnostic {
	code := strings.ToLower(ve.Code)
	if mapped, ok := codeMap[ve.Code]; ok {
		code = mapped
	}

	path := ve.Field
	if path == "" {
		path = "data"
	}

	d := diagnostic{
		Code:     code,
		Message:  ve.Message,
		Path:     path,
		Severity: "error", // all svggen validation failures are errors
		Fix:      inferFix(ve),
		Details:  map[string]any{"pattern": diagramType},
	}

	// Attach next_tool_call for structural errors — point the agent at
	// get_diagram_schema so it can discover the expected shape.
	if ve.Code == core.ErrCodeRequired || ve.Code == core.ErrCodeInvalidType ||
		ve.Code == core.ErrCodeInvalidFormat {
		d.NextToolCall = &toolCallSugg{
			Tool: "get_diagram_schema",
			ArgsTemplate: map[string]any{
				"type": diagramType,
			},
		}
	}

	// Carry the invalid value in details when available.
	if ve.Value != nil {
		d.Details["value"] = ve.Value
	}

	return d
}

// inferFix derives a structured fix suggestion from the svggen validation error
// code and field context. Returns nil when no actionable fix can be inferred.
//
// Fix kinds are constrained to the chart-finding enum documented in
// skills/generate-deck/SKILL.md for validate_diagram:
//
//	align_series, truncate_or_split, replace_value, explicit_scale, reduce_items
//
// Capacity-class constraint errors (too many items/series/slices/etc.) map to
// truncate_or_split. Log/log-scale constraint errors map to explicit_scale.
// Required-field and unknown-field errors that have no natural mapping into
// the chart enum either fall back to replace_value (when supplying a value at
// the path is the obvious remediation) or return nil.
func inferFix(ve core.ValidationError) *fix {
	switch ve.Code {
	case core.ErrCodeRequired:
		// A missing required field can be remediated by supplying a value at
		// the path — closest match in the chart-finding enum is replace_value.
		return &fix{
			Kind:   "replace_value",
			Params: map[string]any{"path": ve.Field},
		}
	case core.ErrCodeInvalidType, core.ErrCodeInvalidFormat, core.ErrCodeInvalidValue:
		params := map[string]any{"path": ve.Field}
		if ve.Value != nil {
			params["invalid_value"] = ve.Value
		}
		return &fix{Kind: "replace_value", Params: params}
	case core.ErrCodeConstraint:
		msg := strings.ToLower(ve.Message)
		field := strings.ToLower(ve.Field)

		// Log/log-scale constraint violations (e.g., negative value on log
		// scale, monotonic-scale violations) map to explicit_scale.
		if strings.Contains(field, "scale") || strings.Contains(msg, "log scale") ||
			strings.Contains(msg, "log-scale") || strings.Contains(msg, "logarithmic") {
			return &fix{
				Kind:   "explicit_scale",
				Params: map[string]any{"path": ve.Field},
			}
		}

		// Capacity-class constraints: too many items/series/categories/etc.
		// We treat both "field is a count-bearing collection" (items, stages,
		// slices, layers, categories, points) and "message indicates a max
		// was exceeded" as capacity hits — agents should truncate or split.
		isCountField := strings.Contains(field, "items") || strings.Contains(field, "stages") ||
			strings.Contains(field, "slices") || strings.Contains(field, "layers") ||
			strings.Contains(field, "categories") || strings.Contains(field, "points")
		isCapacityMsg := strings.Contains(msg, "exceed") || strings.Contains(msg, "too many") ||
			strings.Contains(msg, "maximum") || strings.Contains(msg, "max ")
		if isCountField || isCapacityMsg {
			return &fix{
				Kind:   "truncate_or_split",
				Params: map[string]any{"path": ve.Field},
			}
		}

		// Series/values length-mismatch constraints map to alignment.
		if strings.Contains(field, "series") || strings.Contains(field, "values") {
			return &fix{
				Kind:   "align_series",
				Params: map[string]any{"path": ve.Field},
			}
		}

		return nil
	case core.ErrCodeUnknownField:
		// Removing an unknown key has no analogue in the chart-finding enum,
		// so emit no fix kind. The diagnostic message still identifies the
		// stray key for the agent.
		return nil
	default:
		return nil
	}
}

// convertValidationErrors converts a slice of core.ValidationError into diagnostics.
func convertValidationErrors(diagramType string, errs []core.ValidationError) []diagnostic {
	diagnostics := make([]diagnostic, len(errs))
	for i, ve := range errs {
		diagnostics[i] = convertValidationError(diagramType, ve)
	}
	return diagnostics
}

// --- get_capabilities ---

// capabilitiesToolEntry describes a single MCP tool surfaced in the capabilities response.
type capabilitiesToolEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// capabilitiesDeprecation describes a deprecated input field, tool, or behavior.
// The list is empty today; populate it when adding new deprecations so agents
// can detect them programmatically.
type capabilitiesDeprecation struct {
	Path        string `json:"path"`
	Replacement string `json:"replacement"`
	RemovedIn   string `json:"removed_in,omitempty"`
}

// capabilitiesFeatures advertises optional server features so agents can branch
// on capability rather than version-sniffing.
type capabilitiesFeatures struct {
	DryRender        bool `json:"dry_render"`
	StructuredErrors bool `json:"structured_errors"`
}

// capabilitiesResponse is the JSON shape returned by handleGetCapabilities.
// The schema mirrors json2pptx-mcp's get_capabilities at the field level
// (schema_version, tool list, deprecations, features) so agents can parse
// both responses uniformly.
type capabilitiesResponse struct {
	SchemaVersion string                    `json:"schema_version"`
	ToolList      []capabilitiesToolEntry   `json:"tool_list"`
	ChartTypes    []string                  `json:"chart_types"`
	DiagramTypes  []string                  `json:"diagram_types"`
	Deprecations  []capabilitiesDeprecation `json:"deprecations"`
	Features      capabilitiesFeatures      `json:"features"`
}

// toolCatalog enumerates every tool registered in run() with a one-line
// description. Keep this in sync with the s.AddTool calls — the
// TestGetCapabilities_ToolListMatchesRegisteredTools test enforces parity.
func toolCatalog() []capabilitiesToolEntry {
	entries := []capabilitiesToolEntry{
		{
			Name:        "render_diagram",
			Description: "Render a diagram or chart to SVG or PNG. Supports dry_run for layout-only findings.",
		},
		{
			Name:        "list_diagram_types",
			Description: "List all available diagram/chart types with their aliases.",
		},
		{
			Name:        "validate_diagram",
			Description: "Validate a diagram payload without rendering. Returns structured {valid, errors} envelope.",
		},
		{
			Name:        "get_diagram_schema",
			Description: "Get the expected data schema and a minimal example for a specific diagram type.",
		},
		{
			Name:        "get_capabilities",
			Description: "Returns schema version, tool list, chart/diagram types, deprecations, and feature flags.",
		},
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// classifyRegisteredTypes splits the canonical registered types into chart-
// style and diagram-style buckets. Canonical names ending in "_chart" (or the
// historical "waterfall" chart) are charts; everything else is a diagram.
// Both slices are sorted for stable output.
func classifyRegisteredTypes() (chartTypes, diagramTypes []string) {
	all := svggen.Types()
	for _, t := range all {
		if isChartType(t) {
			chartTypes = append(chartTypes, t)
		} else {
			diagramTypes = append(diagramTypes, t)
		}
	}
	sort.Strings(chartTypes)
	sort.Strings(diagramTypes)
	return chartTypes, diagramTypes
}

// isChartType reports whether a canonical svggen registry name is a chart
// (vs. a diagram). The classification mirrors svggen/init.go: anything
// suffixed "_chart" plus the historical "waterfall" series chart.
func isChartType(canonicalName string) bool {
	if strings.HasSuffix(canonicalName, "_chart") {
		return true
	}
	return canonicalName == "waterfall"
}

func handleGetCapabilities(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chartTypes, diagramTypes := classifyRegisteredTypes()

	resp := capabilitiesResponse{
		SchemaVersion: svggen.Version,
		ToolList:      toolCatalog(),
		ChartTypes:    chartTypes,
		DiagramTypes:  diagramTypes,
		Deprecations:  []capabilitiesDeprecation{},
		Features: capabilitiesFeatures{
			// render_diagram supports dry_run.
			DryRender: true,
			// validate_diagram and render_diagram emit unified {code, message,
			// path, severity, fix, next_tool_call, details} diagnostics.
			StructuredErrors: true,
		},
	}

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal capabilities: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
