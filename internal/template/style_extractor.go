package template

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sebahrens/json2pptx/internal/types"
)

// PlaceholderStyle contains all styling information for a placeholder.
// This is used when generating new layouts to replicate the visual style
// of the template's existing placeholders.
type PlaceholderStyle struct {
	// Position and dimensions (EMUs)
	Bounds types.BoundingBox

	// Text styling
	FontFamily string // Font family name (e.g., "Arial", "Calibri")
	FontSize   int    // Hundredths of a point (e.g., 1800 = 18pt)
	FontColor  string // Hex color (e.g., "#000000") or empty for scheme default
	FontBold   bool
	FontItalic bool

	// Paragraph styling
	LineSpacing  int   // Line spacing percentage (e.g., 100 = single, 150 = 1.5)
	SpaceBefore  int64 // Space before paragraph in EMUs
	SpaceAfter   int64 // Space after paragraph in EMUs
	MarginLeft   int64 // Left margin in EMUs (from bodyPr lIns)
	MarginRight  int64 // Right margin in EMUs (from bodyPr rIns)
	MarginTop    int64 // Top margin in EMUs (from bodyPr tIns)
	MarginBottom int64 // Bottom margin in EMUs (from bodyPr bIns)

	// First level bullet/paragraph styling
	BulletChar    string // Bullet character (empty if none)
	BulletColor   string // Hex color for bullet
	BulletSize    int    // Percentage of text size (e.g., 100)
	BulletIndent  int64  // Indent from margin for text (EMUs)
	BulletMarginL int64  // Left margin for first level (EMUs)

	// Background
	FillType  string // "none", "solid", "gradient"
	FillColor string // If solid fill
}

// ContentAreaBounds represents the usable content area extracted from a layout.
// This defines where content placeholders can be positioned.
type ContentAreaBounds struct {
	X      int64 // Left edge (EMUs from slide left)
	Y      int64 // Top edge (EMUs from slide top)
	Width  int64 // Available width (EMUs)
	Height int64 // Available height (EMUs)
}

// ExtractPlaceholderStyle extracts styling from a specific placeholder in a layout.
// It resolves inherited properties from the slide master when needed.
func ExtractPlaceholderStyle(reader *Reader, layoutIdx int, placeholderType types.PlaceholderType) (*PlaceholderStyle, error) {
	// List layout files to find the one at the given index
	layoutFiles, err := reader.ListFiles("ppt/slideLayouts/slideLayout*.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to list layout files: %w", err)
	}

	if layoutIdx < 0 || layoutIdx >= len(layoutFiles) {
		return nil, fmt.Errorf("layout index %d out of range (0-%d)", layoutIdx, len(layoutFiles)-1)
	}

	layoutFile := layoutFiles[layoutIdx]

	// Read and parse the layout XML
	data, err := reader.ReadFile(layoutFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read layout file %s: %w", layoutFile, err)
	}

	var layout slideLayoutStyleXML
	if err := xml.Unmarshal(data, &layout); err != nil {
		return nil, fmt.Errorf("failed to parse layout XML: %w", err)
	}

	// Find the placeholder by type
	for _, shape := range layout.CommonSlideData.ShapeTree.Shapes {
		if shape.NonVisualProperties.Placeholder == nil {
			continue
		}

		phType := determinePlaceholderType(shape.NonVisualProperties.Placeholder.Type)
		if phType != placeholderType {
			continue
		}

		// Extract style from this shape
		return extractStyleFromShape(&shape, reader, extractLayoutID(layoutFile)), nil
	}

	return nil, fmt.Errorf("placeholder type %s not found in layout %d", placeholderType, layoutIdx)
}

