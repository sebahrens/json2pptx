// Package main provides an MCP (Model Context Protocol) server for svggen diagram rendering.
//
// Usage:
//
//	svggen-mcp              # Start MCP server over stdio
//	svggen-mcp --version    # Print version
//
// The server exposes six tools:
//   - render_diagram: Render a diagram to SVG or PNG
//   - list_diagram_types: List all available diagram types
//   - validate_diagram: Validate diagram input without rendering
//   - get_diagram_schema: Get the data schema for a specific diagram type
//   - get_capabilities: Returns schema_version, tool list, registered chart
//     and diagram types, deprecations, and feature flags for drift detection
//   - get_started: Returns the recommended ordered MCP-call sequence for a
//     stated task (render, preflight-render, embed-in-deck)
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
	s.AddTool(getStartedTool(), handleGetStarted)

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
		mcp.WithDescription("Get the expected data schema and canonical example values for a specific diagram type. Returns example_values with both a minimal example (smallest valid input) and a realistic example (representative shape and content). Mirrors the example_values field returned by json2pptx-mcp.show_pattern so agents can reuse the same exemplar pattern across both tools."),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Diagram type to get schema for."),
		),
	)
}

func getCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_capabilities",
		mcp.WithDescription("Returns this svggen-mcp server's schema version, the live tool list, registered chart and diagram types, per-type chart_capabilities and diagram_capabilities (limits, density behavior, label strategy, required/optional fields — sourced from the svggen package, the single source of truth shared with json2pptx-mcp's get_chart_capabilities/get_diagram_capabilities), deprecations, and feature flags. Use this once per session to detect contract drift without re-reading SKILL.md. Compare schema_version across sessions — a change means the rendering or validation contract may have shifted."),
	)
}

// --- Tool handlers ---

