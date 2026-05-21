// mcp_auto_repair.go implements the auto_repair MCP tool — a server-side
// generate→inspect→repair convergence loop. Each pass collects fit findings
// (static + render-time), scores the deck deterministically, applies the
// proposed repairs, and re-runs. The loop stops when a configurable gate is
// satisfied or when max_passes is exhausted; the final deck is rendered to
// the output directory either way.
//
// The tool eliminates an entire orchestration burden from agents: instead of
// chaining generate_presentation → score_deck → propose_repairs →
// repair_slides_batch → generate_presentation in a hand-coded loop, the agent
// hands the engine a single call and gets back a trace plus a final PPTX.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// --- Defaults ---

const (
	defaultAutoRepairMinScore                = 75
	defaultAutoRepairMaxP0Findings           = 0
	defaultAutoRepairMaxP1Findings           = 2
	defaultAutoRepairRequireTakeawayOnCharts = true
	defaultAutoRepairMaxPasses               = 3
)

// --- Response types ---

// autoRepairOutput is the top-level response for auto_repair.
type autoRepairOutput struct {
	Path        string                 `json:"path,omitempty"`
	FinalScore  int                    `json:"final_score"`
	GatePassed  bool                   `json:"gate_passed"`
	Passes      int                    `json:"passes"`
	Trace       []autoRepairTraceEntry `json:"trace"`
	GateReasons []string               `json:"gate_reasons,omitempty"`
	// IdempotentReplay is true when this response was served from the
	// idempotency cache instead of regenerated.
	IdempotentReplay bool `json:"idempotent_replay,omitempty"`
}

// autoRepairTraceEntry is one iteration of the loop.
type autoRepairTraceEntry struct {
	Pass           int      `json:"pass"`
	Score          int      `json:"score"`
	FindingsCount  int      `json:"findings_count"`
	RepairsApplied []string `json:"repairs_applied"`
}

// autoRepairGate is the convergence gate config (all fields optional in the
// request; missing fields fall back to the defaults above).
type autoRepairGate struct {
	MinScore                int  `json:"min_score"`
	MaxP0Findings           int  `json:"max_p0_findings"`
	MaxP1Findings           int  `json:"max_p1_findings"`
	RequireTakeawayOnCharts bool `json:"require_takeaway_on_charts"`
}

// --- Tool definition ---

