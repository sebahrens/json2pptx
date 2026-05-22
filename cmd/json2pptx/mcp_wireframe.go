// mcp_wireframe.go implements the preview_slide_wireframe MCP tool — a
// LibreOffice-free wireframe renderer that consumes the resolved
// preview_presentation_plan and returns annotated SVG / base64 PNG
// showing per-slide structural geometry: placeholder bounds, grid cells,
// occupancy, and fit-finding markers.
//
// This complements render_slide_image (which requires LibreOffice +
// ImageMagick) by providing a fast, dependency-free path agents can use
// to visualise layout decisions before committing to a full render
// round-trip.
package main

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/svggen"
)

// Wireframe inspection-contract constants. The preview_slide_wireframe
// tool produces a LibreOffice-free structural geometry preview, NOT a
// rendered-pixel inspection. These constants are stamped onto every
// response so an agent cannot treat a wireframe PNG as proof of rendered
// text flow, icon/text collisions, SVG label readability, or visual
// polish. See go-slide-creator-5lr1.
const (
	wireframeInspectionKind = "wireframe_structural"
	wireframeContract       = "structural_only"
)

// wireframeLimitations enumerates, in human-readable form, the checks a
// structural wireframe CANNOT perform. It is returned verbatim on every
// response so the limitation set is machine-discoverable, not buried in
// prose. Rendered visual QA (render_slide_image + inspect_slide_images)
// is required to cover these.
func wireframeLimitations() []string {
	return []string{
		"does not verify rendered text flow or PowerPoint text wrapping",
		"does not detect icon/text collisions or icon overlay insets",
		"does not assess SVG label or chart-axis readability",
		"does not apply template font metrics — geometry is approximate, not pixel-accurate",
		"does not evaluate image fidelity or visual polish",
		"not a substitute for rendered visual QA (render_slide_image + inspect_slide_images)",
	}
}

