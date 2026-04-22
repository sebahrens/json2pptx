// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// regexSldLayoutID extracts the id attribute from <p:sldLayoutId id="NNN" .../> entries.
var regexSldLayoutID = regexp.MustCompile(`<p:sldLayoutId\b[^>]*\bid="(\d+)"`)


// writeOutput performs the single-pass write operation.
// This orchestrates the writing of all components in the correct order.
//
// The function has been decomposed into smaller, focused methods:
//   - writeTemplateFiles: copies unchanged template files
//   - writeSlides: writes presentation.xml and new slides
//   - writeRelationships: writes all relationship files
//   - writeContentTypes: writes [Content_Types].xml
//   - writeMediaFiles: writes all media (images, charts, SVGs)
func (ctx *singlePassContext) writeOutput() error {
	// Pre-allocate relationship IDs for media inserts (images, charts, infographics)
	// This must be done before writing slides so p:pic elements have correct rIds
	if err := ctx.allocateMediaRelIDs(); err != nil {
		return fmt.Errorf("failed to allocate media relationship IDs: %w", err)
	}

	// Pre-allocate relationship IDs for native SVG inserts
	// This must be done before writing slides so p:pic elements have correct rIds
	if err := ctx.allocateNativeSVGRelIDs(); err != nil {
		return fmt.Errorf("failed to allocate native SVG relationship IDs: %w", err)
	}

	// Pre-allocate relationship IDs for panel icon images and generate final group XML.
	// This must be done after allocateNativeSVGRelIDs() so rel IDs don't conflict.
	ctx.allocatePanelIconRelIDs()

	// Pre-allocate relationship IDs for background images.
	// This must be done after all other rel ID allocations so IDs don't conflict.
	ctx.allocateBackgroundRelIDs()

	// Step 1: Copy unchanged template files
	if err := ctx.writeTemplateFiles(); err != nil {
		return err
	}

	// Step 1.5: Write synthetic layout files (from SynthesisManifest)
	if err := ctx.writeSyntheticFiles(); err != nil {
		return err
	}

	// Step 1.6: Write updated docProps/app.xml (if modified)
	if err := ctx.writeDocPropsApp(); err != nil {
		return err
	}

	// Step 2: Write modified presentation.xml and new slides
	if err := ctx.writeSlides(); err != nil {
		return err
	}

	// Step 3: Write all relationships files
	if err := ctx.writeRelationships(); err != nil {
		return err
	}

	// Step 4: Write [Content_Types].xml with image extensions
	if err := ctx.writeContentTypes(); err != nil {
		return err
	}

	// Step 5: Write media files (images, charts, SVGs)
	if err := ctx.writeMediaFiles(); err != nil {
		return err
	}

	// Step 6: Write speaker notes slides (notesSlide XML + rels)
	if err := ctx.writeNotesSlides(); err != nil {
		return err
	}

	return nil
}

// writeTemplateFiles copies unchanged files from the template to the output ZIP.
// It skips files that will be written separately (presentation.xml, slides, relationships).
// When excludeTemplateSlides is true, all template slide files are skipped.
func (ctx *singlePassContext) writeTemplateFiles() error { //nolint:gocognit,gocyclo
	for _, f := range ctx.templateReader.File {
		// Skip presentation.xml - we have a modified version
		if f.Name == PathPresentationXML {
			continue
		}

		// Skip docProps/app.xml when we have an updated version
		if f.Name == PathDocPropsApp {
			if _, ok := ctx.modifiedFiles[PathDocPropsApp]; ok {
				continue
			}
		}

		// Skip presentation.xml.rels - we'll update it with new slide relationships
		if f.Name == PathPresentationRels {
			continue
		}

		// Skip [Content_Types].xml - we'll update it
		if f.Name == PathContentTypes {
			continue
		}

		// Check if this is a template slide file
		if slideNum, ok := parseSlideNum(f.Name); ok {
			// This is a slide in the template
			if ctx.excludeTemplateSlides && slideNum <= ctx.existingSlides {
				// Skip all template slides when excluding them
				continue
			}
			if slideNum > ctx.existingSlides {
				// This shouldn't happen since we're creating new slides
				// but skip just in case
				continue
			}
		}

		// Check if this is a relationships file for a template slide
		if relSlideNum, ok := parseSlideRelsNum(f.Name); ok {
			// Skip template slide relationships when excluding template slides
			if ctx.excludeTemplateSlides && relSlideNum <= ctx.existingSlides {
				continue
			}
			if _, hasUpdates := ctx.slideRelUpdates[relSlideNum]; hasUpdates {
				// We'll write this later with updates
				continue
			}
			if _, hasNativeSVG := ctx.nativeSVGInserts[relSlideNum]; hasNativeSVG {
				// We'll write this later with native SVG relationships
				continue
			}
			if _, hasPanel := ctx.panelShapeInserts[relSlideNum]; hasPanel {
				// We'll write this later with panel icon relationships
				continue
			}
		}

		// Skip notes slide files that will be re-written by writeNotesSlides().
		// Templates like template_2 have existing notesSlide files; writing them
		// again creates duplicate ZIP entries that crash LibreOffice.
		if strings.HasPrefix(f.Name, PathNotesSlides) {
			if noteSlideNum, ok := parseNotesSlideNum(f.Name); ok {
				if _, willRewrite := ctx.slideNotes[noteSlideNum]; willRewrite {
					continue
				}
			}
			if noteSlideNum, ok := parseNotesSlideRelsNum(f.Name); ok {
				if _, willRewrite := ctx.slideNotes[noteSlideNum]; willRewrite {
					continue
				}
			}
		}

		// Skip files that have been placed in syntheticFiles (e.g. normalized
		// slideLayout XMLs). writeSyntheticFiles() will write the updated
		// version; copying the template original here would create duplicate
		// ZIP entries that violate OPC and trigger PowerPoint's repair dialog.
		if _, willRewrite := ctx.syntheticFiles[f.Name]; willRewrite {
			continue
		}

		// When synthetic layouts exist, intercept slide master rels files
		// to add relationships for the synthetic layouts.
		if len(ctx.syntheticFiles) > 0 && strings.HasSuffix(f.Name, ".xml.rels") &&
			strings.Contains(f.Name, "slideMasters/_rels/") {
			if err := ctx.writeSlideMasterRelsWithSynthetic(f); err != nil {
				return fmt.Errorf("failed to write updated %s: %w", f.Name, err)
			}
			continue
		}

		// When synthetic layouts exist, intercept slide master XML files
		// to add <p:sldLayoutId> entries in <p:sldLayoutIdLst>.
		if len(ctx.syntheticFiles) > 0 && strings.HasSuffix(f.Name, ".xml") &&
			strings.Contains(f.Name, "ppt/slideMasters/") &&
			!strings.Contains(f.Name, "_rels/") {
			if err := ctx.writeSlideMasterXMLWithSynthetic(f); err != nil {
				return fmt.Errorf("failed to write updated %s: %w", f.Name, err)
			}
			continue
		}

		// Copy unchanged file
		if err := utils.CopyZipFile(ctx.outputWriter, f); err != nil {
			return fmt.Errorf("failed to copy %s: %w", f.Name, err)
		}
	}
	return nil
}

