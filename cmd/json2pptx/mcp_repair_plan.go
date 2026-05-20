// mcp_repair_plan.go implements the propose_repairs MCP tool — translates
// structured findings (fit_report findings, visual QA findings, or validation
// diagnostics) into a ranked list of fix directives consumable by repair_slide.
//
// Unlike repair_slide, this tool does NOT mutate the deck. It only proposes
// directives, grouped by slide index and ranked by severity/action. Agents can
// inspect the proposed directives, optionally filter them, and then submit the
// chosen ones to repair_slide.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// --- Response types ---

// proposeRepairsOutput is the top-level response for propose_repairs.
type proposeRepairsOutput struct {
	Slides   []proposedSlideRepairs `json:"slides"`
	Unmapped []unmappedFinding      `json:"unmapped,omitempty"`
	Summary  proposeSummary         `json:"summary"`
}

// proposedSlideRepairs groups ranked fix directives for one slide.
type proposedSlideRepairs struct {
	SlideIndex    int                  `json:"slide_index"`
	FindingCount  int                  `json:"finding_count"`
	Directives    []proposedDirective  `json:"directives"`
	BatchToolCall *batchRepairToolCall `json:"batch_tool_call,omitempty"`
}

// batchRepairToolCall is a single ready-to-invoke repair_slide call carrying
// every directive proposed for one slide. Agents can use this to submit the
// whole batch in one round-trip; per-directive tool_call values remain
// available for the more conservative "apply one at a time" workflow.
type batchRepairToolCall struct {
	Tool         string         `json:"tool"`
	ArgsTemplate map[string]any `json:"args_template"`
}

// proposedDirective is a single ranked fix directive, with provenance.
type proposedDirective struct {
	Kind     string         `json:"kind"`
	Params   map[string]any `json:"params,omitempty"`
	Rank     int            `json:"rank"`
	Source   directiveSource `json:"source"`
	ToolCall *patterns.ToolCallSuggestion `json:"tool_call,omitempty"`
}

// directiveSource is the provenance of a directive: which finding produced it,
// what the underlying severity/action was, and where on the slide the issue
// was localized.
type directiveSource struct {
	// Type is "fit" (FitFinding) or "visual" (visualqa.Finding).
	Type string `json:"type"`

	// Code is the fit-finding error code (empty for visual findings).
	Code string `json:"code,omitempty"`

	// Category is the visual QA category (empty for fit findings).
	Category string `json:"category,omitempty"`

	// Severity is a normalized severity label: "error", "warning", "info",
	// or visual QA severities "P0"|"P1"|"P2"|"P3".
	Severity string `json:"severity,omitempty"`

	// Action is the fit-finding action ("refuse", "shrink_or_split", "review",
	// "info"). Empty for visual findings.
	Action string `json:"action,omitempty"`

	// Path is the JSON path the finding pointed at (e.g. "/slides/0/...").
	Path string `json:"path,omitempty"`

	// Message is the original human-readable finding message.
	Message string `json:"message,omitempty"`
}

// unmappedFinding describes a finding that could not be translated into any
// repair_slide directive (review-only categories, unknown shapes, etc.).
type unmappedFinding struct {
	Reason     string `json:"reason"`
	Code       string `json:"code,omitempty"`
	Category   string `json:"category,omitempty"`
	SlideIndex *int   `json:"slide_index,omitempty"`
	Path       string `json:"path,omitempty"`
	Message    string `json:"message,omitempty"`
}

// proposeSummary aggregates counts so agents can dashboard the result.
type proposeSummary struct {
	TotalFindings    int `json:"total_findings"`
	MappedFindings   int `json:"mapped_findings"`
	UnmappedFindings int `json:"unmapped_findings"`
	TotalDirectives  int `json:"total_directives"`
	SlidesAffected   int `json:"slides_affected"`
}

// --- Input parsing ---

