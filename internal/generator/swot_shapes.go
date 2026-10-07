package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// SWOT Native Shapes — 2x2 Grid of Quadrant Cards
// =============================================================================
//
// Replaces SVG-rendered SWOT diagrams with native OOXML grouped shapes.
// Each quadrant is a square-cornered card on the shared neutral surface with
// a bold accent title and bulleted body items (native_surface_style.go). All
// 4 quadrants are wrapped in a single p:grpSp with identity child transform.
//
// Layout:
//
//   ┌─────────────────┐  gap  ┌─────────────────┐
//   │   Strengths     │       │   Weaknesses     │
//   │                 │       │                  │
//   └─────────────────┘       └─────────────────┘
//          gap                        gap
//   ┌─────────────────┐  gap  ┌─────────────────┐
//   │  Opportunities  │       │   Threats        │
//   │                 │       │                  │
//   └─────────────────┘       └─────────────────┘
//
// Color strategy: neutral cards, one accent (the titles). style.colors
// recolours the quadrants as accent tints, in quadrant order.

// SWOT EMU constants.
const (
	// swotGap is the gap between quadrants in EMU.
	// ~0.08" = 7315 EMU — tight gap for 2x2 grid.
	swotGap int64 = 73152

	// swotHeaderFontSize is the quadrant header font size (hundredths of a point).
	// 1600 = 16pt
	swotHeaderFontSize int = 1600

	// swotBodyFontSize is the bullet text font size (hundredths of a point).
	// 1200 = 12pt
	swotBodyFontSize int = 1200

	// swotHeaderHeightRatio is the fraction of quadrant height used for the header area.
	swotHeaderHeightRatio = 0.18

	// swotBodyInset is the text inset for body text (EMU).
	swotBodyInset = pptx.ShapeTextInsetEMU // uniform 0.5 cm shape text margin
)

// swotQuadrantColors defines the accent scheme color for each SWOT quadrant.
// Order: Strengths, Weaknesses, Opportunities, Threats.
var swotQuadrantColors = [4]struct {
	label string
}{
	{"Strengths"}, {"Weaknesses"}, {"Opportunities"}, {"Threats"},
}

// swotDefaultTint is the template-independent default quadrant: the shared
// neutral card with an accent title (native_surface_style.go). The quadrants
// used to take two accent pastels, positive against negative; the polarity is
// already carried by the four labels and the fixed grid, and the second hue
// was the one thing separating a SWOT from every pattern on the deck
// (go-slide-creator-amtkg). swotPolarityColors restores the two-tone look.
func swotDefaultTint(int) taxonomyTint { return nativeSurface{}.cardTint(swotHeaderFontSize) }

// swotTonalTints are the default quadrants resolved against a template: tone
// carries the two readings of the grid (go-slide-creator-w107j). The helpful
// column (Strengths, Opportunities) takes the accent's tint and the harmful
// column (Weaknesses, Threats) the neutral surface; the internal row is the
// deeper step of each, the external row the paler one. One accent, no
// second hue, and four tiles that are no longer interchangeable.
func swotTonalTints(surface nativeSurface) func(int) taxonomyTint {
	tones := [4]nativeTone{
		surface.content(patterns.TonalLighterContent), // Strengths: internal, helpful
		surface.neutral(patterns.NeutralTint8),        // Weaknesses: internal, harmful
		surface.content(patterns.TonalLighterPale),    // Opportunities: external, helpful
		surface.neutral(patterns.NeutralTint4),        // Threats: external, harmful
	}
	return func(i int) taxonomyTint { return tonalTint(tones[i%len(tones)]) }
}

// swotPolarityColors is the style.colors value that restores the two-accent
// polarity tints: Strengths / Opportunities in accent1, Weaknesses / Threats
// in accent2.
var swotPolarityColors = []string{"accent1", "accent2"}

// isSWOTDiagram returns true if the diagram spec is a swot diagram type.
func isSWOTDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "swot"
}

// swotPanels parses the four quadrants into panel data. SWOT data format:
// {"strengths": [...], "weaknesses": [...], "opportunities": [...], "threats": [...]}
func swotPanels(spec *types.DiagramSpec) []nativePanelData {
	quadrantKeys := [4]string{"strengths", "weaknesses", "opportunities", "threats"}
	panels := make([]nativePanelData, 0, len(quadrantKeys))
	for i, key := range quadrantKeys {
		items := parseSWOTStringList(spec.Data[key])
		body := ""
		if len(items) > 0 {
			// Convert items to bullet format ("- " prefix per line)
			bulletLines := make([]string, len(items))
			for j, item := range items {
				bulletLines[j] = "- " + item
			}
			body = strings.Join(bulletLines, "\n")
		}
		panels = append(panels, nativePanelData{
			title: swotQuadrantColors[i].label,
			body:  body,
		})
	}
	return panels
}