// writeSyntheticFiles writes synthetic layout files from SynthesisManifest to the output ZIP.
// These are layout XML and .rels files generated during template analysis when the template
// lacks required capabilities (e.g., two-column layouts).
func (ctx *singlePassContext) writeSyntheticFiles() error {
	if len(ctx.syntheticFiles) == 0 {
		return nil
	}

	// Sort paths for deterministic output ordering
	paths := make([]string, 0, len(ctx.syntheticFiles))
	for path := range ctx.syntheticFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		fw, err := utils.ZipCreateDeterministic(ctx.outputWriter, path)
		if err != nil {
			return fmt.Errorf("failed to create synthetic file %s: %w", path, err)
		}
		if _, err := fw.Write(ctx.syntheticFiles[path]); err != nil {
			return fmt.Errorf("failed to write synthetic file %s: %w", path, err)
		}
	}
	return nil
}

// writeDocPropsApp writes the updated docProps/app.xml from modifiedFiles (if present).
func (ctx *singlePassContext) writeDocPropsApp() error {
	data, ok := ctx.modifiedFiles[PathDocPropsApp]
	if !ok {
		return nil
	}
	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter, PathDocPropsApp)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", PathDocPropsApp, err)
	}
	if _, err := fw.Write(data); err != nil {
		return fmt.Errorf("failed to write %s: %w", PathDocPropsApp, err)
	}
	return nil
}

// updateAppProperties reads docProps/app.xml from the template, updates the slide count
// and word/paragraph estimates, and stores the result in modifiedFiles.
func (ctx *singlePassContext) updateAppProperties(slideCount int) {
	data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, PathDocPropsApp)
	if err != nil {
		// No app.xml in template — nothing to update
		return
	}

	// Use lenient parsing: preserve the full XML as a string and only update
	// the specific elements we need to change. This preserves template-specific
	// extended properties (Company, TitlesOfParts, etc.) that a strict struct
	// would lose.
	content := string(data)

	// Count words and paragraphs from slide content
	words, paragraphs := ctx.countWordsAndParagraphs()

	content = replaceOrInsertXMLElement(content, "Slides", fmt.Sprintf("%d", slideCount))
	content = replaceOrInsertXMLElement(content, "Words", fmt.Sprintf("%d", words))
	content = replaceOrInsertXMLElement(content, "Paragraphs", fmt.Sprintf("%d", paragraphs))
	content = replaceOrInsertXMLElement(content, "HiddenSlides", "0")
	content = replaceOrInsertXMLElement(content, "TotalTime", "0")

	ctx.modifiedFiles[PathDocPropsApp] = []byte(content)

	slog.Debug("updated docProps/app.xml",
		slog.Int("slides", slideCount),
		slog.Int("words", words),
		slog.Int("paragraphs", paragraphs))
}

// countWordsAndParagraphs estimates word and paragraph counts from slide content.
func (ctx *singlePassContext) countWordsAndParagraphs() (words, paragraphs int) {
	for _, spec := range ctx.slideSpecs {
		for _, item := range spec.Content {
			switch item.Type {
			case ContentText:
				if text, ok := item.Value.(string); ok && text != "" {
					words += countWords(text)
					paragraphs++
				}
			case ContentBullets:
				if bullets, ok := item.Value.([]string); ok {
					for _, b := range bullets {
						words += countWords(b)
						paragraphs++
					}
				}
			}
		}
	}
	return words, paragraphs
}

// countWords counts whitespace-delimited tokens in a string.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// replaceOrInsertXMLElement replaces <Tag>...</Tag> with <Tag>value</Tag>,
// or inserts it before </Properties> if not present.
func replaceOrInsertXMLElement(xml, tag, value string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	replacement := openTag + value + closeTag

	// Try to find and replace existing element
	startIdx := strings.Index(xml, openTag)
	if startIdx != -1 {
		endIdx := strings.Index(xml[startIdx:], closeTag)
		if endIdx != -1 {
			return xml[:startIdx] + replacement + xml[startIdx+endIdx+len(closeTag):]
		}
	}

	// Element not found — insert before </Properties>
	closingProps := "</Properties>"
	insertPos := strings.LastIndex(xml, closingProps)
	if insertPos == -1 {
		return xml // Can't find closing tag; return unchanged
	}
	return xml[:insertPos] + "  " + replacement + "\n" + xml[insertPos:]
}

// writeSlideMasterRelsWithSynthetic reads a slide master .rels file from the template,
// appends relationships for all synthetic layouts, and writes the updated version.
func (ctx *singlePassContext) writeSlideMasterRelsWithSynthetic(f *zip.File) error {
	// Read original rels data
	data, err := utils.ReadFileFromZipIndex(ctx.templateIndex, f.Name)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", f.Name, err)
	}

	// Parse existing relationships
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		return fmt.Errorf("failed to parse %s: %w", f.Name, err)
	}

	// Find the highest existing rId number
	maxRID := 0
	for _, rel := range rels.Relationships {
		if strings.HasPrefix(rel.ID, "rId") {
			if num, err := strconv.Atoi(rel.ID[3:]); err == nil && num > maxRID {
				maxRID = num
			}
		}
	}

	// Collect synthetic layout filenames (sorted for deterministic rId assignment)
	layoutNames := ctx.syntheticLayoutNames()

	// Add a relationship for each synthetic layout
	for _, name := range layoutNames {
		maxRID++
		rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
			ID:     fmt.Sprintf("rId%d", maxRID),
			Type:   pptx.RelTypeSlideLayout,
			Target: "../slideLayouts/" + name,
		})
	}

	// Marshal and write
	output, err := xml.Marshal(rels)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", f.Name, err)
	}

	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter, f.Name)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", f.Name, err)
	}
	if _, err := fw.Write([]byte(xml.Header)); err != nil {
		return err
	}
	if _, err := fw.Write(output); err != nil {
		return err
	}

	slog.Debug("updated slide master rels with synthetic layouts",
		slog.String("path", f.Name),
		slog.Int("synthetic_count", len(layoutNames)))

	return nil
}

