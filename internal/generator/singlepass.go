// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// Note: singlePassContext, mediaRel, and nativeSVGInsert are defined in contexts.go
// They have been refactored from a god object into focused sub-contexts:
//   - ZipContext: ZIP I/O operations
//   - SlideContext: Slide tracking and manipulation
//   - MediaContext: Media file management
//   - SVGContext: SVG conversion and embedding
//   - SecurityContext: Security configuration
//   - ChartContext: Chart rendering
//   - OutputContext: Output files and warnings

// slideFileRegexSP matches slide XML files (package-level for single pass)
var slideFileRegexSP = regexp.MustCompile(`ppt/slides/slide\d+\.xml`)

// shapeIDRegex matches id="N" patterns in OOXML to find shape IDs.
// Pre-compiled for efficiency since it's called for every slide with native SVG inserts.
var shapeIDRegex = regexp.MustCompile(`\bid="(\d+)"`)

// findMaxShapeID extracts the maximum shape ID from slide XML using regex.
// Returns the max ID found, or 0 if none found.
func findMaxShapeID(slideData []byte) uint32 {
	matches := shapeIDRegex.FindAllSubmatch(slideData, -1)
	var maxID uint32
	for _, match := range matches {
		if len(match) >= 2 {
			var id uint32
			if _, err := fmt.Sscanf(string(match[1]), "%d", &id); err == nil {
				if id > maxID {
					maxID = id
				}
			}
		}
	}
	return maxID
}

// buildSVGConfig creates an SVG configuration from the request with defaults applied.
func buildSVGConfig(req GenerationRequest) (SVGConfig, SVGNativeCompatibility) {
	svgCfg := SVGConfig{
		Strategy:    SVGConversionStrategy(req.SVGStrategy),
		Scale:       req.SVGScale,
		MaxPNGWidth: req.MaxPNGWidth,
	}
	if svgCfg.Strategy == "" {
		svgCfg.Strategy = SVGStrategyNative
	}
	if svgCfg.Scale <= 0 {
		svgCfg.Scale = DefaultSVGScale
	}
	if svgCfg.MaxPNGWidth == 0 {
		svgCfg.MaxPNGWidth = DefaultMaxPNGWidth
	}

	compatMode := SVGNativeCompatibility(req.SVGNativeCompat)
	if compatMode == "" {
		compatMode = SVGCompatIgnore
	}

	return svgCfg, compatMode
}

// checkNativeSVGCompatibility verifies SVG compatibility when using native strategy.
// It may fall back to PNG strategy if compatibility issues are detected.
func (ctx *singlePassContext) checkNativeSVGCompatibility(compatMode SVGNativeCompatibility) error {
	checker, err := CheckSVGCompatibilityFromReader(&ctx.templateReader.Reader)
	if err != nil {
		return fmt.Errorf("failed to check SVG compatibility: %w", err)
	}

	// Check strict mode - fail if compatibility cannot be confirmed
	if err := checker.CheckStrict(ctx.svgConverter.GetStrategy(), compatMode); err != nil {
		return err
	}

	// Check if we should fallback to PNG
	if checker.ShouldFallback(ctx.svgConverter.GetStrategy(), compatMode) {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf(
			"falling back to PNG strategy: %s",
			checker.CompatibilityMessage,
		))
		ctx.svgConverter.SetStrategy(SVGStrategyPNG)
	} else if warning := checker.GenerateWarning(ctx.svgConverter.GetStrategy()); warning != "" {
		ctx.warnings = append(ctx.warnings, warning)
	}

	return nil
}

