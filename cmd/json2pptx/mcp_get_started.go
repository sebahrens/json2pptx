package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/semantic"
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
	// ArgsTemplate shows the arguments worth sending with this step —
	// token-relevant projections (fields:"compact"), the flags a gate is weak
	// without (fit_report), and "<…>" placeholders naming where a path or hash
	// comes from. Filled once in handleGetStarted from getStartedArgTemplates,
	// so a tool reads the same way in every sequence and a step added later
	// cannot ship without its hint (go-slide-creator-bxve).
	ArgsTemplate map[string]any `json:"args_template,omitempty"`
}

// withArgs fills each step's ArgsTemplate from the shared table.
func withArgs(steps []getStartedStep) []getStartedStep {
	for i := range steps {
		if steps[i].ArgsTemplate == nil {
			steps[i].ArgsTemplate = argsTemplateFor(steps[i].Tool)
		}
	}
	return steps
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
	// TaskWarning is set when the caller asked for a task this tool does not
	// know. The response still carries the "brief" workflow — blocking an
	// agent's first call helps nobody — but it says so rather than letting a
	// typo look deliberate (go-slide-creator-bxve).
	TaskWarning string `json:"task_warning,omitempty"`
	// FastPath is the recommended fast path for this task — the DeckSpec path
	// ending in render_deck_spec for both brief (author it) and revise (patch the
	// deck_id the server already holds). Present only for tasks that have a facade;
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
	QualityWorkflow string `json:"quality_workflow,omitempty"`
	// Runtime is what this server can actually do, in ~200 bytes: whether it can
	// render (the completion rule depends on it), and the directories it reads
	// and writes. It rides the FIRST call because the alternative was learning it
	// from get_capabilities, 183KB into the session, or from the mandatory
	// completion step failing (go-slide-creator-a7fh).
	Runtime getStartedRuntime `json:"runtime"`
}

// getStartedRuntime is the environment block every get_started response carries.
type getStartedRuntime struct {
	// RenderAvailable reports whether LibreOffice and ImageMagick are on PATH.
	// When false the render_* tools and the visual-approval step cannot run.
	RenderAvailable bool `json:"render_available"`
	// MissingCommands names what is absent, e.g. ["libreoffice/soffice", "magick"].
	MissingCommands []string `json:"missing_commands,omitempty"`
	// TemplatesDir and OutputDir are where this server reads templates and
	// writes decks.
	TemplatesDir string `json:"templates_dir"`
	OutputDir    string `json:"output_dir"`
	// SettingsWriteEnabled reports whether the gated write tools are allowed.
	SettingsWriteEnabled bool `json:"settings_write_enabled"`
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
				{Tool: "list_slide_kinds", WhenToCall: fmt.Sprintf("Pick a kind per slide; copy its example and match its item_schema exactly (unknown fields are reported as SEMANTIC_UNKNOWN_FIELD). Its `compositions` list is what a slide's optional pattern/layout override will honour — anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE. A kind reaches %d of the %d patterns; for one of the other %d (swimlane, pyramid, value-chain, scqa-summary and the rest — SKILL.md lists them all) use kind raw_json2pptx and carry the pattern block verbatim, as its example shows.", len(semantic.ReachablePatterns()), len(semantic.ReachablePatterns())+len(semantic.UnreachablePatterns()), len(semantic.UnreachablePatterns()))},
				{Tool: "validate_deck_spec", WhenToCall: "Check the DeckSpec; fix every error and SEMANTIC_UNKNOWN_FIELD / SEMANTIC_DENSITY warning at its path."},
				{Tool: "render_deck_spec", WhenToCall: "Compile and render the DeckSpec to a .pptx; diagnostics map back to semantic_path."},
				{Tool: "render_deck_thumbnails", WhenToCall: "Render ALL slides and inspect every returned image; repair the spec and re-render until every slide looks right."},
			},
			FallsBackTo: tools,
		}
	case "revise":
		// The revise fast path used to be the raw-deck repair chain (auto_repair,
		// or repair_slide when the profile hid it), which named no DeckSpec at all
		// — so an agent that had just authored a deck the recommended way had no
		// documented way to change it and re-sent the whole spec by hand
		// (go-slide-creator-voxp). The first branch is now the spec it already
		// holds, and the cheapest form of that: a deck_id and a patch.
		return &getStartedFastPath{
			Tool:       "render_deck_spec",
			WhenToCall: "RECOMMENDED PATH when the deck was authored as a DeckSpec (task=brief) — you do not resend it. Every validate_deck_spec / render_deck_spec response carries a deck_id: the spec this server is holding. Send deck_id INSTEAD of spec, with patch:[{op:\"replace\", path:\"/slides/3/title\", value:\"…\"}] — op is replace | add | remove, path is a JSON Pointer into the spec (/meta/template to restyle the deck, /slides/6 with add to insert a slide, /slides/2 with remove to drop one). A four-edit revision is one call of a few hundred bytes instead of a full spec re-upload. The response's changed_slides names the 0-based slides that differ, so render_deck_thumbnails them as slide_indices instead of pulling the whole deck again. Handles live 1 hour per server process; if one expires, send the spec again. Still holding the spec and no handle? Edit it and call render_deck_spec — findings come back at semantic_path, so you fix the field the finding names. The raw chain in `sequence` is for a deck authored as raw json2pptx JSON, not as a DeckSpec.",
			Steps: []getStartedStep{
				{Tool: "validate_deck_spec", WhenToCall: "Send deck_id + patch to check an edit before rendering it; the patch is applied to the stored deck, so the next call sees it."},
				{Tool: "render_deck_spec", WhenToCall: "Render the revision (deck_id + patch, or the edited spec). Omit template and the handle keeps the one the last render used."},
				{Tool: "render_deck_thumbnails", WhenToCall: "Pull only the slides named by changed_slides (pass them as slide_indices) and look at each one; re-patch and re-render until they read right, then make one full-deck pass over the revision you ship."},
			},
			FallsBackTo: tools,
		}
	default:
		return nil
	}
}

