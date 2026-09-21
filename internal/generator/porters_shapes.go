package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// =============================================================================
// Porter's Five Forces Native Shapes — Central + 4 Peripheral roundRects
// =============================================================================
//
// Replaces SVG-rendered Porter's Five Forces diagrams with native OOXML grouped
// shapes. Central rivalry roundRect + 4 peripheral force roundRects in cross
// pattern, connected by straightConnector1 with triangle arrowheads. Each box
// has a scheme-colored fill, bold header, intensity indicator text, and factor
// bullet list. All shapes wrapped in a single p:grpSp.
//
// Layout:
//
//                 ┌───────────────┐
//                 │  New Entrants │
//                 │   (accent2)   │
//                 └───────┬───────┘
//                         │ ▼
//   ┌──────────┐    ┌─────┴─────┐    ┌──────────┐
//   │ Suppliers│───▶│  Rivalry  │◀───│  Buyers  │
//   │ (accent4)│    │  (accent1) │    │ (accent5)│
//   └──────────┘    └─────┬─────┘    └──────────┘
//                         │ ▼
//                 ┌───────┴───────┐
//                 │  Substitutes  │
//                 │   (accent3)   │
//                 └───────────────┘
//
// Color strategy: intensity-based accent mapping.
//   High (0.67-1.0): accent1 tint
//   Medium (0.34-0.66): accent3 tint
//   Low (0.0-0.33): accent5 tint

// Porter EMU constants.
const (
	// porterCornerRadius is the roundRect adjustment value.
	porterCornerRadius int64 = 8000

	// porterCenterWidthRatio is the center box width as a fraction of total width.
	porterCenterWidthRatio = 0.30

	// porterCenterHeightRatio is the center box height as a fraction of total height.
	porterCenterHeightRatio = 0.30

	// porterPeripheralWidthRatio is the peripheral box width as a fraction of total width.
	porterPeripheralWidthRatio = 0.28

	// porterPeripheralHeightRatio is the peripheral box height as a fraction of total height.
	porterPeripheralHeightRatio = 0.25

	// porterHeaderFontSize is the force header font size (hundredths of a point).
	// 1200 = 12pt
	porterHeaderFontSize int = 1200

	// porterBodyFontSize is the bullet/factor text font size (hundredths of a point).
	// 1000 = 10pt
	porterBodyFontSize int = 1000

	// porterIntensityFontSize is the intensity label font size (hundredths of a point).
	// 900 = 9pt
	porterIntensityFontSize int = 900

	// porterTextInset is the text inset for all text areas (EMU).
	porterTextInset int64 = 73152 // ~0.08"

	// porterConnectorWidth is the connector line width in EMU.
	// 12700 EMU = 1pt
	porterConnectorWidth int64 = 12700
)

// porterForceType identifies which force position a force occupies.
type porterForceType string

const (
	porterRivalry    porterForceType = "rivalry"
	porterNewEntrant porterForceType = "new_entrants"
	porterSubstitute porterForceType = "substitutes"
	porterSupplier   porterForceType = "suppliers"
	porterBuyer      porterForceType = "buyers"
)

// porterForceData holds parsed data for a single Porter force.
type porterForceData struct {
	forceType porterForceType
	label     string
	// intensity is nil when the payload did not state one. It used to default
	// to 0.5, so every unscored force printed "Medium (50%)" and took the same
	// accent3 tint: a chart that looked like an assessment and was a default
	// (go-slide-creator-ceodq).
	intensity *float64
	factors   []string
}

