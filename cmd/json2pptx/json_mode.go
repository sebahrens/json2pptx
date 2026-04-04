package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ConversionResult holds the result of a single file conversion.
type ConversionResult struct {
	InputPath    string
	OutputPath   string
	Success      bool
	Error        string
	SlideCount   int
	FailedSlides int
	Duration     time.Duration
}

// JSONInput represents the JSON input format for headless conversions.
// This allows direct specification of slides without markdown parsing.
type JSONInput struct {
	// Template is the template name (without .pptx extension)
	Template string `json:"template"`

	// OutputFilename is the desired output filename (optional, defaults to output.pptx)
	OutputFilename string `json:"output_filename,omitempty"`

	// Footer controls slide footer injection (optional, disabled by default)
	Footer *JSONFooter `json:"footer,omitempty"`

	// ThemeOverride allows per-deck theme color/font overrides
	ThemeOverride *ThemeInput `json:"theme_override,omitempty"`

	// Slides contains the slide specifications
	Slides []JSONSlide `json:"slides"`
}

// JSONFooter configures slide footer injection.
type JSONFooter struct {
	// Enabled is the master switch — when false, no footers are injected
	Enabled bool `json:"enabled"`

	// LeftText is the left footer text (e.g., "Acme Corp | Confidential")
	LeftText string `json:"left_text,omitempty"`
}

// JSONSlide represents a single slide in JSON input format.
type JSONSlide struct {
	// LayoutID is the layout identifier (e.g., "slideLayout1", "Title Slide")
	LayoutID string `json:"layout_id"`

	// Content contains the content items for placeholders
	Content []JSONContentItem `json:"content"`

	// SpeakerNotes is optional speaker notes text
	SpeakerNotes string `json:"speaker_notes,omitempty"`

	// Source is optional source attribution text
	Source string `json:"source,omitempty"`

	// Transition is the slide transition type: "fade", "push", "wipe", etc.
	Transition string `json:"transition,omitempty"`

	// TransitionSpeed is the transition speed: "slow", "med", "fast"
	TransitionSpeed string `json:"transition_speed,omitempty"`

	// Build is the build animation: "bullets" for one-by-one bullet reveal
	Build string `json:"build,omitempty"`
}

// JSONContentItem represents content to place in a placeholder.
type JSONContentItem struct {
	// PlaceholderID identifies the target placeholder
	PlaceholderID string `json:"placeholder_id"`

	// Type is the content type: "text", "bullets", "image", or "chart"
	Type string `json:"type"`

	// Value is the content value (type depends on Type field)
	// - text: string
	// - bullets: []string
	// - image: {path: string, alt: string}
	// - chart: {type: string, title: string, data: {label: value}}
	Value json.RawMessage `json:"value"`

	// FontSize overrides the template's default font size (in points, e.g., 72).
	FontSize *float64 `json:"font_size,omitempty"`
}