// getStartedAvailableTasks is the canonical list of accepted task keys.
// Keep sorted; the response echoes this list verbatim.
func getStartedAvailableTasks() []string {
	tasks := []string{"brief", "onboard-template", "revise", "validate-only"}
	sort.Strings(tasks)
	return tasks
}

// buildGetStartedResponse returns the ordered call sequence keyed to the
// caller's stated task. Unknown or empty task strings fall back to "brief",
// which is the default new-deck workflow and the most common entry point.
// buildGetStartedResponse keeps the two-argument form callers and tests use; it
// includes the prose narrative, which is what a CLI caller wants.
func buildGetStartedResponse(task string, rt getStartedRuntime) getStartedResponse {
	return buildGetStartedResponseOpts(task, rt, true)
}

// buildGetStartedResponseOpts is the form the MCP handler uses, where verbose
// is false by default: an MCP client already received the same prose as the
// initialize instructions (go-slide-creator-bxve).
func buildGetStartedResponseOpts(task string, rt getStartedRuntime, verbose bool) getStartedResponse {
	// get_started is the first call an agent makes, so an unrecognised task
	// still answers with the brief workflow rather than blocking. What it must
	// not do is answer SILENTLY: it used to echo task:"brief" with no hint that
	// something else had been asked for, so a mistyped task looked like a
	// deliberate one (go-slide-creator-bxve).
	normalized := task
	var taskWarning string
	switch normalized {
	case "":
		normalized = "brief"
	case "brief", "revise", "validate-only", "onboard-template":
		// valid
	default:
		taskWarning = fmt.Sprintf(
			"Unknown task %q — answering with %q. Valid tasks: %s.",
			task, "brief", strings.Join(getStartedAvailableTasks(), ", "))
		normalized = "brief"
	}

	var seq []getStartedStep
	var notes []string

	switch normalized {
	case "brief":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift and feature flags before doing anything else. The default sections ([runtime, features, deprecations], ~6 KB) are all you need; pass sections:[\"tools\"] only if you want the catalogue tools/list already sent."},
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
			"DECK CHROME, per path. On the fast_path (DeckSpec): `meta.chrome` {confidentiality, client_name, project_code, footer_date, section_crumb, page_numbers:{enabled, format, skip}} — footer_date defaults to meta.date — plus `meta.viewing_mode` and `meta.accent_strategy`; every slide kind also takes `notes` (speaker notes) and `source` (footnote line). On the raw path (generate_presentation): the same block as TOP-LEVEL `chrome`, plus top-level `structure` ({cover, closing, auto_agenda, sections[]}), which expands into a flat slide sequence with auto section dividers and is mutually exclusive with top-level `slides`. `structure` has no DeckSpec equivalent: author the sections as slides, or drop to the raw path via compile_deck_spec(include_compiled_json:true). See get_capabilities.features.{deck_chrome, page_numbers, section_structure, section_crumb}.",
		}
	case "revise":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift since the deck was authored. sections:[\"runtime\"] is enough for that (~0.4 KB)."},
		}
		if toolIsAdvertised("read_presentation") {
			seq = append(seq, getStartedStep{Tool: "read_presentation", WhenToCall: "Inspection-only: extract placeholders/shapes/tables from the existing PPTX to see what was rendered. Output is NOT a PresentationInput and cannot be fed into preview_presentation_plan, repair_slide, or generate_presentation — use it to diff against your authoritative deck JSON, not as a substitute for it."})
		}
		seq = append(seq, []getStartedStep{
			{Tool: "validate_input", WhenToCall: "Run schema + fit checks (fit_report: true) on the deck JSON you intend to revise. Catches drift between the authored deck and the current engine."},
			{Tool: "preview_presentation_plan", WhenToCall: "Dry-run the deck JSON to surface per-slide fit findings whose Fix.Kind directives feed repair_slide."},
			{Tool: "repair_slide", WhenToCall: "Apply targeted fixes (the Fix.Kind vocabulary fit-report emits) to the deck JSON, per slide that has findings. Re-check one repaired slide with render_slide_image, or a handful with render_deck_thumbnails slide_indices, before re-rendering the deck."},
			{Tool: "generate_presentation", WhenToCall: "Regenerate the PPTX from the repaired deck JSON."},
			{Tool: "score_deck", WhenToCall: "Confirm structural metrics improved; this is input-only evidence."},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render every slide from the repaired current revision."},
			{Tool: "inspect_slide_images", WhenToCall: "Inspect all current-revision pixels and record unresolved findings or explicit approval."},
		}...)
		notes = []string{
			"TWO REVISE PATHS, and the one you want depends on how the deck was authored. (1) DeckSpec deck — fast_path: deck_id + patch on validate_deck_spec / render_deck_spec. The server holds the spec, you send the edit, changed_slides tells you which thumbnails to re-pull. (2) Raw json2pptx deck — the numbered `sequence` below: validate_input → preview_presentation_plan → repair_slide → generate_presentation, with the deck JSON in every call. Do not use path 2 on a DeckSpec deck: repair_slide edits compiled slide JSON, so its fixes do not travel back into the spec and are lost on the next render_deck_spec.",
			"A deck_id is per server process and lives 1 hour, refreshed each time you use it. It is not storage: if the server restarts, or the handle expires, send the spec again and you get a new one. Keep your own copy of the spec — the handle saves bytes, it is not the deck's home.",
			"COMPLETION: " + mcpCompletionRule + " A patch is not a review: a one-field edit still needs the changed slides rendered and looked at before the deck is complete.",
			"NO VISION PROVIDER? Render every slide with render_deck_thumbnails (image content blocks), inspect each image yourself, then record the verdict with submit_visual_review {pptx_path, pptx_revision, slides:[{index, verdict, image_path|image_sha256, findings?}], reviewer: host|manual}. Submit the paths/content_hashes render_deck_thumbnails returned for THIS pptx: each image is checked against the server's own render of that slide, and a recycled or foreign image is rejected. Only a complete, current-revision review with verified images and no P0/P1 findings marks the deck visually_reviewed_current_revision; an unverifiable review is recorded as reviewed_unverified_images.",
			"Use this when modifying or repairing an existing PPTX deck.",
			"You MUST supply the authoritative deck JSON for validate_input, preview_presentation_plan, repair_slide, and generate_presentation. read_presentation is a verification aid only — it does not reconstruct a PresentationInput.",
			"If the original deck JSON is unavailable, re-author it from the brief (see task=brief) rather than trying to round-trip read_presentation through the editing tools.",
		}
		if !toolIsAdvertised("auto_repair") {
			notes = append(notes, "The full tool profile (`json2pptx mcp --tools all`, or JSON2PPTX_MCP_TOOLS=all) adds two raw-path conveniences this profile hides: auto_repair, a one-call server-side convergence loop over a raw deck, and apply_deck_patch, a pure slide-level transform of raw deck JSON. Neither is needed for either path above.")
		}
	case "onboard-template":
		// Bring-your-own template (go-slide-creator-ydbk). Every step here is
		// callable by an MCP agent holding only the .pptx file: nothing in this
		// sequence needs an operator to install anything.
		seq = []getStartedStep{
			{Tool: "examine_template", WhenToCall: "First — pass template_path (the .pptx) and base_dir (a directory containing it). Read canonical_coverage: the four content-bearing families (title-slide, section-divider, one-content, qa-closing) must be present, and derivable_layouts[].ready tells you which of the rest the engine can synthesize."},
			{Tool: "describe_finding", WhenToCall: "For each finding examine_template reports — TPL.LAYOUT.MISSING_ROLE above all — to learn what the missing role costs and how to fix the template. A missing family means slides of that type fall back to the blank canvas."},
			{Tool: "list_templates", WhenToCall: "Pass the same template_path to read the file's aspect_ratio, layout_count and table_styles in the same shape as a registered template, so the rest of your authoring is unchanged."},
			{Tool: "generate_presentation", WhenToCall: "Render one test slide per canonical layout, with presentation.template_path set to the .pptx and base_dir to its directory — the deck's own smoke test before you author real content."},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render the test deck to pixels and LOOK at it: a template can pass every structural check and still put white text on a white band. Then start the brief workflow with the same template_path."},
		}
		notes = []string{
			"template_path is how a template that is NOT registered on the server reaches the engine. It is accepted by examine_template, list_templates, validate_input, preview_presentation_plan, generate_presentation (as presentation.template_path) and render_deck_spec. The `template` argument takes a registered NAME only and will reject a path.",
			"CONTAINMENT: template_path is resolved against base_dir (the server's CWD when you omit it) and must stay inside it after ~/$ENV expansion and symlink evaluation. Pass base_dir as the directory holding the .pptx; a path outside it is refused with INVALID_PATH.",
			"PERMANENT INSTALL: copying the .pptx into the server's templates directory (get_capabilities(sections:[\"runtime\"]).runtime.templates_dir) registers it live, with no restart — it is then addressable by file name without .pptx as a normal `template`. That needs filesystem access to that directory; template_path does not.",
			"A template missing a canonical family still renders — those slides fall back to the template's blank canvas — but the deck loses the family's design. Fix the template rather than working around it if you will reuse it.",
		}
	case "validate-only":
		seq = []getStartedStep{
			{Tool: "get_capabilities", WhenToCall: "First — detect schema_version drift before validating against possibly-stale assumptions. sections:[\"runtime\"] is enough for that (~0.4 KB)."},
			{Tool: "list_templates", WhenToCall: "Confirm the deck's template exists and matches expected canonical_layout_ids."},
			{Tool: "validate_input", WhenToCall: "Run schema + fit checks on the deck JSON. Pass fit_report: true for density/overflow findings."},
			{Tool: "preview_presentation_plan", WhenToCall: "Optional — dry-run the plan to inspect layout selection without rendering."},
		}
		notes = []string{
			"Use this when you only need to confirm a deck JSON is valid (no generation).",
			"validate_input is the cheapest single gate that catches the most errors.",
		}
	}

	fastPath := fastPathFor(normalized, seq)
	if fastPath != nil {
		fastPath.Steps = withArgs(fastPath.Steps)
	}

	resp := getStartedResponse{
		Task:           normalized,
		TaskWarning:    taskWarning,
		FastPath:       fastPath,
		Sequence:       withArgs(seq),
		AvailableTasks: getStartedAvailableTasks(),
		Notes:          notes,
		Completion: completionProtocol{
			DraftStatus:    "draft_needs_visual_review",
			CompleteStatus: "visually_reviewed_current_revision",
			Rule:           mcpCompletionRule,
		},
		// quality_workflow repeats the MCP initialize instructions verbatim, and
		// completion_protocol repeats one of its lines in structured form. Every
		// MCP client already received the instructions, so sending 1.4 KB of the
		// same prose in the one response every agent reads first is pure weight
		// (go-slide-creator-bxve). The CLI passes verbose, because a CLI caller
		// never saw them.
		QualityWorkflow: qualityWorkflowFor(rt, verbose),
		Runtime:         rt,
	}
	if !rt.RenderAvailable {
		degradeForMissingRenderTooling(&resp, rt.MissingCommands)
	}
	return resp
}

