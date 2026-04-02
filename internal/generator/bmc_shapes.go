package generator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Business Model Canvas Native Shapes — 9-Box Irregular Grid
// =============================================================================
//
// Replaces SVG-rendered Business Model Canvas diagrams with native OOXML
// grouped shapes (p:grpSp) that use scheme color references.
//
// Layout (5 columns, 3 visual rows):
//
//   Row 1 (top 60%):
//   ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
//   │             │ │ Key        │ │             │ │ Customer   │ │             │
//   │ Key         │ │ Activities │ │ Value       │ │ Relations  │ │ Customer   │
//   │ Partners    │ ├────────────┤ │ Proposition │ ├────────────┤ │ Segments   │
//   │ (full)      │ │ Key        │ │ (full)      │ │ Channels   │ │ (full)     │
//   │             │ │ Resources  │ │             │ │            │ │            │
//   └────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘
//
//   Row 2 (bottom 40%):
//   ┌──────────────────────────┐ ┌──────────────────────────┐
//   │      Cost Structure      │ │     Revenue Streams       │
//   │      (2.5 cols wide)     │ │     (2.5 cols wide)       │
//   └──────────────────────────┘ └──────────────────────────┘
//
// Each box is a rect with a scheme-colored tint fill, a bold header section
// at the top, and bulleted body items below. All boxes are wrapped in a single
// p:grpSp with identity child transform.
//
// Color strategy: each section uses a rotating accent scheme color (accent1–6,
// recycling for 7–9) with lumMod/lumOff tints. Text is dk1.

// BMC EMU constants.
const (
	// bmcGap is the gap between boxes in EMU.
	bmcGap int64 = 73152 // ~0.08" — same as SWOT/PESTEL for consistency

	// bmcCornerRadius is the roundRect adjustment value.
	bmcCornerRadius int64 = 8000

	// bmcHeaderFontSize is the box header font size (hundredths of a point).
	// 1200 = 12pt (smaller than SWOT's 14pt because BMC has 9 dense cells)
	bmcHeaderFontSize int = 1200

	// bmcBodyFontSize is the bullet text font size (hundredths of a point).
	// 1000 = 10pt (compact to fit content in small cells)
	bmcBodyFontSize int = 1000

	// bmcHeaderHeightRatio is the fraction of box height used for the header area.
	bmcHeaderHeightRatio = 0.18

	// bmcBodyInset is the text inset for body text (EMU).
	bmcBodyInset int64 = 72000 // ~0.079"

	// bmcTopRowRatio is the fraction of total height for the top row (5-column).
	bmcTopRowRatio = 0.60
)

// bmcSectionKey is an internal key for BMC sections.
type bmcSectionKey string

const (
	bmcKeyPartners      bmcSectionKey = "key_partners"
	bmcKeyActivities    bmcSectionKey = "key_activities"
	bmcKeyResources     bmcSectionKey = "key_resources"
	bmcValueProposition bmcSectionKey = "value_proposition"
	bmcCustRelations    bmcSectionKey = "customer_relationships"
	bmcChannels         bmcSectionKey = "channels"
	bmcCustSegments     bmcSectionKey = "customer_segments"
	bmcCostStructure    bmcSectionKey = "cost_structure"
	bmcRevenueStreams   bmcSectionKey = "revenue_streams"
)

// bmcSectionColors defines the accent scheme color for each BMC section.
// 9 sections rotate through accent1–accent6, then repeat accent1–accent3.
var bmcSectionColors = map[bmcSectionKey]struct {
	scheme string
	lumMod int
	lumOff int
}{
	bmcKeyPartners:      {"accent1", 20000, 80000},
	bmcKeyActivities:    {"accent2", 20000, 80000},
	bmcKeyResources:     {"accent3", 20000, 80000},
	bmcValueProposition: {"accent4", 20000, 80000},
	bmcCustRelations:    {"accent5", 20000, 80000},
	bmcChannels:         {"accent6", 20000, 80000},
	bmcCustSegments:     {"accent1", 30000, 70000}, // slightly different tint to distinguish from key_partners
	bmcCostStructure:    {"accent2", 30000, 70000},
	bmcRevenueStreams:   {"accent3", 30000, 70000},
}

// bmcDefaultTitles maps section keys to display titles.
var bmcDefaultTitles = map[bmcSectionKey]string{
	bmcKeyPartners:      "Key Partners",
	bmcKeyActivities:    "Key Activities",
	bmcKeyResources:     "Key Resources",
	bmcValueProposition: "Value Proposition",
	bmcCustRelations:    "Customer Relationships",
	bmcChannels:         "Channels",
	bmcCustSegments:     "Customer Segments",
	bmcCostStructure:    "Cost Structure",
	bmcRevenueStreams:   "Revenue Streams",
}

