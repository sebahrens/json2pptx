package generator

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
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
// Each box is a square-cornered card on the shared neutral surface with a
// bold accent header at the top and bulleted body items below. All boxes are wrapped in a single
// p:grpSp with identity child transform.
//
// Color strategy: neutral cards and one accent (the section titles), as the
// bmc-canvas pattern. See native_surface_style.go and taxonomy_palette.go.

// BMC EMU constants.
const (
	// bmcGap is the gap between boxes in EMU.
	bmcGap int64 = 73152 // ~0.08" — same as SWOT/PESTEL for consistency

	// bmcHeaderFontSize is the box header font size (hundredths of a point).
	// 1200 = 12pt (smaller than SWOT's 14pt because BMC has 9 dense cells)
	bmcHeaderFontSize int = 1200

	// bmcBodyFontSize is the bullet text size: the 12pt body step, the
	// smallest size a projected slide carries. It was 10pt on every template
	// (go-slide-creator-6ne1m).
	bmcBodyFontSize int = tokens.TypeScaleBodyHPt

	// bmcMaxBudgetBullets bounds the search for how many bullets a section
	// holds.
	bmcMaxBudgetBullets = 8

	// bmcBottomMaxColumns is the most columns the two bottom cells set their
	// bullets in.
	bmcBottomMaxColumns = 2

	// bmcHeaderHeightRatio is the fraction of box height used for the header area.
	bmcHeaderHeightRatio = 0.18

	// bmcBodyInset is the text inset for body text (EMU).
	bmcBodyInset = pptx.ShapeTextInsetEMU // uniform 0.5 cm shape text margin

	// bmcBulletSpaceAfter is the space after each BMC bullet (hundredths of
	// a point). Nine dense cells cannot afford the 6pt panel spacing.
	bmcBulletSpaceAfter = 200

	// bmcTopRowMinRatio / bmcTopRowMaxRatio bound the share of the height the
	// top row (5-column) takes; bmcCellRectsAt moves it within them to fit the
	// text.
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

// bmcDefaultTint is the template-independent default BMC section: the shared
// neutral card with an accent title, the cell the bmc-canvas pattern draws, so
// the native canvas and the pattern are one look (go-slide-creator-amtkg). The
// nine cells once took accent1–6 plus repeats (go-slide-creator-w0kj), then
// one accent tint with the Value Proposition a step deeper.
func bmcDefaultTint(int) taxonomyTint { return nativeSurface{}.cardTint(bmcHeaderFontSize) }

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
func generateBMCGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint, fontName string) string {
	if len(panels) != 9 {
		slog.Warn("generateBMCGroupXML: expected 9 panels", "got", len(panels))
		return ""
	}

	layout := bmcLayoutCells(panels, bounds, fontName)
	cells := layout.cells

	// The nine sections are peers: when any body needs a shrink, every body
	// is set at the one size at which all of them fit (bmcPeerSize). Left to
	// the writer each body stored its own autofit scale, so Key Resources and
	// Channels — the stacked half-height cells, the first to run out — were
	// set smaller than the sections beside them (go-slide-creator-7ee4f).
	type bmcBody struct {
		name string
		rect pptx.RectEmu
		id   uint32
		tint *taxonomyTint
		text pptx.TextBody
	}
	var headers [][]byte
	var bodies, extra []bmcBody
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
		headers = append(headers, []byte(generateBMCCellHeaderXML(
			panel.title, cell.x, cell.y, cell.w, headerCY,
			headerID, colors,
		)))

		// Body shape: the section's bullets, or the first column of them
		cols := bmcBodyColumns(panel.body, layout.columns(key))
		first := ""
		if len(cols) > 0 {
			first = cols[0]
		}
		body := pptx.RectEmu{X: cell.x, Y: cell.y + headerCY, CX: cell.w, CY: bodyCY}
		bodies = append(bodies, bmcBody{"BMC Body", body, bodyID, &colors,
			bmcColumnText(first, colors.scheme, 0, len(cols), cell.w, layout.pad, fontName)})
		for j := 1; j < len(cols); j++ {
			extra = append(extra, bmcBody{name: "BMC Bullets", rect: nativeColumnRect(body, j, len(cols)),
				text: bmcColumnText(cols[j], colors.scheme, j, len(cols), cell.w, layout.pad, fontName)})
		}
	}
	// Further columns take the IDs after the nine header / body pairs, so a
	// canvas with none keeps the IDs it always had.
	for i := range extra {
		extra[i].id = shapeIDBase + uint32(2*len(panels)+1+i)
	}
	all := append(append([]bmcBody{}, bodies...), extra...)
	texts := make([]*pptx.TextBody, len(all))
	rects := make([]pptx.RectEmu, len(all))
	for i := range all {
		texts[i], rects[i] = &all[i].text, all[i].rect
	}
	bmcApplyPeerSize(texts, rects, bmcPeerSize(texts, rects))
	var children [][]byte
	for i, b := range all[:len(bodies)] {
		children = append(children, headers[i], []byte(generateBMCBodyShapeXML(b.name, b.rect, b.id, b.tint, b.text)))
	}
	for _, b := range all[len(bodies):] {
		children = append(children, []byte(generateBMCBodyShapeXML(b.name, b.rect, b.id, nil, b.text)))
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

// bmcMeasuredBody is a section body as it must be measured: with the
// bullets' hanging indent taken off the line. The writer's autofit measure
// wraps every paragraph at the full text width and ignores the left margin a
// bulleted paragraph's lines start at, so "Run the runtime" (85pt) was held
// to fit a 90pt column whose lines are 76pt wide. The cell was then sized for
// one line less than every renderer sets, LibreOffice shrank that one cell a
// notch or two, and the nine sections were at two sizes
// (go-slide-creator-7ee4f). Measured with the indent in the left inset, the
// wraps are the renderer's.
func bmcMeasuredBody(tb pptx.TextBody) pptx.TextBody {
	var indent int64
	for _, p := range tb.Paragraphs {
		indent = max(indent, p.MarginL)
	}
	tb.Insets[0] += indent
	return tb
}

// bmcPeerSizeStep is the step the shared body size comes down in (hundredths
// of a point): half a point.
const bmcPeerSizeStep = 50

// bmcBodyFits reports whether tb, with every run at size, is written into
// rect with no autofit shrink, measured as the writer measures it (after the
// margin clamp GenerateShape applies).
func bmcBodyFits(tb *pptx.TextBody, rect pptx.RectEmu, size int) bool {
	if len(tb.Paragraphs) == 0 {
		return true
	}
	probe := *tb
	probe.Paragraphs = make([]pptx.Paragraph, len(tb.Paragraphs))
	for i, p := range tb.Paragraphs {
		p.Runs = append([]pptx.Run(nil), p.Runs...)
		for j := range p.Runs {
			p.Runs[j].FontSize = size
		}
		probe.Paragraphs[i] = p
	}
	probe.AutoFitFontScale, probe.AutoFitLnSpcReduction = 0, 0
	probe.Insets = pptx.EffectiveTextInsets(&probe, rect)
	probe.ExplicitInsets = true
	probe = bmcMeasuredBody(probe)
	return pptx.AutofitFitsFor(&probe, rect)
}

// bmcPeerSize is the one body size (hundredths of a point) the peer bodies
// are set at: the authored size when every body fits its box at it, else the
// largest half-point step below it at which they all do. It never goes under
// one point; a canvas that dense is refused by the readability check.
func bmcPeerSize(texts []*pptx.TextBody, rects []pptx.RectEmu) int {
	fits := func(size int) bool {
		for i, tb := range texts {
			if !bmcBodyFits(tb, rects[i], size) {
				return false
			}
		}
		return true
	}
	size := bmcBodyFontSize
	for size > 100 && !fits(size) {
		size -= bmcPeerSizeStep
	}
	return size
}

// bmcApplyPeerSize sets every peer body at size. Below the authored size the
// bodies are written at that size outright and marked with an autofit scale
// of 100%: a stored scale is what PowerPoint would apply per shape and
// LibreOffice ignores (it refits each shape itself, and only the ones that
// overflow), so only a declared size reads the same in both; the 100% mark
// keeps the shapes in the readability scan, which holds a body with a stored
// scale to the 12pt floor (unreadableAutofitFindings).
func bmcApplyPeerSize(texts []*pptx.TextBody, _ []pptx.RectEmu, size int) {
	if size >= bmcBodyFontSize {
		return
	}
	for _, tb := range texts {
		if len(tb.Paragraphs) == 0 {
			continue
		}
		for i := range tb.Paragraphs {
			for j := range tb.Paragraphs[i].Runs {
				tb.Paragraphs[i].Runs[j].FontSize = size
			}
		}
		tb.AutoFitFontScale, tb.AutoFitLnSpcReduction = 100000, 0
	}
}

// generateBMCCellHeaderXML produces the header shape of a BMC cell.
func generateBMCCellHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     "BMC " + title,
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
					FontSize: bmcHeaderFontSize,
					Bold:     true,
					Dirty:    true,
					Color:    tint.titleFill(),
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

// bmcBulletSpaceAfterAt is the space after a bullet at a body bottom margin of
// pad: the air between bullets goes with the first margin step, before type.
func bmcBulletSpaceAfterAt(pad int64) int {
	if pad < bmcBodyInset {
		return 0
	}
	return bmcBulletSpaceAfter
}

// generateBMCBodyShapeXML produces one text-bearing shape of a section body:
// the card in the section's tint, or — with no tint — an unfilled text box
// over it.
func generateBMCBodyShapeXML(name string, rect pptx.RectEmu, shapeID uint32, tint *taxonomyTint, text pptx.TextBody) string {
	opts := pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   rect,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text:     &text,
	}
	if tint != nil {
		opts.Geometry = nativeSurfaceGeometry
		opts.Fill = tint.fill()
		opts.Line = pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()}
		opts.TxBox = false
	}
	b, err := pptx.GenerateShape(opts)
	if err != nil {
		slog.Warn("generateBMCBodyShapeXML failed", "name", name, "error", err)
		return ""
	}
	return string(b)
}

