package generator

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/icons"
)

// panelIconBandRatio is the fraction of a columns card's height reserved for the
// icon band above the header. Applied only when at least one panel in the group
// carries an icon, so icon-free panels keep their original (golden) geometry.
const panelIconBandRatio = 0.24

// panelColumnsBands computes the vertical bands of a columns panel card. When the
// card group has icons, an icon band is reserved at the top and the header/body
// are sized from the remaining height. With no icons, iconBandCY is 0 and the
// header/body geometry is identical to the pre-icon layout (preserves goldens).
func panelColumnsBands(totalHeight int64, hasIcons bool) (iconBandCY, headerCY, gapCY, bodyCY int64) {
	if hasIcons {
		iconBandCY = int64(float64(totalHeight) * panelIconBandRatio)
	}
	remaining := totalHeight - iconBandCY
	headerCY = int64(float64(remaining) * panelHeaderHeightRatio)
	gapCY = int64(float64(remaining) * panelGapHeightRatio)
	bodyCY = remaining - headerCY - gapCY
	return iconBandCY, headerCY, gapCY, bodyCY
}

// panelColumnsIconRect returns the square bounds for a columns card icon, centered
// horizontally within the panel and vertically within the icon band.
func panelColumnsIconRect(panelX, panelY, panelWidth, iconBandCY int64) pptx.RectEmu {
	side := iconBandCY * 9 / 10 // 90% of the band height
	if maxW := panelWidth * 6 / 10; side > maxW {
		side = maxW
	}
	if side < 0 {
		side = 0
	}
	return pptx.RectEmu{
		X:  panelX + (panelWidth-side)/2,
		Y:  panelY + (iconBandCY-side)/2,
		CX: side,
		CY: side,
	}
}

// panelsHaveIcons reports whether any panel in the slice carries a resolved icon.
func panelsHaveIcons(panels []nativePanelData) bool {
	for i := range panels {
		if len(panels[i].iconSVG) > 0 {
			return true
		}
	}
	return false
}

// panelRowIconBandRatio is the fraction of a rows card's width reserved as a
// left-hand icon column. Applied only when the group has icons.
const panelRowIconBandRatio = 0.12

// panelRowsIconBandCX returns the left icon-column width for a rows layout, or 0
// when there are no icons (preserving the original header/body geometry).
func panelRowsIconBandCX(totalWidth int64, hasIcons bool) int64 {
	if !hasIcons {
		return 0
	}
	return int64(float64(totalWidth) * panelRowIconBandRatio)
}

// panelRowsIconRect returns the square icon bounds for a rows card, centered
// within the left icon column and vertically within the row.
func panelRowsIconRect(x, rowY, iconBandCX, rowHeight int64) pptx.RectEmu {
	side := rowHeight * 7 / 10
	if maxW := iconBandCX * 8 / 10; side > maxW {
		side = maxW
	}
	if side < 0 {
		side = 0
	}
	return pptx.RectEmu{
		X:  x + (iconBandCX-side)/2,
		Y:  rowY + (rowHeight-side)/2,
		CX: side,
		CY: side,
	}
}

// statCardIconBandRatio is the fraction of a stat card's height reserved at the
// top for an icon. Applied per-card only when that card has an icon.
const statCardIconBandRatio = 0.30

// statCardIconBandCY returns the top icon-band height for a stat card, or 0 when
// the card has no icon.
func statCardIconBandCY(cardH int64, hasIcon bool) int64 {
	if !hasIcon {
		return 0
	}
	return int64(float64(cardH) * statCardIconBandRatio)
}

// stylishPanelIconDefaultFill is the fallback icon color for stylish_panels,
// whose icons sit on a dark accent band; white keeps currentColor icons legible.
const stylishPanelIconDefaultFill = "#FFFFFF"

// stylishPanelsIconRect returns the square icon bounds centered within a stylish
// panel's top accent band.
func stylishPanelsIconRect(panelX, panelY, panelWidth, accentCY int64) pptx.RectEmu {
	side := accentCY * 6 / 10
	if maxW := panelWidth * 4 / 10; side > maxW {
		side = maxW
	}
	if side < 0 {
		side = 0
	}
	return pptx.RectEmu{
		X:  panelX + (panelWidth-side)/2,
		Y:  panelY + (accentCY-side)/2,
		CX: side,
		CY: side,
	}
}

// panelIconDefaultFill returns the fallback icon fill for a layout mode, applied
// only when the author did not request an explicit fill.
func panelIconDefaultFill(layoutMode string) string {
	if layoutMode == "stylish_panels" {
		return stylishPanelIconDefaultFill
	}
	return ""
}

// statCardIconRect returns the square icon bounds centered at the top of a stat
// card within its reserved icon band.
func statCardIconRect(cardX, cardY, cardW, iconBandCY int64) pptx.RectEmu {
	side := iconBandCY * 9 / 10
	if maxW := cardW * 4 / 10; side > maxW {
		side = maxW
	}
	if side < 0 {
		side = 0
	}
	return pptx.RectEmu{
		X:  cardX + (cardW-side)/2,
		Y:  cardY + (iconBandCY-side)/2,
		CX: side,
		CY: side,
	}
}