// syntheticLayoutNames returns the sorted list of truly new synthetic layout XML
// filenames (e.g., ["slideLayout99.xml"]). Layouts that already exist in the
// template (e.g., normalized existing layouts) are excluded — they don't need
// new relationships. This is used by both the .rels writer and the master XML
// writer to ensure consistent ordering and rId assignment.
func (ctx *singlePassContext) syntheticLayoutNames() []string {
	var names []string
	for path := range ctx.syntheticFiles {
		if strings.HasPrefix(path, "ppt/slideLayouts/") && strings.HasSuffix(path, ".xml") &&
			!strings.Contains(path, "_rels/") {
			// Skip layouts that already exist in the template — these were
			// merely normalized, not newly created, so they already have
			// relationships in the slide master rels.
			if _, exists := ctx.templateIndex[path]; exists {
				continue
			}
			names = append(names, strings.TrimPrefix(path, "ppt/slideLayouts/"))
		}
	}
	sort.Strings(names)
	return names
}

// writeSlideMasterXMLWithSynthetic reads a slide master XML from the template,
// appends <p:sldLayoutId> entries in <p:sldLayoutIdLst> for all synthetic layouts,
// and writes the updated version. The r:id values are derived using the same logic
// as writeSlideMasterRelsWithSynthetic to ensure consistency.
func (ctx *singlePassContext) writeSlideMasterXMLWithSynthetic(f *zip.File) error {
	layoutNames := ctx.syntheticLayoutNames()
	if len(layoutNames) == 0 {
		// No synthetic layout XMLs; just copy the master file unchanged
		return utils.CopyZipFile(ctx.outputWriter, f)
	}

	// Read the corresponding .rels file to compute the same rId assignments.
	// Path: ppt/slideMasters/slideMaster1.xml → ppt/slideMasters/_rels/slideMaster1.xml.rels
	baseName := f.Name[strings.LastIndex(f.Name, "/")+1:]
	relsPath := strings.Replace(f.Name, baseName, "_rels/"+baseName+".rels", 1)

	relsData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, relsPath)
	if err != nil {
		// No .rels file found; copy master unchanged (shouldn't happen in valid PPTX)
		slog.Warn("no rels file found for slide master, copying unchanged",
			slog.String("master", f.Name), slog.String("expected_rels", relsPath))
		return utils.CopyZipFile(ctx.outputWriter, f)
	}

	// Find the max existing rId (same logic as writeSlideMasterRelsWithSynthetic)
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		return fmt.Errorf("failed to parse %s: %w", relsPath, err)
	}
	maxRID := 0
	for _, rel := range rels.Relationships {
		if strings.HasPrefix(rel.ID, "rId") {
			if num, err := strconv.Atoi(rel.ID[3:]); err == nil && num > maxRID {
				maxRID = num
			}
		}
	}

	// Build rId map: layoutName → rId (same assignment order as writeSlideMasterRelsWithSynthetic)
	rIDMap := make(map[string]string, len(layoutNames))
	for i, name := range layoutNames {
		rIDMap[name] = fmt.Sprintf("rId%d", maxRID+i+1)
	}

	// Read the master XML
	masterData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, f.Name)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", f.Name, err)
	}

	// Find max existing layout ID in <p:sldLayoutIdLst>
	// Layout IDs are in the 2147483648+ range (0x80000000+)
	maxLayoutID := int64(2147483648)
	masterStr := string(masterData)

	// Extract existing id values from <p:sldLayoutId id="NNN" .../>
	for _, match := range regexSldLayoutID.FindAllStringSubmatch(masterStr, -1) {
		if id, err := strconv.ParseInt(match[1], 10, 64); err == nil && id > maxLayoutID {
			maxLayoutID = id
		}
	}

	// Build the new entries
	var entries strings.Builder
	for _, name := range layoutNames {
		maxLayoutID++
		fmt.Fprintf(&entries, `<p:sldLayoutId id="%d" r:id="%s"/>`, maxLayoutID, rIDMap[name])
	}

	// Find </p:sldLayoutIdLst> and insert before it
	insertTag := "</p:sldLayoutIdLst>"
	insertPos := strings.Index(masterStr, insertTag)
	if insertPos == -1 {
		// No <p:sldLayoutIdLst> found — this is unusual but handle gracefully
		slog.Warn("no <p:sldLayoutIdLst> found in slide master, copying unchanged",
			slog.String("path", f.Name))
		return utils.CopyZipFile(ctx.outputWriter, f)
	}

	// Splice in the new entries
	modified := masterStr[:insertPos] + entries.String() + masterStr[insertPos:]

	// Write the modified master XML
	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter, f.Name)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", f.Name, err)
	}
	if _, err := io.WriteString(fw, modified); err != nil {
		return err
	}

	slog.Debug("updated slide master XML with synthetic layout IDs",
		slog.String("path", f.Name),
		slog.Int("synthetic_count", len(layoutNames)))

	return nil
}

// writeSlides writes the modified presentation.xml and all new slides to the output ZIP.
func (ctx *singlePassContext) writeSlides() error {
	// Write modified presentation.xml
	if data, ok := ctx.modifiedFiles[PathPresentationXML]; ok {
		fw, err := utils.ZipCreateDeterministic(ctx.outputWriter,PathPresentationXML)
		if err != nil {
			return fmt.Errorf("failed to create presentation.xml: %w", err)
		}
		if _, err := fw.Write(data); err != nil {
			return fmt.Errorf("failed to write presentation.xml: %w", err)
		}
	}

	// Write presentation.xml.rels with new slide relationships
	if err := ctx.writePresentationRelationships(); err != nil {
		return err
	}

	// Write new slides in deterministic order (sorted by slide number)
	slideNums := make([]int, 0, len(ctx.templateSlideData))
	for slideNum := range ctx.templateSlideData {
		slideNums = append(slideNums, slideNum)
	}
	sort.Ints(slideNums)

	for _, slideNum := range slideNums {
		if err := ctx.writeSingleSlide(slideNum, ctx.templateSlideData[slideNum]); err != nil {
			return err
		}
	}
	return nil
}