// generateBMCCellBodyXML produces the body shape of a one-column BMC cell.
func generateBMCCellBodyXML(body string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint, pad int64, fontName string) string {
	return generateBMCBodyShapeXML("BMC Body", pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy}, shapeID, &tint,
		bmcBodyText(body, tint.scheme, pad, fontName))
}

// bmcBodyText is the text body of a BMC cell body with a bottom margin of pad.
// The writer and bmcLayoutCells' measurement share it, so what is measured is
// what is written. fontName is the template's body face: the measure is taken
// in it where it measures the same on every host, so a wide face (Poppins)
// is not sized as Arial and left to overflow its cell.
func bmcBodyText(body, schemeColor string, pad int64, fontName string) pptx.TextBody {
	var paras []pptx.Paragraph
	if body != "" {
		paras = pptx.ParseBulletText(body, pptx.BulletTextOptions{
			FontSize:    bmcBodyFontSize,
			Lang:        "en-US",
			Dirty:       true,
			BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
			SpaceAfter:  bmcBulletSpaceAfterAt(pad),
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
	insets := nativeCardBodyInsets()
	insets[3] = pad
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "t",
		Insets:     insets,
		AutoFit:    "normAutofit",
		Paragraphs: paras,
		ThemeFonts: pptx.ThemeFonts{Major: fontName, Minor: fontName},
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
func bmcCellRects(panels []nativePanelData, bounds types.BoundingBox, fontName string) map[bmcSectionKey]bmcCellRect {
	return bmcLayoutCells(panels, bounds, fontName).cells
}

// bmcLayout is the nine cells, the bottom text margin their bodies are written
// with and whether every body holds its text at the authored size.
type bmcLayout struct {
	cells map[bmcSectionKey]bmcCellRect
	pad   int64
	// bottomCols is how many columns Cost Structure and Revenue Streams set
	// their bullets in: the two cells are half the canvas wide.
	bottomCols int
	fits       bool
}

// columns is how many columns the section's bullets are set in.
func (l bmcLayout) columns(key bmcSectionKey) int {
	if key == bmcCostStructure || key == bmcRevenueStreams {
		return max(1, l.bottomCols)
	}
	return 1
}

// bmcLayoutCells lays the canvas out at the first bottom margin of
// nativeVerticalPadSteps at which every section holds its bullets at 12pt: a
// canvas gives up the air under its lists before its type. One that fits at
// no step keeps the last and the shared shrink bmcCellRectsAt gives its rows
// (go-slide-creator-6ne1m).
func bmcLayoutCells(panels []nativePanelData, bounds types.BoundingBox, fontName string) bmcLayout {
	var l bmcLayout
	for _, pad := range nativeVerticalPadSteps {
		// Width before height: the wide bottom cells take a second column of
		// bullets before any margin is tightened.
		for cols := 1; cols <= bmcBottomMaxColumns; cols++ {
			l.pad, l.bottomCols = pad, cols
			l.cells, l.fits = bmcCellRectsAt(panels, bounds, pad, cols, fontName)
			if l.fits {
				return l
			}
		}
	}
	// Nothing fits: the canvas keeps one column and the last margin, and the
	// rows share the shrink.
	l.bottomCols = 1
	l.cells, l.fits = bmcCellRectsAt(panels, bounds, l.pad, 1, fontName)
	return l
}

// bmcBodyColumns splits a section body — one bullet per line — into the
// bodies of its columns.
func bmcBodyColumns(body string, cols int) []string {
	if body == "" {
		return nil
	}
	split := nativeSplitColumns(strings.Split(body, "\n"), cols)
	out := make([]string, len(split))
	for i, lines := range split {
		out[i] = strings.Join(lines, "\n")
	}
	return out
}

// bmcColumnText is the text body of column j of n of a section body bodyW
// wide.
func bmcColumnText(body, schemeColor string, j, n int, bodyW, pad int64, fontName string) pptx.TextBody {
	text := bmcBodyText(body, schemeColor, pad, fontName)
	text.Insets = nativeColumnInsets(text.Insets, j, n, bodyW)
	return text
}

// bmcFitBudget measures what a section holds at the authored size in bounds:
// how many one-line bullets every section can list at once, and how many
// characters a line takes in a top-row column.
func bmcFitBudget(panels []nativePanelData, bounds types.BoundingBox, fontName string) nativeFitBudget {
	probe := func(n int) bmcLayout {
		filled := append([]nativePanelData{}, panels...)
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "- Item"
		}
		for i := range filled {
			filled[i].body = strings.Join(lines, "\n")
		}
		return bmcLayoutCells(filled, bounds, fontName)
	}
	items := 0
	for n := 1; n <= bmcMaxBudgetBullets; n++ {
		if !probe(n).fits {
			break
		}
		items = n
	}
	colW := (bounds.Width - 4*bmcGap) / 5
	chars := nativeOneLineChars(func(s string) pptx.TextBody {
		return bmcBodyText("- "+s, "", pptx.ShapeTextInsetEMU, fontName)
	}, colW)
	return nativeFitBudget{
		item: "bullet", container: "section", maxItems: items, maxChars: chars,
		sizePt: float64(bmcBodyFontSize) / 100,
	}
}

// bmcCellRectsAt is the canvas laid out with a body bottom margin of pad, and
// whether every section's text then fits its cell unshrunk.
func bmcCellRectsAt(panels []nativePanelData, bounds types.BoundingBox, pad int64, bottomCols int, fontName string) (map[bmcSectionKey]bmcCellRect, bool) {
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
	bodyFixed := nativeCardSeamInsetEMU + pad
	columns := bmcLayout{bottomCols: bottomCols}.columns
	bodyNeed := func(key bmcSectionKey) int64 {
		cols := bmcBodyColumns(body[key], columns(key))
		var need int64
		for j, col := range cols {
			text := bmcMeasuredBody(bmcColumnText(col, "", j, len(cols), cellW(key), pad, fontName))
			w := nativeColumnRect(pptx.RectEmu{CX: cellW(key)}, j, len(cols)).CX
			need = max(need, nativeTextNeedAtMarginEMU(text, w, totalH))
		}
		return max(0, need-bodyFixed)
	}
	// A row is as tall as its tallest cell. Taking the largest title and the
	// largest body separately asked for a cell no section has — the two-line
	// "Customer Relationships" over Key Activities' five lines — and turned
	// a canvas that fits into one that shrinks.
	need := func(keys ...bmcSectionKey) bmcNeed {
		var n bmcNeed
		for _, key := range keys {
			k := bmcNeed{bmcHeaderNeed(title[key], cellW(key), fontName) + bodyFixed, bodyNeed(key)}
			if k.at(1) > n.at(1) {
				n = k
			}
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

	var topH int64
	if top.at(1)+bottom.at(1) <= totalH {
		// Both rows fit: the spare height is shared in proportion to what
		// each row needs. A fixed 60% handed it all to the bottom row on a
		// tall content area — three bullets in a cell that holds eight —
		// and left the stacked cells exactly their measured need, which a
		// renderer whose face runs a little taller than the measure shrank.
		topH, _ = splitByNeed(totalH, top.at(1), bottom.at(1), 1-bmcTopRowMaxRatio)
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
		header := min(max(int64(float64(h)*bmcHeaderHeightRatio), bmcHeaderNeed(title[key], w, fontName)), h)
		return bmcCellRect{x, y, w, h, header}
	}

	cells := map[bmcSectionKey]bmcCellRect{
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
	fits := true
	for _, key := range bmcSectionOrder {
		if body[key] == "" {
			continue
		}
		c := cells[key]
		if c.header+bodyFixed+bodyNeed(key) > c.h+int64(types.EMUPerPoint)/2 {
			fits = false
		}
	}
	return cells, fits
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
func bmcHeaderNeed(title string, w int64, fontName string) int64 {
	if title == "" {
		return 0
	}
	if !fontcache.HostIndependent(fontName) {
		fontName = panelMeasureDefaultFont
	}
	width := int64(float64(measureWidthEMU(w, bmcBodyInset, bmcBodyInset)) / pyramidBoldWidthFactor)
	return measureNativeText(title, fontName, float64(bmcHeaderFontSize)/100, width) + 2*nativeCardSeamInsetEMU
}
