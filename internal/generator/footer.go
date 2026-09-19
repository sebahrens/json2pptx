package generator

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/google/uuid"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// footerFontSize is the footer text size in hundredths of a point.
const footerFontSize = 1050 // 10.5pt in hundredths of a point

// defaultSlideHeightEMU is the standard 16:9 slide height used as a fallback
// when the actual slide height is unknown.
const defaultSlideHeightEMU int64 = 6858000 // 7.50 inches

// Footer positioning constants.
const (
	// defaultFooterMarginMM is the distance from the bottom of the slide to the
	// bottom edge of the footer, used when no footer placeholder exists in the template.
	defaultFooterMarginMM float64 = 5.5
	// emuPerMM converts millimeters to EMU (English Metric Units).
	emuPerMM int64 = 36000
	// defaultFooterCY is the standard footer height in EMU (~0.4 inches).
	defaultFooterCY int64 = 365125
	// minSldNumWidth is the minimum width for the slide number placeholder
	// to accommodate 3-digit numbers (up to 999) at footerFontSize (10.5pt).
	// 0.5 inches = 457200 EMU ≈ 36pt, enough for 3 digits.
	minSldNumWidth int64 = 457200
)

// computeDefaultFooterPositions generates fallback footer positions based on
// slide height, placing footers 5.5mm above the bottom edge.
func computeDefaultFooterPositions(slideHeight int64) map[string]*transformXML {
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	footerMarginEMU := int64(defaultFooterMarginMM * float64(emuPerMM)) // 198000
	fallbackY := slideHeight - footerMarginEMU - defaultFooterCY
	return map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: fallbackY},
			Extent: extentXML{CX: 3200400, CY: defaultFooterCY},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: fallbackY},
			Extent: extentXML{CX: 4114800, CY: defaultFooterCY},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: fallbackY},
			Extent: extentXML{CX: 3200400, CY: defaultFooterCY},
		},
	}
}

// Footer single-line fitting (go-slide-creator-55i1).
const (
	// footerMinFontSize is the smallest size the left footer text shrinks to
	// before it is ellipsized, in hundredths of a point.
	footerMinFontSize = 800 // 8pt
	// footerFontStep is the shrink step, in hundredths of a point.
	footerFontStep = 50
	// footerBoxGap separates the widened left footer box from the slide
	// number box.
	footerBoxGap int64 = 91440 // 0.1in
)

// leftFooterBox returns the box the left footer text renders into. It starts
// at the date (dt) placeholder's left edge — where footer text has always
// been anchored — and extends across the footer (ftr) placeholder, which the
// generator otherwise leaves empty, so the text has the template's full
// footer width instead of the ~3in date slot. The box stops short of the
// slide-number (sldNum) placeholder. Returns nil when there is no dt position.
func leftFooterBox(positions map[string]*transformXML) *transformXML {
	dt, ok := positions["type:dt"]
	if !ok || dt == nil {
		return nil
	}
	box := *dt
	right := dt.Offset.X + dt.Extent.CX
	if ftr, ok := positions["type:ftr"]; ok && ftr != nil && ftr.Offset.X >= dt.Offset.X {
		if r := ftr.Offset.X + ftr.Extent.CX; r > right {
			right = r
		}
	}
	if sn, ok := positions["type:sldNum"]; ok && sn != nil && sn.Offset.X > dt.Offset.X {
		if limit := sn.Offset.X - footerBoxGap; right > limit {
			right = limit
		}
	}
	if right > box.Offset.X {
		box.Extent.CX = right - box.Offset.X
	}
	return &box
}