// writePresentationRelationships writes ppt/_rels/presentation.xml.rels with new slide relationships.
// Each new slide needs a relationship entry in this file to be recognized by PowerPoint/LibreOffice.
// When excludeTemplateSlides is true, template slide relationships are removed.
func (ctx *singlePassContext) writePresentationRelationships() error {
	// Skip if no template reader (e.g., in tests that don't set up full context)
	if ctx.templateReader == nil {
		return nil
	}

	// Skip if no new slides to add
	if len(ctx.slideSpecs) == 0 {
		return nil
	}

	relsFileName := PathPresentationRels

	// Read existing relationships from template
	existingRelsData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, relsFileName)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", relsFileName, err)
	}

	// Parse existing relationships
	var existingRels pptx.RelationshipsXML
	if err := xml.Unmarshal(existingRelsData, &existingRels); err != nil {
		return fmt.Errorf("failed to parse %s: %w", relsFileName, err)
	}

	// When excluding template slides, filter out slide relationships
	if ctx.excludeTemplateSlides {
		filteredRels := make([]pptx.RelationshipXML, 0, len(existingRels.Relationships))
		for _, rel := range existingRels.Relationships {
			// Keep non-slide relationships (masters, layouts, themes, etc.)
			if rel.Type != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" {
				filteredRels = append(filteredRels, rel)
			}
		}
		existingRels.Relationships = filteredRels
	}

	// Calculate starting slide number
	startSlideNum := ctx.existingSlides + 1
	if ctx.excludeTemplateSlides {
		startSlideNum = 1
	}

	// Add relationships for each new slide
	// Relationship IDs must match those used in presentation.xml's p:sldIdLst
	// The rId mapping is built in prepareSlides to ensure unique IDs
	for i := range ctx.slideSpecs {
		slideNum := startSlideNum + i
		rID := ctx.slideRelIDs[slideNum]
		existingRels.Relationships = append(existingRels.Relationships, pptx.RelationshipXML{
			ID:     rID,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide",
			Target: fmt.Sprintf("slides/slide%d.xml", slideNum), // Relative to ppt/_rels/
		})
	}

	// Marshal and write
	relsOutput, err := xml.Marshal(existingRels)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", relsFileName, err)
	}

	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter,relsFileName)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", relsFileName, err)
	}
	if _, err := fw.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("failed to write XML header for %s: %w", relsFileName, err)
	}
	if _, err := fw.Write(relsOutput); err != nil {
		return fmt.Errorf("failed to write %s: %w", relsFileName, err)
	}

	return nil
}

// writeSingleSlide writes a single slide to the output ZIP, handling media and native SVG inserts.
func (ctx *singlePassContext) writeSingleSlide(slideNum int, slide *slideXML) error { //nolint:gocognit,gocyclo
	slidePath := SlidePath(slideNum)

	// Check for table inserts (standalone tables replacing placeholder shapes)
	tableInserts, hasTable := ctx.tableInserts[slideNum]
	if hasTable {
		slide = ctx.removeTablePlaceholders(slide, tableInserts)
	}

	// Check for media inserts (images, charts, infographics)
	mediaRels, hasMedia := ctx.slideRelUpdates[slideNum]
	if hasMedia {
		// Remove placeholder shapes that are being replaced by p:pic elements
		slide = ctx.removeMediaPlaceholders(slide, mediaRels)
	}

	// Check for native panel shape inserts (panel columns rendered as OOXML groups)
	panelInserts, hasPanel := ctx.panelShapeInserts[slideNum]
	if hasPanel {
		slide = ctx.removePanelPlaceholders(slide, panelInserts)
	}

	// Check if this slide has native SVG inserts
	nativeSVGs, hasNativeSVG := ctx.nativeSVGInserts[slideNum]
	if hasNativeSVG {
		// Remove placeholder shapes that are being replaced by native SVG
		slide = ctx.removeNativeSVGPlaceholders(slide, nativeSVGs)
	}

	// Remove template footer placeholder shapes (dt, ftr, sldNum).
	// When our footer feature is enabled, remove ALL footer placeholders
	// (we inject our own populated footer shapes later as raw XML).
	// When footers are disabled, remove only EMPTY footer placeholders
	// to prevent blank shapes from reserving space at the bottom of the slide.
	if ctx.footerConfig != nil && ctx.footerConfig.Enabled {
		slide = removeFooterPlaceholders(slide)
		// Remove layout-inherited brand mark shapes that would duplicate LeftText.
		// Some templates include static brand-mark text shapes in layouts
		// that overlap with the footer's left text zone.
		slide = removeDuplicateFooterBrandMark(slide, ctx.footerConfig.LeftText, ctx.slideHeight)
	} else {
		slide = removeEmptyFooterPlaceholders(slide)
	}

	// Remove any remaining empty placeholder shapes that were not populated
	// with content. Layout-inherited placeholders (e.g., subtitle on closing
	// slides) can appear as blank shapes that overlap populated content in
	// PowerPoint, even though LibreOffice ignores them.
	slide = removeEmptyPlaceholders(slide)

	// Marshal slide XML (no indentation — saves ~15-25% marshal time in ZIP archives)
	slideData, err := xml.Marshal(slide)
	if err != nil {
		return fmt.Errorf("failed to marshal slide %d: %w", slideNum, err)
	}

	// Insert table graphicFrame elements (replacing removed placeholder shapes)
	if hasTable {
		slideData, err = insertTableFrames(slideData, tableInserts)
		if err != nil {
			return fmt.Errorf("failed to insert table frames for slide %d: %w", slideNum, err)
		}
	}

	// Insert p:pic elements for regular media (images, charts, infographics)
	// We do this by string manipulation after marshaling (since our XML structs
	// don't include p:pic support, and the pptx.GeneratePic generates proper XML)
	if hasMedia {
		slideData, err = ctx.insertMediaPics(slideNum, slideData, mediaRels)
		if err != nil {
			return fmt.Errorf("failed to insert media pics for slide %d: %w", slideNum, err)
		}
	}

	// Insert native panel group shapes (p:grpSp elements)
	if hasPanel {
		slideData, err = insertPanelGroups(slideData, panelInserts)
		if err != nil {
			return fmt.Errorf("failed to insert panel groups for slide %d: %w", slideNum, err)
		}
	}

	// Insert raw shape XML fragments (e.g., from shape_grid)
	if spec, ok := ctx.slideContentMap[slideNum]; ok && len(spec.RawShapeXML) > 0 {
		shapes := spec.RawShapeXML
		// Enforce WCAG AA text contrast within each shape_grid cell,
		// unless the slide opts out via contrast_check: false.
		if spec.ContrastCheck == nil || *spec.ContrastCheck {
			shapes = enforceShapeGridContrast(shapes, ctx.themeColors)
		}
		slideData, err = insertRawShapes(slideData, shapes)
		if err != nil {
			return fmt.Errorf("failed to insert raw shapes for slide %d: %w", slideNum, err)
		}
	}

	// Insert native SVG icon pics AFTER raw shapes so icons render on top
	// of the accent-colored shape_grid cells (later in spTree = higher z-order)
	if hasNativeSVG {
		slideData, err = ctx.insertNativeSVGPics(slideNum, slideData, nativeSVGs)
		if err != nil {
			return fmt.Errorf("failed to insert native SVG pics for slide %d: %w", slideNum, err)
		}
	}

	// Insert source attribution text shape if present
	if sourceText, hasSource := ctx.slideSources[slideNum]; hasSource {
		slideData, err = insertSourceNote(slideData, sourceText)
		if err != nil {
			return fmt.Errorf("failed to insert source note for slide %d: %w", slideNum, err)
		}
	}

	// Insert footer shapes if enabled (positions resolved per-layout)
	if ctx.footerConfig != nil && ctx.footerConfig.Enabled {
		var footerPositions map[string]*transformXML
		if spec, ok := ctx.slideContentMap[slideNum]; ok {
			footerPositions = ctx.getFooterPositionsForLayout(spec.LayoutID)
		}
		slideData, err = insertFooters(slideData, ctx.footerConfig, footerPositions)
		if err != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to insert footer for slide %d: %v", slideNum, err))
		}
	}

	// Insert transition and build animation XML if present
	if spec, ok := ctx.slideContentMap[slideNum]; ok {
		slideData = insertTransitionAndBuild(slideData, spec.Transition, spec.TransitionSpeed, spec.Build)
	}

	// Fix OOXML namespace prefixes for compatibility with LibreOffice/PowerPoint
	slideData = fixOOXMLNamespaces(slideData)

	// Insert background image after namespace fix (uses already-prefixed p: and a: tags)
	if bgMedia, hasBg := ctx.slideBgMedia[slideNum]; hasBg && bgMedia.relID != "" {
		slideData = insertBackgroundImage(slideData, bgMedia.relID)
	}

	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter,slidePath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", slidePath, err)
	}
	if _, err := fw.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("failed to write XML header: %w", err)
	}
	if _, err := fw.Write(slideData); err != nil {
		return fmt.Errorf("failed to write slide data: %w", err)
	}
	return nil
}

