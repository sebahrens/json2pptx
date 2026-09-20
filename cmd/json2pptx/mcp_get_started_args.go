package main

// Argument templates for get_started steps (go-slide-creator-bxve)
//
// get_started is the first response every agent reads, and every step in it
// was {tool, when_to_call} with no arguments. Agents therefore called the
// tools exactly as the prose named them — list_templates{} at 153KB instead
// of 44KB with fields:"compact", get_capabilities{} at 183KB instead of the
// ~6KB default projection, list_patterns{} at 70KB instead of 31KB. The
// error-path next_tool_call blocks already carry args_template, so the shape
// existed; it was just missing from the one response that sets the tone.
//
// One map keyed by tool name, so a tool named in several sequences gets the
// same hint everywhere and there is one place to keep correct. Every key is
// checked against the tool's real input schema by
// TestGetStartedArgsTemplatesMatchToolSchemas.

// getStartedArgTemplates is the per-tool argument hint. Values are either a
// real default worth sending or a "<…>" placeholder naming what the agent must
// substitute — the point is to show the SHAPE, not to be copy-pasted blind.
var getStartedArgTemplates = map[string]map[string]any{
	// Token-relevant projections: the default for these three is the full
	// payload, and almost no first call needs it.
	"get_capabilities": {"sections": []string{"runtime", "features"}},
	"list_templates":   {"fields": "compact"},
	"list_patterns":    {"fields": "compact"},

	// Gates that are materially weaker without the flag.
	"validate_input":     {"presentation": "<deck JSON>", "fit_report": true},
	"validate_deck_spec": {"spec": "<DeckSpec>", "strict": "warn"},

	// The path-carrying steps: naming where the value comes from is the whole
	// hint, because the wrong path silently reviews the wrong deck.
	"render_deck_spec":          {"spec": "<DeckSpec>", "template": "<template id from list_templates>"},
	"render_deck_thumbnails":    {"pptx_path": "<path from the render/generate response>"},
	"render_slide_image":        {"pptx_path": "<path from the render/generate response>", "slide_index": 0},
	"generate_presentation":     {"presentation": "<deck JSON>", "strict_fit": "warn"},
	"preview_presentation_plan": {"presentation": "<deck JSON>"},
	"score_deck":                {"presentation": "<deck JSON>"},
	"inspect_slide_images":      {"slide_images": "<images from render_deck_thumbnails>"},
	"submit_visual_review": {
		"pptx_path":     "<path from the render/generate response>",
		"pptx_revision": "<content_hash from that same response>",
	},
	"read_presentation": {"pptx_path": "<path to the existing PPTX>"},
	"repair_slide":      {"presentation": "<deck JSON>", "slide_index": 0},
	"plan_deck":         {"brief": "<the user's brief>"},
	"recommend_visual":  {"intent": "<what this slide should show>"},
}

// argsTemplateFor returns the argument hint for a step's tool, or nil when the
// tool takes no argument worth pre-filling (list_slide_kinds takes none at all).
func argsTemplateFor(tool string) map[string]any {
	return getStartedArgTemplates[tool]
}
