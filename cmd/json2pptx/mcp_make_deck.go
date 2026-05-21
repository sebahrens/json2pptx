// mcp_make_deck.go implements the make_deck MCP tool — a one-call cold-start
// facade that chains plan_deck → expand patterns with exemplar content →
// auto_repair (generate → inspect → repair) until the quality gate passes or
// max_repair_passes is exhausted.
//
// Replaces the chain {plan_deck → recommend_visual → expand_pattern × N →
// generate_presentation → score_deck → propose_repairs → repair_slides_batch →
// generate_presentation} with a single tool call. The intent is to give
// cold-start agents (Claude Code / Codex / Amp) a hard-to-misuse entry point
// that drops them onto a publishable deck without orchestration overhead.
//
// The full 37-tool surface remains available for power users who want to
// drive each step manually.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

const (
	defaultMakeDeckSlideBudget = 10
	defaultMakeDeckTemplate    = "midnight-blue"
)

// makeDeckOutput is the top-level response for make_deck. The first six fields
// mirror autoRepairOutput so agents that already consume auto_repair can reuse
// the same parsing logic; Plan adds the deck outline make_deck computed
// internally so agents can see which patterns were chosen on each slide and
// chain to repair_slide for per-slide content edits.
type makeDeckOutput struct {
	Path        string                 `json:"path,omitempty"`
	FinalScore  int                    `json:"final_score"`
	GatePassed  bool                   `json:"gate_passed"`
	Passes      int                    `json:"passes"`
	Trace       []autoRepairTraceEntry `json:"trace"`
	GateReasons []string               `json:"gate_reasons,omitempty"`
	// QualityMode truth-labels the inspection regime: "deterministic" (default)
	// or "deterministic+visual_qa" when visual_qa.enabled was passed through.
	// Mirrors auto_repair.quality_mode.
	QualityMode string `json:"quality_mode"`
	// VisualQA is present only when visual_qa mode was requested. Same shape as
	// auto_repair.visual_qa.
	VisualQA *visualQAResult      `json:"visual_qa,omitempty"`
	Plan     *makeDeckPlanSummary `json:"plan"`
	// FinalPresentation is the full deck JSON produced after planning and
	// repair. Always present on success. Lets agents continue editing the
	// auto-authored deck (per-slide content via repair_slide, re-validation via
	// validate_input, re-render via generate_presentation) without
	// reconstructing it from the plan summary or trace.
	FinalPresentation json.RawMessage `json:"final_presentation"`
	// IdempotentReplay is true when this response was served from the
	// idempotency cache instead of regenerated.
	IdempotentReplay bool `json:"idempotent_replay,omitempty"`
}

// makeDeckPlanSummary captures the plan_deck decisions that produced the
// deck. Per-slide entries let the agent target individual slides for follow-up
// edits without re-running plan_deck.
type makeDeckPlanSummary struct {
	Template    string              `json:"template"`
	SlideBudget int                 `json:"slide_budget"`
	Slides      []makeDeckPlanSlide `json:"slides"`
}

// makeDeckPlanSlide is the per-slide snapshot of the planner's choice.
type makeDeckPlanSlide struct {
	SlideIndex         int    `json:"slide_index"`
	NarrativeRole      string `json:"narrative_role"`
	RecommendedPattern string `json:"recommended_pattern"`
	Title              string `json:"title"`
}

// makeDeckStyleHints carries the optional shaping knobs the agent can pass to
// influence pattern selection and visual rhythm. Every field is optional.
type makeDeckStyleHints struct {
	SlideBudget    int      `json:"slide_budget"`
	Audience       string   `json:"audience"`
	AccentStrategy string   `json:"accent_strategy"`
	MustInclude    []string `json:"must_include"`
}

// --- Tool definition ---

