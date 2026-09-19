// mcp_repair.go implements the repair_slide MCP tool — incremental targeted
// slide edits using the Fix.Kind vocabulary from fit findings. Instead of
// regenerating an entire deck, agents send a single slide index and a list
// of fix directives. The tool patches the deck and returns the result.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// --- Response types ---

// repairSlideOutput is the top-level response for repair_slide.
type repairSlideOutput struct {
	PatchedDeck  json.RawMessage `json:"patched_deck"`
	AppliedFixes []appliedFix    `json:"applied_fixes"`
	Revision     string          `json:"revision"`
	// Findings is the FindingEnvelope of residual post-patch fit findings for
	// the repaired slide. It is always present (never omitted) so an agent can
	// branch on findings.ok deterministically; findings.findings[] is empty
	// when the patch left no residual issues. This replaces the legacy
	// new_findings []FitFinding array — see docs/AGENT_DIAGNOSTICS.md.
	Findings diagnostics.FindingEnvelope `json:"findings"`
}

// appliedFix reports whether a single fix directive was successfully applied.
type appliedFix struct {
	Kind    string `json:"kind"`
	Applied bool   `json:"applied"`
	Message string `json:"message,omitempty"`

	// Code is a machine-readable error code populated on non-applied results
	// (e.g. "kind_not_supported"). Empty on success.
	Code string `json:"code,omitempty"`

	// SupportedKinds is the full list of fix kinds the engine accepts. Populated
	// when Code is "kind_not_supported" or "advisory_fix_kind" so agents can
	// recover without a separate get_capabilities round-trip.
	SupportedKinds []string `json:"supported_kinds,omitempty"`

	// Alternatives are executable fix kinds that address the same defect as a
	// registered advisory kind. Populated only when Code ==
	// "advisory_fix_kind" (go-slide-creator-ui4c).
	Alternatives []string `json:"alternatives,omitempty"`

	// DidYouMean names the fix kind that can reach the target when this one
	// cannot — e.g. reduce_text aimed at a shape_grid cell, whose text only
	// reduce_cell_text can edit. Populated with Code == "wrong_kind_for_target",
	// alongside a next_tool_call carrying the corrected directive
	// (go-slide-creator-9zof).
	DidYouMean string `json:"did_you_mean,omitempty"`

	// NextToolCall is a machine-readable suggestion for the agent to recover
	// from a non-applied result (e.g. call get_capabilities to discover the
	// current vocabulary). Omitted when there is no actionable next step.
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// repairFixInput is one fix directive from the caller.
type repairFixInput struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params,omitempty"`
}

// --- Tool definition ---

