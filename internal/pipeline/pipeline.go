// Package pipeline provides a conversion pipeline service that orchestrates
// markdown parsing, layout selection, and PPTX generation. This separates
// orchestration concerns from the HTTP API layer.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pagination"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Pipeline is the interface for the presentation conversion pipeline.
// It orchestrates the full conversion flow: parse markdown → select layouts → generate PPTX.
type Pipeline interface {
	// Convert parses markdown, selects layouts, and generates a PPTX file.
	// Returns the generation result or an error.
	Convert(ctx context.Context, req ConvertRequest) (*ConvertResult, error)
}

// ConvertRequest contains the input for a conversion operation.
type ConvertRequest struct {
	// Presentation is the pre-parsed presentation definition (required).
	Presentation *types.PresentationDefinition
	// TemplateAnalysis is the pre-analyzed template structure.
	TemplateAnalysis *types.TemplateAnalysis
	// OutputPath is the path where the generated PPTX should be written.
	OutputPath string
	// TemplatePath is the path to the source template file.
	TemplatePath string
	// SVGStrategy is the SVG conversion strategy: "png" (default), "emf", or "native".
	SVGStrategy string
	// SVGScale is the scale factor for SVG to PNG conversion (0 means use default).
	SVGScale float64
	// SVGNativeCompat is the native SVG compatibility mode: "warn" (default), "fallback", "strict", "ignore".
	SVGNativeCompat string
	// MaxPNGWidth caps the pixel width of PNG fallback images (0 = no cap, default: 2500).
	MaxPNGWidth int
	// ExcludeTemplateSlides controls whether template's example slides are excluded.
	// Defaults to true — template slides are excluded from output.
	// All callers (CLI, API, tests) should set this to true.
	ExcludeTemplateSlides bool
	// Partial enables error-resilient compilation: per-slide errors produce
	// placeholder slides instead of aborting the entire job.
	Partial bool
	// DataOverrides are CLI-provided key=value variable overrides.
	DataOverrides map[string]string
	// BaseDir is the base directory for resolving relative data file paths.
	// If empty, data file references in frontmatter are not loaded.
	BaseDir string
	// DryRun skips PPTX generation and returns validation results only.
	// When true, the pipeline runs all parsing, validation, and layout
	// selection steps but does not produce an output file.
	DryRun bool
	// StrictFit controls how chart/diagram fit findings affect generation.
	// Values: "off", "warn" (default), "strict".
	StrictFit string
}

// ConvertResult contains the output of a conversion operation.
type ConvertResult struct {
	// OutputPath is the path to the generated PPTX file.
	OutputPath string
	// SlideCount is the number of slides generated.
	SlideCount int
	// Warnings contains any warnings generated during conversion.
	Warnings []string
	// FailedSlides lists per-slide errors (only populated in partial mode).
	FailedSlides []SlideError
	// DryRunSlides contains per-slide layout and placeholder details.
	// Only populated when DryRun is true in the request.
	DryRunSlides []DryRunSlide
}