// ExtractContentAreaBounds extracts the content placeholder bounds from a content layout.
// This returns the position and size of the body/content placeholder.
func ExtractContentAreaBounds(reader *Reader, contentLayoutIdx int) (*ContentAreaBounds, error) {
	style, err := ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderBody)
	if err != nil {
		// Try PlaceholderContent if PlaceholderBody not found
		style, err = ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderContent)
		if err != nil {
			return nil, fmt.Errorf("no body or content placeholder found: %w", err)
		}
	}

	return &ContentAreaBounds{
		X:      style.Bounds.X,
		Y:      style.Bounds.Y,
		Width:  style.Bounds.Width,
		Height: style.Bounds.Height,
	}, nil
}

// ExtractContentAreaBoundsFromLayouts finds and extracts content area bounds
// from a slice of parsed layouts.
func ExtractContentAreaBoundsFromLayouts(layouts []types.LayoutMetadata, contentLayoutIdx int) (*ContentAreaBounds, error) {
	if contentLayoutIdx < 0 || contentLayoutIdx >= len(layouts) {
		return nil, fmt.Errorf("content layout index %d out of range", contentLayoutIdx)
	}

	layout := layouts[contentLayoutIdx]

	// Find body or content placeholder
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			return &ContentAreaBounds{
				X:      ph.Bounds.X,
				Y:      ph.Bounds.Y,
				Width:  ph.Bounds.Width,
				Height: ph.Bounds.Height,
			}, nil
		}
	}

	return nil, fmt.Errorf("no body or content placeholder found in layout %d (%s)", contentLayoutIdx, layout.Name)
}

// extractStyleFromShape extracts placeholder style from a shape XML element.
func extractStyleFromShape(shape *shapeStyleXML, reader *Reader, layoutID string) *PlaceholderStyle {
	style := &PlaceholderStyle{
		FillType: "none",
	}

	// Extract bounds from transform
	if shape.ShapeProperties.Transform != nil {
		if shape.ShapeProperties.Transform.Offset != nil {
			style.Bounds.X = shape.ShapeProperties.Transform.Offset.X
			style.Bounds.Y = shape.ShapeProperties.Transform.Offset.Y
		}
		if shape.ShapeProperties.Transform.Extents != nil {
			style.Bounds.Width = shape.ShapeProperties.Transform.Extents.CX
			style.Bounds.Height = shape.ShapeProperties.Transform.Extents.CY
		}
	}

	// Extract body properties (margins)
	if shape.TextBody != nil && shape.TextBody.BodyProperties != nil {
		bp := shape.TextBody.BodyProperties
		style.MarginLeft = bp.LeftInset
		style.MarginRight = bp.RightInset
		style.MarginTop = bp.TopInset
		style.MarginBottom = bp.BottomInset
	}

	// Extract text style from lstStyle if present
	if shape.TextBody != nil && shape.TextBody.ListStyle != nil {
		extractListStyle(shape.TextBody.ListStyle, style)
	}

	// Try to get font properties from master if not found in shape
	if style.FontFamily == "" || style.FontSize == 0 {
		theme := ParseTheme(reader)
		fontResolver := NewMasterFontResolver(reader, &theme)
		masterFonts := fontResolver.GetMasterFontsForLayout(layoutID)

		phType := types.PlaceholderBody
		if shape.NonVisualProperties.Placeholder != nil {
			phType = determinePlaceholderType(shape.NonVisualProperties.Placeholder.Type)
		}

		// Create a shapeXML wrapper for the font resolver
		shapeXML := convertToShapeXML(shape)
		if fontStyle := fontResolver.ResolvePlaceholderFonts(shapeXML, phType, masterFonts); fontStyle != nil {
			if style.FontFamily == "" {
				style.FontFamily = fontStyle.FontFamily
			}
			if style.FontSize == 0 {
				style.FontSize = fontStyle.FontSize
			}
			if style.FontColor == "" {
				style.FontColor = fontStyle.FontColor
			}
		}
	}

	return style
}

