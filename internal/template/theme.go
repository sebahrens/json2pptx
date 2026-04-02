package template

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// ParseTheme extracts theme information from a PPTX template.
// Returns a ThemeInfo with colors and fonts, or a default theme if parsing fails.
func ParseTheme(reader *Reader) types.ThemeInfo {
	// Try to find theme files
	themeFiles, err := reader.ListFiles("ppt/theme/theme*.xml")
	if err != nil || len(themeFiles) == 0 {
		// Return default theme if no theme files found
		return getDefaultTheme()
	}

	// Parse the first theme file (typically theme1.xml)
	themeFile := themeFiles[0]
	data, err := reader.ReadFile(themeFile)
	if err != nil {
		return getDefaultTheme()
	}

	var theme pptx.ThemeXML
	if err := xml.Unmarshal(data, &theme); err != nil {
		return getDefaultTheme()
	}

	// Extract theme name
	name := "Default"
	if theme.Name != "" {
		name = theme.Name
	}

	// Extract colors from color scheme
	colors := extractThemeColors(theme.ThemeElements.ColorScheme)

	// Extract fonts
	titleFont := "Calibri"
	bodyFont := "Calibri"
	if theme.ThemeElements.FontScheme.MajorFont.Latin.Typeface != "" {
		titleFont = theme.ThemeElements.FontScheme.MajorFont.Latin.Typeface
	}
	if theme.ThemeElements.FontScheme.MinorFont.Latin.Typeface != "" {
		bodyFont = theme.ThemeElements.FontScheme.MinorFont.Latin.Typeface
	}

	return types.ThemeInfo{
		Name:      name,
		Colors:    colors,
		TitleFont: titleFont,
		BodyFont:  bodyFont,
	}
}

// colorSlotDef maps a color slot name to a function that extracts the color from the scheme.
type colorSlotDef struct {
	name   string
	getter func(pptx.ColorSchemeXML) pptx.ColorDefXML
}

// colorSlots defines all color slots in the theme color scheme in extraction order.
var colorSlots = []colorSlotDef{
	// Dark colors
	{"dk1", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Dark1 }},
	{"dk2", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Dark2 }},
	// Light colors
	{"lt1", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Light1 }},
	{"lt2", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Light2 }},
	// Accent colors
	{"accent1", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent1 }},
	{"accent2", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent2 }},
	{"accent3", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent3 }},
	{"accent4", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent4 }},
	{"accent5", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent5 }},
	{"accent6", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Accent6 }},
	// Hyperlink colors
	{"hlink", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.Hyperlink }},
	{"folHlink", func(cs pptx.ColorSchemeXML) pptx.ColorDefXML { return cs.FollowedHyperlink }},
}

// extractThemeColors extracts color definitions from the color scheme.
func extractThemeColors(colorScheme pptx.ColorSchemeXML) []types.ThemeColor {
	colors := make([]types.ThemeColor, 0, len(colorSlots))

	for _, slot := range colorSlots {
		if rgb := extractRGB(slot.getter(colorScheme)); rgb != "" {
			colors = append(colors, types.ThemeColor{Name: slot.name, RGB: rgb})
		}
	}

	return colors
}

// extractRGB extracts RGB hex value from a color definition.
// Handles both srgbClr and sysClr color types.
func extractRGB(colorDef pptx.ColorDefXML) string {
	// Try sRGB color first
	if colorDef.SRGBColor.Val != "" {
		return normalizeRGB(colorDef.SRGBColor.Val)
	}

	// Try system color with lastClr fallback
	if colorDef.SystemColor.LastClr != "" {
		return normalizeRGB(colorDef.SystemColor.LastClr)
	}

	return ""
}

// normalizeRGB ensures RGB values are in #RRGGBB format.
func normalizeRGB(rgb string) string {
	// Remove any whitespace
	rgb = strings.TrimSpace(rgb)

	// Add # prefix if missing
	if !strings.HasPrefix(rgb, "#") {
		rgb = "#" + rgb
	}

	// Convert to uppercase for consistency
	rgb = strings.ToUpper(rgb)

	// Validate format (should be 7 characters: #RRGGBB)
	if len(rgb) != 7 {
		return ""
	}

	return rgb
}

// getDefaultTheme returns a default theme for fallback.
func getDefaultTheme() types.ThemeInfo {
	return types.ThemeInfo{
		Name:      "Default",
		TitleFont: "Calibri",
		BodyFont:  "Calibri",
		Colors: []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"},
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "dk2", RGB: "#1F497D"},
			{Name: "lt2", RGB: "#EEECE1"},
			{Name: "accent1", RGB: "#4F81BD"},
			{Name: "accent2", RGB: "#C0504D"},
			{Name: "accent3", RGB: "#9BBB59"},
			{Name: "accent4", RGB: "#8064A2"},
			{Name: "accent5", RGB: "#4BACC6"},
			{Name: "accent6", RGB: "#F79646"},
		},
	}
}

// XML structure definitions for parsing PPTX theme files are in internal/pptx package.

// ParseThemeFromZip extracts theme information from a raw zip.Reader.
// This is useful when you already have an open ZIP archive and don't want
// to create a full template.Reader. Returns default theme on parse failure.
func ParseThemeFromZip(zipReader *zip.Reader) types.ThemeInfo {
	// Find theme files
	var themeFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "ppt/theme/theme1.xml" {
			themeFile = f
			break
		}
	}

	if themeFile == nil {
		// Try any theme file
		for _, f := range zipReader.File {
			if strings.HasPrefix(f.Name, "ppt/theme/theme") && strings.HasSuffix(f.Name, ".xml") {
				themeFile = f
				break
			}
		}
	}

	if themeFile == nil {
		return getDefaultTheme()
	}

	// Read theme file
	rc, err := themeFile.Open()
	if err != nil {
		return getDefaultTheme()
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, utils.MaxZipEntrySize))
	if err != nil {
		return getDefaultTheme()
	}

	var theme pptx.ThemeXML
	if err := xml.Unmarshal(data, &theme); err != nil {
		return getDefaultTheme()
	}

	// Extract theme name
	name := "Default"
	if theme.Name != "" {
		name = theme.Name
	}

	// Extract colors from color scheme
	colors := extractThemeColors(theme.ThemeElements.ColorScheme)

	// Extract fonts
	titleFont := "Calibri"
	bodyFont := "Calibri"
	if theme.ThemeElements.FontScheme.MajorFont.Latin.Typeface != "" {
		titleFont = theme.ThemeElements.FontScheme.MajorFont.Latin.Typeface
	}
	if theme.ThemeElements.FontScheme.MinorFont.Latin.Typeface != "" {
		bodyFont = theme.ThemeElements.FontScheme.MinorFont.Latin.Typeface
	}

	return types.ThemeInfo{
		Name:      name,
		Colors:    colors,
		TitleFont: titleFont,
		BodyFont:  bodyFont,
	}
}