// bmcSectionAliases maps various input key formats to canonical section keys.
var bmcSectionAliases = map[string]bmcSectionKey{
	"key_partners":           bmcKeyPartners,
	"keyPartners":            bmcKeyPartners,
	"partners":               bmcKeyPartners,
	"key_activities":         bmcKeyActivities,
	"keyActivities":          bmcKeyActivities,
	"activities":             bmcKeyActivities,
	"key_resources":          bmcKeyResources,
	"keyResources":           bmcKeyResources,
	"resources":              bmcKeyResources,
	"value_proposition":      bmcValueProposition,
	"value_propositions":     bmcValueProposition,
	"valueProposition":       bmcValueProposition,
	"valuePropositions":      bmcValueProposition,
	"value":                  bmcValueProposition,
	"customer_relationships": bmcCustRelations,
	"customerRelationships":  bmcCustRelations,
	"relationships":          bmcCustRelations,
	"channels":               bmcChannels,
	"customer_segments":      bmcCustSegments,
	"customerSegments":       bmcCustSegments,
	"segments":               bmcCustSegments,
	"customers":              bmcCustSegments,
	"cost_structure":         bmcCostStructure,
	"costStructure":          bmcCostStructure,
	"costs":                  bmcCostStructure,
	"revenue_streams":        bmcRevenueStreams,
	"revenueStreams":         bmcRevenueStreams,
	"revenue":                bmcRevenueStreams,
}

// bmcSectionData holds parsed data for a single BMC section.
type bmcSectionData struct {
	key   bmcSectionKey
	title string
	items []string
}

// isBMCDiagram returns true if the diagram spec is a business_model_canvas type.
func isBMCDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "business_model_canvas"
}

// processBMCNativeShapes parses BMC data from a DiagramSpec and registers
// a panelShapeInsert for native OOXML shape generation.
func (ctx *singlePassContext) processBMCNativeShapes(slideNum int, item ContentItem, shapeIdx int) {
	diagramSpec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("bmc native shapes: invalid diagram spec", "slide", slideNum)
		return
	}

	if ctx.themeOverride != nil {
		slog.Warn("bmc native shapes: themeOverride is set but scheme color refs in BMC shapes will not reflect overrides",
			"slide", slideNum)
	}

	// Parse BMC sections from DiagramSpec.Data
	sections := parseBMCSections(diagramSpec.Data)

	// Convert to nativePanelData. We store sections in a fixed canonical order
	// so that generateBMCGroupXML knows which section each panel represents.
	sectionOrder := []bmcSectionKey{
		bmcKeyPartners, bmcKeyActivities, bmcKeyResources,
		bmcValueProposition, bmcCustRelations, bmcChannels,
		bmcCustSegments, bmcCostStructure, bmcRevenueStreams,
	}

	panels := make([]nativePanelData, len(sectionOrder))
	for i, key := range sectionOrder {
		sec := sections[key]
		title := sec.title
		if title == "" {
			title = bmcDefaultTitles[key]
		}
		body := ""
		if len(sec.items) > 0 {
			bulletLines := make([]string, len(sec.items))
			for j, item := range sec.items {
				bulletLines[j] = "- " + item
			}
			body = strings.Join(bulletLines, "\n")
		}
		panels[i] = nativePanelData{
			title: title,
			body:  body,
		}
	}

	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native bmc shapes: registered",
		"slide", slideNum,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		bmcMode:        true,
	})
}

// parseBMCSections parses BMC section data from the diagram data map.
// Supports both flat format ({key_partners: ["items"]}) and nested format
// ({boxes: {key_partners: {items: ["items"]}}}).
func parseBMCSections(data map[string]any) map[bmcSectionKey]bmcSectionData {
	sections := make(map[bmcSectionKey]bmcSectionData)

	// Check for nested "boxes" structure first
	dataSource := data
	if boxes, ok := data["boxes"]; ok {
		if boxesMap, ok := boxes.(map[string]any); ok {
			dataSource = boxesMap
		}
	}

	for key, alias := range bmcSectionAliases {
		rawVal, ok := dataSource[key]
		if !ok {
			continue
		}
		// Don't overwrite if we already parsed this section from a more
		// specific key alias.
		if _, exists := sections[alias]; exists {
			continue
		}
		sec := bmcSectionData{key: alias}
		switch v := rawVal.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					sec.items = append(sec.items, s)
				}
			}
		case []string:
			sec.items = v
		case map[string]any:
			if title, ok := v["title"].(string); ok {
				sec.title = title
			}
			if items, ok := v["items"].([]any); ok {
				for _, item := range items {
					if s, ok := item.(string); ok {
						sec.items = append(sec.items, s)
					}
				}
			}
		case string:
			sec.items = []string{v}
		}
		sections[alias] = sec
	}

	return sections
}

