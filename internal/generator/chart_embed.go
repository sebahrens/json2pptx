// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"fmt"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/svggen"
)

// RenderAndEmbedOptions configures chart rendering and embedding.
type RenderAndEmbedOptions struct {
	// SlideIndex is the zero-based index of the target slide.
	SlideIndex int

	// Bounds defines the position and size of the chart on the slide.
	// All values are in EMUs (English Metric Units).
	Bounds pptx.RectEmu

	// Name is the shape name visible in PowerPoint's selection pane.
	// Optional. If empty, defaults to "Chart".
	Name string

	// AltText is the alternative text description for accessibility.
	// Optional but recommended.
	AltText string

	// PNGScale is the resolution multiplier for PNG fallback (default: 2.0).
	// Higher values produce better quality for print but larger file sizes.
	PNGScale float64
}

// RenderAndEmbed renders a chart from an svggen request and embeds it into a PPTX slide.
//
// This is a convenience API that combines chart generation with PPTX embedding:
//  1. Renders the chart to SVG using svggen
//  2. Generates a PNG fallback for older viewers
//  3. Embeds both into the specified slide using native SVG+PNG embedding
//
// The chart is rendered using svggen.RenderMultiFormat, which requires the diagram
// type to implement DiagramWithBuilder (for builder-based PNG) or MultiFormatRenderer.
//
// Example:
//
//	doc, closer, err := pptx.OpenDocumentFile("template.pptx")
//	if err != nil {
//	    return err
//	}
//	defer closer.Close()
//
//	chartSpec := &svggen.RequestEnvelope{
//	    Type:  "bar_chart",
//	    Title: "Quarterly Sales",
//	    Data:  map[string]any{"Q1": 100, "Q2": 120, "Q3": 90, "Q4": 150},
//	}
//
//	err = RenderAndEmbed(chartSpec, doc, RenderAndEmbedOptions{
//	    SlideIndex: 0,
//	    Bounds:     pptx.RectFromInches(1, 2, 6, 4),
//	    Name:       "Sales Chart",
//	})
//
// Returns an error if:
//   - The chart request is invalid or rendering fails
//   - The diagram type doesn't support multi-format rendering
//   - The slide index is out of range
//   - The PPTX embedding fails
func RenderAndEmbed(req *svggen.RequestEnvelope, doc *pptx.Document, opts RenderAndEmbedOptions) error {
	// Validate input
	if req == nil {
		return fmt.Errorf("RenderAndEmbed: chart request is required")
	}
	if doc == nil {
		return fmt.Errorf("RenderAndEmbed: document is required")
	}
	if opts.Bounds.IsZero() {
		return fmt.Errorf("RenderAndEmbed: bounds must have non-zero area")
	}

	// Set scale in request if not already set (uses opts.PNGScale or default 2.0)
	pngScale := opts.PNGScale
	if pngScale <= 0 {
		pngScale = 2.0
	}
	if req.Output.Scale <= 0 {
		req.Output.Scale = pngScale
	}

	// Render chart to SVG + PNG
	result, err := svggen.RenderMultiFormat(req, "svg", "png")
	if err != nil {
		return fmt.Errorf("RenderAndEmbed: chart rendering failed: %w", err)
	}

	// Validate we got both formats
	if result.SVG == nil || len(result.SVG.Content) == 0 {
		return fmt.Errorf("RenderAndEmbed: no SVG output from chart rendering")
	}
	if len(result.PNG) == 0 {
		return fmt.Errorf("RenderAndEmbed: no PNG output from chart rendering (diagram may not support multi-format)")
	}

	// Set default name
	name := opts.Name
	if name == "" {
		name = "Chart"
	}

	// Embed into slide
	insertOpts := pptx.InsertOptions{
		SlideIndex: opts.SlideIndex,
		Bounds:     opts.Bounds,
		SVGData:    result.SVG.Content,
		PNGData:    result.PNG,
		Name:       name,
		AltText:    opts.AltText,
	}

	if err := doc.InsertSVG(insertOpts); err != nil {
		return fmt.Errorf("RenderAndEmbed: PPTX embedding failed: %w", err)
	}

	return nil
}

// RenderAndEmbedResult contains information about the embedded chart.
type RenderAndEmbedResult struct {
	// SVGWidth is the rendered SVG width in points.
	SVGWidth float64

	// SVGHeight is the rendered SVG height in points.
	SVGHeight float64

	// PNGSize is the size of the PNG fallback in bytes.
	PNGSize int
}

