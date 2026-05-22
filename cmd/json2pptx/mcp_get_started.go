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
// Surfaces a recommended single-call fast path (a workflow facade) plus an
// ordered, task-keyed sequence of MCP primitives so agents do not have to
// reverse-engineer the workflow from the 45-tool flat catalog returned by
// get_capabilities. Each step pairs an MCP tool name with a one-line
// "when to call" hint, in the order an agent should invoke them.
// ---------------------------------------------------------------------------

// getStartedStep is a single step in the recommended call sequence.
type getStartedStep struct {
	Tool       string `json:"tool"`
	WhenToCall string `json:"when_to_call"`
}

// getStartedFastPath names the single-call workflow facade an agent should
// reach for first, before falling back to the manual primitive Sequence. It is
// the "best-deck path": one tool call that internally orchestrates the same
// primitives the Sequence lists step by step.
type getStartedFastPath struct {
	Tool       string `json:"tool"`
	WhenToCall string `json:"when_to_call"`
	// FallsBackTo is the manual primitive workflow this facade collapses — always
	// the tool names in this response's Sequence — so an agent knows exactly which
	// controllable path to drop to when it needs per-step control.
	FallsBackTo []string `json:"falls_back_to"`
}

// getStartedResponse is the JSON envelope for get_started.
type getStartedResponse struct {
	Task string `json:"task"`
	// FastPath is the recommended single-call facade for this task (make_deck for
	// brief, auto_repair for revise). Present only for tasks that have a facade;
	// omitted for validate-only (pure diagnostics, no facade). Sequence remains
	// the controllable manual path agents drop to when they need per-step control.
	FastPath       *getStartedFastPath `json:"fast_path,omitempty"`
	Sequence       []getStartedStep    `json:"sequence"`
	AvailableTasks []string            `json:"available_tasks"`
	Notes          []string            `json:"notes,omitempty"`
}

