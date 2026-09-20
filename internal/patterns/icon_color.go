package patterns

import (
	"encoding/json"
	"strings"
)

// iconMinContrast is the WCAG 2.x non-text (graphical object) contrast
// minimum an icon needs against the surface it is drawn on.
const iconMinContrast = 3.0

// iconFillOn returns the default colour for an icon overlaid on a shape whose
// fill is fillJSON (a shape-grid fill value: scheme name, hex, "none", or the
// {color, alpha, lumMod, lumOff} object form).
//
// Patterns used to default the icon to the cell accent, which is also the
// fill of solid accent cards, so the icon was drawn in the card's own colour
// and vanished. The rule now mirrors the cell text colour:
//
//   - accent, when it reads on the effective fill (>= 3:1) — i.e. light
//     cards keep an on-brand accent icon;
//   - otherwise lt1 (the text colour patterns use on accent fills) when it
//     clears 3:1 — lt1 on solid accent / dark fills;
//   - otherwise whichever of lt1 / dk1 contrasts better (readableTextOn),
//     e.g. dk1 on a pale yellow accent.
//
// Without a resolvable theme it falls back to lt1 when the fill is the
// unmodified accent (the solid-card case) and to accent otherwise.
func iconFillOn(ctx ExpandContext, fillJSON json.RawMessage, accent string) string {
	tone, ok := parseFillTone(fillJSON)
	if !ok {
		// Transparent / unpainted cell: the icon sits on the slide background.
		tone = fillTone{Color: "lt1"}
	}
	fallback := accent
	if tone.Color == accent && tone.Alpha == 0 && tone.LumMod == 0 && tone.LumOff == 0 {
		fallback = "lt1"
	}
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return fallback
	}
	if accent != "" {
		if ac, acOK := resolveThemeColor(ctx, accent); acOK && ac.ContrastWith(fill) >= iconMinContrast {
			return accent
		}
	}
	// Patterns paint lt1 text on accent fills; keep the icon consistent with
	// that text whenever lt1 clears the non-text minimum, so a mid-tone accent
	// (e.g. coral) doesn't get a black icon next to white text.
	if light, lok := resolveThemeColor(ctx, "lt1"); lok && light.ContrastWith(fill) >= iconMinContrast {
		return "lt1"
	}
	// Non-text: the icon only has to clear the 3:1 graphical bar, and it should
	// land on the same theme ink the card's text does rather than on pure black
	// (go-slide-creator-1sel).
	return readableInkOn(ctx, tone, fallback, iconMinContrast)
}

// parseFillTone decodes a shape-grid fill value into a fillTone. ok is false
// for empty, "none", or unparseable fills.
func parseFillTone(raw json.RawMessage) (fillTone, bool) {
	if len(raw) == 0 {
		return fillTone{}, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "none") {
			return fillTone{}, false
		}
		return fillTone{Color: s}, true
	}
	var obj struct {
		Color  string  `json:"color"`
		Alpha  float64 `json:"alpha"`
		LumMod int     `json:"lumMod"`
		LumOff int     `json:"lumOff"`
		Tint   int     `json:"tint"`
		Shade  int     `json:"shade"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fillTone{}, false
	}
	c := strings.TrimSpace(obj.Color)
	if c == "" || strings.EqualFold(c, "none") {
		return fillTone{}, false
	}
	return fillTone{Color: c, Alpha: obj.Alpha, LumMod: obj.LumMod, LumOff: obj.LumOff, Tint: obj.Tint, Shade: obj.Shade}, true
}
