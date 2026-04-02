package template

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ParseLayouts extracts layout metadata from a PPTX template.
// It resolves placeholder transforms from slide masters when layouts inherit positioning.
// It also resolves font properties from the hierarchy: shape → layout → master → theme.
func ParseLayouts(reader *Reader) ([]types.LayoutMetadata, error) {
	// List all slide layout files
	layoutFiles, err := reader.ListFiles("ppt/slideLayouts/slideLayout*.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to list layout files: %w", err)
	}

	if len(layoutFiles) == 0 {
		return nil, fmt.Errorf("no slide layouts found in template")
	}

	// Create a master position resolver for transform inheritance
	masterResolver := NewMasterPositionResolver(reader)

	// Parse theme for font resolution
	theme := ParseTheme(reader)

	// Create a master font resolver for font inheritance
	fontResolver := NewMasterFontResolver(reader, &theme)

	var layouts []types.LayoutMetadata
	for i, layoutFile := range layoutFiles {
		layout, err := parseLayoutFile(reader, layoutFile, i, masterResolver, fontResolver)
		if err != nil {
			// Log warning but continue - lenient parsing
			slog.Debug("failed to parse layout file",
				slog.String("file", layoutFile),
				slog.String("error", err.Error()),
			)
			continue
		}
		layouts = append(layouts, layout)
	}

	if len(layouts) == 0 {
		return nil, fmt.Errorf("failed to parse any layouts from template")
	}

	return layouts, nil
}

