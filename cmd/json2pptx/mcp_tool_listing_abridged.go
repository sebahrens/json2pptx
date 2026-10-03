package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// The default profile's abridged tools/list (go-slide-creator-mvdt5).
//
// First contact — the initialize instructions, tools/list, get_started, the
// template names and the kind catalogue — has a 40 KiB budget, and tools/list
// was 33 KB of it. Most of that was prose an agent does not need to choose a
// tool or to call it the usual way: the full shape of every response, the
// rules for rarely used arguments, one sentence per enum value, the DeckSpec
// meta outline. The default profile now lists each tool with a description
// that says what it does and what it returns, and arguments described in a
// line.
//
// Nothing is removed from the surface: every argument, type, enum, required
// list and oneOf stays in the listing, the registered tool is untouched (the
// "core" and "all" profiles list it in full), and
// get_started tool:"<name>" returns the full description and input schema of
// any listed tool on demand.

// abridgedListing is what the default profile says about one tool.
type abridgedListing struct {
	// Description replaces the tool description.
	Description string
	// Args maps an argument to its abridged description. A nested property is
	// addressed by a slash path (slides/findings/category), stepping through
	// array items. An empty string drops the description.
	Args map[string]string
	// Edit makes a structural change no description can.
	Edit func(schema map[string]any)
}