// extractListStyle extracts text styling from list style properties.
func extractListStyle(lstStyle *listStyleStyleXML, style *PlaceholderStyle) {
	if lstStyle == nil || lstStyle.Lvl1pPr == nil {
		return
	}

	lvl1 := lstStyle.Lvl1pPr

	// Extract paragraph margins
	style.BulletMarginL = lvl1.MarginLeft
	style.BulletIndent = lvl1.Indent

	// Extract spacing
	if lvl1.SpaceBefore != nil && lvl1.SpaceBefore.SpacePoints != nil {
		style.SpaceBefore = int64(lvl1.SpaceBefore.SpacePoints.Val) * 12700 // centipoints to EMU
	}
	if lvl1.SpaceAfter != nil && lvl1.SpaceAfter.SpacePoints != nil {
		style.SpaceAfter = int64(lvl1.SpaceAfter.SpacePoints.Val) * 12700
	}
	if lvl1.LineSpacing != nil && lvl1.LineSpacing.SpacePercent != nil {
		style.LineSpacing = lvl1.LineSpacing.SpacePercent.Val / 1000 // convert from thousandths
	}

	// Extract bullet properties
	if lvl1.BulletChar != nil {
		style.BulletChar = lvl1.BulletChar.Char
	}
	if lvl1.BulletColor != nil {
		if lvl1.BulletColor.SRGBColor != nil {
			style.BulletColor = normalizeColorHex(lvl1.BulletColor.SRGBColor.Val)
		} else if lvl1.BulletColor.SchemeColor != nil {
			style.BulletColor = lvl1.BulletColor.SchemeColor.Val
		}
	}
	if lvl1.BulletSizePercent != nil {
		style.BulletSize = lvl1.BulletSizePercent.Val / 1000
	}

	// Extract default run properties (font)
	if lvl1.DefRPr != nil {
		defRPr := lvl1.DefRPr
		style.FontSize = defRPr.Size
		style.FontBold = defRPr.Bold
		style.FontItalic = defRPr.Italic

		if defRPr.Latin != nil {
			style.FontFamily = defRPr.Latin.Typeface
		}
		if defRPr.SolidFill != nil {
			if defRPr.SolidFill.SRGBColor != nil {
				style.FontColor = normalizeColorHex(defRPr.SolidFill.SRGBColor.Val)
			} else if defRPr.SolidFill.SchemeColor != nil {
				style.FontColor = defRPr.SolidFill.SchemeColor.Val
			}
		}
	}
}

// convertToShapeXML converts shapeStyleXML to shapeXML for font resolver compatibility.
func convertToShapeXML(shape *shapeStyleXML) *shapeXML {
	result := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{
				Name: shape.NonVisualProperties.ConnectionNonVisual.Name,
			},
		},
	}

	if shape.NonVisualProperties.Placeholder != nil {
		ph := shape.NonVisualProperties.Placeholder
		result.NonVisualProperties.Placeholder = &placeholderXML{
			Type:  ph.Type,
			Index: ph.Index,
		}
	}

	if shape.ShapeProperties.Transform != nil {
		result.ShapeProperties.Transform = &transformXML{}
		if shape.ShapeProperties.Transform.Offset != nil {
			result.ShapeProperties.Transform.Offset = &offsetXML{
				X: shape.ShapeProperties.Transform.Offset.X,
				Y: shape.ShapeProperties.Transform.Offset.Y,
			}
		}
		if shape.ShapeProperties.Transform.Extents != nil {
			result.ShapeProperties.Transform.Extents = &extentsXML{
				CX: shape.ShapeProperties.Transform.Extents.CX,
				CY: shape.ShapeProperties.Transform.Extents.CY,
			}
		}
	}

	// Convert lstStyle for font extraction
	if shape.TextBody != nil && shape.TextBody.ListStyle != nil {
		lstStyle := shape.TextBody.ListStyle
		if lstStyle.Lvl1pPr != nil && lstStyle.Lvl1pPr.DefRPr != nil {
			defRPr := lstStyle.Lvl1pPr.DefRPr
			result.TextBody = &textBodyXML{
				ListStyle: &listStyleXML{
					Lvl1pPr: &levelParagraphPropsXML{
						DefRPr: &defaultRunPropsXML{
							Size: defRPr.Size,
						},
					},
				},
			}
			if defRPr.Latin != nil {
				result.TextBody.ListStyle.Lvl1pPr.DefRPr.Latin = &latinFontXML{
					Typeface: defRPr.Latin.Typeface,
				}
			}
			if defRPr.SolidFill != nil {
				result.TextBody.ListStyle.Lvl1pPr.DefRPr.SolidFill = &solidFillXML{}
				if defRPr.SolidFill.SRGBColor != nil {
					result.TextBody.ListStyle.Lvl1pPr.DefRPr.SolidFill.SRGBColor = &srgbColorXML{
						Val: defRPr.SolidFill.SRGBColor.Val,
					}
				}
				if defRPr.SolidFill.SchemeColor != nil {
					result.TextBody.ListStyle.Lvl1pPr.DefRPr.SolidFill.SchemeColor = &schemeColorXML{
						Val: defRPr.SolidFill.SchemeColor.Val,
					}
				}
			}
		}
	}

	return result
}

