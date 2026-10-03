package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// PESTEL Native Shapes — 3x2 Grid of Segment Cards
// =============================================================================
//
// Replaces SVG-rendered PESTEL diagrams with native OOXML grouped shapes.
// Each segment is a square-cornered card on the shared neutral surface with a
// bold accent title and bulleted body items (native_surface_style.go). All 6 segments are wrapped in a single
// p:grpSp with identity child transform.
//
// Layout (3 columns x 2 rows):
//
//   ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   │ Political │       │ Economic │       │  Social  │
//   │           │       │           │       │           │
//   └──────────┘       └──────────┘       └──────────┘
//         gap                gap                gap
//   ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   │Technology│       │Environmtl│       │  Legal   │
//   │           │       │           │       │           │
//   └──────────┘       └──────────┘       └──────────┘
//
// Color strategy: neutral cards and one accent (the titles). PESTEL's six
// forces are peers, so nothing stands out (go-slide-creator-w0kj);
// style.colors recolours the segments as accent tints.

// PESTEL EMU constants.
const (
	// pestelGap is the gap between segments in EMU.
	// Same as SWOT gap for visual consistency.
	pestelGap int64 = 73152

	// pestelHeaderFontSize is the segment header font size (hundredths of a point).
	// 1400 = 14pt
	pestelHeaderFontSize int = 1400

	// pestelBodyFontSize is the bullet text font size (hundredths of a point).
	// 1100 = 11pt (slightly smaller than SWOT's 12pt to fit more items in smaller cells)
	pestelBodyFontSize int = 1100

	// pestelHeaderHeightRatio is the fraction of segment height used for the header area.
	pestelHeaderHeightRatio = 0.20

	// pestelBodyInset is the text inset for body text (EMU).
	pestelBodyInset = pptx.ShapeTextInsetEMU // uniform 0.5 cm shape text margin
)

// pestelSegmentColors names the PESTEL segments in render order. The fills
// come from taxonomyPalette (go-slide-creator-w0kj).
// Order: Political, Economic, Social, Technological, Environmental, Legal.
var pestelSegmentColors = [6]struct {
	label string
}{
	{"Political"},
	{"Economic"},
	{"Social"},
	{"Technological"},
	{"Environmental"},
	{"Legal"},
}

// isPESTELDiagram returns true if the diagram spec is a pestel diagram type.
func isPESTELDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "pestel"
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
// Each segment is a card in its tint with a bold header and a bulleted body.
func generatePESTELGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint) string {
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
	for _, p := range panels {
		headerCY = max(headerCY, taxonomyTitleHeight(p.title, cellW, pestelBodyInset, pestelHeaderFontSize, ""))
	}
	if headerCY > cellH {
		headerCY = cellH
	}
	bodyCY := cellH - headerCY

	var children [][]byte
	for i, panel := range panels {
		col := i % numCols
		row := i / numCols

		cellX := bounds.X + int64(col)*(cellW+pestelGap)
		cellY := bounds.Y + int64(row)*(cellH+pestelGap)

		sc := uniformTaxonomyTint(i)
		if i < len(tints) {
			sc = tints[i]
		}

		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape: the card fill under a bold left-aligned title
		headerXML := generatePESTELHeaderXML(
			panel.title, cellX, cellY, cellW, headerCY,
			headerID, sc,
		)
		children = append(children, []byte(headerXML))

		// Body shape: the same fill, top-aligned bulleted text
		bodyXML := generatePESTELBodyXML(
			panel.body, cellX, cellY+headerCY, cellW, bodyCY,
			bodyID, sc,
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

// generatePESTELHeaderXML produces the header shape of a PESTEL segment.
func generatePESTELHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "PESTEL " + title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  pptx.ShapeTextInsets(),
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    nativeHeaderAlign,
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     title,
					Lang:     "en-US",
					FontSize: pestelHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    tint.titleFill(),
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

// generatePESTELBodyXML produces the body shape of a PESTEL segment.
func generatePESTELBodyXML(body string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	paras := panelBulletsParagraphs(body, pestelBodyFontSize)

	// Use the same accent color for bullets but with full strength
	bulletColor := pptx.ResolveColorString(tint.scheme)
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}
	diagramPanelBodyColors(paras, tint.scheme)

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "PESTEL Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: nativeSurfaceGeometry,
		Fill:     tint.fill(),
		Line:     pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     pptx.ShapeTextInsets(),
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
