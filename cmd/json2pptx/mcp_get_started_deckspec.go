package main

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// get_started for the default "deckspec" tool profile (go-slide-creator-7bdn6,
// go-slide-creator-mvdt5).
//
// The default profile lists twelve tools and no raw-JSON path, yet get_started
// answered with the workflow written for the full catalogue: a raw_sequence of
// tools the agent could not see, three mentions of make_deck, and pointers to
// SKILL.md, QUALITY.md and WORKFLOW.md that an MCP-only agent does not have.
// It was also 12 KB of the ~95 KB an agent read before authoring anything.
//
// This is the same workflow written for that profile: every step names a
// listed tool, the rules a file used to carry are stated in the step that
// needs them, and the tools the profile hides are named once, in
// hidden_tools, as callable but unlisted.

// hiddenToolsNote marks tool names a response mentions that tools/list does
// not carry in the active profile.
type hiddenToolsNote struct {
	Names []string `json:"names"`
	Note  string   `json:"note"`
}

// hiddenToolsExplanation is the one sentence that goes with such a list.
const hiddenToolsExplanation = "Not in this server's tools/list, but callable by name; `json2pptx mcp --tools core` (or --tools all) lists them. The DeckSpec path needs none of them."

// toolNamePattern matches any registered tool name as a whole word.
var toolNamePattern = sync.OnceValue(func() *regexp.Regexp {
	names := mcpToolNames()
	// Longest first, so render_slide_image_from_json is not read as
	// render_slide_image.
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])(` + strings.Join(quoted, "|") + `)($|[^A-Za-z0-9_])`)
})

// unlistedToolsIn returns, sorted, the registered tools text names that the
// active profile's tools/list does not carry.
func unlistedToolsIn(text string) []string {
	seen := map[string]bool{}
	var out []string
	// Matches consume their delimiters, so adjacent names need a rescan from
	// just past each name.
	for start := 0; start < len(text); {
		loc := toolNamePattern().FindStringSubmatchIndex(text[start:])
		if loc == nil {
			break
		}
		name := text[start+loc[4] : start+loc[5]]
		if !seen[name] && !toolIsAdvertised(name) {
			seen[name] = true
			out = append(out, name)
		}
		start += loc[5]
	}
	sort.Strings(out)
	return out
}

// hiddenToolsIn builds the hidden_tools note for a response body, or nil when
// every tool it names is listed.
func hiddenToolsIn(text string) *hiddenToolsNote {
	names := unlistedToolsIn(text)
	if len(names) == 0 {
		return nil
	}
	return &hiddenToolsNote{Names: names, Note: hiddenToolsExplanation}
}

// deckSpecThumbnailRubric is the per-slide check an agent applies to each
// rendered image; it used to be a pointer to WORKFLOW.md.
const deckSpecThumbnailRubric = "action title of at most two lines, body proves the title, readable text, aligned edges, no orphans, balanced whitespace, meaningful accents, chart units and a source"

// deckSpecReviewStep is the closing step of every DeckSpec sequence.
func deckSpecReviewStep() getStartedStep {
	return getStartedStep{Tool: "submit_visual_review", WhenToCall: "Record a verdict for every slide of the current revision with the image_path or image_sha256 render_deck_thumbnails returned and each open defect as a finding (P0/P1 blocks approval). Only an all-slide, current-revision approval completes the deck."}
}

// deckSpecProfileGetStarted returns the workflow for a task as the default
// profile serves it. ok is false for a task it has no projection for.
func deckSpecProfileGetStarted(task string) (fast *getStartedFastPath, seq []getStartedStep, notes []string, hidden *hiddenToolsNote, ok bool) {
	const template = "<template name from list_templates>"
	const deckID = "<deck_id from the validate/render response>"
	pptx := map[string]any{"pptx_path": "<pptx_path from render_deck_spec>"}
	rawPath := &hiddenToolsNote{
		Names: []string{"validate_input", "generate_presentation", "repair_slide", "list_patterns", "show_pattern", "expand_pattern", "analyze_deck_rhythm", "explain_deck_spec", "get_capabilities"},
		Note:  hiddenToolsExplanation,
	}
	switch task {
	case "brief":
		fast = &getStartedFastPath{
			Tool:        "render_deck_spec",
			WhenToCall:  "A new deck from a brief: write a DeckSpec ({meta:{title, template}, slides:[{kind, …}]}) carrying the user's real content and render it — follow `sequence`. About 30 lines yield a six-slide deck; the compiler picks patterns, layouts and rhythm.",
			FallsBackTo: []string{},
		}
		seq = []getStartedStep{
			{Tool: "plan_deck", WhenToCall: "Optional for 1-4 slides. Drafts deck_spec (a kind per narrative slot, __FILL__ titles), slots[] with guidance and the brief's facts, and unplaced_facts. Rewrite every title as a full-sentence action title of at most 15 words that carries its number.",
				ArgsTemplate: map[string]any{"brief": "<the user's brief>", "format": "deckspec"}},
			{Tool: "list_templates", WhenToCall: "Pick a template name.", ArgsTemplate: map[string]any{"fields": listFieldsNames}},
			{Tool: "list_slide_kinds", WhenToCall: "Choose a kind per slide from the catalogue, then call it again with kinds:[<chosen>] for each kind's copy-ready example; fields:[\"brief\"] returns field signatures and text budgets, fields:[\"item_schema\"] descriptions and aliases. An unknown field is reported as SEMANTIC_UNKNOWN_FIELD. For a visual no kind draws (swimlane, pyramid, gantt, …) recommend_visual returns a runnable slide."},
			{Tool: "validate_deck_spec", WhenToCall: "Check the spec on the template you will render on. Fix every finding with blocking:true at its path (a JSON Pointer into the spec); send a next_tool_call marked patch_verified as given. Keep the deck_id: later edits are deck_id + patch.",
				ArgsTemplate: map[string]any{"spec": "<DeckSpec>", "template": template, "strict": "warn"}},
			{Tool: "render_deck_spec", WhenToCall: "Render the stored spec to a .pptx; it reports the findings validate did. deterministic_ready is a precondition for review, not approval.",
				ArgsTemplate: map[string]any{"deck_id": deckID}},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render ALL slides of this revision and look at every image: " + deckSpecThumbnailRubric + ". Repair with deck_id + patch, re-render changed_slides; at most three repair rounds.",
				ArgsTemplate: pptx},
			deckSpecReviewStep(),
		}
		notes = []string{
			"ONE SLIDE OR TWO? Views that prove the SAME title (a trend, its KPIs, the dated plan) may share a slide: kind regions. An unrelated conclusion gets its own slide.",
			"CHROME AND SECTIONS: page numbers (title and closing skipped), the meta.date footer and, with sections, the tracker are on by default; meta.chrome {confidentiality, client_name, project_code, footer_date, section_crumb, tracker, page_numbers:{enabled, format, skip}} adjusts them. structure {cover, auto_agenda, sections:[{title, slides[]}], closing} instead of flat slides generates the agenda and sequential dividers. Every kind takes notes and source; meta.source is the deck default.",
			"Never ship placeholder copy (__FILL__, a recipe's \"Replace with …\"): it is a blocking SEMANTIC_WEAK_CONTENT.",
		}
		return fast, seq, notes, rawPath, true
	case "revise":
		fast = &getStartedFastPath{
			Tool:        "render_deck_spec",
			WhenToCall:  "A deck authored as a DeckSpec is revised without resending it: every validate_deck_spec / render_deck_spec response carries a deck_id. Send deck_id with patch:[{op, path, value}] — op replace | add | remove | move | copy; path a JSON Pointer (/slides/3/title, /meta/template; add at /slides/6 inserts; a slide id works for its index: /slides/s4/title). changed_slides lists the slides that look different, slide_changes classifies every affected slide, stored:false means the patch was not kept (a refused render, or dry_run).",
			FallsBackTo: []string{},
		}
		patch := []any{map[string]any{"op": "replace", "path": "/slides/3/title", "value": "<new title>"}}
		seq = []getStartedStep{
			{Tool: "validate_deck_spec", WhenToCall: "Check an edit before rendering it: the patch is applied to the stored spec. It also reads the deck back (read: \"spec\" | \"history\" | \"diff:2..5\" | a slide id), finds or replaces a figure everywhere (find, replace), restores a revision (restore) and forks (fork:true).",
				ArgsTemplate: map[string]any{"deck_id": deckID, "patch": patch}},
			{Tool: "render_deck_spec", WhenToCall: "Render the revision (add patch here to edit and render in one call). Omit template: the deck keeps the one it is bound to; patch /meta/template to change it.",
				ArgsTemplate: map[string]any{"deck_id": deckID}},
			{Tool: "render_deck_thumbnails", WhenToCall: "Look at the slides changed_slides names (an empty list means nothing looks different): " + deckSpecThumbnailRubric + ". Re-patch until they read right, then make one full-deck pass over the revision you ship.",
				ArgsTemplate: map[string]any{"pptx_path": "<pptx_path from render_deck_spec>", "slide_indices": "<changed_slides from render_deck_spec>"}},
			deckSpecReviewStep(),
		}
		notes = []string{
			"A deck_id is per server process and expires one hour after its last stored revision. It is not storage: keep your own copy of the spec and send it again when the handle is gone.",
			"A patch is not a review: a one-field edit still needs the changed slides rendered and looked at.",
			"No DeckSpec for the deck (an existing .pptx, or raw json2pptx JSON)? Author it as a DeckSpec from the brief (task=brief). The raw repair chain is among the hidden tools.",
		}
		hidden = &hiddenToolsNote{
			Names: []string{"read_presentation", "validate_input", "preview_presentation_plan", "repair_slide", "generate_presentation", "analyze_deck_rhythm"},
			Note:  hiddenToolsExplanation,
		}
		return fast, seq, notes, hidden, true
	case "validate-only":
		seq = []getStartedStep{
			{Tool: "list_templates", WhenToCall: "Confirm the template the deck names exists.", ArgsTemplate: map[string]any{"fields": listFieldsNames}},
			{Tool: "validate_deck_spec", WhenToCall: "Run the DeckSpec's render into a scratch directory and report what render_deck_spec would: a finding blocks only when blocking is true. templates:[\"all\"] (or a list of names) also reports the spec on other templates.",
				ArgsTemplate: map[string]any{"spec": "<DeckSpec>", "template": template, "strict": "warn"}},
		}
		notes = []string{
			"Nothing is written: validation stores the spec under a deck_id and returns findings only.",
			"Raw json2pptx JSON is validated by a hidden tool (validate_input).",
		}
		hidden = &hiddenToolsNote{Names: []string{"validate_input", "preview_presentation_plan"}, Note: hiddenToolsExplanation}
		return nil, seq, notes, hidden, true
	case "onboard-template":
		seq = []getStartedStep{
			{Tool: "examine_template", WhenToCall: "First: pass template_path (the .pptx) and base_dir (a directory containing it). Read canonical_coverage: the four content-bearing families (title-slide, section-divider, one-content, qa-closing) must be present, and derivable_layouts[].ready says which of the rest the engine can synthesize."},
			{Tool: "describe_finding", WhenToCall: "For each finding examine_template reports — TPL.LAYOUT.MISSING_ROLE above all — to learn what the missing role costs and how to fix the template."},
			{Tool: "list_templates", WhenToCall: "Pass the same template_path to read the file's aspect_ratio, layout_count and table_styles in the shape of a registered template."},
			{Tool: "render_deck_spec", WhenToCall: "Render a short test DeckSpec (title, section, a content kind, closing) with template_path and base_dir: the template's smoke test before real content.",
				ArgsTemplate: map[string]any{"spec": "<DeckSpec>", "template_path": "<the .pptx>", "base_dir": "<directory containing it>"}},
			{Tool: "render_deck_thumbnails", WhenToCall: "Render the test deck and LOOK at it: a template can pass every structural check and still put white text on a white band. Then start task=brief with the same template_path.", ArgsTemplate: pptx},
		}
		notes = []string{
			"template_path is how a template the server has NOT registered reaches the engine: examine_template, list_templates and render_deck_spec accept it. The `template` argument takes a registered NAME only.",
			"CONTAINMENT: template_path is resolved against base_dir (the server's CWD when omitted) and must stay inside it after ~/$ENV expansion and symlink evaluation; a path outside it is refused with INVALID_PATH.",
			"PERMANENT INSTALL: copying the .pptx into runtime.templates_dir (in this response) registers it live, addressable by file name without .pptx.",
			"A template missing a canonical family still renders — those slides fall back to its blank canvas — but the deck loses that family's design.",
		}
		return nil, seq, notes, nil, true
	}
	return nil, nil, nil, nil, false
}