func mcpMakeDeckTool() mcp.Tool {
	return mcp.NewTool("make_deck",
		mcp.WithDescription(`Cold-start facade: ONE call from a natural-language outline to a validated PPTX. Internally chains plan_deck → expand patterns with exemplar content → auto_repair (generate → inspect → repair) until the quality gate passes or max_repair_passes is exhausted.

Use this when you want a publishable deck without orchestrating the 37-tool surface yourself. The full surface remains available for fine-grained control; make_deck is intended as the recommended starting point for cold-start agents.

Quality mode (truth-labeled in the response as quality_mode): like auto_repair, the DEFAULT is "deterministic" — the internal loop scores the deck from static + render-fit findings only, with no rendering and no API key. Pass visual_qa.enabled=true to additionally run the opt-in vision/heuristic visual refinement phase (quality_mode "deterministic+visual_qa"); it inherits auto_repair's visual_qa semantics, requirements, and transparent fallbacks.

Returns {path, final_score, gate_passed, passes, trace[], gate_reasons[], quality_mode, plan, final_presentation, visual_qa?}. The final PPTX is written to the configured output directory whether the gate passed or not. plan.slides[] lets the caller target individual slides via repair_slide for follow-up content edits without re-planning. final_presentation is the full deck JSON the engine authored and repaired (reflects any visual_qa repairs) — feed it straight back into validate_input / generate_presentation / repair_slide to keep editing without rebuilding it from the plan or trace.

Style hints (all optional):
- slide_budget: target deck size, clamped to [3, 30] (default 10).
- audience: free-form audience descriptor (e.g. "board of directors").
- accent_strategy: "primary" (default), "rotate", or "section-keyed".
- must_include: pattern names that MUST appear in the plan.

Quality gate matches auto_repair semantics: same field names, same defaults. Omit it to use the engine defaults (min_score=75, max_p0_findings=0, max_p1_findings=2, require_takeaway_on_charts=true).`),
		mcp.WithRawOutputSchema(outputSchemaMakeDeck),
		mcp.WithString("outline",
			mcp.Required(),
			mcp.Description(`Natural-language brief describing the deck purpose and content (e.g., "Pitch our Series B for an AI infra company"). Used as the brief for plan_deck — the planner derives slide-level narrative roles and pattern recommendations from it.`),
		),
		mcp.WithString("template",
			mcp.Description("Template name (default: midnight-blue). Use list_templates to see options."),
		),
		mcp.WithObject("style_hints",
			mcp.Description("Optional shaping knobs for plan_deck. See tool description for field semantics."),
			mcp.Properties(map[string]any{
				"slide_budget":    map[string]any{"type": "integer", "description": "Target slide count, clamped to [3, 30] (default 10)."},
				"audience":        map[string]any{"type": "string", "description": "Free-form audience descriptor."},
				"accent_strategy": map[string]any{"type": "string", "enum": []string{"primary", "rotate", "section-keyed"}, "description": "Accent rotation strategy."},
				"must_include":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Pattern names that must appear in the plan."},
			}),
		),
		mcp.WithNumber("max_repair_passes",
			mcp.Description("Maximum number of auto_repair iterations (default 3). Clamped to [1, 10]."),
		),
		mcp.WithObject("visual_qa",
			mcp.Description("Opt-in visual-QA mode, inherited from auto_repair. When enabled=true, runs a vision/heuristic visual refinement phase after the deterministic loop. Disabled by default. Same shape and semantics as auto_repair.visual_qa (enabled, model, audit_palette, max_passes, density); see auto_repair for requirements and cost."),
			mcp.Properties(map[string]any{
				"enabled":       map[string]any{"type": "boolean", "description": "Enable the visual-QA phase (default false)."},
				"model":         map[string]any{"type": "string", "description": "Claude vision model override (default claude-haiku-4-5-20251001)."},
				"audit_palette": map[string]any{"type": "boolean", "description": "Also run the deterministic palette ΔE audit on the final deck (default false)."},
				"max_passes":    map[string]any{"type": "integer", "description": "Max visual render→inspect→repair iterations (default 1). Clamped to [1, 3]."},
				"density":       map[string]any{"type": "integer", "description": "Thumbnail DPI for inspection (default 50). Clamped to [25, 150]."},
			}),
		),
		mcp.WithObject("gate",
			mcp.Description("Quality gate configuration. Same shape as auto_repair.gate. Omit to use engine defaults."),
			mcp.Properties(map[string]any{
				"min_score":                  map[string]any{"type": "integer"},
				"max_p0_findings":            map[string]any{"type": "integer"},
				"max_p1_findings":            map[string]any{"type": "integer"},
				"require_takeaway_on_charts": map[string]any{"type": "boolean"},
			}),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: make_deck.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used as the root for resolving relative local-asset paths in any exemplar content that references local files (image_value.path, background.image, shape_grid image/icon paths). When omitted, the server falls back to its process CWD (not portable). Must be an absolute path to an existing directory. Same contract as generate_presentation."),
		),
		idempotencyKeyToolParam(),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleMakeDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idemKey := idempotencyKey(request)
	if cached, ok := mc.idempotency.Get("make_deck", idemKey); ok {
		if out, ok := cached.(*makeDeckOutput); ok {
			clone := *out
			clone.IdempotentReplay = true
			return api.MCPSuccessResult(ctx, &clone)
		}
	}

	outline, err := request.RequireString("outline")
	if err != nil || outline == "" {
		return argMissing("make_deck", "outline", "string", "Pitch our Series B for an AI infra company", nil), nil
	}

	templateName := defaultMakeDeckTemplate
	if t, terr := request.RequireString("template"); terr == nil && t != "" {
		templateName = t
	}

	hints, hintsErr := extractMakeDeckStyleHints(request)
	if hintsErr != nil {
		return hintsErr, nil
	}

	reg := patterns.Default()
	for _, name := range hints.MustInclude {
		if _, ok := reg.Get(name); !ok {
			return argInvalidValue(
				"make_deck",
				"INVALID_PARAMETER",
				"style_hints.must_include",
				fmt.Sprintf("pattern %q not found; use list_patterns to see available patterns", name),
				"array",
				[]any{"kpi-3up"},
				nextCallListPatterns(),
			), nil
		}
	}

	// Phase 1: plan the deck. make_deck builds its slides from each pattern's
	// exemplar content and never surfaces plan_deck's template_support, so it
	// plans template-agnostically (nil context) — the template is applied during
	// expansion and the auto_repair loop.
	plan := deckplan.BuildDeckPlan(reg, deckplan.Params{
		Brief:       outline,
		SlideBudget: hints.SlideBudget,
		Audience:    hints.Audience,
		MustInclude: hints.MustInclude,
	}, planPredictor{reg: reg})

	// Phase 2: expand each planned slide with exemplar content. The cold-start
	// agent supplies no per-slide content, so we use each pattern's canonical
	// ExemplarValues — these are the same realistic placeholders plan_deck
	// uses for findings prediction. Slide titles are derived from the brief
	// and narrative role so the resulting deck reads as a coherent outline
	// rather than N copies of the exemplar's title.
	input := buildPresentationFromPlan(reg, plan, templateName, outline, hints.AccentStrategy)

	// Phase 3: run the auto_repair convergence loop. make_deck and auto_repair
	// share the same gate vocabulary so callers can graduate from one to the
	// other without re-learning the schema. Resolve base_dir up front so the
	// shared loop resolves any exemplar-supplied relative asset paths with the
	// same contract as generate_presentation.
	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return baseDirErr, nil
	}

	gate := extractAutoRepairGate(request)
	maxPasses := extractMakeDeckMaxPasses(request)
	vqa := extractVisualQAConfig(request)

	outputFilename := "make_deck.pptx"
	if reqFilename, ferr := request.RequireString("output_filename"); ferr == nil && reqFilename != "" {
		outputFilename = sanitizeOutputFilename(reqFilename)
	}

	loopOut, errResult := mc.runAutoRepairLoop(ctx, input, baseDir, gate, maxPasses, vqa, outputFilename)
	if errResult != nil {
		return errResult, nil
	}

	out := &makeDeckOutput{
		Path:              loopOut.Path,
		FinalScore:        loopOut.FinalScore,
		GatePassed:        loopOut.GatePassed,
		Passes:            loopOut.Passes,
		Trace:             loopOut.Trace,
		GateReasons:       loopOut.GateReasons,
		QualityMode:       loopOut.QualityMode,
		VisualQA:          loopOut.VisualQA,
		Plan:              planSummaryFromInput(plan, input, templateName),
		FinalPresentation: loopOut.FinalPresentation,
	}

	mc.idempotency.Set("make_deck", idemKey, out)

	mcpResult, err := api.MCPSuccessResult(ctx, out)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Parameter extraction ---

// extractMakeDeckStyleHints parses the style_hints object, applying defaults
// and clamping numeric values. Returns an MCP error result when the value is
// present but malformed (e.g. not an object). When style_hints is absent,
// returns the engine defaults.
func extractMakeDeckStyleHints(request mcp.CallToolRequest) (makeDeckStyleHints, *mcp.CallToolResult) {
	hints := makeDeckStyleHints{SlideBudget: defaultMakeDeckSlideBudget}
	raw, ok := request.GetArguments()["style_hints"]
	if !ok || raw == nil {
		return hints, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return hints, argInvalidJSON("style_hints", fmt.Sprintf("invalid style_hints: %v", err), "object", nil, nil)
	}
	var parsed makeDeckStyleHints
	if err := json.Unmarshal(data, &parsed); err != nil {
		return hints, argInvalidJSON("style_hints", fmt.Sprintf("invalid style_hints: %v", err), "object", nil, nil)
	}
	if parsed.SlideBudget > 0 {
		hints.SlideBudget = parsed.SlideBudget
	}
	hints.Audience = parsed.Audience
	hints.AccentStrategy = parsed.AccentStrategy
	hints.MustInclude = parsed.MustInclude

	if hints.SlideBudget < 3 {
		hints.SlideBudget = 3
	}
	if hints.SlideBudget > 30 {
		hints.SlideBudget = 30
	}
	return hints, nil
}

// extractMakeDeckMaxPasses reads max_repair_passes (preferred) and falls back
// to max_passes for callers that use the auto_repair argument name. Clamped
// to [1, 10].
func extractMakeDeckMaxPasses(request mcp.CallToolRequest) int {
	args := request.GetArguments()
	if raw, ok := args["max_repair_passes"]; ok {
		return clampMaxPasses(anyToInt(raw, defaultAutoRepairMaxPasses))
	}
	if raw, ok := args["max_passes"]; ok {
		return clampMaxPasses(anyToInt(raw, defaultAutoRepairMaxPasses))
	}
	return defaultAutoRepairMaxPasses
}

func clampMaxPasses(n int) int {
	if n < 1 {
		return 1
	}
	if n > 10 {
		return 10
	}
	return n
}

// --- Plan-to-input construction ---

// buildPresentationFromPlan converts a deck plan into a PresentationInput
// where each slide carries the recommended pattern populated with its
// ExemplarValues. The cold-start agent has supplied no per-slide content, so
// exemplar values are the canonical fallback — the same ones plan_deck's
// findings predictor uses, which means the resulting deck reflects the
// plan_deck's predicted_findings output verbatim.
//
// Patterns lacking an Exemplar implementation degrade to a title-only slide
// using the layout that matches the narrative role. This keeps the deck shape
// stable even when individual pattern slots cannot be auto-filled.
func buildPresentationFromPlan(reg *patterns.Registry, plan *deckplan.Result, templateName, outline, accentStrategy string) *PresentationInput {
	slides := make([]SlideInput, 0, len(plan.Slides))
	for _, ps := range plan.Slides {
		title := titleForPlannedSlide(ps, outline)
		slide := SlideInput{
			LayoutID: layoutIDForNarrativeRole(ps.NarrativeRole),
			Content:  []ContentInput{makeTitleContent(title)},
		}

		if patternInput := buildPatternInputForSlide(reg, ps.RecommendedPattern); patternInput != nil {
			slide.Pattern = patternInput
		}
		slides = append(slides, slide)
	}
	return &PresentationInput{
		Template:       templateName,
		AccentStrategy: accentStrategy,
		Slides:         slides,
	}
}

// buildPatternInputForSlide constructs a PatternInput for the named pattern
// using its ExemplarValues. Returns nil when the pattern is unknown or has no
// exemplar — caller should produce a title-only slide in that case.
func buildPatternInputForSlide(reg *patterns.Registry, name string) *PatternInput {
	pat, ok := reg.Get(name)
	if !ok {
		return nil
	}
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		return nil
	}
	values := ex.ExemplarValues()
	if values == nil {
		return nil
	}
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return &PatternInput{
		Name:   name,
		Values: valuesJSON,
	}
}