// JSONOutput represents the JSON output for headless mode.
type JSONOutput struct {
	Success     bool          `json:"success"`
	OutputPath  string        `json:"output_path,omitempty"`
	SlideCount  int           `json:"slide_count,omitempty"`
	DurationMs  int64         `json:"duration_ms,omitempty"`
	Error       string        `json:"error,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
	SlideErrors []SlideError  `json:"slide_errors,omitempty"`
	Quality     *QualityScore `json:"quality,omitempty"`
}

// SlideError describes a render-time failure for a specific slide.
// These are populated from generator.MediaFailure records when charts,
// diagrams, images, or tables fail to render during PPTX generation.
type SlideError struct {
	SlideNumber int    `json:"slide_number"`
	ContentType string `json:"content_type"`           // "diagram", "image", "table"
	DiagramType string `json:"diagram_type,omitempty"` // e.g., "pie_chart", "timeline"
	Error       string `json:"error"`
	Fallback    string `json:"fallback,omitempty"` // what was done instead: "placeholder_image", "skipped"
}

// QualityScore provides an overall quality estimate for the generated deck.
type QualityScore struct {
	Score       float64        `json:"score"`                    // 0.0-1.0 overall quality estimate
	SlideScores []SlideQuality `json:"slide_scores,omitempty"`  // per-slide breakdown
	Issues      []string       `json:"issues,omitempty"`        // quality concerns
}

// SlideQuality provides quality metrics for a single slide.
type SlideQuality struct {
	SlideNumber int      `json:"slide_number"`
	Score       float64  `json:"score"`           // 0.0-1.0
	Issues      []string `json:"issues,omitempty"`
}

// parseJSONInput reads JSON from a file or stdin, applies patch operations if present,
// applies the template override, and validates required fields.
func parseJSONInput(jsonPath, templateOverride string) (*PresentationInput, error) {
	var inputData []byte
	var err error

	if jsonPath == "-" {
		inputData, err = io.ReadAll(os.Stdin)
	} else {
		inputData, err = os.ReadFile(jsonPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON input: %w", err)
	}

	var input PresentationInput
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(inputData, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			return nil, fmt.Errorf("failed to apply patch: %w", patchErr)
		}
		input = *patched
	} else {
		if err := json.Unmarshal(inputData, &input); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	if templateOverride != "" {
		input.Template = strings.TrimSuffix(templateOverride, ".pptx")
	}

	if input.Template == "" {
		return nil, fmt.Errorf("template is required: use -template flag or set \"template\" in JSON input")
	}
	if len(input.Slides) == 0 {
		return nil, fmt.Errorf("at least one slide is required")
	}

	return &input, nil
}

// loadRunConfig loads configuration from configPath (or defaults) and applies CLI overrides.
func loadRunConfig(configPath, templatesDir, outputDir string, chartPNG bool) (config.Config, error) {
	cfg := config.DefaultConfig()
	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return cfg, fmt.Errorf("failed to load config: %w", err)
		}
	}
	if templatesDir != "" {
		cfg.Templates.Dir = templatesDir
	}
	if outputDir != "" {
		cfg.Storage.OutputDir = outputDir
	}
	if chartPNG {
		slog.Warn("--chart-png is deprecated and will be removed in a future release; native SVG is the default strategy")
		cfg.SVG.Strategy = types.SVGStrategyPNG
	}
	return cfg, nil
}

// analyzeTemplateLayouts opens a template, parses layouts, synthesizes missing layouts,
// normalizes placeholder names, and returns the metadata needed for slide conversion.
func analyzeTemplateLayouts(templatePath string) ([]types.LayoutMetadata, map[string][]byte, int64, int64) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return nil, nil, 0, 0
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, nil, 0, 0
	}

	theme := template.ParseTheme(reader)
	slideWidth, slideHeight := template.ParseSlideDimensions(reader)
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		SlideWidth:   slideWidth,
		SlideHeight:  slideHeight,
		Layouts:      layouts,
		Theme:        theme,
	}
	template.SynthesizeIfNeeded(reader, analysis)

	// Normalize placeholder names to canonical form (body, body_2, body_3, etc.)
	normalizedFiles, normErr := template.NormalizeLayoutFiles(reader, analysis.Layouts)
	if normErr == nil && len(normalizedFiles) > 0 {
		if analysis.Synthesis == nil {
			analysis.Synthesis = &types.SynthesisManifest{
				SyntheticFiles: normalizedFiles,
			}
		} else {
			for path, data := range normalizedFiles {
				analysis.Synthesis.SyntheticFiles[path] = data
			}
		}
	}

	var syntheticFiles map[string][]byte
	if analysis.Synthesis != nil {
		syntheticFiles = analysis.Synthesis.SyntheticFiles
	}
	return analysis.Layouts, syntheticFiles, slideWidth, slideHeight
}

// runJSONMode processes JSON input and generates PPTX.
func runJSONMode(jsonPath, jsonOutputPath, templatesDir, outputDir, configPath string, verbose bool, chartPNG bool, templateOverride string) error {
	startTime := time.Now()

	// Parse and validate JSON input
	input, err := parseJSONInput(jsonPath, templateOverride)
	if err != nil {
		return writeJSONError(jsonOutputPath, err)
	}

	// Load configuration with CLI overrides
	cfg, err := loadRunConfig(configPath, templatesDir, outputDir, chartPNG)
	if err != nil {
		return writeJSONError(jsonOutputPath, err)
	}

	// Create output directory
	if err := os.MkdirAll(cfg.Storage.OutputDir, 0755); err != nil {
		return writeJSONError(jsonOutputPath, fmt.Errorf("failed to create output directory: %w", err))
	}

	// Resolve template path using search path (flag, env, home, cwd, embedded)
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, cfg.Templates.Dir)
	if err != nil {
		return writeJSONError(jsonOutputPath, fmt.Errorf("%s", templateNotFoundError(input.Template, cfg.Templates.Dir)))
	}
	defer templateCleanup()

	// Analyze template for layout metadata, synthetic files, and dimensions
	templateLayouts, syntheticFiles, slideWidth, slideHeight := analyzeTemplateLayouts(templatePath)

	// Resolve canonical layout names (e.g. "title", "content", "closing") to
	// concrete layout IDs using tag-based matching against the target template.
	if len(templateLayouts) > 0 {
		for i := range input.Slides {
			if input.Slides[i].LayoutID != "" {
				if resolved, ok := layout.ResolveCanonicalLayoutID(input.Slides[i].LayoutID, templateLayouts); ok {
					input.Slides[i].LayoutID = resolved
				}
			}
		}
	}

	// Resolve any URL references (icon.url, image.url, background.url) by downloading
	// them to a session-scoped cache with SSRF protection.
	if hasURLReferences(input.Slides) {
		resolver, resolverErr := resource.NewResolver(resource.ResolverOptions{})
		if resolverErr != nil {
			return writeJSONError(jsonOutputPath, fmt.Errorf("resource resolver: %w", resolverErr))
		}
		defer resolver.Close()
		if resolverErr := resolveURLs(input.Slides, resolver); resolverErr != nil {
			return writeJSONError(jsonOutputPath, fmt.Errorf("URL resolution: %w", resolverErr))
		}
	}

	// Resolve relative icon paths (icon.path fields) against the JSON input directory.
	// This must happen before convertPresentationSlides so that IconSpec.Path is absolute.
	if jsonPath != "-" {
		inputDir := filepath.Dir(jsonPath)
		if err := resolveIconPaths(input.Slides, inputDir); err != nil {
			return writeJSONError(jsonOutputPath, fmt.Errorf("icon path error: %w", err))
		}
	}

	// Convert typed slides to generator specs (uses templateLayouts for auto-layout selection)
	slideSpecs, err := convertPresentationSlides(input.Slides, templateLayouts, slideWidth, slideHeight)
	if err != nil {
		return writeJSONError(jsonOutputPath, fmt.Errorf("invalid slide specification: %w", err))
	}

	// Pre-validate chart/diagram data structures via svggen Validate().
	// Issues are collected as warnings so generation still proceeds.
	inputWarnings := validateSlidesChartData(input.Slides)

	// Determine output filename — sanitize to prevent path traversal.
	outputFilename := sanitizeOutputFilename(input.OutputFilename)
	outputPath := filepath.Join(cfg.Storage.OutputDir, outputFilename)

	// Generate PPTX
	// ExcludeTemplateSlides=true removes the template's example slides from output,
	// so only our generated slides are in the final PPTX
	genReq := generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slideSpecs,
		SVGStrategy:           string(cfg.SVG.Strategy),
		SVGScale:              cfg.SVG.Scale,
		SVGNativeCompat:       string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:           cfg.SVG.MaxPNGWidth,
		ExcludeTemplateSlides: true,
		SyntheticFiles:        syntheticFiles,
	}

	// Wire footer configuration
	if input.Footer != nil && input.Footer.Enabled {
		genReq.Footer = &generator.FooterConfig{
			Enabled:  true,
			LeftText: input.Footer.LeftText,
		}
	}

	// Wire theme override
	if input.ThemeOverride != nil {
		genReq.ThemeOverride = input.ThemeOverride.ToThemeOverride()
	}

	result, err := generator.Generate(context.Background(), genReq)
	if err != nil {
		return writeJSONError(jsonOutputPath, fmt.Errorf("failed to generate PPTX: %w", err))
	}

	// Merge input-layer warnings with generation warnings
	allWarnings := append(inputWarnings, result.Warnings...)

	// Convert structured media failures to per-slide error details
	slideErrors := convertMediaFailures(result.MediaFailures)

	// Build success output
	output := JSONOutput{
		Success:     true,
		OutputPath:  outputPath,
		SlideCount:  result.SlideCount,
		DurationMs:  time.Since(startTime).Milliseconds(),
		Warnings:    allWarnings,
		SlideErrors: slideErrors,
		Quality:     computeQualityScore(input.Slides, allWarnings),
	}

	// Write JSON output if requested
	if jsonOutputPath != "" {
		return writeJSONOutput(jsonOutputPath, output)
	}

	// Otherwise print summary to stdout
	slog.Info("JSON conversion complete",
		"output", outputPath,
		"slides", result.SlideCount,
		"duration_ms", output.DurationMs,
	)

	return nil
}

// convertJSONSlides converts JSON slide definitions to generator specs.
// Deprecated: Use convertPresentationSlides for typed field support.
func convertJSONSlides(jsonSlides []JSONSlide) ([]generator.SlideSpec, error) {
	specs := make([]generator.SlideSpec, 0, len(jsonSlides))

	for i, jsonSlide := range jsonSlides {
		if jsonSlide.LayoutID == "" {
			return nil, fmt.Errorf("slide %d: layout_id is required", i+1)
		}

		slideType := inferJSONSlideType(jsonSlide)
		contentItems, err := convertJSONContent(jsonSlide.Content, i+1, slideType)
		if err != nil {
			return nil, err
		}

		specs = append(specs, generator.SlideSpec{
			LayoutID:        jsonSlide.LayoutID,
			Content:         contentItems,
			SpeakerNotes:    jsonSlide.SpeakerNotes,
			SourceNote:      jsonSlide.Source,
			Transition:      jsonSlide.Transition,
			TransitionSpeed: jsonSlide.TransitionSpeed,
			Build:           jsonSlide.Build,
		})
	}

	return specs, nil
}

// convertPresentationSlides converts typed SlideInput definitions to generator specs.
// This is the primary conversion path that supports both typed fields (text_value,
// bullets_value, etc.) and legacy json.RawMessage Value field.
// When layouts is non-empty and a slide omits layout_id, auto-layout selection is used.
func convertPresentationSlides(slides []SlideInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) ([]generator.SlideSpec, error) { //nolint:gocognit,gocyclo
	specs := make([]generator.SlideSpec, 0, len(slides))

	// Track layout usage for variety scoring during auto-selection
	var usedLayouts map[string]int
	if len(layouts) > 0 {
		usedLayouts = make(map[string]int)
	}

	// Assign section numbers to section divider slides whose body placeholder
	// would otherwise be cleared, leaving blank whitespace.
	sectionNum := 0
	for i := range slides {
		if inferSlideType(slides[i]) == types.SlideTypeSection {
			sectionNum++
			// Check if any content item targets "body"
			hasBody := false
			for _, ci := range slides[i].Content {
				if ci.PlaceholderID == "body" {
					hasBody = true
					break
				}
			}
			if !hasBody {
				sectionNumStr := fmt.Sprintf("%02d", sectionNum)
				slides[i].Content = append(slides[i].Content, ContentInput{
					PlaceholderID: "body",
					Type:          "text",
					TextValue:     &sectionNumStr,
				})
			}
		}
	}

	for i, slide := range slides {
		if slide.LayoutID == "" {
			if len(layouts) == 0 {
				return nil, fmt.Errorf("slide %d: layout_id is required (no template layouts available for auto-selection)", i+1)
			}

			// Auto-select layout using heuristic engine
			slideDef := jsonSlideToDefinition(slide)
			req := layout.SelectionRequest{
				Slide:   slideDef,
				Layouts: layouts,
				Context: layout.SelectionContext{
					Position:    i,
					TotalSlides: len(slides),
					UsedLayouts: usedLayouts,
				},
			}
			if i > 0 {
				req.Context.PreviousType = specs[i-1].LayoutID
			}

			result, err := layout.SelectLayout(req)
			if err != nil {
				return nil, fmt.Errorf("slide %d: auto-layout selection failed: %w", i+1, err)
			}

			slide.LayoutID = result.LayoutID
			usedLayouts[result.LayoutID]++

			slog.Info("auto-layout selected",
				slog.Int("slide", i+1),
				slog.String("layout_id", result.LayoutID),
				slog.String("slide_type", string(slideDef.Type)),
				slog.Float64("confidence", result.Confidence),
			)

			// Auto-map placeholder IDs for items that don't have one
			var selectedLayout *types.LayoutMetadata
			for j := range layouts {
				if layouts[j].ID == result.LayoutID {
					selectedLayout = &layouts[j]
					break
				}
			}
			if selectedLayout != nil {
				slide.Content = autoMapPlaceholders(slide.Content, *selectedLayout)
			}
		} else {
			if usedLayouts != nil {
				// Track explicitly-specified layouts too, for variety scoring
				usedLayouts[slide.LayoutID]++
			}
			// Resolve virtual placeholder IDs even for explicit layout IDs
			if hasVirtualPlaceholders(slide.Content) {
				for j := range layouts {
					if layouts[j].ID == slide.LayoutID {
						slide.Content = autoMapPlaceholders(slide.Content, layouts[j])
						break
					}
				}
			}
		}

		contentItems, err := convertPresentationContent(slide.Content, i+1, inferSlideType(slide))
		if err != nil {
			return nil, err
		}

		spec := generator.SlideSpec{
			LayoutID:        slide.LayoutID,
			Content:         contentItems,
			SpeakerNotes:    slide.SpeakerNotes,
			SourceNote:      slide.Source,
			Transition:      slide.Transition,
			TransitionSpeed: slide.TransitionSpeed,
			Build:           slide.Build,
		}

		// Convert background image spec
		if slide.Background != nil && slide.Background.Image != "" {
			spec.Background = &generator.BackgroundImage{
				Path: slide.Background.Image,
				Fit:  slide.Background.Fit,
			}
		}

		// Resolve shape_grid into raw p:sp XML fragments
		if slide.ShapeGrid != nil {
			// Virtual layout resolution: derive layout and bounds from template
			var overrideBounds *pptx.RectEmu
			var contentZone *shapegrid.ContentZone

			// Always compute ContentZone from template layouts for shape_grid slides
			// so that DefaultBounds can respect the actual title height, even when
			// the slide has an explicit layout_id and doesn't need virtual resolution.
			if len(layouts) > 0 {
				if vl := resolveVirtualLayout(layouts, slideWidth, slideHeight); vl != nil {
					contentZone = vl.Zone
					if needsVirtualLayout(slide) {
						spec.LayoutID = vl.LayoutID
						overrideBounds = &vl.Bounds
						slog.Info("virtual layout resolved",
							slog.Int("slide", i+1),
							slog.String("layout_id", vl.LayoutID),
						)
					}
				}
			}

			// Use a high-start allocator to avoid colliding with template shape IDs.
			alloc := &pptx.ShapeIDAllocator{}
			alloc.SetMinID(200)
			gridResult, err := resolveShapeGrid(slide.ShapeGrid, alloc, overrideBounds, contentZone, slideWidth, slideHeight)
			if err != nil {
				return nil, fmt.Errorf("slide %d: shape_grid: %w", i+1, err)
			}
			if gridResult != nil {
				spec.RawShapeXML = gridResult.Shapes
				spec.IconInserts = gridResult.IconInserts
				spec.ImageInserts = gridResult.ImageInserts
			}
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// convertPresentationContent converts typed ContentInput items to generator content items.
// Uses ContentInput.ResolveValue() to support both typed fields and legacy Value.
func convertPresentationContent(content []ContentInput, slideNum int, slideType types.SlideType) ([]generator.ContentItem, error) { //nolint:gocognit,gocyclo
	items := make([]generator.ContentItem, 0, len(content))

	for j, ci := range content {
		if ci.PlaceholderID == "" {
			return nil, fmt.Errorf("slide %d, content %d: placeholder_id is required", slideNum, j+1)
		}
		if ci.Type == "" {
			return nil, fmt.Errorf("slide %d, content %d: type is required", slideNum, j+1)
		}

		item := generator.ContentItem{
			PlaceholderID: ci.PlaceholderID,
		}

		// Apply font size override (convert points to hundredths of a point).
		if ci.FontSize != nil && *ci.FontSize > 0 {
			item.FontSize = int(*ci.FontSize * 100)
		}

		resolved, err := ci.ResolveValue()
		if err != nil {
			return nil, fmt.Errorf("slide %d, content %d: %w", slideNum, j+1, err)
		}

		switch ci.Type {
		case "text":
			// Section divider titles use ContentSectionTitle so the generator
			// preserves the template's large font size instead of capping it.
			// Check both the original virtual ID "title" and section slide type:
			// autoMapPlaceholders may have already resolved "title" to the actual
			// placeholder index (e.g., "13"), so we also accept any text item on
			// a section slide — section dividers only contain title text.
			//
			// Title slide titles use ContentTitleSlideTitle to preserve the
			// template's ctrTitle font size (typically 40-60pt) and centered
			// alignment instead of capping to 24pt body-text size.
			if slideType == types.SlideTypeSection {
				item.Type = generator.ContentSectionTitle
			} else if slideType == types.SlideTypeTitle && (isTitlePlaceholderID(ci.PlaceholderID) || isLikelySubtitle(ci.PlaceholderID)) {
				item.Type = generator.ContentTitleSlideTitle
			} else {
				item.Type = generator.ContentText
			}
			text, ok := resolved.(string)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: text resolved to %T, want string", slideNum, j+1, resolved)
			}
			item.Value = text

		case "bullets":
			item.Type = generator.ContentBullets
			bullets, ok := resolved.([]string)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: bullets resolved to %T, want []string", slideNum, j+1, resolved)
			}
			item.Value = bullets

		case "body_and_bullets":
			item.Type = generator.ContentBodyAndBullets
			input, ok := resolved.(*BodyAndBulletsInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: body_and_bullets resolved to %T, want *BodyAndBulletsInput", slideNum, j+1, resolved)
			}
			item.Value = generator.BodyAndBulletsContent{
				Body:         input.Body,
				Bullets:      input.Bullets,
				TrailingBody: input.TrailingBody,
			}

		case "bullet_groups":
			item.Type = generator.ContentBulletGroups
			input, ok := resolved.(*BulletGroupsInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: bullet_groups resolved to %T, want *BulletGroupsInput", slideNum, j+1, resolved)
			}
			item.Value = convertBulletGroupsInput(input)

		case "table":
			item.Type = generator.ContentTable
			input, ok := resolved.(*TableInput)
			if !ok {
				return nil, fmt.Errorf("slide %d, content %d: table resolved to %T, want *TableInput", slideNum, j+1, resolved)
			}
			item.Value = input.ToTableSpec()

		case "chart":
			item.Type = generator.ContentDiagram
			if resolved != nil {
				// Typed field path: ChartValue was set
				chart, ok := resolved.(*types.ChartSpec) //nolint:staticcheck // backward compatibility
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: chart resolved to %T, want *types.ChartSpec", slideNum, j+1, resolved)
				}
				item.Value = chart.ToDiagramSpec()
			} else {
				// Legacy path: parse from Value json.RawMessage
				var chart types.ChartSpec                                    //nolint:staticcheck // backward compatibility
				if err := json.Unmarshal(ci.Value, &chart); err != nil { //nolint:staticcheck // backward compatibility
					return nil, fmt.Errorf("slide %d, content %d: invalid chart value: %w", slideNum, j+1, err)
				}
				if chart.Type == "" {
					return nil, fmt.Errorf("slide %d, content %d: chart type is required", slideNum, j+1)
				}
				item.Value = chart.ToDiagramSpec()
			}

		case "diagram":
			item.Type = generator.ContentDiagram
			if resolved != nil {
				// Typed field path: DiagramValue was set
				diagram, ok := resolved.(*types.DiagramSpec)
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: diagram resolved to %T, want *types.DiagramSpec", slideNum, j+1, resolved)
				}
				item.Value = diagram
			} else {
				// Legacy path: parse from Value json.RawMessage
				var diagram types.DiagramSpec
				if err := json.Unmarshal(ci.Value, &diagram); err != nil {
					return nil, fmt.Errorf("slide %d, content %d: invalid diagram value: %w", slideNum, j+1, err)
				}
				if diagram.Type == "" {
					return nil, fmt.Errorf("slide %d, content %d: diagram type is required", slideNum, j+1)
				}
				item.Value = &diagram
			}

		case "image":
			item.Type = generator.ContentImage
			if resolved != nil {
				// Typed field path: ImageValue was set
				img, ok := resolved.(*ImageInput)
				if !ok {
					return nil, fmt.Errorf("slide %d, content %d: image resolved to %T, want *ImageInput", slideNum, j+1, resolved)
				}
				if img.Path == "" {
					return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
				}
				item.Value = generator.ImageContent{
					Path: img.Path,
					Alt:  img.Alt,
				}
			} else {
				// Legacy path: parse from Value json.RawMessage
				var img struct {
					Path string `json:"path"`
					Alt  string `json:"alt"`
				}
				if err := json.Unmarshal(ci.Value, &img); err != nil {
					return nil, fmt.Errorf("slide %d, content %d: invalid image value: %w", slideNum, j+1, err)
				}
				if img.Path == "" {
					return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
				}
				item.Value = generator.ImageContent{
					Path: img.Path,
					Alt:  img.Alt,
				}
			}

		default:
			return nil, fmt.Errorf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)", slideNum, j+1, ci.Type)
		}

		items = append(items, item)
	}

	// Merge text items targeting the same placeholder (e.g., template_2 section
	// dividers where both "title" and "body" resolve to the same body placeholder).
	// Without merging, the second populateShapeText call overwrites the first.
	items = mergeTextItemsSamePlaceholder(items)

	return items, nil
}

// validateSlidesChartData runs svggen Validate() on chart/diagram content items
// across all slides, returning any validation issues as warning strings.
// This catches structural data problems (e.g., flat map for waterfall charts)
// before render time, surfacing them in the JSON output warnings array.
func validateSlidesChartData(slides []SlideInput) []string {
	var warnings []string
	for i, slide := range slides {
		for j, ci := range slide.Content {
			if ci.Type != "chart" && ci.Type != "diagram" {
				continue
			}
			warnings = append(warnings, validateContentDiagramData(ci, i+1, j+1)...)
		}
	}
	return warnings
}

// validateContentDiagramData resolves a chart/diagram ContentInput to a DiagramSpec
// and validates its data structure via the svggen registry. Returns warning
// strings for validation failures and flat-map auto-conversions.
func validateContentDiagramData(ci ContentInput, slideNum, contentNum int) []string {
	resolved, err := ci.ResolveValue()
	if err != nil {
		return nil // parse errors are caught elsewhere
	}

	var spec *types.DiagramSpec

	switch ci.Type {
	case "chart":
		if resolved != nil {
			chart, ok := resolved.(*types.ChartSpec) //nolint:staticcheck // backward compat
			if !ok {
				return nil
			}
			spec = chart.ToDiagramSpec()
		} else if len(ci.Value) > 0 {
			var chart types.ChartSpec //nolint:staticcheck // backward compat
			if err := json.Unmarshal(ci.Value, &chart); err != nil {
				return nil
			}
			spec = chart.ToDiagramSpec()
		}
	case "diagram":
		if resolved != nil {
			diagram, ok := resolved.(*types.DiagramSpec)
			if !ok {
				return nil
			}
			spec = diagram
		} else if len(ci.Value) > 0 {
			var diagram types.DiagramSpec
			if err := json.Unmarshal(ci.Value, &diagram); err != nil {
				return nil
			}
			spec = &diagram
		}
	}

	return validateDiagramSpecAll(spec, slideNum, contentNum)
}

// validateDiagramSpecAll checks a DiagramSpec for both flat-map conversion warnings
// and svggen data validation issues. Returns all warnings found.
func validateDiagramSpecAll(spec *types.DiagramSpec, slideNum, contentNum int) []string {
	if spec == nil || spec.Type == "" {
		return nil
	}

	var warnings []string

	// Collect flat-map auto-conversion warnings from buildChartData.
	for _, w := range spec.Warnings {
		warnings = append(warnings, fmt.Sprintf("slide %d, content %d: %s", slideNum, contentNum, w))
	}

	// Run svggen structural validation.
	if w := validateDiagramSpec(spec, slideNum, contentNum); w != "" {
		warnings = append(warnings, w)
	}

	return warnings
}

// validateDiagramSpec checks a DiagramSpec's data against the svggen registry's
// diagram-specific Validate() method. Returns a warning string or "".
func validateDiagramSpec(spec *types.DiagramSpec, slideNum, contentNum int) string {
	if spec == nil || spec.Type == "" {
		return ""
	}

	req := &svggen.RequestEnvelope{
		Type: spec.Type,
		Data: spec.Data,
	}

	d := svggen.DefaultRegistry().Get(req.Type)
	if d == nil {
		return "" // unknown types are caught elsewhere
	}

	if err := d.Validate(req); err != nil {
		return fmt.Sprintf("slide %d, content %d: %s data validation: %v",
			slideNum, contentNum, spec.Type, err)
	}
	return ""
}

// mergeTextItemsSamePlaceholder combines text/section-title items that target
// the same placeholder ID. The first item's text becomes "first\nsecond".
// Non-text items and items with unique placeholder IDs are left unchanged.
func mergeTextItemsSamePlaceholder(items []generator.ContentItem) []generator.ContentItem {
	if len(items) <= 1 {
		return items
	}

	// Find placeholder IDs that appear more than once for text items
	seen := make(map[string]int) // placeholder_id -> index of first text item
	for i, item := range items {
		if item.Type != generator.ContentText && item.Type != generator.ContentSectionTitle {
			continue
		}
		if _, ok := seen[item.PlaceholderID]; !ok {
			seen[item.PlaceholderID] = i
		}
	}

	// Merge duplicates
	merged := make([]generator.ContentItem, 0, len(items))
	skip := make(map[int]bool)
	for i, item := range items {
		if skip[i] {
			continue
		}
		if item.Type != generator.ContentText && item.Type != generator.ContentSectionTitle {
			merged = append(merged, item)
			continue
		}
		firstIdx := seen[item.PlaceholderID]
		if firstIdx != i {
			// This is a duplicate — merge into the first occurrence
			continue
		}
		// Collect all text for this placeholder
		text, _ := item.Value.(string)
		for j := i + 1; j < len(items); j++ {
			if items[j].PlaceholderID == item.PlaceholderID &&
				(items[j].Type == generator.ContentText || items[j].Type == generator.ContentSectionTitle) {
				if addText, ok := items[j].Value.(string); ok && addText != "" {
					text += "\n" + addText
				}
				skip[j] = true
			}
		}
		item.Value = text
		merged = append(merged, item)
	}
	return merged
}

// convertJSONContent converts JSON content items to generator content items.
func convertJSONContent(jsonContent []JSONContentItem, slideNum int, slideType types.SlideType) ([]generator.ContentItem, error) { //nolint:gocognit,gocyclo
	items := make([]generator.ContentItem, 0, len(jsonContent))

	for j, jsonItem := range jsonContent {
		if jsonItem.PlaceholderID == "" {
			return nil, fmt.Errorf("slide %d, content %d: placeholder_id is required", slideNum, j+1)
		}
		if jsonItem.Type == "" {
			return nil, fmt.Errorf("slide %d, content %d: type is required", slideNum, j+1)
		}

		item := generator.ContentItem{
			PlaceholderID: jsonItem.PlaceholderID,
		}

		// Apply font size override (convert points to hundredths of a point).
		if jsonItem.FontSize != nil && *jsonItem.FontSize > 0 {
			item.FontSize = int(*jsonItem.FontSize * 100)
		}

		switch jsonItem.Type {
		case "text":
			// Title slide titles use ContentTitleSlideTitle to preserve the
			// template's ctrTitle font size (typically 40-60pt) and centered
			// alignment instead of capping to 24pt body-text size.
			if slideType == types.SlideTypeTitle && (isTitlePlaceholderID(jsonItem.PlaceholderID) || isLikelySubtitle(jsonItem.PlaceholderID)) {
				item.Type = generator.ContentTitleSlideTitle
			} else {
				item.Type = generator.ContentText
			}
			var text string
			if err := json.Unmarshal(jsonItem.Value, &text); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid text value: %w", slideNum, j+1, err)
			}
			item.Value = text

		case "bullets":
			item.Type = generator.ContentBullets
			var bullets []string
			if err := json.Unmarshal(jsonItem.Value, &bullets); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid bullets value (expected array of strings): %w", slideNum, j+1, err)
			}
			item.Value = bullets

		case "image":
			item.Type = generator.ContentImage
			var img struct {
				Path string `json:"path"`
				Alt  string `json:"alt"`
			}
			if err := json.Unmarshal(jsonItem.Value, &img); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid image value: %w", slideNum, j+1, err)
			}
			if img.Path == "" {
				return nil, fmt.Errorf("slide %d, content %d: image path is required", slideNum, j+1)
			}
			item.Value = generator.ImageContent{
				Path: img.Path,
				Alt:  img.Alt,
			}

		case "chart":
			item.Type = generator.ContentDiagram
			var chart types.ChartSpec //nolint:staticcheck // backward compat
			if err := json.Unmarshal(jsonItem.Value, &chart); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid chart value: %w", slideNum, j+1, err)
			}
			if chart.Type == "" {
				return nil, fmt.Errorf("slide %d, content %d: chart type is required", slideNum, j+1)
			}
			// Convert ChartSpec to DiagramSpec at the API boundary
			item.Value = chart.ToDiagramSpec()

		case "diagram":
			// Diagram content type accepts DiagramSpec directly with map[string]any data.
			// Use this for complex diagram types (swot, org_chart, timeline, etc.)
			// that need structured data passed directly as map[string]any.
			item.Type = generator.ContentDiagram
			var diagram types.DiagramSpec
			if err := json.Unmarshal(jsonItem.Value, &diagram); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid diagram value: %w", slideNum, j+1, err)
			}
			if diagram.Type == "" {
				return nil, fmt.Errorf("slide %d, content %d: diagram type is required", slideNum, j+1)
			}
			item.Value = &diagram

		case "table":
			item.Type = generator.ContentTable
			var tableInput TableInput
			if err := json.Unmarshal(jsonItem.Value, &tableInput); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid table value: %w", slideNum, j+1, err)
			}
			item.Value = tableInput.ToTableSpec()

		case "body_and_bullets":
			item.Type = generator.ContentBodyAndBullets
			var input BodyAndBulletsInput
			if err := json.Unmarshal(jsonItem.Value, &input); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid body_and_bullets value: %w", slideNum, j+1, err)
			}
			item.Value = generator.BodyAndBulletsContent{
				Body:         input.Body,
				Bullets:      input.Bullets,
				TrailingBody: input.TrailingBody,
			}

		case "bullet_groups":
			item.Type = generator.ContentBulletGroups
			var input BulletGroupsInput
			if err := json.Unmarshal(jsonItem.Value, &input); err != nil {
				return nil, fmt.Errorf("slide %d, content %d: invalid bullet_groups value: %w", slideNum, j+1, err)
			}
			item.Value = convertBulletGroupsInput(&input)

		default:
			return nil, fmt.Errorf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)", slideNum, j+1, jsonItem.Type)
		}

		items = append(items, item)
	}

	return items, nil
}

// convertBulletGroupsInput converts a BulletGroupsInput to generator.BulletGroupsContent.
func convertBulletGroupsInput(input *BulletGroupsInput) generator.BulletGroupsContent {
	groups := make([]generator.BulletGroup, len(input.Groups))
	for i, g := range input.Groups {
		groups[i] = generator.BulletGroup{
			Header:  g.Header,
			Body:    g.Body,
			Bullets: g.Bullets,
		}
	}
	return generator.BulletGroupsContent{
		Body:         input.Body,
		Groups:       groups,
		TrailingBody: input.TrailingBody,
	}
}

// writeJSONError writes an error response to JSON output or returns the error.
func writeJSONError(jsonOutputPath string, err error) error {
	if jsonOutputPath == "" {
		return err
	}

	output := JSONOutput{
		Success: false,
		Error:   err.Error(),
	}

	if writeErr := writeJSONOutput(jsonOutputPath, output); writeErr != nil {
		return fmt.Errorf("%v (also failed to write JSON output: %v)", err, writeErr)
	}

	return err
}

// computeQualityScore evaluates the quality of a set of slides and
// returns a QualityScore. It can be used by both JSON mode (json_mode.go)
// and markdown mode (generate.go) when --json-output is specified.
// Accepts []SlideInput (typed schema) for full typed + legacy field support.
func computeQualityScore(slides []SlideInput, warnings []string) *QualityScore { //nolint:gocognit,gocyclo
	if len(slides) == 0 {
		return &QualityScore{
			Score:  0.0,
			Issues: []string{"no slides in presentation"},
		}
	}

	const (
		maxBullets      = 8
		maxTitleLen     = 60
		maxSubtitleLen  = 120
		maxContent      = 6
	)

	var slideScores []SlideQuality
	var globalIssues []string
	totalScore := 0.0

	for i, slide := range slides {
		slideScore := 1.0
		var issues []string

		// Check for empty slide (no content items)
		if len(slide.Content) == 0 {
			slideScore -= 0.5
			issues = append(issues, "empty slide with no content")
		}

		// Analyze each content item using ResolveValue for typed + legacy support
		bulletCount := 0
		contentCount := len(slide.Content)
		for _, item := range slide.Content {
			resolved, _ := item.ResolveValue()
			switch item.Type {
			case "text":
				// Check title/subtitle length — inspect the placeholder ID to classify
				if isLikelySubtitle(item.PlaceholderID) {
					if text, ok := resolved.(string); ok && len(text) > maxSubtitleLen {
						penalty := float64(len(text)-maxSubtitleLen) / 100.0
						if penalty > 0.3 {
							penalty = 0.3
						}
						slideScore -= penalty
						issues = append(issues, fmt.Sprintf("subtitle too long (%d chars, max %d)", len(text), maxSubtitleLen))
					}
				} else if isLikelyTitle(item.PlaceholderID) {
					if text, ok := resolved.(string); ok && len(text) > maxTitleLen {
						penalty := float64(len(text)-maxTitleLen) / 100.0
						if penalty > 0.3 {
							penalty = 0.3
						}
						slideScore -= penalty
						issues = append(issues, fmt.Sprintf("title too long (%d chars, max %d)", len(text), maxTitleLen))
					}
				}
			case "bullets":
				if bullets, ok := resolved.([]string); ok {
					bulletCount += len(bullets)
				}
			case "table":
				if table, ok := resolved.(*TableInput); ok {
					if len(table.Rows) == 0 {
						slideScore -= 0.2
						issues = append(issues, "table has no data rows")
					}
					if len(table.Headers) == 0 {
						slideScore -= 0.1
						issues = append(issues, "table has no headers")
					}
				}
			case "body_and_bullets":
				if bab, ok := resolved.(*BodyAndBulletsInput); ok {
					bulletCount += len(bab.Bullets)
				}
			case "bullet_groups":
				if bg, ok := resolved.(*BulletGroupsInput); ok {
					for _, g := range bg.Groups {
						bulletCount += len(g.Bullets)
					}
				}
			case "chart":
				chartIssues := scoreChartData(item)
				for _, ci := range chartIssues {
					slideScore -= 0.3
					issues = append(issues, ci)
				}
			case "diagram":
				diagIssues := scoreDiagramData(item)
				for _, di := range diagIssues {
					slideScore -= 0.3
					issues = append(issues, di)
				}
			}
		}

		// Section divider body misuse: agents sometimes put long sentences into
		// the body placeholder which is designed for a decorative section number
		// (e.g. "01"). Warn when the body text is >4 chars and non-numeric.
		if inferSlideType(slide) == types.SlideTypeSection {
			for _, item := range slide.Content {
				if item.Type == "text" && strings.EqualFold(item.PlaceholderID, "body") {
					resolved, _ := item.ResolveValue()
					if text, ok := resolved.(string); ok {
						trimmed := strings.TrimSpace(text)
						if len(trimmed) > 4 {
							if _, err := strconv.Atoi(trimmed); err != nil {
								slideScore -= 0.3
								issues = append(issues, fmt.Sprintf("section divider body misused as text (%d chars; use short number like '01')", len(trimmed)))
							}
						}
					}
				}
			}
		}

		// Bullet count penalty
		if bulletCount > maxBullets {
			penalty := float64(bulletCount-maxBullets) * 0.05
			if penalty > 0.4 {
				penalty = 0.4
			}
			slideScore -= penalty
			issues = append(issues, fmt.Sprintf("too many bullets (%d, max %d)", bulletCount, maxBullets))
		}

		// Too many content items = crowded
		if contentCount > maxContent {
			penalty := float64(contentCount-maxContent) * 0.05
			if penalty > 0.3 {
				penalty = 0.3
			}
			slideScore -= penalty
			issues = append(issues, fmt.Sprintf("slide is crowded (%d content items)", contentCount))
		}

		// Clamp slide score to [0, 1]
		if slideScore < 0 {
			slideScore = 0
		}

		slideScores = append(slideScores, SlideQuality{
			SlideNumber: i + 1,
			Score:       slideScore,
			Issues:      issues,
		})
		totalScore += slideScore
	}

	// Average slide score
	overallScore := totalScore / float64(len(slides))

	// Penalty for pipeline warnings
	if len(warnings) > 0 {
		warningPenalty := float64(len(warnings)) * 0.02
		if warningPenalty > 0.2 {
			warningPenalty = 0.2
		}
		overallScore -= warningPenalty
		globalIssues = append(globalIssues, fmt.Sprintf("%d pipeline warning(s)", len(warnings)))
	}

	// Clamp overall score
	if overallScore < 0 {
		overallScore = 0
	}
	if overallScore > 1 {
		overallScore = 1
	}

	// Round to 2 decimal places
	overallScore = float64(int(overallScore*100+0.5)) / 100

	return &QualityScore{
		Score:       overallScore,
		SlideScores: slideScores,
		Issues:      globalIssues,
	}
}

// sanitizeOutputFilename strips directory components from a user-supplied
// filename to prevent path-traversal attacks (e.g. "../../evil.pptx").
// It returns a bare filename with a .pptx suffix, defaulting to "output.pptx"
// when the input is empty or resolves to nothing useful.
func sanitizeOutputFilename(raw string) string {
	// filepath.Base strips all directory components:
	//   "../../evil.pptx"  → "evil.pptx"
	//   "/tmp/secret.pptx" → "secret.pptx"
	//   ""                 → "."
	name := filepath.Base(raw)
	if name == "" || name == "." || name == ".." {
		name = "output.pptx"
	}
	if !strings.HasSuffix(name, ".pptx") {
		name += ".pptx"
	}
	return name
}

// isLikelyTitle returns true if a placeholder ID looks like a title placeholder
// (but not a subtitle).
func isLikelyTitle(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	if isLikelySubtitle(placeholderID) {
		return false
	}
	return strings.Contains(lower, "title") || strings.Contains(lower, "heading")
}

// isLikelySubtitle returns true if a placeholder ID looks like a subtitle placeholder.
func isLikelySubtitle(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	return strings.Contains(lower, "subtitle") || strings.Contains(lower, "subheading")
}

// scoreChartData checks a chart content item for basic data structure problems.
// Returns a list of issue strings (empty if no problems detected).
func scoreChartData(item ContentInput) []string {
	spec := item.ChartValue
	if spec == nil && len(item.Value) > 0 {
		var parsed types.ChartSpec //nolint:staticcheck // backward compat
		if err := json.Unmarshal(item.Value, &parsed); err == nil {
			spec = &parsed
		}
	}
	if spec == nil {
		return nil // no data to inspect
	}
	return checkDiagramDataStructure(string(spec.Type), spec.Data)
}

// scoreDiagramData checks a diagram content item for basic data structure problems.
// Returns a list of issue strings (empty if no problems detected).
func scoreDiagramData(item ContentInput) []string {
	spec := item.DiagramValue
	if spec == nil && len(item.Value) > 0 {
		var parsed types.DiagramSpec
		if err := json.Unmarshal(item.Value, &parsed); err == nil {
			spec = &parsed
		}
	}
	if spec == nil {
		return nil
	}
	return checkDiagramDataStructure(spec.Type, spec.Data)
}

// checkDiagramDataStructure validates that a diagram's data map contains
// the required keys for its type. Returns issue descriptions for any
// structural problems found.
func checkDiagramDataStructure(diagramType string, data map[string]any) []string {
	if data == nil {
		return []string{fmt.Sprintf("%s has no data", diagramType)}
	}

	// Normalize type name: handle both canonical and alias forms.
	normalized := strings.ToLower(diagramType)

	var issues []string
	switch normalized {
	case "waterfall":
		_, hasPoints := data["points"]
		_, hasLabels := data["labels"]
		_, hasValues := data["values"]
		if !hasPoints && !(hasLabels && hasValues) {
			issues = append(issues, "waterfall chart missing 'points' array (or 'labels'+'values')")
		}
	case "funnel", "funnel_chart":
		_, hasStages := data["stages"]
		_, hasValues := data["values"]
		_, hasPoints := data["points"]
		if !hasStages && !hasValues && !hasPoints {
			issues = append(issues, "funnel chart missing 'stages' (or 'values'/'points') array")
		}
	case "gauge", "gauge_chart":
		if _, ok := data["value"]; !ok {
			issues = append(issues, "gauge chart missing 'value' field")
		}
		_, hasMin := data["min"]
		_, hasMax := data["max"]
		if !hasMin && !hasMax {
			issues = append(issues, "gauge chart missing 'min'/'max' range")
		}
	case "porters_five_forces", "porter", "porters":
		if _, ok := data["forces"]; !ok {
			// Check if force keys are embedded directly (e.g. data.rivalry)
			directForceKeys := []string{"rivalry", "new_entrants", "substitutes", "suppliers", "buyers"}
			hasAny := false
			for _, k := range directForceKeys {
				if _, ok := data[k]; ok {
					hasAny = true
					break
				}
			}
			if !hasAny {
				issues = append(issues, "porter's five forces missing 'forces' array")
			}
		}
	}
	return issues
}

// convertMediaFailures converts generator.MediaFailure records into
// SlideError structs for JSON output. Each MediaFailure represents a
// chart, diagram, image, or table that failed to render on a specific slide.
func convertMediaFailures(failures []generator.MediaFailure) []SlideError {
	if len(failures) == 0 {
		return nil
	}
	errors := make([]SlideError, len(failures))
	for i, f := range failures {
		errors[i] = SlideError{
			SlideNumber: f.SlideNum,
			ContentType: f.ContentType,
			DiagramType: f.DiagramType,
			Error:       f.Reason,
			Fallback:    f.Fallback,
		}
	}
	return errors
}

// writeJSONOutput writes a JSON output to file or stdout.
func writeJSONOutput(path string, output JSONOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}

	if path == "-" {
		_, err = os.Stdout.Write(data)
		_, _ = os.Stdout.WriteString("\n")
	} else {
		err = os.WriteFile(path, append(data, '\n'), 0644)
	}

	return err
}

// inferSlideType determines the SlideType from an explicit hint or content analysis.
func inferSlideType(slide SlideInput) types.SlideType {
	if slide.SlideType != "" {
		return types.SlideType(slide.SlideType)
	}

	hasChart := false
	hasDiagram := false
	hasImage := false
	hasTable := false
	textCount := 0
	bodyTextCount := 0 // text items that are NOT title/subtitle
	hasBullets := false

	for _, item := range slide.Content {
		switch item.Type {
		case "chart":
			hasChart = true
		case "diagram":
			hasDiagram = true
		case "image":
			hasImage = true
		case "table":
			hasTable = true
		case "text":
			textCount++
			if item.PlaceholderID != "" && !isTitlePlaceholderID(item.PlaceholderID) && !isLikelySubtitle(item.PlaceholderID) {
				bodyTextCount++
			}
		case "bullets", "body_and_bullets", "bullet_groups":
			hasBullets = true
		}
	}

	if hasChart || hasDiagram {
		if hasChart {
			return types.SlideTypeChart
		}
		return types.SlideTypeDiagram
	}
	if hasImage && (textCount > 0 || hasBullets) {
		return types.SlideTypeTwoColumn
	}
	if hasImage {
		return types.SlideTypeImage
	}
	if hasTable {
		return types.SlideTypeContent
	}
	// A slide with only title/subtitle text (no body text, no bullets) is a
	// title-type slide — this covers both opening and closing slides (e.g.,
	// "Thank You" + "Questions?" on slideLayout5).
	if bodyTextCount == 0 && !hasBullets {
		return types.SlideTypeTitle
	}
	return types.SlideTypeContent
}

// isTitlePlaceholderID returns true if the placeholder ID targets the title area
// (not subtitle, body, or other content areas).
func isTitlePlaceholderID(id string) bool {
	lower := strings.ToLower(id)
	return lower == "title" || strings.HasPrefix(lower, "title_")
}

// inferJSONSlideType guesses the slide type from legacy JSON slide content.
// Title slides typically have only "title" and optionally "subtitle" text — no
// body, bullets, charts, or images.
func inferJSONSlideType(slide JSONSlide) types.SlideType {
	for _, item := range slide.Content {
		switch item.Type {
		case "bullets", "chart", "diagram", "image", "table",
			"body_and_bullets", "bullet_groups":
			return types.SlideTypeContent
		}
		// Text in non-title, non-subtitle placeholder → content slide
		if item.Type == "text" {
			lower := strings.ToLower(item.PlaceholderID)
			if lower != "title" && lower != "subtitle" && !strings.HasPrefix(lower, "title_") {
				return types.SlideTypeContent
			}
		}
	}
	return types.SlideTypeTitle
}

// jsonSlideToDefinition builds a types.SlideDefinition from JSON input
// for the layout heuristic engine. It translates ContentInput items into the
// SlideContent fields that the heuristic scorer expects.
func jsonSlideToDefinition(slide SlideInput) types.SlideDefinition { //nolint:gocognit,gocyclo
	def := types.SlideDefinition{
		Type: inferSlideType(slide),
	}

	for _, item := range slide.Content {
		resolved, _ := item.ResolveValue()

		// Populate Slots map for content items with slot markers so that
		// HasSlots() returns true and layout selection requires enough
		// content placeholders (prevents silent slot2 content loss).
		if isSlotMarker(item.PlaceholderID) {
			slotNum, _ := strconv.Atoi(item.PlaceholderID[4:])
			if def.Slots == nil {
				def.Slots = make(map[int]*types.SlotContent)
			}
			sc := &types.SlotContent{SlotNumber: slotNum}
			switch item.Type {
			case "bullets":
				sc.Type = types.SlotContentBullets
			case "body_and_bullets":
				sc.Type = types.SlotContentBodyAndBullets
			case "bullet_groups":
				sc.Type = types.SlotContentBulletGroups
			case "chart":
				sc.Type = types.SlotContentChart
			case "diagram":
				sc.Type = types.SlotContentInfographic
			case "table":
				sc.Type = types.SlotContentTable
			case "image":
				sc.Type = types.SlotContentImage
			case "text":
				sc.Type = types.SlotContentText
			}
			def.Slots[slotNum] = sc
		}

		switch item.Type {
		case "text":
			if def.Title == "" {
				if text, ok := resolved.(string); ok {
					def.Title = text
				}
			} else {
				if text, ok := resolved.(string); ok {
					def.Content.Body = text
				}
			}
		case "bullets":
			if bullets, ok := resolved.([]string); ok {
				def.Content.Bullets = bullets
			}
		case "body_and_bullets":
			if bab, ok := resolved.(*BodyAndBulletsInput); ok {
				def.Content.Body = bab.Body
				def.Content.Bullets = bab.Bullets
				def.Content.BodyAfterBullets = bab.TrailingBody
			}
		case "bullet_groups":
			if bg, ok := resolved.(*BulletGroupsInput); ok {
				var groups []types.BulletGroup
				for _, g := range bg.Groups {
					groups = append(groups, types.BulletGroup{
						Header:  g.Header,
						Body:    g.Body,
						Bullets: g.Bullets,
					})
					// Populate flat Bullets for backward compatibility with
					// layout scoring (mirrors markdown parser behavior).
					def.Content.Bullets = append(def.Content.Bullets, g.Bullets...)
				}
				def.Content.BulletGroups = groups
				if bg.Body != "" {
					def.Content.Body = bg.Body
				}
			}
		case "chart":
			if chart, ok := resolved.(*types.ChartSpec); ok { //nolint:staticcheck // backward compat
				def.Content.DiagramSpec = chart.ToDiagramSpec()
			} else if len(item.Value) > 0 {
				var chart types.ChartSpec //nolint:staticcheck // backward compat
				if json.Unmarshal(item.Value, &chart) == nil {
					def.Content.DiagramSpec = chart.ToDiagramSpec()
				}
			}
		case "diagram":
			if diagram, ok := resolved.(*types.DiagramSpec); ok {
				def.Content.DiagramSpec = diagram
			} else if len(item.Value) > 0 {
				var diagram types.DiagramSpec
				if json.Unmarshal(item.Value, &diagram) == nil {
					def.Content.DiagramSpec = &diagram
				}
			}
		case "table":
			def.Content.TableRaw = "table" // Signal presence for heuristic
		case "image":
			if img, ok := resolved.(*ImageInput); ok {
				def.Content.ImagePath = img.Path
			} else if len(item.Value) > 0 {
				var img struct{ Path string `json:"path"` }
				if json.Unmarshal(item.Value, &img) == nil {
					def.Content.ImagePath = img.Path
				}
			}
		}
	}

	return def
}

// autoMapPlaceholders assigns placeholder IDs to content items that lack them,
// using the selected layout's placeholder metadata. It also resolves virtual
// slot markers ("slot1", "slot2", ...) to actual content placeholder IDs.
func autoMapPlaceholders(items []ContentInput, selectedLayout types.LayoutMetadata) []ContentInput {
	result := make([]ContentInput, len(items))
	copy(result, items)

	// Build slot-to-placeholder mapping for "slot1", "slot2", etc.
	// This mirrors the logic in generator.BuildSlotContentItems.
	contentPlaceholders := generator.FilterContentPlaceholders(selectedLayout.Placeholders)
	slotMap := make(map[string]string, len(contentPlaceholders))
	for i, ph := range contentPlaceholders {
		slotMap[fmt.Sprintf("slot%d", i+1)] = placeholderIDStr(ph)
	}

	titlePH := findFirstPlaceholder(selectedLayout, types.PlaceholderTitle)
	bodyPH := findFirstPlaceholder(selectedLayout, types.PlaceholderBody)
	if bodyPH == nil {
		bodyPH = findFirstPlaceholder(selectedLayout, types.PlaceholderContent)
	}
	subtitlePH := findFirstPlaceholder(selectedLayout, types.PlaceholderSubtitle)
	imagePH := findFirstPlaceholder(selectedLayout, types.PlaceholderImage)
	chartPH := findFirstPlaceholder(selectedLayout, types.PlaceholderChart)

	// Build a map of well-known virtual IDs to actual placeholder IDs.
	// JSON producers use "title", "subtitle", "body" as logical names,
	// but the actual OOXML placeholder types may differ (e.g., "ctrTitle", "subTitle").
	virtualMap := make(map[string]string)
	if titlePH != nil {
		virtualMap["title"] = placeholderIDStr(*titlePH)
	} else if bodyPH != nil {
		// Section divider layouts may lack a title placeholder entirely,
		// using only a body placeholder for the section title text.
		// Fall back to the body placeholder so "title" content items render.
		virtualMap["title"] = placeholderIDStr(*bodyPH)
	}
	if subtitlePH != nil {
		virtualMap["subtitle"] = placeholderIDStr(*subtitlePH)
	}
	if bodyPH != nil {
		virtualMap["body"] = placeholderIDStr(*bodyPH)
	}
	// Map "body_2", "body_3", etc. to the 2nd, 3rd, ... content placeholders.
	// This ensures two-column (and multi-column) layouts populated via "body"/"body_2"
	// target the correct physical placeholder instead of relying on semantic fallback
	// which sorts by area and may map both to the same shape.
	for i := 1; i < len(contentPlaceholders); i++ {
		key := fmt.Sprintf("body_%d", i+1) // body_2, body_3, ...
		virtualMap[key] = placeholderIDStr(contentPlaceholders[i])
	}

	titleAssigned := false
	for i := range result {
		// Resolve virtual placeholder IDs (slots and well-known names).
		if phID, ok := slotMap[result[i].PlaceholderID]; ok {
			result[i].PlaceholderID = phID
			continue
		}
		if phID, ok := virtualMap[result[i].PlaceholderID]; ok {
			if result[i].PlaceholderID == "title" {
				titleAssigned = true
			}
			result[i].PlaceholderID = phID
			continue
		}

		if result[i].PlaceholderID != "" {
			continue // Already has explicit placeholder
		}

		switch result[i].Type {
		case "text":
			if !titleAssigned && titlePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*titlePH)
				titleAssigned = true
			} else if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			} else if subtitlePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*subtitlePH)
			}
		case "bullets", "body_and_bullets", "bullet_groups", "table":
			if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			}
		case "chart", "diagram":
			if chartPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*chartPH)
			} else if bodyPH != nil {
				result[i].PlaceholderID = placeholderIDStr(*bodyPH)
			}
		case "image":
			if imagePH != nil {
				result[i].PlaceholderID = placeholderIDStr(*imagePH)
			}
		}
	}

	return result
}

// hasVirtualPlaceholders returns true if any content item uses a virtual
// placeholder ID that needs resolution (e.g., "slot1", "title", "subtitle", "body").
func hasVirtualPlaceholders(items []ContentInput) bool {
	for _, item := range items {
		switch item.PlaceholderID {
		case "title", "subtitle", "body":
			return true
		}
		if isSlotMarker(item.PlaceholderID) {
			return true
		}
		if isBodyNMarker(item.PlaceholderID) {
			return true
		}
	}
	return false
}

// isBodyNMarker returns true if the placeholder ID is a virtual body_N marker (e.g., "body_2", "body_3").
func isBodyNMarker(id string) bool {
	if !strings.HasPrefix(id, "body_") {
		return false
	}
	_, err := strconv.Atoi(id[5:])
	return err == nil
}

// isSlotMarker returns true if the placeholder ID is a virtual slot marker (e.g., "slot1", "slot2").
func isSlotMarker(id string) bool {
	if !strings.HasPrefix(id, "slot") {
		return false
	}
	_, err := strconv.Atoi(id[4:])
	return err == nil
}

// findFirstPlaceholder returns the first placeholder of a given type in a layout.
func findFirstPlaceholder(layout types.LayoutMetadata, phType types.PlaceholderType) *types.PlaceholderInfo {
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == phType {
			return &layout.Placeholders[i]
		}
	}
	return nil
}

// placeholderIDStr returns the placeholder's canonical ID.
// After normalization, placeholder IDs are unique within a layout
// (e.g., "title", "body", "body_2", "image").
func placeholderIDStr(ph types.PlaceholderInfo) string {
	return ph.ID
}