// degradeForMissingRenderTooling rewrites a response for a server that cannot
// render. Every path here ends in "render it and look at it", which on such a
// server is an instruction to do the impossible: the render steps are dropped,
// the step that now ends the path says to deliver the file and declare it
// unreviewed, and the completion rule is replaced with the one that CAN be
// honoured (go-slide-creator-a7fh).
func degradeForMissingRenderTooling(resp *getStartedResponse, missing []string) {
	resp.Sequence = closeWithDelivery(dropRenderSteps(resp.Sequence), len(resp.Sequence))
	if resp.FastPath != nil {
		resp.FastPath.Steps = closeWithDelivery(dropRenderSteps(resp.FastPath.Steps), len(resp.FastPath.Steps))
		resp.FastPath.FallsBackTo = stepTools(resp.Sequence)
	}
	resp.Completion = completionProtocol{
		DraftStatus:    "draft_needs_visual_review",
		CompleteStatus: "draft_needs_visual_review",
		Rule:           renderToolingWarning(missing),
	}
	resp.Notes = append([]string{"RENDER TOOLING MISSING (" + strings.Join(missing, ", ") + "): the render_* and inspect_slide_images tools fail on this server, so the visual-approval step in the completion rule cannot be performed here. The deck can still be authored, validated and delivered — say it is unreviewed. Install LibreOffice and ImageMagick to restore it."}, resp.Notes...)
}