// abridgedListings holds the default profile's wording per tool.
var abridgedListings = map[string]abridgedListing{
	"describe_finding": {
		Description: `Explain one finding or error code: summary, severity, whether it blocks, when it is emitted, remediation steps and a before/after example. Call it for any code you do not recognize (fit and pattern codes, chart.*, SEMANTIC_*, input / template / render errors). An unknown code returns the closest known one.`,
	},
	"examine_template": {
		Description: `Examine a PPTX template and return its capability report inline (writes no files): theme fonts and colours, canonical layout coverage (title-slide, section-divider, one-content, qa-closing), derivable layouts, each layout's placeholders with capacities, the grid, and findings such as TPL.LAYOUT.MISSING_ROLE. Use it before authoring on a user-provided template. Supply EXACTLY ONE of template_name or template_path.`,
		Args: map[string]string{
			"template_name": "Registered template name. This OR template_path.",
			"template_path": "Local .pptx, resolved against base_dir (else the server CWD) and required to stay inside it. This OR template_name.",
		},
	},
	"get_started": {
		Description: `The recommended workflow for a task, written for the tools this server lists. Call it first.

Default for content-bearing decks: author real content as a DeckSpec; ` + "`list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`" + `, then render and inspect every slide: a passing gate is never completion.

This tools/list is abridged: tool:"<name>" returns that tool's full description and input schema.`,
		Args: map[string]string{
			"skill_version": "schema_version of the installed generate-deck skill; an older one returns skill_warning with the refresh command.",
			"task":          "brief (default, a new deck) | revise (deck_id + patch) | validate-only | onboard-template (vet a user-supplied .pptx).",
		},
	},
	"list_slide_kinds": {
		Description: `Discover DeckSpec slide kinds. Without arguments: one line per kind (what it is for, required_fields). kinds:[…]: each named kind's full summary, typical fields and copy-ready example. fields selects detail instead: brief (one-line field signatures with text budgets), item_schema (fields with descriptions and aliases), budgets (per-field text budgets, measured on template), compositions (pattern / layout overrides), example, item_schema_full (closed JSON Schema). kinds:["raw_json2pptx"] also returns a composed example (several views under one title).`,
		Args: map[string]string{
			"preview": "true: also return an image of each named kind's example (1-4 kinds), rendered on template.",
		},
	},
	"list_templates": {
		Description: `List presentation templates. fields="names": one small row per template (name, aspect ratio, layout count, fonts, primary fill), enough to pick one. The default (compact) adds table styles, canonical layout IDs, colour roles (contrast-safe accents, the ink each accent carries) and body font sizes; fields="full" adds theme colours, grid, layout summaries with placeholders and the supported chart / diagram types. Paginated: cursor + page_size, next_cursor when more remain.`,
		Args: map[string]string{
			"cursor":        "next_cursor of the previous page.",
			"fields":        "names, compact (default) or full.",
			"filter":        "Case-insensitive substring of the template name.",
			"mode":          "Legacy; prefer fields.",
			"page_size":     "Templates per page (default 50, 1-200).",
			"read_only":     "true: write no layout-preview PNGs to the cache.",
			"template":      "Return this one template.",
			"template_path": "One local .pptx the server has not registered; must stay inside base_dir (else the server CWD). Not with template.",
		},
	},
	"plan_deck": {
		Description: `Plan a deck from a brief: an ordered slide outline with a narrative role, layout, pattern and content seeds per slide. format:"deckspec" (recommended) returns deck_spec, a DeckSpec draft whose kinds follow the brief, plus slots[] (path, guidance, facts): fill it using list_slide_kinds, then validate_deck_spec → render_deck_spec. A brief that lists its slides gets one slide per item, in order; deck instructions ("8 slides", "with an agenda") return as constraints[]; every other clause is routed to a slide or listed in unplaced_facts. With format "raw", slides[] are advisory records, not slide inputs.`,
		Args: map[string]string{
			"audience":     `Target audience, e.g. "board of directors"; influences pattern choice.`,
			"brief":        "What the deck is for and what it must say, in natural language.",
			"format":       `"deckspec": a DeckSpec draft for validate_deck_spec / render_deck_spec. "raw" (default): a pattern outline with raw slide skeletons.`,
			"must_include": `Pattern names the plan must use, e.g. ["kpi-3up"].`,
			"slide_budget": "Target slide count (3-30). Default: the count the brief states, else 10.",
			"template":     "Template name: each slide then carries template_support, and a pattern the template cannot host is replaced.",
		},
	},
	"recommend_visual": {
		Description: `Rank visuals for a slide intent across layouts, named patterns, charts, diagrams, raw shape_grid and the regions kind. Every candidate carries data_contract (keys, limits) and a runnable next_tool_call (render_deck_spec with a complete DeckSpec: the kind that compiles to it, else a raw_json2pptx slide); also_as names other forms of the same visual, differs_by what separates near ties. Compose candidates put several views on one slide. A visual with no renderer (Sankey) returns unsupported_visual. With candidates, ONLY those names are scored and all are returned.`,
		Args: map[string]string{
			"candidates":      "Shortlist to rank instead: layout, pattern, chart or diagram names, raw_shape_grid, or a compose:<a>+<b> name this tool returned.",
			"content_hints":   "Optional hints that refine ranking.",
			"intent":          `What the slide should show, e.g. "compare 3 vendors on 5 dimensions".`,
			"prefer_variety":  "true: penalize patterns named in recent_patterns.",
			"preview":         "true: also return an image of each leading candidate (max 4).",
			"recent_patterns": "Pattern names on the preceding slides, in order.",
			"template":        "Template name: candidates carry template_support and ones it cannot host are demoted; previews render on it.",
		},
	},
	"render_deck_spec": {
		Description: `Compile a DeckSpec and render it to a .pptx: the one-call path for a NEW deck. Returns pptx_path, deck_id, diagnostics[] (validate_deck_spec's findings, at JSON Pointer paths), deterministic_ready, publishable and blocking_reasons[]. success means the file was WRITTEN; deterministic_ready means no blocking diagnostic remains; publishable also needs an approved all-slide visual verdict and is false on a fresh render: render every slide with render_deck_thumbnails, inspect the images, then record the verdict with submit_visual_review. quality_summary is an input heuristic, not a visual verdict.`,
		Args: map[string]string{
			"base_dir":        "Absolute directory that relative asset paths in the spec and template_path resolve against and must stay inside. Default: the deck_id's last render root, else the server CWD.",
			"deck_id":         "Handle a validate_deck_spec or render_deck_spec response returned, sent INSTEAD of spec; add patch to edit. Per-process, expires after 1 hour: then send the spec again.",
			"output_filename": "Name of the .pptx inside the server's output directory. Default: derived from meta.title and a digest of the spec.",
			"patch":           `Edits to deck_id's spec, applied first: [{op, path, value | from}]. path is a JSON Pointer; a slide is named by index or id (/slides/3/title, /slides/s4/title); add at /slides/6 inserts, "-" appends. All ops apply or none; the response lists changed_slides.`,
			"restore":         "Revision of deck_id to start from; patch applies on top.",
			"spec":            "The DeckSpec to render: a JSON object ({meta:{title, template}, slides:[{kind, …}]}) or a YAML/JSON string. Send this OR deck_id.",
			"template":        "Template for this call; overrides meta.template. list_templates lists names.",
			"template_path":   "Local .pptx to render with; must stay inside base_dir. Not with template. Check it with examine_template first.",
		},
	},
	"render_deck_thumbnails": {
		Description: `Render a PPTX's slides and return them as MCP image blocks (one JPEG per slide, in slide order) you can look at directly, plus metadata: slides[].index, path (the full-resolution PNG submit_visual_review verifies) and content_hash. Needs LibreOffice and ImageMagick on PATH; cached by file content hash. Use density 50–75 for a full-deck pass (15 slides are ~600KB at 50); after a repair, pass only render_deck_spec's changed_slides as slide_indices.`,
		Args: map[string]string{
			"density":             "DPI, 25-150 (default 50).",
			"include_base64_json": "Legacy: true returns base64 inside the JSON instead of image blocks.",
			"max_slides":          "Render the first N slides (default 50). Not with slide_indices.",
			"slide_indices":       `Render ONLY these slides, by 0-based index or slide id, e.g. [4, "costs"]. Not with max_slides.`,
		},
	},
	"score_deck": {
		Description: `Score a deck's STRUCTURAL quality with deterministic rules: overall_score (0-100, basis="structural"), per-slide scores and findings with fix suggestions. It generates the deck into a temporary directory and scores the generated structure; it never looks at rendered pixels, so it cannot visually approve a deck (use render_deck_thumbnails and look). Send a DeckSpec deck_id, or a raw presentation.`,
		Args: map[string]string{
			"allow_degraded_scoring": "true: score on static analysis when the render pass fails. Default: a render failure is a blocking RENDER_EVIDENCE_INCOMPLETE finding.",
			"deck_id":                "Stored deck handle from validate_deck_spec / render_deck_spec. This OR presentation.",
			"mode":                   "Only 'deterministic' (default) is implemented.",
			"presentation":           "A raw json2pptx presentation ({template, slides[]}). Not a DeckSpec: send its deck_id instead.",
			"presentation/slides":    "",
			"presentation/template":  "",
			"slide_indices":          "0-based slides to score; the rest are skipped.",
			"template":               "Template override. Default: the presentation's own.",
		},
	},
	"submit_visual_review": {
		Description: `Record the visual review verdict for a rendered PPTX: the completion step once you (or a human) have inspected the rendered slides. slides[] must cover EVERY slide exactly once, each with the image render_deck_thumbnails returned for it (path or content_hash); partial coverage, a stale pptx_revision or any other image is rejected and nothing is recorded. status is "visually_reviewed_current_revision" only when every slide is approved with no P0/P1 finding and the render's deterministic gate passed.`,
		Args: map[string]string{
			"pptx_revision":            "sha256 (content_hash) of the reviewed PPTX, as the render returned it.",
			"slides/findings/category": "",
			"slides/findings/severity": "P0 unreadable or broken, P1 major, P2 minor, P3 nitpick. P0/P1 block approval.",
		},
	},
	"validate_deck_spec": {
		Args: map[string]string{
			"spec": `The DeckSpec to validate: a JSON object ({meta:{title, template, …}, slides:[{kind, …}]}; structure:{sections[]} instead of slides generates agenda and dividers) or a YAML/JSON string. Send this OR deck_id. list_slide_kinds lists the kinds (kinds:[chosen] returns examples, fields:[brief] or [item_schema] a kind's fields); get_started tool:"validate_deck_spec" returns the meta and structure outline. Unknown fields report SEMANTIC_UNKNOWN_FIELD.`,
		},
		Edit: func(schema map[string]any) {
			// The meta / structure outline and the kind enum: list_slide_kinds
			// names the kinds, and the outline is one get_started call away.
			if spec := schemaArgument(schema, "spec"); spec != nil {
				delete(spec, "properties")
				if branches, ok := spec["oneOf"].([]any); ok {
					for _, b := range branches {
						if branch, ok := b.(map[string]any); ok {
							delete(branch, "description")
						}
					}
				}
			}
		},
	},
}

