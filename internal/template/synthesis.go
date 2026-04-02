package template

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// requiredCapabilities lists tags that synthesis can provide when missing.
var requiredCapabilities = []string{"two-column", "blank-title"}

// SynthesizeIfNeeded checks whether the template's existing layouts cover all required
// capabilities. For any missing capability, it generates synthetic layouts from the best
// content layout and appends them to the analysis.
//
// This is called after ParseLayouts() and ClassifyLayout() have run. It modifies
// analysis.Layouts in place and populates analysis.Synthesis when layouts are generated.
func SynthesizeIfNeeded(reader *Reader, analysis *types.TemplateAnalysis) {
	missing := findMissingCapabilities(analysis.Layouts)
	if len(missing) == 0 {
		return
	}

	slog.Info("template missing layout capabilities, synthesizing",
		slog.Any("missing", missing),
		slog.String("template", analysis.TemplatePath),
	)

	// Find the best content layout to use as synthesis base.
	bestSliceIdx := findBestContentLayoutIndex(analysis.Layouts)
	if bestSliceIdx < 0 {
		slog.Warn("no suitable content layout found for synthesis")
		return
	}

	// Find the next available layout index for naming synthetic layouts
	nextLayoutNum := findNextLayoutNum(analysis.Layouts)

	// Resolve the slide master that the base layout references so synthetic
	// layouts inherit the correct master's theme, colors, and fonts.
	masterTarget := resolveMasterTarget(reader, analysis.Layouts[bestSliceIdx].ID)

	manifest := &types.SynthesisManifest{
		SyntheticFiles: make(map[string][]byte),
	}

	// Find the title placeholder from the base layout so synthetic layouts
	// can inherit it. Without this, two-column slides have no title shape
	// and the title text is silently dropped.
	//
	// We make a copy with a normalized ID ("title") because synthesis runs
	// before NormalizeLayoutFiles. Without this, synthetic layouts would use
	// raw placeholder names (e.g., "Title 1") while the generator expects
	// canonical names (e.g., "title") for placeholder resolution.
	var baseTitlePH *types.PlaceholderInfo
	baseLayout := analysis.Layouts[bestSliceIdx]
	for i := range baseLayout.Placeholders {
		if baseLayout.Placeholders[i].Type == types.PlaceholderTitle {
			normalized := baseLayout.Placeholders[i] // copy
			normalized.ID = "title"                   // canonical name
			baseTitlePH = &normalized
			break
		}
	}

	// Collect footer and slide number placeholders from base layout for blank-title.
	var baseFooterPHs []types.PlaceholderInfo
	for _, ph := range baseLayout.Placeholders {
		if ph.Type == types.PlaceholderOther {
			baseFooterPHs = append(baseFooterPHs, ph)
		}
	}

	// Synthesize blank-title layout if missing.
	if containsCapability(missing, "blank-title") {
		nextLayoutNum = synthesizeBlankTitle(analysis, manifest, nextLayoutNum, baseTitlePH, baseFooterPHs, bestSliceIdx, masterTarget)
	}

	// Synthesize two-column layouts if missing.
	if containsCapability(missing, "two-column") {
		synthesizeTwoColumn(reader, analysis, manifest, nextLayoutNum, baseTitlePH, bestSliceIdx, masterTarget)
	}

	if len(manifest.SyntheticFiles) > 0 {
		analysis.Synthesis = manifest
	}
}

// containsCapability checks if a capability is in the list.
func containsCapability(caps []string, cap string) bool {
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}

