package template

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Inherited placeholder text colour (go-slide-creator-j4364)
//
// A layout that leaves its placeholder colour to the slide master's txStyles
// produced an empty PlaceholderInfo.FontColor, so the contrast preflight —
// which pairs FontColor with the slide's own background — skipped the
// placeholder entirely. The render-time pass
// (internal/generator/inherited_text_contrast.go) resolves the same colour and
// swaps it, so those slides got a contrast_autofixed at generate time and
// silence at validate time.
//
// The resolution has to go through the layout's <p:clrMapOvr>, not just the
// theme: modern-template's Section Divider maps tx1 -> lt1, so a master colour
// of tx1 renders LIGHT there. Resolving it against the theme alone would
// predict the exact opposite of what renders.

var (
	overrideClrMappingRe = regexp.MustCompile(`(?s)<a:overrideClrMapping\s+([^/>]*)/?>`)
	clrMapAttrRe         = regexp.MustCompile(`([a-zA-Z0-9]+)\s*=\s*"([^"]*)"`)
)

// parseLayoutColorMapOverride returns a layout's clrMapOvr mapping — the
// scheme-slot renaming the layout applies on top of the master's map. Returns
// nil when the layout inherits the master mapping, which is the common case.
//
// It mirrors internal/generator's parser of the same element; the two read the
// same bytes and must agree, which is what the preflight/render parity is
// about.
func parseLayoutColorMapOverride(layoutXML []byte) map[string]string {
	m := overrideClrMappingRe.FindSubmatch(layoutXML)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for _, kv := range clrMapAttrRe.FindAllStringSubmatch(string(m[1]), -1) {
		out[kv[1]] = kv[2]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// applyColorMapOverride maps a scheme colour name through a layout's override.
// Names the override does not mention pass through unchanged.
func applyColorMapOverride(scheme string, override map[string]string) string {
	if mapped, ok := override[scheme]; ok && mapped != "" {
		return mapped
	}
	return scheme
}

// inheritedPlaceholderColor is the colour a placeholder renders at when its
// layout declares none: the master style's colour, mapped through the layout's
// override.
//
// A literal "#RRGGBB" from the master is returned as-is — a colour map renames
// scheme SLOTS and has nothing to say about a hard-coded value. A scheme name
// is returned as a scheme name, mapped: the contrast preflight accepts either
// form and resolves scheme names against the same theme the renderer uses, so
// resolving to hex here would duplicate that and could disagree with it.
func inheritedPlaceholderColor(masterColor string, override map[string]string) string {
	masterColor = strings.TrimSpace(masterColor)
	if masterColor == "" || strings.HasPrefix(masterColor, "#") {
		return masterColor
	}
	return applyColorMapOverride(masterColor, override)
}

// inheritedTitleColor is the colour a title placeholder renders at when it
// declares none: the layout's own lstStyle level 1 if it states one, else the
// master's titleStyle, mapped through the layout's colour map.
func inheritedTitleColor(shape *shapeXML, masterFonts *MasterFontStyles, clrMapOvr map[string]string) string {
	if c := layoutLevel1Color(shape); c != "" {
		return inheritedPlaceholderColor(c, clrMapOvr)
	}
	if masterFonts != nil && masterFonts.TitleStyle != nil {
		return inheritedPlaceholderColor(masterFonts.TitleStyle.FontColor, clrMapOvr)
	}
	return ""
}

// inheritedBodyColor is inheritedTitleColor for a body / content placeholder,
// reading the master's bodyStyle level 1.
func inheritedBodyColor(shape *shapeXML, masterFonts *MasterFontStyles, clrMapOvr map[string]string) string {
	if c := layoutLevel1Color(shape); c != "" {
		return inheritedPlaceholderColor(c, clrMapOvr)
	}
	if masterFonts != nil {
		if st := masterFonts.BodyStyle[0]; st != nil {
			return inheritedPlaceholderColor(st.FontColor, clrMapOvr)
		}
	}
	return ""
}

func inheritedTitleColorMods(shape *shapeXML, masterFonts *MasterFontStyles) types.BackgroundColorModifiers {
	if layoutLevel1Color(shape) != "" {
		return layoutLevel1ColorMods(shape)
	}
	if masterFonts != nil && masterFonts.TitleStyle != nil {
		return masterFonts.TitleStyle.ColorMods
	}
	return types.BackgroundColorModifiers{}
}

func inheritedBodyColorMods(shape *shapeXML, masterFonts *MasterFontStyles) types.BackgroundColorModifiers {
	if layoutLevel1Color(shape) != "" {
		return layoutLevel1ColorMods(shape)
	}
	if masterFonts != nil {
		if style := masterFonts.BodyStyle[0]; style != nil {
			return style.ColorMods
		}
	}
	return types.BackgroundColorModifiers{}
}

func layoutLevel1ColorMods(shape *shapeXML) types.BackgroundColorModifiers {
	if shape == nil || shape.TextBody == nil || shape.TextBody.ListStyle == nil || shape.TextBody.ListStyle.Lvl1pPr == nil || shape.TextBody.ListStyle.Lvl1pPr.DefRPr == nil {
		return types.BackgroundColorModifiers{}
	}
	return colorModifiersFromSolidFill(shape.TextBody.ListStyle.Lvl1pPr.DefRPr.SolidFill)
}

// layoutLevel1Color reads a colour the layout placeholder states in its own
// lstStyle level 1, which overrides the master for that placeholder.
func layoutLevel1Color(shape *shapeXML) string {
	if shape == nil || shape.TextBody == nil || shape.TextBody.ListStyle == nil {
		return ""
	}
	lvl := shape.TextBody.ListStyle.Lvl1pPr
	if lvl == nil || lvl.DefRPr == nil || lvl.DefRPr.SolidFill == nil {
		return ""
	}
	fill := lvl.DefRPr.SolidFill
	if fill.SRGBColor != nil && fill.SRGBColor.Val != "" {
		return normalizeColorHex(fill.SRGBColor.Val)
	}
	if fill.SchemeColor != nil && fill.SchemeColor.Val != "" {
		return fill.SchemeColor.Val
	}
	return ""
}