func mcpAutoRepairTool() mcp.Tool {
	return mcp.NewTool("auto_repair",
		mcp.WithDescription(`Run a server-side generate→inspect→repair convergence loop on a deck. Each pass renders the deck, collects fit findings, scores the deck deterministically, and applies the top-ranked repairs from propose_repairs. The loop stops when the configurable gate is satisfied or when max_passes is exhausted.

Replaces the agent's manual chain (generate_presentation → score_deck → propose_repairs → repair_slides_batch → generate_presentation) with a single tool call. The final PPTX is written to the server output directory either way; gate_passed reports whether convergence succeeded.

Gate fields (all optional, all defaulted):
- min_score (default 75): overall_score must be ≥ this value.
- max_p0_findings (default 0): max count of refuse-action findings tolerated.
- max_p1_findings (default 2): max count of shrink_or_split-action findings tolerated.
- require_takeaway_on_charts (default true): no takeaway_missing finding may remain.

Response shape: {path, final_score, gate_passed, passes, trace[], gate_reasons[]}. trace[i] = {pass, score, findings_count, repairs_applied[]} records score progression so the agent can audit convergence behavior.

When gate_passed is false (max_passes exhausted), gate_reasons lists every unmet criterion so the agent can decide whether to call the tool again with relaxed bounds, switch templates, or escalate to human review.`),
		mcp.WithRawOutputSchema(outputSchemaAutoRepair),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition. Same schema as generate_presentation.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithString("template",
			mcp.Description("Template name override. If omitted, uses the template field from the presentation object."),
		),
		mcp.WithObject("gate",
			mcp.Description("Convergence gate configuration. All fields optional; omitted fields fall back to engine defaults (min_score=75, max_p0_findings=0, max_p1_findings=2, require_takeaway_on_charts=true)."),
			mcp.Properties(map[string]any{
				"min_score":                  map[string]any{"type": "integer", "description": "Minimum acceptable overall_score (default 75)."},
				"max_p0_findings":            map[string]any{"type": "integer", "description": "Maximum refuse-action findings tolerated (default 0)."},
				"max_p1_findings":            map[string]any{"type": "integer", "description": "Maximum shrink_or_split-action findings tolerated (default 2)."},
				"require_takeaway_on_charts": map[string]any{"type": "boolean", "description": "Require takeaway on chart/matrix slides (default true)."},
			}),
		),
		mcp.WithNumber("max_passes",
			mcp.Description("Maximum number of generate→inspect→repair iterations (default 3). Clamped to [1, 10]."),
		),
		mcp.WithString("output_filename",
			mcp.Description("Output filename (default: auto_repair.pptx). Path components are stripped for safety."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory used as the root for resolving relative local-asset paths (image_value.path, background.image, shape_grid image/icon paths). Required when any slide references a relative path and the agent cannot guarantee the server CWD matches the JSON's authoring directory. When omitted, the server falls back to its process CWD (not portable). Must be an absolute path to an existing directory. Same contract as generate_presentation."),
		),
		idempotencyKeyToolParam(),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleAutoRepair(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idemKey := idempotencyKey(request)
	if cached, ok := mc.idempotency.Get("auto_repair", idemKey); ok {
		if out, ok := cached.(*autoRepairOutput); ok {
			clone := *out
			clone.IdempotentReplay = true
			return api.MCPSuccessResult(ctx, &clone)
		}
	}

	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("auto_repair", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nextCallGetInputSchema()), nil
	}
	applyDefaults(&input)
	mc.resolveInputNamedSettings(&input)

	// Honor template override (mirrors score_deck).
	if override, err := request.RequireString("template"); err == nil && override != "" {
		input.Template = override
	}

	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Resolve base_dir up front so relative local-asset paths resolve with the
	// same contract as generate_presentation. A malformed base_dir short-circuits
	// before the loop; runAutoRepairLoop rewrites the relative paths to absolute
	// form once (before the first pass) so every render in the loop embeds the
	// same assets generate_presentation would.
	baseDir, baseDirErr := resolveBaseDir(request)
	if baseDirErr != nil {
		return baseDirErr, nil
	}

	gate := extractAutoRepairGate(request)
	maxPasses := extractMaxPasses(request)

	outputFilename := sanitizeOutputFilename(input.OutputFilename)
	if outputFilename == "" {
		outputFilename = "auto_repair.pptx"
	}
	if reqFilename, err := request.RequireString("output_filename"); err == nil && reqFilename != "" {
		outputFilename = sanitizeOutputFilename(reqFilename)
	}

	output, errResult := mc.runAutoRepairLoop(ctx, &input, baseDir, gate, maxPasses, outputFilename)
	if errResult != nil {
		return errResult, nil
	}

	mc.idempotency.Set("auto_repair", idemKey, output)

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// runAutoRepairLoop encapsulates the template-resolution + convergence-loop +
// final-render core of auto_repair. Extracted so make_deck (the cold-start
// facade) can reuse the same loop after building a PresentationInput from a
// plan rather than receiving one directly from the caller.
//
// Returns either a populated autoRepairOutput (errResult=nil) or an MCP error
// result the caller should pass straight through. Callers must wrap the
// successful output in api.MCPSuccessResult themselves so they can attach
// facade-specific fields (e.g. the deck plan summary in make_deck).
func (mc *mcpConfig) runAutoRepairLoop(
	ctx context.Context,
	input *PresentationInput,
	baseDir string,
	gate autoRepairGate,
	maxPasses int,
	outputFilename string,
) (*autoRepairOutput, *mcp.CallToolResult) {
	// Resolve relative local-asset paths (icons, images, background) against
	// base_dir once, before the convergence loop, mirroring generate_presentation
	// / validate_input. Resolution rewrites paths to absolute form in place; the
	// loop's repair edits never touch asset paths and already-absolute paths pass
	// through unchanged, so resolving once up front is correct for every pass.
	// Error-severity findings short-circuit (the caller passes the result
	// straight through), matching handleGenerate's contract.
	if assetFindings := resolveLocalAssetPaths(input.Slides, baseDir); len(assetFindings) > 0 {
		if assetErrors := diagnostics.FilterBySeverity(assetFindings, diagnostics.SeverityError); len(assetErrors) > 0 {
			return nil, api.MCPDiagnosticsError(assetErrors)
		}
	}

	// Resolve template once — reused on every pass.
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_NOT_FOUND", templateNotFoundError(input.Template, mc.templatesDir), nextCallListTemplates())
	}
	defer templateCleanup()

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates())
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates())
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        template.ParseTheme(reader),
	}
	template.SynthesizeIfNeeded(reader, analysis)
	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	templateMetadata, _ := template.ParseMetadata(reader)
	dataPalette := resolveDataPalette(templateMetadata, analysis.Theme.Colors)

	// Resolve canonical layout names (e.g. "title", "blank") to concrete
	// slideLayoutN IDs once, before findings collection and generation. This
	// lets callers (make_deck, agent-authored JSON) ship the portable canonical
	// names without forcing them to hard-code template-specific IDs. Already-
	// resolved IDs pass through unchanged.
	resolveCanonicalLayoutIDs(input.Slides, layouts)

	outputPath := filepath.Join(mc.outputDir, outputFilename)

	// Convergence loop.
	trace := make([]autoRepairTraceEntry, 0, maxPasses)
	var lastFindings []patterns.FitFinding
	var lastScore int
	var lastGateReasons []string
	gatePassed := false
	passesRun := 0

	for pass := 1; pass <= maxPasses; pass++ {
		passesRun = pass

		findings := collectFitFindings(input, layouts, slideWidth, slideHeight, &analysis.Theme)
		renderFindings := mc.collectRenderFindings(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette)
		findings = append(findings, renderFindings...)

		ds := deterministic.ScoreFromFindings(findings, len(input.Slides))
		gateReasons := evaluateAutoRepairGate(ds, findings, gate)

		entry := autoRepairTraceEntry{
			Pass:           pass,
			Score:          ds.OverallScore,
			FindingsCount:  len(findings),
			RepairsApplied: []string{},
		}

		lastFindings = findings
		lastScore = ds.OverallScore
		lastGateReasons = gateReasons

		if len(gateReasons) == 0 {
			gatePassed = true
			trace = append(trace, entry)
			break
		}

		if pass >= maxPasses {
			trace = append(trace, entry)
			break
		}

		proposed := proposeRepairs(input, fitFindingsToProposeFindings(findings))
		applied := applyProposedRepairs(input, proposed)
		entry.RepairsApplied = applied
		trace = append(trace, entry)

		if len(applied) == 0 {
			break
		}
	}

	finalPath, renderErr := mc.renderAutoRepairFinal(ctx, input, templatePath, layouts, slideWidth, slideHeight, syntheticFiles, templateMetadata, dataPalette, outputPath)
	if renderErr != nil {
		return nil, api.MCPSimpleError("GENERATION_FAILED", fmt.Sprintf("final generation failed: %v", renderErr))
	}

	output := &autoRepairOutput{
		Path:        finalPath,
		FinalScore:  lastScore,
		GatePassed:  gatePassed,
		Passes:      passesRun,
		Trace:       trace,
		GateReasons: lastGateReasons,
	}
	if gatePassed {
		output.GateReasons = nil
	}
	_ = lastFindings
	return output, nil
}