// initializeContext opens the template and creates the output file.
// Returns a cleanup function that should be deferred by the caller.
func (ctx *singlePassContext) initializeContext(templatePath string) (cleanup func(), err error) {
	ctx.templateReader, err = zip.OpenReader(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	ctx.templateIndex = utils.BuildZipIndex(&ctx.templateReader.Reader)

	ctx.outputFile, err = os.Create(ctx.tmpPath)
	if err != nil {
		_ = ctx.templateReader.Close()
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	ctx.outputWriter = zip.NewWriter(ctx.outputFile)

	cleanup = func() {
		_ = ctx.templateReader.Close()
		if ctx.outputFile != nil {
			_ = ctx.outputFile.Close()
		}
		if ctx.tmpPath != "" {
			_ = os.Remove(ctx.tmpPath)
		}
		for _, svgCleanup := range ctx.svgCleanupFuncs {
			svgCleanup()
		}
	}

	// Normalize placeholder names in all template layouts so the generator
	// uses canonical names (title, body, image, etc.) regardless of what
	// the original OOXML template used ("Title 1", "Content Placeholder 2").
	ctx.normalizeTemplateLayouts()

	return cleanup, nil
}

// normalizeTemplateLayouts reads each slideLayout*.xml from the template ZIP,
// applies placeholder name normalization, and stores the result in syntheticFiles.
// Layouts already present in syntheticFiles (e.g., synthesized two-column layouts)
// are also normalized so their placeholder names and types match what the resolver expects.
func (ctx *singlePassContext) normalizeTemplateLayouts() {
	if ctx.syntheticFiles == nil {
		ctx.syntheticFiles = make(map[string][]byte)
	}

	// Pass 1: Normalize layouts from the template ZIP.
	for _, f := range ctx.templateReader.File {
		if !strings.HasPrefix(f.Name, PathSlideLayouts) ||
			!strings.HasSuffix(f.Name, ".xml") ||
			strings.Contains(f.Name, "_rels/") {
			continue
		}

		// Skip if already in syntheticFiles (caller already normalized)
		if _, ok := ctx.syntheticFiles[f.Name]; ok {
			continue
		}

		data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, f.Name)
		if err != nil {
			continue
		}

		if normalized, changed := template.NormalizeLayoutBytes(data); changed {
			ctx.syntheticFiles[f.Name] = normalized
		}
	}

	// Pass 2: Normalize synthesized layout files already in syntheticFiles.
	// These were generated by synthesis and may have raw placeholder names
	// (e.g., "slot1"/"slot2") that need canonical normalization.
	for path, data := range ctx.syntheticFiles {
		if !strings.HasPrefix(path, PathSlideLayouts) ||
			!strings.HasSuffix(path, ".xml") ||
			strings.Contains(path, "_rels/") {
			continue
		}

		if normalized, changed := template.NormalizeLayoutBytes(data); changed {
			ctx.syntheticFiles[path] = normalized
		}
	}
}

// runPipelinePhases executes the 4-phase generation pipeline.
func (ctx *singlePassContext) runPipelinePhases() error {
	// Phase 1: Scan template to understand structure
	if err := ctx.scanTemplate(); err != nil {
		return fmt.Errorf("failed to scan template: %w", err)
	}

	if err := ctx.ctx.Err(); err != nil {
		return fmt.Errorf("generation cancelled after template scan: %w", err)
	}

	// Phase 2: Prepare all slide modifications and new slides
	if err := ctx.prepareSlides(); err != nil {
		return fmt.Errorf("failed to prepare slides: %w", err)
	}

	if err := ctx.ctx.Err(); err != nil {
		return fmt.Errorf("generation cancelled after slide preparation: %w", err)
	}

	// Phase 3: Prepare image content and media files
	if err := ctx.prepareImages(); err != nil {
		return fmt.Errorf("failed to prepare images: %w", err)
	}

	if err := ctx.ctx.Err(); err != nil {
		return fmt.Errorf("generation cancelled after image preparation: %w", err)
	}

	// Phase 3.5: Clear unmapped placeholder shapes to prevent template text leaking
	ctx.clearUnmappedPlaceholders()

	// Phase 3.6: Update docProps/app.xml with accurate slide/word counts
	ctx.updateAppProperties(len(ctx.slideSpecs))

	// Phase 4: Single-pass write - copy unchanged, write modified, add new
	if err := ctx.writeOutput(); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// finalizePPTX closes the ZIP writer, finalizes the output file, and returns the result.
func (ctx *singlePassContext) finalizePPTX(slideCount int) (*GenerationResult, error) {
	if err := ctx.outputWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close ZIP writer: %w", err)
	}
	if err := ctx.outputFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close output file: %w", err)
	}
	ctx.outputFile = nil // prevent double-close in deferred cleanup

	if err := os.Rename(ctx.tmpPath, ctx.outputPath); err != nil {
		return nil, fmt.Errorf("failed to finalize output: %w", err)
	}
	ctx.tmpPath = "" // file renamed; prevent deferred cleanup from removing it

	info, err := os.Stat(ctx.outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output: %w", err)
	}

	// Log structured media failures for observability
	for _, mf := range ctx.mediaFailures {
		slog.Warn("media content failed",
			slog.Int("slide", mf.SlideNum),
			slog.String("placeholder", mf.PlaceholderID),
			slog.String("type", mf.ContentType),
			slog.String("diagram_type", mf.DiagramType),
			slog.String("fallback", mf.Fallback),
			slog.String("reason", mf.Reason),
		)
	}

	return &GenerationResult{
		OutputPath:       ctx.outputPath,
		FileSize:         info.Size(),
		SlideCount:       slideCount,
		Warnings:         ctx.warnings,
		ValidationErrors: ctx.validationErrors,
		MediaFailures:    ctx.mediaFailures,
		ContrastSwaps:    ctx.contrastSwaps,
	}, nil
}