// proposeRepairsFinding is the polymorphic shape accepted from callers. It
// covers both FitFinding and visualqa.Finding fields. Unknown fields are
// tolerated (no DisallowUnknownFields) because agents may pass mildly
// extended shapes.
type proposeRepairsFinding struct {
	// Fit-finding shape (embedded ValidationError + extras).
	Pattern  string                  `json:"pattern,omitempty"`
	Path     string                  `json:"path,omitempty"`
	Code     string                  `json:"code,omitempty"`
	Message  string                  `json:"message,omitempty"`
	Fix      *patterns.FixSuggestion `json:"fix,omitempty"`
	Action   string                  `json:"action,omitempty"`

	// Visual QA finding shape.
	SlideIndex     *int                    `json:"slide_index,omitempty"`
	SlideType      string                  `json:"slide_type,omitempty"`
	Severity       string                  `json:"severity,omitempty"`
	Category       string                  `json:"category,omitempty"`
	Description    string                  `json:"description,omitempty"`
	Location       string                  `json:"location,omitempty"`
	SuggestedFixes []visualqa.SuggestedFix `json:"suggested_fixes,omitempty"`
}

// --- Tool definition ---

func mcpProposeRepairsTool() mcp.Tool {
	return mcp.NewTool("propose_repairs",
		mcp.WithDescription(`Translate structured findings (fit_report findings, visual QA findings, or validation diagnostics) into a ranked list of repair_slide fix directives. Returns proposed directives grouped by slide and ordered by severity — does NOT mutate the deck.

Accepts two finding shapes (polymorphic, mixed input is fine):
- Fit findings: {path, code, message, action, fix:{kind,params}, slide_index?} — emitted by generate_presentation(fit_report=true), validate_input, repair_slide.new_findings.
- Visual QA findings: {slide_index, slide_type, severity, category, suggested_fixes:[{kind,params}], description, location} — emitted by inspect_slide_images.

For each finding the tool:
1. Resolves the target slide (from finding.slide_index, finding.path /slides/N, or fix.params.path).
2. Selects candidate fix kinds: finding.fix (fit) > finding.suggested_fixes (visual) > visualqa category mapping > unmapped.
3. Augments each candidate with a tool_call pointing at repair_slide.

Output:
- slides[]: per-slide directives sorted by severity (error|P0 > warning|P1 > info|P2 > P3), then by action rank (refuse > shrink_or_split > review > info).
- unmapped[]: findings with no mapping (review-only visual QA categories like image_quality, aspect_ratio, border_style; or findings without fix info).
- summary: counts (total_findings, mapped_findings, unmapped_findings, total_directives, slides_affected).

Each directive carries {kind, params, rank, source:{type,code|category,severity,action,path,message}, tool_call}. Agents can submit the directive's tool_call directly, or batch directives for one slide using the per-slide batch_tool_call.`),
		mcp.WithRawOutputSchema(outputSchemaProposeRepairs),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition. Same schema as generate_presentation / repair_slide.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithArray("findings",
			mcp.Required(),
			mcp.Description(`Array of structured findings (fit findings and/or visual QA findings, in any mix). See tool description for shapes.`),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleProposeRepairs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("propose_repairs", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}
	applyDefaults(&input)
	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	findings, ferr := extractProposeFindings(request)
	if ferr != nil {
		return argInvalidValue("propose_repairs", "INVALID_PARAMETER", "findings", ferr.Error(), "array", []any{map[string]any{"code": "BODY_TOO_LONG", "slide_index": 0}}, nil), nil
	}
	if len(findings) == 0 {
		return argMissing("propose_repairs", "findings", "array", []any{map[string]any{"code": "BODY_TOO_LONG", "slide_index": 0, "path": "slides[0].content.body"}}, nil), nil
	}

	output := proposeRepairs(&input, findings)

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Core logic ---

// proposeRepairs is the pure-function core: given a parsed deck and a list of
// findings, return ranked directives grouped by slide. No I/O, no mutation.
//
// Each emitted tool_call (per-directive and per-slide batch) embeds the
// full presentation in its args_template so the call is directly submittable
// to repair_slide — without this slot the agent gets MISSING_PARAMETER.
func proposeRepairs(input *PresentationInput, findings []proposeRepairsFinding) proposeRepairsOutput {
	slideCount := len(input.Slides)

	// Marshal the presentation once into a generic map so each emitted
	// tool_call / batch_tool_call can carry the full repair_slide argument
	// payload (presentation + slide_index + fixes). Without this slot, agents
	// that submit the tool_call verbatim get rejected by repair_slide with
	// MISSING_PARAMETER "presentation is required" — the contract bug this
	// function exists to prevent.
	presentationObj := presentationAsMap(input)

	// Bucket directives by slide index; track unmapped findings separately.
	type bucketEntry struct {
		directive proposedDirective
		score     int // higher = more urgent
	}
	buckets := make(map[int][]bucketEntry)
	findingCounts := make(map[int]int)
	var unmapped []unmappedFinding
	mappedFindings := 0

	for _, f := range findings {
		slideIdx, ok := resolveSlideIndex(f, slideCount)
		isVisual := f.Category != ""

		// Visual QA finding path: prefer caller-supplied suggested_fixes, else
		// fall back to the canonical category→fix mapping.
		if isVisual {
			if !ok {
				unmapped = append(unmapped, unmappedFinding{
					Reason:   "missing_slide_index",
					Category: f.Category,
					Message:  f.Description,
				})
				continue
			}
			candidates := f.SuggestedFixes
			if len(candidates) == 0 {
				candidates = visualqa.SuggestedFixesForCategory(f.Category)
			}
			if len(candidates) == 0 {
				idx := slideIdx
				unmapped = append(unmapped, unmappedFinding{
					Reason:     reasonForVisualCategory(f.Category),
					Category:   f.Category,
					SlideIndex: &idx,
					Message:    f.Description,
				})
				continue
			}

			findingCounts[slideIdx]++
			mappedFindings++
			score := scoreFinding(f, "")
			source := directiveSource{
				Type:     "visual",
				Category: f.Category,
				Severity: normalizeSeverity(f.Severity),
				Path:     buildVisualPath(slideIdx),
				Message:  f.Description,
			}
			for ci, cand := range candidates {
				dir := buildDirective(cand.Kind, cand.Params, slideIdx, source, presentationObj, score-ci /* preserve candidate order within a finding */)
				buckets[slideIdx] = append(buckets[slideIdx], bucketEntry{directive: dir, score: score - ci})
			}
			continue
		}

		// Fit-finding path.
		if !ok {
			unmapped = append(unmapped, unmappedFinding{
				Reason:  "missing_slide_index",
				Code:    f.Code,
				Path:    f.Path,
				Message: f.Message,
			})
			continue
		}

		fix := f.Fix
		if fix == nil || fix.Kind == "" {
			idx := slideIdx
			unmapped = append(unmapped, unmappedFinding{
				Reason:     "no_fix_attached",
				Code:       f.Code,
				SlideIndex: &idx,
				Path:       f.Path,
				Message:    f.Message,
			})
			continue
		}

		// Reject fix kinds that don't map to repair_slide (e.g. "adopt_pattern",
		// "text" placeholder). Those need recommend_pattern, not repair_slide.
		kind := fix.Kind
		if !isRepairFixKind(kind) {
			idx := slideIdx
			unmapped = append(unmapped, unmappedFinding{
				Reason:     fmt.Sprintf("fix_kind_not_repairable:%s", kind),
				Code:       f.Code,
				SlideIndex: &idx,
				Path:       f.Path,
				Message:    f.Message,
			})
			continue
		}

		findingCounts[slideIdx]++
		mappedFindings++
		score := scoreFinding(f, fix.Kind)
		source := directiveSource{
			Type:     "fit",
			Code:     f.Code,
			Action:   f.Action,
			Severity: severityFromAction(f.Action),
			Path:     f.Path,
			Message:  f.Message,
		}
		dir := buildDirective(fix.Kind, cloneParams(fix.Params), slideIdx, source, presentationObj, score)
		buckets[slideIdx] = append(buckets[slideIdx], bucketEntry{directive: dir, score: score})
	}

	// Materialize buckets into the sorted slides array.
	slideIndices := make([]int, 0, len(buckets))
	for idx := range buckets {
		slideIndices = append(slideIndices, idx)
	}
	sort.Ints(slideIndices)

	slides := make([]proposedSlideRepairs, 0, len(slideIndices))
	totalDirectives := 0
	for _, slideIdx := range slideIndices {
		entries := buckets[slideIdx]
		// Sort: higher score first; stable on tie.
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].score > entries[j].score })

		directives := make([]proposedDirective, len(entries))
		batchFixes := make([]any, len(entries))
		for i, e := range entries {
			e.directive.Rank = i
			directives[i] = e.directive
			fixObj := map[string]any{"kind": e.directive.Kind}
			if len(e.directive.Params) > 0 {
				fixObj["params"] = e.directive.Params
			}
			batchFixes[i] = fixObj
		}

		batch := &batchRepairToolCall{
			Tool: "repair_slide",
			ArgsTemplate: map[string]any{
				"presentation": presentationObj,
				"slide_index":  slideIdx,
				"fixes":        batchFixes,
			},
		}

		slides = append(slides, proposedSlideRepairs{
			SlideIndex:    slideIdx,
			FindingCount:  findingCounts[slideIdx],
			Directives:    directives,
			BatchToolCall: batch,
		})
		totalDirectives += len(directives)
	}

	return proposeRepairsOutput{
		Slides:   slides,
		Unmapped: unmapped,
		Summary: proposeSummary{
			TotalFindings:    len(findings),
			MappedFindings:   mappedFindings,
			UnmappedFindings: len(unmapped),
			TotalDirectives:  totalDirectives,
			SlidesAffected:   len(slides),
		},
	}
}