// --- Gate evaluation ---

// evaluateAutoRepairGate returns a list of unmet criteria. Empty result means
// the deck satisfies the gate. Order is deterministic: score → P0 → P1 →
// takeaway, so agents can pattern-match on the leading reason.
func evaluateAutoRepairGate(ds *deterministic.DeckScore, findings []patterns.FitFinding, gate autoRepairGate) []string {
	var reasons []string
	if ds.OverallScore < gate.MinScore {
		reasons = append(reasons, fmt.Sprintf("score %d < min_score %d", ds.OverallScore, gate.MinScore))
	}
	p0 := countFindingsByAction(findings, "refuse")
	if p0 > gate.MaxP0Findings {
		reasons = append(reasons, fmt.Sprintf("%d P0 (refuse) findings exceeds max_p0_findings %d", p0, gate.MaxP0Findings))
	}
	p1 := countFindingsByAction(findings, "shrink_or_split")
	if p1 > gate.MaxP1Findings {
		reasons = append(reasons, fmt.Sprintf("%d P1 (shrink_or_split) findings exceeds max_p1_findings %d", p1, gate.MaxP1Findings))
	}
	if gate.RequireTakeawayOnCharts {
		missing := countFindingsByCode(findings, patterns.ErrCodeTakeawayMissing)
		if missing > 0 {
			reasons = append(reasons, fmt.Sprintf("%d chart/matrix slide(s) missing takeaway", missing))
		}
	}
	return reasons
}

