// Package generator provides PPTX file generation from slide specifications.
// This file implements diagram rendering by calling svggen directly.
package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/internal/types"
)

// DiagramRenderResult contains rendered image data and fit metadata for PPTX embedding.
// When FitMode is "contain" (auto-applied for all charts), the content dimensions
// and offsets are used to properly position the image within the placeholder.
type DiagramRenderResult struct {
	// SVG is the rendered SVG XML content (for native OOXML embedding via asvg:svgBlip).
	SVG []byte

	// PNG is the rendered image bytes (used as fallback for native SVG, or primary for raster strategy).
	PNG []byte

	// ContentWidth is the actual content width in pixels (after fit mode applied).
	ContentWidth float64

	// ContentHeight is the actual content height in pixels (after fit mode applied).
	ContentHeight float64

	// OffsetX is the horizontal offset to center the content in the container (pixels).
	OffsetX float64

	// OffsetY is the vertical offset to center the content in the container (pixels).
	OffsetY float64

	// FitMode indicates the fit mode used ("contain", "cover", or "" for stretch).
	FitMode string

	// ContainerWidth is the original container width before fit mode (pixels).
	ContainerWidth float64

	// ContainerHeight is the original container height before fit mode (pixels).
	ContainerHeight float64
}

// RenderDiagramSpec renders a DiagramSpec directly using svggen and returns the PNG bytes.
// This is the unified rendering function for all diagram types (charts and infographics).
//
// The themeColors parameter allows injecting template colors for consistent styling.
// Returns PNG bytes suitable for embedding in a PPTX document.
func RenderDiagramSpec(spec *types.DiagramSpec, themeColors []types.ThemeColor) ([]byte, error) {
	result, err := RenderDiagramSpecWithMetadata(spec, themeColors, 0, false)
	if err != nil {
		return nil, err
	}
	return result.PNG, nil
}

// RenderDiagramSpecWithMetadata renders a DiagramSpec and returns both image bytes and fit metadata.
// This is used for proper PPTX embedding with contain-mode positioning.
//
// The returned DiagramRenderResult contains:
//   - SVG bytes for native embedding (when svgOnly or both formats requested)
//   - PNG bytes for the image (when svgOnly is false)
//   - ContentWidth/ContentHeight: the actual image dimensions (may differ from container for "contain" mode)
//   - OffsetX/OffsetY: positioning offset to center the image in the placeholder
//   - FitMode: the fit mode that was applied
//
// The themeColors parameter allows injecting template colors for consistent styling.
// maxPNGWidth caps the PNG pixel width (0 = no cap).
// When svgOnly is true, only SVG is rendered (no rasterization), which eliminates the
// tdewolff/canvas mutex bottleneck for diagrams. The caller must supply a fallback PNG
// (e.g., the 1x1 transparent constant) when embedding with native SVG strategy.
func RenderDiagramSpecWithMetadata(spec *types.DiagramSpec, themeColors []types.ThemeColor, maxPNGWidth int, svgOnly bool) (*DiagramRenderResult, error) {
	if spec == nil {
		return nil, fmt.Errorf("diagram spec is required")
	}

	if spec.Type == "" {
		return nil, fmt.Errorf("diagram type is required")
	}

	// Convert DiagramSpec to svggen RequestEnvelope
	req := diagramSpecToSVGGen(spec, themeColors, maxPNGWidth)

	// Determine which formats to request from svggen.
	// When svgOnly is true (native SVG strategy), skip PNG rasterization entirely.
	var formats []string
	if svgOnly {
		formats = []string{"svg"}
	} else {
		formats = []string{"svg", "png"}
	}

	result, err := svggen.RenderMultiFormat(req, formats...)
	if err != nil {
		return nil, fmt.Errorf("svggen render failed: %w", err)
	}

	// When PNG was requested, verify we got data back.
	if !svgOnly && len(result.PNG) == 0 {
		return nil, fmt.Errorf("svggen returned empty PNG data")
	}

	// Extract fit metadata from SVGDocument
	renderResult := &DiagramRenderResult{
		PNG: result.PNG,
	}

	// Include SVG data for native embedding
	if result.SVG != nil && len(result.SVG.Content) > 0 {
		renderResult.SVG = result.SVG.Content
	}

	if result.SVG != nil {
		renderResult.FitMode = result.SVG.FitMode
		renderResult.ContentWidth = result.SVG.Width
		renderResult.ContentHeight = result.SVG.Height
		renderResult.OffsetX = result.SVG.OffsetX
		renderResult.OffsetY = result.SVG.OffsetY
		renderResult.ContainerWidth = result.SVG.ContainerWidth
		renderResult.ContainerHeight = result.SVG.ContainerHeight
	}

	return renderResult, nil
}

// diagramSpecToSVGGen converts a types.DiagramSpec to an svggen.RequestEnvelope.
// maxPNGWidth caps the PNG output width (0 = no cap).
func diagramSpecToSVGGen(spec *types.DiagramSpec, themeColors []types.ThemeColor, maxPNGWidth int) *svggen.RequestEnvelope {
	// Build style spec
	style := svggen.StyleSpec{}

	// Apply explicit style colors if available
	if spec.Style != nil && len(spec.Style.Colors) > 0 {
		style.Palette = svggen.PaletteSpec{Colors: spec.Style.Colors}
	} else if spec.Style != nil && len(spec.Style.ThemeColors) > 0 {
		// Pass full theme colors so StyleGuideFromSpec can build a complete
		// palette with semantic colors (Success, Warning, Error, text colors, etc.)
		themeInputs := make([]svggen.ThemeColorInput, len(spec.Style.ThemeColors))
		for i, tc := range spec.Style.ThemeColors {
			themeInputs[i] = svggen.ThemeColorInput{
				Name: tc.Name,
				RGB:  tc.RGB,
			}
		}
		style.ThemeColors = themeInputs
	} else if len(themeColors) > 0 {
		// Pass full theme colors from template
		themeInputs := make([]svggen.ThemeColorInput, len(themeColors))
		for i, tc := range themeColors {
			themeInputs[i] = svggen.ThemeColorInput{
				Name: tc.Name,
				RGB:  tc.RGB,
			}
		}
		style.ThemeColors = themeInputs
	}

	// Apply other style settings
	if spec.Style != nil {
		style.ShowLegend = spec.Style.ShowLegend
		style.ShowValues = spec.Style.ShowValues
		if spec.Style.FontFamily != "" {
			style.FontFamily = spec.Style.FontFamily
		}
		if spec.Style.Background != "" {
			style.Background = spec.Style.Background
		}
	}

	// Build output spec
	output := svggen.OutputSpec{
		Format:      "png",
		FitMode:     spec.FitMode,
		MaxPNGWidth: maxPNGWidth,
	}

	if spec.Width > 0 {
		output.Width = spec.Width
	} else {
		output.Width = types.DefaultChartWidth
	}

	if spec.Height > 0 {
		output.Height = spec.Height
	} else {
		output.Height = types.DefaultChartHeight
	}

	// Use dynamic scale if set, otherwise use default minimum scale
	if spec.Scale > 0 {
		output.Scale = spec.Scale
	} else {
		output.Scale = types.DefaultMinScale
	}

	return &svggen.RequestEnvelope{
		Type:     spec.Type,
		Title:    spec.Title,
		Subtitle: spec.Subtitle,
		Data:     spec.Data,
		Output:   output,
		Style:    style,
	}
}