// fitFooterText fits footer text onto a single line of a box widthEMU wide:
// it keeps the default footer size when the text fits, otherwise shrinks it
// in half-point steps down to footerMinFontSize, and if the text still wraps,
// ellipsizes it word by word. fontName is the theme body font used for
// measurement (falling back to Arial metrics, which are wider than most body
// fonts, so the fit is conservative). When no font can be resolved the text is
// returned unchanged.
func fitFooterText(text string, widthEMU int64, fontName string) (string, int) {
	ff, _, _ := fontcache.Resolve(fontName, "Arial")
	if ff == nil {
		return text, footerFontSize
	}
	// Usable single-line width: box minus the 0.1in left/right insets, in mm
	// (canvas text-line bounds are millimetres; the face size is points).
	usableMM := float64(widthEMU-2*91440) / 36000
	fits := func(t string, size int) bool {
		face := ff.Face(float64(size)/100, color.Black, canvas.FontRegular, canvas.FontNormal)
		return canvas.NewTextLine(face, t, canvas.Left).Bounds().W() <= usableMM
	}
	for size := footerFontSize; size >= footerMinFontSize; size -= footerFontStep {
		if fits(text, size) {
			return text, size
		}
	}
	words := strings.Fields(text)
	for n := len(words) - 1; n > 0; n-- {
		candidate := strings.TrimRight(strings.Join(words[:n], " "), " ,;:|-–—") + "…"
		if fits(candidate, footerMinFontSize) {
			return candidate, footerMinFontSize
		}
	}
	// A single over-long word: trim runes until it fits.
	runes := []rune(text)
	for n := len(runes) - 1; n > 0; n-- {
		candidate := string(runes[:n]) + "…"
		if fits(candidate, footerMinFontSize) {
			return candidate, footerMinFontSize
		}
	}
	return text, footerMinFontSize
}

// generateFooterShape creates a single p:sp element for a footer zone.
func generateFooterShape(shapeID uint32, name string, xfrm *transformXML, text string, alignment string) string {
	return generateFooterShapeSized(shapeID, name, xfrm, text, alignment, footerFontSize, "")
}

// chromeFill returns the fill every chrome run is drawn with. An empty
// colorHex keeps the inherited scheme color, which is what almost every layout
// wants; a non-empty one pins an explicit color because the scheme color would
// be invisible on this layout's background (go-slide-creator-hln7).
func chromeFill(colorHex string) pptx.Fill {
	if colorHex == "" {
		return pptx.SchemeFill(chromeDefaultScheme)
	}
	// SolidFill writes the value straight into val="", and ECMA-376 wants six
	// bare hex digits — a leading "#" produces XML PowerPoint rejects.
	return pptx.SolidFill(strings.TrimPrefix(colorHex, "#"))
}

