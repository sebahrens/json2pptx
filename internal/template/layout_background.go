package template

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

var (
	layoutBackgroundSrgb   = regexp.MustCompile(`<p:bg\b[^>]*>\s*<p:bgPr\b[^>]*>\s*<a:solidFill\b[^>]*>\s*<a:srgbClr\s+val="([0-9A-Fa-f]{6})"`)
	layoutBackgroundScheme = regexp.MustCompile(`<p:bg\b[^>]*>\s*<p:bgPr\b[^>]*>\s*<a:solidFill\b[^>]*>\s*<a:schemeClr\s+val="([^"]+)"`)
	layoutBackgroundFill   = regexp.MustCompile(`(?s)<p:bg\b[^>]*>\s*<p:bgPr\b[^>]*>\s*<a:solidFill\b[^>]*>(.*?)</a:solidFill>`)
	layoutBackgroundMod    = regexp.MustCompile(`<a:(lumMod|lumOff|tint|shade|alpha)\s+val="(\d+)"`)
)

// ResolveLayoutBackgroundHex resolves a layout or master solid background
// against the theme. A layout's clrMapOvr is applied when that layout XML is
// passed. Unknown, gradient, and image backgrounds return an empty string.
func ResolveLayoutBackgroundHex(layoutXML []byte, colors []types.ThemeColor) string {
	return ResolveBackgroundRefHexWithMods(ResolveLayoutBackgroundRef(layoutXML), ResolveLayoutBackgroundModifiers(layoutXML), colors)
}

// ResolveLayoutBackgroundModifiers reads only the background's solid fill,
// never similarly named transforms on text or shapes later in the layout.
func ResolveLayoutBackgroundModifiers(layoutXML []byte) types.BackgroundColorModifiers {
	fill := layoutBackgroundFill.FindSubmatch(layoutXML)
	if len(fill) != 2 {
		return types.BackgroundColorModifiers{}
	}
	return parseColorModifiers(fill[1])
}

func colorModifiersFromSolidFill(fill *solidFillXML) types.BackgroundColorModifiers {
	if fill == nil {
		return types.BackgroundColorModifiers{}
	}
	return parseColorModifiers(fill.InnerXML)
}

func parseColorModifiers(xml []byte) types.BackgroundColorModifiers {
	var mods types.BackgroundColorModifiers
	for _, match := range layoutBackgroundMod.FindAllSubmatch(xml, -1) {
		value, err := strconv.Atoi(string(match[2]))
		if err != nil {
			continue
		}
		switch string(match[1]) {
		case "lumMod":
			mods.LumMod, mods.HasLumMod = value, true
		case "lumOff":
			mods.LumOff = value
		case "tint":
			mods.Tint = value
		case "shade":
			mods.Shade = value
		case "alpha":
			mods.Alpha, mods.HasAlpha = value, true
		}
	}
	return mods
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

// ResolveBackgroundRefHexWithMods resolves the visible background after OOXML
// luminance, tint, shade, and alpha transforms. Keeping the ref and modifiers
// separate lets theme_override re-resolve both at preflight time.
func ResolveBackgroundRefHexWithMods(ref string, mods types.BackgroundColorModifiers, colors []types.ThemeColor) string {
	baseHex := ResolveBackgroundRefHex(ref, colors)
	if baseHex == "" {
		return ""
	}
	base, err := svggen.ParseColor(baseHex)
	if err != nil {
		return ""
	}
	background := svggen.MustParseColor("#FFFFFF")
	if hex := ResolveBackgroundRefHex("lt1", colors); hex != "" {
		if color, parseErr := svggen.ParseColor(hex); parseErr == nil {
			background = color
		}
	}
	if mods.HasLumMod && mods.LumMod == 0 && mods.LumOff == 0 {
		base = svggen.MustParseColor("#000000")
	}
	if mods.HasAlpha && mods.Alpha == 0 {
		return strings.ToUpper(background.Hex())
	}
	alpha := 1.0
	if mods.HasAlpha {
		alpha = float64(mods.Alpha) / 100000
	}
	return strings.ToUpper(patterns.EffectiveColorMods(base, patterns.ColorMods{
		LumMod: mods.LumMod, LumOff: mods.LumOff, Tint: mods.Tint, Shade: mods.Shade, Alpha: alpha,
	}, background).Hex())
}