// --- Helpers ---

// resolveSlideIndex resolves a finding's target slide index from (in order):
//  1. explicit slide_index (visualqa shape)
//  2. /slides/N prefix on path (fit-finding shape)
//  3. /slides/N prefix on fix.params.path
//
// Returns ok=false when no resolvable index is available, or when the resolved
// index is out of range for the deck.
func resolveSlideIndex(f proposeRepairsFinding, slideCount int) (int, bool) {
	if f.SlideIndex != nil {
		idx := *f.SlideIndex
		if idx >= 0 && idx < slideCount {
			return idx, true
		}
		return 0, false
	}
	if f.Path != "" {
		if idx := slidepath.SlideIndex(f.Path); idx >= 0 && idx < slideCount {
			return idx, true
		}
	}
	if f.Fix != nil {
		if p, ok := f.Fix.Params["path"].(string); ok && p != "" {
			if idx := slidepath.SlideIndex(p); idx >= 0 && idx < slideCount {
				return idx, true
			}
		}
		// reduce_cell_text uses cell_path instead of path.
		if p, ok := f.Fix.Params["cell_path"].(string); ok && p != "" {
			if idx := slidepath.SlideIndex(p); idx >= 0 && idx < slideCount {
				return idx, true
			}
		}
	}
	return 0, false
}

