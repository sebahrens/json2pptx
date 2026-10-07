package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// =============================================================================
// Porter's Five Forces Native Shapes — Central + 4 Peripheral Boxes
// =============================================================================
//
// Replaces SVG-rendered Porter's Five Forces diagrams with native OOXML grouped
// shapes. Rivalry is the one solid accent block in the centre; the four forces
// around it are pentagons whose points aim at it, so the direction of each
// force is carried by its shape and no connector line is drawn
// (go-slide-creator-av25u). Each force has a bold header, an intensity line
// and a bulleted factor list; its fill is a tint step of the accent that
// follows its intensity. All shapes wrapped in a single p:grpSp.
//
// Layout:
//
//                 ┌───────────────┐
//                 │  New Entrants │
//                 └──────╲ ╱──────┘
//   ┌──────────╲    ┌───────────┐    ╱──────────┐
//   │ Suppliers ❯   │  Rivalry  │   ❮  Buyers   │
//   └──────────╱    └───────────┘    ╲──────────┘
//                 ┌──────╱ ╲──────┐
//                 │  Substitutes  │
//                 └───────────────┘
//
// Color strategy: one accent. Rivalry is solid; a scored force takes the
// accent's Lighter 50% (high), 80% (medium) or 90% (low) swatch, an unscored
// force the neutral surface.