// generateSinglePass performs PPTX generation in a single ZIP pass.
// This is the optimized implementation that eliminates the 3x ZIP read/write overhead.
//
// AC1 (C1 fix): Given a generation request with 10 slides,
// When processed, Then only ONE ZIP read and ONE ZIP write operation occurs.
func generateSinglePass(goCtx context.Context, req GenerationRequest) (*GenerationResult, []string, error) {
	svgCfg, compatMode := buildSVGConfig(req)

	ctx := newSinglePassContext(req.OutputPath, req.Slides, req.AllowedImagePaths, req.ExcludeTemplateSlides, req.SyntheticFiles)
	ctx.ctx = goCtx
	ctx.svgConverter = NewSVGConverterWithConfig(svgCfg)
	ctx.themeOverride = req.ThemeOverride
	ctx.footerConfig = req.Footer
	ctx.strictFit = req.StrictFit

	cleanup, err := ctx.initializeContext(req.TemplatePath)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	// Check native SVG compatibility if using native strategy
	if svgCfg.Strategy == SVGStrategyNative {
		if err := ctx.checkNativeSVGCompatibility(compatMode); err != nil {
			return nil, nil, err
		}
	}

	if err := ctx.runPipelinePhases(); err != nil {
		return nil, ctx.warnings, err
	}

	result, err := ctx.finalizePPTX(len(req.Slides))
	if err != nil {
		return nil, ctx.warnings, err
	}

	return result, ctx.warnings, nil
}

