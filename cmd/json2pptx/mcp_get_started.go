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
// reverse-engineer the workflow from the flat tool catalog returned by
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
	// Steps is the short ordered call chain around Tool (brief:
	// list_slide_kinds → validate_deck_spec → render_deck_spec →
	// render_deck_thumbnails). Omitted when the fast path is a single call.
	Steps []getStartedStep `json:"steps,omitempty"`
	// FallsBackTo is the manual primitive workflow this facade collapses — always
	// the tool names in this response's Sequence — so an agent knows exactly which
	// controllable path to drop to when it needs per-step control.
	FallsBackTo []string `json:"falls_back_to"`
}

// getStartedResponse is the JSON envelope for get_started.
type getStartedResponse struct {
	Task string `json:"task"`
	// FastPath is the recommended fast path for this task (the DeckSpec path
	// ending in render_deck_spec for brief, auto_repair for revise). Present only for tasks that have a facade;
	// omitted for validate-only (pure diagnostics, no facade). Sequence remains
	// the controllable manual path agents drop to when they need per-step control.
	FastPath       *getStartedFastPath `json:"fast_path,omitempty"`
	Sequence       []getStartedStep    `json:"sequence"`
	AvailableTasks []string            `json:"available_tasks"`
	Notes          []string            `json:"notes,omitempty"`
	Completion     completionProtocol  `json:"completion_protocol"`
	// QualityWorkflow is the server's MCP `instructions` text, echoed verbatim
	// (same Go const) so MCP clients that do not surface instructions still see
	// the quality workflow.
	QualityWorkflow string `json:"quality_workflow"`
}