// scoreFinding produces an integer priority score so we can sort directives
// from most-urgent to least-urgent. Higher = more urgent.
//
// Tiers (base):
//   1000 — refuse / P0
//    800 — shrink_or_split / P1 / error
//    400 — review / P2 / warning
//    100 — info / P3
//
// A small bonus is added when the original finding already carried a fix
// (preferred over visualqa category fallback).
func scoreFinding(f proposeRepairsFinding, fixKind string) int {
	var base int

	switch f.Action {
	case "refuse":
		base = 1000
	case "shrink_or_split":
		base = 800
	case "review":
		base = 400
	case "info":
		base = 100
	}

	if base == 0 {
		switch normalizeSeverity(f.Severity) {
		case "P0", "error":
			base = 1000
		case "P1", "warning", "warn":
			base = 800
		case "P2":
			base = 400
		case "P3", "info":
			base = 100
		default:
			base = 200
		}
	}

	// Bonus when caller already attached a fix directive — these are more
	// trustworthy than category-mapped fallbacks.
	if fixKind != "" {
		base += 5
	}

	return base
}

// normalizeSeverity maps free-form severity strings to a small canonical set.
// Visual QA severities (P0..P3) pass through; diagnostic severities normalize
// to "error"/"warning"/"info".
func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "p0":
		return "P0"
	case "p1":
		return "P1"
	case "p2":
		return "P2"
	case "p3":
		return "P3"
	case "error":
		return "error"
	case "warning", "warn":
		return "warning"
	case "info":
		return "info"
	case "":
		return ""
	default:
		return s
	}
}

