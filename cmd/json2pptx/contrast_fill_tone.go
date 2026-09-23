package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// effectiveShapeFillColor returns the colour a viewer actually sees for a
// shape_grid fill: like extractShapeFillColor, but an object-form fill that
// carries lumMod / lumOff / alpha modifiers (the tints patterns paint behind
// highlighted rows and callouts) is composed into its effective "#RRGGBB"
// against the visible slide canvas. Judging a light tint by its untinted base accent made
// the contrast preflight predict an auto-replacement of perfectly readable
// dark text.
func effectiveShapeFillColor(raw json.RawMessage, themeColors []types.ThemeColor, slideBackground ...string) string {
	base := extractShapeFillColor(raw)
	if base == "" {
		return ""
	}
	// tint / shade count as much as lumMod / lumOff: timeline-horizontal's
	// gradient chains are built from them, and ignoring them judged a
	// near-white bar by its untinted accent (go-slide-creator-5qotm).
	var obj struct {
		Alpha  float64 `json:"alpha"`
		LumMod int     `json:"lumMod"`
		LumOff int     `json:"lumOff"`
		Tint   int     `json:"tint"`
		Shade  int     `json:"shade"`
	}
	if json.Unmarshal(raw, &obj) != nil ||
		(obj.Alpha == 0 && obj.LumMod == 0 && obj.LumOff == 0 && obj.Tint == 0 && obj.Shade == 0) {
		return base
	}
	c, ok := themeHex(base, themeColors)
	if !ok {
		return base
	}
	alpha := obj.Alpha
	if alpha > 1 {
		alpha /= 100
	}
	bgHex := "lt1"
	if len(slideBackground) > 0 {
		bgHex = slideBackground[0]
	}
	if alpha > 0 && alpha < 1 && len(slideBackground) > 0 && bgHex == "" {
		return "" // The visible photo canvas cannot be reduced to one colour.
	}
	bg, bgOK := themeHex(bgHex, themeColors)
	if !bgOK {
		if alpha > 0 && alpha < 1 && len(slideBackground) > 0 {
			return ""
		}
		bg = svggen.Color{R: 255, G: 255, B: 255, A: 1}
	}
	return patterns.EffectiveColorMods(c, patterns.ColorMods{
		LumMod: obj.LumMod, LumOff: obj.LumOff,
		Tint: obj.Tint, Shade: obj.Shade, Alpha: alpha,
	}, bg).Hex()
}

var themeHexAliases = map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2"}

// themeHex resolves a scheme name or hex literal against the theme colours.
func themeHex(name string, themeColors []types.ThemeColor) (svggen.Color, bool) {
	name = strings.TrimSpace(name)
	if alias, ok := themeHexAliases[name]; ok {
		name = alias
	}
	for _, tc := range themeColors {
		if tc.Name == name {
			c, err := svggen.ParseColor(tc.RGB)
			return c, err == nil
		}
	}
	c, err := svggen.ParseColor(name)
	return c, err == nil
}