type completionProtocol struct {
	DraftStatus    string `json:"draft_status"`
	CompleteStatus string `json:"complete_status"`
	Rule           string `json:"rule"`
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
			Tool:       "render_deck_spec",
			WhenToCall: "RECOMMENDED PATH for a new deck from a brief — write a compact DeckSpec ({meta:{title, archetype}, slides:[{kind, …}]}) carrying the user's real content, then render it in one call. Follow `steps`: list_slide_kinds (each kind's item_schema + copy-ready example) → validate_deck_spec → render_deck_spec → render_deck_thumbnails (look at every slide). A ~30-line DeckSpec yields a real 6-slide deck; the compiler picks patterns, layouts, and rhythm. Fix findings at their semantic_path in the spec and re-render. make_deck is NOT this path: it is a skeleton/wireframe only (exemplar placeholder copy, gate always fails). Drop to the raw primitives in `sequence` (recommend_visual → … → generate_presentation) only when you need a feature outside the DeckSpec schema.",
			Steps: []getStartedStep{
				{Tool: "list_slide_kinds", WhenToCall: "Pick a kind per slide; copy its example and match its item_schema exactly (unknown fields are reported as SEMANTIC_UNKNOWN_FIELD)."},
				{Tool: "validate_deck_spec", WhenToCall: "Check the DeckSpec; fix every error and SEMANTIC_UNKNOWN_FIELD / SEMANTIC_DENSITY warning at its path."},
				{Tool: "render_deck_spec", WhenToCall: "Compile and render the DeckSpec to a .pptx; diagnostics map back to semantic_path."},
				{Tool: "render_deck_thumbnails", WhenToCall: "Render ALL slides and inspect every returned image; repair the spec and re-render until every slide looks right."},
			},
			FallsBackTo: tools,
		}
	case "revise":
		// auto_repair is not advertised in the core profile, and a client model
		// cannot emit a call to a tool absent from tools/list — so recommending
		// it as the fast path made the RECOMMENDED path for the whole "revise"
		// task uncallable as shipped. In core mode the fast path is the
		// in-profile sequence instead (go-slide-creator-mvny).
		if !toolIsAdvertised("auto_repair") {
			return &getStartedFastPath{
				Tool:       "repair_slide",
				WhenToCall: "RECOMMENDED PATH in this tool profile — drive the repair loop yourself with the primitives below: validate_input (fit_report: true) → preview_presentation_plan to collect per-slide Fix.Kind directives → repair_slide per slide → generate_presentation → render_deck_thumbnails and look at every slide. The one-call auto_repair facade exists but is not advertised in this profile; see notes[] if you want it.",
				Steps: []getStartedStep{
					{Tool: "validate_input", WhenToCall: "Schema + fit checks on the deck JSON you intend to revise (pass fit_report: true)."},
					{Tool: "preview_presentation_plan", WhenToCall: "Dry-run to surface the per-slide fit findings whose Fix.Kind directives feed repair_slide."},
					{Tool: "repair_slide", WhenToCall: "Apply the directives per slide that has findings."},
					{Tool: "generate_presentation", WhenToCall: "Regenerate the PPTX from the repaired deck JSON."},
					{Tool: "render_deck_thumbnails", WhenToCall: "Render every slide of the new revision and inspect each image."},
				},
				FallsBackTo: tools,
			}
		}
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
			{Tool: "score_deck", WhenToCall: "Structural rules over the generated deck (0-100, basis=structural); no pixels, so this score cannot visually approve a deck."},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render the current PPTX revision to pixels; every slide must have an image."},
			{Tool: "inspect_slide_images", WhenToCall: "Inspect every rendered slide with a configured provider or host/manual reviewer (a host/manual reviewer records its all-slide verdict with submit_visual_review against the current pptx_revision); repair findings, then render and inspect the new revision again."},
		}
		notes = []string{
			"fast_path is the DeckSpec path (list_slide_kinds → validate_deck_spec → render_deck_spec → render_deck_thumbnails): author the user's real content as a compact DeckSpec and render it. The numbered `sequence` is the raw-primitive path you drop to only for features outside the DeckSpec schema.",
			"make_deck is a skeleton/wireframe tool, not a deck builder: it fills every slide with pattern exemplar placeholder copy, so it always reports gate_passed=false, uses_exemplar_content=true, and \"exemplar_content\" in blocking_reasons. Never ship its output.",
			"COMPLETION: " + mcpCompletionRule,
			"NO VISION PROVIDER? Render every slide with render_deck_thumbnails (image content blocks), inspect each image yourself, then record the verdict with submit_visual_review {pptx_path, pptx_revision, slides:[{index, verdict, image_path|image_sha256, findings?}], reviewer: host|manual}. Submit the paths/content_hashes render_deck_thumbnails returned for THIS pptx: each image is checked against the server's own render of that slide, and a recycled or foreign image is rejected. Only a complete, current-revision review with verified images and no P0/P1 findings marks the deck visually_reviewed_current_revision; an unverifiable review is recorded as reviewed_unverified_images.",
			"This is the canonical new-deck workflow. Each step's output informs the next.",
			"For decks of 1-4 slides you may skip plan_deck and go straight to recommend_visual.",
			"validate_input is mandatory per SKILL.md preconditions — skipping it is a workflow violation even when preview_presentation_plan succeeds.",
			"Advanced deck-level fields (opt-in): top-level `chrome` adds a deck-wide footer (confidentiality / client / project / date) with `chrome.page_numbers` ({current}/{total} formats, auto-skipped on title/closing) and optional `chrome.section_crumb`; top-level `structure` ({cover, closing, auto_agenda, sections[]}) expands into a flat slide sequence with auto section dividers — mutually exclusive with top-level `slides`. See get_capabilities.features.{deck_chrome, page_numbers, section_structure, section_crumb} for versions and authoring hints.",
		}
	case "revise":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift since the deck was authored."},
		}
		if toolIsAdvertised("read_presentation") {
			seq = append(seq, getStartedStep{Tool: "read_presentation", WhenToCall: "Inspection-only: extract placeholders/shapes/tables from the existing PPTX to see what was rendered. Output is NOT a PresentationInput and cannot be fed into preview_presentation_plan, repair_slide, or generate_presentation — use it to diff against your authoritative deck JSON, not as a substitute for it."})
		}
		seq = append(seq, []getStartedStep{
			{Tool: "validate_input", WhenToCall: "Run schema + fit checks (fit_report: true) on the deck JSON you intend to revise. Catches drift between the authored deck and the current engine."},
			{Tool: "preview_presentation_plan", WhenToCall: "Dry-run the deck JSON to surface per-slide fit findings whose Fix.Kind directives feed repair_slide."},
			{Tool: "repair_slide", WhenToCall: "Apply targeted fixes (the Fix.Kind vocabulary fit-report emits) to the deck JSON, per slide that has findings."},
			{Tool: "generate_presentation", WhenToCall: "Regenerate the PPTX from the repaired deck JSON."},
			{Tool: "score_deck", WhenToCall: "Confirm structural metrics improved; this is input-only evidence."},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render every slide from the repaired current revision."},
			{Tool: "inspect_slide_images", WhenToCall: "Inspect all current-revision pixels and record unresolved findings or explicit approval."},
		}...)
		notes = []string{
			"auto_repair and read_presentation are advertised only in the full tool profile. In this profile the sequence above is complete and callable as listed; to use the one-call auto_repair facade instead, ask the operator to start the server with `json2pptx mcp --tools all` (or JSON2PPTX_MCP_TOOLS=all).",
			"COMPLETION: " + mcpCompletionRule + " auto_repair's default loop scores static + render-fit findings only and never looks at a rendered pixel; check publishable / manual_review_required / blocking_reasons, then render and inspect.",
			"NO VISION PROVIDER? Render every slide with render_deck_thumbnails (image content blocks), inspect each image yourself, then record the verdict with submit_visual_review {pptx_path, pptx_revision, slides:[{index, verdict, image_path|image_sha256, findings?}], reviewer: host|manual}. Submit the paths/content_hashes render_deck_thumbnails returned for THIS pptx: each image is checked against the server's own render of that slide, and a recycled or foreign image is rejected. Only a complete, current-revision review with verified images and no P0/P1 findings marks the deck visually_reviewed_current_revision; an unverifiable review is recorded as reviewed_unverified_images.",
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
		Completion: completionProtocol{
			DraftStatus:    "draft_needs_visual_review",
			CompleteStatus: "visually_reviewed_current_revision",
			Rule:           mcpCompletionRule,
		},
		QualityWorkflow: mcpQualityWorkflow,
	}
}