// layoutIDForNarrativeRole is a local mirror of patterns.layoutIDForNarrativeRole
// so make_deck can drive layout selection without exporting an internal helper.
// Canonical layout IDs (title, blank, content, section, closing) resolve at
// generate time against the template's layouts.
func layoutIDForNarrativeRole(role string) string {
	switch role {
	case "opening":
		return "title"
	case "closing":
		return "closing"
	case "framework":
		return "section"
	default:
		return "blank"
	}
}

// titleForPlannedSlide derives a slide title from the plan slot. The opening
// slot gets the brief itself (truncated) so the deck names what it's about;
// every other slot draws from the planner's content_seed so the title at
// least describes the slot's purpose. Empty seeds fall back to a positional
// label so we never emit empty titles (which would trigger output validation).
func titleForPlannedSlide(ps deckplan.Slide, outline string) string {
	const titleCap = 60
	if ps.NarrativeRole == "opening" && outline != "" {
		return deckplan.TruncateBrief(outline, titleCap)
	}
	if ps.ContentSeed != "" {
		return deckplan.TruncateBrief(ps.ContentSeed, titleCap)
	}
	return fmt.Sprintf("Slide %d", ps.SlideIndex+1)
}

// makeTitleContent builds the canonical title ContentInput for a slide. Title
// placeholder ID is portable across every bundled template.
func makeTitleContent(text string) ContentInput {
	t := text
	return ContentInput{
		PlaceholderID: "title",
		Type:          "text",
		TextValue:     &t,
	}
}

