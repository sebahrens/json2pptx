package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// =============================================================================
// PESTEL Native Shapes — 3x2 Grid of roundRect Segments
// =============================================================================
//
// Replaces SVG-rendered PESTEL diagrams with native OOXML grouped shapes.
// Each segment is a roundRect with a scheme-colored tint fill, a bold header
// paragraph, and bulleted body items. All 6 segments are wrapped in a single
// p:grpSp with identity child transform.
//
// Layout (3 columns x 2 rows):
//
//   ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   │ Political │       │ Economic │       │  Social  │
//   │ (accent1) │       │ (accent2) │       │ (accent3) │
//   └──────────┘       └──────────┘       └──────────┘
//         gap                gap                gap
//   ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   │Technology│       │Environmtl│       │  Legal   │
//   │ (accent4) │       │ (accent5) │       │ (accent6) │
//   └──────────┘       └──────────┘       └──────────┘
//
// Color strategy: each segment uses a different accent scheme color (accent1–6)
// with lumMod/lumOff tints so the fill is a light pastel. Text is dk1.

// PESTEL EMU constants.
const (
	// pestelGap is the gap between segments in EMU.
	// Same as SWOT gap for visual consistency.
	pestelGap int64 = 73152

	// pestelCornerRadius is the roundRect adjustment value.
	pestelCornerRadius int64 = 8000

	// pestelHeaderFontSize is the segment header font size (hundredths of a point).
	// 1400 = 14pt
	pestelHeaderFontSize int = 1400

	// pestelBodyFontSize is the bullet text font size (hundredths of a point).
	// 1100 = 11pt (slightly smaller than SWOT's 12pt to fit more items in smaller cells)
	pestelBodyFontSize int = 1100

	// pestelHeaderHeightRatio is the fraction of segment height used for the header area.
	pestelHeaderHeightRatio = 0.20

	// pestelBodyInset is the text inset for body text (EMU).
	pestelBodyInset int64 = 91440 // ~0.1"
)

// pestelSegmentColors defines the accent scheme color for each PESTEL segment.
// Order: Political, Economic, Social, Technological, Environmental, Legal.
var pestelSegmentColors = [6]struct {
	label  string
	scheme string
	lumMod int
	lumOff int
}{
	{"Political", "accent1", 20000, 80000},
	{"Economic", "accent2", 20000, 80000},
	{"Social", "accent3", 20000, 80000},
	{"Technological", "accent4", 20000, 80000},
	{"Environmental", "accent5", 20000, 80000},
	{"Legal", "accent6", 20000, 80000},
}

// isPESTELDiagram returns true if the diagram spec is a pestel diagram type.
func isPESTELDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "pestel"
}

// processPESTELNativeShapes parses PESTEL data from a DiagramSpec and registers
// a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processPESTELNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("pestel native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	// Warn if themeOverride is set — scheme colors won't reflect overrides.
	if ctx.themeOverride != nil {
		slog.Warn("pestel native shapes: themeOverride is set but scheme color refs in PESTEL shapes will not reflect overrides",
			"slide", slideNum)
	}

	// Parse segments from DiagramSpec.Data.
	// Supports two formats:
	//   1. "segments" array: [{"name": "Political", "items": ["Trade policies", ...]}]
	//   2. Individual keys: {"political": ["Trade policies", ...], "economic": [...]}
	panels := parsePESTELSegments(diagramSpec.Data)

	if len(panels) == 0 {
		slog.Warn("pestel native shapes: no segments parsed", "slide", slideNum)
		return
	}

	// Get placeholder bounds from the shape being replaced.
	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native pestel shapes: registered",
		"slide", slideNum,
		"segments", len(panels),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		pestelMode:     true,
	})
}

// parsePESTELSegments extracts segment data from the PESTEL diagram data map.
// Returns up to 6 nativePanelData entries.
func parsePESTELSegments(data map[string]any) []nativePanelData {
	// Try structured "segments" array first (also "factors" alias)
	if segments, ok := data["segments"]; ok {
		return parsePESTELSegmentsArray(segments)
	}
	if factors, ok := data["factors"]; ok {
		return parsePESTELSegmentsArray(factors)
	}

	// Fallback: individual PESTEL category keys
	categoryKeys := [6]struct {
		key   string
		label string
	}{
		{"political", "Political"},
		{"economic", "Economic"},
		{"social", "Social"},
		{"technological", "Technological"},
		{"environmental", "Environmental"},
		{"legal", "Legal"},
	}

	var panels []nativePanelData
	for _, cat := range categoryKeys {
		items := parseSWOTStringList(data[cat.key]) // reuse existing string list parser
		if len(items) == 0 {
			continue
		}
		bulletLines := make([]string, len(items))
		for j, item := range items {
			bulletLines[j] = "- " + item
		}
		panels = append(panels, nativePanelData{
			title: cat.label,
			body:  strings.Join(bulletLines, "\n"),
		})
	}
	return panels
}

