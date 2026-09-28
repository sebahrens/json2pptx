package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// --- Tool definition ---

func mcpPlanDeckTool() mcp.Tool {
	return mcp.NewTool("plan_deck",
		mcp.WithDescription(`Plan a deck from a brief — an ordered slide outline with narrative roles (opening, evidence, comparison, close), a canonical layout per slide (opening "title" and closing "closing" with no pattern; content slides "blank-title" plus a pattern) and content seeds. The plan enforces deck rhythm: no 3 consecutive slides with the same pattern, an emphasis slide (stat-hero, pull-quote, kpi-inline) about every ~5 slides capped at ceil(n/5), two-sided comparison patterns for comparison slots, and accent rotation.

format:"deckspec" (recommended) returns deck_spec: a DeckSpec draft following a storyline (cover, answer, problem, cause, evidence, plan, roadmap, ask, closing) with kinds and __FILL__ action titles, plus slots[] and unplaced_facts; fill it from list_slide_kinds, then validate_deck_spec → render_deck_spec.

Facts in the brief (numbers such as "+23% revenue", named entities such as "EU expansion") are routed verbatim into slide seeds and facts[], quantities to KPI / stat / chart slides first; facts with no room are listed in unplaced_facts.

The plan's slides[] are advisory records, NOT SlideInput objects. For a DeckSpec deck, use the plan to choose kinds and order. For a raw deck, copy each slides[i].skeleton (when present) into presentation.slides[], replace its __FILL__ tokens with real content and validate before generating; a slide without a skeleton is authored from its recommended pattern's schema.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaPlanDeck)),
		mcp.WithString("brief",
			mcp.Required(),
			mcp.Description("Natural-language description of the deck purpose and content (e.g., 'Pitch our Series B for an AI infra company')."),
		),
		mcp.WithNumber("slide_budget",
			mcp.Description("Target number of slides (default: 10, range: 3–30)."),
		),
		mcp.WithString("audience",
			mcp.Description("Target audience (e.g., 'board of directors', 'engineering team', 'investors'). Influences pattern selection."),
		),
		mcp.WithArray("must_include",
			mcp.Description("Pattern names that must appear in the plan (e.g., [\"bmc-canvas\", \"kpi-3up\"])."),
		),
		mcp.WithString("format",
			mcp.Description("\"raw\" (default): pattern outline with raw SlideInput skeletons for the raw path. \"deckspec\": a DeckSpec draft with narrative slots (answer, problem, cause, evidence, plan, roadmap, ask) for validate_deck_spec / render_deck_spec."),
			mcp.Enum("raw", deckplan.FormatDeckSpec),
		),
		mcp.WithString("template",
			mcp.Description("Optional template name (e.g., midnight-blue) to make the plan template-aware. When supplied, every planned slide (and each alternative) carries a template_support object {status: supported|risky|unsupported, reasons[], required_layout} grounded in the template's canonical layouts, derivable layouts, font-aware placeholder capacities, and palette — the same helper recommend_visual uses. A pattern the template cannot host is replaced with a supported alternative when one exists. Use list_templates to discover names."),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handlePlanDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	brief, err := request.RequireString("brief")
	if err != nil {
		return argRequired(request, "plan_deck", "brief", "string", "Pitch our Q3 product launch to the executive team", nil), nil
	}

	slideBudget := 10
	if sb, ok := request.GetArguments()["slide_budget"]; ok {
		if f, ok := sb.(float64); ok {
			slideBudget = int(f)
		}
	}
	if slideBudget < 3 {
		slideBudget = 3
	}
	if slideBudget > 30 {
		slideBudget = 30
	}

	audience := ""
	if a, ok := request.GetArguments()["audience"]; ok {
		if s, ok := a.(string); ok {
			audience = s
		}
	}

	var mustInclude []string
	if miRaw, ok := request.GetArguments()["must_include"]; ok && miRaw != nil {
		miJSON, err := json.Marshal(miRaw)
		if err == nil {
			_ = json.Unmarshal(miJSON, &mustInclude)
		}
	}

	format := "raw"
	if f, ok := request.GetArguments()["format"].(string); ok && f != "" {
		format = f
	}
	if format != "raw" && format != deckplan.FormatDeckSpec {
		return argInvalidValue("plan_deck", "INVALID_PARAMETER", "format", fmt.Sprintf("format must be \"raw\" or %q, got %q", deckplan.FormatDeckSpec, format), "string", deckplan.FormatDeckSpec, nextCallRetry("plan_deck", "format")), nil
	}

	// Validate must_include patterns exist.
	reg := patterns.Default()
	for _, name := range mustInclude {
		if _, ok := reg.Get(name); !ok {
			return argInvalidValue("plan_deck", "INVALID_PARAMETER", "must_include", fmt.Sprintf("must_include pattern %q not found; use list_patterns to see available patterns", name), "array", []any{"kpi-3up"}, nextCallListPatterns()), nil
		}
	}

	// Optional template context — when supplied, the plan becomes template-aware:
	// every slide (and alternative) is annotated with template_support and any
	// recommended pattern the template cannot host is swapped for a supported one.
	analysis, cleanup, errResult := mc.resolveTemplateAnalysis(request)
	if errResult != nil {
		return errResult, nil
	}
	if cleanup != nil {
		defer cleanup()
	}
	var tc *generator.TemplateSupportContext
	templateName := ""
	if analysis != nil {
		tc = generator.NewTemplateSupportContext(analysis, reg)
		if t, ok := request.GetArguments()["template"].(string); ok {
			templateName = t
		}
	}

	if format == deckplan.FormatDeckSpec {
		if analysis == nil {
			templateName = ""
		}
		return planDeckSpecResult(ctx, deckplan.BuildDeckSpecPlan(deckplan.Params{
			Brief:        brief,
			SlideBudget:  slideBudget,
			Audience:     audience,
			TemplateName: templateName,
		}))
	}

	result := deckplan.BuildDeckPlan(reg, deckplan.Params{
		Brief:        brief,
		SlideBudget:  slideBudget,
		Audience:     audience,
		MustInclude:  mustInclude,
		TemplateCtx:  tc,
		TemplateName: templateName,
	}, planPredictor{reg: reg})

	if err := api.ComputeResponseFingerprint(result); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}
	return mcpResult, nil
}

// planDeckSpecResult fingerprints and wraps a format:"deckspec" plan.
func planDeckSpecResult(ctx context.Context, plan *deckplan.DeckSpecPlan) (*mcp.CallToolResult, error) {
	if err := api.ComputeResponseFingerprint(plan); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}
	mcpResult, err := api.MCPSuccessResult(ctx, plan)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}
	return mcpResult, nil
}

// --- Prediction (cell budgets, fit findings) ---

// maxPredictedFindings caps the predicted_findings list emitted per slide.
const maxPredictedFindings = 3

// planPredictor adapts package main's render-coupled forecasters to the
// deckplan.Predictor interface. The fit-finding predictor needs the full
// PresentationInput → collectFitFindings machinery, which lives in package
// main, so the planning core in internal/deckplan reaches it through this seam.
type planPredictor struct {
	reg *patterns.Registry
}

func (p planPredictor) CellBudgets(name string) []deckplan.CellBudget {
	return predictCellBudgetsForPattern(p.reg, name)
}

func (p planPredictor) FitFindings(name string, slideIdx int) []deckplan.Finding {
	return predictFitFindingsForPattern(p.reg, name, slideIdx)
}

// predictCellBudgetsForPattern returns the per-configuration cell budgets the
// renderer would impose on the pattern. Uses the existing text_budget_guide
// computation; returns nil for non-grid patterns.
func predictCellBudgetsForPattern(reg *patterns.Registry, name string) []deckplan.CellBudget {
	pat, ok := reg.Get(name)
	if !ok {
		return nil
	}
	guide := computeTextBudgetGuide(pat)
	if guide == nil || len(guide.Configurations) == 0 {
		return nil
	}
	out := make([]deckplan.CellBudget, 0, len(guide.Configurations))
	for _, c := range guide.Configurations {
		out = append(out, deckplan.CellBudget{
			Columns:        c.Columns,
			Rows:           c.Rows,
			BodyMaxChars:   c.BodyMaxChars,
			HeaderMaxChars: c.HeaderMaxChars,
		})
	}
	return out
}

// predictFitFindingsForPattern expands a pattern with its declared exemplar
// values and runs the full fit-finding collector against a synthetic
// PresentationInput. Returns the top-ranked findings (up to maxPredictedFindings).
//
// No template/theme are available at planning time, so structural checks that
// need a layout (placeholder overflow, footer collision) are skipped and only
// shape-grid-resident detectors fire (text overflow, sparse layout, pattern
// occupancy, table preflight). That is intentional — the plan path must not
// render.
func predictFitFindingsForPattern(reg *patterns.Registry, name string, slideIdx int) []deckplan.Finding {
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

	// Default 16:9 slide bounds, matching computeTextBudgetGuide so budgets
	// and findings are derived from the same canonical geometry.
	const (
		slideWidth  int64 = 9144000
		slideHeight int64 = 5143500
	)
	expandCtx := patterns.ExpandContext{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
		LayoutBounds: patterns.LayoutBounds{
			X:      457200,
			Y:      457200,
			Width:  8229600,
			Height: 4229100,
		},
	}

	grid, err := pat.Expand(expandCtx, values, nil, nil)
	if err != nil || grid == nil {
		return nil
	}
	patterns.ApplyGridDefaults(grid)

	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: grid,
			Pattern:   &PatternInput{Name: name},
		}},
	}

	findings := collectFitFindings(input, nil, slideWidth, slideHeight, nil)
	if len(findings) == 0 {
		return nil
	}

	limit := maxPredictedFindings
	if len(findings) < limit {
		limit = len(findings)
	}
	out := make([]deckplan.Finding, 0, limit)
	for i := 0; i < limit; i++ {
		f := findings[i]
		nextTool := ""
		if f.NextToolCall != nil {
			nextTool = f.NextToolCall.Tool
		}
		out = append(out, deckplan.Finding{
			Code:         f.Code,
			Path:         f.Path,
			Message:      f.Message,
			Action:       f.Action,
			NextToolCall: nextTool,
		})
	}
	return out
}
