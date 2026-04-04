// Registry type and basic operations (Register, Alias, Get, Types, Render)
// have been moved to core/registry.go. They are re-exported via core_aliases.go.
//
// This file contains multi-format rendering support which depends on SVGBuilder
// and therefore stays in the root package.
package svggen

import (
	"fmt"
	"log/slog"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// RenderMultiFormat dispatches a request and returns results in multiple formats.
// It always generates SVG, and optionally generates PNG and/or PDF based on the
// request's Output.Format field and additional requested formats.
//
// The function supports three diagram interface levels:
// 1. MultiFormatRenderer - diagrams that natively support multiple formats
// 2. DiagramWithBuilder - diagrams that expose their builder for format conversion
// 3. Diagram - basic diagrams (SVG only)
func RenderMultiFormat(req *RequestEnvelope, formats ...string) (*RenderResult, error) {
	return renderMultiFormat(DefaultRegistry(), req, formats...)
}

// RegistryRenderMultiFormat is like RenderMultiFormat but uses a specific
// registry instance instead of the default. Use this when you maintain your
// own registry (e.g., in an API server).
func RegistryRenderMultiFormat(r *Registry, req *RequestEnvelope, formats ...string) (*RenderResult, error) {
	return renderMultiFormat(r, req, formats...)
}

// renderMultiFormat implements multi-format rendering for a given registry.
//nolint:gocognit,gocyclo // complex chart rendering logic
func renderMultiFormat(r *Registry, req *RequestEnvelope, formats ...string) (*RenderResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Warn when data is empty — the diagram will render blank.
	if len(req.Data) == 0 {
		slog.Warn("diagram data is empty; output will be blank", "type", req.Type)
	}

	// Clamp extreme float64 values to prevent NaN/Inf in downstream math.
	if req.Data != nil {
		core.ClampDataValues(req.Data)
	}

	d := r.Get(req.Type)
	if d == nil {
		return nil, fmt.Errorf("svggen: unknown diagram type %q", req.Type)
	}

	if err := d.Validate(req); err != nil {
		return nil, fmt.Errorf("svggen: validation failed for %q: %w", req.Type, err)
	}

	// Determine which formats to generate
	formatSet := make(map[string]bool)
	if req.Output.Format != "" {
		formatSet[req.Output.Format] = true
	}
	for _, f := range formats {
		formatSet[f] = true
	}

	needsPNG := formatSet["png"]
	needsPDF := formatSet["pdf"]

	var svgDoc *SVGDocument
	var builder *SVGBuilder
	var err error

	// Try to get the builder if we need PNG or PDF
	if needsPNG || needsPDF {
		// First, check if diagram implements MultiFormatRenderer
		if mfr, ok := d.(MultiFormatRenderer); ok {
			return renderViaMultiFormat(mfr, req, formatSet)
		}

		// Next, check if diagram implements DiagramWithBuilder
		if dwb, ok := d.(DiagramWithBuilder); ok {
			builder, svgDoc, err = dwb.RenderWithBuilder(req)
			if err != nil {
				return nil, err
			}
		} else {
			// Fallback: diagram doesn't support multi-format
			return nil, fmt.Errorf("svggen: diagram %q does not support multi-format rendering; implement DiagramWithBuilder or MultiFormatRenderer", req.Type)
		}
	} else {
		// Only SVG needed - use regular Render
		svgDoc, err = d.Render(req)
		if err != nil {
			return nil, err
		}
	}

	result := &RenderResult{
		SVG:    svgDoc,
		Format: "svg",
		Width:  int(svgDoc.Width),
		Height: int(svgDoc.Height),
	}

	// Generate PNG if needed
	if needsPNG && builder != nil {
		scale := req.Output.Scale
		if scale <= 0 {
			scale = 2.0
		}
		// Cap scale so the output pixel width does not exceed MaxPNGWidth.
		// Pixel width = svgWidth * scale * (96 DPI / 72 pt-per-inch).
		if maxW := req.Output.MaxPNGWidth; maxW > 0 && svgDoc.Width > 0 {
			const dpiRatio = 96.0 / 72.0 // canvas pt→px factor
			pixelW := svgDoc.Width * scale * dpiRatio
			if pixelW > float64(maxW) {
				scale = float64(maxW) / (svgDoc.Width * dpiRatio)
			}
		}
		pngBytes, err := builder.RenderPNG(scale)
		if err != nil {
			return nil, fmt.Errorf("svggen: PNG render failed: %w", err)
		}
		result.PNG = pngBytes
		if req.Output.Format == "png" {
			result.Format = "png"
		}
	}

	// Generate PDF if needed
	if needsPDF && builder != nil {
		pdfBytes, err := builder.RenderPDF()
		if err != nil {
			return nil, fmt.Errorf("svggen: PDF render failed: %w", err)
		}
		result.PDF = pdfBytes
		if req.Output.Format == "pdf" {
			result.Format = "pdf"
		}
	}

	return result, nil
}

// renderViaMultiFormat handles rendering for diagrams implementing MultiFormatRenderer.
func renderViaMultiFormat(mfr MultiFormatRenderer, req *RequestEnvelope, formatSet map[string]bool) (*RenderResult, error) {
	svgDoc, err := mfr.Render(req)
	if err != nil {
		return nil, err
	}

	result := &RenderResult{
		SVG:    svgDoc,
		Format: "svg",
		Width:  int(svgDoc.Width),
		Height: int(svgDoc.Height),
	}

	if formatSet["png"] {
		scale := req.Output.Scale
		if scale <= 0 {
			scale = 2.0
		}
		pngBytes, err := mfr.RenderPNG(req, scale)
		if err != nil {
			return nil, fmt.Errorf("svggen: PNG render failed: %w", err)
		}
		result.PNG = pngBytes
		if req.Output.Format == "png" {
			result.Format = "png"
		}
	}

	if formatSet["pdf"] {
		pdfBytes, err := mfr.RenderPDF(req)
		if err != nil {
			return nil, fmt.Errorf("svggen: PDF render failed: %w", err)
		}
		result.PDF = pdfBytes
		if req.Output.Format == "pdf" {
			result.Format = "pdf"
		}
	}

	return result, nil
}

// DiagramWithBuilder is an optional interface that diagrams can implement
// to expose their SVGBuilder for multi-format rendering.
type DiagramWithBuilder interface {
	Diagram
	// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
	// The builder can then be used to generate PNG/PDF output.
	RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error)
}