// parseLayoutFile parses a single slideLayout XML file.
// masterResolver is used to resolve placeholder transforms inherited from slide masters.
// fontResolver is used to resolve placeholder fonts inherited from slide masters.
func parseLayoutFile(reader *Reader, filename string, index int, masterResolver *MasterPositionResolver, fontResolver *MasterFontResolver) (types.LayoutMetadata, error) {
	data, err := reader.ReadFile(filename)
	if err != nil {
		return types.LayoutMetadata{}, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	var xmlLayout slideLayoutXML
	if err := xml.Unmarshal(data, &xmlLayout); err != nil {
		return types.LayoutMetadata{}, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	// Extract layout name from cSld
	name := "Unknown"
	if xmlLayout.CommonSlideData.Name != "" {
		name = xmlLayout.CommonSlideData.Name
	}

	// Get the layout ID for master resolution
	layoutID := extractLayoutID(filename)

	// Get master positions for this layout (nil if not available)
	var masterPositions map[string]*MasterTransform
	if masterResolver != nil {
		masterPositions = masterResolver.GetMasterPositionsForLayout(layoutID)
	}

	// Get master font styles for this layout (nil if not available)
	var masterFonts *MasterFontStyles
	if fontResolver != nil {
		masterFonts = fontResolver.GetMasterFontsForLayout(layoutID)
	}

	// Extract placeholders from shape tree with master position and font fallback
	placeholders := extractPlaceholders(xmlLayout.CommonSlideData.ShapeTree.Shapes, name, masterPositions, masterFonts, fontResolver)

	// Calculate capacity estimate
	capacity := estimateCapacity(placeholders)

	layoutMeta := types.LayoutMetadata{
		ID:           layoutID,
		Name:         name,
		Index:        index,
		Placeholders: placeholders,
		Capacity:     capacity,
		Tags:         []string{},
	}

	// Classify layout to populate tags
	ClassifyLayout(&layoutMeta)

	return layoutMeta, nil
}

// extractLayoutID extracts the layout ID from the filename.
func extractLayoutID(filename string) string {
	// Extract slideLayoutN.xml -> N
	parts := strings.Split(filename, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		return strings.TrimSuffix(name, ".xml")
	}
	return filename
}

// extractPlaceholders extracts placeholder information from shapes.
// layoutName is used for logging context when transform data is missing.
// masterPositions provides fallback transforms for placeholders that inherit from the master.
// masterFonts provides fallback font styles from the master's txStyles.
// fontResolver is used to resolve fonts from shapes and master.
func extractPlaceholders(shapes []shapeXML, layoutName string, masterPositions map[string]*MasterTransform, masterFonts *MasterFontStyles, fontResolver *MasterFontResolver) []types.PlaceholderInfo {
	var placeholders []types.PlaceholderInfo

	for i, shape := range shapes {
		// Skip shapes without placeholder properties
		if shape.NonVisualProperties.Placeholder == nil {
			continue
		}

		ph := shape.NonVisualProperties.Placeholder

		// Determine placeholder type
		phType := determinePlaceholderType(ph.Type)

		// Extract bounds from transform with master position fallback
		bounds := ResolvePlaceholderBounds(
			shape.ShapeProperties.Transform,
			ph,
			masterPositions,
			layoutName,
			i,
		)

		// Estimate character capacity for text placeholders
		maxChars := 0
		if phType == types.PlaceholderTitle || phType == types.PlaceholderSubtitle || phType == types.PlaceholderBody || phType == types.PlaceholderContent {
			maxChars = estimateMaxChars(bounds)
		}

		// Get placeholder index (defaults to 0 if not specified)
		idx := 0
		if ph.Index != nil {
			idx = *ph.Index
		}

		// Resolve font properties from shape → master hierarchy
		var fontFamily string
		var fontSize int
		var fontColor string
		if fontResolver != nil {
			if fontStyle := fontResolver.ResolvePlaceholderFonts(&shape, phType, masterFonts); fontStyle != nil {
				fontFamily = fontStyle.FontFamily
				fontSize = fontStyle.FontSize
				fontColor = fontStyle.FontColor
			}
		}

		placeholders = append(placeholders, types.PlaceholderInfo{
			ID:         shape.NonVisualProperties.ConnectionNonVisual.Name,
			Type:       phType,
			Index:      idx,
			Bounds:     bounds,
			MaxChars:   maxChars,
			FontFamily: fontFamily,
			FontSize:   fontSize,
			FontColor:  fontColor,
		})
	}

	return placeholders
}

// determinePlaceholderType maps XML placeholder type to our type system.
func determinePlaceholderType(phType string) types.PlaceholderType {
	switch phType {
	case "title", "ctrTitle":
		return types.PlaceholderTitle
	case "subTitle":
		// Subtitle placeholder, typically used on title slides
		return types.PlaceholderSubtitle
	case "body":
		return types.PlaceholderBody
	case "pic":
		return types.PlaceholderImage
	case "chart":
		return types.PlaceholderChart
	case "tbl":
		return types.PlaceholderTable
	case "dt", "ftr", "sldNum", "hdr":
		// Non-content utility placeholders: date, footer, slide number, header
		return types.PlaceholderOther
	default:
		// Generic content placeholder for unlabeled or unknown types
		// This includes placeholders with no type attribute (idx only)
		return types.PlaceholderContent
	}
}

// extractBounds extracts bounding box from shape properties.
// layoutName and shapeIndex are used for logging when transform data is missing.
func extractBounds(props shapePropertiesXML, layoutName string, shapeIndex int) types.BoundingBox {
	if props.Transform == nil {
		slog.Warn("placeholder missing transform",
			"layout", layoutName,
			"shape_index", shapeIndex,
			"needs_master_resolution", true)
		return types.BoundingBox{}
	}

	bounds := types.BoundingBox{}

	if props.Transform.Offset != nil {
		bounds.X = props.Transform.Offset.X
		bounds.Y = props.Transform.Offset.Y
	}

	if props.Transform.Extents != nil {
		bounds.Width = props.Transform.Extents.CX
		bounds.Height = props.Transform.Extents.CY
	}

	return bounds
}

// estimateMaxChars estimates character capacity based on placeholder dimensions.
// Formula: MaxChars = (Width / CharWidthEMU) * (Height / LineHeightEMU) * 0.4
func estimateMaxChars(bounds types.BoundingBox) int {
	const (
		charWidthEMU  = 91440  // 1 inch / 10 characters at standard font
		lineHeightEMU = 274320 // 0.3 inches per line
		safetyFactor  = 0.4    // Safety factor for margins and padding
	)

	if bounds.Width == 0 || bounds.Height == 0 {
		return 0
	}

	charsPerLine := float64(bounds.Width) / charWidthEMU
	lines := float64(bounds.Height) / lineHeightEMU

	maxChars := int(charsPerLine * lines * safetyFactor)

	// Sanity check - avoid unrealistic values
	if maxChars < 0 {
		return 0
	}
	if maxChars > 10000 {
		return 10000
	}

	return maxChars
}

// estimateCapacity calculates capacity estimates for a layout.
// Note: PlaceholderOther (date, footer, slide number) is excluded from capacity calculations.
func estimateCapacity(placeholders []types.PlaceholderInfo) types.CapacityEstimate {
	capacity := types.CapacityEstimate{}

	for _, ph := range placeholders {
		switch ph.Type {
		case types.PlaceholderImage:
			capacity.HasImageSlot = true
		case types.PlaceholderChart:
			capacity.HasChartSlot = true
		case types.PlaceholderBody, types.PlaceholderContent:
			// Both Body and Content placeholders can hold bullets/text
			// PlaceholderContent is used for templates with non-standard types (type="unknown")
			// Estimate bullets: roughly 1 bullet per 100 characters
			if ph.MaxChars > 0 {
				bullets := ph.MaxChars / 100
				if bullets > capacity.MaxBullets {
					capacity.MaxBullets = bullets
				}
			}
			// Estimate text lines: roughly 50 characters per line
			if ph.MaxChars > 0 {
				lines := ph.MaxChars / 50
				if lines > capacity.MaxTextLines {
					capacity.MaxTextLines = lines
				}
			}
			// PlaceholderOther (dt, ftr, sldNum, hdr) - intentionally excluded
		}
	}

	// Determine if text-heavy or visual-focused
	// Note: PlaceholderOther is excluded from this count as well
	textPlaceholders := 0
	visualPlaceholders := 0

	for _, ph := range placeholders {
		// Include PlaceholderContent as text placeholder since it often represents body content
		// Exclude PlaceholderOther (utility placeholders like date/footer/slide number)
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderTitle || ph.Type == types.PlaceholderContent {
			textPlaceholders++
		}
		if ph.Type == types.PlaceholderImage || ph.Type == types.PlaceholderChart {
			visualPlaceholders++
		}
	}

	capacity.TextHeavy = textPlaceholders > 0 && visualPlaceholders == 0
	capacity.VisualFocused = visualPlaceholders > 0 && visualPlaceholders >= textPlaceholders

	return capacity
}

// ExtractPlaceholderBoundsFromZip parses placeholder bounds from layout files in a zip archive.
// Returns a map of layoutID -> placeholderID -> BoundingBox (in EMUs).
// This is a lightweight version of ParseLayouts for use cases that only need placeholder bounds,
// such as pre-rendering charts at the correct size before full slide processing.
//
// Unlike ParseLayouts, this does NOT resolve bounds from master slides. It only extracts bounds
// that are explicitly defined in the layout XML. For placeholders that inherit from master slides,
// the bounds will be zero. This is acceptable for pre-rendering because:
// 1. Most layouts have explicit bounds for content placeholders
// 2. Charts can still fall back to default dimensions if bounds are zero
func ExtractPlaceholderBoundsFromZip(zipReader *zip.Reader) map[string]map[string]types.BoundingBox {
	result := make(map[string]map[string]types.BoundingBox)

	// Find all layout files
	for _, f := range zipReader.File {
		matched, _ := filepath.Match("ppt/slideLayouts/slideLayout*.xml", f.Name)
		if !matched {
			continue
		}

		// Extract layout ID (e.g., "slideLayout1" from "ppt/slideLayouts/slideLayout1.xml")
		layoutID := extractLayoutID(f.Name)

		// Parse the layout file
		bounds, err := parseLayoutBounds(f)
		if err != nil {
			slog.Debug("failed to parse layout bounds",
				slog.String("file", f.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		result[layoutID] = bounds
	}

	return result
}

// parseLayoutBounds parses a single layout file and extracts placeholder bounds.
// Returns a map of placeholderID -> BoundingBox.
func parseLayoutBounds(f *zip.File) (map[string]types.BoundingBox, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, utils.MaxZipEntrySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	var xmlLayout slideLayoutXML
	if err := xml.Unmarshal(data, &xmlLayout); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	bounds := make(map[string]types.BoundingBox)
	for _, shape := range xmlLayout.CommonSlideData.ShapeTree.Shapes {
		// Skip shapes without placeholder properties
		if shape.NonVisualProperties.Placeholder == nil {
			continue
		}

		// Get placeholder ID from shape name
		placeholderID := shape.NonVisualProperties.ConnectionNonVisual.Name
		if placeholderID == "" {
			continue
		}

		// Extract bounds from transform
		if shape.ShapeProperties.Transform != nil {
			bounds[placeholderID] = types.BoundingBox{
				X:      shape.ShapeProperties.Transform.Offset.X,
				Y:      shape.ShapeProperties.Transform.Offset.Y,
				Width:  shape.ShapeProperties.Transform.Extents.CX,
				Height: shape.ShapeProperties.Transform.Extents.CY,
			}
		}
	}

	return bounds, nil
}

// XML structure definitions for parsing PPTX slide layouts

type slideLayoutXML struct {
	XMLName         xml.Name           `xml:"sldLayout"`
	Type            string             `xml:"type,attr"`
	CommonSlideData commonSlideDataXML `xml:"cSld"`
}

type commonSlideDataXML struct {
	Name      string       `xml:"name,attr"`
	ShapeTree shapeTreeXML `xml:"spTree"`
}

type shapeTreeXML struct {
	Shapes []shapeXML `xml:"sp"`
}

type shapeXML struct {
	NonVisualProperties nonVisualPropertiesXML `xml:"nvSpPr"`
	ShapeProperties     shapePropertiesXML     `xml:"spPr"`
	TextBody            *textBodyXML           `xml:"txBody"`
}

// textBodyXML represents the text body of a shape with list styles for font extraction.
type textBodyXML struct {
	ListStyle *listStyleXML `xml:"lstStyle"`
}

// listStyleXML represents list style with level paragraph properties.
// Used for font extraction from placeholder lstStyle.
type listStyleXML struct {
	Lvl1pPr *levelParagraphPropsXML `xml:"lvl1pPr"`
}

// levelParagraphPropsXML represents level paragraph properties with default run properties.
// Defined here for use by both layouts.go and font_resolver.go.
type levelParagraphPropsXML struct {
	DefRPr *defaultRunPropsXML `xml:"defRPr"`
}

// defaultRunPropsXML represents default run properties with font attributes.
type defaultRunPropsXML struct {
	Size      int           `xml:"sz,attr"`
	Latin     *latinFontXML `xml:"latin"`
	SolidFill *solidFillXML `xml:"solidFill"`
}

// latinFontXML represents a Latin font typeface.
type latinFontXML struct {
	Typeface string `xml:"typeface,attr"`
}

// solidFillXML represents a solid fill with color.
type solidFillXML struct {
	SRGBColor   *srgbColorXML   `xml:"srgbClr"`
	SchemeColor *schemeColorXML `xml:"schemeClr"`
}

// srgbColorXML represents an sRGB color value.
type srgbColorXML struct {
	Val string `xml:"val,attr"`
}

// schemeColorXML represents a scheme color reference.
type schemeColorXML struct {
	Val string `xml:"val,attr"`
}

type nonVisualPropertiesXML struct {
	ConnectionNonVisual connectionNonVisualXML `xml:"cNvPr"`
	Placeholder         *placeholderXML        `xml:"nvPr>ph"`
}

type connectionNonVisualXML struct {
	Name string `xml:"name,attr"`
}

type placeholderXML struct {
	Type  string `xml:"type,attr"`
	Index *int   `xml:"idx,attr"`
}

type shapePropertiesXML struct {
	Transform *transformXML `xml:"xfrm"`
}

type transformXML struct {
	Offset  *offsetXML  `xml:"off"`
	Extents *extentsXML `xml:"ext"`
}

type offsetXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type extentsXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}