// RenderAndEmbedWithResult is like RenderAndEmbed but also returns rendering statistics.
//
// Use this when you need information about the rendered chart for logging or debugging.
func RenderAndEmbedWithResult(req *svggen.RequestEnvelope, doc *pptx.Document, opts RenderAndEmbedOptions) (*RenderAndEmbedResult, error) {
	// Validate input
	if req == nil {
		return nil, fmt.Errorf("RenderAndEmbed: chart request is required")
	}
	if doc == nil {
		return nil, fmt.Errorf("RenderAndEmbed: document is required")
	}
	if opts.Bounds.IsZero() {
		return nil, fmt.Errorf("RenderAndEmbed: bounds must have non-zero area")
	}

	// Set default PNG scale
	pngScale := opts.PNGScale
	if pngScale <= 0 {
		pngScale = 2.0
	}

	// Set scale in request if not already set
	if req.Output.Scale <= 0 {
		req.Output.Scale = pngScale
	}

	// Render chart to SVG + PNG
	result, err := svggen.RenderMultiFormat(req, "svg", "png")
	if err != nil {
		return nil, fmt.Errorf("RenderAndEmbed: chart rendering failed: %w", err)
	}

	// Validate we got both formats
	if result.SVG == nil || len(result.SVG.Content) == 0 {
		return nil, fmt.Errorf("RenderAndEmbed: no SVG output from chart rendering")
	}
	if len(result.PNG) == 0 {
		return nil, fmt.Errorf("RenderAndEmbed: no PNG output from chart rendering (diagram may not support multi-format)")
	}

	// Set default name
	name := opts.Name
	if name == "" {
		name = "Chart"
	}

	// Embed into slide
	insertOpts := pptx.InsertOptions{
		SlideIndex: opts.SlideIndex,
		Bounds:     opts.Bounds,
		SVGData:    result.SVG.Content,
		PNGData:    result.PNG,
		Name:       name,
		AltText:    opts.AltText,
	}

	if err := doc.InsertSVG(insertOpts); err != nil {
		return nil, fmt.Errorf("RenderAndEmbed: PPTX embedding failed: %w", err)
	}

	return &RenderAndEmbedResult{
		SVGWidth:  result.SVG.Width,
		SVGHeight: result.SVG.Height,
		PNGSize:   len(result.PNG),
	}, nil
}

// ChartEmbedder provides stateful chart embedding with configuration.
// Use this when embedding multiple charts with consistent settings.
type ChartEmbedder struct {
	// Registry is the svggen registry to use for rendering.
	// If nil, uses svggen.DefaultRegistry().
	Registry *svggen.Registry

	// DefaultPNGScale is the default PNG scale factor.
	// If zero, uses 2.0.
	DefaultPNGScale float64
}

// NewChartEmbedder creates a new ChartEmbedder with default settings.
func NewChartEmbedder() *ChartEmbedder {
	return &ChartEmbedder{
		Registry:        svggen.DefaultRegistry(),
		DefaultPNGScale: 2.0,
	}
}

// Embed renders a chart and embeds it into a PPTX slide.
// This method uses the embedder's configured registry and settings.
func (e *ChartEmbedder) Embed(req *svggen.RequestEnvelope, doc *pptx.Document, opts RenderAndEmbedOptions) error {
	// Validate input
	if req == nil {
		return fmt.Errorf("ChartEmbedder.Embed: chart request is required")
	}
	if doc == nil {
		return fmt.Errorf("ChartEmbedder.Embed: document is required")
	}
	if opts.Bounds.IsZero() {
		return fmt.Errorf("ChartEmbedder.Embed: bounds must have non-zero area")
	}

	// Set default PNG scale
	pngScale := opts.PNGScale
	if pngScale <= 0 {
		pngScale = e.DefaultPNGScale
	}
	if pngScale <= 0 {
		pngScale = 2.0
	}

	// Set scale in request if not already set
	if req.Output.Scale <= 0 {
		req.Output.Scale = pngScale
	}

	// Get registry
	registry := e.Registry
	if registry == nil {
		registry = svggen.DefaultRegistry()
	}

	// Render chart to SVG + PNG
	result, err := svggen.RegistryRenderMultiFormat(registry, req, "svg", "png")
	if err != nil {
		return fmt.Errorf("ChartEmbedder.Embed: chart rendering failed: %w", err)
	}

	// Validate we got both formats
	if result.SVG == nil || len(result.SVG.Content) == 0 {
		return fmt.Errorf("ChartEmbedder.Embed: no SVG output from chart rendering")
	}
	if len(result.PNG) == 0 {
		return fmt.Errorf("ChartEmbedder.Embed: no PNG output from chart rendering (diagram may not support multi-format)")
	}

	// Set default name
	name := opts.Name
	if name == "" {
		name = "Chart"
	}

	// Embed into slide
	insertOpts := pptx.InsertOptions{
		SlideIndex: opts.SlideIndex,
		Bounds:     opts.Bounds,
		SVGData:    result.SVG.Content,
		PNGData:    result.PNG,
		Name:       name,
		AltText:    opts.AltText,
	}

	return doc.InsertSVG(insertOpts)
}
