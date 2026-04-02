package pipeline

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/data"
	"github.com/sebahrens/json2pptx/internal/parser"
	"github.com/sebahrens/json2pptx/internal/themegen"
	"github.com/sebahrens/json2pptx/internal/types"
)

// PrepareRequest holds the inputs for presentation preparation.
type PrepareRequest struct {
	// Markdown is the source content in markdown format.
	Markdown string
	// BaseDir is the base directory for resolving data file references.
	// If empty, data file references are not loaded.
	BaseDir string
	// DataOverrides are CLI-provided key=value variable overrides.
	DataOverrides map[string]string
	// TemplateAnalysis is the pre-analyzed template structure. Used for
	// brand_color and theme override resolution.
	TemplateAnalysis *types.TemplateAnalysis
}

// PrepareResult holds the parsed and validated presentation ready for
// downstream processing (PPTX generation, PDF rendering, etc.).
type PrepareResult struct {
	// Presentation is the fully parsed, validated, and enriched presentation.
	Presentation *types.PresentationDefinition
	// EffectiveAnalysis is the template analysis with any theme overrides applied.
	EffectiveAnalysis *types.TemplateAnalysis
	// Warnings contains non-fatal warnings from data resolution.
	Warnings []string
}

// PreparePresentation runs the shared preprocessing pipeline:
//
//	markdown parsing → error check → variable resolution → validation →
//	title enrichment → agenda generation → brand_color/theme resolution.
//
// Both the PPTX pipeline (Convert) and the PDF preview path call this
// function, then diverge at the generation step.
func PreparePresentation(req PrepareRequest) (*PrepareResult, error) {
	markdown := req.Markdown

	// Step 1: Parse markdown (frontmatter + slides + content extraction).
	presentation, err := parser.Parse(markdown)
	if err != nil {
		return nil, &ParseError{Err: err, Message: err.Error()}
	}

	// Check for fatal parsing errors.
	if parser.HasErrors(presentation) {
		errors := parser.GetErrorsByLevel(presentation, types.ErrorLevelError)
		if len(errors) > 0 {
			return nil, &ParseError{
				Err:     fmt.Errorf("%s", errors[0].Message),
				Line:    errors[0].Line,
				Field:   errors[0].Field,
				Message: errors[0].Message,
			}
		}
		return nil, &ParseError{Err: fmt.Errorf("markdown parsing failed")}
	}

	// Step 1.3: Resolve {{ variables }} from frontmatter data and CLI overrides.
	var dataWarnings []string
	if len(presentation.Metadata.Data) > 0 || len(req.DataOverrides) > 0 {
		dataCtx, dw, err := data.BuildContext(presentation.Metadata.Data, req.DataOverrides, req.BaseDir)
		if err != nil {
			return nil, &ParseError{Err: err, Message: fmt.Sprintf("data context: %v", err)}
		}
		dataWarnings = append(dw, data.ResolveVariables(presentation, dataCtx)...)
	}

	// Validate presentation structure — collect all errors, not just the first.
	validationErrors := parser.ValidatePresentation(presentation)
	if len(validationErrors) > 0 {
		msgs := make([]string, len(validationErrors))
		for i, ve := range validationErrors {
			msgs[i] = ve.Message
		}
		return nil, &ValidationError{
			Line:    validationErrors[0].Line,
			Field:   validationErrors[0].Field,
			Message: strings.Join(msgs, "; "),
		}
	}

	// Step 1.35: Enrich title slides with frontmatter metadata.
	EnrichTitleSlides(presentation)

	// Step 1.4: Auto-generate agenda slide (when enabled via frontmatter).
	if presentation.Metadata.AutoAgenda {
		GenerateAgenda(presentation)
	}

	// Step 1.6: Resolve brand_color into a ThemeOverride (if specified).
	if presentation.Metadata.BrandColor != "" {
		resolved, err := themegen.ResolveBrandColor(presentation.Metadata.BrandColor, presentation.Metadata.ThemeOverride)
		if err != nil {
			return nil, &ValidationError{Line: 1, Field: "brand_color", Message: err.Error()}
		}
		presentation.Metadata.ThemeOverride = resolved
	}

	// Step 1.7: Apply theme overrides from frontmatter (if any).
	effectiveAnalysis := req.TemplateAnalysis
	if presentation.Metadata.ThemeOverride != nil {
		analysisCopy := *req.TemplateAnalysis
		analysisCopy.Theme = analysisCopy.Theme.ApplyOverride(presentation.Metadata.ThemeOverride)
		effectiveAnalysis = &analysisCopy
	}

	return &PrepareResult{
		Presentation:      presentation,
		EffectiveAnalysis: effectiveAnalysis,
		Warnings:          dataWarnings,
	}, nil
}