// synthesizeBlankTitle generates a blank+title layout with only the title placeholder
// and utility placeholders (footer, slide number). Returns the next layout number.
func synthesizeBlankTitle(
	analysis *types.TemplateAnalysis,
	manifest *types.SynthesisManifest,
	nextLayoutNum int,
	baseTitlePH *types.PlaceholderInfo,
	baseFooterPHs []types.PlaceholderInfo,
	baseSliceIdx int,
	masterTarget string,
) int {
	if baseTitlePH == nil {
		slog.Warn("no title placeholder found for blank-title synthesis")
		return nextLayoutNum
	}

	layoutNum := nextLayoutNum
	nextLayoutNum++

	// Generate layout XML with title + utility placeholders only.
	xmlBytes := GenerateBlankTitleLayoutXMLBytes(baseTitlePH, baseFooterPHs, layoutNum)
	layoutPath := fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", layoutNum)
	manifest.SyntheticFiles[layoutPath] = xmlBytes

	relsBytes := GenerateLayoutRelsXMLBytes(masterTarget)
	relsPath := fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", layoutNum)
	manifest.SyntheticFiles[relsPath] = relsBytes

	// Build metadata with title + utility placeholders.
	var placeholders []types.PlaceholderInfo
	placeholders = append(placeholders, *baseTitlePH)
	placeholders = append(placeholders, baseFooterPHs...)

	layoutID := fmt.Sprintf("slideLayout%d", layoutNum)
	metadata := types.LayoutMetadata{
		ID:           layoutID,
		Name:         "Blank + Title",
		Index:        len(analysis.Layouts),
		Placeholders: placeholders,
		Tags:         []string{},
	}
	ClassifyLayout(&metadata)
	// Ensure "virtual-base" tag is present for the grid resolver.
	metadata.Tags = append(metadata.Tags, "virtual-base")

	analysis.Layouts = append(analysis.Layouts, metadata)

	slog.Info("synthesized layout",
		slog.String("id", layoutID),
		slog.String("name", "Blank + Title"),
		slog.Int("placeholders", len(placeholders)),
	)

	return nextLayoutNum
}

// synthesizeTwoColumn generates two-column layout variants from the best content layout.
func synthesizeTwoColumn(
	reader *Reader,
	analysis *types.TemplateAnalysis,
	manifest *types.SynthesisManifest,
	nextLayoutNum int,
	baseTitlePH *types.PlaceholderInfo,
	bestSliceIdx int,
	masterTarget string,
) {
	// Extract content area from already-parsed layouts (which have resolved bounds
	// from master inheritance). We can't use ExtractContentAreaBounds(reader, ...)
	// because programmatic templates have empty <p:spPr/> in layouts and inherit
	// transforms from the slide master — the raw XML has no bounds.
	contentArea, err := ExtractContentAreaBoundsFromLayouts(analysis.Layouts, bestSliceIdx)
	if err != nil {
		slog.Warn("failed to extract content area for synthesis",
			slog.String("error", err.Error()),
		)
		return
	}

	// When the content layout has zero-area bounds (master resolution failed or
	// bounds are inherited but not resolved), fall back to a standard content area
	// derived from the title placeholder. This ensures synthesis produces usable
	// two-column layouts even on templates with implicit positioning.
	if contentArea.Width == 0 || contentArea.Height == 0 {
		*contentArea = defaultContentAreaFromTitle(baseTitlePH)
		slog.Info("content area bounds were zero, using fallback",
			slog.Int64("width", contentArea.Width),
			slog.Int64("height", contentArea.Height),
		)
	}

	// Try to extract placeholder style from the template XML for font/bullet info.
	// Fall back to an empty style if the layout has no explicit styling (inherited from master)
	// or if the reader is not available.
	var baseStyle *PlaceholderStyle
	if reader != nil {
		baseFileIdx := analysis.Layouts[bestSliceIdx].Index
		baseStyle, err = ExtractPlaceholderStyle(reader, baseFileIdx, types.PlaceholderBody)
		if err != nil {
			baseStyle, err = ExtractPlaceholderStyle(reader, baseFileIdx, types.PlaceholderContent)
			if err != nil {
				baseStyle = &PlaceholderStyle{}
			}
		}
	} else {
		baseStyle = &PlaceholderStyle{}
	}

	// Generate layouts using the resolved content area and extracted style
	config := DefaultLayoutGeneratorConfig()
	generated := GenerateHorizontalLayouts(*contentArea, *baseStyle, config)

	// Set the base layout index for all generated layouts
	for i := range generated {
		generated[i].BasedOnIdx = bestSliceIdx
	}

	// Filter to only the two-column variants
	needed := filterLayoutsForCapabilities(generated, []string{"two-column"})
	if len(needed) == 0 {
		return
	}

	for i := range needed {
		// Set title bounds from the base layout's title placeholder so the
		// generated XML emits explicit <a:xfrm> instead of empty <p:spPr/>.
		if baseTitlePH != nil {
			bounds := baseTitlePH.Bounds
			needed[i].TitleBounds = &bounds
		}

		gl := needed[i]
		layoutNum := nextLayoutNum
		nextLayoutNum++

		// Generate layout XML bytes (include title placeholder shape).
		// Pass the title placeholder name so the XML matches the metadata.
		titleName := "Title 1" // default
		if baseTitlePH != nil {
			titleName = baseTitlePH.ID
		}
		xmlBytes := GenerateLayoutXMLBytes(gl, layoutNum, titleName)
		layoutPath := fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", layoutNum)
		manifest.SyntheticFiles[layoutPath] = xmlBytes

		// Generate rels XML bytes
		relsBytes := GenerateLayoutRelsXMLBytes(masterTarget)
		relsPath := fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", layoutNum)
		manifest.SyntheticFiles[relsPath] = relsBytes

		// Convert GeneratedLayout to LayoutMetadata (include title placeholder)
		layoutID := fmt.Sprintf("slideLayout%d", layoutNum)
		metadata := convertGeneratedToMetadata(gl, layoutID, len(analysis.Layouts), baseTitlePH)
		analysis.Layouts = append(analysis.Layouts, metadata)

		slog.Info("synthesized layout",
			slog.String("id", layoutID),
			slog.String("name", gl.Name),
			slog.Int("placeholders", len(gl.Placeholders)),
		)
	}
}