// scanTemplate reads the template to understand its structure
func (ctx *singlePassContext) scanTemplate() error { //nolint:gocognit,gocyclo
	// Count existing slides and scan media files for collision-free numbering
	var mediaPaths []string
	for _, f := range ctx.templateReader.File {
		if slideFileRegexSP.MatchString(f.Name) {
			ctx.existingSlides++
		}
		if strings.HasPrefix(f.Name, PathMedia) {
			mediaPaths = append(mediaPaths, f.Name)
		}
	}
	ctx.media.ScanPaths(mediaPaths)

	// Extract actual slide dimensions from presentation.xml <p:sldSz>
	ctx.extractSlideDimensions()

	// Parse theme colors for chart styling
	themeInfo := template.ParseThemeFromZip(&ctx.templateReader.Reader)
	// Apply per-deck theme overrides from frontmatter (if any)
	if ctx.themeOverride != nil {
		themeInfo, _ = themeInfo.ApplyOverride(ctx.themeOverride)
	}
	ctx.themeColors = themeInfo.Colors
	ctx.themeFontName = themeInfo.BodyFont

	// Detect logo images in slide layouts so title/diagram positions can be adjusted
	ctx.logoZones = detectLogoZones(&ctx.templateReader.Reader, ctx.templateIndex)

	// Initialize per-layout footer position cache (populated lazily during slide writing)
	if ctx.footerConfig != nil && ctx.footerConfig.Enabled {
		ctx.footerPositionsByLayout = make(map[string]map[string]*transformXML)
	}

	// Build slide content map
	// When excluding template slides, new slides start at 1
	// Otherwise, they start at existingSlides+1
	startSlideNum := ctx.existingSlides + 1
	if ctx.excludeTemplateSlides {
		startSlideNum = 1
	}
	for i, slide := range ctx.slideSpecs {
		slideNum := startSlideNum + i
		ctx.slideContentMap[slideNum] = slide
		if slide.SpeakerNotes != "" {
			ctx.slideNotes[slideNum] = slide.SpeakerNotes
		}
		if slide.SourceNote != "" {
			ctx.slideSources[slideNum] = slide.SourceNote
		}
		// Register icon inserts from shape_grid as native SVG inserts
		for iconIdx, icon := range slide.IconInserts {
			sourceID := fmt.Sprintf("icon-s%d-i%d", slideNum, iconIdx)
			svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(sourceID)
			ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
				svgData:        icon.SVGData,
				pngData:        transparentPNG1x1,
				svgMediaFile:   svgMediaFile,
				pngMediaFile:   pngMediaFile,
				offsetX:        icon.OffsetX,
				offsetY:        icon.OffsetY,
				extentCX:       icon.ExtentCX,
				extentCY:       icon.ExtentCY,
				placeholderIdx: -1, // No placeholder to remove — injected as new p:pic
			})
		}
		// Register image inserts from shape_grid as media relationships
		for imgIdx, img := range slide.ImageInserts {
			// Validate image path for security (prevent path traversal)
			if err := ValidateImagePathWithConfig(img.Path, ctx.allowedImagePaths); err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("shape_grid image: path validation failed: %v", err))
				continue
			}
			// Verify image file exists
			if _, err := os.Stat(img.Path); err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("shape_grid image: file not found: %s", img.Path))
				continue
			}
			// Route SVG images through native SVG embedding (asvg:svgBlip)
			if IsSVGFile(img.Path) {
				svgData, err := os.ReadFile(img.Path)
				if err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("shape_grid image: failed to read SVG file: %v", err))
					continue
				}
				sourceID := fmt.Sprintf("gridsvg-s%d-i%d", slideNum, imgIdx)
				svgMediaFile, pngMediaFile := ctx.allocSVGPNGPair(sourceID)
				ctx.nativeSVGInserts[slideNum] = append(ctx.nativeSVGInserts[slideNum], nativeSVGInsert{
					svgData:        svgData,
					pngData:        transparentPNG1x1,
					svgMediaFile:   svgMediaFile,
					pngMediaFile:   pngMediaFile,
					offsetX:        img.OffsetX,
					offsetY:        img.OffsetY,
					extentCX:       img.ExtentCX,
					extentCY:       img.ExtentCY,
					placeholderIdx: -1, // No placeholder to remove — injected as new p:pic
				})
				continue
			}
			mediaFileName := ctx.allocateMediaSlot(img.Path)
			ctx.slideRelUpdates[slideNum] = append(ctx.slideRelUpdates[slideNum], mediaRel{
				imagePath:      img.Path,
				mediaFileName:  mediaFileName,
				offsetX:        img.OffsetX,
				offsetY:        img.OffsetY,
				extentCX:       img.ExtentCX,
				extentCY:       img.ExtentCY,
				placeholderIdx: -1, // No placeholder to remove — injected as new p:pic
			})
		}
		// Register background image as media relationship
		if slide.Background != nil && slide.Background.Path != "" {
			bgPath := slide.Background.Path
			if err := ValidateImagePathWithConfig(bgPath, ctx.allowedImagePaths); err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("background image: path validation failed: %v", err))
			} else if _, err := os.Stat(bgPath); err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("background image: file not found: %s", bgPath))
			} else {
				mediaFileName := ctx.allocateMediaSlot(bgPath)
				ctx.slideBgMedia[slideNum] = mediaRel{
					imagePath:     bgPath,
					mediaFileName: mediaFileName,
				}
			}
		}
	}

	return nil
}

// extractSlideDimensions reads slide width and height from presentation.xml <p:sldSz>.
// Falls back to standard 16:9 dimensions if the element is missing or unreadable.
func (ctx *singlePassContext) extractSlideDimensions() {
	data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, PathPresentationXML)
	if err != nil {
		slog.Debug("could not read presentation.xml for slide dimensions", slog.String("error", err.Error()))
		return
	}

	var pres pptx.PresentationXML
	if err := xml.Unmarshal(data, &pres); err != nil {
		slog.Debug("could not parse presentation.xml for slide dimensions", slog.String("error", err.Error()))
		return
	}

	if pres.SlideSize != nil && pres.SlideSize.CX > 0 && pres.SlideSize.CY > 0 {
		ctx.slideWidth = pres.SlideSize.CX
		ctx.slideHeight = pres.SlideSize.CY
		slog.Debug("slide dimensions extracted",
			slog.Int64("width", ctx.slideWidth),
			slog.Int64("height", ctx.slideHeight),
		)
	}
}