// previewSlideWireframeOutput is the JSON envelope returned by the
// preview_slide_wireframe tool. The PNG envelope fields (index,
// png_base64, width, height) mirror render_slide_image so agents can
// reuse the same handling code.
//
// The inspection-contract fields (inspection_kind, contract,
// not_text_flow_safe, limitations) are always populated and mark this
// output as structural-only — see the wireframe contract constants above.
type previewSlideWireframeOutput struct {
	Index            int      `json:"index"`
	InspectionKind   string   `json:"inspection_kind"`
	Contract         string   `json:"contract"`
	NotTextFlowSafe  bool     `json:"not_text_flow_safe"`
	Limitations      []string `json:"limitations"`
	SVG              string   `json:"svg,omitempty"`
	PNG64            string   `json:"png_base64,omitempty"`
	Width            int      `json:"width,omitempty"`
	Height           int      `json:"height,omitempty"`
	CellCount        int      `json:"cell_count"`
	PlaceholderCount int      `json:"placeholder_count"`
	FindingCount     int      `json:"finding_count"`
	LayoutID         string   `json:"layout_id,omitempty"`
	LayoutName       string   `json:"layout_name,omitempty"`
	SlideType        string   `json:"slide_type,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// mcpPreviewSlideWireframeTool returns the tool definition for
// preview_slide_wireframe.
func mcpPreviewSlideWireframeTool() mcp.Tool {
	return mcp.NewTool("preview_slide_wireframe",
		mcp.WithDescription(`Render an annotated wireframe of one slide's resolved plan as SVG and/or base64 PNG, without LibreOffice or ImageMagick. Shows the slide frame, layout placeholders, shape_grid cells (with row/col/kind/dimensions), occupancy, and fit-finding markers (severity-coded badges on the affected cell, plus a footer strip for off-cell findings).

STRUCTURAL ONLY — NOT VISUAL QA. The response carries inspection_kind="wireframe_structural", contract="structural_only", and not_text_flow_safe=true. This previews layout geometry; it does NOT model PowerPoint text wrapping, font metrics, icon/text collisions, SVG label readability, or image fidelity (see the limitations[] field). A wireframe PNG is NOT rendered-pixel evidence — rendered visual inspection (render_slide_image/render_deck_thumbnails followed by inspect_slide_images) is still required before a deck counts as visually verified.

Use this for fast structural sanity-checks before paying for a full generate_presentation + render_slide_image round-trip. Same plan-resolution path as preview_presentation_plan: pass the same presentation JSON, plus slide_index to pick which slide to render.

Output formats: pass format="svg" for SVG only, format="png" for base64 PNG only, or format="both" (default) for both. PNG generation is more expensive (rasterization) than SVG-only; prefer "svg" when an SVG viewer is available.`),
		mcp.WithRawOutputSchema(outputSchemaPreviewSlideWireframe),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Presentation definition. Same schema as generate_presentation / preview_presentation_plan.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithNumber("slide_index",
			mcp.Required(),
			mcp.Description("0-based index of the slide to render."),
		),
		mcp.WithString("format",
			mcp.Description(`Output format: "svg", "png", or "both" (default).`),
			mcp.Enum("svg", "png", "both"),
		),
		mcp.WithNumber("width_px",
			mcp.Description("Canvas width in points (pixels at standard DPI). Default: 960. Range: 320-2400."),
		),
	)
}

// handlePreviewSlideWireframe is the MCP handler for preview_slide_wireframe.
//
//nolint:gocognit,gocyclo // straight-line resolve+render flow; mirrors handlePreviewPlan structure
func (mc *mcpConfig) handlePreviewSlideWireframe(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("preview_slide_wireframe", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	// slide_index is required and 0-based.
	slideIdxF, ok := request.GetArguments()["slide_index"].(float64)
	if !ok {
		return argMissing("preview_slide_wireframe", "slide_index", "integer", 0, nil), nil
	}
	slideIdx := int(slideIdxF)
	if slideIdx < 0 {
		return argInvalidValue("preview_slide_wireframe", "INVALID_PARAMETER", "slide_index", "slide_index must be >= 0", "integer", 0, nil), nil
	}

	// Optional format (default "both").
	format := "both"
	if v, ok := request.GetArguments()["format"].(string); ok && v != "" {
		format = v
	}
	includeSVG := format == "svg" || format == "both"
	includePNG := format == "png" || format == "both"
	if !includeSVG && !includePNG {
		return argInvalidValue("preview_slide_wireframe", "INVALID_PARAMETER", "format", fmt.Sprintf("format must be one of \"svg\", \"png\", \"both\" — got %q", format), "string", "both", nil), nil
	}

	// Optional width_px (clamped 320..2400, default 960).
	widthPx := 960.0
	if v, ok := request.GetArguments()["width_px"].(float64); ok && v > 0 {
		widthPx = v
	}
	if widthPx < 320 {
		widthPx = 320
	} else if widthPx > 2400 {
		widthPx = 2400
	}

	// Parse JSON input.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nextCallGetInputSchema()), nil
	}

	applyDefaults(&input)

	if errResult := validatePreviewBoundary(&input); errResult != nil {
		return errResult, nil
	}

	if slideIdx >= len(input.Slides) {
		return argInvalidValue("preview_slide_wireframe", "INVALID_PARAMETER", "slide_index",
			fmt.Sprintf("slide_index %d out of range (deck has %d slides)", slideIdx, len(input.Slides)), "integer", 0, nil), nil
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

	// Resolve canonical layout names before plan resolution.
	resolveCanonicalLayoutIDs(input.Slides, tctx.layouts)

	// Reuse the existing preview plan resolver so the wireframe always
	// reflects the same geometry the engine would produce at generate time.
	plan := resolvePreviewSlides(&input, tctx)
	findings := computePreviewFitFindings(&input, &plan, tctx, true /*verbose*/)

	if slideIdx >= len(plan.ResolvedSlides) {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("resolved plan missing slide %d", slideIdx)), nil
	}
	rs := plan.ResolvedSlides[slideIdx]

	wf := buildWireframeRequest(slideIdx, &input, &rs, findings, tctx, widthPx)

	rendered, err := svggen.RenderWireframe(wf, svggen.RenderWireframeOptions{
		IncludeSVG: includeSVG,
		IncludePNG: includePNG,
		PNGScale:   1.0,
	})
	if err != nil {
		return api.MCPSimpleError("RENDER_FAILED", fmt.Sprintf("wireframe render failed: %v", err)), nil
	}

	out := previewSlideWireframeOutput{
		Index:            slideIdx,
		InspectionKind:   wireframeInspectionKind,
		Contract:         wireframeContract,
		NotTextFlowSafe:  true,
		Limitations:      wireframeLimitations(),
		Width:            rendered.Width,
		Height:           rendered.Height,
		CellCount:        len(wf.Cells),
		PlaceholderCount: len(wf.Placeholders),
		FindingCount:     len(wf.Findings),
		LayoutID:         rs.LayoutID,
		LayoutName:       rs.LayoutName,
		SlideType:        rs.SlideType,
	}
	if includeSVG {
		out.SVG = string(rendered.SVG)
	}
	if includePNG {
		out.PNG64 = base64.StdEncoding.EncodeToString(rendered.PNG)
	}
	// Pass through warnings/errors so the caller can fix plan issues
	// before iterating on visual feedback.
	out.Warnings = plan.Warnings
	out.Errors = plan.Errors

	mcpResult, err := api.MCPSuccessResult(ctx, out)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// buildWireframeRequest projects one resolved slide into the
// svggen.WireframeRequest shape, including any fit findings whose
// JSON-Pointer path points into this slide.
func buildWireframeRequest(slideIdx int, input *PresentationInput, rs *resolvedSlide, findings []patterns.FitFinding, tctx *previewTemplateContext, widthPx float64) *svggen.WireframeRequest {
	wf := &svggen.WireframeRequest{
		SlideIndex:    slideIdx,
		LayoutID:      rs.LayoutID,
		LayoutName:    rs.LayoutName,
		SlideType:     rs.SlideType,
		TemplateName:  input.Template,
		SlideWidth:    float64(tctx.slideWidth),
		SlideHeight:   float64(tctx.slideHeight),
		OutputWidthPx: widthPx,
	}

	// Title from the first text-ish content, if any — informational only.
	if slideIdx < len(input.Slides) {
		wf.Title = firstSlideTitle(input.Slides[slideIdx])
	}

	// Placeholders (only those with geometry).
	for _, p := range rs.Placeholders {
		if p.Geometry == nil {
			continue
		}
		wf.Placeholders = append(wf.Placeholders, svggen.WireframePlaceholder{
			ID:       p.ResolvedID,
			Remapped: p.Remapped,
			Rect: svggen.WireframeRect{
				X: float64(p.Geometry.X),
				Y: float64(p.Geometry.Y),
				W: float64(p.Geometry.Width),
				H: float64(p.Geometry.Height),
			},
		})
	}

	// Cells from shape_grid resolution.
	if rs.ShapeGridResolution != nil {
		for _, c := range rs.ShapeGridResolution.Cells {
			wf.Cells = append(wf.Cells, svggen.WireframeCell{
				Row:  c.Row,
				Col:  c.Col,
				Kind: c.Kind,
				Rect: svggen.WireframeRect{
					X: float64(c.X),
					Y: float64(c.Y),
					W: float64(c.W),
					H: float64(c.H),
				},
			})
		}
	}

	// Occupancy.
	if rs.Occupancy != nil {
		wf.Occupancy = &svggen.WireframeOccupancy{
			FilledPct:   rs.Occupancy.FilledPct,
			FilledSlots: rs.Occupancy.FilledSlots,
			TotalSlots:  rs.Occupancy.TotalSlots,
		}
	}

	// Findings whose path points into this slide. Cell-attached ones
	// carry row/col so the renderer can paint a badge in the right cell.
	for _, f := range findings {
		if slidepath.SlideIndex(f.Path) != slideIdx {
			continue
		}
		wfd := svggen.WireframeFinding{
			Code:    f.Code,
			Action:  f.Action,
			Message: f.Message,
		}
		if _, r, c, ok := slidepath.ParseGridCell(f.Path); ok {
			wfd.HasCell = true
			wfd.Row = r
			wfd.Col = c
		}
		wf.Findings = append(wf.Findings, wfd)
	}

	return wf
}

// firstSlideTitle extracts a short title string from the first text-ish
// content on a slide, for display in the wireframe header. Returns "" if
// no obvious title is available.
func firstSlideTitle(s SlideInput) string {
	for _, ci := range s.Content {
		if ci.PlaceholderID == "title" && ci.TextValue != nil && *ci.TextValue != "" {
			return *ci.TextValue
		}
	}
	for _, ci := range s.Content {
		if ci.Type == "text" && ci.TextValue != nil && *ci.TextValue != "" {
			return *ci.TextValue
		}
	}
	return ""
}