// closeWithDelivery turns the step a path now ends on into the delivery step,
// but only when the path lost its render tail: a validate-only path never
// produced a deck and has nothing to hand over or disclaim. before is the step
// count prior to dropping, so an unchanged path is left exactly as it was.
func closeWithDelivery(steps []getStartedStep, before int) []getStartedStep {
	if len(steps) == 0 || len(steps) == before {
		return steps
	}
	last := &steps[len(steps)-1]
	last.WhenToCall = last.WhenToCall +
		" LAST STEP ON THIS SERVER: rendering is unavailable, so hand back pptx_path (or read the json2pptx://deck/<name> resource) and state plainly that the deck is UNREVIEWED — no slide has been looked at. Do not claim the completion rule was met."
	return steps
}

// dropRenderSteps removes the steps that need the render toolchain.
func dropRenderSteps(steps []getStartedStep) []getStartedStep {
	out := make([]getStartedStep, 0, len(steps))
	for _, step := range steps {
		switch step.Tool {
		case "render_deck_thumbnails", "render_slide_image", "inspect_slide_images", "submit_visual_review":
			continue
		}
		out = append(out, step)
	}
	return out
}

// stepTools projects a step list to its tool names.
func stepTools(steps []getStartedStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Tool
	}
	return out
}

func mcpGetStartedTool() mcp.Tool {
	return mcp.NewTool("get_started",
		mcp.WithDescription(getStartedToolDescription()),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaGetStarted)),
		mcp.WithString("task",
			mcp.Description("Optional task scope: \"brief\" (new deck, default), \"revise\" (modify existing deck), \"validate-only\" (validate JSON without generating), or \"onboard-template\" (vet and render with a user-supplied .pptx). An unknown value answers with \"brief\" and says so in task_warning."),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include quality_workflow, the prose workflow narrative. Omitted by default because it repeats the MCP initialize instructions verbatim, which every client already received; completion_protocol carries the same rule in structured form. Pass true if you did not read the initialize instructions."),
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

func (mc *mcpConfig) handleGetStarted(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task := ""
	if raw, ok := request.GetArguments()["task"]; ok && raw != nil {
		// A non-string task used to be ignored silently: get_started answered
		// with the brief workflow as though nothing had been asked, which is the
		// one wrong-typed argument in the whole surface that produced no error at
		// all (go-slide-creator-6072). An unknown STRING still falls back to
		// brief — that is a documented default, not a mistake.
		t, isString := raw.(string)
		if !isString {
			return argInvalidValue("get_started", diagnostics.CodeInvalidParameter, "task",
				fmt.Sprintf("task must be a string, got %s; one of %s", jsonTypeName(raw), strings.Join(getStartedAvailableTasks(), ", ")),
				"string", "brief", nil), nil
		}
		task = t
	}

	verbose := request.GetArguments()["verbose"] == true
	resp := buildGetStartedResponseOpts(task, mc.getStartedRuntime(), verbose)

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal get_started response: %v", err)), nil
	}
	return mcpResult, nil
}

// renderDependencyStatus reports whether the render toolchain is installed. It
// is a variable so a test can pin the answer: without that, every workflow
// assertion depends on what happens to be on the runner's PATH — CI has no
// LibreOffice, so the degraded workflow would be the one under test there
// (go-slide-creator-a7fh).
var renderDependencyStatus = render.DependencyStatus

// getStartedRuntime reports what this server can do, for the runtime block.
func (mc *mcpConfig) getStartedRuntime() getStartedRuntime {
	available, missing := renderDependencyStatus()
	templatesDir, outputDir := "", ""
	if mc != nil {
		templatesDir, outputDir = resolveRuntimeDirs(mc.templatesDir, mc.outputDir)
	}
	return getStartedRuntime{
		RenderAvailable:      available,
		MissingCommands:      missing,
		TemplatesDir:         templatesDir,
		OutputDir:            outputDir,
		SettingsWriteEnabled: settingsWriteAllowed(),
	}
}

// qualityWorkflowFor returns the prose workflow narrative only when the caller
// asked for it. See the comment at its use site.
func qualityWorkflowFor(rt getStartedRuntime, verbose bool) string {
	if !verbose {
		return ""
	}
	return mcpInstructionsFor(rt.RenderAvailable, rt.MissingCommands)
}