// DryRunSlide describes the layout selection and placeholder mapping for a
// single slide during a dry-run validation pass.
type DryRunSlide struct {
	SlideNumber  int                 `json:"slide_number"`
	Title        string              `json:"title"`
	LayoutID     string              `json:"layout_id"`
	LayoutName   string              `json:"layout_name,omitempty"`
	Placeholders []DryRunPlaceholder `json:"placeholders"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// DryRunPlaceholder describes how a content field maps to a specific
// placeholder during dry-run validation.
type DryRunPlaceholder struct {
	PlaceholderID string `json:"placeholder_id"`
	ContentField  string `json:"content_field"`
	ContentType   string `json:"content_type"`
	Truncated     bool   `json:"truncated,omitempty"`
	TruncateAt    int    `json:"truncate_at,omitempty"`
	MaxChars      int    `json:"max_chars,omitempty"`
}

// DefaultPipeline is the production implementation of Pipeline.
type DefaultPipeline struct {
	generator generator.Generator
}

// NewPipeline creates a new Pipeline with default settings.
func NewPipeline() *DefaultPipeline {
	return NewPipelineWithGenerator(generator.NewGenerator())
}

// NewPipelineWithGenerator creates a new Pipeline with a custom generator.
// This allows injecting mock generators for testing.
func NewPipelineWithGenerator(gen generator.Generator) *DefaultPipeline {
	return &DefaultPipeline{
		generator: gen,
	}
}

// Convert implements the Pipeline interface.
// It orchestrates layout selection and PPTX generation.
// When req.Presentation is set, markdown parsing is skipped entirely.
func (p *DefaultPipeline) Convert(ctx context.Context, req ConvertRequest) (*ConvertResult, error) {
	var presentation *types.PresentationDefinition
	var effectiveAnalysis *types.TemplateAnalysis

	if req.Presentation == nil {
		return nil, fmt.Errorf("presentation is required; markdown parsing has been removed")
	}

	// Pre-parsed presentation provided.
	presentation = req.Presentation
	effectiveAnalysis = req.TemplateAnalysis

	// Apply theme overrides if specified.
	var themeWarnings []string
	if presentation.Metadata.ThemeOverride != nil && req.TemplateAnalysis != nil {
		analysisCopy := *req.TemplateAnalysis
		analysisCopy.Theme, themeWarnings = analysisCopy.Theme.ApplyOverride(presentation.Metadata.ThemeOverride)
		effectiveAnalysis = &analysisCopy
	}

	// Step 1.5: Auto-paginate overflowing slides (when enabled via frontmatter).
	// Uses template layout capacity to determine the bullet threshold instead
	// of a fixed default, so slides with more bullets than any layout supports
	// are split into continuation slides rather than being crammed.
	paginationWarnings, _ := pagination.PaginateWithLayouts(presentation, req.TemplateAnalysis.Layouts)

	// Step 1.55: Split slides whose content overflows any available layout.
	// This handles two cases:
	//   - Slot-based slides with more slots than template capacity
	//   - Standard slides with chart + body but no chart placeholder
	splitWarnings := SplitContentOverflow(presentation, req.TemplateAnalysis.Layouts)
	paginationWarnings = append(paginationWarnings, splitWarnings...)

	// Step 2: Convert slides to generator format with heuristic layout selection
	slideSpecs, slideWarnings, slideErrors, err := p.convertSlides(ctx, presentation, effectiveAnalysis, req.Partial || req.DryRun)
	if err != nil {
		return nil, &LayoutError{Err: err}
	}

	// Merge warnings from theme overrides, pagination, and layout selection
	var allWarnings []string
	allWarnings = append(allWarnings, themeWarnings...)
	allWarnings = append(allWarnings, paginationWarnings...)
	allWarnings = append(allWarnings, slideWarnings...)

	// Dry-run mode: skip PPTX generation, return validation results
	if req.DryRun {
		dryRunSlides := buildDryRunSlides(presentation, slideSpecs, effectiveAnalysis)
		return &ConvertResult{
			SlideCount:   len(slideSpecs),
			Warnings:     allWarnings,
			FailedSlides: slideErrors,
			DryRunSlides: dryRunSlides,
		}, nil
	}

	// Step 3: Generate PPTX
	genReq := generator.GenerationRequest{
		TemplatePath:          req.TemplatePath,
		OutputPath:            req.OutputPath,
		Slides:                slideSpecs,
		SVGStrategy:           req.SVGStrategy,
		SVGScale:              req.SVGScale,
		SVGNativeCompat:       req.SVGNativeCompat,
		MaxPNGWidth:           req.MaxPNGWidth,
		ExcludeTemplateSlides: req.ExcludeTemplateSlides,
		ThemeOverride:         presentation.Metadata.ThemeOverride,
		StrictFit:             req.StrictFit,
	}
	if effectiveAnalysis.Synthesis != nil {
		genReq.SyntheticFiles = effectiveAnalysis.Synthesis.SyntheticFiles
	}

	result, err := p.generator.Generate(ctx, genReq)
	if err != nil {
		return nil, &GenerationError{Err: err}
	}

	allWarnings = append(allWarnings, result.Warnings...)

	return &ConvertResult{
		OutputPath:   req.OutputPath,
		SlideCount:   result.SlideCount,
		Warnings:     allWarnings,
		FailedSlides: slideErrors,
	}, nil
}

// buildDryRunSlides constructs per-slide dry-run details from the slide specs
// and the template analysis. It matches each slide spec back to its layout
// metadata and content item mappings.
func buildDryRunSlides(
	presentation *types.PresentationDefinition,
	specs []generator.SlideSpec,
	analysis *types.TemplateAnalysis,
) []DryRunSlide {
	// Build layout lookup
	layoutByID := make(map[string]types.LayoutMetadata, len(analysis.Layouts))
	for _, l := range analysis.Layouts {
		layoutByID[l.ID] = l
	}

	// Build placeholder max_chars lookup per layout
	type phKey struct{ layoutID, phID string }
	phMaxChars := make(map[phKey]int)
	for _, l := range analysis.Layouts {
		for _, ph := range l.Placeholders {
			phMaxChars[phKey{l.ID, ph.ID}] = ph.MaxChars
		}
	}

	slides := make([]DryRunSlide, 0, len(specs))
	for i, spec := range specs {
		ds := DryRunSlide{
			SlideNumber: i + 1,
			LayoutID:    spec.LayoutID,
		}

		// Set title from the presentation definition when available
		if i < len(presentation.Slides) {
			ds.Title = presentation.Slides[i].Title
		}

		// Set layout name from template metadata
		if lm, ok := layoutByID[spec.LayoutID]; ok {
			ds.LayoutName = lm.Name
		}

		// Build placeholder details from content items
		for _, ci := range spec.Content {
			ph := DryRunPlaceholder{
				PlaceholderID: ci.PlaceholderID,
				ContentType:   string(ci.Type),
			}
			ph.MaxChars = phMaxChars[phKey{spec.LayoutID, ci.PlaceholderID}]
			ds.Placeholders = append(ds.Placeholders, ph)
		}

		slides = append(slides, ds)
	}

	return slides
}

// convertSlides delegates to the shared ConvertSlidesPartial function.
func (p *DefaultPipeline) convertSlides(
	ctx context.Context,
	presentation *types.PresentationDefinition,
	templateAnalysis *types.TemplateAnalysis,
	partial bool,
) ([]generator.SlideSpec, []string, []SlideError, error) {
	return ConvertSlidesPartial(presentation, templateAnalysis, partial)
}

// SlideError describes a per-slide failure during conversion.
type SlideError struct {
	SlideNumber int    // 1-based slide number
	Stage       string // "layout_selection", "content_routing"
	Err         error
}

func (e *SlideError) Error() string {
	return fmt.Sprintf("slide %d (%s): %v", e.SlideNumber, e.Stage, e.Err)
}

// ConvertSlides converts parsed slides to generator format with heuristic layout selection.
// It iterates over each slide, selects the best layout, and builds content items.
// When a slide has ::slotN:: markers, content is routed through the slot router
// instead of the standard content builder.
// Any per-slide error is fatal — the entire batch fails.
func ConvertSlides(
	presentation *types.PresentationDefinition,
	templateAnalysis *types.TemplateAnalysis,
) ([]generator.SlideSpec, []string, error) {
	specs, warnings, _, err := ConvertSlidesPartial(presentation, templateAnalysis, false)
	if err != nil {
		return nil, nil, err
	}
	return specs, warnings, nil
}

// ConvertSlidesPartial converts parsed slides to generator format with heuristic layout selection.
// When partial is true, per-slide errors are collected instead of aborting. Failed slides
// are replaced with error placeholder slides showing the error message.
// Returns: slide specs, warnings, per-slide errors, and a fatal error (if any).
func ConvertSlidesPartial(
	presentation *types.PresentationDefinition,
	templateAnalysis *types.TemplateAnalysis,
	partial bool,
) ([]generator.SlideSpec, []string, []SlideError, error) {
	// Assign section numbers to section divider slides. Templates design
	// section dividers with a decorative body placeholder (e.g., a large "01"
	// or "#" indicator). Without body text, this placeholder is cleared,
	// leaving ~30% of the slide blank. Setting the body to the section number
	// fills the placeholder with the template's intended design.
	sectionNum := 0
	for i := range presentation.Slides {
		if presentation.Slides[i].Type == types.SlideTypeSection {
			sectionNum++
			if presentation.Slides[i].Content.Body == "" {
				presentation.Slides[i].Content.Body = fmt.Sprintf("%02d", sectionNum)
			}
		}
	}

	specs := make([]generator.SlideSpec, 0, len(presentation.Slides))
	var allWarnings []string
	var slideErrors []SlideError

	// Build layout lookup by ID for slot routing
	layoutByID := make(map[string]types.LayoutMetadata, len(templateAnalysis.Layouts))
	for _, l := range templateAnalysis.Layouts {
		layoutByID[l.ID] = l
	}

	// Find a fallback layout for error placeholder slides.
	// Pick the first layout that has both a title and body placeholder.
	fallbackLayoutID := findFallbackLayout(templateAnalysis.Layouts)

	// Track layout usage for variety scoring
	usedLayouts := make(map[string]int)
	var previousLayoutID string

	for i, slide := range presentation.Slides {
		// Apply default transition from frontmatter when slide has no per-slide override
		if slide.Transition == "" && presentation.Metadata.Transition != "" {
			slide.Transition = presentation.Metadata.Transition
		}
		if slide.TransitionSpeed == "" && presentation.Metadata.TransitionSpeed != "" {
			slide.TransitionSpeed = presentation.Metadata.TransitionSpeed
		}

		spec, warnings, slideErr := convertSingleSlide(slide, i, len(presentation.Slides), templateAnalysis, layoutByID, usedLayouts, previousLayoutID)
		if slideErr != nil {
			if !partial {
				return nil, nil, nil, slideErr
			}
			slideErrors = append(slideErrors, SlideError{
				SlideNumber: i + 1,
				Stage:       classifySlideError(slideErr),
				Err:         slideErr,
			})
			specs = append(specs, makeErrorPlaceholderSlide(i+1, slideErr, fallbackLayoutID))
			continue
		}

		usedLayouts[spec.LayoutID]++
		previousLayoutID = spec.LayoutID
		allWarnings = append(allWarnings, warnings...)
		specs = append(specs, spec)
	}

	return specs, allWarnings, slideErrors, nil
}

// convertSingleSlide converts a single parsed slide to a generator SlideSpec.
// Returns the spec, any warnings, or an error if conversion fails.
//
// Uses ranked layout selection with a single-pass retry: if the primary layout
// would cause text overflow (detected via textfit pre-flight), the next-best
// layout is tried before falling back to truncation.
func convertSingleSlide(
	slide types.SlideDefinition,
	index, totalSlides int,
	templateAnalysis *types.TemplateAnalysis,
	layoutByID map[string]types.LayoutMetadata,
	usedLayouts map[string]int,
	previousLayoutID string,
) (generator.SlideSpec, []string, error) {
	selectionReq := layout.SelectionRequest{
		Slide:   slide,
		Layouts: templateAnalysis.Layouts,
		Context: layout.SelectionContext{
			Position:     index,
			TotalSlides:  totalSlides,
			PreviousType: previousLayoutID,
			UsedLayouts:  usedLayouts,
		},
		Theme: &templateAnalysis.Theme,
	}

	ranked, err := layout.SelectLayoutRanked(selectionReq, 2)
	if err != nil {
		return generator.SlideSpec{}, nil, fmt.Errorf("failed to select layout for slide %d: %w", index+1, err)
	}

	selection := ranked.Primary

	// Single-pass retry: if the primary layout would overflow, try the alternate.
	if len(ranked.Alternates) > 0 && estimateBodyOverflow(slide, selection, templateAnalysis.Layouts) {
		alt := ranked.Alternates[0]
		if !estimateBodyOverflow(slide, alt, templateAnalysis.Layouts) {
			slog.Info("layout retry: switching to alternate layout to avoid text overflow",
				slog.Int("slide_index", index),
				slog.String("original_layout", selection.LayoutID),
				slog.String("alternate_layout", alt.LayoutID),
			)
			selection = alt
		}
	}

	var warnings []string
	warnings = append(warnings, selection.Warnings...)

	var contentItems []generator.ContentItem

	if slide.HasSlots() {
		selectedLayout, ok := layoutByID[selection.LayoutID]
		if !ok {
			return generator.SlideSpec{}, nil, fmt.Errorf("layout '%s' not found in template for slide %d", selection.LayoutID, index+1)
		}

		slotResult, slotErr := generator.BuildSlotContentItems(
			slide.Slots,
			selectedLayout,
			generator.SlotPopulationConfig{Theme: &templateAnalysis.Theme},
		)
		if slotErr != nil {
			return generator.SlideSpec{}, nil, fmt.Errorf("failed to route slot content for slide %d: %w", index+1, slotErr)
		}

		contentItems = slotResult.ContentItems
		warnings = append(warnings, slotResult.Warnings...)

		// Also add title mapping from the standard content builder
		titleItems := generator.BuildContentItems(slide, selection.Mappings)
		for _, item := range titleItems {
			if item.Type == generator.ContentText {
				for _, m := range selection.Mappings {
					if m.ContentField == "title" && m.PlaceholderID == item.PlaceholderID {
						contentItems = append(contentItems, item)
						break
					}
				}
			}
		}
	} else {
		contentItems = generator.BuildContentItems(slide, selection.Mappings)
	}

	spec := generator.SlideSpec{
		LayoutID:        selection.LayoutID,
		Content:         contentItems,
		SpeakerNotes:    slide.SpeakerNotes,
		SourceNote:      slide.Source,
		Transition:      slide.Transition,
		TransitionSpeed: slide.TransitionSpeed,
		Build:           slide.Build,
	}

	return spec, warnings, nil
}

// classifySlideError determines the stage of a slide conversion error.
func classifySlideError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "select layout") {
		return "layout_selection"
	}
	return "content_routing"
}

// findFallbackLayout returns the ID of a layout suitable for error placeholder slides.
// It prefers layouts with both title and body placeholders; falls back to the first layout.
func findFallbackLayout(layouts []types.LayoutMetadata) string {
	// First pass: find a layout with title and body
	for _, l := range layouts {
		hasTitle, hasBody := false, false
		for _, ph := range l.Placeholders {
			switch ph.Type {
			case types.PlaceholderTitle:
				hasTitle = true
			case types.PlaceholderBody, types.PlaceholderContent:
				hasBody = true
			}
		}
		if hasTitle && hasBody {
			return l.ID
		}
	}
	// Fallback: first layout
	if len(layouts) > 0 {
		return layouts[0].ID
	}
	return "slideLayout1"
}

// makeErrorPlaceholderSlide creates a simple slide spec showing an error message.
func makeErrorPlaceholderSlide(slideNum int, slideErr error, fallbackLayoutID string) generator.SlideSpec {
	title := fmt.Sprintf("Error: Slide %d", slideNum)
	body := fmt.Sprintf("This slide failed to generate:\n\n%v", slideErr)

	return generator.SlideSpec{
		LayoutID: fallbackLayoutID,
		Content: []generator.ContentItem{
			{PlaceholderID: "title", Type: generator.ContentText, Value: title},
			{PlaceholderID: "body", Type: generator.ContentText, Value: body},
		},
	}
}