func countFindingsByAction(findings []patterns.FitFinding, action string) int {
	n := 0
	for _, f := range findings {
		if f.Action == action {
			n++
		}
	}
	return n
}

func countFindingsByCode(findings []patterns.FitFinding, code string) int {
	n := 0
	for _, f := range findings {
		if f.Code == code {
			n++
		}
	}
	return n
}

// --- Helpers ---

// extractAutoRepairGate reads the gate object from the request, filling in
// every field with the engine default when absent. The default for
// RequireTakeawayOnCharts is true, so we have to distinguish "explicit false"
// from "absent" — that requires a presence check rather than a zero-value
// fallback.
func extractAutoRepairGate(request mcp.CallToolRequest) autoRepairGate {
	gate := autoRepairGate{
		MinScore:                defaultAutoRepairMinScore,
		MaxP0Findings:           defaultAutoRepairMaxP0Findings,
		MaxP1Findings:           defaultAutoRepairMaxP1Findings,
		RequireTakeawayOnCharts: defaultAutoRepairRequireTakeawayOnCharts,
	}
	args := request.GetArguments()
	raw, ok := args["gate"]
	if !ok || raw == nil {
		return gate
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return gate
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return gate
	}
	if v, ok := parsed["min_score"]; ok {
		gate.MinScore = anyToInt(v, gate.MinScore)
	}
	if v, ok := parsed["max_p0_findings"]; ok {
		gate.MaxP0Findings = anyToInt(v, gate.MaxP0Findings)
	}
	if v, ok := parsed["max_p1_findings"]; ok {
		gate.MaxP1Findings = anyToInt(v, gate.MaxP1Findings)
	}
	if v, ok := parsed["require_takeaway_on_charts"]; ok {
		if b, ok := v.(bool); ok {
			gate.RequireTakeawayOnCharts = b
		}
	}
	return gate
}

func anyToInt(v any, fallback int) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return fallback
		}
		return int(i)
	case int:
		return x
	case int64:
		return int(x)
	}
	return fallback
}

// extractMaxPasses reads the max_passes parameter, clamping to [1, 10].
func extractMaxPasses(request mcp.CallToolRequest) int {
	max := defaultAutoRepairMaxPasses
	args := request.GetArguments()
	if raw, ok := args["max_passes"]; ok {
		max = anyToInt(raw, defaultAutoRepairMaxPasses)
	}
	if max < 1 {
		max = 1
	}
	if max > 10 {
		max = 10
	}
	return max
}

