package template

import "strings"

// XML namespace constants used by synthesis.go for generating layout XML.
const (
	nsDrawingML      = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPresentationML = "http://schemas.openxmlformats.org/presentationml/2006/main"
	nsRelationships  = "http://schemas.openxmlformats.org/package/2006/relationships"

	relTypeSlideMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"
)

// stripHashPrefix removes a leading '#' from a hex color string.
func stripHashPrefix(color string) string {
	if len(color) > 0 && color[0] == '#' {
		return color[1:]
	}
	return color
}

// xmlEscapeAttr escapes a string for use in an XML attribute value.
func xmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// xmlEscapeText escapes a string for use as XML text content.
func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
