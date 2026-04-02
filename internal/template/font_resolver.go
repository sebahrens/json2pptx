// Package template provides PPTX template analysis functions.
package template

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// MasterFontResolver resolves font properties from slide masters.
// This is used when a layout's placeholder has no explicit font definitions.
type MasterFontResolver struct {
	reader *Reader
	cache  map[string]*MasterFontStyles // masterPath -> font styles
	theme  *types.ThemeInfo             // Theme for resolving font references
}

// MasterFontStyles contains font styles from a slide master's txStyles.
type MasterFontStyles struct {
	TitleStyle *FontStyle            // p:titleStyle
	BodyStyle  map[int]*FontStyle    // p:bodyStyle lvlNpPr (0-8)
	OtherStyle map[int]*FontStyle    // p:otherStyle lvlNpPr (0-8)
}

// FontStyle represents resolved font properties.
type FontStyle struct {
	FontFamily string // Resolved font family name
	FontSize   int    // Font size in hundredths of a point (e.g., 1400 = 14pt)
	FontColor  string // Font color as hex string (e.g., "#000000")
}

// NewMasterFontResolver creates a resolver for font properties.
func NewMasterFontResolver(reader *Reader, theme *types.ThemeInfo) *MasterFontResolver {
	return &MasterFontResolver{
		reader: reader,
		cache:  make(map[string]*MasterFontStyles),
		theme:  theme,
	}
}