// fastPathFor returns the workflow-facade fast path for a task, or nil when the
// task has no facade. FallsBackTo is the tool names in seq, so the facade and
// the manual path it collapses stay in lockstep automatically.
func fastPathFor(task string, seq []getStartedStep) *getStartedFastPath {
	tools := make([]string, len(seq))
	for i, s := range seq {
		tools[i] = s.Tool
	}
	switch task {
	case "brief":
		return &getStartedFastPath{
			Tool:        "make_deck",
			WhenToCall:  "FASTEST PATH (recommended cold start) — ONE call from a natural-language outline to a DRAFT, auto-repaired PPTX. make_deck internally chains plan_deck → expand patterns with exemplar content → auto_repair, so you skip the whole manual sequence. NOTE: the output is a SKELETON, not publishable — it fills slides with pattern exemplar PLACEHOLDER content, so the response reports content_status=\"exemplar_skeleton\", uses_exemplar_content=true, publishable=false even when the gate passes. After the call, replace the exemplar copy via repair_slide and run the rendered visual-QA / manual-review branch (see notes) before shipping. Drop to the manual primitives in `sequence` (recommend_visual → … → generate_presentation) when you want per-slide control over copy, patterns, or layout.",
			FallsBackTo: tools,
		}
	case "revise":
		return &getStartedFastPath{
			Tool:        "auto_repair",
			WhenToCall:  "FASTEST PATH — server-side convergence loop (generate → inspect → repair) that drives an existing deck JSON to a configurable quality gate in one call. Reach for it to converge a deck automatically. Drop to the manual primitives in `sequence` (validate_input → preview_presentation_plan → repair_slide → generate_presentation) when you want targeted, per-slide repairs you control.",
			FallsBackTo: tools,
		}
	default:
		return nil
	}
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
			"fast_path (make_deck) is the recommended cold-start entry point: one call to a DRAFT PPTX skeleton (NOT a publishable deck — it uses pattern exemplar placeholder content, so its response reports content_status=\"exemplar_skeleton\", uses_exemplar_content=true, publishable=false). The numbered `sequence` is the controllable path you drop to when you want to author per-slide content or drive each primitive yourself — make_deck is the workflow facade, the sequence is the manual primitives it composes.",
			"RENDERED VISUAL-QA / MANUAL-REVIEW BRANCH (do this before publishing anything): the deterministic loop in make_deck / auto_repair / score_deck never looks at a rendered pixel, and a passing gate is NOT the same as a publishable deck. Either pass visual_qa:{enabled:true} to make_deck / auto_repair to run the in-loop vision/heuristic refinement phase, OR render the final PPTX (render_deck_thumbnails / render_slide_image) and inspect it (inspect_slide_images) yourself. Always required when publishable=false / manual_review_required=true (e.g. exemplar-skeleton make_deck output or a degraded/gate-failed run) — branch on the response's blocking_reasons to see what to fix.",
			"This is the canonical new-deck workflow. Each step's output informs the next.",
			"For decks of 1-4 slides you may skip plan_deck and go straight to recommend_visual.",
			"validate_input is mandatory per SKILL.md preconditions — skipping it is a workflow violation even when preview_presentation_plan succeeds.",
			"Advanced deck-level fields (opt-in): top-level `chrome` adds a deck-wide footer (confidentiality / client / project / date) with `chrome.page_numbers` ({current}/{total} formats, auto-skipped on title/closing) and optional `chrome.section_crumb`; top-level `structure` ({cover, closing, auto_agenda, sections[]}) expands into a flat slide sequence with auto section dividers — mutually exclusive with top-level `slides`. See get_capabilities.features.{deck_chrome, page_numbers, section_structure, section_crumb} for versions and authoring hints.",
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
			"fast_path (auto_repair) is the recommended one-call path for converging an existing deck JSON to a quality gate. The numbered `sequence` is the controllable path you drop to for targeted, per-slide repairs you drive yourself — auto_repair is the workflow facade, the sequence is the manual primitives it composes.",
			"RENDERED VISUAL-QA / MANUAL-REVIEW BRANCH: a passing gate from auto_repair is NOT the same as publishable — the default loop scores from static + render-fit findings only and never inspects a rendered pixel. Check the response's publishable / manual_review_required / blocking_reasons; when review is required, either pass visual_qa:{enabled:true} to auto_repair or render the final PPTX (render_deck_thumbnails / render_slide_image) and inspect it (inspect_slide_images) before shipping.",
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
		FastPath:       fastPathFor(normalized, seq),
		Sequence:       seq,
		AvailableTasks: getStartedAvailableTasks(),
		Notes:          notes,
	}
}

func mcpGetStartedTool() mcp.Tool {
	return mcp.NewTool("get_started",
		mcp.WithDescription(`Returns the recommended workflow for a stated task: a single-call fast path (a workflow facade) plus the ordered manual primitive sequence it composes. Use this as your first call to learn the json2pptx workflow without reading the full 45-tool catalog.

The response carries two complementary paths:
- fast_path: the recommended single-call facade — make_deck for "brief", auto_repair for "revise". Call this alone for a fast result without orchestrating the tool surface yourself. NOTE: the result is not automatically publishable — make_deck returns a DRAFT skeleton with exemplar placeholder content (publishable=false), and even auto_repair's deterministic gate is not a substitute for the rendered visual-QA / manual-review branch (see notes); branch on the response's publishable / manual_review_required / blocking_reasons. Its falls_back_to lists the manual primitives it collapses. Omitted for "validate-only" (pure diagnostics, no facade).
- sequence: the controllable manual path — the ordered primitives to drive by hand when you need per-slide or per-step control.

Pass "task" to scope both paths:
- "brief" (default): authoring a new deck — fast_path make_deck; manual sequence get_capabilities → list_templates → plan_deck → recommend_visual → validate_input → preview_presentation_plan → generate_presentation → score_deck.
- "revise": modifying an existing PPTX — fast_path auto_repair; manual sequence get_capabilities → read_presentation (inspection-only; not fed downstream) → validate_input → preview_presentation_plan → repair_slide → generate_presentation → score_deck.
- "validate-only": just checking a deck JSON is valid (no fast_path) — get_capabilities → list_templates → validate_input → preview_presentation_plan.

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
