// Package diagnostics — describe-finding registry.
//
// describe-finding is the single read surface an agent uses to resolve the
// meaning of any finding code emitted anywhere in the pipeline, in one extra
// tool call, without scanning docs. This file backs it from the shared
// diagnostics taxonomy so the metadata covers the dotted-namespace codes
// (TPL/FIT/GRID/RENDER/POLICY/INPUT) emitted by MCP, CLI, HTTP generation,
// validation, repair, render, inspect, palette audit, and output validation —
// not just the patterns-only fit findings.
//
// Lookup is unified through Describe: the patterns.FindingMeta registry owns the
// lowercase fit/chart/pattern codes (and the few SCREAMING_SNAKE codes it
// already documents, e.g. the TEMPLATE_METADATA_* warnings and UNKNOWN_ENUM),
// and codeMetaRegistry below owns the remaining diagnostics.AllCodes() codes.
// TestDescribeCoversAllDiagnosticCodes asserts that every declared code resolves,
// so adding a code to codes.go without a describe entry fails CI.
package diagnostics

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Severity ranks reused from the describe_finding output schema. Diagnostics
// codes are MCP error / warning results, so they map onto "refuse" (the engine
// refused to proceed) or "review" (advisory; the run continued).
const (
	describeSeverityRefuse = "refuse"
	describeSeverityReview = "review"
)

// Describe returns the agent-facing metadata for a finding code, or
// (nil, false) when no entry exists. The code may be a bare legacy code
// ("MISSING_PARAMETER", "placeholder_overflow", "chart.zero_sum_pie") or a
// dotted namespaced code from a finding envelope ("INPUT.MISSING_PARAMETER",
// "FIT.placeholder_overflow") — the namespace prefix is stripped before lookup
// so the describe_command examples emitted on the wire are runnable verbatim.
func Describe(code string) (*patterns.FindingMeta, bool) {
	legacy := stripNamespacePrefix(code)
	if m, ok := patterns.GetFindingMeta(legacy); ok {
		return m, true
	}
	if m, ok := codeMetaRegistry[legacy]; ok {
		out := m
		return &out, true
	}
	return nil, false
}