// tableStyleIndex provides lazy, cached lookup of table style GUIDs declared in
// a template's ppt/tableStyles.xml.  Parse happens at most once per instance.
type tableStyleIndex struct {
	reader *Reader
	once   sync.Once
	styles map[string]string // styleId → styleName
}

// newTableStyleIndex creates an index bound to the given template reader.
// No parsing occurs until lookup is called.
func newTableStyleIndex(r *Reader) *tableStyleIndex {
	return &tableStyleIndex{reader: r}
}

// lookup returns the human-readable style name for a GUID such as
// "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}".  If the GUID is not declared in
// the template (or the XML is missing/malformed), ok is false.
func (idx *tableStyleIndex) lookup(id string) (name string, ok bool) {
	idx.once.Do(idx.parse)
	name, ok = idx.styles[id]
	return
}

// parse reads and unmarshals ppt/tableStyles.xml.  Called at most once via
// sync.Once.  Malformed or missing XML results in an empty index and a debug
// log — never a load failure.
func (idx *tableStyleIndex) parse() {
	idx.styles = make(map[string]string)

	data, err := idx.reader.ReadFile("ppt/tableStyles.xml")
	if err != nil {
		slog.Debug("tableStyleIndex: tableStyles.xml not found, index will be empty",
			"template", idx.reader.Path(), "error", err)
		return
	}

	var lst tblStyleLstXML
	if err := xml.Unmarshal(data, &lst); err != nil {
		slog.Debug("tableStyleIndex: malformed tableStyles.xml, index will be empty",
			"template", idx.reader.Path(), "error", err)
		return
	}

	for _, s := range lst.Styles {
		if s.StyleID != "" {
			idx.styles[s.StyleID] = s.StyleName
		}
	}
}

// tblStyleLstXML is the root element of ppt/tableStyles.xml.
type tblStyleLstXML struct {
	XMLName xml.Name         `xml:"tblStyleLst"`
	Default string           `xml:"def,attr,omitempty"`
	Styles  []tblStyleXML    `xml:"tblStyle"`
}

// tblStyleXML represents a single <a:tblStyle> entry.  We only need the ID and
// name — the full style definition (bands, fills, borders) is opaque to this
// index.
type tblStyleXML struct {
	StyleID   string `xml:"styleId,attr"`
	StyleName string `xml:"styleName,attr"`
}

// XML structure definitions for style extraction (more detailed than layout parsing)

type slideLayoutStyleXML struct {
	XMLName         xml.Name                 `xml:"sldLayout"`
	CommonSlideData commonSlideDataStyleXML  `xml:"cSld"`
}

type commonSlideDataStyleXML struct {
	Name      string              `xml:"name,attr"`
	ShapeTree shapeTreeStyleXML   `xml:"spTree"`
}