// Porter EMU constants.
const (
	// porterCenterWidthRatio is the center box width as a fraction of total width.
	porterCenterWidthRatio = 0.30

	// porterCenterHeightRatio is the center box's share of the height when
	// its text needs less.
	porterCenterHeightRatio = 0.30

	// porterPeripheralWidthRatio is the peripheral box width as a fraction of total width.
	porterPeripheralWidthRatio = 0.28

	// porterPeripheralHeightRatio is a peripheral box's share of the height
	// when its text needs less.
	porterPeripheralHeightRatio = 0.25

	// porterBandWidthRatio is the width of the new-entrant and substitute
	// boxes in the band form. They have their row to themselves, so they run
	// from just inside one side box to just inside the other.
	porterBandWidthRatio = 0.42

	// porterBandHeaderShare is the part of a band its header column takes; the
	// factors are set beside it.
	porterBandHeaderShare = 0.38

	// porterBandMinWidthEMU is the narrowest band whose two columns each hold
	// a line of text (290pt): a narrower region stacks factors under headers.
	porterBandMinWidthEMU int64 = 290 * 12700

	// porterBandGutterEMU is the factor column's left inset in a band.
	porterBandGutterEMU = pptx.ShapeTextInsetEMU / 2

	// porterMinConnectorGapRatio is the smallest gap, as a fraction of total
	// height, left for the connector between Rivalry and the boxes above and
	// below it; porterMinConnectorGapEMU (10pt) is its floor, the room an
	// arrowhead needs to read as one.
	porterMinConnectorGapRatio       = 0.05
	porterMinConnectorGapEMU   int64 = 14 * 12700

	// porterHeaderFontSize is the force header font size (hundredths of a
	// point): 14pt bold, one step above the factors under it; rivalry's is
	// porterCenterHeaderFontSize.
	porterHeaderFontSize       int = 1400
	porterCenterHeaderFontSize int = 1400

	// porterBodyFontSize is the factor text size: the 12pt body step, the
	// smallest size a projected slide carries. It was 10pt
	// (go-slide-creator-6ne1m).
	porterBodyFontSize int = tokens.TypeScaleBodyHPt

	// porterIntensityFontSize is the intensity line's size. It was 9pt, under
	// every readable floor before any shrink.
	porterIntensityFontSize int = tokens.TypeScaleBodyHPt

	// porterHeaderSpaceAfter separates a force's header block from the factors
	// under it (hundredths of a point).
	porterHeaderSpaceAfter int = 300

	// porterMaxBudgetFactors bounds the search for how many factors a force
	// box holds.
	porterMaxBudgetFactors = 8

	// porterTipGapEMU is the space between a force's point and the rivalry
	// block it aims at (4pt): close enough to read as pointing at it.
	porterTipGapEMU int64 = 4 * 12700

	// porterTipMaxDepthEMU caps the depth of a force's point (0.5"), and
	// porterTipMaxAspect its depth against the side it sits on: a point is
	// a blunt arrowhead, not a spike.
	porterTipMaxDepthEMU int64 = 457200
	porterTipMaxAspect         = 0.35

	// porterTipOverlapEMU is how far a point's shape reaches under its force
	// box, so no renderer shows a seam between the two same-coloured shapes.
	porterTipOverlapEMU int64 = 12700
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

// porterNeutralScheme is the surface an unscored force takes: the shared
// neutral card (native_surface_style.go), which asserts nothing. It was the
// template's lt2 under a dk2 outline — an outlined box, the one surface the
// shared style does not draw (go-slide-creator-amtkg).
const porterNeutralScheme = patterns.NeutralSurfaceColor

// porterAccent is the one hue of the diagram: the intensity steps and the
// connectors.
const porterAccent = "accent1"

// porterHighLighter is the swatch of a high-intensity force: Lighter 50%, a
// step deeper than the tonal system's Lighter 60% so that high and low stay
// apart on a dark accent too, and still light enough for dark text.
const porterHighLighter = 50

// porterIntensityColor maps ordinal intensity to three "Lighter N%" swatches
// of the same template accent, the rungs of the patterns' tonal system. An unstated intensity takes the neutral surface:
// colour-coding a force nobody scored asserts a reading the author never made.
func porterIntensityColor(intensity *float64) (scheme string, lumMod, lumOff int) {
	if intensity == nil {
		lumMod, lumOff = patterns.NeutralSurfaceMods(patterns.NeutralTint4)
		return porterNeutralScheme, lumMod, lumOff
	}
	switch {
	case *intensity >= 0.67:
		return porterAccent, (100 - porterHighLighter) * 1000, porterHighLighter * 1000 // Lighter 50%
	case *intensity >= 0.34:
		return porterAccent, (100 - patterns.TonalLighterContent) * 1000, patterns.TonalLighterContent * 1000 // Lighter 80%
	default:
		return porterAccent, (100 - patterns.TonalLighterPale) * 1000, patterns.TonalLighterPale * 1000 // Lighter 90%
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

// porterPanels converts a Porter's spec to nativePanelData for the
// panelShapeInsert system, encoding force metadata into the panel fields.
func porterPanels(diagramSpec *types.DiagramSpec) []nativePanelData {
	forces := parsePorterForces(diagramSpec.Data)
	panels := make([]nativePanelData, 0, len(forces))
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
	return panels
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
func generatePortersFiveGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, env nativeDiagramEnv) string {
	if len(panels) == 0 {
		slog.Warn("generatePortersFiveGroupXML: no panels provided")
		return ""
	}
	layout := layoutPorter(porterForcesFromPanels(panels), bounds, env)

	var children [][]byte
	nextID := shapeIDBase + 1

	shapeBounds := make(map[porterForceType]pptx.RectEmu)

	// Generate force box shapes, in the layout's fixed order.
	for _, box := range layout.boxes {
		shapeBounds[box.force.forceType] = box.rect
		children = append(children, []byte(generatePorterForceBoxXML(box, nextID, env)))
		nextID++
		if box.factorText != nil {
			children = append(children, []byte(generatePorterFactorColumnXML(box, nextID)))
			nextID++
		}
	}

	// Each peripheral force points at rivalry: a triangle on the side that
	// faces the centre, in the force's own fill, makes its box a pentagon.
	for _, box := range layout.boxes {
		if box.force.forceType == porterRivalry {
			continue
		}
		if tip := generatePorterTipXML(box, shapeBounds[porterRivalry], nextID, env.themeColors...); tip != "" {
			children = append(children, []byte(tip))
			nextID++
		}
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
	fill := base
	if lumOff > 0 {
		fill = patterns.EffectiveColorMods(base, taxonomyTint{scheme: scheme, lumMod: lumMod, lumOff: lumOff}.mods(), light)
	}
	if light.ContrastWith(fill) > dark.ContrastWith(fill) {
		return "lt1"
	}
	return "dk2"
}

// porterForcesFromPanels decodes the forces a panel list carries: the label in
// title, "forceType:intensity" in value and one "- factor" line per factor in
// body.
func porterForcesFromPanels(panels []nativePanelData) map[porterForceType]porterForceData {
	forceMap := make(map[porterForceType]porterForceData, len(panels))
	for _, p := range panels {
		f := porterForceData{label: p.title}
		// An empty intensity is a force the payload did not score, and stays nil.
		if parts := strings.SplitN(p.value, ":", 2); len(parts) == 2 {
			f.forceType = porterForceType(parts[0])
			if parts[1] != "" {
				var v float64
				if _, err := fmt.Sscanf(parts[1], "%f", &v); err == nil {
					f.intensity = &v
				}
			}
		}
		for _, line := range strings.Split(p.body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") {
				f.factors = append(f.factors, strings.TrimPrefix(line, "- "))
			}
		}
		forceMap[f.forceType] = f
	}
	return forceMap
}

// porterBoxFill keeps unscored boxes on the neutral surface and lightens
// scored accents with an RGB tint, matching the color used for
// intensity-text contrast checks.
func porterBoxFill(scheme string, lumMod, lumOff int) pptx.Fill {
	return taxonomyTint{scheme: scheme, lumMod: lumMod, lumOff: lumOff}.fill()
}

// generatePorterForceBoxXML produces the square-cornered shape of a force box:
// its fill and the text set in it. A filled surface carries no outline.
func generatePorterForceBoxXML(box porterBox, shapeID uint32, env nativeDiagramEnv) string {
	text := box.text
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Porter %s", box.force.label),
		Bounds:   box.rect,
		Geometry: nativeSurfaceGeometry,
		// Rivalry is the solid accent; a scored force a tint step of it; an
		// unscored force sits on the shared neutral surface.
		Fill: porterForceFill(box.force, env.themeColors),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &text,
	})
	if err != nil {
		slog.Warn("generatePorterForceBoxXML failed", "error", err)
		return ""
	}
	return string(b)
}

// porterForceFill is the fill of a force's box and of its point.
func porterForceFill(f porterForceData, themeColors []types.ThemeColor) pptx.Fill {
	if f.forceType == porterRivalry {
		return nativeSurface{colors: themeColors}.emphasis().fill()
	}
	scheme, lumMod, lumOff := porterForceTint(f, themeColors)
	return porterBoxFill(scheme, lumMod, lumOff)
}

// porterForceTint is porterIntensityColor resolved against the template: the
// intensity steps are tints of the template's primary accent, the colour the
// rivalry block is filled with, so the diagram keeps one hue on a template
// whose primary fill is not accent1.
func porterForceTint(f porterForceData, themeColors []types.ThemeColor) (scheme string, lumMod, lumOff int) {
	scheme, lumMod, lumOff = porterIntensityColor(f.intensity)
	if scheme == porterAccent {
		scheme = nativeSurface{colors: themeColors}.accent()
	}
	return scheme, lumMod, lumOff
}

// porterForceInk names the scheme colour of a force's text: the emphasis ink
// on rivalry's solid accent, the text role elsewhere.
func porterForceInk(f porterForceData, themeColors []types.ThemeColor) string {
	if f.forceType == porterRivalry {
		return nativeSurface{colors: themeColors}.emphasis().ink
	}
	return "dk1"
}

// porterTipRect is the rectangle of the point a peripheral force aims at
// rivalry: on the box side that faces the centre, as long as that side and
// reaching to porterTipGapEMU short of the rivalry block. ok is false when
// the two boxes leave no room for a point.
func porterTipRect(box porterBox, center pptx.RectEmu) (rect pptx.RectEmu, ok bool) {
	r := box.rect
	depth := func(gap, side int64) int64 {
		return min(gap-porterTipGapEMU, porterTipMaxDepthEMU, int64(float64(side)*porterTipMaxAspect))
	}
	const minDepth = 6 * 12700
	switch box.force.forceType {
	case porterNewEntrant:
		d := depth(center.Y-(r.Y+r.CY), r.CX)
		return pptx.RectEmu{X: r.X, Y: r.Y + r.CY - porterTipOverlapEMU, CX: r.CX, CY: d + porterTipOverlapEMU}, d >= minDepth
	case porterSubstitute:
		d := depth(r.Y-(center.Y+center.CY), r.CX)
		return pptx.RectEmu{X: r.X, Y: r.Y - d, CX: r.CX, CY: d + porterTipOverlapEMU}, d >= minDepth
	case porterSupplier:
		d := depth(center.X-(r.X+r.CX), r.CY)
		return pptx.RectEmu{X: r.X + r.CX - porterTipOverlapEMU, Y: r.Y, CX: d + porterTipOverlapEMU, CY: r.CY}, d >= minDepth
	case porterBuyer:
		d := depth(r.X-(center.X+center.CX), r.CY)
		return pptx.RectEmu{X: r.X - d, Y: r.Y, CX: d + porterTipOverlapEMU, CY: r.CY}, d >= minDepth
	}
	return pptx.RectEmu{}, false
}

// generatePorterTipXML produces the point of a peripheral force: a triangle
// in the force's fill whose apex faces the rivalry block. Above and below it
// is the triangle preset (flipped to point down); left and right it is a
// pentagon with no body, which is a triangle that points sideways without a
// rotation.
func generatePorterTipXML(box porterBox, center pptx.RectEmu, shapeID uint32, themeColors ...types.ThemeColor) string {
	rect, ok := porterTipRect(box, center)
	if !ok {
		return ""
	}
	opts := pptx.ShapeOptions{
		ID:     shapeID,
		Name:   fmt.Sprintf("Porter %s Point", box.force.label),
		Bounds: rect,
		Fill:   porterForceFill(box.force, themeColors),
		Line:   pptx.Line{Width: 0, Fill: pptx.NoFill()},
	}
	switch box.force.forceType {
	case porterNewEntrant:
		opts.Geometry, opts.FlipV = pptx.GeomTriangle, true
	case porterSubstitute:
		opts.Geometry = pptx.GeomTriangle
	default:
		// homePlate's point is adj/100000 of its shorter side deep: at the
		// preset maximum the whole shape is the point.
		opts.Geometry = pptx.GeomHomePlate
		opts.Adjustments = []pptx.AdjustValue{{Name: "adj", Value: 100000 * rect.CX / max(min(rect.CX, rect.CY), 1)}}
		opts.FlipH = box.force.forceType == porterBuyer
	}
	b, err := pptx.GenerateShape(opts)
	if err != nil {
		slog.Warn("generatePorterTipXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePorterFactorColumnXML produces the factor column of a band: an
// unfilled text box over the right of the band's own shape.
func generatePorterFactorColumnXML(box porterBox, shapeID uint32) string {
	text := *box.factorText
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     fmt.Sprintf("Porter %s Factors", box.force.label),
		Bounds:   box.factorRect,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text:     &text,
	})
	if err != nil {
		slog.Warn("generatePorterFactorColumnXML failed", "error", err)
		return ""
	}
	return string(b)
}

// porterForceWithDefault is the force drawn at a position: the parsed force,
// or — for a force the payload never mentioned — its own name and nothing
// else: no factors, and no intensity, because there is no assessment to show.
func porterForceWithDefault(forceMap map[porterForceType]porterForceData, ft porterForceType) porterForceData {
	if f, ok := forceMap[ft]; ok {
		return f
	}
	return porterForceData{forceType: ft, label: porterDefaultLabel(ft)}
}

// Laying the cross out from its text (go-slide-creator-6ne1m).
//
// Rivalry sits in the centre with a force on each side and the arrows point
// in. The column of new entrants, rivalry and substitutes is what a landscape
// content area is short of: three boxes of header, intensity and three factors
// at 12pt are 300pt of text and margin, and the shortest shipped content area
// is 273pt high. The boxes used to be written at 10pt and 9pt and shrunk from
// there. They are now sized in this order, and the type is the last thing to
// give:
//
//  1. Width before height. The boxes above and below rivalry have their rows
//     to themselves, so on a region wide enough they become bands — header and
//     intensity on the left, the factors beside them — and take three lines of
//     height instead of five.
//  2. Height from measured text. Every box keeps its default share of the
//     height unless its text needs more; the connector gaps take what is left.
//  3. Padding before type. A column that still does not fit steps the top and
//     bottom text margins down nativeVerticalPadSteps.
//  4. Only then does the column shrink together and the writer store an
//     autofit scale, which generation reports with the budget porterFitBudget
//     measured.

// porterBox is one force box laid out: its rectangle, the text set in the
// box's own shape and — in the band form — the factor column beside it.
type porterBox struct {
	force      porterForceData
	rect       pptx.RectEmu
	text       pptx.TextBody
	factorRect pptx.RectEmu
	factorText *pptx.TextBody
}

// porterLayout is the five boxes in drawing order (rivalry, new entrants,
// substitutes, suppliers, buyers) and whether every one of them holds its text
// at the authored size.
type porterLayout struct {
	boxes []porterBox
	fits  bool
}

// porterBanded reports whether the boxes above and below rivalry are drawn as
// bands: the region is wide enough for two columns and one of them has
// factors to set beside its header.
func porterBanded(forceMap map[porterForceType]porterForceData, totalW int64) bool {
	if int64(float64(totalW)*porterBandWidthRatio) < porterBandMinWidthEMU {
		return false
	}
	return len(porterForceWithDefault(forceMap, porterNewEntrant).factors) > 0 ||
		len(porterForceWithDefault(forceMap, porterSubstitute).factors) > 0
}

// porterBoxText builds the text of a force box of width w at a top / bottom
// margin of pad. Stacked, the factors follow the header in the box's own
// shape. Banded, the box keeps the header column and factors is the column
// beside it, factorW wide.
func porterBoxText(f porterForceData, isCenter, banded bool, w, pad int64, env nativeDiagramEnv) (box pptx.TextBody, factors *pptx.TextBody, factorW int64) {
	fonts := pptx.ThemeFonts{Major: env.fontName, Minor: env.fontName}
	header := porterHeaderParagraphs(f, isCenter, env.themeColors)
	bullets := porterFactorParagraphs(f, env.themeColors)
	box = pptx.TextBody{
		Wrap:       "square",
		Anchor:     "ctr",
		Insets:     [4]int64{pptx.ShapeTextInsetEMU, pad, pptx.ShapeTextInsetEMU, pad},
		AutoFit:    "normAutofit",
		ThemeFonts: fonts,
	}
	if !banded || len(bullets) == 0 {
		if len(bullets) > 0 {
			header[len(header)-1].SpaceAfter = porterHeaderSpaceAfter
		}
		box.Paragraphs = append(header, bullets...)
		return box, nil, 0
	}
	headW := int64(float64(w) * porterBandHeaderShare)
	factorW = w - headW
	box.Paragraphs = header
	box.Insets[2] = factorW
	factors = &pptx.TextBody{
		Wrap:       "square",
		Anchor:     "ctr",
		Insets:     [4]int64{porterBandGutterEMU, pad, pptx.ShapeTextInsetEMU, pad},
		AutoFit:    "normAutofit",
		Paragraphs: bullets,
		ThemeFonts: fonts,
	}
	return box, factors, factorW
}

// layoutPorter lays the five forces out in bounds.
func layoutPorter(forceMap map[porterForceType]porterForceData, bounds types.BoundingBox, env nativeDiagramEnv) porterLayout {
	totalW, totalH := bounds.Width, bounds.Height
	centerW := int64(float64(totalW) * porterCenterWidthRatio)
	sideW := int64(float64(totalW) * porterPeripheralWidthRatio)
	banded := porterBanded(forceMap, totalW)
	topW := sideW
	if banded {
		topW = int64(float64(totalW) * porterBandWidthRatio)
	}
	minGap := max(int64(float64(totalH)*porterMinConnectorGapRatio), porterMinConnectorGapEMU)

	need := func(ft porterForceType, isCenter, band bool, w, pad int64) int64 {
		box, factors, factorW := porterBoxText(porterForceWithDefault(forceMap, ft), isCenter, band, w, pad, env)
		h := nativeTextNeedAtMarginEMU(box, w, totalH)
		if factors != nil {
			h = max(h, nativeTextNeedAtMarginEMU(*factors, factorW, totalH))
		}
		return h
	}
	share := func(ratio float64) int64 { return int64(float64(totalH) * ratio) }

	var center, top, side, pad int64
	fits := false
	for _, pad = range nativeVerticalPadSteps {
		needCenter := need(porterRivalry, true, false, centerW, pad)
		needTop := max(need(porterNewEntrant, false, banded, topW, pad), need(porterSubstitute, false, banded, topW, pad))
		needSide := max(need(porterSupplier, false, false, sideW, pad), need(porterBuyer, false, false, sideW, pad))
		center = max(share(porterCenterHeightRatio), needCenter)
		top = max(share(porterPeripheralHeightRatio), needTop)
		side = min(totalH, max(share(porterPeripheralHeightRatio), needSide))

		// The column gives its spare height — what the default shares hold
		// over the text's need — to the connector gaps before anything else.
		room := totalH - 2*minGap
		if over := 2*top + center - room; over > 0 {
			spare := (center - needCenter) + 2*(top-needTop)
			if spare >= over {
				f := float64(spare-over) / float64(spare)
				center = needCenter + int64(float64(center-needCenter)*f)
				top = needTop + int64(float64(top-needTop)*f)
			}
		}
		if 2*top+center <= room && needSide <= totalH {
			fits = true
			break
		}
	}
	if !fits {
		// Nothing left to give but the type: the column shrinks together and
		// the writer stores the autofit scale each box then needs.
		if column := 2*top + center; column+2*minGap > totalH {
			f := float64(totalH-2*minGap) / float64(column)
			center = int64(float64(center) * f)
			top = int64(float64(top) * f)
		}
	}

	cx := bounds.X + totalW/2
	cy := bounds.Y + totalH/2
	place := func(ft porterForceType, isCenter, band bool, x, y, w, h int64) porterBox {
		f := porterForceWithDefault(forceMap, ft)
		box, factors, factorW := porterBoxText(f, isCenter, band, w, pad, env)
		out := porterBox{force: f, rect: pptx.RectEmu{X: x, Y: y, CX: w, CY: h}, text: box, factorText: factors}
		if factors != nil {
			out.factorRect = pptx.RectEmu{X: x + w - factorW, Y: y, CX: factorW, CY: h}
		}
		return out
	}
	return porterLayout{
		fits: fits,
		boxes: []porterBox{
			place(porterRivalry, true, false, cx-centerW/2, cy-center/2, centerW, center),
			place(porterNewEntrant, false, banded, cx-topW/2, bounds.Y, topW, top),
			place(porterSubstitute, false, banded, cx-topW/2, bounds.Y+totalH-top, topW, top),
			place(porterSupplier, false, false, bounds.X, cy-side/2, sideW, side),
			place(porterBuyer, false, false, bounds.X+totalW-sideW, cy-side/2, sideW, side),
		},
	}
}

// porterFitBudget measures what a force box holds at the authored size in
// bounds: how many one-line factors every force can list at once, and how many
// characters one of those lines takes in the narrowest factor column. It is
// what a TEXT_BELOW_READABLE_MIN on a five-forces diagram tells the author to
// cut to.
func porterFitBudget(forceMap map[porterForceType]porterForceData, bounds types.BoundingBox, env nativeDiagramEnv) nativeFitBudget {
	withFactors := func(n int, text string) map[porterForceType]porterForceData {
		probe := make(map[porterForceType]porterForceData, 5)
		for _, ft := range []porterForceType{porterRivalry, porterNewEntrant, porterSubstitute, porterSupplier, porterBuyer} {
			f := porterForceWithDefault(forceMap, ft)
			f.factors = make([]string, n)
			for i := range f.factors {
				f.factors[i] = text
			}
			probe[ft] = f
		}
		return probe
	}
	items := 0
	for n := 1; n <= porterMaxBudgetFactors; n++ {
		if !layoutPorter(withFactors(n, "Factor"), bounds, env).fits {
			break
		}
		items = n
	}
	// The narrowest factor column decides how long a one-line factor may be.
	chars := 0
	layout := layoutPorter(withFactors(1, "Factor"), bounds, env)
	for _, box := range layout.boxes {
		body, w := box.text, box.rect.CX
		if box.factorText != nil {
			body, w = *box.factorText, box.factorRect.CX
		}
		last := body.Paragraphs[len(body.Paragraphs)-1]
		n := nativeOneLineChars(func(s string) pptx.TextBody {
			p := last
			p.Runs = []pptx.Run{last.Runs[0]}
			p.Runs[0].Text = s
			probe := body
			probe.Paragraphs = []pptx.Paragraph{p}
			return probe
		}, w)
		if chars == 0 || n < chars {
			chars = n
		}
	}
	return nativeFitBudget{
		item: "factor", container: "force", maxItems: items, maxChars: chars,
		sizePt: float64(porterBodyFontSize) / 100,
	}
}

// porterHeaderParagraphs are a force box's header and, when the author stated
// one, its intensity line.
func porterHeaderParagraphs(f porterForceData, isCenter bool, themeColors []types.ThemeColor) []pptx.Paragraph {
	scheme, lumMod, lumOff := porterForceTint(f, themeColors)
	// The intensity line used to be painted in the box's own scheme colour on
	// the box's own tint of it: "Medium (50%)" measured 1.55:1 on the old accent3
	// tile. Pick it against the fill the reader actually sees, the same way the
	// heatmap picks its value colour (go-slide-creator-ceodq).
	intensityColor := porterIntensityTextColor(scheme, lumMod, lumOff, themeColors)

	// Header paragraph — bold, left-aligned like every native card title
	headerSize := porterHeaderFontSize
	ink := porterForceInk(f, themeColors)
	if isCenter {
		headerSize = porterCenterHeaderFontSize
		// On the solid accent every line takes the emphasis ink.
		intensityColor = ink
	}
	paras := []pptx.Paragraph{{
		Align:    nativeHeaderAlign,
		NoBullet: true,
		Runs: []pptx.Run{{
			Text:     f.label,
			Lang:     "en-US",
			FontSize: headerSize,
			Bold:     true,
			Dirty:    true,
			Color:    pptx.SchemeFill(ink),
		}},
	}}

	// Intensity label paragraph — only when the author stated one. Printing
	// "Medium (50%)" for a force nobody scored put a number on the slide that
	// came from a default (go-slide-creator-ceodq).
	if f.intensity != nil {
		intensityText := fmt.Sprintf("%s (%.0f%%)", porterIntensityLabel(*f.intensity), *f.intensity*100)
		paras = append(paras, pptx.Paragraph{
			Align:    nativeHeaderAlign,
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
	return paras
}

// porterFactorParagraphs are a force's factors as a bulleted list: every
// factor the author gave. A list cut at four (three for rivalry) left the
// fifth factor off the slide with nothing said; a list too long for its box
// is now measured and reported instead.
func porterFactorParagraphs(f porterForceData, themeColors []types.ThemeColor) []pptx.Paragraph {
	// Bullets take the text ink: an accent bullet on a tint of the same
	// accent is the least readable mark on the slide.
	ink := porterForceInk(f, themeColors)
	paras := make([]pptx.Paragraph, 0, len(f.factors))
	for _, factor := range f.factors {
		paras = append(paras, pptx.Paragraph{
			Align: "l",
			// The hanging indent every bulleted list of the engine takes. A
			// bullet with no margin sits against its text, and each wrapped
			// line starts under the glyph (go-slide-creator-6ne1m).
			MarginL: pptx.BulletMarginLeft,
			Indent:  pptx.BulletIndent,
			Bullet: &pptx.BulletDef{
				Char: pptx.DefaultBulletChar,
				// Without a buFont the bullet glyph is resolved in the
				// theme font, where U+2022 may be absent — LibreOffice
				// then substitutes, which is the stray "*" marker reported
				// in the bullet colour (go-slide-creator-2zej). Arial is
				// the same buFont pptx.BulletOptions defaults to.
				Font:  pptx.DefaultBulletFont,
				Color: pptx.SchemeFill(ink),
			},
			Runs: []pptx.Run{{
				Text:     factor,
				Lang:     "en-US",
				FontSize: porterBodyFontSize,
				Dirty:    true,
				Color:    pptx.SchemeFill(ink),
			}},
		})
	}
	return paras
}