// removeTablePlaceholders removes placeholder shapes that are being replaced by table graphicFrame elements.
func (ctx *singlePassContext) removeTablePlaceholders(slide *slideXML, tables []tableInsert) *slideXML {
	removeIdxs := make(map[int]bool)
	for _, t := range tables {
		removeIdxs[t.placeholderIdx] = true
	}
	filteredShapes := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for i, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		if !removeIdxs[i] {
			filteredShapes = append(filteredShapes, shape)
		}
	}
	slide.CommonSlideData.ShapeTree.Shapes = filteredShapes
	return slide
}

// insertTableFrames inserts <p:graphicFrame> elements for tables before </p:spTree>.
func insertTableFrames(slideData []byte, tables []tableInsert) ([]byte, error) {
	var frames []string
	for _, t := range tables {
		frames = append(frames, t.graphicFrameXML)
	}

	insertion := strings.Join(frames, "\n")
	return pptx.InsertIntoSpTree(slideData, []byte(insertion), pptx.InsertAtEnd)
}

// removeMediaPlaceholders removes placeholder shapes that are being replaced by p:pic elements.
// Returns a modified copy of the slide to avoid mutating the original.
func (ctx *singlePassContext) removeMediaPlaceholders(slide *slideXML, mediaRels []mediaRel) *slideXML {
	// Build set of indices to remove
	removeIdxs := make(map[int]bool)
	for _, mr := range mediaRels {
		removeIdxs[mr.placeholderIdx] = true
	}

	// Filter shapes, keeping only those not being removed
	filteredShapes := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for i, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		if !removeIdxs[i] {
			filteredShapes = append(filteredShapes, shape)
		}
	}
	slide.CommonSlideData.ShapeTree.Shapes = filteredShapes
	return slide
}

// insertMediaPics inserts p:pic elements for media content (images, charts, infographics).
// It finds the closing </p:spTree> tag and inserts the p:pic elements before it.
// Relationship IDs are already assigned during writeNewSlideRelationships.
func (ctx *singlePassContext) insertMediaPics(slideNum int, slideData []byte, mediaRels []mediaRel) ([]byte, error) {
	slog.Debug("insertMediaPics: processing media relationships",
		slog.Int("slide_num", slideNum),
		slog.Int("media_rel_count", len(mediaRels)))
	for i, mr := range mediaRels {
		slog.Debug("insertMediaPics: media relationship details",
			slog.Int("index", i),
			slog.String("rel_id", mr.relID),
			slog.String("media_file_name", mr.mediaFileName),
			slog.Int("placeholder_idx", mr.placeholderIdx),
			slog.Int64("extent_cx", mr.extentCX),
			slog.Int64("extent_cy", mr.extentCY))
	}

	// Find the next shape ID using pre-compiled regex (faster than O(n) string scan)
	maxID := findMaxShapeID(slideData)
	nextShapeID := maxID + 1
	if nextShapeID < 100 {
		nextShapeID = 100 // Start high to avoid conflicts with typical OOXML IDs
	}

	// Generate p:pic elements for each media item
	var picsToInsert []string
	for i := range mediaRels {
		mr := &mediaRels[i]
		mr.shapeID = nextShapeID
		nextShapeID++

		// Skip if no dimensions (shape wasn't properly configured)
		if mr.extentCX == 0 && mr.extentCY == 0 {
			continue
		}

		// Generate p:pic XML using the pptx package
		// relID is assigned during writeNewSlideRelationships (rId2, rId3, etc.)
		picXML, err := pptx.GeneratePicSimpleNoNS(
			mr.shapeID,
			mr.relID,
			mr.offsetX,
			mr.offsetY,
			mr.extentCX,
			mr.extentCY,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate p:pic for media: %w", err)
		}
		picsToInsert = append(picsToInsert, string(picXML))
	}

	// Store the updated media rels back (with shape IDs)
	ctx.slideRelUpdates[slideNum] = mediaRels

	if len(picsToInsert) == 0 {
		return slideData, nil
	}

	// Insert the p:pic elements before </p:spTree>
	insertion := strings.Join(picsToInsert, "\n")
	return pptx.InsertIntoSpTree(slideData, []byte(insertion), pptx.InsertAtEnd)
}

// removeNativeSVGPlaceholders removes placeholder shapes that are being replaced by native SVG.
// Returns a modified copy of the slide to avoid mutating the original.
func (ctx *singlePassContext) removeNativeSVGPlaceholders(slide *slideXML, nativeSVGs []nativeSVGInsert) *slideXML {
	// Build set of indices to remove
	removeIdxs := make(map[int]bool)
	for _, svg := range nativeSVGs {
		removeIdxs[svg.placeholderIdx] = true
	}

	// Filter shapes, keeping only those not being removed
	filteredShapes := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for i, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		if !removeIdxs[i] {
			filteredShapes = append(filteredShapes, shape)
		}
	}
	slide.CommonSlideData.ShapeTree.Shapes = filteredShapes
	return slide
}

