package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
)

// ---------------------------------------------------------------------------
// get_started — first-call discovery tool
//
// Surfaces an ordered, task-keyed sequence of MCP tools so agents do not have
// to reverse-engineer the workflow from the 35+ flat tool catalog returned by
// get_capabilities. Each step pairs an MCP tool name with a one-line
// "when to call" hint, in the order an agent should invoke them.
// ---------------------------------------------------------------------------

// getStartedStep is a single step in the recommended call sequence.
type getStartedStep struct {
	Tool       string `json:"tool"`
	WhenToCall string `json:"when_to_call"`
}

// getStartedResponse is the JSON envelope for get_started.
type getStartedResponse struct {
	Task         string           `json:"task"`
	Sequence     []getStartedStep `json:"sequence"`
	AvailableTasks []string       `json:"available_tasks"`
	Notes        []string         `json:"notes,omitempty"`
}

// getStartedAvailableTasks is the canonical list of accepted task keys.
// Keep sorted; the response echoes this list verbatim.
func getStartedAvailableTasks() []string {
	tasks := []string{"brief", "revise", "validate-only"}
	sort.Strings(tasks)
	return tasks
}

// buildGetStartedResponse returns the ordered call sequence keyed to the
// caller's stated task. Unknown or empty task strings fall back to "brief",
// which is the default new-deck workflow and the most common entry point.
func buildGetStartedResponse(task string) getStartedResponse {
	normalized := task
	switch normalized {
	case "":
		normalized = "brief"
	case "brief", "revise", "validate-only":
		// valid
	default:
		normalized = "brief"
	}

	var seq []getStartedStep
	var notes []string

	switch normalized {
	case "brief":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift and feature flags before doing anything else."},
			{Tool: "list_templates", WhenToCall: "Pick a template; read canonical_layout_ids, color_roles, layout_summaries, table_styles."},
			{Tool: "plan_deck", WhenToCall: "Turn the user's brief into an ordered slide outline with per-slide patterns and narrative roles. Recommended for any deck > 4 slides."},
			{Tool: "recommend_visual", WhenToCall: "Per slide intent, rank candidate layouts/patterns/charts/diagrams before committing to one."},
			{Tool: "validate_input", WhenToCall: "Once the full deck JSON is assembled, run schema + fit checks (pass fit_report: true). Cheapest single gate before preview/generate; SKILL.md lists this as a precondition for generate_presentation."},
			{Tool: "preview_presentation_plan", WhenToCall: "Dry-run the validated deck JSON to verify layout selection, placeholder mapping, and fit findings without rendering."},
			{Tool: "generate_presentation", WhenToCall: "Produce the PPTX once validate + preview are clean. Pass strict_fit: \"warn\" (default) or \"strict\" for refuse-on-overflow."},
			{Tool: "score_deck", WhenToCall: "Final — score the generated deck (0-100) for variety, coverage, and structure."},
		}
		notes = []string{
			"This is the canonical new-deck workflow. Each step's output informs the next.",
			"For decks of 1-4 slides you may skip plan_deck and go straight to recommend_visual.",
			"validate_input is mandatory per SKILL.md preconditions — skipping it is a workflow violation even when preview_presentation_plan succeeds.",
		}
	case "revise":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift since the deck was authored."},
			{Tool: "read_presentation", WhenToCall: "Inspection-only: extract placeholders/shapes/tables from the existing PPTX to see what was rendered. Output is NOT a PresentationInput and cannot be fed into preview_presentation_plan, repair_slide, or generate_presentation — use it to diff against your authoritative deck JSON, not as a substitute for it."},
			{Tool: "validate_input", WhenToCall: "Run schema + fit checks (fit_report: true) on the deck JSON you intend to revise. Catches drift between the authored deck and the current engine."},
			{Tool: "preview_presentation_plan", WhenToCall: "Dry-run the deck JSON to surface per-slide fit findings whose Fix.Kind directives feed repair_slide."},
			{Tool: "repair_slide", WhenToCall: "Apply targeted fixes (the Fix.Kind vocabulary fit-report emits) to the deck JSON, per slide that has findings."},
			{Tool: "generate_presentation", WhenToCall: "Regenerate the PPTX from the repaired deck JSON."},
			{Tool: "score_deck", WhenToCall: "Final — confirm the revision improved the deck score."},
		}
		notes = []string{
			"Use this when modifying or repairing an existing PPTX deck.",
			"You MUST supply the authoritative deck JSON for validate_input, preview_presentation_plan, repair_slide, and generate_presentation. read_presentation is a verification aid only — it does not reconstruct a PresentationInput.",
			"If the original deck JSON is unavailable, re-author it from the brief (see task=brief) rather than trying to round-trip read_presentation through the editing tools.",
		}
	case "validate-only":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift before validating against possibly-stale assumptions."},
			{Tool: "list_templates", WhenToCall: "Confirm the deck's template exists and matches expected canonical_layout_ids."},
			{Tool: "validate_input", WhenToCall: "Run schema + fit checks on the deck JSON. Pass fit_report: true for density/overflow findings."},
			{Tool: "preview_presentation_plan", WhenToCall: "Optional — dry-run the plan to inspect layout selection without rendering."},
		}
		notes = []string{
			"Use this when you only need to confirm a deck JSON is valid (no generation).",
			"validate_input is the cheapest single gate that catches the most errors.",
		}
	}

	return getStartedResponse{
		Task:           normalized,
		Sequence:       seq,
		AvailableTasks: getStartedAvailableTasks(),
		Notes:          notes,
	}
}

func mcpGetStartedTool() mcp.Tool {
	return mcp.NewTool("get_started",
		mcp.WithDescription(`Returns the recommended ordered MCP-call sequence for a stated task. Use this as your first call to learn the json2pptx workflow without reading the full 35+ tool catalog.

Pass "task" to scope the sequence:
- "brief" (default): authoring a new deck — get_capabilities → list_templates → plan_deck → recommend_visual → validate_input → preview_presentation_plan → generate_presentation → score_deck.
- "revise": modifying an existing PPTX — get_capabilities → read_presentation (inspection-only; not fed downstream) → validate_input → preview_presentation_plan → repair_slide → generate_presentation → score_deck.
- "validate-only": just checking a deck JSON is valid — get_capabilities → list_templates → validate_input → preview_presentation_plan.

Each step in the response includes a one-line when_to_call hint. The response also lists every available task key so agents can discover the supported scopes.`),
		mcp.WithRawOutputSchema(outputSchemaGetStarted),
		mcp.WithString("task",
			mcp.Description("Optional task scope: \"brief\" (new deck, default), \"revise\" (modify existing deck), or \"validate-only\" (validate JSON without generating). Unknown values fall back to \"brief\"."),
		),
	)
}

func handleGetStarted(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task := ""
	if t, err := request.RequireString("task"); err == nil {
		task = t
	}

	resp := buildGetStartedResponse(task)

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal get_started response: %v", err)), nil
	}
	return mcpResult, nil
}