type shapeTreeStyleXML struct {
	Shapes []shapeStyleXML `xml:"sp"`
}

type shapeStyleXML struct {
	NonVisualProperties nvSpPrStyleXML       `xml:"nvSpPr"`
	ShapeProperties     spPrStyleXML         `xml:"spPr"`
	TextBody            *txBodyStyleXML      `xml:"txBody"`
}

type nvSpPrStyleXML struct {
	ConnectionNonVisual cNvPrStyleXML   `xml:"cNvPr"`
	Placeholder         *phStyleXML     `xml:"nvPr>ph"`
}

type cNvPrStyleXML struct {
	Name string `xml:"name,attr"`
}

type phStyleXML struct {
	Type  string `xml:"type,attr"`
	Index *int   `xml:"idx,attr"`
}

type spPrStyleXML struct {
	Transform *xfrmStyleXML `xml:"xfrm"`
}

type xfrmStyleXML struct {
	Offset  *offStyleXML `xml:"off"`
	Extents *extStyleXML `xml:"ext"`
}

type offStyleXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type extStyleXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

type txBodyStyleXML struct {
	BodyProperties *bodyPrStyleXML     `xml:"bodyPr"`
	ListStyle      *listStyleStyleXML  `xml:"lstStyle"`
}

type bodyPrStyleXML struct {
	LeftInset   int64 `xml:"lIns,attr"`
	TopInset    int64 `xml:"tIns,attr"`
	RightInset  int64 `xml:"rIns,attr"`
	BottomInset int64 `xml:"bIns,attr"`
}

type listStyleStyleXML struct {
	Lvl1pPr *lvlPPrStyleXML `xml:"lvl1pPr"`
}

type lvlPPrStyleXML struct {
	MarginLeft  int64                `xml:"marL,attr"`
	Indent      int64                `xml:"indent,attr"`
	SpaceBefore *spcStyleXML         `xml:"spcBef"`
	SpaceAfter  *spcStyleXML         `xml:"spcAft"`
	LineSpacing *spcStyleXML         `xml:"lnSpc"`
	BulletNone  *struct{}            `xml:"buNone"`
	BulletChar  *buCharStyleXML      `xml:"buChar"`
	BulletColor *buClrStyleXML       `xml:"buClr"`
	BulletSizePercent *buSzPctStyleXML `xml:"buSzPct"`
	DefRPr      *defRPrStyleXML      `xml:"defRPr"`
}

type spcStyleXML struct {
	SpacePoints  *spcPtsStyleXML  `xml:"spcPts"`
	SpacePercent *spcPctStyleXML  `xml:"spcPct"`
}

type spcPtsStyleXML struct {
	Val int `xml:"val,attr"`
}

type spcPctStyleXML struct {
	Val int `xml:"val,attr"`
}

type buCharStyleXML struct {
	Char string `xml:"char,attr"`
}

type buClrStyleXML struct {
	SRGBColor   *srgbClrStyleXML   `xml:"srgbClr"`
	SchemeColor *schemeClrStyleXML `xml:"schemeClr"`
}

type srgbClrStyleXML struct {
	Val string `xml:"val,attr"`
}

type schemeClrStyleXML struct {
	Val string `xml:"val,attr"`
}

type buSzPctStyleXML struct {
	Val int `xml:"val,attr"`
}

type defRPrStyleXML struct {
	Size      int                  `xml:"sz,attr"`
	Bold      bool                 `xml:"b,attr"`
	Italic    bool                 `xml:"i,attr"`
	Latin     *latinFontStyleXML   `xml:"latin"`
	SolidFill *solidFillStyleXML   `xml:"solidFill"`
}

type latinFontStyleXML struct {
	Typeface string `xml:"typeface,attr"`
}

type solidFillStyleXML struct {
	SRGBColor   *srgbClrStyleXML   `xml:"srgbClr"`
	SchemeColor *schemeClrStyleXML `xml:"schemeClr"`
}