func mcpGetStartedTool() mcp.Tool {
	return mcp.NewTool("get_started",
		mcp.WithDescription(getStartedToolDescription()),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaGetStarted)),
		mcp.WithString("task",
			mcp.Description("Optional task scope: \"brief\" (new deck, default), \"revise\" (modify existing deck), or \"validate-only\" (validate JSON without generating). Unknown values fall back to \"brief\"."),
		),
	)
}

// getStartedToolDescription renders the description for the active tool profile.
// get_started is the tool an agent reads first, and its description names the
// tools the response recommends — so under a profile that hides auto_repair,
// make_deck and read_presentation it must not name them. Advertising a workflow
// built on tools absent from tools/list is what made the whole "revise" path
// uncallable in core mode (go-slide-creator-mvny).
func getStartedToolDescription() string {
	reviseFastPath := "auto_repair"
	if !toolIsAdvertised("auto_repair") {
		reviseFastPath = "repair_slide (per-slide repair driven by you; this profile advertises no one-call repair facade)"
	}
	makeDeckNote := " make_deck is a skeleton/wireframe only (exemplar placeholder copy; gate always fails)."
	if !toolIsAdvertised("make_deck") {
		makeDeckNote = ""
	}
	reviseInspect := "read_presentation (inspection-only; not fed downstream) → "
	if !toolIsAdvertised("read_presentation") {
		reviseInspect = ""
	}
	return fmt.Sprintf(`Returns the recommended workflow for a stated task: a single-call fast path (a workflow facade) plus the ordered manual primitive sequence it composes. Use this as your first call to learn the json2pptx workflow without reading the full tool list.

The response carries two complementary paths:
- fast_path: the recommended path — for "brief" the DeckSpec path ending in render_deck_spec (steps: list_slide_kinds → validate_deck_spec → render_deck_spec → render_deck_thumbnails), for "revise" %[1]s.%[2]s A passing deterministic gate is never completion: render all slides and inspect every image (completion_protocol.rule). Its falls_back_to lists the manual primitives. Omitted for "validate-only" (pure diagnostics, no facade).
- sequence: the controllable manual path — the ordered primitives to drive by hand when you need per-slide or per-step control.

Pass "task" to scope both paths:
- "brief" (default): authoring a new deck — fast_path render_deck_spec (DeckSpec); manual sequence get_capabilities → list_templates → plan_deck → recommend_visual → validate_input → preview_presentation_plan → generate_presentation → score_deck.
- "revise": modifying an existing PPTX — fast_path %[1]s; manual sequence get_capabilities → %[3]svalidate_input → preview_presentation_plan → repair_slide → generate_presentation → score_deck.
- "validate-only": just checking a deck JSON is valid (no fast_path) — get_capabilities → list_templates → validate_input → preview_presentation_plan.

Each step in the response includes a one-line when_to_call hint. The response also lists every available task key so agents can discover the supported scopes, and quality_workflow repeats the server instructions (the 5-step quality workflow).`, reviseFastPath, makeDeckNote, reviseInspect)
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
