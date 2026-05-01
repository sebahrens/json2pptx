package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// --- Response types ---

// previewPlanOutput is the top-level response for preview_presentation_plan.
type previewPlanOutput struct {
	ResolvedSlides []resolvedSlide      `json:"resolved_slides"`
	Warnings       []string             `json:"warnings,omitempty"`
	Errors         []string             `json:"errors,omitempty"`
	FitFindings    []patterns.FitFinding `json:"fit_findings,omitempty"`
}

// resolvedSlide describes the fully resolved plan for one slide.
type resolvedSlide struct {
	SlideIndex          int                   `json:"slide_index"`
	LayoutID            string                `json:"layout_id"`
	LayoutIDSource      string                `json:"layout_id_source"` // "inline", "auto", "fallback"
	LayoutName          string                `json:"layout_name,omitempty"`
	SlideType           string                `json:"slide_type,omitempty"`
	Placeholders        []resolvedPlaceholder `json:"placeholders"`
	ExpandedPattern     *resolvedPattern      `json:"expanded_pattern,omitempty"`
	ShapeGridResolution *resolvedShapeGrid    `json:"shape_grid_resolution,omitempty"`
	AppliedDefaults     *resolvedDefaults     `json:"applied_defaults,omitempty"`
}

// resolvedPlaceholder describes one content→placeholder mapping after resolution.
type resolvedPlaceholder struct {
	InputID    string        `json:"input_id"`           // Original placeholder_id from input
	ResolvedID string        `json:"resolved_id"`        // Actual placeholder ID after virtual mapping
	Remapped   bool          `json:"remapped,omitempty"` // True if input_id != resolved_id
	Type       string        `json:"type"`               // Content type
	Geometry   *resolvedGeom `json:"geometry,omitempty"`  // Placeholder bounds from template
}