// writeRelationships writes all relationship files for slides that have media or native SVG inserts.
func (ctx *singlePassContext) writeRelationships() error {
	// Write relationships for ALL new slides in deterministic order
	relSlideNums := make([]int, 0, len(ctx.slideContentMap))
	for slideNum := range ctx.slideContentMap {
		relSlideNums = append(relSlideNums, slideNum)
	}
	sort.Ints(relSlideNums)

	for _, slideNum := range relSlideNums {
		if err := ctx.writeNewSlideRelationships(slideNum, ctx.slideContentMap[slideNum].LayoutID); err != nil {
			return err
		}
	}
	return nil
}

// writeNewSlideRelationships writes the relationships file for a new slide.
// Every slide needs at minimum a relationship to its slide layout.
func (ctx *singlePassContext) writeNewSlideRelationships(slideNum int, layoutID string) error {
	relsFileName := SlideRelsPath(slideNum)

	// Start with base relationship to layout
	rels := pptx.RelationshipsXML{
		XMLName: xml.Name{Space: pptx.NsPackageRels, Local: "Relationships"},
		Relationships: []pptx.RelationshipXML{
			{
				ID:     "rId1",
				Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout",
				Target: fmt.Sprintf("../slideLayouts/%s.xml", layoutID), // Relative to ppt/slides/_rels/
			},
		},
	}

	nextRelID := 2

	// Add relationships for regular media if present and assign relIDs
	if mediaRels, hasMedia := ctx.slideRelUpdates[slideNum]; hasMedia {
		for i := range mediaRels {
			relID := fmt.Sprintf("rId%d", nextRelID)
			mediaRels[i].relID = relID // Store relID for p:pic generation
			rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
				ID:     relID,
				Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
				Target: "../media/" + mediaRels[i].mediaFileName,
			})
			nextRelID++
		}
		// Update the slice back since we modified it
		ctx.slideRelUpdates[slideNum] = mediaRels
	}

	// Add relationships for native SVG inserts if present.
	// Use the pre-allocated rIds from allocateNativeSVGRelIDs() to stay in sync
	// with the p:pic elements already written to slide XML.
	if nativeSVGs, hasNativeSVG := ctx.nativeSVGInserts[slideNum]; hasNativeSVG {
		for _, svg := range nativeSVGs {
			rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
				ID:     svg.pngRelID,
				Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
				Target: "../media/" + svg.pngMediaFile,
			})
			rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
				ID:     svg.svgRelID,
				Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
				Target: "../media/" + svg.svgMediaFile,
			})
		}
		// Advance nextRelID past the pre-allocated SVG rIds (2 per insert: PNG + SVG)
		nextRelID += len(nativeSVGs) * 2
	}

	// Add relationships for panel icon images if present.
	// Use the pre-allocated rIds from allocatePanelIconRelIDs().
	if panelInserts, hasPanel := ctx.panelShapeInserts[slideNum]; hasPanel {
		for _, ins := range panelInserts {
			for _, panel := range ins.panels {
				if panel.iconRelID == "" {
					continue
				}
				rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
					ID:     panel.iconRelID,
					Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
					Target: "../media/" + panel.iconMediaFile,
				})
				nextRelID++
			}
		}
	}

	// Add relationship for background image if present (uses pre-allocated relID)
	if bgMedia, hasBg := ctx.slideBgMedia[slideNum]; hasBg && bgMedia.relID != "" {
		rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
			ID:     bgMedia.relID,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			Target: "../media/" + bgMedia.mediaFileName,
		})
		nextRelID++
	}

	// Add relationship to notes slide if this slide has speaker notes
	if _, hasNotes := ctx.slideNotes[slideNum]; hasNotes {
		rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
			ID:     fmt.Sprintf("rId%d", nextRelID),
			Type:   pptx.RelTypeNotesSlide,
			Target: fmt.Sprintf("../notesSlides/notesSlide%d.xml", slideNum),
		})
	}

	return ctx.writeRelationshipsFile(relsFileName, rels)
}

// writeNativeSVGOnlyRelationships writes the relationships file for a slide with only native SVG inserts.
func (ctx *singlePassContext) writeNativeSVGOnlyRelationships(slideNum int, nativeSVGs []nativeSVGInsert) error {
	relsFileName := SlideRelsPath(slideNum)

	existingRels := ctx.loadOrCreateRelationships(relsFileName)
	ctx.appendNativeSVGRelationships(&existingRels, nativeSVGs)

	return ctx.writeRelationshipsFile(relsFileName, existingRels)
}

// loadOrCreateRelationships loads existing relationships from the template or creates a new empty one.
func (ctx *singlePassContext) loadOrCreateRelationships(relsFileName string) pptx.RelationshipsXML {
	var existingRels pptx.RelationshipsXML
	relsData, err := utils.ReadFileFromZipIndex(ctx.templateIndex, relsFileName)
	if err == nil {
		if parseErr := xml.Unmarshal(relsData, &existingRels); parseErr != nil {
			existingRels = pptx.RelationshipsXML{
				XMLName: xml.Name{Space: pptx.NsPackageRels, Local: "Relationships"},
			}
		}
	} else {
		existingRels = pptx.RelationshipsXML{
			XMLName: xml.Name{Space: pptx.NsPackageRels, Local: "Relationships"},
		}
	}
	return existingRels
}


// appendNativeSVGRelationships appends PNG and SVG relationships for native SVG inserts.
func (ctx *singlePassContext) appendNativeSVGRelationships(rels *pptx.RelationshipsXML, nativeSVGs []nativeSVGInsert) {
	for _, svg := range nativeSVGs {
		// PNG relationship
		rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
			ID:     svg.pngRelID,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			Target: "../media/" + svg.pngMediaFile,
		})
		// SVG relationship
		rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
			ID:     svg.svgRelID,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			Target: "../media/" + svg.svgMediaFile,
		})
	}
}

// writeRelationshipsFile writes a relationships XML file to the output ZIP.
func (ctx *singlePassContext) writeRelationshipsFile(relsFileName string, rels pptx.RelationshipsXML) error {
	// Ensure the xmlns attribute is set for LibreOffice compatibility
	rels.Xmlns = pptx.NsPackageRels
	relsOutput, err := xml.Marshal(rels)
	if err != nil {
		return fmt.Errorf("failed to marshal relationships for %s: %w", relsFileName, err)
	}

	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter,relsFileName)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", relsFileName, err)
	}
	if _, err := fw.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("failed to write XML header for %s: %w", relsFileName, err)
	}
	if _, err := fw.Write(relsOutput); err != nil {
		return fmt.Errorf("failed to write relationships for %s: %w", relsFileName, err)
	}
	return nil
}