// resolvePanelIcon resolves a panel's "icon" field (from DiagramSpec.Data) into
// raw SVG markup for native OOXML embedding. It accepts the same forms as other
// layouts' icon model:
//
//   - string shorthand: bundled icon name ("rocket"), inline "<svg>…</svg>", or
//     a "data:image/svg+xml" URI.
//   - object: {name|svg_data|fill|alt|...}.
//
// File-path and URL sources are resolved earlier (at the input layer, where the
// base directory and network access live), so they arrive here pre-resolved as
// svg_data; an unresolved path/url returns nil with a reason for the caller to
// log. Returns (nil, "", "") when there is no icon.
//
// defaultFill is the recolor applied when the author did not request an explicit
// fill. Recoloring only rewrites currentColor (see ApplyIconFill), so SVGs that
// carry their own explicit colors are unaffected. It lets a layout whose icons
// sit on a dark band (stylish_panels) pick a legible default without overriding
// author intent.
func resolvePanelIcon(field any, defaultFill string) (svg []byte, alt, skipReason string) {
	switch v := field.(type) {
	case nil:
		return nil, "", ""
	case string:
		return resolvePanelIconString(v, defaultFill)
	case map[string]any:
		return resolvePanelIconObject(v, defaultFill)
	default:
		return nil, "", fmt.Sprintf("unsupported icon type %T", field)
	}
}

func resolvePanelIconObject(m map[string]any, defaultFill string) (svg []byte, alt, skipReason string) {
	alt, _ = m["alt"].(string)
	fill, _ := m["fill"].(string)
	if fill == "" {
		fill = defaultFill
	}

	if sd, _ := m["svg_data"].(string); strings.TrimSpace(sd) != "" {
		b := []byte(sd)
		if fill != "" {
			b = ApplyIconFill(b, fill)
		}
		return b, alt, ""
	}
	if name, _ := m["name"].(string); strings.TrimSpace(name) != "" {
		b, reason := lookupBundledIcon(name, fill)
		return b, alt, reason
	}
	if p, _ := m["path"].(string); strings.TrimSpace(p) != "" {
		return nil, alt, "icon path not pre-resolved (resolve at input layer)"
	}
	if u, _ := m["url"].(string); strings.TrimSpace(u) != "" {
		return nil, alt, "icon url not pre-resolved (resolve at input layer)"
	}
	return nil, alt, ""
}

func resolvePanelIconString(s, fill string) (svg []byte, alt, skipReason string) {
	if strings.TrimSpace(s) == "" {
		return nil, "", ""
	}
	switch svggen.ClassifyIcon(s) {
	case svggen.IconKindInlineSVG:
		b := []byte(s)
		if fill != "" {
			b = ApplyIconFill(b, fill)
		}
		return b, "", ""
	case svggen.IconKindDataURI:
		b, err := decodeSVGDataURI(s)
		if err != nil {
			return nil, "", err.Error()
		}
		if fill != "" {
			b = ApplyIconFill(b, fill)
		}
		return b, "", ""
	case svggen.IconKindURL:
		return nil, "", "icon url not pre-resolved (resolve at input layer)"
	case svggen.IconKindFilePath:
		return nil, "", "icon path not pre-resolved (resolve at input layer)"
	default:
		// Bundled icon name.
		b, reason := lookupBundledIcon(s, fill)
		return b, "", reason
	}
}

func lookupBundledIcon(name, fill string) (svg []byte, skipReason string) {
	b, err := icons.Lookup(name)
	if err != nil {
		return nil, fmt.Sprintf("unknown bundled icon %q", name)
	}
	if fill != "" {
		b = ApplyIconFill(b, fill)
	}
	return b, ""
}

// decodeSVGDataURI decodes a "data:image/svg+xml" URI into raw SVG bytes,
// supporting both base64 and URL/percent-encoded payloads.
func decodeSVGDataURI(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, fmt.Errorf("malformed data URI")
	}
	meta, payload := uri[:comma], uri[comma+1:]
	if strings.Contains(meta, ";base64") {
		return base64.StdEncoding.DecodeString(payload)
	}
	decoded, err := url.QueryUnescape(payload)
	if err != nil {
		return []byte(payload), nil //nolint:nilerr // fall back to raw payload
	}
	return []byte(decoded), nil
}

// ApplyIconFill rewrites currentColor stroke/fill on an SVG's root tag to the
// given color so recoloring behaves identically across every layout that embeds
// icons. fill should be a literal SVG color (hex like "#1F4E79"). Returns the
// input unchanged when there is no <svg> tag or nothing to recolor.
func ApplyIconFill(svgData []byte, fill string) []byte {
	s := string(svgData)
	svgStart := strings.Index(s, "<svg")
	if svgStart < 0 {
		return svgData
	}
	closeIdx := strings.Index(s[svgStart:], ">")
	if closeIdx < 0 {
		return svgData
	}
	tagEnd := svgStart + closeIdx
	tag := s[svgStart:tagEnd]
	modified := false

	if i := strings.Index(tag, ` stroke="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` stroke="%s"`, fill) + tag[i+len(` stroke="currentColor"`):]
		modified = true
	}
	if i := strings.Index(tag, ` fill="currentColor"`); i >= 0 {
		tag = tag[:i] + fmt.Sprintf(` fill="%s"`, fill) + tag[i+len(` fill="currentColor"`):]
		modified = true
	}
	if !modified {
		return svgData
	}
	return []byte(s[:svgStart] + tag + s[tagEnd:])
}
