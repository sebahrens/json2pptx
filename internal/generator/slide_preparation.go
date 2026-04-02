// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// slidePreparationInput contains the inputs needed to prepare a single slide.
type slidePreparationInput struct {
	slideSpec            SlideSpec
	slideNum             int
	relID                string
	masterPositionsCache map[string]map[string]*transformXML
}

// slidePreparationResult contains the result of preparing a single slide.
type slidePreparationResult struct {
	slide      *slideXML
	slideEntry string
	warnings   []string
}

// prepareSlides creates new slides and prepares text content.
func (ctx *singlePassContext) prepareSlides() error {
	presentationData, err := ctx.loadPresentationXML()
	if err != nil {
		return err
	}

	nextRelID := ctx.findMaxPresentationRelID() + 1
	nextSlideNum := ctx.calculateStartingSlideNum()
	masterPositionsCache := make(map[string]map[string]*transformXML)

	var newSlideEntries []string
	for _, slideSpec := range ctx.slideSpecs {
		input := slidePreparationInput{
			slideSpec:            slideSpec,
			slideNum:             nextSlideNum,
			relID:                fmt.Sprintf("rId%d", nextRelID),
			masterPositionsCache: masterPositionsCache,
		}

		result, err := ctx.prepareSingleSlide(input)
		if err != nil {
			return err
		}

		ctx.templateSlideData[nextSlideNum] = result.slide
		ctx.slideRelIDs[nextSlideNum] = input.relID
		ctx.warnings = append(ctx.warnings, result.warnings...)
		newSlideEntries = append(newSlideEntries, result.slideEntry)

		nextSlideNum++
		nextRelID++
	}

	ctx.updatePresentationSlideList(presentationData, newSlideEntries)
	return nil
}

// loadPresentationXML reads and validates presentation.xml from the template.
func (ctx *singlePassContext) loadPresentationXML() ([]byte, error) {
	presentationData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, PathPresentationXML)
	if err != nil {
		return nil, fmt.Errorf("failed to read presentation.xml: %w", err)
	}

	var presentation pptx.PresentationXML
	if err := xml.Unmarshal(presentationData, &presentation); err != nil {
		return nil, fmt.Errorf("failed to parse presentation.xml: %w", err)
	}

	return presentationData, nil
}

// calculateStartingSlideNum determines where new slides should start numbering.
func (ctx *singlePassContext) calculateStartingSlideNum() int {
	if ctx.excludeTemplateSlides {
		return 1
	}
	return ctx.existingSlides + 1
}

// prepareSingleSlide processes a single slide spec and returns the prepared slide.
func (ctx *singlePassContext) prepareSingleSlide(input slidePreparationInput) (slidePreparationResult, error) {
	layoutData, err := ctx.readLayoutFile(input.slideSpec.LayoutID)
	if err != nil {
		return slidePreparationResult{}, err
	}

	masterPositions := ctx.getMasterPositionsForLayout(input.slideSpec.LayoutID, input.masterPositionsCache)
	if masterPositions == nil {
		slog.Info("master position resolution returned nil, shapes may have zero bounds",
			slog.String("layout_id", input.slideSpec.LayoutID),
			slog.Int("slide_num", input.slideNum),
		)
	}

	slide, err := ctx.createAndParseSlide(layoutData, input.slideNum, masterPositions, input.slideSpec.LayoutID)
	if err != nil {
		return slidePreparationResult{}, err
	}

	var warnings []string
	if len(input.slideSpec.Content) > 0 {
		warnings = ctx.populateTextInSlide(slide, input.slideSpec.Content, input.slideSpec.LayoutID)
	}

	// Enforce WCAG AA text contrast against the layout background.
	// Some templates (e.g. section dividers) use accent scheme colors for
	// body text that have poor contrast on the layout's background fill.
	bgHex := extractLayoutBackgroundColor(layoutData, ctx.themeColors)
	if bgHex != "" {
		enforceTextContrastInSlide(slide, bgHex, ctx.themeColors)
	}

	slideEntry := fmt.Sprintf(`<p:sldId id="%d" r:id="%s"/>`, uint32(256+input.slideNum), input.relID)

	return slidePreparationResult{
		slide:      slide,
		slideEntry: slideEntry,
		warnings:   warnings,
	}, nil
}

// readLayoutFile reads a layout file, checking synthetic files first then the template ZIP.
func (ctx *singlePassContext) readLayoutFile(layoutID string) ([]byte, error) {
	layoutPath := LayoutPath(layoutID)
	if data, ok := ctx.syntheticFiles[layoutPath]; ok {
		return data, nil
	}
	layoutData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, layoutPath)
	if err != nil {
		available := ctx.availableLayoutIDs()
		return nil, fmt.Errorf("%s", layoutNotFoundError(layoutID, available))
	}
	return layoutData, nil
}