// resolvedGeom holds placeholder geometry in EMUs.
type resolvedGeom struct {
	X      int64 `json:"x"`
	Y      int64 `json:"y"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

// resolvedPattern describes a pattern expansion result.
type resolvedPattern struct {
	Name                string `json:"name"`
	CellsAfterExpansion int    `json:"cells_after_expansion"`
}

// resolvedShapeGrid describes virtual layout resolution for shape_grid slides.
type resolvedShapeGrid struct {
	VirtualLayoutUsed bool          `json:"virtual_layout_used"`
	LayoutID          string        `json:"layout_id,omitempty"`
	Geometry          *resolvedGeom `json:"geometry,omitempty"`
}

// resolvedDefaults reports which deck-level defaults were applied to this slide.
type resolvedDefaults struct {
	TableStyle bool `json:"table_style,omitempty"`
	CellStyle  bool `json:"cell_style,omitempty"`
}

// --- Tool definition ---

func mcpPreviewPlanTool() mcp.Tool {
	return mcp.NewTool("preview_presentation_plan",
		mcp.WithDescription(`Resolve the full generation plan without rendering a PPTX. Returns per-slide layout selection, placeholder mapping, pattern expansion, and shape_grid resolution — everything the engine decides before rendering.

Use this to preview what generate_presentation will do: which layout each slide gets, how virtual placeholders (title, body, slot1) resolve to actual IDs, what geometry each placeholder has, and what fit findings exist. Fix issues in the plan before paying a full generation round-trip.`),
		mcp.WithString("json_input",
			mcp.Description(`JSON string containing the presentation definition. Mutually exclusive with "presentation" (object form). Same format as generate_presentation.`),
		),
		mcp.WithObject("presentation",
			mcp.Description(`Structured object form of the presentation definition. Mutually exclusive with "json_input" (string form). Same schema as generate_presentation.`),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, include fit_findings in the response. Default: true."),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit. Default: false."),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handlePreviewPlan(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, ambigErr := resolveStringOrObject(request, "json_input", "presentation")
	if ambigErr != nil {
		return ambigErr, nil
	}
	if jsonStr == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "json_input or presentation is required"), nil
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "json_input", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	// Apply deck-level defaults before resolution.
	applyDefaults(&input)

	// Boundary validation.
	if errResult := validatePreviewBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Resolve template.
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", templateNotFoundError(input.Template, mc.templatesDir)), nil
	}
	defer templateCleanup()

	tctx, err := loadPreviewTemplate(templatePath)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err)), nil
	}
	defer func() { _ = tctx.reader.Close() }()

	// Resolve all slides.
	output := resolvePreviewSlides(&input, tctx)

	// Fit findings (default true for preview).
	fitReport := true
	if v, ok := request.GetArguments()["fit_report"].(bool); ok {
		fitReport = v
	}
	if fitReport {
		verboseFit, _ := request.GetArguments()["verbose_fit"].(bool)
		output.FitFindings = computePreviewFitFindings(&input, &output, tctx, verboseFit)
	}

	// Collect boundary warnings.
	for _, w := range checkInputUnknownKeys([]byte(jsonStr)) {
		output.Warnings = append(output.Warnings, w.Error())
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Helpers ---

// validatePreviewBoundary checks required fields and returns an error result or nil.
func validatePreviewBoundary(input *PresentationInput) *mcp.CallToolResult {
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

// previewTemplateContext holds resolved template data for the preview handler.
type previewTemplateContext struct {
	reader       *template.Reader
	layouts      []types.LayoutMetadata
	layoutByID   map[string]types.LayoutMetadata
	metadata     *types.TemplateMetadata
	slideWidth   int64
	slideHeight  int64
}

// loadPreviewTemplate opens and analyzes a template for the preview tool.
func loadPreviewTemplate(templatePath string) (*previewTemplateContext, error) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, err
	}

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
	}
	template.SynthesizeIfNeeded(reader, analysis)

	layoutByID := make(map[string]types.LayoutMetadata, len(analysis.Layouts))
	for _, l := range analysis.Layouts {
		layoutByID[l.ID] = l
	}

	metadata, _ := template.ParseMetadata(reader)

	return &previewTemplateContext{
		reader:      reader,
		layouts:     analysis.Layouts,
		layoutByID:  layoutByID,
		metadata:    metadata,
		slideWidth:  slideWidth,
		slideHeight: slideHeight,
	}, nil
}

// resolvePreviewSlides resolves all slides in the input against the template.
func resolvePreviewSlides(input *PresentationInput, tctx *previewTemplateContext) previewPlanOutput {
	output := previewPlanOutput{
		ResolvedSlides: make([]resolvedSlide, 0, len(input.Slides)),
	}

	var usedLayouts map[string]int
	if len(tctx.layouts) > 0 {
		usedLayouts = make(map[string]int)
	}

	for i := range input.Slides {
		rs := resolveOneSlide(i, &input.Slides[i], input, tctx, usedLayouts, &output)
		output.ResolvedSlides = append(output.ResolvedSlides, rs)
	}

	return output
}

// resolveOneSlide resolves layout, placeholders, patterns, and shape_grid for a single slide.
func resolveOneSlide(i int, slide *SlideInput, input *PresentationInput, tctx *previewTemplateContext, usedLayouts map[string]int, output *previewPlanOutput) resolvedSlide { //nolint:gocognit,gocyclo
	rs := resolvedSlide{
		SlideIndex:   i,
		Placeholders: []resolvedPlaceholder{},
		SlideType:    string(inferSlideType(*slide)),
	}

	// Report applied defaults.
	if input.Defaults != nil {
		ad := &resolvedDefaults{}
		if input.Defaults.TableStyle != nil {
			ad.TableStyle = true
		}
		if input.Defaults.CellStyle != nil {
			ad.CellStyle = true
		}
		if ad.TableStyle || ad.CellStyle {
			rs.AppliedDefaults = ad
		}
	}

	// Layout resolution.
	resolveSlideLayout(i, slide, input, tctx, usedLayouts, output, &rs)

	// Placeholder resolution.
	resolveSlidePlaceholders(slide, tctx, &rs)

	// Pattern expansion.
	if slide.Pattern != nil && slide.ShapeGrid == nil {
		resolveSlidePattern(i, slide, tctx, output, &rs)
	}

	// Shape grid virtual layout resolution.
	if slide.ShapeGrid != nil && len(tctx.layouts) > 0 {
		resolveSlideShapeGrid(slide, tctx, &rs)
	}

	return rs
}

// resolveSlideLayout resolves the layout_id for a slide (inline, auto, or fallback).
func resolveSlideLayout(i int, slide *SlideInput, input *PresentationInput, tctx *previewTemplateContext, usedLayouts map[string]int, output *previewPlanOutput, rs *resolvedSlide) {
	if slide.LayoutID == "" {
		if len(tctx.layouts) == 0 {
			output.Errors = append(output.Errors,
				fmt.Sprintf("slide %d: layout_id is required (no template layouts available)", i+1))
			rs.LayoutIDSource = "fallback"
			return
		}

		// Auto-select layout.
		slideDef := jsonSlideToDefinition(*slide)
		req := layout.SelectionRequest{
			Slide:   slideDef,
			Layouts: tctx.layouts,
			Context: layout.SelectionContext{
				Position:    i,
				TotalSlides: len(input.Slides),
				UsedLayouts: usedLayouts,
			},
		}
		if i > 0 && len(output.ResolvedSlides) > 0 {
			req.Context.PreviousType = output.ResolvedSlides[i-1].LayoutID
		}

		result, err := layout.SelectLayout(req)
		if err != nil {
			output.Errors = append(output.Errors,
				fmt.Sprintf("slide %d: auto-layout selection failed: %v", i+1, err))
			rs.LayoutIDSource = "fallback"
			return
		}

		rs.LayoutID = result.LayoutID
		rs.LayoutIDSource = "auto"
		usedLayouts[result.LayoutID]++

		slog.Info("preview: auto-layout selected",
			slog.Int("slide", i+1),
			slog.String("layout_id", result.LayoutID),
			slog.Float64("confidence", result.Confidence),
		)
	} else {
		rs.LayoutID = slide.LayoutID
		rs.LayoutIDSource = "inline"
		if usedLayouts != nil {
			usedLayouts[slide.LayoutID]++
		}
	}

	// Resolve layout name.
	if lm, ok := tctx.layoutByID[rs.LayoutID]; ok {
		rs.LayoutName = lm.Name
	}
}

// resolveSlidePlaceholders resolves virtual placeholder IDs and attaches geometry.
func resolveSlidePlaceholders(slide *SlideInput, tctx *previewTemplateContext, rs *resolvedSlide) {
	resolvedContent := make([]ContentInput, len(slide.Content))
	copy(resolvedContent, slide.Content)

	if rs.LayoutID != "" {
		if selectedLayout, ok := tctx.layoutByID[rs.LayoutID]; ok {
			if slide.LayoutID == "" || hasVirtualPlaceholders(slide.Content) {
				resolvedContent = autoMapPlaceholders(resolvedContent, selectedLayout)
			}
		}
	}

	for j, ci := range resolvedContent {
		rp := resolvedPlaceholder{
			InputID:    slide.Content[j].PlaceholderID,
			ResolvedID: ci.PlaceholderID,
			Remapped:   slide.Content[j].PlaceholderID != ci.PlaceholderID,
			Type:       ci.Type,
		}

		// Attach geometry from template layout.
		if lm, ok := tctx.layoutByID[rs.LayoutID]; ok {
			for _, ph := range lm.Placeholders {
				if ph.ID == ci.PlaceholderID {
					rp.Geometry = boundingBoxToGeom(ph.Bounds)
					break
				}
			}
		}

		rs.Placeholders = append(rs.Placeholders, rp)
	}
}

// resolveSlidePattern expands a pattern and updates the slide's shape_grid.
func resolveSlidePattern(i int, slide *SlideInput, tctx *previewTemplateContext, output *previewPlanOutput, rs *resolvedSlide) {
	expCtx := patterns.ExpandContext{
		Metadata:    tctx.metadata,
		SlideWidth:  tctx.slideWidth,
		SlideHeight: tctx.slideHeight,
	}
	expanded, expandWarnings, err := expandPattern(slide.Pattern, expCtx, patterns.Default())
	if err != nil {
		output.Errors = append(output.Errors,
			fmt.Sprintf("slide %d: pattern %q: %v", i+1, slide.Pattern.Name, err))
		return
	}
	cellCount := 0
	for _, row := range expanded.Rows {
		cellCount += len(row.Cells)
	}
	rs.ExpandedPattern = &resolvedPattern{
		Name:                slide.Pattern.Name,
		CellsAfterExpansion: cellCount,
	}
	output.Warnings = append(output.Warnings, expandWarnings...)
	slide.ShapeGrid = expanded
}

// resolveSlideShapeGrid resolves virtual layout for shape_grid slides.
func resolveSlideShapeGrid(slide *SlideInput, tctx *previewTemplateContext, rs *resolvedSlide) {
	sgr := &resolvedShapeGrid{}
	if vl := resolveVirtualLayout(tctx.layouts, tctx.slideWidth, tctx.slideHeight); vl != nil {
		if needsVirtualLayout(*slide) {
			sgr.VirtualLayoutUsed = true
			sgr.LayoutID = vl.LayoutID
			sgr.Geometry = &resolvedGeom{
				X:      vl.Bounds.X,
				Y:      vl.Bounds.Y,
				Width:  vl.Bounds.CX,
				Height: vl.Bounds.CY,
			}
			rs.LayoutID = vl.LayoutID
			if lm, ok := tctx.layoutByID[vl.LayoutID]; ok {
				rs.LayoutName = lm.Name
			}
		}
	}
	rs.ShapeGridResolution = sgr
}

// computePreviewFitFindings runs fit detectors against the resolved plan.
func computePreviewFitFindings(input *PresentationInput, output *previewPlanOutput, tctx *previewTemplateContext, verbose bool) []patterns.FitFinding {
	resolvedInput := *input
	resolvedSlides := make([]SlideInput, len(input.Slides))
	copy(resolvedSlides, input.Slides)
	for i, rs := range output.ResolvedSlides {
		if i < len(resolvedSlides) && rs.LayoutID != "" {
			resolvedSlides[i].LayoutID = rs.LayoutID
		}
	}
	resolvedInput.Slides = resolvedSlides

	findings := collectFitFindings(&resolvedInput, tctx.layouts, tctx.slideWidth, tctx.slideHeight)
	return BudgetFitFindings(findings, DefaultFindingBudget, verbose)
}

// boundingBoxToGeom converts a BoundingBox to a resolvedGeom.
func boundingBoxToGeom(bb types.BoundingBox) *resolvedGeom {
	return &resolvedGeom{
		X:      bb.X,
		Y:      bb.Y,
		Width:  bb.Width,
		Height: bb.Height,
	}
}