// schemaArgument walks a slash path of property names from an input schema,
// stepping through array items, and returns that property's schema.
func schemaArgument(schema map[string]any, path string) map[string]any {
	node := schema
	for _, name := range strings.Split(path, "/") {
		if items, ok := node["items"].(map[string]any); ok {
			node = items
		}
		props, _ := node["properties"].(map[string]any)
		next, ok := props[name].(map[string]any)
		if !ok {
			return nil
		}
		node = next
	}
	return node
}

// withAbridgedListing applies the default profile's wording to one tool. A
// tool with no entry, or an argument path the schema does not have, is left
// as registered (TestAbridgedListingKeepsEveryArgument fails on the latter).
func withAbridgedListing(tool mcp.Tool) mcp.Tool {
	entry, ok := abridgedListings[tool.Name]
	if !ok {
		return tool
	}
	if entry.Description != "" {
		tool.Description = entry.Description
	}
	if len(entry.Args) == 0 && entry.Edit == nil {
		return tool
	}
	raw := tool.RawInputSchema
	if len(raw) == 0 {
		var err error
		if raw, err = json.Marshal(tool.InputSchema); err != nil {
			return tool
		}
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return tool
	}
	paths := make([]string, 0, len(entry.Args))
	for path := range entry.Args {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		arg := schemaArgument(schema, path)
		if arg == nil {
			continue
		}
		if text := entry.Args[path]; text == "" {
			delete(arg, "description")
		} else {
			arg["description"] = text
		}
	}
	if entry.Edit != nil {
		entry.Edit(schema)
	}
	out, err := json.Marshal(schema)
	if err != nil {
		return tool
	}
	tool.RawInputSchema = out
	tool.InputSchema = mcp.ToolInputSchema{}
	return tool
}