// availableLayoutIDs returns a list of layout IDs available in the template ZIP and synthetic files.
func (ctx *singlePassContext) availableLayoutIDs() []string {
	seen := make(map[string]bool)
	var ids []string

	// Collect from template ZIP
	if ctx.templateReader != nil {
		for _, f := range ctx.templateReader.File {
			if strings.HasPrefix(f.Name, PathSlideLayouts) &&
				strings.HasSuffix(f.Name, ".xml") &&
				!strings.Contains(f.Name, "_rels/") {
				// Extract layout ID from path like "ppt/slideLayouts/slideLayout1.xml"
				name := f.Name[len(PathSlideLayouts):]
				name = strings.TrimSuffix(name, ".xml")
				if name != "" && !seen[name] {
					seen[name] = true
					ids = append(ids, name)
				}
			}
		}
	}

	// Collect from synthetic files
	for path := range ctx.syntheticFiles {
		if strings.HasPrefix(path, PathSlideLayouts) &&
			strings.HasSuffix(path, ".xml") &&
			!strings.Contains(path, "_rels/") {
			name := path[len(PathSlideLayouts):]
			name = strings.TrimSuffix(name, ".xml")
			if name != "" && !seen[name] {
				seen[name] = true
				ids = append(ids, name)
			}
		}
	}

	return ids
}

// readFileWithSyntheticFallback reads a file from synthetic files first, then the template ZIP.
func (ctx *singlePassContext) readFileWithSyntheticFallback(path string) ([]byte, error) {
	if data, ok := ctx.syntheticFiles[path]; ok {
		return data, nil
	}
	return utils.ReadFileFromZipIndex(ctx.templateIndex, path)
}

// createAndParseSlide creates a slide from layout data and returns the XML structure.
func (ctx *singlePassContext) createAndParseSlide(layoutData []byte, slideNum int, masterPositions map[string]*transformXML, layoutID string) (*slideXML, error) {
	slide, err := createSlideFromLayout(layoutData, slideNum, masterPositions)
	if err != nil {
		return nil, fmt.Errorf("failed to create slide from layout: %w", err)
	}

	// Adjust shapes that overlap with the detected logo zone.
	// Only apply the shift for layouts that actually contain a logo image.
	if ctx.logoZones != nil {
		if zone := ctx.logoZones[layoutID]; zone != nil {
			ctx.adjustShapesForLogoZone(slide, zone)
		}
	}

	return slide, nil
}

// adjustShapesForLogoZone shifts title and content placeholders to the right
// when their left edge falls inside the given logo zone. The width is reduced
// by the same amount so the right edge stays in the same position.
// Only shapes whose Y position overlaps vertically with the logo are affected;
// shapes well below the logo (e.g., footer) are left untouched.
func (ctx *singlePassContext) adjustShapesForLogoZone(slide *slideXML, zone *LogoZone) {
	if zone == nil {
		return
	}

	for i := range slide.CommonSlideData.ShapeTree.Shapes {
		shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
		xfrm := shape.ShapeProperties.Transform
		if xfrm == nil {
			continue
		}

		// Only adjust shapes that vertically overlap with the logo zone.
		// A shape overlaps if its top edge is above the logo bottom.
		if xfrm.Offset.Y >= zone.Bottom {
			continue
		}

		// Only adjust shapes whose left edge is inside the logo zone
		if xfrm.Offset.X >= zone.Right {
			continue
		}

		// Only adjust title and content placeholders (not footers, slide numbers, etc.)
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph == nil {
			continue
		}
		// After normalization, ctrTitle → title and implicit body (type="") → type="body".
		if ph.Type != "title" && ph.Type != "body" {
			continue
		}

		// Calculate how much to shift right
		shift := zone.Right - xfrm.Offset.X

		slog.Debug("adjusting shape for logo zone",
			slog.String("shape_name", shape.NonVisualProperties.ConnectionNonVisual.Name),
			slog.String("ph_type", ph.Type),
			slog.Int64("original_x", xfrm.Offset.X),
			slog.Int64("shift", shift),
			slog.Int64("new_x", xfrm.Offset.X+shift),
		)

		xfrm.Offset.X += shift
		if xfrm.Extent.CX > shift {
			xfrm.Extent.CX -= shift
		}
	}
}