// generateSWOTGroupXML produces the complete <p:grpSp> XML for a 2x2 SWOT grid.
// tints are the resolved per-quadrant fills (see taxonomy_palette.go).
// Each quadrant is a card in its tint with a bold header and a bulleted body.
func generateSWOTGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint) string {
	if len(panels) != 4 {
		slog.Warn("generateSWOTGroupXML: expected 4 panels", "got", len(panels))
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// 2x2 grid: each quadrant is (totalWidth - gap) / 2 wide, (totalHeight - gap) / 2 tall
	quadW := (totalWidth - swotGap) / 2
	quadH := (totalHeight - swotGap) / 2

	// Header height within each quadrant: the default band, or the measured
	// title when it needs more.
	headerCY := swotHeaderHeight(panels, quadW, quadH)
	bodyCY := quadH - headerCY

	// Quadrant positions: [top-left, top-right, bottom-left, bottom-right]
	positions := [4]struct{ x, y int64 }{
		{bounds.X, bounds.Y},                                     // Strengths (top-left)
		{bounds.X + quadW + swotGap, bounds.Y},                   // Weaknesses (top-right)
		{bounds.X, bounds.Y + quadH + swotGap},                   // Opportunities (bottom-left)
		{bounds.X + quadW + swotGap, bounds.Y + quadH + swotGap}, // Threats (bottom-right)
	}

	var children [][]byte
	for i, panel := range panels {
		pos := positions[i]
		qc := swotDefaultTint(i)
		if i < len(tints) {
			qc = tints[i]
		}
		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape: the card fill under a bold left-aligned title
		headerXML := generateSWOTHeaderXML(
			panel.title, pos.x, pos.y, quadW, headerCY,
			headerID, qc,
		)
		children = append(children, []byte(headerXML))

		// Body shape: the same fill, top-aligned bulleted text
		bodyXML := generateSWOTBodyXML(
			panel.body, pos.x, pos.y+headerCY, quadW, bodyCY,
			bodyID, qc,
		)
		children = append(children, []byte(bodyXML))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "SWOT Analysis",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateSWOTGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateSWOTHeaderXML produces the header shape of a SWOT quadrant.
func generateSWOTHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	text := swotHeaderText(title, tint)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "SWOT " + title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text:     &text,
	})
	if err != nil {
		slog.Warn("generateSWOTHeaderXML failed", "error", err)
		return ""
	}
	return string(b)
}

// swotHeaderText is a quadrant header's text body: the uniform margin on its
// visible edges and the seam inset against its own body below.
func swotHeaderText(title string, tint taxonomyTint) pptx.TextBody {
	return pptx.TextBody{
		Wrap:    "square",
		Anchor:  "ctr",
		Insets:  nativeCardHeaderInsets(),
		AutoFit: "noAutofit",
		Paragraphs: []pptx.Paragraph{{
			Align:    nativeHeaderAlign,
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     title,
				Lang:     "en-US",
				FontSize: swotHeaderFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    tint.titleFill(),
			}},
		}},
	}
}

// swotBodyText is a quadrant body's text body, shared by the writer and the
// fit measurement (swotGridHeight).
func swotBodyText(body, schemeColor string) pptx.TextBody {
	paras := panelBulletsParagraphs(body, swotBodyFontSize)

	// Bullets take the text ink on a theme-linked tint: an accent bullet on a
	// tint of the same accent is the faintest mark in the quadrant. An
	// authored hex keeps its own text colour (diagramPanelBodyColors).
	bulletColor := pptx.SchemeFill("dk1")
	if !pptx.IsSchemeColor(schemeColor) && schemeColor != "" {
		bulletColor = pptx.ResolveColorString(schemeColor)
	}

	// Override bullet color in parsed paragraphs
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}
	diagramPanelBodyColors(paras, schemeColor)
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "t",
		Insets:     nativeCardBodyInsets(),
		AutoFit:    "normAutofit",
		Paragraphs: paras,
	}
}

// swotHeaderHeight is the header band every quadrant shares: the default
// ratio of the quadrant, or the tallest measured title when that needs more.
// The title is measured as a normAutofit body so the need is the height at
// which it would not shrink; the header itself is written noAutofit.
func swotHeaderHeight(panels []nativePanelData, quadW, quadH int64) int64 {
	headerCY := int64(float64(quadH) * swotHeaderHeightRatio)
	for _, p := range panels {
		probe := swotHeaderText(p.title, taxonomyTint{})
		probe.AutoFit = "normAutofit"
		headerCY = max(headerCY, nativeTextNeedEMU(probe, quadW, quadH))
	}
	return min(headerCY, quadH)
}

// swotGridHeight is the 2x2 grid height at which no quadrant body shrinks,
// measured with the writer's own text bodies (see fitNativeFramework).
func swotGridHeight(panels []nativePanelData, bounds types.BoundingBox) nativeFrameworkHeight {
	quadW := (bounds.Width - swotGap) / 2
	if quadW <= 2*swotBodyInset || len(panels) == 0 {
		return nativeFrameworkHeight{}
	}
	limit := 4 * bounds.Height
	var header, body, ink int64
	for _, p := range panels {
		probe := swotHeaderText(p.title, taxonomyTint{})
		probe.AutoFit = "normAutofit"
		h := nativeTextNeedEMU(probe, quadW, limit)
		b := max(nativeTextNeedEMU(swotBodyText(p.body, ""), quadW, limit),
			2*panelLineHeightEMU(swotBodyFontSize)+nativeCardSeamInsetEMU+swotBodyInset)
		header, body = max(header, h), max(body, b)
		ink += max(0, h-swotBodyInset-nativeCardSeamInsetEMU) + max(0, b-swotBodyInset-nativeCardSeamInsetEMU)
	}
	quadNeed := max(header+body, int64(float64(body)/(1-swotHeaderHeightRatio)))
	rows := int64((len(panels) + 1) / 2)
	return nativeFrameworkHeight{box: rows*quadNeed + (rows-1)*swotGap, ink: ink / 2}
}

// generateSWOTBodyXML produces the body shape of a SWOT quadrant.
func generateSWOTBodyXML(body string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	text := swotBodyText(body, tint.scheme)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "SWOT Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text:     &text,
	})
	if err != nil {
		slog.Warn("generateSWOTBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}

// parseSWOTStringList parses a value as a list of strings.
// Handles both []any (from JSON unmarshal) and []string.
func parseSWOTStringList(v any) []string {
	if v == nil {
		return nil
	}
	switch items := v.(type) {
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return items
	default:
		return nil
	}
}