// generateFooterShapeSized creates a footer p:sp element with an explicit
// font size (hundredths of a point).
func generateFooterShapeSized(shapeID uint32, name string, xfrm *transformXML, text string, alignment string, fontSize int, colorHex string) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   pptx.RectEmu{X: xfrm.Offset.X, Y: xfrm.Offset.Y, CX: xfrm.Extent.CX, CY: xfrm.Extent.CY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			Wrap:   "square",
			Anchor: "ctr",
			Insets: [4]int64{91440, 0, 91440, 0},
			Paragraphs: []pptx.Paragraph{{
				Align: alignment,
				Runs: []pptx.Run{{
					Text:     text,
					Lang:     "en-US",
					FontSize: fontSize,
					Dirty:    true,
					Color:    chromeFill(colorHex),
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// generateFooterShapes creates the p:sp elements for the footer zones.
// nextID is the first slide-unique shape ID to assign; each emitted footer
// shape consumes one ID, allocated sequentially so the left and right footers
// never collide with each other or with existing slide shapes.
//
// The left footer text is laid out on a single line (go-slide-creator-55i1):
// its box spans the dt + ftr placeholder width (see leftFooterBox) and the
// text shrinks, then ellipsizes, to fit that width. fontName is the theme
// body font used for measurement.
func generateFooterShapes(positions map[string]*transformXML, config *FooterConfig, nextID uint32, fontName, colorHex string, slideIndex int) string {
	var shapes []string

	// Size the page-number box first: it grows leftward from a fixed right edge,
	// so the left footer has to be laid out against the widened box or the two
	// overlap (go-slide-creator-pss1z).
	pageNum := resolvePageNumberSizing(positions, config.PageNumberFormat, config.TotalSlides, fontName)
	layout := positions
	if pageNum != nil {
		layout = withSldNum(positions, pageNum.box)
	}

	// Left footer (dt position, widened across ftr): configurable text, which may
	// vary per slide when a section crumb is enabled.
	if box := leftFooterBox(layout); box != nil && config.LeftTextFor(slideIndex) != "" {
		text, size := fitFooterText(config.LeftTextFor(slideIndex), box.Extent.CX, fontName)
		shapes = append(shapes, generateFooterShapeSized(nextID, "Footer Left", box, text, "l", size, colorHex))
		nextID++
	}

	// Right footer (sldNum position): auto-updating slide number field.
	// This is the last footer zone, so nextID is consumed but not advanced.
	if pageNum != nil {
		if config.PageNumberFormat != "" {
			shapes = append(shapes, generateFormattedSlideNumShape(nextID, "Footer Right", pageNum.box, config.PageNumberFormat, config.TotalSlides, pageNum.fontSize, colorHex))
		} else {
			shapes = append(shapes, generateSlideNumShape(nextID, "Footer Right", pageNum.box, pageNum.fontSize, colorHex))
		}
	}

	return strings.Join(shapes, "\n")
}

// generateSlideNumShape creates a footer shape with an auto-updating slide number field.
func generateSlideNumShape(shapeID uint32, name string, xfrm *transformXML, fontSize int, colorHex string) string {
	fieldID := "{" + uuid.New().String() + "}"
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   pptx.RectEmu{X: xfrm.Offset.X, Y: xfrm.Offset.Y, CX: xfrm.Extent.CX, CY: xfrm.Extent.CY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			// wrap="none": a page number that does not fit overflows its box
			// rather than stacking "2 /" over "10" in the corner.
			Wrap:   "none",
			Anchor: "ctr",
			Insets: [4]int64{91440, 0, 91440, 0},
			Paragraphs: []pptx.Paragraph{{
				Align: "r",
				Runs: []pptx.Run{{
					Text:      "\u2039#\u203a",
					Lang:      "en-US",
					FontSize:  fontSize,
					Dirty:     true,
					Color:     chromeFill(colorHex),
					FieldType: "slidenum",
					FieldID:   fieldID,
				}},
			}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// generateFormattedSlideNumShape creates a footer shape with a formatted page number.
// The format string may contain {current} (replaced by an auto-updating slidenum field)
// and {total} (replaced by a static total count). Text segments between fields are
// emitted as plain-text runs so PowerPoint renders "Slide 3 / 30" correctly.
func generateFormattedSlideNumShape(shapeID uint32, name string, xfrm *transformXML, format string, totalSlides, fontSize int, colorHex string) string {
	runs := buildPageNumberRuns(format, totalSlides, fontSize, colorHex)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   pptx.RectEmu{X: xfrm.Offset.X, Y: xfrm.Offset.Y, CX: xfrm.Extent.CX, CY: xfrm.Extent.CY},
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text: &pptx.TextBody{
			// wrap="none" for the same reason as the plain slide number: one line,
			// overflowing if it must (go-slide-creator-pss1z).
			Wrap:       "none",
			Anchor:     "ctr",
			Insets:     [4]int64{91440, 0, 91440, 0},
			Paragraphs: []pptx.Paragraph{{Align: "r", Runs: runs}},
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// buildPageNumberRuns splits a format string like "{current} / {total}" into
// pptx.Run slices: {current} becomes a slidenum field, {total} becomes a static
// text run with the total count, and everything else becomes plain text runs.
func buildPageNumberRuns(format string, totalSlides, fontSize int, colorHex string) []pptx.Run {
	var runs []pptx.Run
	remaining := format
	for len(remaining) > 0 {
		// Find the next placeholder
		currentIdx := strings.Index(remaining, "{current}")
		totalIdx := strings.Index(remaining, "{total}")

		// Pick the earliest placeholder
		nextIdx := -1
		nextLen := 0
		nextKind := ""
		if currentIdx >= 0 && (totalIdx < 0 || currentIdx <= totalIdx) {
			nextIdx = currentIdx
			nextLen = len("{current}")
			nextKind = "current"
		} else if totalIdx >= 0 {
			nextIdx = totalIdx
			nextLen = len("{total}")
			nextKind = "total"
		}

		if nextIdx < 0 {
			// No more placeholders — emit the rest as plain text
			if remaining != "" {
				runs = append(runs, pptx.Run{
					Text:     remaining,
					Lang:     "en-US",
					FontSize: fontSize,
					Dirty:    true,
					Color:    chromeFill(colorHex),
				})
			}
			break
		}

		// Emit text before the placeholder
		if nextIdx > 0 {
			runs = append(runs, pptx.Run{
				Text:     remaining[:nextIdx],
				Lang:     "en-US",
				FontSize: fontSize,
				Dirty:    true,
				Color:    chromeFill(colorHex),
			})
		}

		switch nextKind {
		case "current":
			fieldID := "{" + uuid.New().String() + "}"
			runs = append(runs, pptx.Run{
				Text:      "\u2039#\u203a",
				Lang:      "en-US",
				FontSize:  fontSize,
				Dirty:     true,
				Color:     chromeFill(colorHex),
				FieldType: "slidenum",
				FieldID:   fieldID,
			})
		case "total":
			runs = append(runs, pptx.Run{
				Text:     fmt.Sprintf("%d", totalSlides),
				Lang:     "en-US",
				FontSize: fontSize,
				Dirty:    true,
				Color:    chromeFill(colorHex),
			})
		}

		remaining = remaining[nextIdx+nextLen:]
	}

	return runs
}

// insertFooters inserts footer shapes into slide XML before </p:spTree>.
// fontName is the theme body font used to fit the left footer text on one line.
func insertFooters(slideData []byte, footerConfig *FooterConfig, positions map[string]*transformXML, fontName, colorHex string, slideIndex int) ([]byte, error) {
	if footerConfig == nil || !footerConfig.Enabled {
		return slideData, nil
	}

	if len(positions) == 0 {
		return slideData, nil
	}

	// Allocate slide-unique IDs above any existing shape (including the
	// takeaway/source-note shapes injected earlier on this slide).
	footerXML := generateFooterShapes(positions, footerConfig, findMaxShapeID(slideData)+1, fontName, colorHex, slideIndex)
	if footerXML == "" {
		return slideData, nil
	}

	return pptx.InsertIntoSpTree(slideData, []byte(footerXML), pptx.InsertAtEnd)
}

// extractSlideTitle finds the title text from a slide's content items.
// It looks for content items whose PlaceholderID contains "title" (case-insensitive).
func extractSlideTitle(content []ContentItem) string {
	for _, item := range content {
		if item.Type != ContentText {
			continue
		}
		if isLikelyTitlePlaceholder(item.PlaceholderID) {
			if text, ok := item.Value.(string); ok {
				return text
			}
		}
	}
	return ""
}

// isLikelyTitlePlaceholder returns true if a placeholder ID looks like a title.
func isLikelyTitlePlaceholder(placeholderID string) bool {
	lower := strings.ToLower(placeholderID)
	return strings.Contains(lower, "title") || strings.Contains(lower, "heading")
}

// footerPlaceholderTypes are the OOXML placeholder types used for footer zones.
var footerPlaceholderTypes = map[string]bool{
	"dt":     true, // Date/time
	"ftr":    true, // Footer text
	"sldNum": true, // Slide number
}

// removeFooterPlaceholders removes template footer placeholder shapes (dt, ftr, sldNum)
// from a slide so they don't overlap with our injected footer shapes.
func removeFooterPlaceholders(slide *slideXML) *slideXML {
	filtered := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for _, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph != nil && footerPlaceholderTypes[ph.Type] {
			continue // Skip footer placeholder shapes
		}
		filtered = append(filtered, shape)
	}
	slide.CommonSlideData.ShapeTree.Shapes = filtered
	return slide
}

// removeEmptyFooterPlaceholders removes footer placeholder shapes (dt, ftr, sldNum)
// that have no text content. Templates often include these placeholders with empty
// <a:p></a:p> elements; PowerPoint normally auto-populates them from presentation-level
// settings (Insert > Header & Footer), but our generator does not set those properties,
// so the shapes render as blank rectangles occupying space at the bottom of the slide.
func removeEmptyFooterPlaceholders(slide *slideXML) *slideXML {
	filtered := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for _, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph != nil && footerPlaceholderTypes[ph.Type] && isShapeTextEmpty(&shape) {
			continue // Skip empty footer placeholder shapes
		}
		filtered = append(filtered, shape)
	}
	slide.CommonSlideData.ShapeTree.Shapes = filtered
	return slide
}

// isShapeTextEmpty returns true if a shape has no visible text content.
// A shape is considered empty if it has no text body, no paragraphs,
// or all paragraphs have zero runs (i.e., only empty <a:p/> elements).
func isShapeTextEmpty(shape *shapeXML) bool {
	if shape.TextBody == nil {
		return true
	}
	if len(shape.TextBody.Paragraphs) == 0 {
		return true
	}
	for _, para := range shape.TextBody.Paragraphs {
		for _, run := range para.Runs {
			if strings.TrimSpace(run.Text) != "" {
				return false
			}
		}
	}
	return true
}

// removeEmptyPlaceholders removes any placeholder shape that has no visible
// text content. This prevents layout-inherited empty placeholders (e.g., an
// unpopulated subtitle on a closing slide) from covering populated shapes in
// PowerPoint. Footer placeholders are excluded because they are handled
// separately by removeFooterPlaceholders / removeEmptyFooterPlaceholders.
func removeEmptyPlaceholders(slide *slideXML) *slideXML {
	filtered := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for _, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		ph := shape.NonVisualProperties.NvPr.Placeholder
		if ph != nil && !footerPlaceholderTypes[ph.Type] && isShapeTextEmpty(&shape) {
			continue // Skip empty non-footer placeholder shapes
		}
		filtered = append(filtered, shape)
	}
	slide.CommonSlideData.ShapeTree.Shapes = filtered
	return slide
}

// getShapeText returns the concatenated text content of a shape.
func getShapeText(shape *shapeXML) string {
	if shape.TextBody == nil {
		return ""
	}
	var parts []string
	for _, para := range shape.TextBody.Paragraphs {
		for _, run := range para.Runs {
			parts = append(parts, run.Text)
		}
	}
	return strings.Join(parts, "")
}

// removeDuplicateFooterBrandMark removes non-placeholder text shapes from the slide
// that would duplicate the footer's LeftText. Some templates include static brand-mark
// text shapes in slide layouts that get copied into generated slides. When our
// footer injection adds the same text via LeftText, the brand mark appears
// twice. This function removes layout-inherited brand marks that match the footer text.
//
// A shape is removed when ALL of these conditions are met:
//   - It has no placeholder element (non-placeholder text box)
//   - Its text content exactly matches the footer LeftText (case-sensitive)
//   - It is positioned in the footer zone (bottom 10% of slide height)
func removeDuplicateFooterBrandMark(slide *slideXML, leftText string, slideHeight int64) *slideXML {
	if leftText == "" {
		return slide
	}
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	// Footer zone threshold: shapes with Y position in the bottom 10% of the slide
	footerZoneY := slideHeight - slideHeight/10 // 90% from top

	filtered := make([]shapeXML, 0, len(slide.CommonSlideData.ShapeTree.Shapes))
	for _, shape := range slide.CommonSlideData.ShapeTree.Shapes {
		ph := shape.NonVisualProperties.NvPr.Placeholder
		// Only consider non-placeholder shapes in the footer zone
		if ph == nil &&
			shape.ShapeProperties.Transform != nil &&
			shape.ShapeProperties.Transform.Offset.Y >= footerZoneY &&
			strings.TrimSpace(getShapeText(&shape)) == strings.TrimSpace(leftText) {
			continue // Skip — this brand mark would duplicate the footer LeftText
		}
		filtered = append(filtered, shape)
	}
	slide.CommonSlideData.ShapeTree.Shapes = filtered
	return slide
}

// resolveFooterPositions extracts footer placeholder positions from a master positions map.
// Returns positions for keys "type:dt", "type:ftr", "type:sldNum".
// If the master doesn't define these, returns default positions.
// Positions are clamped to ensure they remain within the visible slide area.
// slideHeight is the actual slide height in EMU (0 = use 16:9 default).
func resolveFooterPositions(masterPositions map[string]*transformXML, slideHeight int64) map[string]*transformXML {
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	footerKeys := []string{"type:dt", "type:ftr", "type:sldNum"}

	positions := make(map[string]*transformXML)
	for _, key := range footerKeys {
		if xfrm, ok := masterPositions[key]; ok {
			positions[key] = xfrm
		}
	}

	// If no footer positions found in master/layout, compute defaults from slide height
	if len(positions) == 0 {
		for k, v := range computeDefaultFooterPositions(slideHeight) {
			positions[k] = v
		}
	}

	// Normalize vertical alignment: all footer elements must share the same Y and CY
	// so they appear on a single baseline. Templates often define different Y positions
	// for dt, ftr, and sldNum placeholders, leading to visually misaligned footers.
	normalizeFooterVerticalPositions(positions, slideHeight)

	// Enforce minimum width for slide number placeholder so double/triple-digit
	// numbers don't wrap to multiple lines. Keep right edge fixed, expand leftward.
	if pos, ok := positions["type:sldNum"]; ok && pos.Extent.CX < minSldNumWidth {
		rightEdge := pos.Offset.X + pos.Extent.CX
		pos.Extent.CX = minSldNumWidth
		pos.Offset.X = rightEdge - minSldNumWidth
	}

	// Clamp footer positions to ensure they stay within the visible slide area.
	// Some templates define system footer placeholders (dt) below the
	// slide boundary because PowerPoint's Header & Footer dialog hides them.
	for _, pos := range positions {
		clampFooterPosition(pos, slideHeight)
	}

	return positions
}

// normalizeFooterVerticalPositions ensures all footer elements share the same Y and CY
// so they appear vertically aligned. Uses the highest visible Y position (lowest on slide
// but still on-screen) and maximum CY across all positions. Off-screen placeholders
// (y + cy > slideHeight, common for hidden date fields) are excluded from the Y
// calculation so they don't drag visible footers off-screen.
func normalizeFooterVerticalPositions(positions map[string]*transformXML, slideHeight int64) {
	if len(positions) == 0 {
		return
	}
	if slideHeight <= 0 {
		slideHeight = defaultSlideHeightEMU
	}
	var visibleY, maxCY int64
	for _, pos := range positions {
		if pos.Offset.Y+pos.Extent.CY <= slideHeight {
			if pos.Offset.Y > visibleY {
				visibleY = pos.Offset.Y
			}
		}
		if pos.Extent.CY > maxCY {
			maxCY = pos.Extent.CY
		}
	}
	for _, pos := range positions {
		pos.Offset.Y = visibleY
		pos.Extent.CY = maxCY
	}
}

// clampFooterPosition ensures a footer shape stays within the visible slide area.
// If the bottom edge (Y + Height) exceeds slideHeight, Y is adjusted upward.
func clampFooterPosition(pos *transformXML, slideHeight int64) {
	if pos.Offset.Y+pos.Extent.CY > slideHeight {
		pos.Offset.Y = slideHeight - pos.Extent.CY
	}
}