// writeContentTypes writes the [Content_Types].xml file with image extensions, slide overrides, and notes overrides.
func (ctx *singlePassContext) writeContentTypes() error {
	ctData, err := updateContentTypes(ctx.templateIndex, ctx.usedExtensions)
	if err != nil {
		return fmt.Errorf("failed to update [Content_Types].xml: %w", err)
	}

	// Add content type overrides for new slides (required by OOXML spec)
	ctData, err = addSlideContentTypeOverrides(ctData, ctx.slideContentMap, ctx.existingSlides, ctx.excludeTemplateSlides)
	if err != nil {
		return fmt.Errorf("failed to add slide content types: %w", err)
	}

	// Add content type overrides for synthetic layout files
	if len(ctx.syntheticFiles) > 0 {
		ctData, err = addSyntheticLayoutContentTypes(ctData, ctx.syntheticFiles)
		if err != nil {
			return fmt.Errorf("failed to add synthetic layout content types: %w", err)
		}
	}

	// Add content type overrides for notes slides
	if len(ctx.slideNotes) > 0 {
		ctData, err = addNotesSlideContentTypes(ctData, ctx.slideNotes)
		if err != nil {
			return fmt.Errorf("failed to add notes slide content types: %w", err)
		}
	}

	fw, err := utils.ZipCreateDeterministic(ctx.outputWriter,PathContentTypes)
	if err != nil {
		return fmt.Errorf("failed to create [Content_Types].xml: %w", err)
	}
	if _, err := fw.Write(ctData); err != nil {
		return fmt.Errorf("failed to write [Content_Types].xml: %w", err)
	}
	return nil
}

// addSlideContentTypeOverrides adds Override entries for all generated slides.
// When excludeTemplateSlides is true, existing template slide overrides are removed
// and replaced with overrides for the new slides starting from slide1.
// When false, existing overrides are kept and new ones are appended.
func addSlideContentTypeOverrides(ctData []byte, slideContentMap map[int]SlideSpec, existingSlides int, excludeTemplateSlides bool) ([]byte, error) {
	var contentTypes pptx.ContentTypesXML
	if err := xml.Unmarshal(ctData, &contentTypes); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml for slides: %w", err)
	}

	// When excluding template slides, remove stale slide overrides
	if excludeTemplateSlides {
		filtered := make([]pptx.ContentTypeOverride, 0, len(contentTypes.Overrides))
		for _, ovr := range contentTypes.Overrides {
			if ovr.ContentType != pptx.ContentTypeSlide {
				filtered = append(filtered, ovr)
			}
		}
		contentTypes.Overrides = filtered
	}

	// Build set of existing slide overrides to avoid duplicates
	existingOverrides := make(map[string]bool)
	for _, ovr := range contentTypes.Overrides {
		existingOverrides[ovr.PartName] = true
	}

	// Add overrides for new slides in deterministic order
	slideNums := make([]int, 0, len(slideContentMap))
	for slideNum := range slideContentMap {
		slideNums = append(slideNums, slideNum)
	}
	sort.Ints(slideNums)

	for _, slideNum := range slideNums {
		partName := "/" + SlidePath(slideNum)
		if existingOverrides[partName] {
			continue
		}
		contentTypes.Overrides = append(contentTypes.Overrides, pptx.ContentTypeOverride{
			PartName:    partName,
			ContentType: pptx.ContentTypeSlide,
		})
	}

	modifiedData, err := xml.Marshal(contentTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml with slides: %w", err)
	}

	return append([]byte(xml.Header), modifiedData...), nil
}

// addSyntheticLayoutContentTypes adds content type overrides for synthetic layout files.
// Only .xml layout files (not .rels) need overrides.
func addSyntheticLayoutContentTypes(ctData []byte, syntheticFiles map[string][]byte) ([]byte, error) {
	var contentTypes pptx.ContentTypesXML
	if err := xml.Unmarshal(ctData, &contentTypes); err != nil {
		return nil, fmt.Errorf("failed to parse [Content_Types].xml for synthetic layouts: %w", err)
	}

	// Build set of existing overrides to avoid duplicates.
	// Normalized layouts that replace existing template layouts share the same
	// part name — adding another Override would create duplicate entries that
	// violate OPC and trigger PowerPoint's repair dialog.
	existingOverrides := make(map[string]bool)
	for _, ovr := range contentTypes.Overrides {
		existingOverrides[ovr.PartName] = true
	}

	// Collect layout XML paths (skip .rels files) and sort for determinism
	var layoutPaths []string
	for path := range syntheticFiles {
		if strings.HasSuffix(path, ".xml") && strings.Contains(path, "slideLayout") {
			layoutPaths = append(layoutPaths, path)
		}
	}
	sort.Strings(layoutPaths)

	for _, path := range layoutPaths {
		partName := "/" + path
		if existingOverrides[partName] {
			continue
		}
		contentTypes.Overrides = append(contentTypes.Overrides, pptx.ContentTypeOverride{
			PartName:    partName,
			ContentType: pptx.ContentTypeSlideLayout,
		})
	}

	modifiedData, err := xml.Marshal(contentTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal [Content_Types].xml with synthetic layouts: %w", err)
	}

	return append([]byte(xml.Header), modifiedData...), nil
}

// writeMediaFiles writes all media files to the output ZIP.
// This includes file-based images, byte-based charts, and native SVG files.
func (ctx *singlePassContext) writeMediaFiles() error {
	// Write file-based media (images)
	if err := ctx.writeFileBasedMedia(); err != nil {
		return err
	}

	// Write byte-based media (charts)
	ctx.writeByteBasedMedia()

	// Write native SVG media files (both SVG and PNG for each insert)
	ctx.writeNativeSVGMedia()

	// Write panel icon media files (PNG images from iconBytes)
	ctx.writePanelIconMedia()

	return nil
}

// writeFileBasedMedia writes file-based images to the output ZIP.
// AC2 (C2 fix): Stream images in chunks instead of loading entirely into memory.
func (ctx *singlePassContext) writeFileBasedMedia() error {
	// Sort image paths for deterministic output ordering
	imagePaths := make([]string, 0, len(ctx.mediaFiles))
	for imagePath := range ctx.mediaFiles {
		imagePaths = append(imagePaths, imagePath)
	}
	sort.Strings(imagePaths)

	for _, imagePath := range imagePaths {
		mediaFileName := ctx.mediaFiles[imagePath]
		mediaPath := MediaPath(mediaFileName)
		if err := streamImageToZip(ctx.outputWriter, imagePath, mediaPath); err != nil {
			ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to stream image %s: %v", imagePath, err))
			// Continue with other files instead of failing
		}
	}
	return nil
}