// AllDescribableCodes returns the sorted union of every code that Describe can
// resolve: the patterns fit/chart/pattern codes plus the diagnostics codes
// declared here. describe_finding advertises this list on the unknown-code
// error path.
func AllDescribableCodes() []string {
	set := make(map[string]struct{}, len(codeMetaRegistry)+64)
	for _, c := range patterns.AllFindingMetaCodes() {
		set[c] = struct{}{}
	}
	for c := range codeMetaRegistry {
		set[c] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// stripNamespacePrefix removes a leading "<NS>." when NS is one of the declared
// finding namespaces. Non-namespace dotted prefixes (e.g. "chart.") are left
// intact so they reach the patterns registry verbatim.
func stripNamespacePrefix(code string) string {
	for _, ns := range AllNamespaces() {
		if strings.HasPrefix(code, ns+".") {
			return code[len(ns)+1:]
		}
	}
	return code
}

// codeMetaRegistry documents every diagnostics.AllCodes() code that the
// patterns registry does not already own. Keep it in sync with codes.go:
// TestDescribeCoversAllDiagnosticCodes fails the build when a declared code has
// no entry here or in the patterns registry.
var codeMetaRegistry = map[string]patterns.FindingMeta{
	// ---- Codes emitted as bare string literals by cmd/json2pptx ----
	//
	// These used to be absent from the catalogue, so describe_finding — the
	// documented "use after any tool returns a finding you do not recognize"
	// recovery path — answered UNKNOWN_FINDING_CODE for them
	// (go-slide-creator-7zrt). TestDescribeFindingCoversCodesEmittedInCmd keeps
	// the set closed: a new Code string literal in cmd/json2pptx fails the build
	// until it is described here.

	"REQUIRED": {
		Code:        "REQUIRED",
		Summary:     "A required field of the request payload is absent or empty.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Payload validation (preview, patch, repair, dry-run) finds a required field missing — e.g. a presentation with no template, or with an empty slides array.",
		RemediationSteps: []string{
			"Supply the field named in the finding's path.",
			"For \"template\", call list_templates to pick a valid name; for \"slides\", supply at least one slide object.",
		},
		ExampleBefore: `{"slides": [...]}  // no "template"`,
		ExampleAfter:  `{"template": "midnight-blue", "slides": [...]}`,
		RelatedCodes:  []string{CodeMissingParameter, CodeInvalidParameter},
	},

	"FILE_READ_ERROR": {
		Code:        "FILE_READ_ERROR",
		Summary:     "An input file named on the command line could not be read.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The CLI validate path cannot open or read the deck JSON it was given (missing path, wrong permissions, a directory).",
		RemediationSteps: []string{
			"Check the path exists and is a readable file, not a directory.",
			"Prefer an absolute path; a relative one resolves against the process working directory.",
		},
		RelatedCodes: []string{CodeInvalidPath},
	},

	"PATCH_ERROR": {
		Code:        "PATCH_ERROR",
		Summary:     "A deck patch could not be applied to the presentation.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "apply_deck_patch / the dry-run patch path fails while applying an operation — usually a path that does not exist in the target deck, or an operation shape the patch format does not accept.",
		RemediationSteps: []string{
			"Re-read the target deck and confirm every patch path resolves against it (slide indices are 0-based).",
			"Apply the operations one at a time to find which one fails; the message carries the underlying error.",
		},
		RelatedCodes: []string{CodeInvalidPath, CodeInvalidSlideIndex},
	},

	"INVALID_STRUCTURE": {
		Code:        "INVALID_STRUCTURE",
		Summary:     "The deck-level \"structure\" envelope could not be expanded into a slide sequence.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A deck uses the top-level \"structure\" field (cover / closing / sections / auto_agenda) and its shape is invalid — e.g. a section with no slides, or a malformed cover block.",
		RemediationSteps: []string{
			"Fix the structure block named in the message; each section needs a title and at least one slide.",
			"Call get_input_schema for the structure envelope's exact shape, or drop to a flat top-level \"slides\" array instead.",
		},
		RelatedCodes: []string{"STRUCTURE_AND_SLIDES", CodeInvalidSlide},
	},

	"STRUCTURE_AND_SLIDES": {
		Code:        "STRUCTURE_AND_SLIDES",
		Summary:     "The deck sets both \"structure\" and \"slides\", which are mutually exclusive.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A presentation carries the top-level \"structure\" envelope AND a top-level \"slides\" array. structure expands INTO a slide sequence, so supplying both is ambiguous.",
		RemediationSteps: []string{
			"Keep \"structure\" and remove \"slides\" to let the envelope generate the sequence (cover, sections, dividers, closing).",
			"Or keep \"slides\" and remove \"structure\" to author the flat sequence yourself.",
		},
		ExampleBefore: `{"structure": {...}, "slides": [...]}`,
		ExampleAfter:  `{"structure": {...}}`,
		RelatedCodes:  []string{"INVALID_STRUCTURE"},
	},

	"COMPOSE_SEGMENT_EXPAND_FAILED": {
		Code:        "COMPOSE_SEGMENT_EXPAND_FAILED",
		Summary:     "One segment of a slide's compose envelope failed to expand into a shape grid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "preview / validate expands each compose segment's pattern; a segment whose pattern name is unknown or whose values fail the pattern's value schema reports this, naming the slide.",
		RemediationSteps: []string{
			"Read the message for the underlying pattern error — it is the same one generate_presentation would report.",
			"Call show_pattern for the segment's pattern to check its value schema, then fix that segment's values.",
			"Confirm the segment count and directions are within the compose caps reported by get_capabilities.",
		},
		RelatedCodes: []string{CodePatternError, CodeUnknownPattern},
	},

	"no_emoji_violation": {
		Code:        "no_emoji_violation",
		Summary:     "Deck content contains an emoji codepoint, which is rejected everywhere in the input.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The boundary validator in validate_input / generate_presentation finds an emoji or pictographic character in any authored string — icon fields, pattern values, shape text, titles, bullets, headers, captions, table cells.",
		RemediationSteps: []string{
			"Remove the emoji from the field named in the finding's path.",
			"For an icon, use a bundled SVG icon name instead (call list_icons), or supply path / url / svg_data.",
			"Plain Unicode symbols outside the emoji range (arrows like → and ←) are still allowed in text, but not in icon fields.",
		},
		ExampleBefore: `{"header": "🚀 Launch"}`,
		ExampleAfter:  `{"header": "Launch", "icon": {"name": "rocket"}}`,
		RelatedCodes:  []string{CodeIconBundledNameUnknown},
	},

	"grid_violation": {
		Code:        "grid_violation",
		Summary:     "A shape grid's geometry does not sit on the deck's rhythm grid.",
		Severity:    describeSeverityReview,
		WhenEmitted: "Rhythm-grid analysis finds shape bounds that do not align to the resolved column/row grid, which reads as visual drift between slides that should look aligned.",
		RemediationSteps: []string{
			"Snap the offending bounds to the grid values the message names, or drop the explicit bounds and let the pattern place the block.",
			"When an off-grid position is deliberate (a deliberately offset hero element), the finding is advisory and can be left.",
		},
		RelatedCodes: []string{patterns.ErrCodeSlideBoundsOverflow},
	},

	"style_collision": {
		Code:        "style_collision",
		Summary:     "Two style sources set the same property on one element, so one silently loses.",
		Severity:    describeSeverityReview,
		WhenEmitted: "Validation finds an explicit per-element style that collides with a deck-level default or a table style — e.g. a cell_style default and an inline cell style setting the same fill.",
		RemediationSteps: []string{
			"Decide which source should own the property and remove it from the other.",
			"Prefer the deck-level \"defaults\" block for deck-wide choices and inline styles only for genuine exceptions.",
		},
		RelatedCodes: []string{"redundant_field"},
	},

	"redundant_field": {
		Code:        "redundant_field",
		Summary:     "Both a typed content field and the legacy \"value\" field are set; the typed field wins and \"value\" is ignored.",
		Severity:    describeSeverityReview,
		WhenEmitted: "A content item carries e.g. both text_value and value. The engine uses the typed field, so the \"value\" content never renders.",
		RemediationSteps: []string{
			"Remove the \"value\" field and keep the typed one named in the message (text_value, bullets_value, table_value, chart_value, …).",
			"If the \"value\" content is the one you wanted, move it into the typed field.",
		},
		ExampleBefore: `{"type": "text", "text_value": "New", "value": "Old"}`,
		ExampleAfter:  `{"type": "text", "text_value": "New"}`,
		RelatedCodes:  []string{"legacy_authoring_form"},
	},

	"legacy_authoring_form": {
		Code:        "legacy_authoring_form",
		Summary:     "A content item uses the legacy \"value\" field instead of the typed field for its content type.",
		Severity:    describeSeverityReview,
		WhenEmitted: "Validation finds \"value\" on a content item whose type has a dedicated typed field. It still works, but the typed field is the supported authoring form.",
		RemediationSteps: []string{
			"Rename \"value\" to the typed field the message names for this content type.",
		},
		ExampleBefore: `{"type": "bullets", "value": ["a", "b"]}`,
		ExampleAfter:  `{"type": "bullets", "bullets_value": ["a", "b"]}`,
		RelatedCodes:  []string{"redundant_field"},
	},

	"kind_not_supported": {
		Code:        "kind_not_supported",
		Summary:     "repair_slide was asked for a fix kind that does not exist.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A repair request names a fix kind in neither vocabulary (get_capabilities.vocabularies.repair_fix_kinds / advisory_fix_kinds). The response's supported_kinds lists what repair_slide can apply.",
		RemediationSteps: []string{
			"Pick a kind from the response's supported_kinds field.",
			"Check the spelling against get_capabilities.vocabularies.repair_fix_kinds.",
		},
		RelatedCodes: []string{"semantic_review_required", "advisory_fix_kind", "wrong_kind_for_target"},
	},

	"advisory_fix_kind": {
		Code:        "advisory_fix_kind",
		Summary:     "The fix kind is real, but its remedy is an authoring decision rather than a mechanical edit.",
		Severity:    describeSeverityReview,
		WhenEmitted: "repair_slide was given a registered ADVISORY kind (add_detail_or_resize, grow_pattern, review, truncation_summary, … — see get_capabilities.vocabularies.advisory_fix_kinds). These are legitimately emitted by findings; no edit can execute them, so the response carries the decision to make in message and executable alternatives in alternatives[].",
		RemediationSteps: []string{
			"Read the response's message: it states what you have to decide (add detail, merge slides, rewrite the line, pick another pattern).",
			"Or apply one of the response's alternatives[], which are all executable kinds addressing the same defect.",
			"Do not re-send the same kind; it is not a caller mistake and the answer will not change.",
		},
		RelatedCodes: []string{"kind_not_supported", "wrong_kind_for_target"},
	},

	"wrong_kind_for_target": {
		Code:        "wrong_kind_for_target",
		Summary:     "The fix kind cannot reach the text it was aimed at; another kind can.",
		Severity:    describeSeverityReview,
		WhenEmitted: "A repair targets content the kind does not edit — typically reduce_text on a slide whose text lives in shape_grid cells, which only reduce_cell_text can change. The response names the right kind in did_you_mean and carries a ready-to-send directive in next_tool_call.",
		RemediationSteps: []string{
			"Submit the response's next_tool_call verbatim: it carries the corrected kind and the cell_path.",
			"When targeting a grid cell yourself, use reduce_cell_text with cell_path \"/slides/N/shape_grid/rows/R/cells/C\" and max_chars.",
		},
		RelatedCodes: []string{"kind_not_supported", "advisory_fix_kind", "fit_overflow"},
	},

	"semantic_review_required": {
		Code:        "semantic_review_required",
		Summary:     "A text-shortening repair was refused because it would have removed meaning.",
		Severity:    describeSeverityReview,
		WhenEmitted: "reduce_text / shorten_title detect that the truncation point would drop a number, unit, negation, or qualifier — changing what the sentence claims — so the fix is reported un-applied rather than silently altering the meaning.",
		RemediationSteps: []string{
			"Rewrite the text yourself so it fits without losing the number, unit, or negation.",
			"Or split the content across two slides / cells so the full sentence survives.",
			"Do not re-request the same automatic truncation; it will refuse again for the same reason.",
		},
		ExampleBefore: `"Margin did not improve in FY24 (-2.1pp)"  // truncating drops "-2.1pp"`,
		ExampleAfter:  `"Margin fell 2.1pp in FY24"`,
		RelatedCodes:  []string{"kind_not_supported"},
	},

	"duplicate_title": {
		Code:        "duplicate_title",
		Summary:     "Two or more slides carry the same title.",
		Severity:    describeSeverityReview,
		WhenEmitted: "score_deck's composition pass finds repeated slide titles, which makes the deck hard to navigate and suggests an unsplit topic.",
		RemediationSteps: []string{
			"Differentiate the titles with a subtopic suffix (\"Pricing — Plans\", \"Pricing — Margins\").",
			"If the slides genuinely duplicate each other, merge them.",
		},
		RelatedCodes: []string{patterns.ErrCodeDuplicateTitle},
	},

	"pattern_run": {
		Code:        "pattern_run",
		Summary:     "The same pattern repeats across several consecutive slides, flattening the deck's rhythm.",
		Severity:    describeSeverityReview,
		WhenEmitted: "score_deck finds a run of consecutive slides using one pattern. The message names the pattern, the run length, and the slide index range.",
		RemediationSteps: []string{
			"Swap one slide in the run to a different pattern; call recommend_visual for the slide's intent to rank alternatives.",
			"An emphasis slide (stat-hero, pull-quote) in the middle of a run breaks it effectively.",
		},
		RelatedCodes: []string{"density_monotony", "missing_emphasis"},
	},

	"density_monotony": {
		Code:        "density_monotony",
		Summary:     "Every slide carries a similar content density, so the deck has no visual pacing.",
		Severity:    describeSeverityReview,
		WhenEmitted: "score_deck's density coefficient of variation across slides is low — content-heavy and content-light slides are not mixed.",
		RemediationSteps: []string{
			"Alternate dense slides with light ones: follow a table or dense grid with a stat-hero, pull-quote, or section divider.",
			"Call analyze_deck_rhythm for the per-slide density figures behind this score.",
		},
		RelatedCodes: []string{"pattern_run", "missing_emphasis"},
	},

	"missing_emphasis": {
		Code:        "missing_emphasis",
		Summary:     "A long deck contains no emphasis slide to break its monotony.",
		Severity:    describeSeverityReview,
		WhenEmitted: "score_deck finds 10 or more slides with no emphasis pattern (stat-hero, pull-quote) anywhere in the sequence.",
		RemediationSteps: []string{
			"Add a stat-hero slide for the deck's single most important number, or a pull-quote for a customer/stakeholder voice.",
			"Place it at a natural pause — after a section, or before the recommendation.",
		},
		RelatedCodes: []string{"density_monotony", "pattern_run"},
	},

	"accent_dominance": {
		Code:        "accent_dominance",
		Summary:     "One accent color is used on nearly every accented slide, so accent carries no signal.",
		Severity:    describeSeverityReview,
		WhenEmitted: "score_deck finds a single accent dominating the share of accented slides. The message names the accent and its share.",
		RemediationSteps: []string{
			"Set the deck-level \"accent_strategy\" to \"rotate\" or \"section-keyed\" so accents vary across slides or sections.",
			"Or set per-slide accents deliberately, reserving one accent for emphasis.",
		},
		RelatedCodes: []string{patterns.ErrCodeAccentOverload},
	},

	// ---- Input family — request / JSON-payload problems ----

	CodeMissingParameter: {
		Code:        CodeMissingParameter,
		Summary:     "A required tool or CLI argument was not supplied.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Argument validation finds a required parameter absent or empty.",
		RemediationSteps: []string{
			"Supply the parameter named in evidence.path.",
			"Replay the call in next_tool_call with the missing field filled in; example_value shows a valid value.",
		},
		ExampleBefore: `repair_slide({"slide": {...}})  // "fixes" omitted`,
		ExampleAfter:  `repair_slide({"slide": {...}, "fixes": [{"kind": "reduce_text"}]})`,
		RelatedCodes:  []string{CodeInvalidParameter},
	},
	CodeInvalidParameter: {
		Code:        CodeInvalidParameter,
		Summary:     "A supplied argument has the wrong type or an illegal value.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Argument validation finds a parameter present but malformed (wrong JSON type or out-of-range value).",
		RemediationSteps: []string{
			"Correct the value at evidence.path to match evidence.expected_type.",
			"Consult get_input_schema for the parameter's accepted shape.",
		},
		RelatedCodes: []string{CodeMissingParameter, CodeInvalidJSON, CodeUnknownEnum},
	},
	CodeUnknownParameter: {
		Code:        CodeUnknownParameter,
		Summary:     "A tool call supplied an argument the tool does not accept.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Every MCP tool call is checked against the tool's input schema before the handler runs; an argument name the schema does not declare is rejected instead of being silently ignored (e.g. plan_deck slide_count, whose real name is slide_budget).",
		RemediationSteps: []string{
			"Rename the argument at evidence.path to fix.params.did_you_mean (also named in the message), or remove it.",
			"The message lists every accepted argument; tools/list carries the full input schema.",
		},
		ExampleBefore: `plan_deck({"brief": "...", "slide_count": 8})`,
		ExampleAfter:  `plan_deck({"brief": "...", "slide_budget": 8})`,
		RelatedCodes:  []string{CodeInvalidParameter, CodeMissingParameter},
	},
	CodeInvalidGrid: {
		Code:        CodeInvalidGrid,
		Summary:     "A shape_grid is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Dry-run / preflight finds a shape_grid whose rows, columns, or cell spans do not form a valid grid.",
		RemediationSteps: []string{
			"Fix the grid's rows/cols and cell row_span/col_span so cells tile the grid without gaps or overlaps.",
			"Start from a known-good skeleton via expand_pattern, then edit values.",
		},
		RelatedCodes: []string{CodePatternError, patterns.ErrCodeInvalidShape},
	},
	CodeInvalidJSON: {
		Code:        CodeInvalidJSON,
		Summary:     "The JSON payload could not be parsed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The deck JSON or request body is not syntactically valid JSON.",
		RemediationSteps: []string{
			"Fix the syntax error at the reported offset (unbalanced braces, trailing commas, unquoted keys).",
			"Validate the document with a JSON linter before resubmitting.",
		},
		RelatedCodes: []string{CodeInvalidKey, CodeInvalidParameter},
	},
	CodeInvalidKey: {
		Code:        CodeInvalidKey,
		Summary:     "An object contains a key the schema does not allow.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Strict key checking finds an unknown or misspelled field on a slide, content item, or value object.",
		RemediationSteps: []string{
			"Remove or rename the offending key at evidence.path.",
			"Check get_input_schema for the allowed field names at that level.",
		},
		RelatedCodes: []string{CodeInvalidParameter, patterns.ErrCodeUnknownKey},
	},
	CodeInvalidSlide: {
		Code:        CodeInvalidSlide,
		Summary:     "A slide object is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A slide cannot be interpreted — e.g. it has no resolvable layout_id/slide_type or malformed content.",
		RemediationSteps: []string{
			"Give the slide a valid layout_id or slide_type and well-formed content[].",
			"Compare the slide against get_input_schema.",
		},
		RelatedCodes: []string{CodeInvalidSlideIndex, CodeValidationFailed},
	},
	CodeInvalidSlideIndex: {
		Code:        CodeInvalidSlideIndex,
		Summary:     "A slide index argument is out of range.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A tool references a slide index that does not exist in the deck.",
		RemediationSteps: []string{
			"Use a 0-based index inside [0, slide_count).",
			"Read the deck length first if you are unsure how many slides exist.",
		},
		RelatedCodes: []string{CodeInvalidPath, CodeInvalidSlide},
	},
	CodeInvalidPath: {
		Code:        CodeInvalidPath,
		Summary:     "A JSON path or file path argument is malformed or unresolvable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A path argument cannot be parsed or does not resolve to a node/file.",
		RemediationSteps: []string{
			"Supply a valid path; for files prefer an absolute path.",
			"For JSON pointers, confirm each segment exists in the document.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeInvalidSlideIndex},
	},
	CodeAmbiguousInput: {
		Code:        CodeAmbiguousInput,
		Summary:     "The request could be interpreted more than one way.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Two mutually exclusive inputs were supplied (e.g. both inline JSON and a file path for the same argument).",
		RemediationSteps: []string{
			"Supply exactly one of the conflicting inputs.",
			"Check the message for which inputs collided.",
		},
		RelatedCodes: []string{CodeInvalidParameter},
	},
	CodeUnsupported: {
		Code:        CodeUnsupported,
		Summary:     "The requested operation or option is not supported.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A tool receives an operation or feature it does not implement (e.g. an unknown deck-patch op kind).",
		RemediationSteps: []string{
			"Use one of the supported values listed in fix.params.allowed.",
			"Check get_capabilities for the supported operation set.",
		},
		RelatedCodes: []string{CodeInvalidParameter, CodeUnknownEnum},
	},
	CodeIdempotencyConflict: {
		Code:        CodeIdempotencyConflict,
		Summary:     "An idempotency_key was reused for a request whose content changed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "generate_presentation, auto_repair, or make_deck receives an idempotency_key that already cached a successful response, but the new request's normalized fingerprint differs from the original. Replaying would return a deck for the wrong content, so the server refuses.",
		RemediationSteps: []string{
			"Use a fresh idempotency_key when the deck/outline or any other argument changes — the key is a retry token, not a request id.",
			"To replay the original result, restore the exact input that produced it (evidence.original_fingerprint identifies it).",
		},
		ExampleBefore: `generate_presentation({"presentation": {/* edited */}, "idempotency_key": "turn-7"})  // key already used for different content`,
		ExampleAfter:  `generate_presentation({"presentation": {/* edited */}, "idempotency_key": "turn-8"})  // new key for new content`,
		RelatedCodes:  []string{CodeInvalidParameter},
	},

	// ---- Template family — template lookup / parsing failures ----

	CodeTemplateNotFound: {
		Code:        CodeTemplateNotFound,
		Summary:     "The named template could not be found in the templates directory.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Template resolution cannot find a .pptx matching the requested template name.",
		RemediationSteps: []string{
			"Call list_templates and pick a listed name.",
			"Confirm --templates-dir points at the directory holding the .pptx files.",
		},
		RelatedCodes: []string{CodeTemplatesDir, CodeTemplateError},
	},
	CodeTemplateError: {
		Code:        CodeTemplateError,
		Summary:     "The template could not be parsed or is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Template parsing fails — corrupt archive, missing theme, or unreadable slide layouts.",
		RemediationSteps: []string{
			"Run validate-template to see the structural problem.",
			"Re-export the template, or fall back to a bundled template.",
		},
		RelatedCodes: []string{CodeTemplateNotFound, CodeTemplateMetadataParse},
	},
	CodeTemplatesDir: {
		Code:        CodeTemplatesDir,
		Summary:     "The templates directory is missing or unreadable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The configured templates directory does not exist or cannot be listed.",
		RemediationSteps: []string{
			"Pass a valid --templates-dir (MCP: templates_dir).",
			"Confirm the directory exists and contains .pptx files.",
		},
		RelatedCodes: []string{CodeTemplateNotFound},
	},

	// ---- Resource family — file and asset lookup failures ----

	CodeFileNotFound: {
		Code:        CodeFileNotFound,
		Summary:     "A referenced file does not exist.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A file path argument (deck JSON or an asset) cannot be opened.",
		RemediationSteps: []string{
			"Check the path; prefer an absolute path.",
			"Confirm the file exists and is readable by the process.",
		},
		RelatedCodes: []string{CodeInvalidPath, CodeReadFailed},
	},
	CodeStyleNotFound: {
		Code:        CodeStyleNotFound,
		Summary:     "A referenced named style is not defined.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A table/cell style_id or named style cannot be resolved against the template.",
		RemediationSteps: []string{
			"List the template's styles via list_templates and pick one.",
			"Or drop the style reference to fall back to the template default.",
		},
		RelatedCodes: []string{CodeUnknownTableStyleID},
	},
	CodeUnknownTableStyleID: {
		Code:        CodeUnknownTableStyleID,
		Summary:     "A table references a style_id the template does not define.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Table generation cannot find the requested style_id in the template's table_styles registry.",
		RemediationSteps: []string{
			"Pick a style_id from list_templates → table_styles.",
			"Or omit style_id to use the template default.",
		},
		RelatedCodes: []string{CodeStyleNotFound, patterns.ErrCodeUnknownTableStyleID},
	},
	CodeUnknownThemeColor: {
		Code:        CodeUnknownThemeColor,
		Summary:     "A semantic color name is not part of the template theme.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Color resolution receives a scheme name outside the accent1..accent6 / lt1 / dk1 / lt2 / dk2 set.",
		RemediationSteps: []string{
			"Use a valid scheme name; run resolve-theme to inspect the template's palette.",
			"Avoid raw hex unless the template's brand allowlist permits it.",
		},
		RelatedCodes: []string{CodeUnknownEnum, patterns.ErrCodeHexFillNonBrand},
	},
	CodeUnknownPattern: {
		Code:        CodeUnknownPattern,
		Summary:     "A pattern name is not registered.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Pattern expansion receives a name not present in the pattern registry.",
		RemediationSteps: []string{
			"Call list_patterns and pick a registered name.",
			"Use recommend_pattern to find a pattern that fits the content.",
		},
		RelatedCodes: []string{CodePatternError},
	},
	CodeIconPath: {
		Code:        CodeIconPath,
		Summary:     "An icon path argument is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path cannot be used (a general path failure not covered by a more specific ICON_PATH_* code).",
		RemediationSteps: []string{
			"Supply a valid icon path, or use a bundled icon name instead.",
			"List bundled icons via the icons command.",
		},
		RelatedCodes: []string{CodeIconNotFound, CodeIconPathExtInvalid},
	},
	CodeIconNotFound: {
		Code:        CodeIconNotFound,
		Summary:     "The requested icon could not be found.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name or path does not resolve to a bundled icon or a readable file.",
		RemediationSteps: []string{
			"List icons and use a known name; check evidence.suggestions for near matches.",
			"For a file icon, confirm the path exists.",
		},
		RelatedCodes: []string{CodeIconAmbiguous, CodeIconBundledNameUnknown},
	},
	CodeIconPathExtInvalid: {
		Code:        CodeIconPathExtInvalid,
		Summary:     "An icon path has an unsupported file extension.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path's extension is not one of the allowed image/SVG types.",
		RemediationSteps: []string{
			"Use a .svg or .png asset.",
			"Convert the source asset to a supported format.",
		},
		RelatedCodes: []string{CodeIconPath},
	},
	CodeIconPathTraversal: {
		Code:        CodeIconPathTraversal,
		Summary:     "An icon path attempts directory traversal outside the allowed root.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path contains \"..\" segments that escape the asset root.",
		RemediationSteps: []string{
			"Place the asset under the allowed asset root.",
			"Reference it with a relative path that does not contain \"..\".",
		},
		RelatedCodes: []string{CodeIconPathSymlinkEscape, CodeIconPath},
	},
	CodeIconPathSymlinkEscape: {
		Code:        CodeIconPathSymlinkEscape,
		Summary:     "An icon path resolves through a symlink that escapes the allowed root.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Symlink resolution of an icon path lands outside the asset root.",
		RemediationSteps: []string{
			"Store the asset directly under the asset root rather than behind a symlink.",
			"Reference the real file location.",
		},
		RelatedCodes: []string{CodeIconPathTraversal, CodeIconPath},
	},
	CodeIconAmbiguous: {
		Code:        CodeIconAmbiguous,
		Summary:     "An icon name matches more than one bundled icon.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name resolves to multiple bundled icons across icon sets.",
		RemediationSteps: []string{
			"Use the fully-qualified icon name from fix.params / evidence.suggestions.",
			"Disambiguate by prefixing the icon set.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconMissing: {
		Code:        CodeIconMissing,
		Summary:     "An icon reference is empty.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon entry supplies none of name, path, or svg_data.",
		RemediationSteps: []string{
			"Supply exactly one of name, path, or svg_data on the icon entry.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconList: {
		Code:        CodeIconList,
		Summary:     "Listing the available icons failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The bundled icon catalog cannot be enumerated.",
		RemediationSteps: []string{
			"Retry the request.",
			"If it persists, report the failure with the input_sha256.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconBundledNameUnknown: {
		Code:        CodeIconBundledNameUnknown,
		Summary:     "A bundled icon name is not in the catalog.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name does not match any bundled icon.",
		RemediationSteps: []string{
			"List icons and pick a known name.",
			"Check evidence.suggestions for the closest valid names.",
		},
		RelatedCodes: []string{CodeIconNotFound, CodeIconAmbiguous},
	},
	CodeIconFillIgnoredInline: {
		Code:        CodeIconFillIgnoredInline,
		Summary:     "An icon `fill` was ignored because inline `svg_data` is set.",
		Severity:    describeSeverityReview,
		WhenEmitted: "An icon supplies both inline svg_data and a fill color; the engine cannot recolor pre-rendered SVG markup, so fill is dropped.",
		RemediationSteps: []string{
			"Pre-color the inline svg_data markup directly.",
			"Or remove svg_data and use name/path with fill instead.",
		},
		ExampleBefore: `{"icon": {"svg_data": "<svg>...</svg>", "fill": "accent1"}}`,
		ExampleAfter:  `{"icon": {"name": "rocket", "fill": "accent1"}}`,
	},
	CodeImagePath: {
		Code:        CodeImagePath,
		Summary:     "An image path is invalid or unresolvable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An image_value path cannot be opened or is an unsupported format.",
		RemediationSteps: []string{
			"Check the path and use a supported image format (PNG/JPG).",
			"Confirm the file is readable by the process.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeBackgroundImagePath},
	},
	CodeBackgroundImagePath: {
		Code:        CodeBackgroundImagePath,
		Summary:     "A slide background image path is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A slide's background image path cannot be opened or is an unsupported format.",
		RemediationSteps: []string{
			"Check the background image path and format.",
			"Confirm the file is readable.",
		},
		RelatedCodes: []string{CodeImagePath},
	},
	CodeAssetPathEnvUnset: {
		Code:        CodeAssetPathEnvUnset,
		Summary:     "An asset path requires an environment variable that is not set.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An asset is referenced via an env-rooted path but the named environment variable is unset.",
		RemediationSteps: []string{
			"Set the environment variable named in the message.",
			"Or reference the asset with a direct path instead of an env-rooted one.",
		},
		RelatedCodes: []string{CodeImagePath, CodeIconPath},
	},
	CodeAssetTooLarge: {
		Code:        CodeAssetTooLarge,
		Summary:     "An asset exceeds the maximum allowed size.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon or image asset's byte size exceeds the configured cap (see get_capabilities for the limit).",
		RemediationSteps: []string{
			"Recompress or downscale the asset to fit under the limit.",
			"Use a vector (SVG) icon instead of a large raster image.",
		},
		RelatedCodes: []string{CodeImagePath},
	},
	CodeURLFetchFailed: {
		Code:        CodeURLFetchFailed,
		Summary:     "A remote asset URL could not be fetched.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An http(s) asset fetch fails — network error, non-200 status, or timeout.",
		RemediationSteps: []string{
			"Check the URL and network reachability.",
			"Download the asset and reference it by local path instead.",
		},
		RelatedCodes: []string{CodeURLResolverInit},
	},
	CodeURLResolverInit: {
		Code:        CodeURLResolverInit,
		Summary:     "The URL asset resolver failed to initialize.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The remote-asset resolver cannot start (e.g. cache directory or configuration problem).",
		RemediationSteps: []string{
			"Check the resolver's cache directory and permissions.",
			"Retry; reference assets locally if remote fetch is unavailable.",
		},
		RelatedCodes: []string{CodeURLFetchFailed},
	},
	CodeSVGInvalidRoot: {
		Code:        CodeSVGInvalidRoot,
		Summary:     "Inline SVG data has an invalid root element.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "svg_data does not begin with a valid <svg> root element.",
		RemediationSteps: []string{
			"Ensure the markup is a well-formed SVG document with an <svg> root.",
		},
		RelatedCodes: []string{CodeSVGParseError, CodeSVGUnsafeXML},
	},
	CodeSVGUnsafeXML: {
		Code:        CodeSVGUnsafeXML,
		Summary:     "Inline SVG contains unsafe XML (DTD, entities, or external references).",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The SVG safety scan finds disallowed XML constructs that could trigger entity expansion or external fetches.",
		RemediationSteps: []string{
			"Remove DOCTYPE/entity declarations and external references from the SVG.",
			"Sanitize the SVG with a trusted tool before embedding.",
		},
		RelatedCodes: []string{CodeSVGParseError, CodeSVGInvalidRoot},
	},
	CodeSVGParseError: {
		Code:        CodeSVGParseError,
		Summary:     "Inline SVG data could not be parsed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "SVG XML parsing fails on the supplied svg_data.",
		RemediationSteps: []string{
			"Fix the SVG markup; validate it with an SVG tool.",
		},
		RelatedCodes: []string{CodeSVGInvalidRoot, CodeSVGUnsafeXML},
	},

	// ---- Render family — generation and rendering failures ----

	CodeGenerationFailed: {
		Code:        CodeGenerationFailed,
		Summary:     "Deck generation failed before output was written.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The generation pipeline returns an error prior to producing the .pptx.",
		RemediationSteps: []string{
			"Read the message for the failing stage.",
			"Run validate / preflight on the deck to isolate the cause, then retry.",
		},
		RelatedCodes: []string{CodeRenderFailed, CodeValidationFailed},
	},
	CodeReadFailed: {
		Code:        CodeReadFailed,
		Summary:     "The input deck or a referenced file could not be read.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Reading the deck JSON or a referenced file fails (I/O error or permissions).",
		RemediationSteps: []string{
			"Check the path and file permissions.",
			"Confirm the deck JSON is present and well-formed.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeInvalidJSON},
	},
	CodeRenderFailed: {
		Code:        CodeRenderFailed,
		Summary:     "Rendering the deck to PPTX or images failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A render step (OOXML assembly or image conversion) returns an error.",
		RemediationSteps: []string{
			"Read the message for the failing render stage.",
			"For image output, confirm LibreOffice and ImageMagick are available.",
			"If the message says LibreOffice produced no PDF, the deck is not the problem: close any open LibreOffice window and retry the call.",
		},
		RelatedCodes: []string{CodeGenerationFailed, CodeLibreOfficeUnavailable},
	},
	CodeLibreOfficeUnavailable: {
		Code:        CodeLibreOfficeUnavailable,
		Summary:     "LibreOffice is required for this operation but is not available.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PPTX→image step needs LibreOffice and it is not installed or not on PATH.",
		RemediationSteps: []string{
			"Install LibreOffice and ensure the soffice binary is on PATH.",
			"Skip image conversion if only the .pptx is needed.",
		},
		RelatedCodes: []string{CodeImageMagickUnavailable, CodeRenderFailed},
	},
	CodeImageMagickUnavailable: {
		Code:        CodeImageMagickUnavailable,
		Summary:     "ImageMagick is required for this operation but is not available.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An image conversion step needs ImageMagick and it is not installed or not on PATH.",
		RemediationSteps: []string{
			"Install ImageMagick and ensure convert/magick is on PATH.",
			"Skip image conversion if only the .pptx is needed.",
		},
		RelatedCodes: []string{CodeLibreOfficeUnavailable, CodeRenderFailed},
	},
	CodeLibreOfficeTimeout: {
		Code:        CodeLibreOfficeTimeout,
		Summary:     "LibreOffice exceeded its render deadline and was killed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PPTX→PDF conversion subprocess ran past its bounded deadline; its process group was terminated so it could not hold the render lock indefinitely.",
		RemediationSteps: []string{
			"Retry the render (pass force=true) — a single wedged conversion is often transient.",
			"If it recurs, the LibreOffice environment is likely stuck: restart it, or skip image rendering and ship the .pptx without thumbnails.",
		},
		RelatedCodes: []string{CodeImageMagickTimeout, CodeLibreOfficeUnavailable, CodeRenderFailed},
	},
	CodeImageMagickTimeout: {
		Code:        CodeImageMagickTimeout,
		Summary:     "ImageMagick exceeded its render deadline and was killed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PDF→PNG conversion subprocess ran past its bounded deadline and its process group was terminated.",
		RemediationSteps: []string{
			"Retry the render (pass force=true).",
			"If it recurs, lower the render density or skip image rendering and ship the .pptx without thumbnails.",
		},
		RelatedCodes: []string{CodeLibreOfficeTimeout, CodeImageMagickUnavailable, CodeRenderFailed},
	},
	CodeOutputDir: {
		Code:        CodeOutputDir,
		Summary:     "The output directory is missing or not writable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The engine cannot create or write to the requested output directory.",
		RemediationSteps: []string{
			"Pass a writable --output directory.",
			"Check directory permissions and available disk space.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},
	CodePatternError: {
		Code:        CodePatternError,
		Summary:     "A pattern failed to expand.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Pattern expansion returns an error from bad values or an internal failure.",
		RemediationSteps: []string{
			"Validate the pattern values with validate_pattern.",
			"Inspect the contract via show_pattern and correct the values.",
		},
		RelatedCodes: []string{CodeUnknownPattern, CodeInvalidGrid},
	},
	CodeStrictFit: {
		Code:        CodeStrictFit,
		Summary:     "Strict-fit mode refused the deck because content overflows.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "generate runs with strict_fit enabled and a fit finding carries a refuse-level action.",
		RemediationSteps: []string{
			"Fix the underlying fit finding (split or shorten the offending content).",
			"Or lower strict-fit to warn to allow the engine's shrink/truncate fallback.",
		},
		RelatedCodes: []string{patterns.ErrCodeFitOverflow, patterns.ErrCodePlaceholderOverflow, patterns.ErrCodeDensityExceeded},
	},
	CodeValidationFailed: {
		Code:        CodeValidationFailed,
		Summary:     "Input validation failed with at least one error-severity finding.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "validate_input (or the validate gate before generation) reports an error-severity problem.",
		RemediationSteps: []string{
			"Read each finding in the envelope and fix it.",
			"Re-run validate until ok is true, then generate.",
		},
		RelatedCodes: []string{CodeGenerationFailed, CodeInvalidSlide},
	},
	CodeOutputValidationError: {
		Code:        CodeOutputValidationError,
		Summary:     "The generated PPTX failed structural output validation.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Post-generation validate-output finds the produced .pptx structurally invalid.",
		RemediationSteps: []string{
			"Report the failure with the offending deck for diagnosis.",
			"Re-run with a simpler deck to isolate the slide that triggers it.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},
	CodeOverlayFailed: {
		Code:        CodeOverlayFailed,
		Summary:     "Rendering an annotated overlay failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A preview/examine overlay (the annotated SVG/PNG) could not be produced.",
		RemediationSteps: []string{
			"Retry; the underlying report is still produced without the overlay.",
			"Check that the overlay inputs (layout geometry) resolved.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},

	// ---- Settings family — template settings operations ----

	CodeSettingsError: {
		Code:        CodeSettingsError,
		Summary:     "A template-settings operation failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Reading or writing template settings returns an error.",
		RemediationSteps: []string{
			"Check the settings store path and permissions.",
			"Retry the operation.",
		},
		RelatedCodes: []string{CodeSettingsWriteDisabled},
	},
	CodeSettingsWriteDisabled: {
		Code:        CodeSettingsWriteDisabled,
		Summary:     "Writing template settings is disabled.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A write to template settings is attempted while settings writes are disabled by configuration.",
		RemediationSteps: []string{
			"Enable settings writes in the server configuration.",
			"Run in an environment that permits settings writes.",
		},
		RelatedCodes: []string{CodeSettingsError},
	},

	// ---- Inspect family — visual QA failures ----

	CodeInvalidImage: {
		Code:        CodeInvalidImage,
		Summary:     "An image supplied for inspection is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "inspect_slide_images receives an unreadable or malformed image.",
		RemediationSteps: []string{
			"Supply a valid PNG/JPG image.",
			"Re-render the slide to images before inspecting.",
		},
		RelatedCodes: []string{CodeInspectDisabled},
	},
	CodeInspectDisabled: {
		Code:        CodeInspectDisabled,
		Summary:     "Visual inspection is disabled.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "inspect is invoked while the visual-QA backend is disabled or unconfigured.",
		RemediationSteps: []string{
			"Enable and configure the inspect backend.",
			"Skip inspection if the visual-QA agent is unavailable.",
		},
		RelatedCodes: []string{CodeInvalidImage},
	},
	CodeVisionTimeout: {
		Code:        CodeVisionTimeout,
		Summary:     "A vision inspection API call exceeded its deadline.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A Claude vision request during inspect_slide_images (or the visual_qa loop) ran past its per-request or total inspection deadline.",
		RemediationSteps: []string{
			"Retry the inspection — a single stalled API call is usually transient.",
			"If it recurs, reduce parallelism or fall back to the heuristic inspector (unset ANTHROPIC_API_KEY) to degrade gracefully.",
		},
		RelatedCodes: []string{CodeInspectDisabled, CodeInvalidImage},
	},
	CodeVisionInspectionFailed: {
		Code:        CodeVisionInspectionFailed,
		Summary:     "A slide's vision inspection failed, so it produced no findings.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A Claude vision call during inspect_slide_images (or the visual_qa loop) returned an API error, a transport/decode failure, or malformed output for that slide. The slide was NOT inspected — zero findings here means the inspection failed, not that the slide is clean.",
		RemediationSteps: []string{
			"Treat the inspection as inconclusive for the affected slide(s) named in inspection_status / failed_slide_count — do not ship on the basis of zero findings.",
			"Retry the inspection; if it recurs, fall back to the heuristic inspector (unset ANTHROPIC_API_KEY) to degrade gracefully.",
		},
		RelatedCodes: []string{CodeVisionTimeout, CodeHeuristicInspectionFailed, CodeInspectDisabled},
	},
	CodeHeuristicInspectionFailed: {
		Code:        CodeHeuristicInspectionFailed,
		Summary:     "A slide's heuristic inspection could not run (e.g. the image failed to decode).",
		Severity:    describeSeverityReview,
		WhenEmitted: "The pure-Go heuristic inspector (used when ANTHROPIC_API_KEY is unset) could not decode a slide image, so it produced no findings for that slide. The result is degraded — absence of findings does not mean the slide is clean.",
		RemediationSteps: []string{
			"Re-render the slide to a valid PNG/JPG and re-inspect.",
			"For a full vision-backed inspection, set ANTHROPIC_API_KEY.",
		},
		RelatedCodes: []string{CodeVisionInspectionFailed, CodeInvalidImage},
	},

	// ---- Semantic family — compact semantic deck-spec validation gates ----

	CodeSemanticRequired: {
		Code:        CodeSemanticRequired,
		Summary:     "A required field is missing from the semantic deck spec.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "semantic validation finds a required field absent — e.g. meta.title, a slide's kind-specific required payload, or a chart_insight series.",
		RemediationSteps: []string{
			"Add the field named in evidence.path to the semantic spec.",
			"Consult the semantic schema (json2pptx semantic schema) for the required fields of each slide kind.",
		},
		ExampleBefore: `{"meta": {}, "slides": [...]}`,
		ExampleAfter:  `{"meta": {"title": "Q2 Board Update"}, "slides": [...]}`,
		RelatedCodes:  []string{CodeSemanticUnknownKind, CodeSemanticTakeawayRequired},
	},
	CodeSemanticUnknownKind: {
		Code:        CodeSemanticUnknownKind,
		Summary:     "A slide declares a kind that is not in the semantic vocabulary.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "semantic validation finds a slide whose kind is missing or not one of the registered slide kinds.",
		RemediationSteps: []string{
			"Set the slide's kind at evidence.path to a registered kind.",
			"List the registered kinds via json2pptx semantic schema.",
		},
		ExampleBefore: `{"kind": "bogus_kind", "title": "Oops"}`,
		ExampleAfter:  `{"kind": "title", "title": "Hello"}`,
		RelatedCodes:  []string{CodeSemanticRequired, CodeSemanticUnknownArchetype},
	},
	CodeSemanticUnknownField: {
		Code:        CodeSemanticUnknownField,
		Summary:     "A field in the semantic deck spec is not recognized (top-level, meta, slide payload, list entry, or chart object).",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "semantic parsing finds a top-level key other than meta or slides (error — most often a stale spec using deck instead of meta), or validation finds a slide payload key, list-entry key (e.g. slides[i].kpis[j].valeu), or chart key that the kind's compiler never reads (warning; error under strict) — its content would otherwise be dropped silently. The path names the dropped key; fix.params.did_you_mean names the likely intended key.",
		RemediationSteps: []string{
			"Rename the unknown field to the suggested key (fix.params.did_you_mean, e.g. deck -> meta, takeawy -> takeaway), or remove it.",
			"A semantic deck spec has exactly two top-level fields: meta and slides; each slide kind accepts exactly the fields in list_slide_kinds item_schema (see json2pptx semantic schema).",
		},
		ExampleBefore: `{"deck": {"title": "Q2 Review"}, "slides": [...]}`,
		ExampleAfter:  `{"meta": {"title": "Q2 Review"}, "slides": [...]}`,
		RelatedCodes:  []string{CodeSemanticRequired, CodeSemanticUnknownKind},
	},
	CodeSemanticUnknownArchetype: {
		Code:        CodeSemanticUnknownArchetype,
		Summary:     "meta.archetype is not one of the registered deck archetypes.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "semantic validation finds meta.archetype set to a value outside the registered archetype vocabulary.",
		RemediationSteps: []string{
			"Set meta.archetype to a registered archetype, or remove it.",
			"List the registered archetypes via json2pptx semantic schema.",
		},
		RelatedCodes: []string{CodeSemanticUnknownKind},
	},
	CodeSemanticTakeawayRequired: {
		Code:        CodeSemanticTakeawayRequired,
		Summary:     "A content slide carries no one-line takeaway.",
		Severity:    describeSeverityReview,
		WhenEmitted: "semantic validation finds a content-bearing slide (executive_summary, kpi_snapshot, chart_insight, comparison, process, roadmap, decision) with no takeaway (or insight) line. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Add a takeaway line at evidence.path stating the slide's single message.",
			"For a chart_insight slide an insight line satisfies the requirement.",
		},
		RelatedCodes: []string{CodeSemanticWeakContent, CodeSemanticDensity},
	},
	CodeSemanticDensity: {
		Code:        CodeSemanticDensity,
		Summary:     "A slide's item count falls outside its recommended density range.",
		Severity:    describeSeverityReview,
		WhenEmitted: "semantic validation finds a count outside the advisory range for the slide kind — e.g. kpi_snapshot kpis not in 2–6, executive_summary points not in 3–5, process steps not in 3–8, roadmap phases not in 3–6, or a comparison with other than two (or unbalanced) columns. A count outside the visual pattern's range degrades the slide to a bullet list instead of the planned visual. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Adjust the item count at evidence.path into the recommended range.",
			"Split overflowing content across multiple slides, or merge sparse slides.",
		},
		RelatedCodes: []string{CodeSemanticTakeawayRequired, CodeSemanticRequired},
	},
	CodeSemanticWeakContent: {
		Code:        CodeSemanticWeakContent,
		Summary:     "A field still contains placeholder or filler text.",
		Severity:    describeSeverityReview,
		WhenEmitted: "semantic validation finds placeholder markers (TBD, lorem ipsum, __FILL__, TODO, FIXME, placeholder) in a text field. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Replace the placeholder text at evidence.path with real content.",
		},
		ExampleBefore: `{"kind": "section", "title": "TBD"}`,
		ExampleAfter:  `{"kind": "section", "title": "Financial Review"}`,
		RelatedCodes:  []string{CodeSemanticRequired},
	},
	CodeSemanticFieldType: {
		Code:        CodeSemanticFieldType,
		Summary:     "A payload field is present but has the wrong JSON type for its slide kind.",
		Severity:    describeSeverityReview,
		WhenEmitted: "semantic validation finds a field whose value is not the type its kind's compiler reads — e.g. a numeric or boolean title, or points/steps/columns supplied as a scalar instead of an array. The compiler silently drops wrong-typed values, so the content would otherwise vanish without a finding. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Give the field at evidence.path the expected type: a string for title/subtitle/takeaway/insight/source/recommendation, an array for points/steps/phases/columns/options/insights/kpis.",
			"For list fields, wrap a single value in an array (e.g. \"points\": [\"one point\"] rather than \"points\": \"one point\").",
		},
		ExampleBefore: `{"kind": "executive_summary", "title": "Q3", "points": "single point"}`,
		ExampleAfter:  `{"kind": "executive_summary", "title": "Q3", "points": ["single point"]}`,
		RelatedCodes:  []string{CodeSemanticRequired, CodeSemanticDensity},
	},
	CodeChartSeriesLengthMismatch: {
		Code:        CodeChartSeriesLengthMismatch,
		Summary:     "A chart series has a different number of values than the chart has categories.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "chart validation finds a series whose values array is shorter or longer than categories. The renderer plots the values it has and leaves the rest of the categories empty, so a truncated array — the most likely slip after editing a chart by hand — used to ship as a confidently wrong chart that every gate called clean.",
		RemediationSteps: []string{
			"Give every series exactly one value per category.",
			"Pad a short series with 0 where the figure is genuinely missing, or drop the categories you have no data for.",
		},
		ExampleBefore: `{"categories": ["Q1","Q2","Q3","Q4"], "series": [{"name": "Revenue", "values": [10]}]}`,
		ExampleAfter:  `{"categories": ["Q1","Q2","Q3","Q4"], "series": [{"name": "Revenue", "values": [10, 12, 15, 18]}]}`,
		RelatedCodes:  []string{CodeChartValueNotNumeric, CodeSemanticFieldType},
	},
	CodeChartValueNotNumeric: {
		Code:        CodeChartValueNotNumeric,
		Summary:     "A chart value is not a number.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "chart validation finds a quoted figure (\"10\"), a null, or an object where a number belongs. The chart then renders as an empty plot with a format-error label.",
		RemediationSteps: []string{
			"Write the values unquoted: [10, 20, 30], not [\"10\", \"20\", \"30\"].",
			"Replace a null with 0, or drop its category.",
		},
		ExampleBefore: `{"categories": ["A","B"], "series": [{"name": "Revenue", "values": ["1", "two"]}]}`,
		ExampleAfter:  `{"categories": ["A","B"], "series": [{"name": "Revenue", "values": [1, 2]}]}`,
		RelatedCodes:  []string{CodeChartSeriesLengthMismatch},
	},
	CodeSemanticPatternNotAvailable: {
		Code:        CodeSemanticPatternNotAvailable,
		Summary:     "A slide's pattern / layout override names a composition its kind cannot compile to, so the override is ignored.",
		Severity:    describeSeverityReview,
		WhenEmitted: "semantic validation finds a \"pattern\" or \"layout\" on a slide whose value is not one of that kind's compositions. The compiler keeps its own choice and the override does nothing; before this code it did nothing SILENTLY, so an agent varying a monotonous run (analyze_deck_rhythm's \"break this run\") could not tell its attempt had been dropped. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Use one of the values in fix.params.allowed, which is the list for this kind.",
			"Call list_slide_kinds and read the kind's compositions[] to see every pattern / layout it accepts and why each exists.",
			"Or remove the override and let the compiler choose — that is what is happening either way.",
		},
		ExampleBefore: `{"kind": "comparison", "pattern": "table-highlight", "columns": [...]}`,
		ExampleAfter:  `{"kind": "comparison", "pattern": "card-grid", "columns": [...]}`,
		RelatedCodes:  []string{CodeSemanticRhythmMonotony},
	},
	CodeSemanticRhythmMonotony: {
		Code:        CodeSemanticRhythmMonotony,
		Summary:     "Three or more consecutive slides share the same visual family.",
		Severity:    describeSeverityReview,
		WhenEmitted: "deck-rhythm analysis finds a run of 3+ adjacent slides that compile to the same visual family (e.g. three KPI slides back-to-back), which reads as monotonous. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Vary the slide kinds across the run, or move a section divider into the middle of it.",
			"Reorder slides so similar treatments are not adjacent.",
		},
		RelatedCodes: []string{CodeSemanticRhythmDensity, CodeSemanticRhythmSectioning},
	},
	CodeSemanticRhythmDensity: {
		Code:        CodeSemanticRhythmDensity,
		Summary:     "Three or more consecutive slides are content-dense.",
		Severity:    describeSeverityReview,
		WhenEmitted: "deck-rhythm analysis finds a run of 3+ adjacent heavy-density slides, which fatigues the audience. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Break the run with a lighter slide (a section divider, a single statistic, or a pull quote).",
			"Split a dense slide's content across two slides so each carries less.",
		},
		RelatedCodes: []string{CodeSemanticRhythmMonotony, CodeSemanticDensity},
	},
	CodeSemanticRhythmSectioning: {
		Code:        CodeSemanticRhythmSectioning,
		Summary:     "A long deck has no section dividers to break it into chapters.",
		Severity:    describeSeverityReview,
		WhenEmitted: "deck-rhythm analysis finds a deck of more than 8 slides with no section/transition slide. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Add one or more `section` slides to group the deck into chapters.",
		},
		RelatedCodes: []string{CodeSemanticRhythmMonotony},
	},
	CodeSemanticRhythmSynthesis: {
		Code:        CodeSemanticRhythmSynthesis,
		Summary:     "An executive deck carries no synthesis or decision slide.",
		Severity:    describeSeverityReview,
		WhenEmitted: "deck-rhythm analysis finds an executive-archetype deck (e.g. board_update, qbr, strategy_proposal) with no executive_summary or decision slide to land the message. Promoted to an error under strict validation.",
		RemediationSteps: []string{
			"Add an `executive_summary` slide near the front or a `decision` slide near the end.",
		},
		RelatedCodes: []string{CodeSemanticTakeawayRequired},
	},

	// ---- Internal family — unexpected server-side failures ----

	CodeInternal: {
		Code:        CodeInternal,
		Summary:     "An unexpected internal error occurred.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An unhandled server-side failure surfaces; the operation could not complete.",
		RemediationSteps: []string{
			"Retry the request.",
			"If it persists, report the failure with the input_sha256 for correlation.",
		},
	},
}