// advertisedToolNames lists, sorted, the tools the active profile's tools/list
// carries.
func advertisedToolNames() []string {
	var out []string
	for _, name := range mcpToolNames() {
		if toolIsAdvertised(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// toolDetail is get_started tool:"<name>": what the abridged listing left out.
type toolDetail struct {
	Tool        string          `json:"tool"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	Listed      bool            `json:"listed"`
	Note        string          `json:"note,omitempty"`
}

// fullToolListing returns a tool as the active profile listed it before
// abridging: the registered tool, with the DeckSpec outline in place of the
// closed per-kind schema under the default profile.
func fullToolListing(name string) (toolDetail, error) {
	ctor, ok := toolConstructors()[name]
	if !ok {
		return toolDetail{}, fmt.Errorf("unknown tool %q", name)
	}
	tool := ctor()
	if activeToolProfile() == toolProfileDeckSpec {
		switch name {
		case "validate_deck_spec":
			tool = withDeckSpecOutlineInput(tool)
		case "render_deck_spec":
			tool = withDeckSpecReferenceInput(tool)
		}
	}
	raw := tool.RawInputSchema
	if len(raw) == 0 {
		var err error
		if raw, err = json.Marshal(tool.InputSchema); err != nil {
			return toolDetail{}, err
		}
	}
	detail := toolDetail{Tool: name, Description: tool.Description, InputSchema: json.RawMessage(raw), Listed: toolIsAdvertised(name)}
	if !detail.Listed {
		detail.Note = hiddenToolsExplanation
	}
	return detail, nil
}

// toolDetailResponse wraps a tool's detail and marks the tools its text names
// that the active profile does not list.
func toolDetailResponse(detail toolDetail) map[string]any {
	resp := map[string]any{"tool_detail": detail}
	text := detail.Description + string(detail.InputSchema)
	if detail.Listed {
		// A hidden tool's own name is already explained by its note.
		if hidden := hiddenToolsIn(text); hidden != nil {
			resp["hidden_tools"] = hidden
		}
	} else if hidden := hiddenToolsIn(text + " " + detail.Tool); hidden != nil {
		resp["hidden_tools"] = hidden
	}
	return resp
}