// updatePresentationSlideList updates presentation.xml with the new slide entries.
func (ctx *singlePassContext) updatePresentationSlideList(presentationData []byte, newSlideEntries []string) {
	var updatedPresentation string
	if ctx.excludeTemplateSlides {
		updatedPresentation = replaceSlideListInPresentationXML(string(presentationData), newSlideEntries)
	} else {
		updatedPresentation = insertSlideEntriesIntoPresentationXML(string(presentationData), newSlideEntries)
	}
	ctx.modifiedFiles[PathPresentationXML] = []byte(updatedPresentation)
}

// getMasterPositionsForLayout retrieves placeholder positions from the slide master
// associated with the given layout. Results are cached to avoid re-parsing.
//
// This function delegates to template.ParseSlideMasterPositions for the actual parsing,
// then converts template.MasterTransform to transformXML for use by resolveEmptyTransforms.
func (ctx *singlePassContext) getMasterPositionsForLayout(layoutID string, cache map[string]map[string]*transformXML) map[string]*transformXML {
	// Find the master for this layout by reading the layout's relationships
	layoutRelsPath := LayoutRelsPath(layoutID)
	relsData, err := ctx.readFileWithSyntheticFallback(layoutRelsPath)
	if err != nil {
		slog.Debug("master position resolution failed: layout rels file not found",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.String("error", err.Error()),
		)
		return nil // No rels file, can't determine master
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		slog.Debug("master position resolution failed: layout rels XML parse error",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Find the slideMaster relationship
	var masterPath string
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			// Target is relative (e.g., "../slideMasters/slideMaster1.xml")
			// Convert to absolute path within the ZIP using template package's resolver
			// Note: Must strip trailing slash from PathSlideLayouts for correct "../" resolution
			basePath := strings.TrimSuffix(PathSlideLayouts, "/")
			masterPath = template.ResolveRelativePath(basePath, rel.Target)
			break
		}
	}

	if masterPath == "" {
		slog.Debug("master position resolution failed: no slideMaster relationship",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.Int("relationship_count", len(rels.Relationships)),
		)
		return nil // No master relationship found
	}

	// Check cache
	if positions, ok := cache[masterPath]; ok {
		slog.Debug("master positions resolved from cache",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.Int("position_count", len(positions)),
		)
		return positions
	}

	// Load and parse the master using the template package
	masterData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, masterPath)
	if err != nil {
		slog.Debug("master position resolution failed: master file not found",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Delegate to template package for parsing
	masterTransforms, err := template.ParseSlideMasterPositions(masterData)
	if err != nil {
		slog.Debug("master position resolution failed: master XML parse error",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Convert template.MasterTransform to transformXML for use by resolveEmptyTransforms
	positions := convertMasterTransformsToXML(masterTransforms)

	// Cache for future layouts using the same master
	cache[masterPath] = positions
	slog.Debug("master positions resolved successfully",
		slog.String("layout_id", layoutID),
		slog.String("master_path", masterPath),
		slog.Int("position_count", len(positions)),
	)
	return positions
}

// convertMasterTransformsToXML converts template.MasterTransform map to transformXML map.
// This bridges the template package's flat struct to the generator's nested struct.
func convertMasterTransformsToXML(masterTransforms map[string]*template.MasterTransform) map[string]*transformXML {
	positions := make(map[string]*transformXML, len(masterTransforms))
	for key, mt := range masterTransforms {
		positions[key] = &transformXML{
			Offset: offsetXML{X: mt.OffsetX, Y: mt.OffsetY},
			Extent: extentXML{CX: mt.ExtentCX, CY: mt.ExtentCY},
		}
	}
	return positions
}

// findMaxPresentationRelID finds the maximum relationship ID in the presentation's relationship file.
func (ctx *singlePassContext) findMaxPresentationRelID() int {
	relsFileName := PathPresentationRels
	maxID := 0

	relsData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, relsFileName)
	if err != nil {
		return maxID
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return maxID
	}

	for _, rel := range rels.Relationships {
		var num int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &num); err == nil {
			if num > maxID {
				maxID = num
			}
		}
	}

	return maxID
}

// populateTextInSlide populates text and bullet content in a slide.
// Image/chart content is skipped here and handled in prepareImages.
// layoutID is used to look up the slide master's bullet level configuration.
func (ctx *singlePassContext) populateTextInSlide(slide *slideXML, content []ContentItem, layoutID string) []string {
	var warnings []string
	resolver := newPlaceholderResolver(slide.CommonSlideData.ShapeTree.Shapes)
	warnings = append(warnings, resolver.warnings...)

	// Get the first bullet level from the slide master for this layout
	masterBulletLevel := ctx.getFirstBulletLevelForLayout(layoutID)

	for _, item := range content {
		// Skip visual content types - they are handled in prepareImages
		if item.Type == ContentImage || item.Type == ContentDiagram || item.Type == ContentTable {
			continue
		}

		shapeIdx, tier, found := resolver.ResolveWithFallback(item.PlaceholderID)
		if !found {
			available := resolver.Keys()
			slog.Warn("placeholder not found in layout",
				slog.String("placeholder_id", item.PlaceholderID),
				slog.String("layout_id", layoutID),
				slog.Any("available", available),
			)
			warnings = append(warnings, placeholderNotFoundError(item.PlaceholderID, layoutID, available))
			continue
		}

		// Log non-exact resolutions for observability.
		if tier != TierExact {
			resolvedName := slide.CommonSlideData.ShapeTree.Shapes[shapeIdx].NonVisualProperties.ConnectionNonVisual.Name
			logFallbackResolution(item.PlaceholderID, resolvedName, tier, layoutID)
		}

		shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
		if err := populateShapeText(shape, item, masterBulletLevel, ctx.themeFontName); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	return warnings
}

// clearUnmappedPlaceholders removes template sample text from placeholder shapes
// that were not targeted by any content item. This prevents layout placeholder text
// (e.g., "Click to add text") from leaking into the final output.
//
// Content placeholders (body, subtitle, obj, implicit) are always cleared if unmapped.
// Placeholders with hasCustomPrompt="1" are also cleared regardless of type, since
// their custom prompt text (e.g., "0" for section numbers) would otherwise render
// as literal content in the output.
// Non-placeholder shapes (decorative elements, logos) are never touched.
func (ctx *singlePassContext) clearUnmappedPlaceholders() {
	for slideNum, slide := range ctx.templateSlideData {
		// Build resolver to translate idx:N placeholder IDs to canonical shape names.
		resolver := newPlaceholderResolver(slide.CommonSlideData.ShapeTree.Shapes)

		// Build set of canonical shape names that were targeted by content items.
		// Uses ResolveWithFallback so that content using legacy names (e.g.,
		// "Content Placeholder 2") or idx:N syntax correctly marks the resolved
		// shape as populated, even when matched via semantic/fuzzy/positional tier.
		populated := make(map[string]bool)
		if spec, ok := ctx.slideContentMap[slideNum]; ok {
			for _, item := range spec.Content {
				if shapeIdx, _, ok := resolver.ResolveWithFallback(item.PlaceholderID); ok {
					shapeName := slide.CommonSlideData.ShapeTree.Shapes[shapeIdx].NonVisualProperties.ConnectionNonVisual.Name
					populated[shapeName] = true
				}
			}
		}

		shapes := slide.CommonSlideData.ShapeTree.Shapes
		for i := range shapes {
			shape := &shapes[i]
			ph := shape.NonVisualProperties.NvPr.Placeholder
			if ph == nil {
				continue // Not a placeholder — decorative shape, skip
			}

			// Determine if this placeholder should be cleared when unmapped.
			shouldClear := false
			switch ph.Type {
			case "body", "subTitle", "obj", "":
				// Content placeholders are always cleared if unmapped
				shouldClear = true
			case "title", "ctrTitle":
				// Title placeholders are always cleared if unmapped.
				// Without this, empty title shapes cause PowerPoint to render
				// the layout's prompt text (e.g., "Click to add title") which
				// leaks into the visible output.
				shouldClear = true
			default:
				// dt, ftr, sldNum, pic, chart, tbl, etc. — preserve
			}

			if !shouldClear {
				continue
			}

			// After normalization, the canonical shape name is the single lookup key.
			shapeName := shape.NonVisualProperties.ConnectionNonVisual.Name
			if !populated[shapeName] && shape.TextBody != nil && len(shape.TextBody.Paragraphs) > 0 {
				// Clear text content by replacing with a single empty paragraph.
				// This preserves the shape XML structure (needed for layout inheritance)
				// while removing the template sample text and hasCustomPrompt content
				// (e.g., the "0" section number placeholder).
				shape.TextBody.Paragraphs = []paragraphXML{emptyParagraph()}

				// Strip hasCustomPrompt from the slide's placeholder element.
				// This attribute is layout-only (ECMA-376 §19.3.1.36) — when
				// present in a slide, it causes PowerPoint to display the
				// layout's custom prompt text (e.g., "0" in 350pt orange for
				// section numbers) even though the slide's text body is empty.
				if ph.HasCustomPrompt != "" {
					ph.HasCustomPrompt = ""
				}
			}
		}
	}
}