// findMissingCapabilities returns required capabilities not present in any existing layout's tags.
func findMissingCapabilities(layouts []types.LayoutMetadata) []string {
	present := make(map[string]bool)
	for _, layout := range layouts {
		for _, tag := range layout.Tags {
			present[tag] = true
		}
	}

	var missing []string
	for _, cap := range requiredCapabilities {
		if !present[cap] {
			missing = append(missing, cap)
		}
	}
	return missing
}

// findBestContentLayoutIndex returns the index of the best content layout for synthesis.
// It selects the layout with a "content" tag and the largest body/content placeholder area.
// When all content layouts have zero-area body placeholders (e.g., bounds inherited from
// master but not resolved), it falls back to the first content-tagged layout that has a
// body or content placeholder.
func findBestContentLayoutIndex(layouts []types.LayoutMetadata) int {
	bestIdx := -1
	var bestArea int64
	firstContentIdx := -1

	for i, layout := range layouts {
		// Must be a content layout
		hasContent := false
		for _, tag := range layout.Tags {
			if tag == "content" {
				hasContent = true
				break
			}
		}
		if !hasContent {
			continue
		}

		// Find the largest body/content placeholder
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				// Track first content layout with a body/content placeholder as fallback
				if firstContentIdx < 0 {
					firstContentIdx = i
				}
				area := ph.Bounds.Width * ph.Bounds.Height
				if area > bestArea {
					bestArea = area
					bestIdx = i
				}
			}
		}
	}

	// Fall back to first content layout when all have zero-area bounds
	if bestIdx < 0 && firstContentIdx >= 0 {
		slog.Info("all content layouts have zero-area body placeholders, using first as fallback",
			slog.Int("fallback_index", firstContentIdx),
			slog.String("layout_name", layouts[firstContentIdx].Name),
		)
		return firstContentIdx
	}

	return bestIdx
}