// getFooterPositionsForLayout returns footer placeholder positions for a given layout.
// Positions are resolved from the layout's slide master and cached per-layout.
// Different layouts may reference different slide masters, so footer positions can vary.
func (ctx *singlePassContext) getFooterPositionsForLayout(layoutID string) map[string]*transformXML {
	// Check cache first
	if positions, ok := ctx.footerPositionsByLayout[layoutID]; ok {
		return positions
	}

	// Find the master for this layout by reading the layout's relationships
	layoutRelsPath := LayoutRelsPath(layoutID)
	relsData, err := ctx.readFileWithSyntheticFallback(layoutRelsPath)
	if err != nil {
		slog.Debug("footer: layout rels not found, using default positions",
			slog.String("layout_id", layoutID),
		)
		positions := resolveFooterPositions(nil, ctx.slideHeight)
		ctx.footerPositionsByLayout[layoutID] = positions
		return positions
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		slog.Debug("footer: layout rels parse error, using default positions",
			slog.String("layout_id", layoutID),
		)
		positions := resolveFooterPositions(nil, ctx.slideHeight)
		ctx.footerPositionsByLayout[layoutID] = positions
		return positions
	}

	// Find the slideMaster relationship
	var masterPath string
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			basePath := strings.TrimSuffix(PathSlideLayouts, "/")
			masterPath = template.ResolveRelativePath(basePath, rel.Target)
			break
		}
	}

	if masterPath == "" {
		slog.Debug("footer: no master relationship for layout, using default positions",
			slog.String("layout_id", layoutID),
		)
		positions := resolveFooterPositions(nil, ctx.slideHeight)
		ctx.footerPositionsByLayout[layoutID] = positions
		return positions
	}

	masterData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, masterPath)
	if err != nil {
		slog.Warn("footer: failed to read slide master, using default positions",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		positions := resolveFooterPositions(nil, ctx.slideHeight)
		ctx.footerPositionsByLayout[layoutID] = positions
		return positions
	}

	masterTransforms, err := template.ParseSlideMasterPositions(masterData)
	if err != nil {
		slog.Warn("footer: failed to parse slide master, using default positions",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		positions := resolveFooterPositions(nil, ctx.slideHeight)
		ctx.footerPositionsByLayout[layoutID] = positions
		return positions
	}

	allPositions := convertMasterTransformsToXML(masterTransforms)

	// If the master has no footer positions, check the layout XML itself
	hasFooterInMaster := false
	for key := range allPositions {
		if strings.HasPrefix(key, "type:dt") || strings.HasPrefix(key, "type:ftr") || strings.HasPrefix(key, "type:sldNum") {
			hasFooterInMaster = true
			break
		}
	}

	if !hasFooterInMaster {
		layoutPath := fmt.Sprintf(PatternSlideLayoutXML, layoutID)
		layoutData, err := ctx.readFileWithSyntheticFallback(layoutPath)
		if err == nil {
			layoutPositions := parseLayoutFooterPositions(layoutData)
			for key, pos := range layoutPositions {
				allPositions[key] = pos
			}
		}
	}

	positions := resolveFooterPositions(allPositions, ctx.slideHeight)
	ctx.footerPositionsByLayout[layoutID] = positions

	slog.Debug("footer positions extracted for layout",
		slog.String("layout_id", layoutID),
		slog.String("master_path", masterPath),
		slog.Int("footer_position_count", len(positions)),
	)
	return positions
}

// parseLayoutFooterPositions extracts footer placeholder positions from layout XML.
// Returns positions keyed by "type:dt", "type:ftr", "type:sldNum".
func parseLayoutFooterPositions(layoutData []byte) map[string]*transformXML {
	var layout slideLayoutXML
	if err := xml.Unmarshal(layoutData, &layout); err != nil {
		return nil
	}

	positions := make(map[string]*transformXML)
	for _, shape := range layout.CommonSlideData.ShapeTree.Shapes {
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph == nil || !footerPlaceholderTypes[ph.Type] {
			continue
		}
		if shape.ShapeProperties.Transform == nil {
			continue
		}
		key := "type:" + ph.Type
		positions[key] = shape.ShapeProperties.Transform
	}
	return positions
}

// detectLogoZones scans each slide layout for small decorative images (logos)
// in the top-left corner. Returns a per-layout map so the logo zone shift is
// only applied to slides that actually use a layout containing a logo.
//
// A shape is considered a logo candidate when:
//   - It is a p:pic element (picture shape) in a slide layout
//   - It is positioned in the top-left quadrant (x < 25% slide width, y < 25% slide height)
//   - It is small relative to the slide (width < 25% and height < 25%)
//   - It is NOT a full-slide background image (width > 90% of slide)
//
// When multiple logo candidates exist within the same layout, the one with the
// largest right-edge is used so all content clears the widest logo placement.
func detectLogoZones(zipReader *zip.Reader, idx utils.ZipIndex) map[string]*LogoZone {
	// Standard widescreen slide dimensions in EMU
	const (
		slideWidthEMU  int64 = 12192000 // 13.33 inches
		slideHeightEMU int64 = 6858000  // 7.50 inches
		logoMarginEMU  int64 = 137160   // 0.15 inches
	)

	zones := make(map[string]*LogoZone)

	for _, f := range zipReader.File {
		if !strings.HasPrefix(f.Name, "ppt/slideLayouts/") ||
			!strings.HasSuffix(f.Name, ".xml") ||
			strings.Contains(f.Name, "_rels/") {
			continue
		}

		data, err := utils.ReadFileFromZipIndex(idx, f.Name)
		if err != nil {
			continue
		}

		// Extract layout basename (e.g. "slideLayout1" from "ppt/slideLayouts/slideLayout1.xml")
		layoutBasename := strings.TrimPrefix(f.Name, "ppt/slideLayouts/")
		layoutBasename = strings.TrimSuffix(layoutBasename, ".xml")

		var bestZone *LogoZone
		pics := parseLayoutPictures(data)
		for _, pic := range pics {
			if pic.ExtCX <= 0 || pic.ExtCY <= 0 {
				continue
			}
			if pic.ExtCX > slideWidthEMU*9/10 {
				continue
			}
			if pic.OffX > slideWidthEMU/4 || pic.OffY > slideHeightEMU/4 {
				continue
			}
			if pic.ExtCX > slideWidthEMU/4 || pic.ExtCY > slideHeightEMU/4 {
				continue
			}

			right := pic.OffX + pic.ExtCX
			bottom := pic.OffY + pic.ExtCY

			if bestZone == nil || right > bestZone.Right {
				bestZone = &LogoZone{Right: right, Bottom: bottom}
			}

			slog.Debug("logo candidate detected in layout",
				slog.String("layout", f.Name),
				slog.String("pic_name", pic.Name),
				slog.Int64("off_x", pic.OffX),
				slog.Int64("off_y", pic.OffY),
				slog.Int64("ext_cx", pic.ExtCX),
				slog.Int64("ext_cy", pic.ExtCY),
			)
		}

		if bestZone != nil {
			bestZone.Right += logoMarginEMU
			zones[layoutBasename] = bestZone

			slog.Info("logo zone detected in layout",
				slog.String("layout", layoutBasename),
				slog.Int64("logo_right_emu", bestZone.Right),
				slog.Int64("logo_bottom_emu", bestZone.Bottom),
			)
		}
	}

	if len(zones) == 0 {
		return nil
	}
	return zones
}

// layoutPicInfo holds position and size data for a p:pic element in a layout.
type layoutPicInfo struct {
	Name  string
	OffX  int64
	OffY  int64
	ExtCX int64
	ExtCY int64
}

// parseLayoutPictures extracts p:pic elements from a slide layout XML using a
// lightweight streaming XML decoder. Only the transform (position/size) data is
// extracted — no images are loaded.
func parseLayoutPictures(data []byte) []layoutPicInfo {
	// Use a simple XML struct that captures p:pic elements within the shape tree.
	// The shape tree is <p:cSld><p:spTree>, and pictures are <p:pic> children.
	type picNvCNvPr struct {
		Name string `xml:"name,attr"`
	}
	type picNvPicPr struct {
		CNvPr picNvCNvPr `xml:"cNvPr"`
	}
	type picOff struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	}
	type picExt struct {
		CX int64 `xml:"cx,attr"`
		CY int64 `xml:"cy,attr"`
	}
	type picXfrm struct {
		Off picOff `xml:"off"`
		Ext picExt `xml:"ext"`
	}
	type picSpPr struct {
		Xfrm picXfrm `xml:"xfrm"`
	}
	type picXML struct {
		NvPicPr picNvPicPr `xml:"nvPicPr"`
		SpPr    picSpPr    `xml:"spPr"`
	}

	// Parse the layout using a streaming decoder to find p:pic elements
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var pics []layoutPicInfo

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		// Match p:pic in the presentationml namespace, or just "pic" in local name
		if se.Name.Local == "pic" {
			var pic picXML
			if err := decoder.DecodeElement(&pic, &se); err != nil {
				continue
			}
			pics = append(pics, layoutPicInfo{
				Name:  pic.NvPicPr.CNvPr.Name,
				OffX:  pic.SpPr.Xfrm.Off.X,
				OffY:  pic.SpPr.Xfrm.Off.Y,
				ExtCX: pic.SpPr.Xfrm.Ext.CX,
				ExtCY: pic.SpPr.Xfrm.Ext.CY,
			})
		}
	}

	return pics
}