// generateBMCGroupXML produces the complete <p:grpSp> XML for a 9-box BMC grid.
// Panels must be in canonical order: key_partners, key_activities, key_resources,
// value_proposition, customer_relationships, channels, customer_segments,
// cost_structure, revenue_streams.
func generateBMCGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32) string {
	if len(panels) != 9 {
		slog.Warn("generateBMCGroupXML: expected 9 panels", "got", len(panels))
		return ""
	}

	totalW := bounds.Width
	totalH := bounds.Height

	// Split into top row (60%) and bottom row (40%)
	topH := int64(float64(totalH) * bmcTopRowRatio)
	bottomH := totalH - topH

	// 5 equal columns in the top row
	colW := (totalW - 4*bmcGap) / 5

	// Top row: columns 0 and 2 and 4 are full height; columns 1 and 3 are split
	fullH := topH - bmcGap // leave gap before bottom row
	halfH := (fullH - bmcGap) / 2

	// Section order matches panels array
	sectionOrder := []bmcSectionKey{
		bmcKeyPartners, bmcKeyActivities, bmcKeyResources,
		bmcValueProposition, bmcCustRelations, bmcChannels,
		bmcCustSegments, bmcCostStructure, bmcRevenueStreams,
	}

	// Define cell positions (x, y, w, h) for each of the 9 sections
	type cellRect struct {
		x, y, w, h int64
	}

	topY := bounds.Y
	bottomY := bounds.Y + topH

	cells := map[bmcSectionKey]cellRect{
		// Column 0: Key Partners (full height)
		bmcKeyPartners: {bounds.X, topY, colW, fullH},

		// Column 1: Key Activities (top half) + Key Resources (bottom half)
		bmcKeyActivities: {bounds.X + colW + bmcGap, topY, colW, halfH},
		bmcKeyResources:  {bounds.X + colW + bmcGap, topY + halfH + bmcGap, colW, halfH},

		// Column 2: Value Proposition (full height)
		bmcValueProposition: {bounds.X + 2*(colW+bmcGap), topY, colW, fullH},

		// Column 3: Customer Relationships (top half) + Channels (bottom half)
		bmcCustRelations: {bounds.X + 3*(colW+bmcGap), topY, colW, halfH},
		bmcChannels:      {bounds.X + 3*(colW+bmcGap), topY + halfH + bmcGap, colW, halfH},

		// Column 4: Customer Segments (full height)
		bmcCustSegments: {bounds.X + 4*(colW+bmcGap), topY, colW, fullH},

		// Bottom row: Cost Structure (left half) + Revenue Streams (right half)
		bmcCostStructure: {bounds.X, bottomY, (totalW - bmcGap) / 2, bottomH},
		bmcRevenueStreams: {bounds.X + (totalW-bmcGap)/2 + bmcGap, bottomY,
			totalW - (totalW-bmcGap)/2 - bmcGap, bottomH},
	}

	var children [][]byte
	for i, panel := range panels {
		key := sectionOrder[i]
		cell := cells[key]
		colors := bmcSectionColors[key]

		headerCY := int64(float64(cell.h) * bmcHeaderHeightRatio)
		bodyCY := cell.h - headerCY

		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape
		headerXML := generateBMCCellHeaderXML(
			panel.title, cell.x, cell.y, cell.w, headerCY,
			headerID, colors.scheme, colors.lumMod, colors.lumOff,
		)
		children = append(children, []byte(headerXML))

		// Body shape
		bodyXML := generateBMCCellBodyXML(
			panel.body, cell.x, cell.y+headerCY, cell.w, bodyCY,
			bodyID, colors.scheme, colors.lumMod, colors.lumOff,
		)
		children = append(children, []byte(bodyXML))
	}

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Business Model Canvas",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateBMCGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateBMCCellHeaderXML produces a roundRect header shape for a BMC cell.
func generateBMCCellHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "BMC " + title,
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: bmcCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  [4]int64{bmcBodyInset, 0, bmcBodyInset, 0},
			AutoFit: "noAutofit",
			Paragraphs: []pptx.Paragraph{{
				Align:    "ctr",
				NoBullet: true,
				Runs: []pptx.Run{{
					Text:     title,
					Lang:     "en-US",
					FontSize: bmcHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    pptx.SchemeFill("dk1"),
				}},
			}},
		},
	})
	if err != nil {
		slog.Warn("generateBMCCellHeaderXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateBMCCellBodyXML produces a roundRect body shape for a BMC cell.
func generateBMCCellBodyXML(body string, x, y, cx, cy int64, shapeID uint32, schemeColor string, lumMod, lumOff int) string {
	paras := panelBulletsParagraphs(body, bmcBodyFontSize)

	// Use the accent color (full strength) for bullets
	bulletColor := pptx.SchemeFill(schemeColor)
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}

	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "BMC Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: bmcCornerRadius},
		},
		Fill: pptx.SchemeFill(schemeColor, pptx.LumMod(lumMod), pptx.LumOff(lumOff)),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:       "square",
			Anchor:     "t",
			Insets:     [4]int64{bmcBodyInset, bmcBodyInset, bmcBodyInset, bmcBodyInset},
			AutoFit:    "normAutofit",
			Paragraphs: paras,
		},
	})
	if err != nil {
		slog.Warn("generateBMCCellBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}
