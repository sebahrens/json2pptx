// Package generator provides PPTX slide generation from JSON slide definitions.
//
// The generator package is the core engine that transforms slide definitions into
// fully-formed PowerPoint presentations. It handles all aspects of PPTX assembly:
//
//   - Slide creation from template layouts
//   - Text and bullet content population
//   - Image embedding with proper scaling
//   - Chart and SVG diagram integration
//   - Table rendering
//
// # Architecture
//
// The package uses a pipeline approach:
//
//	SlideDefinition → ContentMapping → SlidePreparation → PPTX Assembly
//
// Content items are routed to placeholders via the slot router, which matches
// content types to available placeholder slots.
//
// # Usage
//
//	req := generator.GenerationRequest{
//	    TemplatePath: "templates/corporate.pptx",
//	    OutputPath:   "output/presentation.pptx",
//	    Slides:       slideSpecs,
//	}
//	result, err := generator.Generate(ctx, req)
//
// # Chart Integration
//
// Charts and diagrams are rendered via the svggen package and embedded as
// PNG images for maximum PowerPoint compatibility. The chart_render and
// chart_embed submodules handle batch rendering and PPTX insertion.
//
// # Dependencies
//
// This package integrates with internal/pptx for low-level PPTX manipulation,
// github.com/sebahrens/json2pptx/svggen for chart rendering, and internal/types for shared data models.
package generator