func mcpRepairSlideTool() mcp.Tool {
	return mcp.NewTool("repair_slide",
		mcp.WithDescription(`Apply targeted fixes to a single slide without regenerating the entire deck. Accepts the full deck JSON, a slide index (0-based), and a list of fix directives using the same Fix.Kind vocabulary that fit_report emits.

Returns the patched deck JSON, a report of which fixes were applied, and post-patch fit findings for the modified slide.

All fix kinds accept an optional "path" parameter (JSON Pointer, RFC 6901) to disambiguate which content element to target. When omitted, the fix applies to the first matching element on the slide.

Supported fix kinds (full vocabulary — also enumerated by get_capabilities.vocabularies.repair_fix_kinds):

Text / title fits:
- reduce_text: Truncate bullets/body text. Params: path (string, optional), max_items (int, for bullets), max_length (int, for text).
- shorten_title: Truncate a title to max_length characters. Params: path (string, optional), max_length (int).
- reduce_cell_text: Truncate a shape_grid cell's text to fit within a character budget. Params: cell_path (string, required, JSON Pointer e.g. "/slides/0/shape_grid/rows/1/cells/2"), max_chars (int, required). Truncates to max_chars-1 visible characters plus a single ellipsis (…). Handles markdown emphasis safely. Agents should prefer pre-generation budget awareness via expand_pattern over post-generation repair.

Layout / pagination:
- split_at_row: Split a table across pages using the split_slide envelope. Params: path (string, optional), row (int, rows per page), title_suffix (string, optional), repeat_headers (bool, optional).
- swap_layout: Change the slide's layout_id. Params: layout_id (string, required).

Color / theme:
- use_one_of: Replace a field value with a valid option. Params: path (string), value (string).
- replace_color: Replace one color with another in shape_grid fills. Params: from (string, color to find), to (string, replacement color). Also accepts original_color/replacement_color from contrast_autofixed findings.
- use_semantic_color: Replace a hex fill with a semantic scheme color. Params: path (string, JSON Pointer e.g. "/slides/0/shape_grid/rows/0/cells/0/shape/fill"), value (string, scheme name e.g. "accent1").

Pattern shape:
- split_pattern: Split a single pattern slide into two slides by row. Params: first (int, optional, number of filled cells to keep on slide 1; defaults to half), title_part_2 (string, optional, suffix appended to slide 2's title; defaults to "(continued)"), path (string, optional: on a pattern slide, split the pattern.values array at this key instead — slide 1 keeps the first "first" items, slide 2 the rest).
- swap_pattern: Replace the slide's pattern with a different one. Params: to (string, required, target pattern name), values (object, optional, new values for the target pattern), overrides (object, optional), cell_overrides (object, optional).
- reshape_grid: Change the grid shape by adjusting rows/columns. For pattern slides, updates the pattern values; for raw grids, redistributes cells. Params: rows (int, optional), columns (int or []int, optional). At least one is required.
- set_pattern_style: Change the style variant in a pattern's overrides (e.g. timeline-horizontal "dots" to "chevron"). Params: style (string, required).
- set_max_height_pct: Cap a pattern slide's height budget (slides[i].pattern.max_height_pct) so its boxes shrink to their content instead of stretching. Params: max_height_pct (number, required, 0 < pct <= 100; ~35 for a single sparse row). The mechanical remedy for underfilled / overtall-lane findings.

Pattern values (field-level edits to slide.pattern.values):
- rename_field: Rename a top-level key in pattern values (or slide-level fields). Params: from (string, required, current key name), to (string, required, new key name).
- reshape_value: Replace a pattern-values field with a restructured value (e.g. array → object). The field must already exist. Params: path (string, required, key in pattern.values), value (any, required, replacement value in the target shape).
- provide_value: Set a pattern-values field to an agent-supplied value, creating the key if missing. Params: path (string, required, key in pattern.values), value (any, required).
- replace_value: Replace an existing pattern-values field with a new value (typically to bring it within valid bounds). The field must already exist. Params: path (string, required, key in pattern.values), value (any, required).
- reduce_items: Truncate an array field in pattern values to max_items entries. Params: path (string, required, array key in pattern.values), max_items (int, required, > 0), confirm_semantic_change (bool, optional). Refused with code "semantic_review_required" (and a next_tool_call proposing split_pattern) when a dropped item carries a number, unit, negation, or qualifier.
- add_items: Append items to an array field in pattern values (creates the array if missing). Params: path (string, required, array key in pattern.values), items (array, required, items to append).
- resize_list: Adjust an array field in pattern values to exactly count entries. Truncates if too many; returns applied=false with guidance if too few (agent must supply additional items via add_items). Params: path (string, required, array key in pattern.values), count (int, required, > 0), confirm_semantic_change (bool, optional). Same fact-loss guard as reduce_items.
- remove_key: Remove a key from pattern overrides or pattern values (overrides checked first). Params: key (string, required, key to remove).
- remove_field: Remove a top-level field from pattern values or slide-level fields. Params: path (string, required, field name to remove).

Heuristic:
- autofix_visual: Apply a heuristic fix based on a visual QA finding category. Params: category (string, required, the visual QA finding category e.g. "text_overflow", "contrast"). Tries each candidate fix kind for the category in order until one succeeds. Additional params are forwarded to the underlying fix handler.

Two non-applied outcomes, distinguished by code:
- Unknown kinds return {applied: false, code: "kind_not_supported", message: "kind_not_supported", supported_kinds: [...executable vocabulary...], next_tool_call: {tool: "get_capabilities", args_template: {}}}.
- ADVISORY kinds — the ones findings legitimately emit whose remedy is an authoring decision (add_detail_or_resize, grow_pattern, review, truncation_summary, …; enumerated by get_capabilities.vocabularies.advisory_fix_kinds) — return {applied: false, code: "advisory_fix_kind", message: "<what you have to decide>", alternatives: [...executable kinds that address the same defect...], supported_kinds: [...]}. That is not a caller mistake: act on the guidance or apply one of the alternatives; do not retry the same kind.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaRepairSlide)),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition. Same schema as generate_presentation.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("0-based index of the slide to repair."),
			mcp.Required(),
		),
		mcp.WithString("expected_revision", mcp.Description("Optional revision precondition: the `revision` string from the response that produced these fixes (a repair plan, or a prior repair_slide response). A stale revision rejects the mutation.")),
		mcp.WithArray("fixes",
			mcp.Description(`Array of fix directives: [{"kind":"reduce_text","params":{"max_items":5}}, ...]. Each directive has a "kind" (string) and optional "params" (object).`),
			mcp.Required(),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleRepairSlide(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("repair_slide", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// Parse the deck.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}
	applyDefaults(&input)
	currentRevision := presentationRevision(&input)
	if expected, err := request.RequireString("expected_revision"); err == nil && expected != "" && expected != currentRevision {
		return argInvalidValue("repair_slide", "STALE_REVISION", "expected_revision", fmt.Sprintf("stale revision: got %s, current is %s", expected, currentRevision), "string", currentRevision, nil), nil
	}

	// Validate required fields.
	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Extract slide_index.
	slideIdx, err := extractSlideIndex(request, len(input.Slides))
	if err != nil {
		return argInvalidValue("repair_slide", "INVALID_PARAMETER", "slide_index", err.Error(), "integer", 0, nil), nil
	}

	// Extract fixes array.
	fixes, err := extractFixes(request)
	if err != nil {
		return argInvalidValue("repair_slide", "INVALID_PARAMETER", "fixes", err.Error(), "array", []any{map[string]any{"kind": "reduce_text"}}, nil), nil
	}
	if len(fixes) == 0 {
		return argMissing("repair_slide", "fixes", "array", []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 5}}}, nil), nil
	}

	// Apply each fix to the target slide.
	var applied []appliedFix
	for _, fix := range fixes {
		result := applyRepairFix(&input, slideIdx, fix)
		applied = append(applied, result)
	}

	// Resolve template for post-patch fit findings.
	var newFindings []patterns.FitFinding
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err == nil {
		defer templateCleanup()
		reader, err := template.OpenTemplate(templatePath)
		if err == nil {
			defer func() { _ = reader.Close() }()
			layouts, err := template.ParseLayouts(reader)
			if err == nil {
				slideWidth, slideHeight := template.ParseSlideDimensions(reader)
				theme := template.ParseTheme(reader)
				allFindings := collectFitFindings(&input, layouts, slideWidth, slideHeight, &theme)
				// Filter to only findings for the repaired slide (and any slides
				// created by split_at_row, which follow the original index).
				newFindings = filterFindingsForSlide(allFindings, slideIdx)
			}
		}
	}

	// Marshal the patched deck.
	patchedJSON, err := json.Marshal(input)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal patched deck: %v", err)), nil
	}

	output := repairSlideOutput{
		PatchedDeck:  patchedJSON,
		AppliedFixes: applied,
		Revision:     presentationRevision(&input),
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "repair_slide",
			Template:    input.Template,
			InputSHA256: diagnostics.ComputeInputSHA256([]byte(jsonStr)),
		}, diagnostics.FromFitFindings(newFindings)),
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Fix application ---

// applyRepairFix applies a single fix directive to the input, returning the result.
func applyRepairFix(input *PresentationInput, slideIdx int, fix repairFixInput) appliedFix {
	switch fix.Kind {
	case "reduce_text":
		return applyReduceText(input, slideIdx, fix.Params)
	case "shorten_title":
		return applyShortenTitle(input, slideIdx, fix.Params)
	case "split_at_row":
		return applySplitAtRow(input, slideIdx, fix.Params)
	case "swap_layout":
		return applySwapLayout(input, slideIdx, fix.Params)
	case "use_one_of":
		return applyUseOneOf(input, slideIdx, fix.Params)
	case "replace_color":
		return applyReplaceColor(input, slideIdx, fix.Params)
	case "use_semantic_color":
		return applyUseSemanticColor(input, slideIdx, fix.Params)
	case "split_pattern":
		return applySplitPattern(input, slideIdx, fix.Params)
	case "swap_pattern":
		return applySwapPattern(input, slideIdx, fix.Params)
	case "reshape_grid":
		return applyReshapeGrid(input, slideIdx, fix.Params)
	case "set_pattern_style":
		return applySetPatternStyle(input, slideIdx, fix.Params)
	case "set_max_height_pct":
		return applySetMaxHeightPct(input, slideIdx, fix.Params)
	case "reduce_cell_text":
		return applyReduceCellText(input, slideIdx, fix.Params)
	case "rename_field":
		return applyRenameField(input, slideIdx, fix.Params)
	case "reshape_value":
		return applyReshapeValue(input, slideIdx, fix.Params)
	case "provide_value":
		return applyProvideValue(input, slideIdx, fix.Params)
	case "replace_value":
		return applyReplaceValue(input, slideIdx, fix.Params)
	case "reduce_items":
		return applyReduceItems(input, slideIdx, fix.Params)
	case "add_items":
		return applyAddItems(input, slideIdx, fix.Params)
	case "resize_list":
		return applyResizeList(input, slideIdx, fix.Params)
	case "remove_key":
		return applyRemoveKey(input, slideIdx, fix.Params)
	case "remove_field":
		return applyRemoveField(input, slideIdx, fix.Params)
	case "autofix_visual":
		return applyAutofixVisual(input, slideIdx, fix.Params)
	default:
		return unappliedFix(fix.Kind)
	}
}

// unappliedFix explains a kind applyRepairFix did not handle. A registered
// advisory kind is not a caller mistake — findings legitimately emit it — so it
// answers with the kind's guidance and the executable kinds that address the
// same defect, instead of the bare "kind_not_supported" that stalled the
// documented repair loop (go-slide-creator-ui4c).
func unappliedFix(kind string) appliedFix {
	out := appliedFix{
		Kind:           kind,
		Applied:        false,
		Code:           "kind_not_supported",
		Message:        "kind_not_supported",
		SupportedKinds: repairFixKinds(),
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool:         "get_capabilities",
			ArgsTemplate: map[string]any{},
		},
	}
	if info, ok := patterns.FixKind(kind); ok && info.Class == patterns.FixClassAdvisory {
		out.Code = "advisory_fix_kind"
		out.Message = info.Guidance
		out.Alternatives = info.Alternatives
	}
	return out
}

// ellipsis marks truncated text, matching reduce_cell_text's single U+2026.
const ellipsis = "\u2026"

// minBulletWords / minBulletChars are the floors a proportional trim respects: a
// bullet cut below them is a fragment, not a shorter bullet.
const (
	minBulletWords = 4
	minBulletChars = 24
)

// reduceTextBudget is the budget a reduce_text directive asks for. Findings
// express the same limit three ways — BODY_TOO_LONG in words, cell/placeholder
// overflow in chars (max_chars), bullet-count findings in items — and before
// go-slide-creator-9zof only max_items and max_length were honored, so
// BODY_TOO_LONG's own next_tool_call
// (repair_slide{reduce_text, {current_words, max_words}}) applied nothing and
// reported "no text content found to reduce on this slide".
type reduceTextBudget struct {
	maxItems int
	maxWords int
	maxChars int
	confirm  bool
}

func newReduceTextBudget(params map[string]any) reduceTextBudget {
	chars := intParam(params, "max_length", 0)
	if chars <= 0 {
		// Cell and placeholder overflow findings carry max_chars.
		chars = intParam(params, "max_chars", 0)
	}
	return reduceTextBudget{
		maxItems: intParam(params, "max_items", 0),
		maxWords: intParam(params, "max_words", 0),
		maxChars: chars,
		confirm:  boolParam(params, "confirm_semantic_change", false),
	}
}

// active reports whether the directive asks for anything at all.
func (b reduceTextBudget) active() bool {
	return b.maxItems > 0 || b.maxWords > 0 || b.maxChars > 0
}

// applyReduceText trims text, bullets, body_and_bullets and bullet_groups on a
// slide to a max_items / max_words / max_chars budget.
// When params["path"] is set (e.g. "/slides/0/content/body"), only the matching
// content item is targeted; otherwise all content on the slide is processed.
func applyReduceText(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := &input.Slides[slideIdx]
	targetPath := stringParam(params, "path", "")
	budget := newReduceTextBudget(params)
	if !budget.active() {
		return appliedFix{Kind: "reduce_text", Applied: false, Message: "no budget given: set max_words, max_chars (alias max_length), or max_items"}
	}

	modified := false
	for i := range slide.Content {
		ci := &slide.Content[i]
		if targetPath != "" && !contentMatchesPath(slideIdx, i, ci.PlaceholderID, targetPath) {
			continue
		}
		changed, refusal := reduceContentItem(ci, budget)
		if refusal != nil {
			return *refusal
		}
		if changed {
			modified = true
		}
	}

	if modified {
		return appliedFix{Kind: "reduce_text", Applied: true}
	}
	return reduceTextNoTarget(slide, slideIdx, targetPath, params)
}

// reduceContentItem applies the budget to one content item, returning whether it
// changed and a refusal when trimming would drop a protected fact.
func reduceContentItem(ci *ContentInput, b reduceTextBudget) (bool, *appliedFix) {
	modified := false

	if ci.BulletsValue != nil {
		trimmed, changed, refusal := reduceBulletList(*ci.BulletsValue, b)
		if refusal != nil {
			return false, refusal
		}
		if changed {
			ci.BulletsValue = &trimmed
			modified = true
		}
	}

	if bab := ci.BodyAndBulletsValue; bab != nil {
		trimmed, changed, refusal := reduceBulletList(bab.Bullets, b)
		if refusal != nil {
			return false, refusal
		}
		if changed {
			bab.Bullets = trimmed
			modified = true
		}
		if body, changed, refusal := reduceParagraph(bab.Body, b); refusal != nil {
			return false, refusal
		} else if changed {
			bab.Body = body
			modified = true
		}
	}

	if bg := ci.BulletGroupsValue; bg != nil {
		if b.maxItems > 0 && len(bg.Groups) > b.maxItems {
			bg.Groups = bg.Groups[:b.maxItems]
			modified = true
		}
		// A word/char budget applies across the groups' bullets, not to the
		// group count: dropping a whole group loses a heading and its points.
		for gi := range bg.Groups {
			trimmed, changed, refusal := reduceBulletList(bg.Groups[gi].Bullets, reduceTextBudget{
				maxWords: perGroupBudget(b.maxWords, len(bg.Groups)),
				maxChars: perGroupBudget(b.maxChars, len(bg.Groups)),
				confirm:  b.confirm,
			})
			if refusal != nil {
				return false, refusal
			}
			if changed {
				bg.Groups[gi].Bullets = trimmed
				modified = true
			}
		}
	}

	if ci.TextValue != nil {
		trimmed, changed, refusal := reduceParagraph(*ci.TextValue, b)
		if refusal != nil {
			return false, refusal
		}
		if changed {
			ci.TextValue = &trimmed
			modified = true
		}
	}

	return modified, nil
}

// perGroupBudget splits a slide-level budget across N bullet groups.
func perGroupBudget(total, groups int) int {
	if total <= 0 || groups <= 0 {
		return 0
	}
	per := total / groups
	if per < 1 {
		per = 1
	}
	return per
}

// reduceBulletList trims a bullet list to the budget. max_items cuts the list;
// a word or char budget is distributed across the surviving bullets in
// proportion to their length, so nine long bullets become nine short ones
// instead of two long ones — dropping seven bullets would lose seven points
// (go-slide-creator-9zof).
func reduceBulletList(items []string, b reduceTextBudget) ([]string, bool, *appliedFix) {
	if len(items) == 0 {
		return items, false, nil
	}
	out := append([]string(nil), items...)
	changed := false
	if b.maxItems > 0 && len(out) > b.maxItems {
		dropped := strings.Join(out[b.maxItems:], " ")
		kept := strings.Join(out[:b.maxItems], " ")
		if !b.confirm && losesProtectedFacts(kept+" "+dropped, kept) {
			return items, false, &appliedFix{
				Kind:    "reduce_text",
				Applied: false,
				Code:    "semantic_review_required",
				Message: fmt.Sprintf("dropping bullets %d-%d would remove a number, unit, negation, or qualifier — rewrite them shorter, move them to a second slide, or pass confirm_semantic_change", b.maxItems+1, len(out)),
			}
		}
		out = out[:b.maxItems]
		changed = true
	}

	total := 0
	for _, it := range out {
		total += len(strings.Fields(it))
	}
	if b.maxWords <= 0 || total <= b.maxWords {
		if b.maxChars > 0 {
			trimmed, charChanged, refusal := reduceBulletChars(out, b)
			if refusal != nil {
				return items, false, refusal
			}
			return trimmed, changed || charChanged, nil
		}
		return out, changed, nil
	}

	for i := range out {
		words := strings.Fields(out[i])
		if len(words) == 0 {
			continue
		}
		share := b.maxWords * len(words) / total
		if share < minBulletWords {
			share = minBulletWords
		}
		if share >= len(words) {
			continue
		}
		shortened := strings.Join(words[:share], " ") + ellipsis
		if !b.confirm && losesProtectedFacts(out[i], shortened) {
			return items, false, &appliedFix{
				Kind:    "reduce_text",
				Applied: false,
				Code:    "semantic_review_required",
				Message: fmt.Sprintf("trimming bullet %d from %d to %d words would remove a number, unit, negation, or qualifier — rewrite it shorter, split the slide, or pass confirm_semantic_change", i+1, len(words), share),
			}
		}
		out[i] = shortened
		changed = true
	}
	return out, changed, nil
}

// reduceBulletChars enforces a character budget across bullets the same way.
func reduceBulletChars(items []string, b reduceTextBudget) ([]string, bool, *appliedFix) {
	total := 0
	for _, it := range items {
		total += len([]rune(it))
	}
	if b.maxChars <= 0 || total <= b.maxChars {
		return items, false, nil
	}
	out := append([]string(nil), items...)
	changed := false
	for i := range out {
		runes := len([]rune(out[i]))
		if runes == 0 {
			continue
		}
		share := b.maxChars * runes / total
		if share < minBulletChars {
			share = minBulletChars
		}
		if share >= runes {
			continue
		}
		shortened := truncateWordsToChars(out[i], share)
		if !b.confirm && losesProtectedFacts(out[i], shortened) {
			return items, false, &appliedFix{
				Kind:    "reduce_text",
				Applied: false,
				Code:    "semantic_review_required",
				Message: fmt.Sprintf("trimming bullet %d to %d characters would remove a number, unit, negation, or qualifier — rewrite it shorter or pass confirm_semantic_change", i+1, share),
			}
		}
		out[i] = shortened
		changed = true
	}
	return out, changed, nil
}

// reduceParagraph trims a single body string to the word / char budget at a word
// boundary.
func reduceParagraph(text string, b reduceTextBudget) (string, bool, *appliedFix) {
	if strings.TrimSpace(text) == "" {
		return text, false, nil
	}
	trimmed, ok := shortenTitleAtWordBoundary(text, b.maxWords, b.maxChars)
	if !ok {
		return text, false, nil
	}
	if !b.confirm && losesProtectedFacts(text, trimmed) {
		return text, false, &appliedFix{
			Kind:    "reduce_text",
			Applied: false,
			Code:    "semantic_review_required",
			Message: "truncation would remove a number, unit, negation, or qualifier; recompose/split or explicitly confirm semantic change",
		}
	}
	return trimmed, true, nil
}

// truncateWordsToChars trims text to at most maxChars runes, cutting at a word
// boundary and appending an ellipsis.
func truncateWordsToChars(text string, maxChars int) string {
	if len([]rune(text)) <= maxChars {
		return text
	}
	words := strings.Fields(text)
	kept := words
	for len(kept) > 1 && len([]rune(strings.Join(kept, " ")))+1 > maxChars {
		kept = kept[:len(kept)-1]
	}
	joined := strings.Join(kept, " ")
	if len([]rune(joined))+1 > maxChars {
		// A single word longer than the budget: fall back to a rune cut.
		return truncateWithEllipsis(joined, maxChars)
	}
	return joined + ellipsis
}

// reduceTextNoTarget explains a reduce_text directive that matched nothing.
// On a shape_grid slide the right kind is reduce_cell_text with a cell_path, and
// fit_overflow findings on grid cells used to suggest reduce_text — so the answer
// names the kind and the cell instead of a bare failure (go-slide-creator-9zof).
func reduceTextNoTarget(slide *SlideInput, slideIdx int, targetPath string, params map[string]any) appliedFix {
	cellPath := gridCellPath(targetPath)
	if cellPath == "" && slide.ShapeGrid != nil && len(slide.Content) == 0 {
		cellPath = fullestGridCellPath(slide, slideIdx)
	}
	if cellPath == "" {
		return appliedFix{Kind: "reduce_text", Applied: false, Message: "no text content found to reduce on this slide"}
	}
	maxChars := intParam(params, "max_chars", 0)
	if maxChars <= 0 {
		maxChars = intParam(params, "max_length", 0)
	}
	fixParams := map[string]any{"cell_path": cellPath}
	if maxChars > 0 {
		fixParams["max_chars"] = maxChars
	}
	return appliedFix{
		Kind:       "reduce_text",
		Applied:    false,
		Code:       "wrong_kind_for_target",
		DidYouMean: "reduce_cell_text",
		Message:    fmt.Sprintf("this slide's text lives in a shape_grid cell, not a content item — reduce_text cannot reach it; apply reduce_cell_text with cell_path %q", cellPath),
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool: "repair_slide",
			ArgsTemplate: map[string]any{
				"slide_index": slideIdx,
				"fixes":       []any{map[string]any{"kind": "reduce_cell_text", "params": fixParams}},
			},
		},
	}
}

// gridCellPath reduces a finding path that points inside a shape_grid cell to
// the cell path reduce_cell_text expects
// ("/slides/0/shape_grid/rows/1/cells/2"), dropping any trailing field such as
// "/shape/text". Returns "" when the path is not a grid-cell path.
func gridCellPath(path string) string {
	if path == "" {
		return ""
	}
	slideIdx, rowIdx, cellIdx, ok := slidepath.ParseGridCell(path)
	if !ok {
		return ""
	}
	return slidepath.GridCell(slideIdx, rowIdx, cellIdx)
}

// fullestGridCellPath returns the path of the grid cell carrying the most text,
// the best guess at what an untargeted reduce_text on a grid-only slide meant.
// Returns "" when the slide has no cell with text.
func fullestGridCellPath(slide *SlideInput, slideIdx int) string {
	if slide.ShapeGrid == nil {
		return ""
	}
	best, bestLen := "", 0
	for r, row := range slide.ShapeGrid.Rows {
		for c, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
				continue
			}
			if n := len(cell.Shape.Text); n > bestLen {
				best, bestLen = slidepath.GridCell(slideIdx, r, c), n
			}
		}
	}
	return best
}

// applyShortenTitle truncates the title placeholder text.
// When params["path"] is set, the specific placeholder is targeted by path.
// minShortenedTitleWords is the fewest words a truncated title may keep. Below
// this a headline is a fragment, not a shorter headline.
const minShortenedTitleWords = 3

// maxShortenedTitleWordLossFrac is the share of a title's words that truncation
// may drop. At or beyond it the remedy is rewriting, not cutting — losing half a
// headline leaves a fragment — so the fix refuses and reports
// semantic_review_required.
const maxShortenedTitleWordLossFrac = 0.5

func applyShortenTitle(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := &input.Slides[slideIdx]
	targetPath := stringParam(params, "path", "")

	// The finding that suggests this fix (HEADLINE_TOO_LONG) describes the
	// problem in WORDS, while repair_slide's own description documents
	// max_length in characters. Accept both so an agent replaying the finding's
	// next_tool_call verbatim and an agent constructing the call from the tool
	// description get the same behaviour (go-slide-creator-28zf).
	maxWords := intParam(params, "max_words", 0)
	maxLength := intParam(params, "max_length", 0)
	if maxWords <= 0 && maxLength <= 0 {
		maxLength = 50 // historical default
	}

	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID != "title" {
			continue
		}
		if targetPath != "" && !contentMatchesPath(slideIdx, i, ci.PlaceholderID, targetPath) {
			continue
		}
		if ci.TextValue == nil {
			continue
		}

		original := *ci.TextValue
		truncated, ok := shortenTitleAtWordBoundary(original, maxWords, maxLength)
		if !ok {
			return appliedFix{Kind: "shorten_title", Applied: false, Message: "title already within the requested budget"}
		}

		// A title cut down to a fragment is worse than a long title: it reads as
		// broken and it makes the score look better, which is exactly the
		// wrong trade. Refuse and let the author rewrite (go-slide-creator-28zf).
		origWords := len(strings.Fields(original))
		keptWords := len(strings.Fields(truncated))
		if keptWords < minShortenedTitleWords ||
			(origWords > 0 && float64(origWords-keptWords)/float64(origWords) >= maxShortenedTitleWordLossFrac) {
			return appliedFix{
				Kind:    "shorten_title",
				Applied: false,
				Code:    "semantic_review_required",
				Message: fmt.Sprintf(
					"shortening to the requested budget would cut the title from %d words to %d, leaving a fragment — rewrite the headline instead of truncating it, or move the detail into the body or takeaway",
					origWords, keptWords),
			}
		}
		if !boolParam(params, "confirm_semantic_change", false) && losesProtectedFacts(original, truncated) {
			return appliedFix{Kind: "shorten_title", Applied: false, Code: "semantic_review_required", Message: "truncation would remove a number, unit, negation, or qualifier"}
		}

		ci.TextValue = &truncated
		return appliedFix{Kind: "shorten_title", Applied: true}
	}
	return appliedFix{Kind: "shorten_title", Applied: false, Message: "no title placeholder found on this slide"}
}

// shortenTitleAtWordBoundary trims a title to a word or character budget,
// always cutting at a word boundary and never mid-rune. It returns the
// truncated title and whether any trimming was needed.
//
// The previous implementation byte-sliced at maxLength, producing
// "Workstream 1: Our comprehensive enterprise-wide di" — a mid-word fragment,
// and on multi-byte text a broken rune (go-slide-creator-28zf).
func shortenTitleAtWordBoundary(title string, maxWords, maxLength int) (string, bool) {
	words := strings.Fields(title)
	if len(words) == 0 {
		return title, false
	}

	kept := words
	trimmed := false
	if maxWords > 0 && len(kept) > maxWords {
		kept = kept[:maxWords]
		trimmed = true
	}

	if maxLength > 0 {
		for len(kept) > 1 && len([]rune(strings.Join(kept, " "))) > maxLength {
			kept = kept[:len(kept)-1]
			trimmed = true
		}
	}

	if !trimmed {
		return title, false
	}

	// A cut that lands on a function word reads as an unfinished sentence
	// ("... despite the"), so drop trailing articles, prepositions and
	// conjunctions before returning.
	for len(kept) > 1 && isTrailingFunctionWord(kept[len(kept)-1]) {
		kept = kept[:len(kept)-1]
	}

	// Drop a trailing colon or dash left dangling by the cut.
	out := strings.TrimRight(strings.Join(kept, " "), " :;,-–—")
	return out, out != title
}

// trailingFunctionWords are words a headline must not end on: cutting there
// leaves the line reading as though it were interrupted.
var trailingFunctionWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"of": true, "in": true, "on": true, "at": true, "to": true, "for": true,
	"with": true, "by": true, "from": true, "as": true, "into": true,
	"than": true, "that": true, "which": true, "via": true, "per": true,
	"despite": true, "across": true, "over": true, "under": true, "between": true,
}

// isTrailingFunctionWord reports whether a word (ignoring trailing punctuation
// and case) is one a headline must not end on.
func isTrailingFunctionWord(word string) bool {
	w := strings.ToLower(strings.Trim(word, " .,;:!?-–—"))
	return trailingFunctionWords[w]
}

// applySplitAtRow wraps the target slide in a split_slide envelope, delegating
// to the existing split_slide machinery.
// When params["path"] is set, the specific content-level table is targeted.
func applySplitAtRow(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := input.Slides[slideIdx]
	targetPath := stringParam(params, "path", "")

	// Check that the slide has a table — use path to disambiguate if given.
	tableIdx := -1
	if targetPath != "" {
		for i := range slide.Content {
			if slide.Content[i].Type == "table" && contentMatchesPath(slideIdx, i, slide.Content[i].PlaceholderID, targetPath) {
				tableIdx = i
				break
			}
		}
	} else {
		tableIdx, _ = findTableContent(slide.Content)
	}
	if tableIdx < 0 {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: "slide has no table content to split"}
	}

	groupSize := intParam(params, "row", 0)
	if groupSize <= 0 {
		groupSize = intParam(params, "group_size", 0)
	}
	if groupSize <= 0 {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: "row (rows per page) parameter is required and must be > 0"}
	}

	titleSuffix := stringParam(params, "title_suffix", " ({page}/{total})")
	repeatHeaders := boolParam(params, "repeat_headers", true)

	splitInput := SplitSlideInput{
		Type: "split_slide",
		Base: slide,
		Split: SplitConfig{
			By:            "table.rows",
			GroupSize:     groupSize,
			TitleSuffix:   titleSuffix,
			RepeatHeaders: repeatHeaders,
		},
	}

	expanded, err := expandSplitSlide(splitInput)
	if err != nil {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: fmt.Sprintf("split failed: %v", err)}
	}

	// Replace the original slide with the expanded slides.
	newSlides := make([]SlideInput, 0, len(input.Slides)-1+len(expanded))
	newSlides = append(newSlides, input.Slides[:slideIdx]...)
	newSlides = append(newSlides, expanded...)
	newSlides = append(newSlides, input.Slides[slideIdx+1:]...)
	input.Slides = newSlides

	return appliedFix{
		Kind:    "split_at_row",
		Applied: true,
		Message: fmt.Sprintf("split into %d slides with %d rows each", len(expanded), groupSize),
	}
}

// applySwapLayout changes the slide's layout_id.
func applySwapLayout(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	layoutID := stringParam(params, "layout_id", "")
	if layoutID == "" {
		return appliedFix{Kind: "swap_layout", Applied: false, Message: "layout_id parameter is required"}
	}

	input.Slides[slideIdx].LayoutID = layoutID
	return appliedFix{Kind: "swap_layout", Applied: true}
}

// applyUseOneOf replaces a specific field value on the slide.
func applyUseOneOf(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	value := stringParam(params, "value", "")

	if value == "" {
		return appliedFix{Kind: "use_one_of", Applied: false, Message: "value parameter is required"}
	}

	slide := &input.Slides[slideIdx]

	// Handle common paths.
	switch path {
	case "layout_id":
		slide.LayoutID = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "transition":
		slide.Transition = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "transition_speed":
		slide.TransitionSpeed = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "build":
		slide.Build = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	default:
		// For content-level paths, try to match placeholder_id.type
		for i := range slide.Content {
			ci := &slide.Content[i]
			if path == fmt.Sprintf("content[%d].type", i) || path == ci.PlaceholderID+".type" {
				ci.Type = value
				return appliedFix{Kind: "use_one_of", Applied: true}
			}
		}
		return appliedFix{Kind: "use_one_of", Applied: false, Message: fmt.Sprintf("path %q not recognized for slide-level use_one_of", path)}
	}
}

// applyReplaceColor replaces occurrences of a specific color in shape_grid fills.
// Accepts params from contrast_autofixed findings (original_color/replacement_color)
// or the canonical form (from/to).
func applyReplaceColor(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	from := stringParam(params, "from", "")
	to := stringParam(params, "to", "")

	// Also accept the names emitted by contrast_autofixed findings.
	if from == "" {
		from = stringParam(params, "original_color", "")
	}
	if to == "" {
		to = stringParam(params, "replacement_color", "")
	}

	if from == "" || to == "" {
		return appliedFix{Kind: "replace_color", Applied: false, Message: "from/to (or original_color/replacement_color) parameters are required"}
	}

	slide := &input.Slides[slideIdx]
	if slide.ShapeGrid == nil {
		return appliedFix{Kind: "replace_color", Applied: false, Message: "slide has no shape_grid"}
	}

	modified := replaceColorInShapeGrid(slide.ShapeGrid, from, to)
	if modified {
		return appliedFix{Kind: "replace_color", Applied: true}
	}
	return appliedFix{Kind: "replace_color", Applied: false, Message: fmt.Sprintf("color %q not found in shape_grid fills", from)}
}

// applyUseSemanticColor replaces a hex fill at a specific path with a semantic
// scheme color name (e.g. "accent1", "dk1").
func applyUseSemanticColor(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	value := stringParam(params, "value", "")

	if value == "" {
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "value parameter is required (scheme color name, e.g. accent1)"}
	}

	slide := &input.Slides[slideIdx]
	if slide.ShapeGrid == nil {
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "slide has no shape_grid"}
	}

	// If a path is provided, try to resolve it to a specific cell fill.
	if path != "" {
		if setFillAtPath(slide.ShapeGrid, slideIdx, path, value) {
			return appliedFix{Kind: "use_semantic_color", Applied: true}
		}
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: fmt.Sprintf("path %q not found or not a fill field", path)}
	}

	// Without a path, replace all hex fills on the slide with the semantic color.
	modified := replaceAllHexFills(slide.ShapeGrid, value)
	if modified {
		return appliedFix{Kind: "use_semantic_color", Applied: true}
	}
	return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "no hex fills found in shape_grid"}
}

// replaceColorInShapeGrid walks all cells in a shape grid and replaces fill
// colors matching `from` with `to`.
func replaceColorInShapeGrid(grid *ShapeGridInput, from, to string) bool {
	modified := false
	fromNorm := normalizeColor(from)
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil || cell.Shape == nil {
				continue
			}
			if replaceFillColor(cell.Shape, fromNorm, to) {
				modified = true
			}
		}
	}
	return modified
}

// replaceFillColor replaces a fill color on a shape spec if it matches fromNorm.
func replaceFillColor(shape *ShapeSpecInput, fromNorm, to string) bool {
	if len(shape.Fill) == 0 {
		return false
	}

	// Try string form.
	var s string
	if err := json.Unmarshal(shape.Fill, &s); err == nil {
		if normalizeColor(s) == fromNorm {
			newFill, _ := json.Marshal(to)
			shape.Fill = newFill
			return true
		}
		return false
	}

	// Try object form.
	var obj ShapeFillInput
	if err := json.Unmarshal(shape.Fill, &obj); err == nil {
		if normalizeColor(obj.Color) == fromNorm {
			obj.Color = to
			newFill, _ := json.Marshal(obj)
			shape.Fill = newFill
			return true
		}
	}
	return false
}

// setFillAtPath sets the fill color at a specific path like
// "/slides/N/shape_grid/rows/R/cells/C/shape/fill".
func setFillAtPath(grid *ShapeGridInput, slideIdx int, path, value string) bool {
	pathSlideIdx, rowIdx, cellIdx, ok := slidepath.ParseGridCell(path)
	if !ok {
		return false
	}
	// Verify the slide index matches.
	if pathSlideIdx != slideIdx {
		return false
	}
	if rowIdx < 0 || rowIdx >= len(grid.Rows) {
		return false
	}
	if cellIdx < 0 || cellIdx >= len(grid.Rows[rowIdx].Cells) {
		return false
	}
	cell := grid.Rows[rowIdx].Cells[cellIdx]
	if cell == nil || cell.Shape == nil {
		return false
	}
	newFill, _ := json.Marshal(value)
	cell.Shape.Fill = newFill
	return true
}

// replaceAllHexFills replaces all hex fill colors in a shape grid with a semantic color.
func replaceAllHexFills(grid *ShapeGridInput, semanticColor string) bool {
	modified := false
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil || cell.Shape == nil || len(cell.Shape.Fill) == 0 {
				continue
			}
			// Check string form.
			var s string
			if err := json.Unmarshal(cell.Shape.Fill, &s); err == nil {
				if isHexColor(s) {
					newFill, _ := json.Marshal(semanticColor)
					cell.Shape.Fill = newFill
					modified = true
				}
				continue
			}
			// Check object form.
			var obj ShapeFillInput
			if err := json.Unmarshal(cell.Shape.Fill, &obj); err == nil {
				if isHexColor(obj.Color) {
					obj.Color = semanticColor
					newFill, _ := json.Marshal(obj)
					cell.Shape.Fill = newFill
					modified = true
				}
			}
		}
	}
	return modified
}

// normalizeColor normalizes a color string for comparison by lowercasing and
// stripping leading "#".
func normalizeColor(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	c = strings.TrimPrefix(c, "#")
	return c
}

// isHexColor reports whether a string looks like a hex color (#RGB, #RRGGBB, or without #).
func isHexColor(s string) bool {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// contentMatchesPath reports whether a content item at the given slide/content
// index matches the provided JSON Pointer path. Matches by placeholder name
// (e.g. "/slides/0/content/body") or by array index (e.g. "/slides/0/content/1").
func contentMatchesPath(slideIdx, contentIdx int, placeholderID, path string) bool {
	// Match by placeholder name.
	if path == slidepath.Content(slideIdx, placeholderID) {
		return true
	}
	// Match by array index.
	if path == slidepath.ContentIndex(slideIdx, contentIdx) {
		return true
	}
	// Also match if path is a sub-path (e.g. "/slides/0/content/body/text").
	byName := slidepath.Content(slideIdx, placeholderID)
	if slidepath.HasPrefix(path, byName) {
		return true
	}
	return slidepath.HasPrefix(path, slidepath.ContentIndex(slideIdx, contentIdx))
}

// applyAutofixVisual accepts a visual QA finding and applies the first
// successful fix from the category's mapped fix kinds. The params must include
// "category" (the visual QA finding category). Additional params are forwarded
// to the underlying fix kind handler.
func applyAutofixVisual(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	category := stringParam(params, "category", "")
	if category == "" {
		return appliedFix{Kind: "autofix_visual", Applied: false, Message: "category parameter is required (visual QA finding category)"}
	}

	candidates := visualqa.SuggestedFixesForCategory(category)
	if len(candidates) == 0 {
		return appliedFix{Kind: "autofix_visual", Applied: false, Message: fmt.Sprintf("no repair mapping for visual QA category %q", category)}
	}

	// Try each candidate fix kind in order until one succeeds.
	for _, candidate := range candidates {
		// Merge candidate params with caller-supplied params (caller wins).
		mergedParams := make(map[string]any)
		for k, v := range candidate.Params {
			mergedParams[k] = v
		}
		for k, v := range params {
			if k == "category" {
				continue // don't forward the category itself
			}
			mergedParams[k] = v
		}

		result := applyRepairFix(input, slideIdx, repairFixInput{
			Kind:   candidate.Kind,
			Params: mergedParams,
		})
		if result.Applied {
			return appliedFix{
				Kind:    "autofix_visual",
				Applied: true,
				Message: fmt.Sprintf("applied %s for %s category", candidate.Kind, category),
			}
		}
	}

	return appliedFix{
		Kind:    "autofix_visual",
		Applied: false,
		Message: fmt.Sprintf("none of the candidate fixes %v succeeded for category %q", fixKindNames(candidates), category),
	}
}

// fixKindNames extracts the kind names from a slice of SuggestedFix.
func fixKindNames(fixes []visualqa.SuggestedFix) []string {
	names := make([]string, len(fixes))
	for i, f := range fixes {
		names[i] = f.Kind
	}
	return names
}

// --- Helpers ---

// validateRepairBoundary checks required fields for repair_slide.
func validateRepairBoundary(input *PresentationInput) *mcp.CallToolResult {
	var diags []diagnostics.Diagnostic
	if input.Template == "" {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required",
			Severity: diagnostics.SeverityError,
		})
	}
	if len(input.Slides) == 0 {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity: diagnostics.SeverityError,
		})
	}
	if diagnostics.HasErrors(diags) {
		return api.MCPDiagnosticsError(diags)
	}
	return nil
}

// extractSlideIndex extracts and validates the slide_index parameter.
func extractSlideIndex(request mcp.CallToolRequest, slideCount int) (int, error) {
	args := request.GetArguments()
	raw, ok := args["slide_index"]
	if !ok {
		return 0, fmt.Errorf("slide_index is required")
	}

	// MCP passes numbers as float64.
	var idx int
	switch v := raw.(type) {
	case float64:
		idx = int(v)
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("slide_index must be an integer, got %v", raw)
		}
		idx = int(i)
	default:
		return 0, fmt.Errorf("slide_index must be a number, got %T", raw)
	}

	if idx < 0 || idx >= slideCount {
		return 0, fmt.Errorf("slide_index %d out of range (deck has %d slides, valid range 0-%d)", idx, slideCount, slideCount-1)
	}
	return idx, nil
}

// extractFixes extracts the fixes array from the request.
func extractFixes(request mcp.CallToolRequest) ([]repairFixInput, error) {
	args := request.GetArguments()
	raw, ok := args["fixes"]
	if !ok {
		return nil, fmt.Errorf("fixes is required")
	}

	// Re-marshal and unmarshal to handle the various shapes MCP might send.
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("fixes: %w", err)
	}

	var fixes []repairFixInput
	if err := json.Unmarshal(data, &fixes); err != nil {
		return nil, fmt.Errorf("fixes must be an array of {kind, params?} objects: %w", err)
	}

	return fixes, nil
}

// filterFindingsForSlide returns findings whose path references the given slide index.
func filterFindingsForSlide(findings []patterns.FitFinding, slideIdx int) []patterns.FitFinding {
	prefix := slidepath.Slide(slideIdx)
	var filtered []patterns.FitFinding
	for _, f := range findings {
		if slidepath.HasPrefix(f.Path, prefix) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// intParam extracts an integer parameter with a default.
func intParam(params map[string]any, key string, defaultVal int) int {
	if params == nil {
		return defaultVal
	}
	raw, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case int:
		return v
	}
	return defaultVal
}

// floatParam extracts a numeric parameter, reporting whether it was present and
// numeric. Unlike intParam it does not fold a missing key into a default, since
// "no value" and "0" mean different things for a percentage.
func floatParam(params map[string]any, key string) (float64, bool) {
	if params == nil {
		return 0, false
	}
	raw, ok := params[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// stringParam extracts a string parameter with a default.
func stringParam(params map[string]any, key string, defaultVal string) string {
	if params == nil {
		return defaultVal
	}
	if v, ok := params[key].(string); ok {
		return v
	}
	return defaultVal
}

// applySplitPattern splits a pattern/shape_grid slide into two slides by
// dividing the grid rows so that roughly "first" cells end up on slide 1
// and "second" cells on slide 2. The second slide gets a title suffix.
func applySplitPattern(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := input.Slides[slideIdx]
	grid := slide.ShapeGrid
	if (grid == nil || len(grid.Rows) == 0) && slide.Pattern != nil {
		if stringParam(params, "path", "") == "" {
			// Findings on a pattern slide describe the problem (12 filled
			// slots, recommended max 9), not the field to split, so every
			// auto_repair directive refused with "slide has no shape_grid"
			// and the loop stalled at zero repairs (go-slide-creator-wmfo).
			// Infer the repeated field: the values array the pattern renders
			// one cell per.
			if key := longestPatternValuesArray(slide.Pattern); key != "" {
				next := make(map[string]any, len(params)+1)
				for k, v := range params {
					next[k] = v
				}
				next["path"] = key
				params = next
			}
		}
		if stringParam(params, "path", "") != "" {
			return splitPatternValues(input, slideIdx, params)
		}
	}
	if grid == nil || len(grid.Rows) == 0 {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: "slide has no shape_grid to split (pass path to split a pattern.values array)"}
	}

	firstN := intParam(params, "first", 0)
	if firstN <= 0 {
		// Default: split evenly by total filled cells.
		total := countFilledCells(grid)
		firstN = (total + 1) / 2
	}
	titlePart2 := stringParam(params, "title_part_2", "(continued)")

	// Walk rows, accumulating cells until we reach the split point.
	splitRow := findSplitRow(grid, firstN)
	if splitRow <= 0 || splitRow >= len(grid.Rows) {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: "cannot determine a valid row split point"}
	}

	// Build two slides from the original.
	slide1 := cloneSlideForSplit(slide, grid.Rows[:splitRow], grid)
	slide2 := cloneSlideForSplit(slide, grid.Rows[splitRow:], grid)

	// Apply title suffix to slide 2.
	appendTitleSuffix(slide2.Content, " "+titlePart2)

	// Only first slide gets speaker notes and source.
	slide2.SpeakerNotes = ""
	slide2.Source = ""

	// Clear the pattern field on both — the grid is already expanded.
	slide1.Pattern = nil
	slide2.Pattern = nil

	// Replace original slide with the two new slides.
	newSlides := make([]SlideInput, 0, len(input.Slides)+1)
	newSlides = append(newSlides, input.Slides[:slideIdx]...)
	newSlides = append(newSlides, slide1, slide2)
	newSlides = append(newSlides, input.Slides[slideIdx+1:]...)
	input.Slides = newSlides

	cells1 := countFilledCells(slide1.ShapeGrid)
	cells2 := countFilledCells(slide2.ShapeGrid)
	return appliedFix{
		Kind:    "split_pattern",
		Applied: true,
		Message: fmt.Sprintf("split into 2 slides (%d + %d cells)", cells1, cells2),
	}
}

// longestPatternValuesArray returns the key of the array field in a pattern's
// values that carries the most items — the repeated field a pattern renders one
// cell per, and so the one a split divides. Returns "" when no field holds at
// least two items, or when two fields tie (splitting the wrong one would
// silently restructure the slide).
func longestPatternValuesArray(pattern *PatternInput) string {
	if pattern == nil || len(pattern.Values) == 0 {
		return ""
	}
	var values map[string]any
	if err := json.Unmarshal(pattern.Values, &values); err != nil {
		return ""
	}
	best, bestLen, tied := "", 1, false
	for key, raw := range values {
		arr, ok := raw.([]any)
		if !ok || len(arr) < 2 {
			continue
		}
		switch {
		case len(arr) > bestLen:
			best, bestLen, tied = key, len(arr), false
		case len(arr) == bestLen && key != best:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}

// splitPatternValues splits a pattern slide into two by dividing the array at
// pattern.values[path]: slide 1 keeps the first `first` items (default: half),
// slide 2 — a copy of the slide with the title suffixed — carries the rest. No
// item is dropped, so it is the fact-preserving alternative to reduce_items /
// resize_list.
func splitPatternValues(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := input.Slides[slideIdx]
	path := stringParam(params, "path", "")
	var valuesMap map[string]any
	if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
	}
	arr, ok := valuesMap[path].([]any)
	if !ok {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: fmt.Sprintf("field %q is not an array", path)}
	}
	firstN := intParam(params, "first", 0)
	if firstN <= 0 {
		firstN = (len(arr) + 1) / 2
	}
	if len(arr) < 2 || firstN >= len(arr) {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: fmt.Sprintf("%q has %d items; nothing to move to a second slide", path, len(arr))}
	}
	titlePart2 := stringParam(params, "title_part_2", "(continued)")

	half := func(items []any) (SlideInput, error) {
		vals := make(map[string]any, len(valuesMap))
		for k, v := range valuesMap {
			vals[k] = v
		}
		vals[path] = items
		// Patterns that declare their own grid shape require it to match the
		// item count exactly (card-grid: "cells must contain exactly 12 items
		// (columns=4 x rows=3)"), so a split that moves items without resizing
		// the grid produces a deck that fails to generate — which is how the
		// first working auto_repair pass broke the render (go-slide-creator-wmfo).
		resizeGridDims(vals, len(items))
		encoded, err := json.Marshal(vals)
		if err != nil {
			return SlideInput{}, err
		}
		out := slide
		pat := *slide.Pattern
		pat.Values = encoded
		out.Pattern = &pat
		out.ShapeGrid = nil
		out.Content = append([]ContentInput(nil), slide.Content...)
		return out, nil
	}
	slide1, err := half(arr[:firstN])
	if err != nil {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: fmt.Sprintf("failed to marshal values: %v", err)}
	}
	slide2, err := half(arr[firstN:])
	if err != nil {
		return appliedFix{Kind: "split_pattern", Applied: false, Message: fmt.Sprintf("failed to marshal values: %v", err)}
	}
	appendTitleSuffix(slide2.Content, " "+titlePart2)
	slide2.SpeakerNotes = ""
	slide2.Source = ""

	// Refuse rather than hand back a deck that cannot generate: each half must
	// satisfy the pattern's own contract. Values that were ALREADY invalid are
	// not this fix's doing — the pattern validator reports those — so the guard
	// only applies when the original expands cleanly.
	originalValid := validatePatternHalf(&slide, slideIdx) == nil
	for i, half := range []SlideInput{slide1, slide2} {
		if !originalValid {
			break
		}
		if err := validatePatternHalf(&half, slideIdx+i); err != nil {
			return appliedFix{
				Kind:    "split_pattern",
				Applied: false,
				Code:    "semantic_review_required",
				Message: fmt.Sprintf("splitting %q at %d leaves values this pattern rejects: %v — split at a different point, or move items to a new slide yourself", path, firstN, err),
			}
		}
	}

	newSlides := make([]SlideInput, 0, len(input.Slides)+1)
	newSlides = append(newSlides, input.Slides[:slideIdx]...)
	newSlides = append(newSlides, slide1, slide2)
	newSlides = append(newSlides, input.Slides[slideIdx+1:]...)
	input.Slides = newSlides
	return appliedFix{Kind: "split_pattern", Applied: true, Message: fmt.Sprintf("split %q into 2 slides (%d + %d items)", path, firstN, len(arr)-firstN)}
}

// resizeGridDims rewrites numeric "columns"/"rows" values so a pattern that
// declares its own grid shape still describes exactly n items. It only touches
// keys the values already carry.
func resizeGridDims(vals map[string]any, n int) {
	_, hasCols := numericValue(vals["columns"])
	_, hasRows := numericValue(vals["rows"])
	if !hasCols && !hasRows {
		return
	}
	cols, rows := balancedGridDims(n)
	if hasCols {
		vals["columns"] = cols
	}
	if hasRows {
		vals["rows"] = rows
	}
}

// balancedGridDims factors n into the most balanced columns x rows that is
// exactly n, preferring wider-than-tall (the shape that fills a widescreen
// slide). A prime count yields a single row.
func balancedGridDims(n int) (int, int) {
	if n <= 0 {
		return 1, 1
	}
	bestCols, bestRows, bestSpread := n, 1, n
	for rows := 1; rows*rows <= n; rows++ {
		if n%rows != 0 {
			continue
		}
		cols := n / rows
		if spread := cols - rows; spread >= 0 && spread < bestSpread {
			bestCols, bestRows, bestSpread = cols, rows, spread
		}
	}
	return bestCols, bestRows
}

// numericValue reads a JSON number that may have decoded as float64 or int.
func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// validatePatternHalf expands one half of a split to confirm the pattern accepts
// its values.
func validatePatternHalf(slide *SlideInput, slideIdx int) error {
	if slide.Pattern == nil {
		return nil
	}
	ctx := patterns.ExpandContext{
		SlideWidth:  validationDefaultSlideWidthEMU,
		SlideHeight: validationDefaultSlideHeightEMU,
		SlideIndex:  slideIdx,
	}
	_, _, err := expandPattern(slide.Pattern, ctx, patterns.Default())
	return err
}

// countFilledCells counts non-nil cells in a shape grid.
func countFilledCells(grid *ShapeGridInput) int {
	if grid == nil {
		return 0
	}
	n := 0
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				n++
			}
		}
	}
	return n
}

// findSplitRow returns the row index at which the cumulative filled cell count
// reaches or exceeds targetCells. Returns the index of the first row that
// belongs to slide 2.
func findSplitRow(grid *ShapeGridInput, targetCells int) int {
	cumulative := 0
	for ri, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				cumulative++
			}
		}
		if cumulative >= targetCells {
			return ri + 1
		}
	}
	return len(grid.Rows)
}

// cloneSlideForSplit creates a copy of the slide with the given grid rows.
func cloneSlideForSplit(src SlideInput, rows []GridRowInput, srcGrid *ShapeGridInput) SlideInput {
	newGrid := &ShapeGridInput{
		Bounds:  srcGrid.Bounds,
		Gap:     srcGrid.Gap,
		ColGap:  srcGrid.ColGap,
		RowGap:  srcGrid.RowGap,
		Columns: srcGrid.Columns,
		Rows:    make([]GridRowInput, len(rows)),
	}
	copy(newGrid.Rows, rows)

	// Copy content slice.
	content := make([]ContentInput, len(src.Content))
	copy(content, src.Content)

	return SlideInput{
		LayoutID:        src.LayoutID,
		SlideType:       src.SlideType,
		Eyebrow:         src.Eyebrow,
		Background:      src.Background,
		Content:         content,
		ShapeGrid:       newGrid,
		SpeakerNotes:    src.SpeakerNotes,
		Source:          src.Source,
		Takeaway:        src.Takeaway,
		Transition:      src.Transition,
		TransitionSpeed: src.TransitionSpeed,
		Build:           src.Build,
		ContrastCheck:   src.ContrastCheck,
	}
}

// appendTitleSuffix appends a suffix to the title content item (if present).
func appendTitleSuffix(content []ContentInput, suffix string) {
	for i := range content {
		if content[i].PlaceholderID == "title" && content[i].Type == "text" && content[i].TextValue != nil {
			newVal := *content[i].TextValue + suffix
			content[i].TextValue = &newVal
			return
		}
	}
}

// applySwapPattern replaces the slide's pattern with a new one, carrying over
// values from the params. This allows the repair loop to switch e.g. card-grid
// to kpi-3up without regenerating the entire deck.
func applySwapPattern(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	to := stringParam(params, "to", "")
	if to == "" {
		return appliedFix{Kind: "swap_pattern", Applied: false, Message: "to parameter is required (target pattern name)"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil {
		return appliedFix{Kind: "swap_pattern", Applied: false, Message: "slide has no pattern to swap"}
	}

	// Verify target pattern exists in the registry.
	reg := patterns.Default()
	if _, ok := reg.Get(to); !ok {
		msg := fmt.Sprintf("unknown target pattern %q", to)
		if suggestion, ok := reg.Suggest(to); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return appliedFix{Kind: "swap_pattern", Applied: false, Message: msg}
	}

	// Update the pattern name.
	slide.Pattern.Name = to

	// If new values are provided, replace them.
	if rawValues, ok := params["values"]; ok {
		valuesJSON, err := json.Marshal(rawValues)
		if err != nil {
			return appliedFix{Kind: "swap_pattern", Applied: false, Message: fmt.Sprintf("failed to marshal values: %v", err)}
		}
		slide.Pattern.Values = valuesJSON
	}

	// If new overrides are provided, replace them.
	if rawOverrides, ok := params["overrides"]; ok {
		overridesJSON, err := json.Marshal(rawOverrides)
		if err != nil {
			return appliedFix{Kind: "swap_pattern", Applied: false, Message: fmt.Sprintf("failed to marshal overrides: %v", err)}
		}
		slide.Pattern.Overrides = overridesJSON
	}

	// If new cell_overrides are provided, replace them.
	if rawCellOverrides, ok := params["cell_overrides"]; ok {
		coMap, ok := rawCellOverrides.(map[string]any)
		if ok {
			cellOverrides := make(map[string]json.RawMessage, len(coMap))
			for k, v := range coMap {
				data, err := json.Marshal(v)
				if err != nil {
					continue
				}
				cellOverrides[k] = data
			}
			slide.Pattern.CellOverrides = cellOverrides
		}
	}

	// Clear any pre-expanded shape_grid — the pipeline will re-expand the pattern.
	slide.ShapeGrid = nil

	return appliedFix{Kind: "swap_pattern", Applied: true, Message: fmt.Sprintf("swapped to pattern %q", to)}
}

// applyReshapeGrid changes the grid dimensions by adjusting columns and/or
// redistributing cells into new rows. This allows fixing sparse layouts by
// changing the grid shape without changing the pattern or content.
func applyReshapeGrid(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := &input.Slides[slideIdx]

	// If the slide uses a pattern, modify its values to change shape.
	if slide.Pattern != nil {
		return reshapePatternValues(slide, params)
	}

	// If the slide has a raw shape_grid, reshape it directly.
	if slide.ShapeGrid != nil {
		return reshapeRawGrid(slide, params)
	}

	return appliedFix{Kind: "reshape_grid", Applied: false, Message: "slide has no shape_grid or pattern to reshape"}
}

// reshapePatternValues updates rows/columns fields in the pattern's values JSON.
func reshapePatternValues(slide *SlideInput, params map[string]any) appliedFix {
	// Parse current values as a generic map.
	var valuesMap map[string]any
	if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
		return appliedFix{Kind: "reshape_grid", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
	}

	modified := false

	// Update rows if specified.
	if rows := intParam(params, "rows", 0); rows > 0 {
		valuesMap["rows"] = rows
		modified = true
	}

	// Update columns if specified — accept int or []int.
	if rawCols, ok := params["columns"]; ok {
		valuesMap["columns"] = rawCols
		modified = true
	}

	if !modified {
		return appliedFix{Kind: "reshape_grid", Applied: false, Message: "rows or columns parameter is required"}
	}

	newValues, err := json.Marshal(valuesMap)
	if err != nil {
		return appliedFix{Kind: "reshape_grid", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
	}
	slide.Pattern.Values = newValues

	// Clear pre-expanded grid so the pipeline re-expands with new dimensions.
	slide.ShapeGrid = nil

	return appliedFix{Kind: "reshape_grid", Applied: true}
}

// reshapeRawGrid redistributes cells in an existing shape_grid into new row/column layout.
func reshapeRawGrid(slide *SlideInput, params map[string]any) appliedFix {
	grid := slide.ShapeGrid
	newCols := intParam(params, "columns", 0)
	newRows := intParam(params, "rows", 0)

	if newCols <= 0 && newRows <= 0 {
		return appliedFix{Kind: "reshape_grid", Applied: false, Message: "rows or columns parameter is required"}
	}

	// Collect all non-nil cells from the existing grid.
	var cells []*jsonschema.GridCellInput
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				cells = append(cells, cell)
			}
		}
	}

	if len(cells) == 0 {
		return appliedFix{Kind: "reshape_grid", Applied: false, Message: "grid has no cells to redistribute"}
	}

	// Determine target layout.
	if newCols <= 0 {
		newCols = len(cells) // single row
		if newRows > 0 {
			newCols = (len(cells) + newRows - 1) / newRows
		}
	}
	if newRows <= 0 {
		newRows = (len(cells) + newCols - 1) / newCols
	}

	// Redistribute cells into new rows.
	newGridRows := make([]jsonschema.GridRowInput, 0, newRows)
	cellIdx := 0
	for r := 0; r < newRows && cellIdx < len(cells); r++ {
		rowCells := make([]*jsonschema.GridCellInput, newCols)
		for c := 0; c < newCols && cellIdx < len(cells); c++ {
			rowCells[c] = cells[cellIdx]
			cellIdx++
		}
		newGridRows = append(newGridRows, jsonschema.GridRowInput{Cells: rowCells})
	}

	grid.Rows = newGridRows

	// Update the columns field to reflect new column count.
	colJSON, _ := json.Marshal(newCols)
	grid.Columns = colJSON

	return appliedFix{Kind: "reshape_grid", Applied: true, Message: fmt.Sprintf("reshaped to %d columns × %d rows", newCols, len(newGridRows))}
}

// applySetPatternStyle changes the "style" field in a pattern's overrides.
// This allows switching e.g. timeline-horizontal from "dots" to "chevron"
// without regenerating the slide content.
func applySetPatternStyle(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	style := stringParam(params, "style", "")
	if style == "" {
		return appliedFix{Kind: "set_pattern_style", Applied: false, Message: "style parameter is required"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil {
		return appliedFix{Kind: "set_pattern_style", Applied: false, Message: "slide has no pattern"}
	}

	// Parse existing overrides (may be nil/empty).
	var overridesMap map[string]any
	if len(slide.Pattern.Overrides) > 0 {
		if err := json.Unmarshal(slide.Pattern.Overrides, &overridesMap); err != nil {
			overridesMap = make(map[string]any)
		}
	} else {
		overridesMap = make(map[string]any)
	}

	overridesMap["style"] = style
	newOverrides, err := json.Marshal(overridesMap)
	if err != nil {
		return appliedFix{Kind: "set_pattern_style", Applied: false, Message: fmt.Sprintf("failed to marshal overrides: %v", err)}
	}
	slide.Pattern.Overrides = newOverrides

	// Clear pre-expanded grid so the pipeline re-expands with new style.
	slide.ShapeGrid = nil

	return appliedFix{Kind: "set_pattern_style", Applied: true, Message: fmt.Sprintf("set style to %q", style)}
}

// applySetMaxHeightPct caps a pattern slide's height budget so its boxes shrink
// to their content instead of stretching to fill the slide. It is the mechanical
// half of the underfill / overtall-lane findings, whose own remediation steps
// already say "set max_height_pct to ~35" — before this, that advice named no
// executable directive (go-slide-creator-ui4c).
func applySetMaxHeightPct(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	const kind = "set_max_height_pct"
	pct, ok := floatParam(params, "max_height_pct")
	if !ok {
		return appliedFix{Kind: kind, Applied: false, Message: "max_height_pct parameter is required (number, 10-100)"}
	}
	if pct <= 0 || pct > 100 {
		return appliedFix{Kind: kind, Applied: false, Message: fmt.Sprintf("max_height_pct must be in (0, 100], got %g", pct)}
	}
	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil {
		return appliedFix{Kind: kind, Applied: false, Message: "slide has no pattern; cap a raw shape_grid with explicit bounds or row heights instead"}
	}
	prev := slide.Pattern.MaxHeightPct
	slide.Pattern.MaxHeightPct = pct
	// Drop the pre-expanded grid so the pipeline re-expands inside the new cap.
	slide.ShapeGrid = nil
	if prev > 0 {
		return appliedFix{Kind: kind, Applied: true, Message: fmt.Sprintf("max_height_pct %g -> %g", prev, pct)}
	}
	return appliedFix{Kind: kind, Applied: true, Message: fmt.Sprintf("capped pattern height at %g%% of the content area", pct)}
}

// applyReduceCellText truncates a shape_grid cell's text to max_chars,
// appending a single ellipsis character (U+2026). If truncation breaks a
// markdown emphasis pair (**bold** or *italic*), the orphaned markers are
// stripped from the truncated output.
//
// Agents should prefer pre-generation budget awareness via expand_pattern over
// post-generation repair.
func applyReduceCellText(input *PresentationInput, slideIdx int, params map[string]any) appliedFix { //nolint:gocyclo // Three compatible text encodings plus semantic safety checks.
	cellPath := stringParam(params, "cell_path", "")
	maxChars := intParam(params, "max_chars", 0)

	if cellPath == "" {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "cell_path parameter is required"}
	}
	if maxChars <= 1 {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "max_chars must be > 1"}
	}

	slide := &input.Slides[slideIdx]
	if slide.ShapeGrid == nil {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "slide has no shape_grid"}
	}

	pathSlideIdx, rowIdx, cellIdx, ok := slidepath.ParseGridCell(cellPath)
	if !ok {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: fmt.Sprintf("cannot parse cell path %q", cellPath)}
	}
	if pathSlideIdx != slideIdx {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: fmt.Sprintf("cell_path slide index %d does not match slide_index %d", pathSlideIdx, slideIdx)}
	}
	if rowIdx < 0 || rowIdx >= len(slide.ShapeGrid.Rows) {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: fmt.Sprintf("row index %d out of range", rowIdx)}
	}
	row := &slide.ShapeGrid.Rows[rowIdx]
	if cellIdx < 0 || cellIdx >= len(row.Cells) {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: fmt.Sprintf("cell index %d out of range", cellIdx)}
	}
	cell := row.Cells[cellIdx]
	if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "cell has no text content"}
	}

	// Extract text content — handle string form, object form (content field),
	// and paragraphs form.
	var s string
	if err := json.Unmarshal(cell.Shape.Text, &s); err == nil {
		// Simple string form.
		truncated := truncateWithEllipsis(s, maxChars)
		if truncated == s {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "text already within max_chars"}
		}
		if !boolParam(params, "confirm_semantic_change", false) && losesProtectedFacts(s, truncated) {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Code: "semantic_review_required", Message: "truncation would remove a number, unit, negation, or qualifier; reshape or split the grid instead"}
		}
		newText, _ := json.Marshal(truncated)
		cell.Shape.Text = newText
		return appliedFix{Kind: "reduce_cell_text", Applied: true}
	}

	// Object form with "content" or "paragraphs".
	var obj map[string]any
	if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
		return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "cannot parse cell text"}
	}

	// Paragraphs form: truncate each paragraph's content, distributing budget.
	if rawParas, ok := obj["paragraphs"]; ok {
		paras, ok := rawParas.([]any)
		if !ok || len(paras) == 0 {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "empty paragraphs array"}
		}
		modified := truncateParagraphs(paras, maxChars)
		if !modified {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "text already within max_chars"}
		}
		obj["paragraphs"] = paras
		newText, _ := json.Marshal(obj)
		cell.Shape.Text = newText
		return appliedFix{Kind: "reduce_cell_text", Applied: true}
	}

	// Object form with "content" string.
	if content, ok := obj["content"].(string); ok {
		truncated := truncateWithEllipsis(content, maxChars)
		if truncated == content {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "text already within max_chars"}
		}
		if !boolParam(params, "confirm_semantic_change", false) && losesProtectedFacts(content, truncated) {
			return appliedFix{Kind: "reduce_cell_text", Applied: false, Code: "semantic_review_required", Message: "truncation would remove a number, unit, negation, or qualifier; reshape or split the grid instead"}
		}
		obj["content"] = truncated
		newText, _ := json.Marshal(obj)
		cell.Shape.Text = newText
		return appliedFix{Kind: "reduce_cell_text", Applied: true}
	}

	return appliedFix{Kind: "reduce_cell_text", Applied: false, Message: "cell text has no recognizable content"}
}

var protectedFactRE = regexp.MustCompile(`(?i)(?:\b(?:not|no|never|without|only|at least|at most|approximately|about|more than|less than)\b|[-+]?\d+(?:[.,]\d+)?(?:%|x|bps|bp|k|m|bn|ms|s|h|d|gb|mb|usd|eur|chf)?\b)`)

// losesProtectedFacts prevents an automatic density repair from silently
// changing a claim. It compares the protected semantic tokens before and after
// truncation; ordinary prose can still be shortened automatically.
func losesProtectedFacts(before, after string) bool {
	remaining := append([]string(nil), protectedFactRE.FindAllString(strings.ToLower(after), -1)...)
	for _, token := range protectedFactRE.FindAllString(strings.ToLower(before), -1) {
		found := -1
		for i, candidate := range remaining {
			if candidate == token {
				found = i
				break
			}
		}
		if found < 0 {
			return true
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	return false
}

// truncateWithEllipsis truncates text to maxChars-1 visible characters plus a
// single ellipsis (U+2026). If the truncation point falls inside a markdown
// emphasis span, the orphaned markers are stripped.
func truncateWithEllipsis(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}

	// Truncate to maxChars-1 to leave room for ellipsis.
	cutLen := maxChars - 1
	if cutLen < 0 {
		cutLen = 0
	}
	truncated := string(runes[:cutLen])

	// Fix broken markdown emphasis before appending ellipsis.
	truncated = fixBrokenEmphasis(truncated)

	return truncated + "\u2026"
}

// fixBrokenEmphasis strips orphaned markdown emphasis markers from the end of
// a truncated string. It handles both ** (bold) and * (italic) markers.
//
// The approach: count unmatched opening markers. If the truncated text has an
// odd number of bold or italic delimiters (meaning one was opened but not
// closed), remove the opening marker.
func fixBrokenEmphasis(s string) string {
	// Process bold (**) first, then italic (*).
	s = fixEmphasisPair(s, "**")
	s = fixEmphasisPair(s, "*")
	return s
}

// fixEmphasisPair checks if the delimiter has an odd count (meaning an unclosed
// opening). If so, it removes the last unmatched opening occurrence.
func fixEmphasisPair(s, delim string) string {
	count := countNonOverlapping(s, delim)
	if count%2 == 0 {
		return s // balanced
	}
	// Remove the last occurrence of the delimiter.
	lastIdx := strings.LastIndex(s, delim)
	if lastIdx < 0 {
		return s
	}
	return s[:lastIdx] + s[lastIdx+len(delim):]
}

// countNonOverlapping counts non-overlapping occurrences of substr in s.
// For "**" counting, we need to handle the nesting: count ** first (consuming
// chars), then * on the remainder.
func countNonOverlapping(s, substr string) int {
	if substr == "*" {
		// When counting single *, we must not count those that are part of **.
		// Replace ** with a placeholder, count remaining *, then restore.
		temp := strings.ReplaceAll(s, "**", "\x00\x00")
		return strings.Count(temp, "*")
	}
	return strings.Count(s, substr)
}

// truncateParagraphs truncates a paragraphs array so the total content length
// fits within maxChars. Returns true if any modification was made.
func truncateParagraphs(paras []any, maxChars int) bool {
	// Calculate total length across all paragraphs.
	total := 0
	for _, p := range paras {
		pMap, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if c, ok := pMap["content"].(string); ok {
			total += len([]rune(c))
		}
	}
	if total <= maxChars {
		return false
	}

	// Distribute budget proportionally, truncating from the last paragraph.
	remaining := maxChars
	modified := false
	for i, p := range paras {
		pMap, ok := p.(map[string]any)
		if !ok {
			continue
		}
		content, ok := pMap["content"].(string)
		if !ok {
			continue
		}
		runes := []rune(content)
		if remaining <= 0 {
			// No budget left — remove this paragraph's content.
			pMap["content"] = "\u2026"
			paras[i] = pMap
			modified = true
			remaining -= 1
			continue
		}
		if len(runes) > remaining {
			pMap["content"] = truncateWithEllipsis(content, remaining)
			paras[i] = pMap
			modified = true
			remaining = 0
		} else {
			remaining -= len(runes)
		}
	}
	return modified
}

// applyRenameField renames a JSON field on the slide. The fix params contain
// "from" (the unknown key) and "to" (the correct key). Works at slide level,
// content level, and pattern values level by re-marshaling the slide JSON.
func applyRenameField(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	from := stringParam(params, "from", "")
	to := stringParam(params, "to", "")

	if from == "" || to == "" {
		return appliedFix{Kind: "rename_field", Applied: false, Message: "from and to parameters are required"}
	}

	slide := &input.Slides[slideIdx]

	// Try pattern values first.
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		if renamed, ok := renameJSONKey(slide.Pattern.Values, from, to); ok {
			slide.Pattern.Values = renamed
			slide.ShapeGrid = nil // force re-expansion
			return appliedFix{Kind: "rename_field", Applied: true, Message: fmt.Sprintf("renamed %q to %q in pattern values", from, to)}
		}
	}

	// Try slide-level fields via round-trip.
	slideJSON, err := json.Marshal(slide)
	if err != nil {
		return appliedFix{Kind: "rename_field", Applied: false, Message: fmt.Sprintf("failed to marshal slide: %v", err)}
	}
	if renamed, ok := renameJSONKey(slideJSON, from, to); ok {
		if err := json.Unmarshal(renamed, slide); err != nil {
			return appliedFix{Kind: "rename_field", Applied: false, Message: fmt.Sprintf("failed to unmarshal renamed slide: %v", err)}
		}
		return appliedFix{Kind: "rename_field", Applied: true, Message: fmt.Sprintf("renamed %q to %q", from, to)}
	}

	return appliedFix{Kind: "rename_field", Applied: false, Message: fmt.Sprintf("field %q not found on slide", from)}
}

// renameJSONKey renames a top-level key in a JSON object. Returns the updated
// JSON and true if the rename occurred.
func renameJSONKey(raw json.RawMessage, from, to string) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	val, ok := obj[from]
	if !ok {
		return nil, false
	}
	delete(obj, from)
	obj[to] = val
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, false
	}
	return result, true
}

// applyReshapeValue replaces a field's value with a restructured version.
// The fix params contain "path" (the field name) and "value" (the replacement
// value in the target shape). This is used when a value has the wrong
// structure (e.g., an array where an object is expected).
func applyReshapeValue(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	rawValue, hasValue := params["value"]

	if path == "" {
		return appliedFix{Kind: "reshape_value", Applied: false, Message: "path parameter is required"}
	}
	if !hasValue {
		return appliedFix{Kind: "reshape_value", Applied: false, Message: "value parameter is required"}
	}

	slide := &input.Slides[slideIdx]

	// Try pattern values first.
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		var valuesMap map[string]any
		if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
			return appliedFix{Kind: "reshape_value", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
		}
		if _, exists := valuesMap[path]; exists {
			valuesMap[path] = rawValue
			newValues, err := json.Marshal(valuesMap)
			if err != nil {
				return appliedFix{Kind: "reshape_value", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
			}
			slide.Pattern.Values = newValues
			slide.ShapeGrid = nil // force re-expansion
			return appliedFix{Kind: "reshape_value", Applied: true, Message: fmt.Sprintf("reshaped %q in pattern values", path)}
		}
	}

	return appliedFix{Kind: "reshape_value", Applied: false, Message: fmt.Sprintf("field %q not found in pattern values", path)}
}

// applyProvideValue sets a field in pattern values to a value supplied by the agent.
func applyProvideValue(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	rawValue, hasValue := params["value"]

	if path == "" {
		return appliedFix{Kind: "provide_value", Applied: false, Message: "path parameter is required"}
	}
	if !hasValue {
		return appliedFix{Kind: "provide_value", Applied: false, Message: "value parameter is required"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		var valuesMap map[string]any
		if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
			return appliedFix{Kind: "provide_value", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
		}
		valuesMap[path] = rawValue
		newValues, err := json.Marshal(valuesMap)
		if err != nil {
			return appliedFix{Kind: "provide_value", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
		}
		slide.Pattern.Values = newValues
		slide.ShapeGrid = nil
		return appliedFix{Kind: "provide_value", Applied: true, Message: fmt.Sprintf("set %q in pattern values", path)}
	}

	return appliedFix{Kind: "provide_value", Applied: false, Message: "slide has no pattern values to update"}
}

// applyReplaceValue replaces a field value in pattern values with a new value
// supplied by the agent (typically to bring it within valid bounds).
func applyReplaceValue(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	rawValue, hasValue := params["value"]

	if path == "" {
		return appliedFix{Kind: "replace_value", Applied: false, Message: "path parameter is required"}
	}
	if !hasValue {
		return appliedFix{Kind: "replace_value", Applied: false, Message: "value parameter is required"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		var valuesMap map[string]any
		if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
			return appliedFix{Kind: "replace_value", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
		}
		if _, exists := valuesMap[path]; !exists {
			return appliedFix{Kind: "replace_value", Applied: false, Message: fmt.Sprintf("field %q not found in pattern values", path)}
		}
		valuesMap[path] = rawValue
		newValues, err := json.Marshal(valuesMap)
		if err != nil {
			return appliedFix{Kind: "replace_value", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
		}
		slide.Pattern.Values = newValues
		slide.ShapeGrid = nil
		return appliedFix{Kind: "replace_value", Applied: true, Message: fmt.Sprintf("replaced %q in pattern values", path)}
	}

	return appliedFix{Kind: "replace_value", Applied: false, Message: "slide has no pattern values to update"}
}

// applyReduceItems truncates an array field in pattern values to max_items.
func applyReduceItems(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	maxItems := intParam(params, "max_items", 0)

	if path == "" {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: "path parameter is required"}
	}
	if maxItems <= 0 {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: "max_items parameter must be > 0"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: "slide has no pattern values"}
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
	}

	arr, ok := valuesMap[path].([]any)
	if !ok {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: fmt.Sprintf("field %q is not an array", path)}
	}
	if len(arr) <= maxItems {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: fmt.Sprintf("%q already has %d items (max %d)", path, len(arr), maxItems)}
	}

	if refusal, blocked := guardDroppedItems("reduce_items", slideIdx, path, arr[maxItems:], maxItems, params); blocked {
		return refusal
	}

	valuesMap[path] = arr[:maxItems]
	newValues, err := json.Marshal(valuesMap)
	if err != nil {
		return appliedFix{Kind: "reduce_items", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
	}
	slide.Pattern.Values = newValues
	slide.ShapeGrid = nil
	return appliedFix{Kind: "reduce_items", Applied: true, Message: fmt.Sprintf("reduced %q from %d to %d items", path, len(arr), maxItems)}
}

// guardDroppedItems is the fact-loss guard for list-truncating repairs
// (reduce_items, resize_list). It flattens the items that would be dropped and
// refuses the mutation when they carry a protected fact — a number, unit,
// negation, or qualifier (the same losesProtectedFacts vocabulary that guards
// reduce_text / shorten_title / reduce_cell_text). The refusal proposes a
// split_pattern repair that moves the overflow items to a continuation slide
// instead of deleting them. confirm_semantic_change: true bypasses the guard.
func guardDroppedItems(kind string, slideIdx int, path string, dropped []any, keep int, params map[string]any) (appliedFix, bool) {
	if boolParam(params, "confirm_semantic_change", false) {
		return appliedFix{}, false
	}
	var parts []string
	for _, item := range dropped {
		parts = collectItemText(item, parts)
	}
	if !losesProtectedFacts(strings.Join(parts, " "), "") {
		return appliedFix{}, false
	}
	return appliedFix{
		Kind:    kind,
		Applied: false,
		Code:    "semantic_review_required",
		Message: fmt.Sprintf("dropping %d item(s) from %q would remove a number, unit, negation, or qualifier; split the slide instead (split_pattern moves the overflow items to a continuation slide) or pass confirm_semantic_change: true", len(dropped), path),
		NextToolCall: &patterns.ToolCallSuggestion{
			Tool: "repair_slide",
			ArgsTemplate: map[string]any{
				"slide_index": slideIdx,
				"fixes": []any{map[string]any{
					"kind":   "split_pattern",
					"params": map[string]any{"path": path, "first": keep},
				}},
			},
		},
	}, true
}

// collectItemText appends every string leaf of a pattern-values item (string,
// object, or nested array) to parts, in a deterministic key order.
func collectItemText(item any, parts []string) []string {
	switch v := item.(type) {
	case string:
		return append(parts, v)
	case float64:
		return append(parts, strconv.FormatFloat(v, 'f', -1, 64))
	case []any:
		for _, e := range v {
			parts = collectItemText(e, parts)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = collectItemText(v[k], parts)
		}
	}
	return parts
}

// applyAddItems is a placeholder for the add_items fix kind. Since the repair
// tool cannot generate content, the agent must supply the items via the "items"
// param. If not provided, returns applied=false with guidance.
func applyAddItems(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	rawItems, hasItems := params["items"]

	if path == "" {
		return appliedFix{Kind: "add_items", Applied: false, Message: "path parameter is required"}
	}
	if !hasItems {
		return appliedFix{Kind: "add_items", Applied: false, Message: "items parameter is required (array of items to append)"}
	}

	newItems, ok := rawItems.([]any)
	if !ok {
		return appliedFix{Kind: "add_items", Applied: false, Message: "items parameter must be an array"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return appliedFix{Kind: "add_items", Applied: false, Message: "slide has no pattern values"}
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
		return appliedFix{Kind: "add_items", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
	}

	existing, ok := valuesMap[path].([]any)
	if !ok {
		existing = []any{}
	}
	valuesMap[path] = append(existing, newItems...)
	newValues, err := json.Marshal(valuesMap)
	if err != nil {
		return appliedFix{Kind: "add_items", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
	}
	slide.Pattern.Values = newValues
	slide.ShapeGrid = nil
	return appliedFix{Kind: "add_items", Applied: true, Message: fmt.Sprintf("added %d items to %q", len(newItems), path)}
}

// applyResizeList adjusts an array field in pattern values to exactly count items.
// Truncates if too many; returns not-applied if too few (agent must supply items).
func applyResizeList(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	count := intParam(params, "count", 0)

	if path == "" {
		return appliedFix{Kind: "resize_list", Applied: false, Message: "path parameter is required"}
	}
	if count <= 0 {
		return appliedFix{Kind: "resize_list", Applied: false, Message: "count parameter must be > 0"}
	}

	slide := &input.Slides[slideIdx]
	if slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return appliedFix{Kind: "resize_list", Applied: false, Message: "slide has no pattern values"}
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err != nil {
		return appliedFix{Kind: "resize_list", Applied: false, Message: fmt.Sprintf("failed to parse pattern values: %v", err)}
	}

	arr, ok := valuesMap[path].([]any)
	if !ok {
		return appliedFix{Kind: "resize_list", Applied: false, Message: fmt.Sprintf("field %q is not an array", path)}
	}

	if len(arr) == count {
		return appliedFix{Kind: "resize_list", Applied: false, Message: fmt.Sprintf("%q already has exactly %d items", path, count)}
	}

	if len(arr) > count {
		if refusal, blocked := guardDroppedItems("resize_list", slideIdx, path, arr[count:], count, params); blocked {
			return refusal
		}
		valuesMap[path] = arr[:count]
	} else {
		return appliedFix{Kind: "resize_list", Applied: false, Message: fmt.Sprintf("%q has %d items but needs %d; provide additional items via add_items", path, len(arr), count)}
	}

	newValues, err := json.Marshal(valuesMap)
	if err != nil {
		return appliedFix{Kind: "resize_list", Applied: false, Message: fmt.Sprintf("failed to marshal updated values: %v", err)}
	}
	slide.Pattern.Values = newValues
	slide.ShapeGrid = nil
	return appliedFix{Kind: "resize_list", Applied: true, Message: fmt.Sprintf("resized %q from %d to %d items", path, len(arr), count)}
}

// applyRemoveKey removes a key from pattern values or overrides.
func applyRemoveKey(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	key := stringParam(params, "key", "")

	if key == "" {
		return appliedFix{Kind: "remove_key", Applied: false, Message: "key parameter is required"}
	}

	slide := &input.Slides[slideIdx]

	// Try pattern overrides first (cell_overrides keys are typically the target).
	if slide.Pattern != nil && len(slide.Pattern.Overrides) > 0 {
		var overridesMap map[string]json.RawMessage
		if err := json.Unmarshal(slide.Pattern.Overrides, &overridesMap); err == nil {
			if _, exists := overridesMap[key]; exists {
				delete(overridesMap, key)
				newOverrides, err := json.Marshal(overridesMap)
				if err == nil {
					slide.Pattern.Overrides = newOverrides
					slide.ShapeGrid = nil
					return appliedFix{Kind: "remove_key", Applied: true, Message: fmt.Sprintf("removed %q from pattern overrides", key)}
				}
			}
		}
	}

	// Try pattern values.
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		var valuesMap map[string]json.RawMessage
		if err := json.Unmarshal(slide.Pattern.Values, &valuesMap); err == nil {
			if _, exists := valuesMap[key]; exists {
				delete(valuesMap, key)
				newValues, err := json.Marshal(valuesMap)
				if err == nil {
					slide.Pattern.Values = newValues
					slide.ShapeGrid = nil
					return appliedFix{Kind: "remove_key", Applied: true, Message: fmt.Sprintf("removed %q from pattern values", key)}
				}
			}
		}
	}

	return appliedFix{Kind: "remove_key", Applied: false, Message: fmt.Sprintf("key %q not found in pattern values or overrides", key)}
}

// applyRemoveField removes a field from the slide's pattern values.
func applyRemoveField(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")

	if path == "" {
		return appliedFix{Kind: "remove_field", Applied: false, Message: "path parameter is required"}
	}

	slide := &input.Slides[slideIdx]

	// Try pattern values.
	if slide.Pattern != nil && len(slide.Pattern.Values) > 0 {
		if removed, ok := removeJSONKey(slide.Pattern.Values, path); ok {
			slide.Pattern.Values = removed
			slide.ShapeGrid = nil
			return appliedFix{Kind: "remove_field", Applied: true, Message: fmt.Sprintf("removed %q from pattern values", path)}
		}
	}

	// Try slide-level removal via round-trip.
	slideJSON, err := json.Marshal(slide)
	if err != nil {
		return appliedFix{Kind: "remove_field", Applied: false, Message: fmt.Sprintf("failed to marshal slide: %v", err)}
	}
	if removed, ok := removeJSONKey(slideJSON, path); ok {
		if err := json.Unmarshal(removed, slide); err != nil {
			return appliedFix{Kind: "remove_field", Applied: false, Message: fmt.Sprintf("failed to unmarshal slide: %v", err)}
		}
		return appliedFix{Kind: "remove_field", Applied: true, Message: fmt.Sprintf("removed %q from slide", path)}
	}

	return appliedFix{Kind: "remove_field", Applied: false, Message: fmt.Sprintf("field %q not found", path)}
}

// removeJSONKey removes a top-level key from a JSON object. Returns the updated
// JSON and true if the key was found and removed.
func removeJSONKey(raw json.RawMessage, key string) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	if _, ok := obj[key]; !ok {
		return nil, false
	}
	delete(obj, key)
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, false
	}
	return result, true
}

// boolParam extracts a boolean parameter with a default.
func boolParam(params map[string]any, key string, defaultVal bool) bool {
	if params == nil {
		return defaultVal
	}
	if v, ok := params[key].(bool); ok {
		return v
	}
	return defaultVal
}
