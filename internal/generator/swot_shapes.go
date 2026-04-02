package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// SWOT Native Shapes — 2x2 Grid of roundRect Quadrants
// =============================================================================
//
// Replaces SVG-rendered SWOT diagrams with native OOXML grouped shapes.
// Each quadrant is a roundRect with a scheme-colored tint fill, a bold header
// paragraph, and bulleted body items. All 4 quadrants are wrapped in a single
// p:grpSp with identity child transform.
//
// Layout:
//
//   ┌─────────────────┐  gap  ┌─────────────────┐
//   │   Strengths     │       │   Weaknesses     │
//   │   (accent1)     │       │   (accent2)      │
//   └─────────────────┘       └─────────────────┘
//          gap                        gap
//   ┌─────────────────┐  gap  ┌─────────────────┐
//   │  Opportunities  │       │   Threats        │
//   │   (accent3)     │       │   (accent4)      │
//   └─────────────────┘       └─────────────────┘
//
// Color strategy: each quadrant uses a different accent scheme color with
// high lumMod/lumOff tints so the fill is a light pastel. Text is dk1.
// This ensures theme-awareness across all templates.

// SWOT EMU constants.
const (
	// swotGap is the gap between quadrants in EMU.
	// ~0.08" = 7315 EMU — tight gap for 2x2 grid.
	swotGap int64 = 73152

	// swotCornerRadius is the roundRect adjustment value.
	// 16667 = OOXML default ~1/6 of shortest side; we use a smaller
	// fixed value for a subtle rounded look.
	swotCornerRadius int64 = 8000

	// swotHeaderFontSize is the quadrant header font size (hundredths of a point).
	// 1400 = 14pt
	swotHeaderFontSize int = 1400

	// swotBodyFontSize is the bullet text font size (hundredths of a point).
	// 1200 = 12pt
	swotBodyFontSize int = 1200

	// swotHeaderHeightRatio is the fraction of quadrant height used for the header area.
	swotHeaderHeightRatio = 0.18

	// swotBodyInset is the text inset for body text (EMU).
	swotBodyInset int64 = 91440 // ~0.1"
)

// swotQuadrantColors defines the accent scheme color for each SWOT quadrant.
// Order: Strengths, Weaknesses, Opportunities, Threats.
var swotQuadrantColors = [4]struct {
	label  string
	scheme string
	lumMod int
	lumOff int
}{
	{"Strengths", "accent1", 20000, 80000},
	{"Weaknesses", "accent2", 20000, 80000},
	{"Opportunities", "accent3", 20000, 80000},
	{"Threats", "accent4", 20000, 80000},
}

// isSWOTDiagram returns true if the diagram spec is a swot diagram type.
func isSWOTDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "swot"
}

// processSWOTNativeShapes parses SWOT data from a DiagramSpec and registers
// a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processSWOTNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("swot native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	// Warn if themeOverride is set — scheme colors won't reflect overrides.
	if ctx.themeOverride != nil {
		slog.Warn("swot native shapes: themeOverride is set but scheme color refs in SWOT shapes will not reflect overrides",
			"slide", slideNum)
	}

	// Parse the 4 quadrants into panel data.
	// SWOT data format: {"strengths": [...], "weaknesses": [...], "opportunities": [...], "threats": [...]}
	quadrantKeys := [4]string{"strengths", "weaknesses", "opportunities", "threats"}
	var panels []nativePanelData

	for i, key := range quadrantKeys {
		items := parseSWOTStringList(diagramSpec.Data[key])
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

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native swot shapes: registered",
		"slide", slideNum,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		// groupXML is generated during allocatePanelIconRelIDs via generateSWOTGroupXML
		swotMode: true,
	})
}

// generateSWOTGroupXML produces the complete <p:grpSp> XML for a 2x2 SWOT grid.
// Each quadrant is a roundRect with a tinted scheme fill, bold header, and bulleted body.
func generateSWOTGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	if len(panels) != 4 {
		slog.Warn("generateSWOTGroupXML: expected 4 panels", "got", len(panels))
		return ""
	}

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	// 2x2 grid: each quadrant is (totalWidth - gap) / 2 wide, (totalHeight - gap) / 2 tall
	quadW := (totalWidth - swotGap) / 2
	quadH := (totalHeight - swotGap) / 2

	// Header height within each quadrant
	headerCY := int64(float64(quadH) * swotHeaderHeightRatio)
	bodyCY := quadH - headerCY

	// Quadrant positions: [top-left, top-right, bottom-left, bottom-right]
	positions := [4]struct{ x, y int64 }{
		{bounds.X, bounds.Y},                                  // Strengths (top-left)
		{bounds.X + quadW + swotGap, bounds.Y},                // Weaknesses (top-right)
		{bounds.X, bounds.Y + quadH + swotGap},                // Opportunities (bottom-left)
		{bounds.X + quadW + swotGap, bounds.Y + quadH + swotGap}, // Threats (bottom-right)
	}

	var children [][]byte
	for i, panel := range panels {
		pos := positions[i]
		qc := swotQuadrantColors[i]
		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape: roundRect with scheme fill, centered bold text
		headerXML := generateSWOTHeaderXML(
			panel.title, pos.x, pos.y, quadW, headerCY,
			headerID, qc.scheme, qc.lumMod, qc.lumOff,
		)
		children = append(children, []byte(headerXML))

		// Body shape: roundRect with same scheme fill, top-aligned bulleted text
		bodyXML := generateSWOTBodyXML(
			panel.body, pos.x, pos.y+headerCY, quadW, bodyCY,
			bodyID, qc.scheme, qc.lumMod, qc.lumOff,
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

// generateSWOTHeaderXML produces a roundRect header shape for a SWOT quadrant.
func generateSWOTHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "SWOT " + title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: swotCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{swotBodyInset, 0, swotBodyInset, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     title,
					Lang:     "en-US",
					FontSize: swotHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateSWOTHeaderXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateSWOTBodyXML produces a roundRect body shape for a SWOT quadrant.
func generateSWOTBodyXML(body string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	paras := panelBulletsParagraphs(body, swotBodyFontSize)

	// Use the same accent color for bullets but with full strength
	bulletColor := pptx.SchemeFill(schemeColor)

	// Override bullet color in parsed paragraphs
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "SWOT Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: swotCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{swotBodyInset, swotBodyInset, swotBodyInset, swotBodyInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
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