// severityFromAction maps a fit-finding action to a normalized severity label,
// so directives sourced from fit findings expose the same severity vocabulary
// downstream agents already use.
func severityFromAction(action string) string {
	switch action {
	case "refuse":
		return "error"
	case "shrink_or_split":
		return "warning"
	case "review":
		return "info"
	case "info":
		return "info"
	}
	return ""
}

// reasonForVisualCategory returns a stable reason code for an unmapped visual
// QA finding so agents can branch deterministically.
func reasonForVisualCategory(category string) string {
	if visualqa.IsReviewOnly(category) {
		return "review_only_category"
	}
	if !visualqa.ValidCategory(category) {
		return "unknown_category"
	}
	return "no_fix_for_category"
}

// buildVisualPath constructs a slide-scoped path for visual QA findings that
// otherwise have no path (they only carry slide_index).
func buildVisualPath(slideIdx int) string {
	return slidepath.Slide(slideIdx)
}

// isRepairFixKind reports whether kind is in the set of fix kinds that
// repair_slide actually accepts. Single source of truth is repairFixKinds()
// in mcp_capabilities.go.
func isRepairFixKind(kind string) bool {
	for _, k := range repairFixKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// cloneParams makes a shallow copy of a params map so callers can't mutate
// the directive after the fact. Returns nil for empty input so the omitempty
// JSON tag drops the field rather than emitting "{}".
func cloneParams(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// buildDirective wires up a proposedDirective with its repair_slide tool call.
// The presentation map is embedded directly so the args_template is a complete
// repair_slide invocation — agents can submit it without having to thread the
// deck through manually.
func buildDirective(kind string, params map[string]any, slideIdx int, source directiveSource, presentation map[string]any, _ int) proposedDirective {
	fixObj := map[string]any{"kind": kind}
	if len(params) > 0 {
		fixObj["params"] = params
	}
	return proposedDirective{
		Kind:   kind,
		Params: params,
		Source: source,
		ToolCall: &patterns.ToolCallSuggestion{
			Tool: "repair_slide",
			ArgsTemplate: map[string]any{
				"presentation": presentation,
				"slide_index":  slideIdx,
				"fixes":        []any{fixObj},
			},
		},
	}
}

// presentationAsMap converts a parsed PresentationInput back into a generic
// map[string]any so it can be embedded inside repair_slide tool_call args.
// Returns an empty map on marshal failure (which is impossible in practice
// for a valid PresentationInput) so callers don't have to nil-check.
func presentationAsMap(input *PresentationInput) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// extractProposeFindings reads the "findings" array argument and decodes it
// into the polymorphic shape.
func extractProposeFindings(request mcp.CallToolRequest) ([]proposeRepairsFinding, error) {
	args := request.GetArguments()
	raw, ok := args["findings"]
	if !ok {
		return nil, fmt.Errorf("findings is required")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("findings: %w", err)
	}
	var findings []proposeRepairsFinding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, fmt.Errorf("findings must be an array of finding objects: %w", err)
	}
	return findings, nil
}
