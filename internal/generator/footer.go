package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/google/uuid"
)

// Footer shape IDs — high values to avoid conflicts with content shapes and source note (999).
const (
	footerLeftShapeID  uint32 = 990
	footerRightShapeID uint32 = 992
	footerFontSize            = 1050 // 10.5pt in hundredths of a point
)

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

// generateFooterShape creates a single p:sp element for a footer zone.
func generateFooterShape(shapeID uint32, name string, xfrm *transformXML, text string, alignment string) string {
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
					FontSize: footerFontSize,
					Dirty:    true,
					Color:    pptx.SchemeFill("tx1"),
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
func generateFooterShapes(positions map[string]*transformXML, leftText string) string {
	var shapes []string

	// Left footer (dt position): configurable text
	if pos, ok := positions["type:dt"]; ok && leftText != "" {
		shapes = append(shapes, generateFooterShape(footerLeftShapeID, "Footer Left", pos, leftText, "l"))
	}

	// Right footer (sldNum position): auto-updating slide number field
	if pos, ok := positions["type:sldNum"]; ok {
		shapes = append(shapes, generateSlideNumShape(footerRightShapeID, "Footer Right", pos))
	}

	return strings.Join(shapes, "\n")
}

// generateSlideNumShape creates a footer shape with an auto-updating slide number field.
func generateSlideNumShape(shapeID uint32, name string, xfrm *transformXML) string {
	fieldID := "{" + uuid.New().String() + "}"
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
				Align: "r",
				Runs: []pptx.Run{{
					Text:      "\u2039#\u203a",
					Lang:      "en-US",
					FontSize:  footerFontSize,
					Dirty:     true,
					Color:     pptx.SchemeFill("tx1"),
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

// insertFooters inserts footer shapes into slide XML before </p:spTree>.
func insertFooters(slideData []byte, footerConfig *FooterConfig, positions map[string]*transformXML) ([]byte, error) {
	if footerConfig == nil || !footerConfig.Enabled {
		return slideData, nil
	}

	if len(positions) == 0 {
		return slideData, nil
	}

	footerXML := generateFooterShapes(positions, footerConfig.LeftText)
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