//nolint:gocyclo // structured error envelopes per parameter inflate branch count
func handleRenderDiagram(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	diagramType, err := request.RequireString("type")
	if err != nil {
		return emitErrorResult(diagnostic{
			Code:     CodeRequired,
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
			Code:     CodeUnknownDiagramType,
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
			Code:     CodeRequired,
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
			Code:     CodeInvalidType,
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
				Code:     CodeInvalidType,
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
				Code:     CodeInvalidValue,
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
				Code:     CodeInvalidValue,
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
			Code:     CodeRenderFailed,
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
				Code:     CodeRenderFailed,
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
				Code:     CodeRenderFailed,
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
			Code:     CodeInvalidValue,
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
				Code:     CodeParseFailed,
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
				Code:     CodeInvalidValue,
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

	// exampleValues mirrors the show_pattern.example_values field returned by
	// json2pptx-mcp so agents can use the same key across both tools. It carries
	// two flavours: a minimal example (smallest valid input the diagram accepts)
	// and a realistic example (representative shape and content).
	type exampleValuesEnvelope struct {
		Minimal   any `json:"minimal,omitempty"`
		Realistic any `json:"realistic,omitempty"`
	}

	type schemaResult struct {
		Type          string                 `json:"type"`
		Description   string                 `json:"description"`
		Example       any                    `json:"example,omitempty"`        // deprecated: use example_values.realistic; retained for back-compat
		ExampleValues *exampleValuesEnvelope `json:"example_values,omitempty"` // canonical examples matching show_pattern.example_values
		DataSchema    any                    `json:"data_schema,omitempty"`
	}

	result := schemaResult{
		Type:        diagramType,
		Description: schema.description,
		Example:     schema.realistic,
	}

	if schema.minimal != nil || schema.realistic != nil {
		result.ExampleValues = &exampleValuesEnvelope{
			Minimal:   schema.minimal,
			Realistic: schema.realistic,
		}
	}

	// Include the machine-readable data schema when the diagram provides one.
	if ds, ok := d.(svggen.DiagramWithSchema); ok {
		result.DataSchema = ds.DataSchema()
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

// diagramSchema holds description and example data for a diagram type. Both a
// minimal and a realistic example are surfaced so agents can either bootstrap
// with the smallest valid payload or copy a representative shape.
type diagramSchema struct {
	description string
	minimal     any
	realistic   any
}

// getSchemaForType returns a human-readable schema with example for known types.
func getSchemaForType(typ string) diagramSchema {
	schemas := map[string]diagramSchema{
		"bar_chart": {
			description: "Bar chart with categories and series. Supports grouped, stacked, and horizontal variants.",
			minimal: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{"name": "Revenue", "values": []any{100, 150, 120, 180}},
					map[string]any{"name": "Cost", "values": []any{60, 80, 70, 95}},
				},
			},
		},
		"line_chart": {
			description: "Line chart with categories and series. Supports multiple lines, area fill, and smooth curves.",
			minimal: map[string]any{
				"categories": []any{"T1", "T2"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
				"series": []any{
					map[string]any{"name": "Sales", "values": []any{40, 55, 70, 65, 80, 95}},
					map[string]any{"name": "Forecast", "values": []any{42, 58, 68, 72, 85, 100}},
				},
			},
		},
		"pie_chart": {
			description: "Pie or donut chart with labeled slices.",
			minimal: map[string]any{
				"slices": []any{
					map[string]any{"label": "A", "value": 60},
					map[string]any{"label": "B", "value": 40},
				},
			},
			realistic: map[string]any{
				"slices": []any{
					map[string]any{"label": "Product A", "value": 40},
					map[string]any{"label": "Product B", "value": 30},
					map[string]any{"label": "Product C", "value": 20},
					map[string]any{"label": "Other", "value": 10},
				},
			},
		},
		"radar_chart": {
			description: "Radar/spider chart with axes and series.",
			minimal: map[string]any{
				"axes": []any{"A", "B", "C"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{50, 60, 70}},
				},
			},
			realistic: map[string]any{
				"axes": []any{"Speed", "Power", "Range", "Defense", "Accuracy"},
				"series": []any{
					map[string]any{"name": "Player A", "values": []any{80, 70, 90, 60, 85}},
					map[string]any{"name": "Player B", "values": []any{60, 85, 70, 80, 75}},
				},
			},
		},
		"scatter_chart": {
			description: "Scatter plot with x/y data points.",
			minimal: map[string]any{
				"series": []any{
					map[string]any{
						"name": "S1",
						"points": []any{
							map[string]any{"x": 1, "y": 1},
							map[string]any{"x": 2, "y": 2},
						},
					},
				},
			},
			realistic: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Group A",
						"points": []any{
							map[string]any{"x": 10, "y": 20},
							map[string]any{"x": 30, "y": 40},
							map[string]any{"x": 50, "y": 35},
						},
					},
					map[string]any{
						"name": "Group B",
						"points": []any{
							map[string]any{"x": 15, "y": 55},
							map[string]any{"x": 45, "y": 70},
						},
					},
				},
			},
		},
		"bubble_chart": {
			description: "Bubble chart with x/y/size data points.",
			minimal: map[string]any{
				"series": []any{
					map[string]any{
						"name": "S1",
						"points": []any{
							map[string]any{"x": 1, "y": 1, "size": 10},
							map[string]any{"x": 2, "y": 2, "size": 20},
						},
					},
				},
			},
			realistic: map[string]any{
				"series": []any{
					map[string]any{
						"name": "Markets",
						"points": []any{
							map[string]any{"x": 10, "y": 20, "size": 30, "label": "US"},
							map[string]any{"x": 40, "y": 50, "size": 20, "label": "EU"},
							map[string]any{"x": 25, "y": 65, "size": 15, "label": "APAC"},
						},
					},
				},
			},
		},
		"waterfall": {
			description: "Waterfall chart showing incremental changes to a total.",
			minimal: map[string]any{
				"items": []any{
					map[string]any{"label": "Start", "value": 100},
					map[string]any{"label": "Change", "value": -20},
					map[string]any{"label": "End", "value": 0, "is_total": true},
				},
			},
			realistic: map[string]any{
				"items": []any{
					map[string]any{"label": "Revenue", "value": 500},
					map[string]any{"label": "COGS", "value": -200},
					map[string]any{"label": "OpEx", "value": -150},
					map[string]any{"label": "Tax", "value": -50},
					map[string]any{"label": "Net Profit", "value": 0, "is_total": true},
				},
			},
		},
		"org_chart": {
			description: "Organizational chart with hierarchical nodes.",
			minimal: map[string]any{
				"nodes": []any{
					map[string]any{"id": "root", "label": "Lead"},
					map[string]any{"id": "child", "label": "Report", "parent": "root"},
				},
			},
			realistic: map[string]any{
				"nodes": []any{
					map[string]any{"id": "ceo", "label": "CEO", "title": "John Smith"},
					map[string]any{"id": "vp1", "label": "VP Engineering", "title": "Jane Doe", "parent": "ceo"},
					map[string]any{"id": "vp2", "label": "VP Sales", "title": "Sam Lee", "parent": "ceo"},
					map[string]any{"id": "vp3", "label": "VP Finance", "title": "Pat Kim", "parent": "ceo"},
				},
			},
		},
		"gantt": {
			description: "Gantt chart for project timelines with tasks and dependencies.",
			minimal: map[string]any{
				"tasks": []any{
					map[string]any{"id": "t1", "name": "Task", "start": "2026-01-01", "end": "2026-01-15"},
				},
			},
			realistic: map[string]any{
				"tasks": []any{
					map[string]any{"id": "t1", "name": "Design", "start": "2026-01-01", "end": "2026-01-15"},
					map[string]any{"id": "t2", "name": "Develop", "start": "2026-01-15", "end": "2026-02-15", "depends_on": []any{"t1"}},
					map[string]any{"id": "t3", "name": "Test", "start": "2026-02-15", "end": "2026-03-01", "depends_on": []any{"t2"}},
				},
			},
		},
		"timeline": {
			description: "Timeline with dated events.",
			minimal: map[string]any{
				"events": []any{
					map[string]any{"date": "2026-01", "title": "Start"},
					map[string]any{"date": "2026-06", "title": "End"},
				},
			},
			realistic: map[string]any{
				"events": []any{
					map[string]any{"date": "2026-01", "title": "Project Start", "description": "Kicked off development"},
					map[string]any{"date": "2026-04", "title": "Alpha"},
					map[string]any{"date": "2026-08", "title": "Beta Release"},
					map[string]any{"date": "2026-12", "title": "Launch"},
				},
			},
		},
		"funnel_chart": {
			description: "Funnel chart showing progressive narrowing stages.",
			minimal: map[string]any{
				"stages": []any{
					map[string]any{"label": "Top", "value": 100},
					map[string]any{"label": "Bottom", "value": 25},
				},
			},
			realistic: map[string]any{
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
			minimal: map[string]any{
				"layers": []any{
					map[string]any{"label": "Top"},
					map[string]any{"label": "Middle"},
					map[string]any{"label": "Base"},
				},
			},
			realistic: map[string]any{
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
			minimal: map[string]any{
				"circles": []any{
					map[string]any{"label": "A"},
					map[string]any{"label": "B"},
				},
			},
			realistic: map[string]any{
				"circles": []any{
					map[string]any{"label": "Set A", "items": []any{"a", "b", "c"}},
					map[string]any{"label": "Set B", "items": []any{"c", "d", "e"}},
					map[string]any{"label": "Set C", "items": []any{"e", "f", "a"}},
				},
			},
		},
		"swot": {
			description: "SWOT analysis matrix (Strengths, Weaknesses, Opportunities, Threats).",
			minimal: map[string]any{
				"strengths":     []any{"S1"},
				"weaknesses":    []any{"W1"},
				"opportunities": []any{"O1"},
				"threats":       []any{"T1"},
			},
			realistic: map[string]any{
				"strengths":     []any{"Strong brand", "Loyal customers"},
				"weaknesses":    []any{"High costs", "Limited reach"},
				"opportunities": []any{"New markets", "Partnerships"},
				"threats":       []any{"Competition", "Regulation"},
			},
		},
		"matrix_2x2": {
			description: "2x2 matrix/quadrant diagram with labeled axes and points. Points go in 'points' (NOT 'items'); x/y are on a 0-100 scale by default (origin bottom-left, quadrant split at 50). Use the 'quadrants' form for coordinate-free placement.",
			minimal: map[string]any{
				"x_axis_label": "X",
				"y_axis_label": "Y",
				"points": []any{
					map[string]any{"label": "Item", "x": 50, "y": 50},
				},
			},
			realistic: map[string]any{
				"x_axis_label":    "Effort",
				"y_axis_label":    "Impact",
				"quadrant_labels": []any{"Quick Wins", "Major Projects", "Fill-Ins", "Thankless Tasks"},
				"points": []any{
					map[string]any{"label": "Quick Win", "x": 20, "y": 80},
					map[string]any{"label": "Major Project", "x": 80, "y": 90},
					map[string]any{"label": "Fill In", "x": 30, "y": 30},
					map[string]any{"label": "Avoid", "x": 80, "y": 20},
				},
			},
		},
		"fishbone": {
			description: "Fishbone (Ishikawa) cause-and-effect diagram.",
			minimal: map[string]any{
				"effect": "Effect",
				"categories": []any{
					map[string]any{"name": "Cat1", "causes": []any{"Cause"}},
				},
			},
			realistic: map[string]any{
				"effect": "Production Delays",
				"categories": []any{
					map[string]any{"name": "People", "causes": []any{"Training", "Staffing"}},
					map[string]any{"name": "Process", "causes": []any{"Bottleneck", "Handoffs"}},
					map[string]any{"name": "Technology", "causes": []any{"Downtime", "Legacy systems"}},
					map[string]any{"name": "Materials", "causes": []any{"Supply gaps"}},
				},
			},
		},
		"heatmap": {
			description: "Heatmap with rows, columns, and values.",
			minimal: map[string]any{
				"rows":    []any{"R1", "R2"},
				"columns": []any{"C1", "C2"},
				"values":  []any{[]any{1, 2}, []any{3, 4}},
			},
			realistic: map[string]any{
				"rows":    []any{"Mon", "Tue", "Wed", "Thu", "Fri"},
				"columns": []any{"Morning", "Afternoon", "Evening"},
				"values":  []any{[]any{3, 7, 2}, []any{5, 9, 4}, []any{1, 6, 8}, []any{4, 8, 5}, []any{2, 5, 9}},
			},
		},
		"treemap_chart": {
			description: "Treemap showing hierarchical data as nested rectangles.",
			minimal: map[string]any{
				"items": []any{
					map[string]any{"label": "A", "value": 60},
					map[string]any{"label": "B", "value": 40},
				},
			},
			realistic: map[string]any{
				"items": []any{
					map[string]any{"label": "Category A", "value": 60},
					map[string]any{"label": "Category B", "value": 30},
					map[string]any{"label": "Category C", "value": 10},
					map[string]any{"label": "Category D", "value": 5},
				},
			},
		},
		"gauge_chart": {
			description: "Gauge/dial chart showing a single metric against a range.",
			minimal: map[string]any{
				"value": 50,
				"min":   0,
				"max":   100,
			},
			realistic: map[string]any{
				"value":      75,
				"min":        0,
				"max":        100,
				"label":      "Performance",
				"unit":       "%",
				"thresholds": []any{30, 70},
			},
		},
		"value_chain": {
			description: "Porter's Value Chain diagram with primary and support activities.",
			minimal: map[string]any{
				"primary": []any{
					map[string]any{"label": "Inbound"},
					map[string]any{"label": "Operations"},
					map[string]any{"label": "Outbound"},
				},
				"support": []any{
					map[string]any{"label": "Infrastructure"},
				},
			},
			realistic: map[string]any{
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
			description: "Porter's Five Forces competitive analysis diagram. Each force is keyed by 'type' (canonical: rivalry, new_entrants, substitutes, suppliers, buyers) with an 'intensity' from 0.0 to 1.0 (NOT 'position'/'level'). An object-keyed form is also accepted (top-level rivalry/new_entrants/substitutes/supplier_power/buyer_power keys).",
			minimal: map[string]any{
				"forces": []any{
					map[string]any{"type": "rivalry", "intensity": 0.5},
					map[string]any{"type": "new_entrants", "intensity": 0.4},
					map[string]any{"type": "substitutes", "intensity": 0.3},
					map[string]any{"type": "suppliers", "intensity": 0.5},
					map[string]any{"type": "buyers", "intensity": 0.6},
				},
			},
			realistic: map[string]any{
				"industry_name": "Enterprise SaaS",
				"forces": []any{
					map[string]any{"type": "rivalry", "label": "Competitive Rivalry", "intensity": 0.8, "factors": []any{"Many competitors", "Low switching costs"}},
					map[string]any{"type": "new_entrants", "label": "Threat of New Entrants", "intensity": 0.4},
					map[string]any{"type": "substitutes", "label": "Threat of Substitutes", "intensity": 0.3},
					map[string]any{"type": "suppliers", "label": "Supplier Power", "intensity": 0.5},
					map[string]any{"type": "buyers", "label": "Buyer Power", "intensity": 0.7},
				},
			},
		},
		"pestel": {
			description: "PESTEL analysis diagram covering Political, Economic, Social, Technological, Environmental, Legal factors.",
			minimal: map[string]any{
				"factors": []any{
					map[string]any{"category": "Political", "items": []any{"Item"}},
				},
			},
			realistic: map[string]any{
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
			description: "9-box talent grid with performance and potential axes. People go in 'employees' (NOT 'people'); performance/potential are the strings \"low\"|\"medium\"|\"high\" (NOT numbers — numeric values are ignored and dump everyone in the center cell). Axis names use x_axis_label/y_axis_label. Alternatively place people explicitly via 'cells'.",
			minimal: map[string]any{
				"x_axis_label": "Performance",
				"y_axis_label": "Potential",
				"employees": []any{
					map[string]any{"name": "Person", "performance": "medium", "potential": "medium"},
				},
			},
			realistic: map[string]any{
				"x_axis_label": "Performance",
				"y_axis_label": "Potential",
				"employees": []any{
					map[string]any{"name": "Alice", "performance": "high", "potential": "high"},
					map[string]any{"name": "Bob", "performance": "medium", "potential": "high"},
					map[string]any{"name": "Carol", "performance": "high", "potential": "medium"},
					map[string]any{"name": "Dan", "performance": "low", "potential": "medium"},
				},
			},
		},
		"donut_chart": {
			description: "Donut chart (pie with a hollow center) with labeled slices.",
			minimal: map[string]any{
				"slices": []any{
					map[string]any{"label": "A", "value": 60},
					map[string]any{"label": "B", "value": 40},
				},
			},
			realistic: map[string]any{
				"slices": []any{
					map[string]any{"label": "Product A", "value": 40},
					map[string]any{"label": "Product B", "value": 30},
					map[string]any{"label": "Product C", "value": 20},
					map[string]any{"label": "Other", "value": 10},
				},
			},
		},
		"area_chart": {
			description: "Area chart with filled regions under one or more series lines.",
			minimal: map[string]any{
				"categories": []any{"T1", "T2"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
				"series": []any{
					map[string]any{"name": "Sales", "values": []any{40, 55, 70, 65, 80, 95}},
				},
			},
		},
		"stacked_bar_chart": {
			description: "Stacked bar chart with categories and additive series segments.",
			minimal: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
					map[string]any{"name": "S2", "values": []any{15, 25}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{"name": "Product", "values": []any{100, 150, 120, 180}},
					map[string]any{"name": "Services", "values": []any{60, 80, 70, 95}},
					map[string]any{"name": "Support", "values": []any{20, 30, 25, 35}},
				},
			},
		},
		"stacked_area_chart": {
			description: "Stacked area chart where series fills are stacked additively.",
			minimal: map[string]any{
				"categories": []any{"T1", "T2"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
					map[string]any{"name": "S2", "values": []any{15, 25}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
				"series": []any{
					map[string]any{"name": "Direct", "values": []any{40, 55, 70, 65, 80, 95}},
					map[string]any{"name": "Partner", "values": []any{20, 28, 35, 32, 40, 45}},
				},
			},
		},
		"grouped_bar_chart": {
			description: "Grouped (clustered) bar chart with categories and side-by-side series.",
			minimal: map[string]any{
				"categories": []any{"A", "B"},
				"series": []any{
					map[string]any{"name": "S1", "values": []any{10, 20}},
					map[string]any{"name": "S2", "values": []any{15, 25}},
				},
			},
			realistic: map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series": []any{
					map[string]any{"name": "Plan", "values": []any{100, 150, 120, 180}},
					map[string]any{"name": "Actual", "values": []any{95, 160, 130, 175}},
				},
			},
		},
		"business_model_canvas": {
			description: "Business Model Canvas with 9 building blocks.",
			minimal: map[string]any{
				"key_partners":           []any{"Partner"},
				"key_activities":         []any{"Activity"},
				"key_resources":          []any{"Resource"},
				"value_proposition":      []any{"Value"},
				"customer_segments":      []any{"Segment"},
				"channels":               []any{"Channel"},
				"customer_relationships": []any{"Relationship"},
				"revenue_streams":        []any{"Revenue"},
				"cost_structure":         []any{"Cost"},
			},
			realistic: map[string]any{
				"key_partners":           []any{"Suppliers", "Distributors"},
				"key_activities":         []any{"Production", "Marketing"},
				"key_resources":          []any{"IP", "Staff"},
				"value_proposition":      []any{"Quality", "Speed"},
				"customer_segments":      []any{"B2B", "B2C"},
				"channels":               []any{"Online", "Retail"},
				"customer_relationships": []any{"Self-service", "Community"},
				"revenue_streams":        []any{"Subscriptions", "Licensing"},
				"cost_structure":         []any{"Fixed costs", "Variable costs"},
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
	Code         string          `json:"code"`                    // SCREAMING_SNAKE_CASE code, e.g. "REQUIRED"
	Message      string          `json:"message"`                 // human-readable description
	Path         string          `json:"path,omitempty"`          // JSON path, e.g. "data.series[0].values"
	Severity     string          `json:"severity"`                // "error", "warning", "info"
	Fix          *fix            `json:"fix,omitempty"`           // optional structured remediation
	NextToolCall *toolCallSugg   `json:"next_tool_call,omitempty"` // machine-readable next MCP tool call
	Details      map[string]any  `json:"details,omitempty"`       // additional context (pattern, value, etc.)
}

// Canonical SCREAMING_SNAKE_CASE diagnostic codes emitted by svggen-mcp.
//
// These match the casing convention used by json2pptx-mcp (internal/diagnostics
// codes such as MISSING_PARAMETER, INVALID_JSON, TEMPLATE_NOT_FOUND) so agents
// dispatching on diagnostic.code can use a single equality check across both
// MCP servers. Prior to schema 4.23.0, svggen-mcp emitted lowercase_snake codes
// (required, invalid_type, …); the legacy → canonical mapping is surfaced via
// get_capabilities.deprecations for the deprecation window.
const (
	CodeRequired           = "REQUIRED"
	CodeInvalidType        = "INVALID_TYPE"
	CodeInvalidFormat      = "INVALID_FORMAT"
	CodeInvalidValue       = "INVALID_VALUE"
	CodeUnknownField       = "UNKNOWN_FIELD"
	CodeParseFailed        = "PARSE_FAILED"
	CodeConstraint         = "CONSTRAINT"
	CodeUnknownDiagram     = "UNKNOWN_DIAGRAM"
	CodeUnknownDiagramType = "UNKNOWN_DIAGRAM_TYPE"
	CodeRenderFailed       = "RENDER_FAILED"
)

// legacyCodeAliases maps the pre-4.23.0 lowercase_snake codes svggen-mcp used
// to emit to the canonical SCREAMING_SNAKE codes the server emits today. It is
// surfaced through get_capabilities.deprecations so agents that branched on the
// old casing know exactly which new code to dispatch on. Order is alphabetical
// by legacy code for stable output.
var legacyCodeAliases = []struct {
	Legacy    string
	Canonical string
}{
	{"constraint", CodeConstraint},
	{"invalid_format", CodeInvalidFormat},
	{"invalid_type", CodeInvalidType},
	{"invalid_value", CodeInvalidValue},
	{"parse_failed", CodeParseFailed},
	{"render_failed", CodeRenderFailed},
	{"required", CodeRequired},
	{"unknown_diagram", CodeUnknownDiagram},
	{"unknown_diagram_type", CodeUnknownDiagramType},
	{"unknown_field", CodeUnknownField},
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

// codeMap normalizes svggen core SCREAMING_SNAKE codes to the canonical
// svggen-mcp diagnostic code (also SCREAMING_SNAKE). The mapping is identity
// today — the table exists so future renames inside svggen core can be
// absorbed at the MCP boundary without changing the agent-visible code.
var codeMap = map[string]string{
	core.ErrCodeRequired:       CodeRequired,
	core.ErrCodeInvalidType:    CodeInvalidType,
	core.ErrCodeInvalidFormat:  CodeInvalidFormat,
	core.ErrCodeInvalidValue:   CodeInvalidValue,
	core.ErrCodeUnknownField:   CodeUnknownField,
	core.ErrCodeParseFailed:    CodeParseFailed,
	core.ErrCodeConstraint:     CodeConstraint,
	core.ErrCodeUnknownDiagram: CodeUnknownDiagram,
}

// convertValidationError maps a svggen core.ValidationError to a diagnostic
// at the MCP boundary, producing the same JSON shape as
// internal/diagnostics.Diagnostic. The svggen internal type is unchanged.
func convertValidationError(diagramType string, ve core.ValidationError) diagnostic {
	code := ve.Code
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

	// Attach next_tool_call so agents can recover without prose-parsing.
	// Shape errors (missing field, wrong type/format) point at
	// get_diagram_schema for schema discovery; constraint errors point at
	// validate_diagram so the agent can re-check after applying the fix.
	switch ve.Code {
	case core.ErrCodeRequired, core.ErrCodeInvalidType, core.ErrCodeInvalidFormat:
		d.NextToolCall = &toolCallSugg{
			Tool: "get_diagram_schema",
			ArgsTemplate: map[string]any{
				"type": diagramType,
			},
		}
	case core.ErrCodeConstraint:
		d.NextToolCall = &toolCallSugg{
			Tool: "validate_diagram",
			ArgsTemplate: map[string]any{
				"type": diagramType,
				"data": "<provide corrected data>",
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
// (schema_version, tool_list, registry, vocabularies, deprecations, features)
// so agents can parse both responses uniformly.
//
// chart_types / diagram_types are retained for backwards compatibility; new
// code should read registry.charts and registry.diagrams which carry the
// same content under the standardized field names.
type capabilitiesResponse struct {
	SchemaVersion string                    `json:"schema_version"`
	ToolList      []capabilitiesToolEntry   `json:"tool_list"`
	ChartTypes    []string                  `json:"chart_types"`
	DiagramTypes  []string                  `json:"diagram_types"`
	// ChartCapabilities and DiagramCapabilities expose per-type limits
	// (max_series, max_points, max_categories, max_nodes, max_depth),
	// overflow/density behavior, label strategy, required/optional fields, and
	// authoring surface. Values come from svggen.ChartCapabilities() and
	// svggen.DiagramCapabilitiesReady() — the same single source of truth that
	// backs json2pptx-mcp's get_chart_capabilities / get_diagram_capabilities,
	// so agents can fetch capability metadata directly from the renderer
	// without a json2pptx round-trip.
	ChartCapabilities   []svggen.ChartCapability   `json:"chart_capabilities"`
	DiagramCapabilities []svggen.DiagramCapability `json:"diagram_capabilities"`
	// Registry mirrors json2pptx-mcp's registry block. patterns is intentionally
	// empty because svggen-mcp owns chart/diagram rendering, not pattern
	// composition — keeping the key present means agents can read the same
	// shape from both servers.
	Registry      capabilitiesRegistry      `json:"registry"`
	// Vocabularies mirrors json2pptx-mcp's vocabularies block. fix_kinds
	// enumerates the chart-finding remediation enum that validate_diagram and
	// render_diagram emit; finding_codes enumerates the chart.* finding codes
	// the renderer can surface.
	Vocabularies  capabilitiesVocabularies  `json:"vocabularies"`
	Deprecations  []capabilitiesDeprecation `json:"deprecations"`
	Features      capabilitiesFeatures      `json:"features"`
}

// capabilitiesRegistry mirrors json2pptx-mcp's registry block so agents can
// read one shape across both MCP servers. svggen-mcp leaves patterns empty.
type capabilitiesRegistry struct {
	Charts   []string `json:"charts"`
	Diagrams []string `json:"diagrams"`
	Patterns []string `json:"patterns"`
}

// capabilitiesVocabularies advertises the vocabularies that drive svggen-mcp
// diagnostics: the fix_kinds returned on validation errors and the chart.*
// finding_codes surfaced during render. Mirrors json2pptx-mcp's vocabularies
// shape at the field level (subset — json2pptx adds content_types, etc.).
type capabilitiesVocabularies struct {
	FixKinds     []string `json:"fix_kinds"`
	FindingCodes []string `json:"finding_codes"`
}

// buildSvggenRegistry classifies the live svggen registry into the
// standardized {charts, diagrams, patterns} shape. Patterns is empty.
func buildSvggenRegistry() capabilitiesRegistry {
	charts, diagrams := classifyRegisteredTypes()
	return capabilitiesRegistry{
		Charts:   charts,
		Diagrams: diagrams,
		Patterns: []string{},
	}
}

// buildSvggenDeprecations enumerates retired surfaces that agents may still
// have wired up. Each entry uses path = "diagnostic.code:<legacy>" so the
// existing {path, replacement, removed_in} shape carries the legacy →
// canonical code mapping during the deprecation window. The casing migration
// landed in schema 4.23.0; the legacy lowercase codes are still accepted by
// downstream consumers as aliases but will be removed once the deprecation
// window closes.
func buildSvggenDeprecations() []capabilitiesDeprecation {
	out := make([]capabilitiesDeprecation, 0, len(legacyCodeAliases))
	for _, a := range legacyCodeAliases {
		out = append(out, capabilitiesDeprecation{
			Path:        "diagnostic.code:" + a.Legacy,
			Replacement: a.Canonical,
		})
	}
	return out
}

// buildSvggenVocabularies returns the chart-finding fix kinds and finding
// codes exposed by svggen. Both slices are sorted for stable output.
func buildSvggenVocabularies() capabilitiesVocabularies {
	fixKinds := []string{
		svggen.FixKindAlignSeries,
		svggen.FixKindExplicitScale,
		svggen.FixKindIncreaseCanvas,
		svggen.FixKindReduceItems,
		svggen.FixKindReplaceValue,
		svggen.FixKindTruncateOrSplit,
	}
	sort.Strings(fixKinds)
	findingCodes := []string{
		svggen.FindingAllZeroSeries,
		svggen.FindingAutoLogScaleApplied,
		svggen.FindingCapacityExceeded,
		svggen.FindingInvalidNumeric,
		svggen.FindingInvalidTimeFormat,
		svggen.FindingLabelClipped,
		svggen.FindingLabelEllipsized,
		svggen.FindingLabelTruncated,
		svggen.FindingLegendOverflowDropped,
		svggen.FindingNegativeOnLog,
		svggen.FindingOverflowSuppressed,
		svggen.FindingScatterLabelSkipped,
		svggen.FindingTickThinned,
		svggen.FindingZeroSumPie,
	}
	sort.Strings(findingCodes)
	return capabilitiesVocabularies{
		FixKinds:     fixKinds,
		FindingCodes: findingCodes,
	}
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
			Description: "Get the expected data schema and canonical example_values (minimal + realistic) for a specific diagram type. Mirrors show_pattern.example_values from json2pptx-mcp.",
		},
		{
			Name:        "get_capabilities",
			Description: "Returns schema version, tool list, chart/diagram types, deprecations, and feature flags.",
		},
		{
			Name:        "get_started",
			Description: "Returns the recommended ordered MCP-call sequence for a stated task (render, preflight-render, embed-in-deck).",
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
		SchemaVersion:       svggen.Version,
		ToolList:            toolCatalog(),
		ChartTypes:          chartTypes,
		DiagramTypes:        diagramTypes,
		ChartCapabilities:   svggen.ChartCapabilities(),
		DiagramCapabilities: svggen.DiagramCapabilitiesReady(),
		Registry:            buildSvggenRegistry(),
		Vocabularies:        buildSvggenVocabularies(),
		Deprecations:        buildSvggenDeprecations(),
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
