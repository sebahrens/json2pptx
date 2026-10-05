package main

import (
	"context"
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
		mcp.WithDescription(`Plan a deck from a brief — an ordered slide outline with narrative roles (opening, evidence, comparison, close), a canonical layout per slide (opening "title" and closing "closing" with no pattern; content slides "blank-title" plus a pattern) and content seeds. Rhythm: no 3 consecutive slides with the same pattern; emphasis (stat-hero, pull-quote) only for the brief's own headline number or quote.

format:"deckspec" (recommended) returns deck_spec — a DeckSpec draft whose kinds follow the brief — plus slots[] (path, guidance, facts), unplaced_facts[], constraints[] ("8 slides", "on midnight-blue") and budget / budget_note. A sentence that names a slide in the kind catalogue's own words is drafted as that kind, in brief order, with its facts in the kind's fields (bridge, matrix_2x2, image_case, architecture, team, decision / option_matrix, roadmap, process, table, comparison, kpi_snapshot, chart_insight, regions); the narrative defaults are drafted from what is left, and "Ask:" / next steps is the one closer. The cover carries the deck name the brief gives, else __FILL__ — never a cut fragment; no dividers under 12 slides unless asked. Fill the rest from list_slide_kinds, then validate_deck_spec → render_deck_spec.

The plan's slides[] are advisory records, NOT SlideInput objects. For a raw deck, copy each slides[i].skeleton into presentation.slides[], replace its __FILL__ tokens and validate before generating.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaPlanDeck)),
		mcp.WithString("brief",
			mcp.Required(),
			mcp.Description("Natural-language description of the deck purpose and content (e.g., 'Pitch our Series B for an AI infra company')."),
		),
		mcp.WithNumber("slide_budget",
			mcp.Description("Target number of slides (3–30). Default: the count the brief states, else 10."),
		),
		mcp.WithString("audience",
			mcp.Description("Target audience (e.g., 'board of directors', 'engineering team', 'investors'). Influences pattern selection."),
		),
		mcp.WithArray("must_include",
			mcp.Description("Pattern names that must appear in the plan (e.g., [\"bmc-canvas\", \"kpi-3up\"])."),
		),
		mcp.WithString("format",
			mcp.Description("\"raw\" (default): pattern outline with raw SlideInput skeletons for the raw path. \"deckspec\": a DeckSpec draft for validate_deck_spec / render_deck_spec — the slides the brief names, by kind and in brief order, then the narrative slots drafted from what is left."),
			mcp.Enum("raw", deckplan.FormatDeckSpec),
		),
		mcp.WithString("template",
			mcp.Description("Template name. Every planned slide then carries template_support {status: supported|risky|unsupported, reasons[], required_layout}, and a pattern the template cannot host is replaced with a supported alternative."),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handlePlanDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	brief, err := request.RequireString("brief")
	if err != nil {
		return argRequired(request, "plan_deck", "brief", "string", "Pitch our Q3 product launch to the executive team", nil), nil
	}

	// Without slide_budget the plan takes the slide count the brief states
	// ("8 slides"), else the default of 10.
	slideBudget, budgetExplicit := 10, false
	if sb, ok := request.GetArguments()["slide_budget"]; ok {
		if f, ok := sb.(float64); ok {
			slideBudget, budgetExplicit = int(f), true
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
	if bad := decodeOptionalArg(request, "plan_deck", "must_include", &mustInclude, "array of pattern names", []string{"kpi-3up"}); bad != nil {
		return bad, nil
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
			Brief:          brief,
			SlideBudget:    slideBudget,
			BudgetExplicit: budgetExplicit,
			Audience:       audience,
			TemplateName:   templateName,
		}))
	}

	result := deckplan.BuildDeckPlan(reg, deckplan.Params{
		Brief:          brief,
		SlideBudget:    slideBudget,
		BudgetExplicit: budgetExplicit,
		Audience:       audience,
		MustInclude:    mustInclude,
		TemplateCtx:    tc,
		TemplateName:   templateName,
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