// defaultContentAreaFromTitle returns a fallback content area for synthesis when the
// content layout's body placeholder has zero bounds. It positions the content below the
// title (or at a standard offset when title is unavailable) using standard 16:9 dimensions.
func defaultContentAreaFromTitle(titlePH *types.PlaceholderInfo) ContentAreaBounds {
	const (
		defaultSlideWidth  int64 = 12192000 // 16:9 standard width
		defaultSlideHeight int64 = 6858000  // 16:9 standard height
		defaultMarginX     int64 = 457200   // 0.5 inch
		defaultTitleBottom int64 = 1600200  // ~1.75 inches from top
		defaultFooterTop   int64 = 6400800  // footer starts ~7 inches down
	)

	x := defaultMarginX
	y := defaultTitleBottom
	width := defaultSlideWidth - 2*defaultMarginX

	// Use actual title bounds if available
	if titlePH != nil && titlePH.Bounds.Height > 0 {
		x = titlePH.Bounds.X
		y = titlePH.Bounds.Y + titlePH.Bounds.Height
		width = titlePH.Bounds.Width
	}

	height := defaultFooterTop - y
	if height <= 0 {
		height = defaultSlideHeight - y - defaultMarginX
	}

	return ContentAreaBounds{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

// filterLayoutsForCapabilities selects generated layouts that fill the missing capabilities.
func filterLayoutsForCapabilities(layouts []GeneratedLayout, missing []string) []GeneratedLayout {
	var result []GeneratedLayout

	for _, cap := range missing {
		switch cap {
		case "two-column":
			// Include 50/50 as the balanced default, plus 60/40 and 40/60
			// asymmetric variants. The heuristic's narrow-diagram penalty
			// steers slides with complex diagrams toward the variant that
			// gives the diagram the wider column, while text-only two-column
			// slides naturally prefer 50/50 (no penalty on any variant).
			wanted := []string{"content-2-50-50", "content-2-60-40", "content-2-40-60"}
			for _, id := range wanted {
				for _, gl := range layouts {
					if gl.ID == id {
						result = append(result, gl)
						break
					}
				}
			}
		}
	}

	return result
}

// findNextLayoutNum scans existing layouts and returns the next available slideLayout number.
// Returns maxExisting+1 for sequential numbering (e.g., 70 for a template with 69 layouts).
func findNextLayoutNum(layouts []types.LayoutMetadata) int {
	maxNum := 0
	for _, layout := range layouts {
		// Extract number from ID like "slideLayout5"
		var num int
		if _, err := fmt.Sscanf(layout.ID, "slideLayout%d", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}
	return maxNum + 1
}

// convertGeneratedToMetadata converts a GeneratedLayout into a LayoutMetadata
// with proper tags assigned via ClassifyLayout.
// baseTitlePH is the title placeholder from the base content layout; if non-nil,
// it is included so that two-column slides can display titles.
func convertGeneratedToMetadata(gl GeneratedLayout, layoutID string, index int, baseTitlePH *types.PlaceholderInfo) types.LayoutMetadata {
	var placeholders []types.PlaceholderInfo

	// Include the title placeholder from the base layout if available.
	// The title shape inherits its position from the slide master (empty spPr),
	// so we copy the resolved bounds from the base layout's title placeholder.
	if baseTitlePH != nil {
		placeholders = append(placeholders, *baseTitlePH)
	}

	for i, gp := range gl.Placeholders {
		// Use idx=10+ to avoid collision with master placeholders (title=0, body=1)
		phIdx := 10 + i

		ph := types.PlaceholderInfo{
			ID:       gp.ID,
			Type:     types.PlaceholderContent,
			Index:    phIdx,
			Bounds:   gp.Bounds,
			MaxChars: estimateMaxChars(gp.Bounds),
		}

		// Copy font properties from the generated placeholder's style
		if gp.Style.FontFamily != "" {
			ph.FontFamily = gp.Style.FontFamily
		}
		if gp.Style.FontSize != 0 {
			ph.FontSize = gp.Style.FontSize
		}
		if gp.Style.FontColor != "" {
			ph.FontColor = gp.Style.FontColor
		}

		placeholders = append(placeholders, ph)
	}

	capacity := estimateCapacity(placeholders)

	metadata := types.LayoutMetadata{
		ID:           layoutID,
		Name:         gl.Name,
		Index:        index,
		Placeholders: placeholders,
		Capacity:     capacity,
		Tags:         []string{},
	}

	ClassifyLayout(&metadata)

	return metadata
}

// GenerateLayoutXMLBytes generates the slideLayout XML content for a generated layout.
// This is a standalone version of Expander.generateLayoutXML that doesn't require a Package.
// titleName is the shape name for the title placeholder (e.g., "Title 1" or "Title Placeholder 1").
func GenerateLayoutXMLBytes(layout GeneratedLayout, layoutNum int, titleName string) []byte {
	// Pre-allocate buffer with estimated size
	buf := make([]byte, 0, 2048)

	buf = append(buf, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`...)
	buf = append(buf, '\n')

	buf = append(buf, `<p:sldLayout xmlns:a="`...)
	buf = append(buf, nsDrawingML...)
	buf = append(buf, `" xmlns:r="`...)
	buf = append(buf, nsRelationships...)
	buf = append(buf, `" xmlns:p="`...)
	buf = append(buf, nsPresentationML...)
	buf = append(buf, `" preserve="1">`...)

	buf = append(buf, `<p:cSld name="`...)
	buf = append(buf, xmlEscapeAttr(layout.Name)...)
	buf = append(buf, `">`...)

	buf = append(buf, `<p:spTree>`...)

	// Group shape properties (required)
	buf = append(buf, `<p:nvGrpSpPr>`...)
	buf = append(buf, `<p:cNvPr id="1" name=""/>`...)
	buf = append(buf, `<p:cNvGrpSpPr/>`...)
	buf = append(buf, `<p:nvPr/>`...)
	buf = append(buf, `</p:nvGrpSpPr>`...)
	buf = append(buf, `<p:grpSpPr>`...)
	buf = append(buf, `<a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm>`...)
	buf = append(buf, `</p:grpSpPr>`...)

	// Add title placeholder shape. When TitleBounds is set, emit explicit <a:xfrm>
	// so the title is full-width instead of inheriting the master's narrow position.
	shapeID := 2
	buf = appendTitlePlaceholder(buf, shapeID, titleName, layout.TitleBounds)
	shapeID++

	// Placeholder indices start at 10+ to avoid collision with master
	for i, ph := range layout.Placeholders {
		phIdx := 10 + i
		buf = appendSlotPlaceholder(buf, ph, shapeID, phIdx)
		shapeID++
	}

	buf = append(buf, `</p:spTree>`...)
	buf = append(buf, `</p:cSld>`...)

	buf = append(buf, `<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>`...)
	buf = append(buf, `</p:sldLayout>`...)

	return buf
}

// appendTitlePlaceholder appends a title placeholder shape.
// When bounds is non-nil, explicit <a:xfrm> is emitted so the title uses the
// resolved position from the base layout instead of inheriting the master's
// (often narrow/indented) title position.
// titleName must match the base layout's title placeholder name (e.g., "Title 1"
// or "Title Placeholder 1") so that the generator's placeholder map can find it.
func appendTitlePlaceholder(buf []byte, shapeID int, titleName string, bounds *types.BoundingBox) []byte {
	buf = append(buf, `<p:sp>`...)
	buf = append(buf, `<p:nvSpPr>`...)
	buf = append(buf, fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, shapeID, xmlEscapeAttr(titleName))...)
	buf = append(buf, `<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>`...)
	buf = append(buf, `<p:nvPr>`...)
	buf = append(buf, `<p:ph type="title"/>`...)
	buf = append(buf, `</p:nvPr>`...)
	buf = append(buf, `</p:nvSpPr>`...)
	if bounds != nil {
		buf = append(buf, `<p:spPr>`...)
		buf = append(buf, `<a:xfrm>`...)
		buf = append(buf, fmt.Sprintf(`<a:off x="%d" y="%d"/>`, bounds.X, bounds.Y)...)
		buf = append(buf, fmt.Sprintf(`<a:ext cx="%d" cy="%d"/>`, bounds.Width, bounds.Height)...)
		buf = append(buf, `</a:xfrm>`...)
		buf = append(buf, `</p:spPr>`...)
	} else {
		buf = append(buf, `<p:spPr/>`...)
	}
	buf = append(buf, `<p:txBody>`...)
	buf = append(buf, `<a:bodyPr/>`...)
	buf = append(buf, `<a:lstStyle/>`...)
	buf = append(buf, `<a:p><a:endParaRPr lang="en-US"/></a:p>`...)
	buf = append(buf, `</p:txBody>`...)
	buf = append(buf, `</p:sp>`...)
	return buf
}

// appendSlotPlaceholder appends a placeholder shape XML to the buffer.
func appendSlotPlaceholder(buf []byte, ph GeneratedPlaceholder, shapeID, phIdx int) []byte {
	buf = append(buf, `<p:sp>`...)
	buf = append(buf, `<p:nvSpPr>`...)
	buf = append(buf, fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, shapeID, xmlEscapeAttr(ph.ID))...)
	buf = append(buf, `<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>`...)
	buf = append(buf, `<p:nvPr>`...)
	buf = append(buf, fmt.Sprintf(`<p:ph idx="%d"/>`, phIdx)...)
	buf = append(buf, `</p:nvPr>`...)
	buf = append(buf, `</p:nvSpPr>`...)

	buf = append(buf, `<p:spPr>`...)
	buf = append(buf, `<a:xfrm>`...)
	buf = append(buf, fmt.Sprintf(`<a:off x="%d" y="%d"/>`, ph.Bounds.X, ph.Bounds.Y)...)
	buf = append(buf, fmt.Sprintf(`<a:ext cx="%d" cy="%d"/>`, ph.Bounds.Width, ph.Bounds.Height)...)
	buf = append(buf, `</a:xfrm>`...)
	buf = append(buf, `</p:spPr>`...)

	buf = append(buf, `<p:txBody>`...)
	buf = appendBodyPr(buf, &ph.Style)
	buf = appendLstStyle(buf, &ph.Style)
	buf = append(buf, `<a:p>`...)
	buf = append(buf, fmt.Sprintf(`<a:r><a:rPr lang="en-US"/><a:t>::%s::</a:t></a:r>`, xmlEscapeText(ph.ID))...)
	buf = append(buf, `</a:p>`...)
	buf = append(buf, `</p:txBody>`...)

	buf = append(buf, `</p:sp>`...)
	return buf
}

// appendBodyPr appends the <a:bodyPr> element to the buffer.
func appendBodyPr(buf []byte, style *PlaceholderStyle) []byte {
	hasMargins := style.MarginLeft != 0 || style.MarginRight != 0 ||
		style.MarginTop != 0 || style.MarginBottom != 0
	if !hasMargins {
		return append(buf, `<a:bodyPr/>`...)
	}
	buf = append(buf, `<a:bodyPr`...)
	if style.MarginLeft != 0 {
		buf = append(buf, fmt.Sprintf(` lIns="%d"`, style.MarginLeft)...)
	}
	if style.MarginTop != 0 {
		buf = append(buf, fmt.Sprintf(` tIns="%d"`, style.MarginTop)...)
	}
	if style.MarginRight != 0 {
		buf = append(buf, fmt.Sprintf(` rIns="%d"`, style.MarginRight)...)
	}
	if style.MarginBottom != 0 {
		buf = append(buf, fmt.Sprintf(` bIns="%d"`, style.MarginBottom)...)
	}
	return append(buf, `/>`...)
}

// appendLstStyle appends the <a:lstStyle> element to the buffer.
// Always emits a non-empty lstStyle because appendDefRPr now always emits
// at least b="0" to prevent master bold inheritance.
func appendLstStyle(buf []byte, style *PlaceholderStyle) []byte {

	buf = append(buf, `<a:lstStyle>`...)
	buf = append(buf, `<a:lvl1pPr`...)
	if style.BulletMarginL != 0 {
		buf = append(buf, fmt.Sprintf(` marL="%d"`, style.BulletMarginL)...)
	}
	if style.BulletIndent != 0 {
		buf = append(buf, fmt.Sprintf(` indent="%d"`, style.BulletIndent)...)
	}
	buf = append(buf, `>`...)

	if style.LineSpacing != 0 {
		buf = append(buf, fmt.Sprintf(`<a:lnSpc><a:spcPct val="%d"/></a:lnSpc>`, style.LineSpacing*1000)...)
	}
	if style.SpaceBefore != 0 {
		buf = append(buf, fmt.Sprintf(`<a:spcBef><a:spcPts val="%d"/></a:spcBef>`, style.SpaceBefore/12700)...)
	}
	if style.SpaceAfter != 0 {
		buf = append(buf, fmt.Sprintf(`<a:spcAft><a:spcPts val="%d"/></a:spcAft>`, style.SpaceAfter/12700)...)
	}

	if style.BulletChar != "" {
		if style.BulletColor != "" {
			buf = append(buf, fmt.Sprintf(`<a:buClr><a:srgbClr val="%s"/></a:buClr>`, stripHashPrefix(style.BulletColor))...)
		}
		if style.BulletSize != 0 {
			buf = append(buf, fmt.Sprintf(`<a:buSzPct val="%d"/>`, style.BulletSize*1000)...)
		}
		buf = append(buf, fmt.Sprintf(`<a:buChar char="%s"/>`, xmlEscapeAttr(style.BulletChar))...)
	}
	// When BulletChar is empty, omit the bullet element entirely so the slot
	// inherits bullet styling from the master. Previously <a:buNone/> was
	// emitted here which suppressed bullet markers even when users placed
	// bullet content in the slot.

	buf = appendDefRPr(buf, style)
	buf = append(buf, `</a:lvl1pPr>`...)
	buf = append(buf, `</a:lstStyle>`...)
	return buf
}

// appendDefRPr appends the <a:defRPr> element to the buffer.
// Always emits b="0" (or b="1") to prevent inheriting bold from the master's
// bodyStyle, which defaults to b="1" on many templates.
func appendDefRPr(buf []byte, style *PlaceholderStyle) []byte {
	// Always emit defRPr with the b attribute to override master bold inheritance.
	hasFontAttrs := true
	hasFontChildren := style.FontFamily != "" || style.FontColor != ""

	if !hasFontAttrs && !hasFontChildren {
		return buf
	}

	buf = append(buf, `<a:defRPr`...)
	if style.FontSize != 0 {
		buf = append(buf, fmt.Sprintf(` sz="%d"`, style.FontSize)...)
	}
	if style.FontBold {
		buf = append(buf, ` b="1"`...)
	} else {
		buf = append(buf, ` b="0"`...)
	}
	if style.FontItalic {
		buf = append(buf, ` i="1"`...)
	}

	if !hasFontChildren {
		return append(buf, `/>`...)
	}

	buf = append(buf, `>`...)
	if style.FontColor != "" {
		buf = append(buf, fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, stripHashPrefix(style.FontColor))...)
	}
	if style.FontFamily != "" {
		buf = append(buf, fmt.Sprintf(`<a:latin typeface="%s"/>`, xmlEscapeAttr(style.FontFamily))...)
	}
	return append(buf, `</a:defRPr>`...)
}

// GenerateBlankTitleLayoutXMLBytes generates a layout XML with only the title placeholder
// and utility placeholders (footer, slide number). No body/content placeholders.
func GenerateBlankTitleLayoutXMLBytes(titlePH *types.PlaceholderInfo, footerPHs []types.PlaceholderInfo, layoutNum int) []byte {
	buf := make([]byte, 0, 1536)

	buf = append(buf, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`...)
	buf = append(buf, '\n')

	buf = append(buf, `<p:sldLayout xmlns:a="`...)
	buf = append(buf, nsDrawingML...)
	buf = append(buf, `" xmlns:r="`...)
	buf = append(buf, nsRelationships...)
	buf = append(buf, `" xmlns:p="`...)
	buf = append(buf, nsPresentationML...)
	buf = append(buf, `" preserve="1">`...)

	buf = append(buf, `<p:cSld name="Blank + Title">`...)

	buf = append(buf, `<p:spTree>`...)

	// Group shape properties (required)
	buf = append(buf, `<p:nvGrpSpPr>`...)
	buf = append(buf, `<p:cNvPr id="1" name=""/>`...)
	buf = append(buf, `<p:cNvGrpSpPr/>`...)
	buf = append(buf, `<p:nvPr/>`...)
	buf = append(buf, `</p:nvGrpSpPr>`...)
	buf = append(buf, `<p:grpSpPr>`...)
	buf = append(buf, `<a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm>`...)
	buf = append(buf, `</p:grpSpPr>`...)

	// Title placeholder with explicit bounds
	shapeID := 2
	titleBounds := &titlePH.Bounds
	buf = appendTitlePlaceholder(buf, shapeID, titlePH.ID, titleBounds)
	shapeID++

	// Footer and slide number placeholders (inherit position from master)
	for _, ph := range footerPHs {
		buf = appendUtilityPlaceholder(buf, shapeID, ph)
		shapeID++
	}

	buf = append(buf, `</p:spTree>`...)
	buf = append(buf, `</p:cSld>`...)

	buf = append(buf, `<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>`...)
	buf = append(buf, `</p:sldLayout>`...)

	return buf
}

// appendUtilityPlaceholder appends a footer/slide-number/date placeholder shape.
// These inherit positioning from the slide master (empty spPr).
func appendUtilityPlaceholder(buf []byte, shapeID int, ph types.PlaceholderInfo) []byte {
	// Map PlaceholderOther ID names to OOXML type attributes.
	phType := inferUtilityPhType(ph.ID)

	buf = append(buf, `<p:sp>`...)
	buf = append(buf, `<p:nvSpPr>`...)
	buf = append(buf, fmt.Sprintf(`<p:cNvPr id="%d" name="%s"/>`, shapeID, xmlEscapeAttr(ph.ID))...)
	buf = append(buf, `<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>`...)
	buf = append(buf, `<p:nvPr>`...)
	buf = append(buf, fmt.Sprintf(`<p:ph type="%s" sz="quarter" idx="%d"/>`, phType, ph.Index)...)
	buf = append(buf, `</p:nvPr>`...)
	buf = append(buf, `</p:nvSpPr>`...)
	buf = append(buf, `<p:spPr/>`...)
	buf = append(buf, `<p:txBody>`...)
	buf = append(buf, `<a:bodyPr/>`...)
	buf = append(buf, `<a:lstStyle/>`...)
	buf = append(buf, `<a:p><a:endParaRPr lang="en-US"/></a:p>`...)
	buf = append(buf, `</p:txBody>`...)
	buf = append(buf, `</p:sp>`...)
	return buf
}

// inferUtilityPhType maps a placeholder name to the OOXML type attribute.
func inferUtilityPhType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "footer"):
		return "ftr"
	case strings.Contains(lower, "slide number") || strings.Contains(lower, "number"):
		return "sldNum"
	case strings.Contains(lower, "date"):
		return "dt"
	default:
		return "ftr" // safe fallback
	}
}