// parsePESTELSegmentsArray parses a "segments" or "factors" array value.
func parsePESTELSegmentsArray(v any) []nativePanelData {
	segSlice, ok := v.([]any)
	if !ok {
		return nil
	}

	var panels []nativePanelData
	for _, segItem := range segSlice {
		segMap, ok := segItem.(map[string]any)
		if !ok {
			continue
		}

		title := ""
		if name, ok := segMap["name"].(string); ok {
			title = name
		} else if category, ok := segMap["category"].(string); ok {
			title = category
		}

		body := ""
		if items, ok := segMap["items"]; ok {
			if itemSlice := parseSWOTStringList(items); len(itemSlice) > 0 {
				bulletLines := make([]string, len(itemSlice))
				for j, item := range itemSlice {
					bulletLines[j] = "- " + item
				}
				body = strings.Join(bulletLines, "\n")
			}
		}

		panels = append(panels, nativePanelData{
			title: title,
			body:  body,
		})
	}
	return panels
}

// generatePESTELGroupXML produces the complete <p:grpSp> XML for a 3x2 PESTEL grid.
// Each segment is a roundRect with a tinted scheme fill, bold header, and bulleted body.
func generatePESTELGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	n := len(panels)
	if n == 0 {
		slog.Warn("generatePESTELGroupXML: no panels provided")
		return ""
	}

	// Layout: 3 columns x 2 rows (or fewer if < 6 segments)
	numCols := 3
	numRows := (n + numCols - 1) / numCols // ceil division

	totalWidth := bounds.Width
	totalHeight := bounds.Height

	cellW := (totalWidth - int64(numCols-1)*pestelGap) / int64(numCols)
	cellH := (totalHeight - int64(numRows-1)*pestelGap) / int64(numRows)

	// Header height within each cell
	headerCY := int64(float64(cellH) * pestelHeaderHeightRatio)
	bodyCY := cellH - headerCY

	var children [][]byte
	for i, panel := range panels {
		col := i % numCols
		row := i / numCols

		cellX := bounds.X + int64(col)*(cellW+pestelGap)
		cellY := bounds.Y + int64(row)*(cellH+pestelGap)

		// Pick color: use pestelSegmentColors if within range, otherwise cycle accents
		sc := pestelSegmentColors[i%len(pestelSegmentColors)]

		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape: roundRect with scheme fill, centered bold text
		headerXML := generatePESTELHeaderXML(
			panel.title, cellX, cellY, cellW, headerCY,
			headerID, sc.scheme, sc.lumMod, sc.lumOff,
		)
		children = append(children, []byte(headerXML))

		// Body shape: roundRect with same scheme fill, top-aligned bulleted text
		bodyXML := generatePESTELBodyXML(
			panel.body, cellX, cellY+headerCY, cellW, bodyCY,
			bodyID, sc.scheme, sc.lumMod, sc.lumOff,
		)
		children = append(children, []byte(bodyXML))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "PESTEL Analysis",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generatePESTELGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePESTELHeaderXML produces a roundRect header shape for a PESTEL segment.
func generatePESTELHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "PESTEL " + title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: pestelCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{pestelBodyInset, 0, pestelBodyInset, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     title,
					Lang:     "en-US",
					FontSize: pestelHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generatePESTELHeaderXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generatePESTELBodyXML produces a roundRect body shape for a PESTEL segment.
func generatePESTELBodyXML(body string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	paras := panelBulletsParagraphs(body, pestelBodyFontSize)

	// Use the same accent color for bullets but with full strength
	bulletColor := pptx.SchemeFill(schemeColor)
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "PESTEL Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: pestelCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{pestelBodyInset, pestelBodyInset, pestelBodyInset, pestelBodyInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generatePESTELBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}
