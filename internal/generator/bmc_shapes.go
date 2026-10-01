package generator

import (
	"fmt"
	"log/slog"
	"sort"
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
// Color strategy: one hue — the deck's accent1 — with the Value Proposition a
// step deeper. Text is dk1. See taxonomy_palette.go.

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
	bmcBodyInset = pptx.ShapeTextInsetEMU // uniform 0.5 cm shape text margin

	// bmcBulletSpaceAfter is the space after each BMC bullet (hundredths of
	// a point). Nine dense cells cannot afford the 6pt panel spacing.
	bmcBulletSpaceAfter = 200

	// bmcTopRowRatio is the default fraction of total height for the top row
	// (5-column); bmcCellRects moves it within [min, max] to fit the text.
	bmcTopRowRatio    = 0.60
	bmcTopRowMinRatio = 0.45
	bmcTopRowMaxRatio = 0.80

	// bmcStackMinShare is the smallest share of a stacked column either of
	// its two cells keeps, however unequal their text.
	bmcStackMinShare = 0.3
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

// bmcSectionOrder is the canonical order the panels array is built in and the
// order generateBMCGroupXML reads back, so an index means the same section on
// both sides. It was duplicated as a local in each.
var bmcSectionOrder = []bmcSectionKey{
	bmcKeyPartners, bmcKeyActivities, bmcKeyResources,
	bmcValueProposition, bmcCustRelations, bmcChannels,
	bmcCustSegments, bmcCostStructure, bmcRevenueStreams,
}

// bmcDefaultTint is the fill for BMC section i in render order. One hue — the
// deck's accent1 — with the Value Proposition carried a step deeper, because it
// is the one cell the canvas actually privileges: every other block exists to
// explain it. The nine cells used to take accent1–6 plus repeats, which on a
// forest-green deck rendered as a nine-colour rainbow (go-slide-creator-w0kj).
func bmcDefaultTint(i int) taxonomyTint {
	if i < len(bmcSectionOrder) && bmcSectionOrder[i] == bmcValueProposition {
		return taxonomyDeep
	}
	return taxonomyLight
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

// bmcSectionAliases maps every accepted input key to its canonical section.
// Agents are told the nine required keys by capabilities / skill_info / the
// bmc-canvas pattern (customer_relations, value_propositions), while older
// decks and the examples use customer_relationships / value_proposition; the
// native canvas reads both spellings, because a documented key used to render
// an empty cell with no warning (go-slide-creator-b7qqg.17). The synonyms also
// cover the semantic framework compiler's list, so one vocabulary works on
// every path.
var bmcSectionAliases = map[string]bmcSectionKey{
	"customer_relations":     bmcCustRelations,
	"customer_relationship":  bmcCustRelations,
	"customerRelations":      bmcCustRelations,
	"channel":                bmcChannels,
	"revenues":               bmcRevenueStreams,
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

// bmcNonSectionKeys are data keys that are not sections but are never content
// loss (the nested-boxes wrapper, and titles the slide draws elsewhere).
var bmcNonSectionKeys = map[string]bool{"boxes": true, "title": true, "subtitle": true}

// bmcAliasKeysSorted returns the alias keys in a fixed order: each section's
// canonical key first, then the rest alphabetically. Ranging over the map made
// the winner random when an author supplied two spellings of one section.
func bmcAliasKeysSorted() []string {
	keys := make([]string, 0, len(bmcSectionAliases))
	for k := range bmcSectionAliases {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ci := keys[i] == string(bmcSectionAliases[keys[i]])
		cj := keys[j] == string(bmcSectionAliases[keys[j]])
		if ci != cj {
			return ci
		}
		return keys[i] < keys[j]
	})
	return keys
}

// bmcDataSource returns the map holding the sections: the nested "boxes"
// object when present, else the data itself.
func bmcDataSource(data map[string]any) map[string]any {
	if boxes, ok := data["boxes"].(map[string]any); ok {
		return boxes
	}
	return data
}

// BMCIgnoredKeys lists, sorted, the data keys a native business_model_canvas
// never reads. Their content would vanish from the slide, so preflight reports
// them as diagram.data_key_ignored.
func BMCIgnoredKeys(data map[string]any) []string {
	var out []string
	for key := range bmcDataSource(data) {
		if _, ok := bmcSectionAliases[key]; ok || bmcNonSectionKeys[key] {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
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

	panels := bmcPanels(diagramSpec)
	if ignored := BMCIgnoredKeys(diagramSpec.Data); len(ignored) > 0 {
		slog.Warn("native bmc: data keys are not canvas sections and are not drawn",
			"slide", slideNum, "keys", ignored)
	}

	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	placeholderBounds := getPlaceholderBounds(shape, nil)

	slog.Info("native bmc shapes: registered",
		"slide", slideNum,
		"bounds", fmt.Sprintf("%dx%d+%d+%d", placeholderBounds.Width, placeholderBounds.Height, placeholderBounds.X, placeholderBounds.Y))

	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], panelShapeInsert{
		altText:        diagramAltText(item),
		placeholderIdx: shapeIdx,
		bounds:         placeholderBounds,
		panels:         panels,
		bmcMode:        true,
		taxonomyTints:  taxonomyPalette(diagramSpec, len(bmcSectionOrder), bmcDefaultTint),
	})
}

// bmcPanels converts a BMC spec to panel data. Sections are stored in
// bmcSectionOrder so generateBMCGroupXML knows which section each panel
// represents.
func bmcPanels(spec *types.DiagramSpec) []nativePanelData {
	sections := parseBMCSections(spec.Data)
	panels := make([]nativePanelData, len(bmcSectionOrder))
	for i, key := range bmcSectionOrder {
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
		panels[i] = nativePanelData{title: title, body: body}
	}
	return panels
}

// parseBMCSections parses BMC section data from the diagram data map.
// Supports both flat format ({key_partners: ["items"]}) and nested format
// ({boxes: {key_partners: {items: ["items"]}}}).
func parseBMCSections(data map[string]any) map[bmcSectionKey]bmcSectionData {
	sections := make(map[bmcSectionKey]bmcSectionData)
	dataSource := bmcDataSource(data)

	// Two spellings of one section (customer_relations and
	// customer_relationships) are merged, canonical spelling first, rather
	// than one being discarded at random.
	for _, key := range bmcAliasKeysSorted() {
		rawVal, ok := dataSource[key]
		if !ok {
			continue
		}
		alias := bmcSectionAliases[key]
		sec := sections[alias]
		sec.key = alias
		switch v := rawVal.(type) {
		case []any, []string, string:
			sec.items = append(sec.items, bmcStrings(v)...)
		case map[string]any:
			// {title, items} is the native shape; {header, bullets} is the
			// bmc-canvas pattern's cell shape, accepted so a payload moved
			// between the two paths keeps its text.
			for _, titleKey := range []string{"title", "header"} {
				if title, ok := v[titleKey].(string); ok && title != "" && sec.title == "" {
					sec.title = title
				}
			}
			sec.items = append(sec.items, bmcStrings(v["items"])...)
			sec.items = append(sec.items, bmcStrings(v["bullets"])...)
		}
		sections[alias] = sec
	}

	return sections
}

// bmcStrings reads a section value as a list of strings.
func bmcStrings(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// generateBMCGroupXML produces the complete <p:grpSp> XML for a 9-box BMC grid.
// Panels must be in canonical order: key_partners, key_activities, key_resources,
// value_proposition, customer_relationships, channels, customer_segments,
// cost_structure, revenue_streams.
func generateBMCGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint) string {
	if len(panels) != 9 {
		slog.Warn("generateBMCGroupXML: expected 9 panels", "got", len(panels))
		return ""
	}

	cells := bmcCellRects(panels, bounds)

	var children [][]byte
	for i, panel := range panels {
		key := bmcSectionOrder[i]
		cell := cells[key]
		colors := bmcDefaultTint(i)
		if i < len(tints) {
			colors = tints[i]
		}

		headerCY := cell.header
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
		Fill: diagramTintFill(schemeColor, lumMod, lumOff),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &pptx.TextBody{
			Wrap:    "square",
			Anchor:  "ctr",
			Insets:  pptx.ShapeTextInsets(),
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
					Color:    diagramPanelTextFill(schemeColor),
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
	text := bmcBodyText(body, schemeColor)
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "BMC Body",
		Bounds:   pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy},
		Geometry: pptx.GeomRoundRect,
		Adjustments: []pptx.AdjustValue{
			{Name: "adj", Value: bmcCornerRadius},
		},
		Fill: diagramTintFill(schemeColor, lumMod, lumOff),
		Line: pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()},
		Text: &text,
	})
	if err != nil {
		slog.Warn("generateBMCCellBodyXML failed", "error", err)
		return ""
	}
	return string(b)
}