// writeByteBasedMedia writes byte-based media (charts) directly from memory.
// This eliminates temp file I/O for chart images.
func (ctx *singlePassContext) writeByteBasedMedia() {
	// Track which media files have been written to avoid duplicates
	writtenMedia := make(map[string]bool)
	for imagePath := range ctx.mediaFiles {
		writtenMedia[ctx.mediaFiles[imagePath]] = true
	}

	// Sort slide numbers for deterministic output ordering
	byteSlideNums := make([]int, 0, len(ctx.slideRelUpdates))
	for slideNum := range ctx.slideRelUpdates {
		byteSlideNums = append(byteSlideNums, slideNum)
	}
	sort.Ints(byteSlideNums)

	for _, slideNum := range byteSlideNums {
		for _, mr := range ctx.slideRelUpdates[slideNum] {
			// Skip if already written (file-based) or no data (file path based)
			if writtenMedia[mr.mediaFileName] || mr.data == nil {
				continue
			}
			mediaPath := MediaPath(mr.mediaFileName)
			if err := streamBytesToZip(ctx.outputWriter, mr.data, mediaPath); err != nil {
				ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to write chart image %s: %v", mr.mediaFileName, err))
				continue
			}
			writtenMedia[mr.mediaFileName] = true
		}
	}
}

// writeNativeSVGMedia writes native SVG media files (both SVG and PNG for each insert).
func (ctx *singlePassContext) writeNativeSVGMedia() {
	// Sort slide numbers for deterministic output ordering
	svgSlideNums := make([]int, 0, len(ctx.nativeSVGInserts))
	for slideNum := range ctx.nativeSVGInserts {
		svgSlideNums = append(svgSlideNums, slideNum)
	}
	sort.Ints(svgSlideNums)

	for _, slideNum := range svgSlideNums {
		for _, svg := range ctx.nativeSVGInserts[slideNum] {
			// Write SVG file (from byte data or file path)
			svgMediaPath := MediaPath(svg.svgMediaFile)
			if len(svg.svgData) > 0 {
				if err := streamBytesToZip(ctx.outputWriter, svg.svgData, svgMediaPath); err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to write SVG data %s: %v", svg.svgMediaFile, err))
					continue
				}
			} else {
				if err := streamImageToZip(ctx.outputWriter, svg.svgPath, svgMediaPath); err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to stream SVG %s: %v", svg.svgPath, err))
					continue
				}
			}

			// Write PNG fallback file (from byte data or file path)
			pngMediaPath := MediaPath(svg.pngMediaFile)
			if len(svg.pngData) > 0 {
				if err := streamBytesToZip(ctx.outputWriter, svg.pngData, pngMediaPath); err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to write PNG data %s: %v", svg.pngMediaFile, err))
					continue
				}
			} else {
				if err := streamImageToZip(ctx.outputWriter, svg.pngPath, pngMediaPath); err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to stream PNG fallback %s: %v", svg.pngPath, err))
					continue
				}
			}
		}
	}
}

// writePanelIconMedia writes panel icon image files to the output ZIP.
// Only panels with non-nil iconBytes and an allocated iconMediaFile are written.
func (ctx *singlePassContext) writePanelIconMedia() {
	// Sort slide numbers for deterministic output ordering
	slideNums := make([]int, 0, len(ctx.panelShapeInserts))
	for slideNum := range ctx.panelShapeInserts {
		slideNums = append(slideNums, slideNum)
	}
	sort.Ints(slideNums)

	for _, slideNum := range slideNums {
		for _, ins := range ctx.panelShapeInserts[slideNum] {
			for _, panel := range ins.panels {
				if len(panel.iconBytes) == 0 || panel.iconMediaFile == "" {
					continue
				}
				mediaPath := MediaPath(panel.iconMediaFile)
				if err := streamBytesToZip(ctx.outputWriter, panel.iconBytes, mediaPath); err != nil {
					ctx.warnings = append(ctx.warnings, fmt.Sprintf("failed to write panel icon %s: %v", panel.iconMediaFile, err))
				}
			}
		}
	}
}

// streamImageToZip streams an image file directly to the ZIP archive.
// AC2 (C2 fix): Given a slide with a 50MB image, When generating PPTX,
// Then peak memory increase is less than 5MB above baseline.
//
// This avoids loading the entire image into memory by using io.Copy
// which streams in small chunks (typically 32KB).
func streamImageToZip(w *zip.Writer, imagePath, zipPath string) error {
	// Open source file
	src, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Create destination in ZIP with deterministic timestamp
	dst, err := utils.ZipCreateDeterministic(w, zipPath)
	if err != nil {
		return fmt.Errorf("failed to create ZIP entry: %w", err)
	}

	// Stream in chunks (io.Copy uses 32KB buffer by default)
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to stream image data: %w", err)
	}

	return nil
}

// insertRawShapes inserts pre-generated <p:sp> XML fragments before </p:spTree>.
func insertRawShapes(slideData []byte, shapes [][]byte) ([]byte, error) {
	var parts []string
	for _, s := range shapes {
		parts = append(parts, string(s))
	}
	insertion := strings.Join(parts, "\n")
	return pptx.InsertIntoSpTree(slideData, []byte(insertion), pptx.InsertAtEnd)
}

// insertBackgroundImage injects a <p:bg> element into slide XML before <p:spTree>.
// The OOXML structure is: <p:cSld> <p:bg>...</p:bg> <p:spTree>... </p:spTree> </p:cSld>
//
// Called AFTER fixOOXMLNamespaces, so all tags are already namespace-prefixed.
// The fill inside p:bgPr uses DrawingML (a:) namespace for blipFill, blip, stretch, fillRect.
func insertBackgroundImage(slideData []byte, relID string) []byte {
	for _, marker := range [][]byte{[]byte("<p:spTree>"), []byte("<p:spTree ")} {
		if pos := bytes.Index(slideData, marker); pos != -1 {
			bgXML := fmt.Sprintf(
				`<p:bg><p:bgPr>`+
					`<a:blipFill><a:blip r:embed="%s"/>`+
					`<a:stretch><a:fillRect/></a:stretch>`+
					`</a:blipFill>`+
					`<a:effectLst/>`+
					`</p:bgPr></p:bg>`,
				relID,
			)
			result := make([]byte, 0, len(slideData)+len(bgXML))
			result = append(result, slideData[:pos]...)
			result = append(result, bgXML...)
			result = append(result, slideData[pos:]...)
			return result
		}
	}
	return slideData
}

// streamBytesToZip writes byte data directly to the ZIP archive.
// This is used for chart images that are rendered in-memory, avoiding temp file I/O.
// Memory is bounded by the size of the data parameter (largest single part).
func streamBytesToZip(w *zip.Writer, data []byte, zipPath string) error {
	// Create destination in ZIP with deterministic timestamp
	dst, err := utils.ZipCreateDeterministic(w, zipPath)
	if err != nil {
		return fmt.Errorf("failed to create ZIP entry: %w", err)
	}

	// Write bytes directly
	if _, err := dst.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}