// clampPorterIntensity holds intensity inside the 0.0-1.0 range the input
// schema documents. Out-of-range values were carried through to the label,
// which printed "High (150%)" and "Low (-20%)" — a number no reader can place
// on a five-forces chart, from a field whose own hint says 0.0-1.0
// (go-slide-creator-umji).
func clampPorterIntensity(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// porterNeutralScheme is the surface an unscored force takes: the template's
// own light neutral, which asserts nothing.
const porterNeutralScheme = "lt2"

// porterIntensityColor maps intensity to scheme color + tint.
// High = accent1, Medium = accent3, Low = accent5. An unstated intensity takes
// the neutral surface: colour-coding a force nobody scored asserts a reading
// the author never made.
func porterIntensityColor(intensity *float64) (scheme string, lumMod, lumOff int) {
	if intensity == nil {
		return porterNeutralScheme, 0, 0
	}
	switch {
	case *intensity >= 0.67:
		return "accent1", 40000, 60000 // accent1 tint
	case *intensity >= 0.34:
		return "accent3", 40000, 60000 // accent3 tint
	default:
		return "accent5", 40000, 60000 // accent5 tint
	}
}

// porterIntensityLabel returns a human-readable intensity label.
func porterIntensityLabel(intensity float64) string {
	switch {
	case intensity >= 0.67:
		return "High"
	case intensity >= 0.34:
		return "Medium"
	default:
		return "Low"
	}
}

// porterDefaultLabel returns a default display label for a force type.
func porterDefaultLabel(ft porterForceType) string {
	switch ft {
	case porterRivalry:
		return "Competitive Rivalry"
	case porterNewEntrant:
		return "Threat of New Entrants"
	case porterSubstitute:
		return "Threat of Substitutes"
	case porterSupplier:
		return "Supplier Power"
	case porterBuyer:
		return "Buyer Power"
	default:
		return string(ft)
	}
}

// isPortersFiveForcesDiagram returns true if the diagram spec is a porters_five_forces type.
func isPortersFiveForcesDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "porters_five_forces"
}

// processPortersFiveForceNativeShapes parses Porter's Five Forces data from a DiagramSpec
// and registers a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processPortersFiveForceNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("porters native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("porters native shapes: themeOverride is set but scheme color refs won't reflect overrides",
			"slide", slideNum)
	}

	forces := parsePorterForces(diagramSpec.Data)
	if len(forces) == 0 {
		slog.Warn("porters native shapes: no forces parsed", "slide", slideNum)
		return
	}

	// Convert to nativePanelData for the panelShapeInsert system.
	// We encode force metadata into the panel fields.
	var panels []nativePanelData
	for _, f := range forces {
		body := ""
		if len(f.factors) > 0 {
			lines := make([]string, len(f.factors))
			for j, factor := range f.factors {
				lines[j] = "- " + factor
			}
			body = strings.Join(lines, "\n")
		}
		panels = append(panels, nativePanelData{
			title: f.label,
			body:  body,
			value: porterPanelValue(f),
		})
	}

	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native porters shapes: registered",
		"slide", slideNum,
		"forces", len(panels),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		altText:         diagramAltText(item),
		placeholderIdx:  shapeIdx,
		bounds:          placeholderBounds,
		panels:          panels,
		portersFiveMode: true,
	})
}

// porterDefaultLabels maps each force type to its default display label.
var porterDefaultLabels = map[porterForceType]string{
	porterRivalry:    "Competitive Rivalry",
	porterNewEntrant: "Threat of New Entrants",
	porterSubstitute: "Threat of Substitutes",
	porterSupplier:   "Supplier Power",
	porterBuyer:      "Buyer Power",
}

// porterForceKeyAliases maps object-keyed force keys (and common synonyms) to a
// canonical porterForceType. Used when the diagram data is supplied as a map of
// force objects keyed by name (e.g. {"rivalry": {...}, "buyer_power": {...}})
// rather than as a "forces" array.
var porterForceKeyAliases = map[string]porterForceType{
	"rivalry":                       porterRivalry,
	"competitive_rivalry":           porterRivalry,
	"new_entrants":                  porterNewEntrant,
	"threat_of_new_entrants":        porterNewEntrant,
	"substitutes":                   porterSubstitute,
	"threat_of_substitutes":         porterSubstitute,
	"buyers":                        porterBuyer,
	"buyer_power":                   porterBuyer,
	"bargaining_power_of_buyers":    porterBuyer,
	"suppliers":                     porterSupplier,
	"supplier_power":                porterSupplier,
	"bargaining_power_of_suppliers": porterSupplier,
}