// fitFindingsToProposeFindings adapts FitFindings into the polymorphic shape
// proposeRepairs accepts. proposeRepairs's internal logic understands both
// fit-finding and visual-QA shapes; we feed it the fit-finding fields here.
func fitFindingsToProposeFindings(findings []patterns.FitFinding) []proposeRepairsFinding {
	out := make([]proposeRepairsFinding, len(findings))
	for i, f := range findings {
		out[i] = proposeRepairsFinding{
			Pattern: f.Pattern,
			Path:    f.Path,
			Code:    f.Code,
			Message: f.Message,
			Fix:     f.Fix,
			Action:  f.Action,
		}
	}
	return out
}

// applyProposedRepairs walks each proposed slide's directives and applies the
// FIRST applicable repair per slide. Applying every ranked directive on a
// single slide tends to double-up (e.g. two reduce_text calls on the same
// body) without improving the score; the propose_repairs ranking already
// surfaces the best repair first, so taking just the top directive per slide
// is the right convergence step. Returns a human-readable summary of each
// repair that actually landed, suitable for the trace.
func applyProposedRepairs(input *PresentationInput, proposed proposeRepairsOutput) []string {
	var applied []string
	for _, slide := range proposed.Slides {
		for _, dir := range slide.Directives {
			params := adaptAutoRepairParams(input, slide.SlideIndex, dir.Kind, dir.Params)
			result := applyRepairFix(input, slide.SlideIndex, repairFixInput{Kind: dir.Kind, Params: params})
			if result.Applied {
				applied = append(applied, fmt.Sprintf("%s on slide %d", dir.Kind, slide.SlideIndex))
				break
			}
		}
	}
	return applied
}

// adaptAutoRepairParams translates fit-finding fix params (which describe the
// PROBLEM — e.g. current_words=120, max_words=80) into repair_slide params
// (which describe the ACTION — e.g. max_items=4). The agent normally bridges
// this vocabulary mismatch by hand; auto_repair removes the agent from the
// loop, so it must do the translation itself.
//
// Today this only matters for reduce_text against BODY_TOO_LONG findings, but
// the function is structured so additional kind→params adapters can be added
// without changing the call site.
func adaptAutoRepairParams(input *PresentationInput, slideIdx int, kind string, params map[string]any) map[string]any {
	if kind != "reduce_text" || params == nil {
		return params
	}
	if hasActionableReduceTextParam(params) {
		return params
	}
	maxWords := anyToInt(params["max_words"], 0)
	if maxWords <= 0 {
		return params
	}
	if slideIdx < 0 || slideIdx >= len(input.Slides) {
		return params
	}

	body := pickBodyContentForReduce(&input.Slides[slideIdx])
	if body == nil {
		return params
	}
	return reduceTextParamsForBody(params, body, maxWords)
}

// hasActionableReduceTextParam returns true when params already carry a
// repair_slide-actionable directive (max_items or max_length), in which case
// we trust the caller's intent and skip translation.
func hasActionableReduceTextParam(params map[string]any) bool {
	if _, ok := params["max_items"]; ok {
		return true
	}
	if _, ok := params["max_length"]; ok {
		return true
	}
	return false
}

// pickBodyContentForReduce finds the content item that BODY_TOO_LONG most
// likely refers to. Preference order: a "body*" placeholder with reducible
// content; then any non-title content with bullets; then any non-title text.
func pickBodyContentForReduce(slide *SlideInput) *ContentInput {
	for i := range slide.Content {
		ci := &slide.Content[i]
		if !strings.HasPrefix(ci.PlaceholderID, "body") {
			continue
		}
		if isReducibleContent(ci) {
			return ci
		}
	}
	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID == "title" {
			continue
		}
		if ci.BulletsValue != nil || ci.BodyAndBulletsValue != nil {
			return ci
		}
	}
	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID == "title" {
			continue
		}
		if ci.TextValue != nil {
			return ci
		}
	}
	return nil
}