// defaultMasterTarget is the fallback when the base layout's rels cannot be read.
const defaultMasterTarget = "../slideMasters/slideMaster1.xml"

// resolveMasterTarget reads the base layout's relationship file and returns the
// slide master Target attribute (e.g., "../slideMasters/slideMaster2.xml").
// Falls back to slideMaster1.xml when the reader is nil or the rels cannot be parsed.
func resolveMasterTarget(reader *Reader, baseLayoutID string) string {
	if reader == nil || baseLayoutID == "" {
		return defaultMasterTarget
	}
	relsPath := fmt.Sprintf("ppt/slideLayouts/_rels/%s.xml.rels", baseLayoutID)
	data, err := reader.ReadFile(relsPath)
	if err != nil {
		slog.Debug("cannot read base layout rels, using default master",
			slog.String("layout_id", baseLayoutID),
			slog.String("error", err.Error()),
		)
		return defaultMasterTarget
	}
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		slog.Debug("cannot parse base layout rels, using default master",
			slog.String("layout_id", baseLayoutID),
			slog.String("error", err.Error()),
		)
		return defaultMasterTarget
	}
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			return rel.Target
		}
	}
	return defaultMasterTarget
}

// GenerateLayoutRelsXMLBytes generates the relationship file XML for a synthetic layout.
// masterTarget is the relative path to the slide master (e.g., "../slideMasters/slideMaster2.xml").
func GenerateLayoutRelsXMLBytes(masterTarget string) []byte {
	buf := make([]byte, 0, 512)
	buf = append(buf, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`...)
	buf = append(buf, '\n')
	buf = append(buf, `<Relationships xmlns="`...)
	buf = append(buf, nsRelationships...)
	buf = append(buf, `">`...)
	buf = append(buf, `<Relationship Id="rId1" Type="`...)
	buf = append(buf, relTypeSlideMaster...)
	buf = append(buf, `" Target="`...)
	buf = append(buf, masterTarget...)
	buf = append(buf, `"/>`...)
	buf = append(buf, `</Relationships>`...)
	return buf
}