// planSummaryFromInput packages the planner's decisions into the agent-visible
// plan summary returned in makeDeckOutput. Reads back from the final input so
// titles reflect what was actually rendered (after any auto_repair edits).
func planSummaryFromInput(plan *deckplan.Result, input *PresentationInput, templateName string) *makeDeckPlanSummary {
	summary := &makeDeckPlanSummary{
		Template:    templateName,
		SlideBudget: plan.SlideBudget,
		Slides:      make([]makeDeckPlanSlide, 0, len(plan.Slides)),
	}
	for i, ps := range plan.Slides {
		title := ""
		if i < len(input.Slides) {
			title = extractTitleFromSlide(&input.Slides[i])
		}
		summary.Slides = append(summary.Slides, makeDeckPlanSlide{
			SlideIndex:         ps.SlideIndex,
			NarrativeRole:      ps.NarrativeRole,
			RecommendedPattern: ps.RecommendedPattern,
			Title:              title,
		})
	}
	return summary
}

// extractTitleFromSlide pulls the title text value from a slide for the plan
// summary. Returns an empty string when no title content is present.
func extractTitleFromSlide(slide *SlideInput) string {
	for _, c := range slide.Content {
		if c.PlaceholderID != "title" {
			continue
		}
		if c.TextValue != nil {
			return *c.TextValue
		}
	}
	return ""
}