// bmcBodyText is the text body of a BMC cell body. The writer and
// bmcCellRects' measurement share it, so what is measured is what is written.
func bmcBodyText(body, schemeColor string) pptx.TextBody {
	var paras []pptx.Paragraph
	if body != "" {
		paras = pptx.ParseBulletText(body, pptx.BulletTextOptions{
			FontSize:    bmcBodyFontSize,
			Lang:        "en-US",
			Dirty:       true,
			BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
			SpaceAfter:  bmcBulletSpaceAfter,
		})
	}
	// Use the accent color (full strength) for bullets
	bulletColor := pptx.ResolveColorString(schemeColor)
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

// bmcCellRect is one BMC section's box.
type bmcCellRect struct {
	x, y, w, h int64
	header     int64 // height of the header band at the top of the cell
}

// bmcCellRects lays out the nine sections inside bounds. The canvas keeps its
// Osterwalder shape — five top columns (Key Activities over Key Resources and
// Customer Relationships over Channels), Cost Structure and Revenue Streams
// below — but the two row boundaries follow the measured text: the top row
// keeps its default 60% unless a row needs more, and the split inside the
// stacked columns follows what the upper and lower cells need. A dense canvas
// on a short content area used to keep 60/40 and 50/50 and store a 20%
// autofit into its half-height cells (go-slide-creator-zbo58).
func bmcCellRects(panels []nativePanelData, bounds types.BoundingBox) map[bmcSectionKey]bmcCellRect {
	totalW, totalH := bounds.Width, bounds.Height
	colW := (totalW - 4*bmcGap) / 5
	wideLW := (totalW - bmcGap) / 2
	wideRW := totalW - wideLW - bmcGap

	body := make(map[bmcSectionKey]string, len(bmcSectionOrder))
	title := make(map[bmcSectionKey]string, len(bmcSectionOrder))
	for i, key := range bmcSectionOrder {
		if i < len(panels) {
			body[key], title[key] = panels[i].body, panels[i].title
		}
	}
	width := map[bmcSectionKey]int64{bmcCostStructure: wideLW, bmcRevenueStreams: wideRW}
	cellW := func(key bmcSectionKey) int64 {
		if w, ok := width[key]; ok {
			return w
		}
		return colW
	}
	// need is what a cell takes at full size: its fixed part (wrapped title,
	// body insets) and the body text, which an autofit shrink scales.
	bodyFixed := nativeCardSeamInsetEMU + bmcBodyInset
	need := func(keys ...bmcSectionKey) bmcNeed {
		var n bmcNeed
		for _, key := range keys {
			fixed := bmcHeaderNeed(title[key], cellW(key)) + bodyFixed
			text := max(0, nativeTextNeedEMU(bmcBodyText(body[key], ""), cellW(key), totalH)-bodyFixed)
			n = bmcNeed{max(n.fixed, fixed), max(n.text, text)}
		}
		return n
	}
	upper := need(bmcKeyActivities, bmcCustRelations)
	lower := need(bmcKeyResources, bmcChannels)
	stack := bmcNeed{upper.fixed + lower.fixed + bmcGap, upper.text + lower.text}
	top := need(bmcKeyPartners, bmcValueProposition, bmcCustSegments)
	if stack.at(1) > top.at(1) {
		top = stack
	}
	top.fixed += bmcGap // the gap before the bottom row
	bottom := need(bmcCostStructure, bmcRevenueStreams)

	topH := int64(float64(totalH) * bmcTopRowRatio)
	if top.at(1)+bottom.at(1) <= totalH {
		// Both rows fit: keep the default split when it serves both, else
		// move the boundary only as far as the short row needs.
		topH = min(max(topH, top.at(1)), totalH-bottom.at(1))
	} else {
		// Neither row can have all it needs: give both the same shrink.
		topH = top.at(bmcSharedScale(totalH, top, bottom))
	}
	topH = min(max(topH, int64(float64(totalH)*bmcTopRowMinRatio)), int64(float64(totalH)*bmcTopRowMaxRatio))
	bottomH := totalH - topH

	fullH := topH - bmcGap // leave gap before bottom row
	stackH := fullH - bmcGap
	upperH, _ := splitByNeed(stackH, upper.at(1), lower.at(1), bmcStackMinShare)
	if upper.at(1)+lower.at(1) > stackH {
		upperH = upper.at(bmcSharedScale(stackH, upper, lower))
		upperH = min(max(upperH, int64(float64(stackH)*bmcStackMinShare)), int64(float64(stackH)*(1-bmcStackMinShare)))
	}
	lowerH := stackH - upperH

	topY := bounds.Y
	lowY := topY + upperH + bmcGap
	bottomY := bounds.Y + topH
	colX := func(i int64) int64 { return bounds.X + i*(colW+bmcGap) }
	rect := func(key bmcSectionKey, x, y, w, h int64) bmcCellRect {
		header := min(max(int64(float64(h)*bmcHeaderHeightRatio), bmcHeaderNeed(title[key], w)), h)
		return bmcCellRect{x, y, w, h, header}
	}

	return map[bmcSectionKey]bmcCellRect{
		bmcKeyPartners:      rect(bmcKeyPartners, colX(0), topY, colW, fullH),
		bmcKeyActivities:    rect(bmcKeyActivities, colX(1), topY, colW, upperH),
		bmcKeyResources:     rect(bmcKeyResources, colX(1), lowY, colW, lowerH),
		bmcValueProposition: rect(bmcValueProposition, colX(2), topY, colW, fullH),
		bmcCustRelations:    rect(bmcCustRelations, colX(3), topY, colW, upperH),
		bmcChannels:         rect(bmcChannels, colX(3), lowY, colW, lowerH),
		bmcCustSegments:     rect(bmcCustSegments, colX(4), topY, colW, fullH),
		bmcCostStructure:    rect(bmcCostStructure, bounds.X, bottomY, wideLW, bottomH),
		bmcRevenueStreams:   rect(bmcRevenueStreams, bounds.X+wideLW+bmcGap, bottomY, wideRW, bottomH),
	}
}

// bmcNeed is a region's height need split into the part an autofit shrink
// cannot reduce (titles, insets, gaps) and the body text it scales.
type bmcNeed struct{ fixed, text int64 }

// at is the height the region needs with its body text scaled by s.
func (n bmcNeed) at(s float64) int64 { return n.fixed + int64(s*float64(n.text)) }

// bmcSharedScale is the text scale at which two stacked regions exactly fill
// total, so neither is shrunk further than the other.
func bmcSharedScale(total int64, a, b bmcNeed) float64 {
	if a.text+b.text <= 0 {
		return 1
	}
	return min(1, max(0, float64(total-a.fixed-b.fixed)/float64(a.text+b.text)))
}

// bmcHeaderNeed is the height a section title needs at the header size in a
// cell of width w: its wrapped lines plus the seam inset above and below. The
// header's own 0.5 cm vertical margin is clamped away in a band this short,
// as it always was; what must not happen is a two-line title ("Customer
// Relationships" on a narrow canvas) cut off by a one-line band.
func bmcHeaderNeed(title string, w int64) int64 {
	if title == "" {
		return 0
	}
	width := int64(float64(measureWidthEMU(w, bmcBodyInset, bmcBodyInset)) / pyramidBoldWidthFactor)
	return measureNativeText(title, panelMeasureDefaultFont, float64(bmcHeaderFontSize)/100, width) + 2*nativeCardSeamInsetEMU
}
