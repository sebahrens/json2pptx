package template

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// placeholderFillStops retains a layout placeholder's own fill separately
// from the slide canvas. Keep scheme references, not resolved hex values, so
// contrast preflight can apply a deck's theme_override later.
func placeholderFillStops(shape *shapeXML, override map[string]string) []types.PlaceholderFillStop {
	if shape == nil {
		return nil
	}
	sp := &shape.ShapeProperties
	if fill := sp.SolidFill; fill != nil {
		ref := placeholderFillRef(fill.SRGBColor, fill.SchemeColor, override)
		if ref != "" {
			return []types.PlaceholderFillStop{{Ref: ref, Mods: colorModifiersFromSolidFill(fill)}}
		}
	}
	if sp.GradientFill == nil {
		return nil
	}
	stops := make([]types.PlaceholderFillStop, 0, len(sp.GradientFill.Stops))
	for _, stop := range sp.GradientFill.Stops {
		ref := placeholderFillRef(stop.SRGBColor, stop.SchemeColor, override)
		stops = append(stops, types.PlaceholderFillStop{
			Position: stop.Position,
			Ref:      ref,
			Mods:     parseColorModifiers(stop.InnerXML),
		})
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].Position < stops[j].Position })
	return stops
}

func placeholderFillRef(srgb *srgbColorXML, scheme *schemeColorXML, override map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return "#" + strings.ToUpper(srgb.Val)
	}
	if scheme != nil && scheme.Val != "" {
		return applyColorMapOverride(scheme.Val, override)
	}
	return ""
}
