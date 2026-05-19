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
	"github.com/sebahrens/json2pptx/internal/slidepath"
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

	// ResponseFingerprint is a sha256 hex digest of the canonical JSON of this
	// response with the field zeroed. Agents may use it as a cache key.
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`

	// composeFindings holds structured fit findings emitted during
	// per-segment compose resolution (carrying segment_index). They are
	// merged into FitFindings during computePreviewFitFindings so they
	// participate in the same budgeting pass.
	composeFindings []patterns.FitFinding `json:"-"`
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
	ExpandedCompose     *resolvedCompose      `json:"expanded_compose,omitempty"`
	ShapeGridResolution *resolvedShapeGrid    `json:"shape_grid_resolution,omitempty"`
	AppliedDefaults     *resolvedDefaults     `json:"applied_defaults,omitempty"`
	Occupancy           *resolvedOccupancy    `json:"occupancy,omitempty"`
}

// resolvedOccupancy reports grid slot usage for a slide's shape_grid.
type resolvedOccupancy struct {
	FilledPct   float64 `json:"filled_pct"`
	FilledSlots int     `json:"filled_slots"`
	TotalSlots  int     `json:"total_slots"`
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

// resolvedCompose describes the per-segment expansion of a compose envelope
// so agents can see how each child segment was expanded and where it sits
// within the merged grid without having to re-run expansion themselves.
type resolvedCompose struct {
	Direction string                   `json:"direction"`
	Segments  []resolvedComposeSegment `json:"segments"`
}

// resolvedComposeSegment is one child segment of a compose envelope after
// expansion. BoundsPct expresses the segment's rectangle as percentages of
// the merged grid's region (0..100). RowRange and ColRange give the merged
// grid's row/col indices the segment occupies (start inclusive, end exclusive).
type resolvedComposeSegment struct {
	Index               int                        `json:"index"`
	Pattern             string                     `json:"pattern"`
	CellsAfterExpansion int                        `json:"cells_after_expansion"`
	BoundsPct           resolvedComposeSegmentRect `json:"bounds_pct"`
	RowRange            [2]int                     `json:"row_range"`
	ColRange            [2]int                     `json:"col_range"`
}

// resolvedComposeSegmentRect is a segment's rectangle in compose-merged space,
// expressed as percentages of the merged grid (0..100).
type resolvedComposeSegmentRect struct {
	XPct      float64 `json:"x_pct"`
	YPct      float64 `json:"y_pct"`
	WidthPct  float64 `json:"width_pct"`
	HeightPct float64 `json:"height_pct"`
}

// resolvedShapeGrid describes virtual layout resolution for shape_grid slides.
type resolvedShapeGrid struct {
	VirtualLayoutUsed bool                     `json:"virtual_layout_used"`
	LayoutID          string                   `json:"layout_id,omitempty"`
	Geometry          *resolvedGeom            `json:"geometry,omitempty"`
	Cells             []resolvedShapeGridCell  `json:"cells,omitempty"`
}

// resolvedShapeGridCell is one resolved grid cell rectangle, suitable for
// wireframe rendering without paying a full PPTX generation round-trip.
// Coordinates are in EMUs (914400 EMU = 1 inch).
type resolvedShapeGridCell struct {
	Row  int    `json:"row"`
	Col  int    `json:"col"`
	X    int64  `json:"x"`
	Y    int64  `json:"y"`
	W    int64  `json:"w"`
	H    int64  `json:"h"`
	Kind string `json:"kind"`
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
		mcp.WithRawOutputSchema(outputSchemaPreviewPlan),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Presentation definition. Same schema as generate_presentation.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithBoolean("fit_report",
			mcp.Description("When true, include fit_findings in the response. Default: true."),
			mcp.DefaultBool(true),
		),
		mcp.WithBoolean("verbose_fit",
			mcp.Description("When true, return all fit findings without the per-slide budget limit. Default: false."),
		),
		mcp.WithBoolean("strict_unknown_keys",
			mcp.Description("When true, unknown JSON keys are errors that block preview. When false (default), unknown keys are reported as warnings and preview proceeds."),
		),
	)
}

// --- Handler ---

// handlePreviewPlan returns the resolved presentation plan without rendering.
// All error responses (boundary errors, template lookup, marshal failure) carry
// a next_tool_call suggestion so the agent can chain forward without having to
// infer the recovery path from prose.
func (mc *mcpConfig) handlePreviewPlan(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return mcpErrorWithNext("MISSING_PARAMETER", "presentation is required", nextCallGetInputSchema()), nil
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseErrorWithNext("INVALID_JSON", "presentation", fmt.Sprintf("invalid JSON: %v", err), nextCallGetInputSchema()), nil
	}

	// Apply deck-level defaults before resolution.
	applyDefaults(&input)

	// Expand structure block into flat slides (mutually exclusive with
	// top-level slides). Mirrors the CLI path so MCP and CLI agree on the
	// effective slide list before boundary validation runs.
	if structDiags := applyStructureExpansion(&input); len(structDiags) > 0 {
		return api.MCPDiagnosticsError(structDiags), nil
	}

	// Boundary validation.
	if errResult := validatePreviewBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Unknown keys — warnings by default, errors when strict_unknown_keys=true.
	// In strict mode we fail-fast before paying for template resolution so a
	// typo'd field surfaces as a typed diagnostic, matching generate/validate.
	strictUnknownKeys, _ := request.GetArguments()["strict_unknown_keys"].(bool)
	if strictUnknownKeys {
		if diags := unknownKeyDiags([]byte(jsonStr), true); len(diags) > 0 {
			return api.MCPDiagnosticsError(diags), nil
		}
	}

	// Resolve template.
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err != nil {
		return mcpErrorWithNext("TEMPLATE_NOT_FOUND", templateNotFoundError(input.Template, mc.templatesDir), nextCallListTemplates()), nil
	}
	defer templateCleanup()

	tctx, err := loadPreviewTemplate(templatePath)
	if err != nil {
		return mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("template analysis failed: %v", err), nextCallListTemplates()), nil
	}
	defer func() { _ = tctx.reader.Close() }()

	// Resolve canonical layout names (e.g. "title", "content", "blank") to
	// concrete layout IDs before resolution.
	resolveCanonicalLayoutIDs(input.Slides, tctx.layouts)

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

	if err := api.ComputeResponseFingerprint(&output); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("preview_presentation_plan", "presentation")), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("preview_presentation_plan", "presentation")), nil
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
			Severity:     diagnostics.SeverityError,
			NextToolCall: nextCallListTemplates(),
		})
	}
	if len(input.Slides) == 0 {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity:     diagnostics.SeverityError,
			NextToolCall: nextCallGetInputSchema(),
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
	theme        *types.ThemeInfo
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
	_ = template.SynthesizeIfNeeded(reader, analysis)

	layoutByID := make(map[string]types.LayoutMetadata, len(analysis.Layouts))
	for _, l := range analysis.Layouts {
		layoutByID[l.ID] = l
	}

	metadata, _ := template.ParseMetadata(reader)
	theme := template.ParseTheme(reader)

	return &previewTemplateContext{
		reader:      reader,
		layouts:     analysis.Layouts,
		layoutByID:  layoutByID,
		metadata:    metadata,
		slideWidth:  slideWidth,
		slideHeight: slideHeight,
		theme:       &theme,
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

	// Compute per-slide section indices for accent strategy.
	sectionIndices := make([]int, len(input.Slides))
	{
		secIdx := 0
		for i := range input.Slides {
			if i > 0 && inferSlideType(input.Slides[i]) == types.SlideTypeSection {
				secIdx++
			}
			sectionIndices[i] = secIdx
		}
	}
	accentStrategy := patterns.AccentStrategy(input.AccentStrategy)

	for i := range input.Slides {
		rs := resolveOneSlide(i, &input.Slides[i], input, tctx, usedLayouts, &output, accentStrategy, sectionIndices[i])
		output.ResolvedSlides = append(output.ResolvedSlides, rs)
	}

	return output
}

// resolveOneSlide resolves layout, placeholders, patterns, and shape_grid for a single slide.
func resolveOneSlide(i int, slide *SlideInput, input *PresentationInput, tctx *previewTemplateContext, usedLayouts map[string]int, output *previewPlanOutput, accentStrategy patterns.AccentStrategy, sectionIndex int) resolvedSlide { //nolint:gocognit,gocyclo
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

	// Compose expansion.
	if slide.Compose != nil && slide.ShapeGrid == nil {
		resolveSlideCompose(i, slide, tctx, output, &rs, accentStrategy, sectionIndex)
	}

	// Pattern expansion.
	if slide.Pattern != nil && slide.ShapeGrid == nil {
		resolveSlidePattern(i, slide, tctx, output, &rs, accentStrategy, sectionIndex)
	}

	// Shape grid resolution: virtual layout (when template layouts are
	// available) and per-cell wireframe rectangles (whenever the slide has a
	// resolved grid, including patterns/compose that produced one above).
	if slide.ShapeGrid != nil {
		resolveSlideShapeGrid(slide, tctx, &rs)
	}

	// Grid occupancy for preview response.
	if slide.ShapeGrid != nil && len(slide.ShapeGrid.Rows) > 0 {
		rs.Occupancy = computeResolvedOccupancy(slide.ShapeGrid)
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
func resolveSlidePattern(i int, slide *SlideInput, tctx *previewTemplateContext, output *previewPlanOutput, rs *resolvedSlide, accentStrategy patterns.AccentStrategy, sectionIndex int) {
	expCtx := patterns.ExpandContext{
		Metadata:       tctx.metadata,
		SlideWidth:     tctx.slideWidth,
		SlideHeight:    tctx.slideHeight,
		AccentStrategy: accentStrategy,
		SlideIndex:     i,
		SectionIndex:   sectionIndex,
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
	for _, w := range expandWarnings {
		output.Warnings = append(output.Warnings,
			fmt.Sprintf("slide %d: %s", i+1, w))
		if f := patternWarningAsFinding(i, slide.Pattern.Name, w); f != nil {
			output.composeFindings = append(output.composeFindings, *f)
		}
	}
	slide.ShapeGrid = expanded
}

// resolveSlideCompose expands a compose envelope and updates the slide's
// shape_grid. It also populates rs.ExpandedCompose with per-segment expansion
// metadata (pattern name, cell count, bounds_pct, and row/col ranges in the
// merged grid) so agents can attribute findings to a specific segment, and
// emits structured fit findings carrying segment_index for any
// compose-time warnings.
func resolveSlideCompose(i int, slide *SlideInput, tctx *previewTemplateContext, output *previewPlanOutput, rs *resolvedSlide, accentStrategy patterns.AccentStrategy, sectionIndex int) {
	expCtx := patterns.ExpandContext{
		Metadata:       tctx.metadata,
		SlideWidth:     tctx.slideWidth,
		SlideHeight:    tctx.slideHeight,
		AccentStrategy: accentStrategy,
		SlideIndex:     i,
		SectionIndex:   sectionIndex,
	}
	expanded, composeWarnings, err := expandCompose(slide.Compose, expCtx, patterns.Default())
	if err != nil {
		output.Errors = append(output.Errors,
			fmt.Sprintf("slide %d: compose: %v", i+1, err))
		// Even on merge failure, the error itself is segment-attributable
		// when expandPattern surfaced "compose: segment[N]: ...". Emit a
		// structured finding so the agent can target the offending segment.
		if rs != nil {
			if f := composeErrorAsFinding(i, err); f != nil {
				output.composeFindings = append(output.composeFindings, *f)
			}
		}
		return
	}
	for _, w := range composeWarnings {
		output.Warnings = append(output.Warnings,
			fmt.Sprintf("slide %d: %s", i+1, w))
		if f := composeWarningAsFinding(i, w); f != nil {
			output.composeFindings = append(output.composeFindings, *f)
		}
	}
	slide.ShapeGrid = expanded
	if rs != nil {
		rs.ExpandedCompose = buildExpandedCompose(slide.Compose, expCtx, patterns.Default(), expanded)
	}
}

// resolveSlideShapeGrid resolves virtual layout and per-cell wireframe rects
// for shape_grid slides.
func resolveSlideShapeGrid(slide *SlideInput, tctx *previewTemplateContext, rs *resolvedSlide) {
	sgr := &resolvedShapeGrid{}
	if len(tctx.layouts) > 0 {
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
	}
	sgr.Cells = collectResolvedGridCells(slide.ShapeGrid, tctx.slideWidth, tctx.slideHeight)
	rs.ShapeGridResolution = sgr
}

// collectResolvedGridCells resolves a ShapeGridInput to a slice of wireframe
// cells (row, col, x, y, w, h, kind) in EMUs. Returns nil when the grid cannot
// be resolved (e.g. invalid column percentages).
func collectResolvedGridCells(grid *ShapeGridInput, slideWidth, slideHeight int64) []resolvedShapeGridCell {
	if grid == nil || len(grid.Rows) == 0 {
		return nil
	}
	result := resolveGridForStructural(grid, slideWidth, slideHeight)
	if result == nil || len(result.Cells) == 0 {
		return nil
	}
	cells := make([]resolvedShapeGridCell, 0, len(result.Cells))
	for i := range result.Cells {
		rc := &result.Cells[i]
		cells = append(cells, resolvedShapeGridCell{
			Row:  rc.RowIdx,
			Col:  rc.ColIdx,
			X:    rc.CellBounds.X,
			Y:    rc.CellBounds.Y,
			W:    rc.CellBounds.CX,
			H:    rc.CellBounds.CY,
			Kind: string(rc.Kind),
		})
	}
	return cells
}

// computeResolvedOccupancy computes grid slot usage for the preview response.
func computeResolvedOccupancy(grid *ShapeGridInput) *resolvedOccupancy {
	numCols := inferGridColumns(grid)
	if numCols <= 0 {
		return nil
	}
	totalSlots := len(grid.Rows) * numCols
	filledSlots := 0
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				filledSlots++
			}
		}
	}
	if totalSlots <= 0 {
		return nil
	}
	filledPct := float64(filledSlots) / float64(totalSlots) * 100.0
	return &resolvedOccupancy{
		FilledPct:   filledPct,
		FilledSlots: filledSlots,
		TotalSlots:  totalSlots,
	}
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

	findings := collectFitFindings(&resolvedInput, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.theme)
	findings = append(findings, placeholderRemappedFindings(output.ResolvedSlides)...)
	if len(output.composeFindings) > 0 {
		findings = append(findings, output.composeFindings...)
	}
	// Attribute compose-merged grid cell findings to their originating
	// segment using ExpandedCompose's per-segment row/col ranges.
	attachComposeSegmentIndex(findings, output.ResolvedSlides)
	return BudgetFitFindings(findings, DefaultFindingBudget, verbose)
}

// placeholderRemappedFindings emits an info-level fit finding for every
// resolved placeholder whose virtual placeholder_id was remapped to a concrete
// layout placeholder. This mirrors the render-time emission in
// generator/slide_preparation.go so agents iterating fit_findings can see
// remappings without walking resolved_slides[].placeholders[].remapped.
func placeholderRemappedFindings(resolvedSlides []resolvedSlide) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, rs := range resolvedSlides {
		for contentIdx, rp := range rs.Placeholders {
			if !rp.Remapped {
				continue
			}
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path:    slidepath.ContentField(rs.SlideIndex, contentIdx, "placeholder_id"),
					Code:    patterns.ErrCodePlaceholderRemapped,
					Message: fmt.Sprintf("slide %d: placeholder %q remapped to %q for layout %q", rs.SlideIndex+1, rp.InputID, rp.ResolvedID, rs.LayoutID),
					Fix: &patterns.FixSuggestion{
						Kind:   "remap_placeholder",
						Params: map[string]any{"from": rp.InputID, "to": rp.ResolvedID},
					},
				},
				Action: "info",
			})
		}
	}
	return out
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