// parsePorterForces extracts force data from the Porter's Five Forces diagram
// data map. Two input shapes are supported:
//
//   - Array form: data["forces"] is a list of {type, label, intensity, factors}
//     objects.
//   - Object-keyed form: data has top-level keys naming each force (e.g.
//     "rivalry", "new_entrants", "substitutes", "buyer_power", "supplier_power")
//     whose values are {label, intensity, description|factors} objects.
func parsePorterForces(data map[string]any) []porterForceData {
	if forcesRaw, ok := data["forces"]; ok {
		if forceSlice, ok := forcesRaw.([]any); ok {
			return parsePorterForcesArray(forceSlice)
		}
	}
	return parsePorterForcesObjectKeyed(data)
}

// parsePorterForcesArray handles the data["forces"] = []{...} array form.
func parsePorterForcesArray(forceSlice []any) []porterForceData {
	var forces []porterForceData
	for _, item := range forceSlice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		ft := porterForceType("")
		if t, ok := m["type"].(string); ok {
			ft = porterForceType(t)
		}
		if ft == "" {
			continue
		}

		forces = append(forces, porterForceFromMap(ft, m))
	}
	return forces
}

// parsePorterForcesObjectKeyed handles the object-keyed form where each force is
// a top-level key in the data map. Forces are emitted in canonical layout order
// (rivalry, new entrants, substitutes, suppliers, buyers) for deterministic output.
func parsePorterForcesObjectKeyed(data map[string]any) []porterForceData {
	// Collect parsed forces by canonical type so synonyms collapse correctly.
	byType := make(map[porterForceType]porterForceData)
	for key, raw := range data {
		ft, ok := porterForceKeyAliases[strings.ToLower(strings.TrimSpace(key))]
		if !ok {
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, seen := byType[ft]; seen {
			continue // first occurrence wins
		}
		byType[ft] = porterForceFromMap(ft, m)
	}

	// Emit in fixed layout order.
	order := []porterForceType{porterRivalry, porterNewEntrant, porterSubstitute, porterSupplier, porterBuyer}
	var forces []porterForceData
	for _, ft := range order {
		if f, ok := byType[ft]; ok {
			forces = append(forces, f)
		}
	}
	return forces
}

// porterForceFromMap builds a porterForceData from a force object map, applying
// the default label for the force type when none is supplied.
func porterForceFromMap(ft porterForceType, m map[string]any) porterForceData {
	label := porterDefaultLabels[ft]
	if l, ok := m["label"].(string); ok && l != "" {
		label = l
	}

	var intensity *float64
	if v, ok := m["intensity"].(float64); ok {
		clamped := clampPorterIntensity(v)
		intensity = &clamped
	}

	// Prefer an explicit "factors" list; otherwise fall back to a "description"
	// string as a single supporting line (object-keyed decks commonly use this).
	var factors []string
	if f, ok := m["factors"]; ok {
		factors = parseSWOTStringList(f) // reuse existing string list parser
	}
	if len(factors) == 0 {
		if d, ok := m["description"].(string); ok && d != "" {
			factors = []string{d}
		}
	}

	return porterForceData{
		forceType: ft,
		label:     label,
		intensity: intensity,
		factors:   factors,
	}
}

// porterPanelValue encodes a force for the panel round trip. An unstated
// intensity encodes as an empty field rather than a number, so "nobody scored
// this" survives the trip instead of becoming 0.5 on the other side.
func porterPanelValue(f porterForceData) string {
	if f.intensity == nil {
		return string(f.forceType) + ":"
	}
	return fmt.Sprintf("%s:%.2f", string(f.forceType), *f.intensity)
}

// generatePortersFiveGroupXML produces the complete <p:grpSp> XML for a Porter's
// Five Forces diagram. The panels slice encodes force data via the value field
// (format: "forceType:intensity").
func generatePortersFiveGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, themeColors []types.ThemeColor) string {
	if len(panels) == 0 {
		slog.Warn("generatePortersFiveGroupXML: no panels provided")
		return ""
	}

	// Parse force data back from panel encoding.
	forces := make([]porterForceData, len(panels))
	for i, p := range panels {
		forces[i] = porterForceData{
			label: p.title,
		}
		// Parse "forceType:intensity" from value field. An empty intensity is
		// a force the payload did not score, and stays nil.
		if parts := strings.SplitN(p.value, ":", 2); len(parts) == 2 {
			forces[i].forceType = porterForceType(parts[0])
			if parts[1] != "" {
				var v float64
				if _, err := fmt.Sscanf(parts[1], "%f", &v); err == nil {
					forces[i].intensity = &v
				}
			}
		}
		// Parse factors from body
		if p.body != "" {
			for _, line := range strings.Split(p.body, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "- ") {
					forces[i].factors = append(forces[i].factors, strings.TrimPrefix(line, "- "))
				}
			}
		}
	}

	// Compute layout dimensions in EMU.
	totalW := bounds.Width
	totalH := bounds.Height

	centerW := int64(float64(totalW) * porterCenterWidthRatio)
	centerH := int64(float64(totalH) * porterCenterHeightRatio)
	periW := int64(float64(totalW) * porterPeripheralWidthRatio)
	periH := int64(float64(totalH) * porterPeripheralHeightRatio)

	// Center of the diagram.
	cx := bounds.X + totalW/2
	cy := bounds.Y + totalH/2

	// Center box position.
	centerX := cx - centerW/2
	centerY := cy - centerH/2

	// Peripheral box positions (centered on their axis).
	topX := cx - periW/2
	topY := bounds.Y
	bottomX := cx - periW/2
	bottomY := bounds.Y + totalH - periH
	leftX := bounds.X
	leftY := cy - periH/2
	rightX := bounds.X + totalW - periW
	rightY := cy - periH/2

	// Map forces by type for easy lookup.
	forceMap := make(map[porterForceType]porterForceData)
	for _, f := range forces {
		forceMap[f.forceType] = f
	}

	// Fixed rendering order for deterministic output.
	type forceLayout struct {
		ft         porterForceType
		x, y, w, h int64
		isCenter   bool
	}
	layouts := []forceLayout{
		{porterRivalry, centerX, centerY, centerW, centerH, true},
		{porterNewEntrant, topX, topY, periW, periH, false},
		{porterSubstitute, bottomX, bottomY, periW, periH, false},
		{porterSupplier, leftX, leftY, periW, periH, false},
		{porterBuyer, rightX, rightY, periW, periH, false},
	}

	var children [][]byte
	nextID := shapeIDBase + 1

	// Track shape IDs for connector references.
	shapeIDs := make(map[porterForceType]uint32)
	shapeBounds := make(map[porterForceType]pptx.RectEmu)

	// Generate force box shapes.
	for _, layout := range layouts {
		f, ok := forceMap[layout.ft]
		if !ok {
			// A force the payload never mentioned is drawn with its own name
			// and nothing else: no factors, and no intensity, because there is
			// no assessment to show.
			f = porterForceData{
				forceType: layout.ft,
				label:     porterDefaultLabel(layout.ft),
			}
		}

		shapeID := nextID
		shapeIDs[layout.ft] = shapeID
		shapeBounds[layout.ft] = pptx.RectEmu{X: layout.x, Y: layout.y, CX: layout.w, CY: layout.h}
		nextID++

		xml := generatePorterForceBoxXML(f, layout.x, layout.y, layout.w, layout.h, shapeID, layout.isCenter, themeColors)
		children = append(children, []byte(xml))
	}

	// Generate connectors between center and peripherals.
	// Each connector goes from the peripheral toward the center (arrow points to center).
	// Sites are resolved through pptx.ConnectionSiteIndex rather than written
	// as literals. The horizontal pairs used to assume rect site 1 = right and
	// 3 = left, but OOXML lists rect sites counter-clockwise from the top
	// (0 = top, 1 = left, 2 = bottom, 3 = right), so both connectors attached to
	// the FAR side of their box and ran straight through its text into Rivalry.
	// The same literals were corrected for shapegrid in 13e4292; this caller was
	// missed (go-slide-creator-2zej). Naming the sides keeps them in step.
	site := func(sd pptx.ConnectionSide) int {
		return pptx.ConnectionSiteIndex(pptx.GeomRoundRect, sd)
	}
	connectorPairs := []struct {
		from     porterForceType
		to       porterForceType
		fromSite int // Connection site on 'from' shape
		toSite   int // Connection site on 'to' shape
	}{
		// New entrants sit above Rivalry: leave its bottom, enter Rivalry's top.
		{porterNewEntrant, porterRivalry, site(pptx.SideBottom), site(pptx.SideTop)},
		// Substitutes sit below: leave its top, enter Rivalry's bottom.
		{porterSubstitute, porterRivalry, site(pptx.SideTop), site(pptx.SideBottom)},
		// Suppliers sit to the left: leave its right, enter Rivalry's left.
		{porterSupplier, porterRivalry, site(pptx.SideRight), site(pptx.SideLeft)},
		// Buyers sit to the right: leave its left, enter Rivalry's right.
		{porterBuyer, porterRivalry, site(pptx.SideLeft), site(pptx.SideRight)},
	}

	for _, cp := range connectorPairs {
		fromID, hasFrom := shapeIDs[cp.from]
		toID, hasTo := shapeIDs[cp.to]
		if !hasFrom || !hasTo {
			continue
		}

		// Find force for color
		f := forceMap[cp.from]
		scheme, _, _ := porterIntensityColor(f.intensity)
		route := pptx.Route(
			pptx.ShapeOptions{Bounds: shapeBounds[cp.from], Geometry: pptx.GeomRoundRect},
			pptx.ShapeOptions{Bounds: shapeBounds[cp.to], Geometry: pptx.GeomRoundRect},
			false,
		)

		connXML := generatePorterConnectorXML(
			nextID, fromID, cp.fromSite, toID, cp.toSite, route.Bounds,
			route.FlipH, route.FlipV, scheme,
		)
		children = append(children, []byte(connXML))
		nextID++
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Porters Five Forces",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generatePortersFiveGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// porterIntensityTextColor names the scheme colour the intensity line is
// printed in: whichever of the light / dark text roles reads on the box's own
// tinted fill. Without theme colours there is nothing to measure and the
// historical scheme-on-scheme stands.
func porterIntensityTextColor(scheme string, lumMod, lumOff int, themeColors []types.ThemeColor) string {
	base, err := svggen.ParseColor(resolveSchemeColorToHex(scheme, themeColors))
	if err != nil {
		return scheme
	}
	light, lErr := svggen.ParseColor(resolveSchemeColorToHex("lt1", themeColors))
	if lErr != nil {
		light = svggen.Color{R: 255, G: 255, B: 255, A: 1}
	}
	dark, dErr := svggen.ParseColor(resolveSchemeColorToHex("dk2", themeColors))
	if dErr != nil {
		dark = svggen.Color{A: 1}
	}
	fill := patterns.EffectiveColor(base, lumMod, lumOff, 1, light)
	if light.ContrastWith(fill) > dark.ContrastWith(fill) {
		return "lt1"
	}
	return "dk2"
}

// porterBoxFill builds the box fill, omitting the luminance modifiers when
// there are none: pptx.LumMod(0) writes a 0% luminance, which renders black.
func porterBoxFill(scheme string, lumMod, lumOff int) pptx.Fill {
	if lumMod == 0 && lumOff == 0 {
		return pptx.SchemeFill(scheme)
	}
	return pptx.SchemeFill(scheme, pptx.LumMod(lumMod), pptx.LumOff(lumOff))
}

// porterOutlineScheme keeps an unscored box's outline visible: lt2 on white is
// not an edge, so the neutral box is drawn with the structural dark instead.
func porterOutlineScheme(fillScheme string) string {
	if fillScheme == porterNeutralScheme {
		return "dk2"
	}
	return fillScheme
}

// generatePorterForceBoxXML produces a single roundRect shape for a force box.
func generatePorterForceBoxXML(f porterForceData, x, y, w, h int64, shapeID uint32, isCenter bool, themeColors []types.ThemeColor) string {
	scheme, lumMod, lumOff := porterIntensityColor(f.intensity)
	// The intensity line used to be painted in the box's own scheme colour on
	// the box's own tint of it: "Medium (50%)" measured 1.55:1 on the accent3
	// tile. Pick it against the fill the reader actually sees, the same way the
	// heatmap picks its value colour (go-slide-creator-ceodq).
	intensityColor := porterIntensityTextColor(scheme, lumMod, lumOff, themeColors)

	// Build text paragraphs: header + intensity label + factors
	var paras []pptx.Paragraph

	// Header paragraph — bold, centered
	headerSize := porterHeaderFontSize
	if isCenter {
		headerSize = porterHeaderFontSize + 200 // 14pt for center
	}
	paras = append(paras, pptx.Paragraph{
		Align:    "ctr",
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     f.label,
			Lang:     "en-US",
			FontSize: headerSize,
			Bold:     true,
			Dirty:    true,
			Color:    pptx.SchemeFill("dk1"),
		}},
	})

	// Intensity label paragraph — only when the author stated one. Printing
	// "Medium (50%)" for a force nobody scored put a number on the slide that
	// came from a default (go-slide-creator-ceodq).
	if f.intensity != nil {
		intensityText := fmt.Sprintf("%s (%.0f%%)", porterIntensityLabel(*f.intensity), *f.intensity*100)
		paras = append(paras, pptx.Paragraph{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     intensityText,
				Lang:     "en-US",
				FontSize: porterIntensityFontSize,
				Bold:     false,
				Italic:   true,
				Dirty:    true,
				Color:    pptx.SchemeFill(intensityColor),
			}},
		})
	}

	// Factor bullets (if any)
	if len(f.factors) > 0 {
		maxFactors := 4
		if isCenter {
			maxFactors = 3
		}
		shown := f.factors
		if len(shown) > maxFactors {
			shown = shown[:maxFactors]
		}
		for _, factor := range shown {
			paras = append(paras, pptx.Paragraph{
				Align: "l",
				Bullet: &pptx.BulletDef{
					Char: "\u2022",
					// Without a buFont the bullet glyph is resolved in the
					// theme font, where U+2022 may be absent — LibreOffice
					// then substitutes, which is the stray "*" marker reported
					// in the bullet colour (go-slide-creator-2zej). Arial is
					// the same buFont pptx.BulletOptions defaults to.
					Font:  pptx.DefaultBulletFont,
					Color: pptx.SchemeFill(porterOutlineScheme(scheme)),
				},
				Runs: []pptx.Run{{
					Text:     factor,
					Lang:     "en-US",
					FontSize: porterBodyFontSize,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			})
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Porter %s", f.label),
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: w, CY: h},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: porterCornerRadius},
		},
		// LumMod(0) is 0% luminance — black — not "no modifier", so an unscored
		// force's neutral fill has to be emitted without the modifiers at all.
		Fill: porterBoxFill(scheme, lumMod, lumOff),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill(porterOutlineScheme(scheme))},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "ctr",
			Insets:     [4]int64{porterTextInset, porterTextInset, porterTextInset, porterTextInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generatePorterForceBoxXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePorterConnectorXML produces a straightConnector1 between two shapes.
func generatePorterConnectorXML(connID, fromShapeID uint32, fromSite int, toShapeID uint32, toSite int,
	bounds pptx.RectEmu, flipH, flipV bool, scheme string,
) string {
	b, err := pptx.GenerateConnector(pptx.ConnectorOptions{
		ID:       connID,
		Name:     fmt.Sprintf("Porter Connector %d", connID),
		Geometry: pptx.GeomStraightConnector1,
		// PowerPoint does not derive a visible path from stCxn/endCxn when the
		// stored transform is the old 1x1 placeholder. LibreOffice does, which
		// hid the portability bug. Keep the attachment metadata and also persist
		// the resolved path so both applications draw the same connector
		// (go-slide-creator-7ec2s).
		Bounds: bounds,
		Line: pptx.Line{
			Width: porterConnectorWidth,
			Fill:  pptx.SchemeFill(scheme),
		},
		FlipH: flipH,
		FlipV: flipV,
		TailEnd: &pptx.ArrowHead{
			Type: "triangle",
			W:    "med",
			Len:  "med",
		},
		StartConn: &pptx.ConnectionRef{
			ShapeID: fromShapeID,
			SiteIdx: fromSite,
		},
		EndConn: &pptx.ConnectionRef{
			ShapeID: toShapeID,
			SiteIdx: toSite,
		},
	})
	if err != nil {
		slog.Warn("generatePorterConnectorXML failed", "error", err)
		return ""
	}
	return string(b)
}