// GetMasterFontsForLayout retrieves font styles from the slide master
// associated with the given layout. Results are cached to avoid re-parsing.
func (r *MasterFontResolver) GetMasterFontsForLayout(layoutID string) *MasterFontStyles {
	// Find the master for this layout by reading the layout's relationships
	layoutRelsPath := fmt.Sprintf("ppt/slideLayouts/_rels/%s.xml.rels", layoutID)
	relsData, err := r.reader.ReadFile(layoutRelsPath)
	if err != nil {
		slog.Debug("master font resolution failed: layout rels file not found",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		slog.Debug("master font resolution failed: layout rels XML parse error",
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
			masterPath = ResolveRelativePath("ppt/slideLayouts", rel.Target)
			break
		}
	}

	if masterPath == "" {
		slog.Debug("master font resolution failed: no slideMaster relationship",
			slog.String("layout_id", layoutID),
		)
		return nil
	}

	// Check cache
	if styles, ok := r.cache[masterPath]; ok {
		return styles
	}

	// Load and parse the master
	masterData, err := r.reader.ReadFile(masterPath)
	if err != nil {
		slog.Debug("master font resolution failed: master file not found",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	styles := r.parseMasterFontStyles(masterData)
	r.cache[masterPath] = styles
	return styles
}

// parseMasterFontStyles extracts font styles from a slide master.
func (r *MasterFontResolver) parseMasterFontStyles(masterData []byte) *MasterFontStyles {
	var master masterFontXML
	if err := xml.Unmarshal(masterData, &master); err != nil {
		slog.Debug("failed to parse master font styles",
			slog.String("error", err.Error()),
		)
		return nil
	}

	styles := &MasterFontStyles{
		BodyStyle:  make(map[int]*FontStyle),
		OtherStyle: make(map[int]*FontStyle),
	}

	// Parse title style (level 1 only)
	if master.TxStyles.TitleStyle.Lvl1pPr.DefRPr != nil {
		styles.TitleStyle = r.parseDefaultRunProps(master.TxStyles.TitleStyle.Lvl1pPr.DefRPr)
	}

	// Parse body style (all levels)
	r.parseLevelStyles(&master.TxStyles.BodyStyle, styles.BodyStyle)

	// Parse other style (all levels)
	r.parseLevelStyles(&master.TxStyles.OtherStyle, styles.OtherStyle)

	return styles
}

// parseLevelStyles extracts font styles from level paragraph properties.
func (r *MasterFontResolver) parseLevelStyles(styleXML *textStyleXML, styles map[int]*FontStyle) {
	levelProps := []*levelParagraphPropsXML{
		styleXML.Lvl1pPr,
		styleXML.Lvl2pPr,
		styleXML.Lvl3pPr,
		styleXML.Lvl4pPr,
		styleXML.Lvl5pPr,
		styleXML.Lvl6pPr,
		styleXML.Lvl7pPr,
		styleXML.Lvl8pPr,
		styleXML.Lvl9pPr,
	}

	for level, props := range levelProps {
		if props != nil && props.DefRPr != nil {
			styles[level] = r.parseDefaultRunProps(props.DefRPr)
		}
	}
}

// parseDefaultRunProps extracts font style from default run properties.
func (r *MasterFontResolver) parseDefaultRunProps(defRPr *defaultRunPropsXML) *FontStyle {
	style := &FontStyle{}

	// Extract font size (sz attribute, in hundredths of a point)
	if defRPr.Size > 0 {
		style.FontSize = defRPr.Size
	}

	// Extract font family from latin typeface
	if defRPr.Latin != nil && defRPr.Latin.Typeface != "" {
		style.FontFamily = r.resolveFontReference(defRPr.Latin.Typeface)
	}

	// Extract font color
	style.FontColor = r.extractColor(defRPr)

	return style
}

// resolveFontReference resolves theme font references like +mj-lt and +mn-lt.
func (r *MasterFontResolver) resolveFontReference(typeface string) string {
	if r.theme == nil {
		return typeface
	}

	switch typeface {
	case "+mj-lt", "+mj-ea", "+mj-cs":
		// Major font = title font
		return r.theme.TitleFont
	case "+mn-lt", "+mn-ea", "+mn-cs":
		// Minor font = body font
		return r.theme.BodyFont
	default:
		// Explicit font name
		return typeface
	}
}

// extractColor extracts color from default run properties.
func (r *MasterFontResolver) extractColor(defRPr *defaultRunPropsXML) string {
	if defRPr.SolidFill == nil {
		return ""
	}

	// Try sRGB color first
	if defRPr.SolidFill.SRGBColor != nil && defRPr.SolidFill.SRGBColor.Val != "" {
		return normalizeColorHex(defRPr.SolidFill.SRGBColor.Val)
	}

	// Try scheme color
	if defRPr.SolidFill.SchemeColor != nil && defRPr.SolidFill.SchemeColor.Val != "" {
		return r.resolveSchemeColor(defRPr.SolidFill.SchemeColor.Val)
	}

	return ""
}

// resolveSchemeColor resolves a scheme color reference to a hex value.
func (r *MasterFontResolver) resolveSchemeColor(schemeName string) string {
	if r.theme == nil {
		return ""
	}

	// Map scheme color names to theme color names
	colorMap := map[string]string{
		"tx1":     "dk1",
		"tx2":     "dk2",
		"bg1":     "lt1",
		"bg2":     "lt2",
		"dk1":     "dk1",
		"dk2":     "dk2",
		"lt1":     "lt1",
		"lt2":     "lt2",
		"accent1": "accent1",
		"accent2": "accent2",
		"accent3": "accent3",
		"accent4": "accent4",
		"accent5": "accent5",
		"accent6": "accent6",
		"hlink":   "hlink",
		"folHlink": "folHlink",
	}

	themeName, ok := colorMap[schemeName]
	if !ok {
		return ""
	}

	for _, color := range r.theme.Colors {
		if color.Name == themeName {
			return color.RGB
		}
	}

	return ""
}

// normalizeColorHex ensures color values are in #RRGGBB format.
func normalizeColorHex(color string) string {
	color = strings.TrimSpace(color)
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	return strings.ToUpper(color)
}

// ResolvePlaceholderFonts determines font properties for a placeholder,
// checking shape txBody/lstStyle, then falling back to master styles.
func (r *MasterFontResolver) ResolvePlaceholderFonts(
	shape *shapeXML,
	placeholderType types.PlaceholderType,
	masterStyles *MasterFontStyles,
) *FontStyle {
	// First, try to extract from the shape's own txBody/lstStyle
	if fonts := r.extractShapeFonts(shape); fonts != nil {
		return fonts
	}

	// Fall back to master styles based on placeholder type
	if masterStyles == nil {
		return nil
	}

	switch placeholderType {
	case types.PlaceholderTitle:
		return masterStyles.TitleStyle
	case types.PlaceholderSubtitle:
		// Subtitles typically use body style but at a different level
		// Try level 0 body style first (same as body content)
		if style, ok := masterStyles.BodyStyle[0]; ok {
			return style
		}
	case types.PlaceholderBody, types.PlaceholderContent:
		// Use level 0 body style as default
		if style, ok := masterStyles.BodyStyle[0]; ok {
			return style
		}
	default:
		// Use other style for non-content placeholders
		if style, ok := masterStyles.OtherStyle[0]; ok {
			return style
		}
	}

	return nil
}

// extractShapeFonts attempts to extract fonts from a shape's lstStyle.
func (r *MasterFontResolver) extractShapeFonts(shape *shapeXML) *FontStyle {
	if shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return nil
	}

	lstStyle := shape.TextBody.ListStyle

	// Check lvl1pPr for default fonts (most common)
	if lstStyle.Lvl1pPr != nil && lstStyle.Lvl1pPr.DefRPr != nil {
		return r.parseDefaultRunProps(lstStyle.Lvl1pPr.DefRPr)
	}

	return nil
}

// XML structure definitions for parsing slide master font styles

type masterFontXML struct {
	XMLName  xml.Name     `xml:"sldMaster"`
	TxStyles txStylesXML  `xml:"txStyles"`
}

type txStylesXML struct {
	TitleStyle textStyleXML `xml:"titleStyle"`
	BodyStyle  textStyleXML `xml:"bodyStyle"`
	OtherStyle textStyleXML `xml:"otherStyle"`
}

type textStyleXML struct {
	Lvl1pPr *levelParagraphPropsXML `xml:"lvl1pPr"`
	Lvl2pPr *levelParagraphPropsXML `xml:"lvl2pPr"`
	Lvl3pPr *levelParagraphPropsXML `xml:"lvl3pPr"`
	Lvl4pPr *levelParagraphPropsXML `xml:"lvl4pPr"`
	Lvl5pPr *levelParagraphPropsXML `xml:"lvl5pPr"`
	Lvl6pPr *levelParagraphPropsXML `xml:"lvl6pPr"`
	Lvl7pPr *levelParagraphPropsXML `xml:"lvl7pPr"`
	Lvl8pPr *levelParagraphPropsXML `xml:"lvl8pPr"`
	Lvl9pPr *levelParagraphPropsXML `xml:"lvl9pPr"`
}

// XML types levelParagraphPropsXML, defaultRunPropsXML, latinFontXML,
// solidFillXML, srgbColorXML, schemeColorXML are defined in layouts.go
// and shared between layouts and font resolution.
