package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
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
	intensity float64
	factors   []string
}

// porterIntensityColor maps intensity to scheme color + tint.
// High = accent1, Medium = accent3, Low = accent5.
func porterIntensityColor(intensity float64) (scheme string, lumMod, lumOff int) {
	switch {
	case intensity >= 0.67:
		return "accent1", 40000, 60000 // accent1 tint
	case intensity >= 0.34:
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
			value: fmt.Sprintf("%s:%.2f", string(f.forceType), f.intensity),
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
		placeholderIdx:   shapeIdx,
		bounds:           placeholderBounds,
		panels:           panels,
		portersFiveMode:  true,
	})
}

// parsePorterForces extracts force data from the Porter's Five Forces diagram data map.
func parsePorterForces(data map[string]any) []porterForceData {
	forcesRaw, ok := data["forces"]
	if !ok {
		return nil
	}

	forceSlice, ok := forcesRaw.([]any)
	if !ok {
		return nil
	}

	// Default labels for each force type.
	defaultLabels := map[porterForceType]string{
		porterRivalry:    "Competitive Rivalry",
		porterNewEntrant: "Threat of New Entrants",
		porterSubstitute: "Threat of Substitutes",
		porterSupplier:   "Supplier Power",
		porterBuyer:      "Buyer Power",
	}

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

		label := defaultLabels[ft]
		if l, ok := m["label"].(string); ok && l != "" {
			label = l
		}

		intensity := 0.5
		if v, ok := m["intensity"].(float64); ok {
			intensity = v
		}

		var factors []string
		if f, ok := m["factors"]; ok {
			factors = parseSWOTStringList(f) // reuse existing string list parser
		}

		forces = append(forces, porterForceData{
			forceType: ft,
			label:     label,
			intensity: intensity,
			factors:   factors,
		})
	}

	return forces
}

// generatePortersFiveGroupXML produces the complete <p:grpSp> XML for a Porter's
// Five Forces diagram. The panels slice encodes force data via the value field
// (format: "forceType:intensity").
func generatePortersFiveGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
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
		// Parse "forceType:intensity" from value field
		if parts := strings.SplitN(p.value, ":", 2); len(parts) == 2 {
			forces[i].forceType = porterForceType(parts[0])
			_, _ = fmt.Sscanf(parts[1], "%f", &forces[i].intensity)
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
		ft          porterForceType
		x, y, w, h int64
		isCenter    bool
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

	// Generate force box shapes.
	for _, layout := range layouts {
		f, ok := forceMap[layout.ft]
		if !ok {
			// Use defaults if force not provided.
			f = porterForceData{
				forceType: layout.ft,
				label:     porterDefaultLabel(layout.ft),
				intensity: 0.5,
			}
		}

		shapeID := nextID
		shapeIDs[layout.ft] = shapeID
		nextID++

		xml := generatePorterForceBoxXML(f, layout.x, layout.y, layout.w, layout.h, shapeID, layout.isCenter)
		children = append(children, []byte(xml))
	}

	// Generate connectors between center and peripherals.
	// Each connector goes from the peripheral toward the center (arrow points to center).
	connectorPairs := []struct {
		from     porterForceType
		to       porterForceType
		fromSite int // Connection site on 'from' shape
		toSite   int // Connection site on 'to' shape
	}{
		{porterNewEntrant, porterRivalry, 2, 0},  // top bottom → center top
		{porterSubstitute, porterRivalry, 0, 2},  // bottom top → center bottom
		{porterSupplier, porterRivalry, 1, 3},     // left right → center left
		{porterBuyer, porterRivalry, 3, 1},        // right left → center right
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

		connXML := generatePorterConnectorXML(
			nextID, fromID, cp.fromSite, toID, cp.toSite,
			scheme,
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

// generatePorterForceBoxXML produces a single roundRect shape for a force box.
func generatePorterForceBoxXML(f porterForceData, x, y, w, h int64, shapeID uint32, isCenter bool) string {
	scheme, lumMod, lumOff := porterIntensityColor(f.intensity)

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

	// Intensity label paragraph
	intensityText := fmt.Sprintf("%s (%.0f%%)", porterIntensityLabel(f.intensity), f.intensity*100)
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
			Color:    pptx.SchemeFill(scheme),
		}},
	})

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
					Char:  "\u2022",
					Color: pptx.SchemeFill(scheme),
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
		Fill: pptx.SchemeFill(scheme, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.SchemeFill(scheme)},
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
func generatePorterConnectorXML(connID, fromShapeID uint32, fromSite int, toShapeID uint32, toSite int, scheme string) string {
	b, err := pptx.GenerateConnector(pptx.ConnectorOptions{
		ID:       connID,
		Name:     fmt.Sprintf("Porter Connector %d", connID),
		Geometry: pptx.GeomStraightConnector1,
		Bounds:   pptx.RectEmu{X: 0, Y: 0, CX: 1, CY: 1}, // Position computed by PowerPoint from stCxn/endCxn
		Line: pptx.Line{
			Width: porterConnectorWidth,
			Fill:  pptx.SchemeFill(scheme),
		},
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