func isReducibleContent(ci *ContentInput) bool {
	return ci.BulletsValue != nil || ci.BodyAndBulletsValue != nil || ci.TextValue != nil
}

// reduceTextParamsForBody computes the {max_items} or {max_length} that
// brings the body's word count within maxWords. Returns the original params
// unchanged when no truncation is needed.
func reduceTextParamsForBody(params map[string]any, body *ContentInput, maxWords int) map[string]any {
	out := cloneStringMap(params)
	switch {
	case body.BulletsValue != nil:
		return assignBulletReduce(out, *body.BulletsValue, maxWords, params)
	case body.BodyAndBulletsValue != nil:
		return assignBulletReduce(out, body.BodyAndBulletsValue.Bullets, maxWords, params)
	case body.TextValue != nil:
		out["max_length"] = maxWords * 6
		return out
	}
	return params
}

// assignBulletReduce walks bullets and sets max_items / max_length on out
// according to where the running word total first crosses maxWords. Returns
// originalParams when every bullet fits — no repair needed.
func assignBulletReduce(out map[string]any, bullets []string, maxWords int, originalParams map[string]any) map[string]any {
	words := 0
	for i, b := range bullets {
		w := len(strings.Fields(b))
		if words+w > maxWords {
			if i == 0 {
				// Single bullet already over budget; truncate by char count
				// instead. ~6 chars per word is a workable approximation
				// (English averages ~5 chars/word + space).
				out["max_length"] = maxWords * 6
				return out
			}
			out["max_items"] = i
			return out
		}
		words += w
	}
	return originalParams
}

func cloneStringMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// renderAutoRepairFinal renders the converged (or best-effort) deck to the
// configured output path. Mirrors collectRenderFindings's pipeline assembly
// but writes to a stable location instead of a temp dir.
func (mc *mcpConfig) renderAutoRepairFinal(
	ctx context.Context,
	input *PresentationInput,
	templatePath string,
	layouts []types.LayoutMetadata,
	slideWidth, slideHeight int64,
	syntheticFiles map[string][]byte,
	templateMetadata *types.TemplateMetadata,
	dataPalette []string,
	outputPath string,
) (string, error) {
	var rhythmGrid *resolvedGrid
	if input.Grid != nil {
		rhythmGrid = resolveGrid(input.Grid, layouts, slideWidth, slideHeight)
	}

	slideSpecs, _, _, err := convertPresentationSlides(input.Slides, layouts, slideWidth, slideHeight, templateMetadata, rhythmGrid, patterns.AccentStrategy(input.AccentStrategy), nil, false)
	if err != nil {
		return "", fmt.Errorf("convert slides: %w", err)
	}

	genReq := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		SVGStrategy:           string(mc.cfg.SVG.Strategy),
		SVGScale:              mc.cfg.SVG.Scale,
		SVGNativeCompat:       string(mc.cfg.SVG.NativeCompatibility),
		MaxPNGWidth:           mc.cfg.SVG.MaxPNGWidth,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
		StrictFit:             "warn",
		DataPalette:           dataPalette,
	}
	if input.Chrome != nil {
		genReq.Footer = chromeToFooterConfig(input.Chrome, len(slideSpecs))
		applyChromeSkip(slideSpecs, input.Chrome, input.Slides, layouts)
	} else if input.Footer != nil && input.Footer.Enabled {
		genReq.Footer = &generator.FooterConfig{
			Enabled:  true,
			LeftText: input.Footer.LeftText,
		}
	}
	if input.ThemeOverride != nil {
		genReq.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}

	if _, err := generator.Generate(ctx, genReq); err != nil {
		return "", err
	}
	return outputPath, nil
}
