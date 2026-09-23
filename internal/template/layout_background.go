package template

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

var (
	layoutBackgroundSrgb   = regexp.MustCompile(`<p:bg\b[^>]*>\s*<p:bgPr\b[^>]*>\s*<a:solidFill\b[^>]*>\s*<a:srgbClr\s+val="([0-9A-Fa-f]{6})"`)
	layoutBackgroundScheme = regexp.MustCompile(`<p:bg\b[^>]*>\s*<p:bgPr\b[^>]*>\s*<a:solidFill\b[^>]*>\s*<a:schemeClr\s+val="([^"]+)"`)
)

// ResolveLayoutBackgroundHex resolves a layout or master solid background
// against the theme. A layout's clrMapOvr is applied when that layout XML is
// passed. Unknown, gradient, and image backgrounds return an empty string.
func ResolveLayoutBackgroundHex(layoutXML []byte, colors []types.ThemeColor) string {
	return ResolveBackgroundRefHex(ResolveLayoutBackgroundRef(layoutXML), colors)
}

// ResolveLayoutBackgroundRef retains a literal hex or a mapped scheme slot so
// callers can re-resolve it after a deck theme_override changes the palette.
func ResolveLayoutBackgroundRef(layoutXML []byte) string {
	if match := layoutBackgroundSrgb.FindSubmatch(layoutXML); len(match) == 2 {
		return "#" + strings.ToUpper(string(match[1]))
	}
	match := layoutBackgroundScheme.FindSubmatch(layoutXML)
	if len(match) != 2 {
		return ""
	}
	return applyColorMapOverride(string(match[1]), parseLayoutColorMapOverride(layoutXML))
}

// ResolveBackgroundRefHex resolves a retained layout/master background source
// against the *effective* deck theme, including theme_override.
func ResolveBackgroundRefHex(ref string, colors []types.ThemeColor) string {
	if strings.HasPrefix(ref, "#") {
		return strings.ToUpper(ref)
	}
	name := ref
	switch ref {
	case "tx1":
		name = "dk1"
	case "tx2":
		name = "dk2"
	case "bg1":
		name = "lt1"
	case "bg2":
		name = "lt2"
	}
	for _, color := range colors {
		if color.Name == name {
			return "#" + strings.ToUpper(strings.TrimPrefix(color.RGB, "#"))
		}
	}
	return ""
}
