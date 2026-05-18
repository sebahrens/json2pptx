package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// get_started — first-call discovery tool for svggen-mcp
//
// Mirrors json2pptx-mcp/cmd/json2pptx/mcp_get_started.go: surfaces an ordered,
// task-keyed sequence of MCP tools so agents do not have to reverse-engineer
// the workflow. Each step pairs an MCP tool name with a one-line
// "when to call" hint, in the order an agent should invoke them.
// ---------------------------------------------------------------------------

// getStartedStep is a single step in the recommended call sequence.
type getStartedStep struct {
	Tool       string `json:"tool"`
	WhenToCall string `json:"when_to_call"`
}

// getStartedResponse is the JSON envelope for get_started.
type getStartedResponse struct {
	Task           string           `json:"task"`
	Sequence       []getStartedStep `json:"sequence"`
	AvailableTasks []string         `json:"available_tasks"`
	Notes          []string         `json:"notes,omitempty"`
}

// getStartedAvailableTasks is the canonical list of accepted task keys.
// Keep sorted; the response echoes this list verbatim.
func getStartedAvailableTasks() []string {
	tasks := []string{"render", "preflight-render", "embed-in-deck"}
	sort.Strings(tasks)
	return tasks
}

// buildGetStartedResponse returns the ordered call sequence keyed to the
// caller's stated task. Unknown or empty task strings fall back to "render",
// which is the default one-shot diagram workflow and the most common entry
// point.
func buildGetStartedResponse(task string) getStartedResponse {
	normalized := task
	switch normalized {
	case "":
		normalized = "render"
	case "render", "preflight-render", "embed-in-deck":
		// valid
	default:
		normalized = "render"
	}

	var seq []getStartedStep
	var notes []string

	switch normalized {
	case "render":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift, feature flags (dry_render, structured_errors), and which chart/diagram types are registered."},
			{Tool: "list_diagram_types", WhenToCall: "Discover canonical type names and accepted aliases before composing a render_diagram payload."},
			{Tool: "get_diagram_schema", WhenToCall: "Per chosen type, fetch the expected data shape and a minimal example so you can structure the data field correctly."},
			{Tool: "render_diagram", WhenToCall: "Produce the SVG (default) or PNG once you have a valid data payload. Use format: \"svg\" to embed inline via shape_grid icon.svg_data."},
		}
		notes = []string{
			"This is the canonical one-shot diagram workflow. Use it when you trust your data shape.",
			"If you want structured validation before rendering, prefer the \"preflight-render\" task.",
		}
	case "preflight-render":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift and confirm dry_render is advertised."},
			{Tool: "list_diagram_types", WhenToCall: "Confirm the type you want is registered (or pick the canonical name for an alias you have in hand)."},
			{Tool: "get_diagram_schema", WhenToCall: "Fetch the data schema for the chosen type before you build the payload."},
			{Tool: "validate_diagram", WhenToCall: "Validate the data payload without rendering. Returns {valid, errors} with fix.kind suggestions on failure."},
			{Tool: "render_diagram", WhenToCall: "Optional dry_run: true — re-runs layout/labeling and returns only structured findings (chart.tick_thinned, chart.label_clipped, etc.) without producing bytes."},
			{Tool: "render_diagram", WhenToCall: "Final — produce SVG or PNG once validation and dry_run are clean."},
		}
		notes = []string{
			"Use this when validation feedback is cheaper than a failed render — e.g., bulk payloads or untrusted input.",
			"validate_diagram catches required-field, type-mismatch, and constraint errors before any layout work runs.",
			"render_diagram with dry_run: true is the same render path minus the byte output — it surfaces layout findings the validator does not see.",
		}
	case "embed-in-deck":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect svggen-mcp schema_version drift before composing payloads."},
			{Tool: "list_diagram_types", WhenToCall: "Pick the chart or diagram type for the slide cell you intend to fill with svg_data."},
			{Tool: "get_diagram_schema", WhenToCall: "Fetch the data schema for the chosen type so the inline payload matches the renderer."},
			{Tool: "validate_diagram", WhenToCall: "Optional preflight — validate the data payload before rendering, especially if the deck will be regenerated frequently."},
			{Tool: "render_diagram", WhenToCall: "Render to format: \"svg\" with the deck's theme_colors in style — the returned SVG markup is what you paste into a shape_grid cell as icon.svg_data."},
		}
		notes = []string{
			"This workflow targets the shape_grid icon.svg_data inline-embed path documented in skills/generate-deck/SKILL.md.",
			"Outside this server: call json2pptx-mcp's resolve_theme(template_name) first and copy the returned theme_colors array verbatim into render_diagram's style — that keeps the SVG palette in sync with the deck template.",
			"Request format: \"svg\" (not \"png\") for inline embedding; the renderer returns the raw <svg> markup as the tool result text.",
		}
	}

	return getStartedResponse{
		Task:           normalized,
		Sequence:       seq,
		AvailableTasks: getStartedAvailableTasks(),
		Notes:          notes,
	}
}

func getStartedTool() mcp.Tool {
	return mcp.NewTool("get_started",
		mcp.WithDescription(`Returns the recommended ordered MCP-call sequence for a stated svggen-mcp task. Use this as your first call to learn the svggen-mcp workflow without reading the full tool catalog or SKILL.md.

Pass "task" to scope the sequence:
- "render" (default): one-shot diagram render — get_capabilities → list_diagram_types → get_diagram_schema → render_diagram.
- "preflight-render": validate before rendering — get_capabilities → list_diagram_types → get_diagram_schema → validate_diagram → render_diagram (dry_run) → render_diagram.
- "embed-in-deck": render SVG for inline shape_grid embedding — get_capabilities → list_diagram_types → get_diagram_schema → validate_diagram → render_diagram (format: svg). Note: call json2pptx-mcp's resolve_theme separately to keep the palette in sync.

Each step in the response includes a one-line when_to_call hint. The response also lists every available task key so agents can discover the supported scopes.`),
		mcp.WithString("task",
			mcp.Description("Optional task scope: \"render\" (one-shot, default), \"preflight-render\" (validate before render), or \"embed-in-deck\" (render SVG for inline shape_grid embedding). Unknown values fall back to \"render\"."),
		),
	)
}

func handleGetStarted(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task := ""
	if t, err := request.RequireString("task"); err == nil {
		task = t
	}

	resp := buildGetStartedResponse(task)

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal get_started response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
