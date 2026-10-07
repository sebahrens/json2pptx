package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
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

	// pestelBodyFontSize is the bullet text size: the 12pt body step, the
	// smallest size a projected slide carries. It was 11pt
	// (go-slide-creator-6ne1m).
	pestelBodyFontSize int = tokens.TypeScaleBodyHPt

	// pestelMaxColumns is the most columns a segment sets its bullets in.
	pestelMaxColumns = 2

	// pestelMaxBudgetBullets bounds the search for how many bullets a segment
	// holds.
	pestelMaxBudgetBullets = 10

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

// Laying the grid out from its text (go-slide-creator-6ne1m).
//
// A PESTEL cell was a header and a body that each kept the uniform margin on
// all four sides — 1 cm of dead space at the seam between a title and its own
// first bullet — with 6pt after every 11pt bullet. Three bullets a cell did
// not fit the two rows of the shortest shipped content areas and were stored
// at 78%. The body is now 12pt and the cell gives, in this order:
//
//  1. The seam. Header and body share one card, so the margin between them is
//     the seam inset, as in the BMC and SWOT cards.
//  2. Width before height. A cell whose bullets do not fit in one column sets
//     them in two.
//  3. Padding before type. A grid that still does not fit steps the top and
//     bottom text margins, and the space between bullets, down
//     nativeVerticalPadSteps.
//  4. Only then does the writer store an autofit scale, which generation
//     reports with the budget pestelFitBudget measured.

// pestelLayout is the grid's geometry and how many columns each cell sets its
// bullets in.
type pestelLayout struct {
	numCols, numRows       int
	cellW, cellH, headerCY int64
	pad                    int64
	columns                []int
	fits                   bool
}

// pestelBulletSpaceAfterAt is the space after a bullet at a top / bottom
// margin of pad (hundredths of a point): the air between bullets steps down
// with the margin, before type.
func pestelBulletSpaceAfterAt(pad int64) int {
	switch {
	case pad >= pestelBodyInset:
		return panelBulletSpaceAfter
	case pad >= nativeVerticalPadSteps[1]:
		return panelBulletSpaceAfter / 2
	}
	return 0
}

// pestelHeaderText is the text body of a segment header.
func pestelHeaderText(title string, tint taxonomyTint, pad int64, fontName string) pptx.TextBody {
	insets := nativeCardHeaderInsets()
	insets[1] = pad
	anchor := "ctr"
	if tint.open {
		// An open heading stands on its rule, flush with its left end.
		anchor = "b"
		insets[0] = 0
	}
	return pptx.TextBody{
		Wrap:    "square",
		Anchor:  anchor,
		Insets:  insets,
		AutoFit: "normAutofit",
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
		ThemeFonts: pptx.ThemeFonts{Major: fontName, Minor: fontName},
	}
}

// pestelColumnText is the text body of bullet column j of n in a segment body
// bodyW wide.
func pestelColumnText(body string, j, n int, bodyW, pad int64, tint taxonomyTint, fontName string) pptx.TextBody {
	var paras []pptx.Paragraph
	if body != "" {
		paras = pptx.ParseBulletText(body, pptx.BulletTextOptions{
			FontSize:    pestelBodyFontSize,
			Lang:        "en-US",
			Dirty:       true,
			BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
			SpaceAfter:  pestelBulletSpaceAfterAt(pad),
		})
		paras[len(paras)-1].SpaceAfter = 0
	}
	// Use the same accent color for bullets but with full strength
	bulletColor := pptx.ResolveColorString(tint.scheme)
	if tint.open {
		// An open segment's bullets sit on the page: the text ink.
		bulletColor = pptx.SchemeFill("dk1")
	}
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}
	diagramPanelBodyColors(paras, tint.scheme)
	insets := nativeCardBodyInsets()
	insets[3] = pad
	if tint.open && j == 0 {
		// No surface, no margin from its edge: the bullets start on the
		// heading's rule, which starts on the slide's text edge.
		insets[0] = 0
	}
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "t",
		Insets:     nativeColumnInsets(insets, j, n, bodyW),
		AutoFit:    "normAutofit",
		Paragraphs: paras,
		ThemeFonts: pptx.ThemeFonts{Major: fontName, Minor: fontName},
	}
}

// pestelBodyColumns splits a segment body — one bullet per line — into the
// bodies of its columns.
func pestelBodyColumns(body string, cols int) []string {
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

// layoutPESTEL lays the segments out in bounds: three columns, as many rows as
// the segments need.
func layoutPESTEL(panels []nativePanelData, bounds types.BoundingBox, fontName string) pestelLayout {
	l := pestelLayout{numCols: 3}
	l.numRows = (len(panels) + l.numCols - 1) / l.numCols
	if l.numRows == 0 {
		return l
	}
	l.cellW = (bounds.Width - int64(l.numCols-1)*pestelGap) / int64(l.numCols)
	l.cellH = (bounds.Height - int64(l.numRows-1)*pestelGap) / int64(l.numRows)
	l.columns = make([]int, len(panels))

	fitsIn := func(body string, cols int, bodyH, pad int64) bool {
		split := pestelBodyColumns(body, cols)
		for j, col := range split {
			w := nativeColumnRect(pptx.RectEmu{CX: l.cellW}, j, len(split)).CX
			if nativeTextNeedAtMarginEMU(pestelColumnText(col, j, len(split), l.cellW, pad, taxonomyTint{}, fontName), w, bodyH+1) > bodyH {
				return false
			}
		}
		return true
	}
	for _, pad := range nativeVerticalPadSteps {
		l.pad = pad
		l.headerCY = 0
		for _, p := range panels {
			l.headerCY = max(l.headerCY, nativeHeaderNeedEMU(pestelHeaderText(p.title, taxonomyTint{}, pad, fontName), l.cellW, l.cellH))
		}
		bodyH := l.cellH - l.headerCY
		l.fits = l.headerCY < l.cellH/2
		for i, p := range panels {
			l.columns[i] = 0
			for c := 1; c <= pestelMaxColumns && l.columns[i] == 0; c++ {
				if p.body == "" || fitsIn(p.body, c, bodyH, pad) {
					l.columns[i] = c
				}
			}
			if l.columns[i] == 0 {
				l.columns[i] = 1
				l.fits = false
			}
		}
		if l.fits {
			break
		}
	}
	return l
}

// pestelFitBudget measures what a segment holds at the authored size in
// bounds: how many one-line bullets every segment can list, and how many
// characters one of those lines takes in a column of that list.
func pestelFitBudget(panels []nativePanelData, bounds types.BoundingBox, fontName string) nativeFitBudget {
	probe := func(n int) pestelLayout {
		filled := append([]nativePanelData{}, panels...)
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "- Item"
		}
		for i := range filled {
			filled[i].body = strings.Join(lines, "\n")
		}
		return layoutPESTEL(filled, bounds, fontName)
	}
	items, columns := 0, 1
	for n := 1; n <= pestelMaxBudgetBullets; n++ {
		l := probe(n)
		if !l.fits {
			break
		}
		items, columns = n, l.columns[0]
	}
	l := probe(1)
	j := min(1, columns-1)
	chars := nativeOneLineChars(func(s string) pptx.TextBody {
		return pestelColumnText("- "+s, j, columns, l.cellW, pptx.ShapeTextInsetEMU, taxonomyTint{}, fontName)
	}, nativeColumnRect(pptx.RectEmu{CX: l.cellW}, j, columns).CX)
	return nativeFitBudget{
		item: "bullet", container: "segment", maxItems: items, maxChars: chars,
		sizePt: float64(pestelBodyFontSize) / 100,
	}
}

// generatePESTELGroupXML produces the complete <p:grpSp> XML for a 3x2 PESTEL grid.
// Each segment is a card in its tint with a bold header and a bulleted body.
func generatePESTELGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint, fontName string) string {
	n := len(panels)
	if n == 0 {
		slog.Warn("generatePESTELGroupXML: no panels provided")
		return ""
	}
	l := layoutPESTEL(panels, bounds, fontName)
	bodyCY := l.cellH - l.headerCY

	type extraColumn struct {
		rect pptx.RectEmu
		text pptx.TextBody
	}
	var children [][]byte
	var extra []extraColumn
	for i, panel := range panels {
		col := i % l.numCols
		row := i / l.numCols

		cellX := bounds.X + int64(col)*(l.cellW+pestelGap)
		cellY := bounds.Y + int64(row)*(l.cellH+pestelGap)

		sc := uniformTaxonomyTint(i)
		if i < len(tints) {
			sc = tints[i]
		}

		headerID := shapeIDBase + uint32(i*2) + 1
		bodyID := shapeIDBase + uint32(i*2) + 2

		// Header shape: the card fill under a bold left-aligned title
		children = append(children, []byte(generatePESTELShapeXML(
			"PESTEL "+panel.title, pptx.RectEmu{X: cellX, Y: cellY, CX: l.cellW, CY: l.headerCY}, headerID, &sc,
			pestelHeaderText(panel.title, sc, l.pad, fontName))))

		// Body shape: the same fill, top-aligned bullets in its first column
		cols := pestelBodyColumns(panel.body, l.columns[i])
		first := ""
		if len(cols) > 0 {
			first = cols[0]
		}
		body := pptx.RectEmu{X: cellX, Y: cellY + l.headerCY, CX: l.cellW, CY: bodyCY}
		children = append(children, []byte(generatePESTELShapeXML(
			"PESTEL Body", body, bodyID, &sc, pestelColumnText(first, 0, len(cols), l.cellW, l.pad, sc, fontName))))
		for j := 1; j < len(cols); j++ {
			extra = append(extra, extraColumn{
				rect: nativeColumnRect(body, j, len(cols)),
				text: pestelColumnText(cols[j], j, len(cols), l.cellW, l.pad, sc, fontName),
			})
		}
	}
	// Further columns take the IDs after the header / body pairs, so a grid
	// with none keeps the IDs it always had.
	for i, col := range extra {
		children = append(children, []byte(generatePESTELShapeXML(
			"PESTEL Bullets", col.rect, shapeIDBase+uint32(2*n+1+i), nil, col.text)))
	}
	// An open segment's heading stands on one rule, as wide as its text.
	ruleID := shapeIDBase + uint32(2*n+1+len(extra))
	for i := range panels {
		sc := uniformTaxonomyTint(i)
		if i < len(tints) {
			sc = tints[i]
		}
		if !sc.open {
			continue
		}
		cellX := bounds.X + int64(i%l.numCols)*(l.cellW+pestelGap)
		cellY := bounds.Y + int64(i/l.numCols)*(l.cellH+pestelGap)
		children = append(children, []byte(nativeRuleXML("PESTEL Rule", pptx.RectEmu{
			X: cellX, Y: cellY + l.headerCY - nativeRuleWidthEMU/2,
			CX: l.cellW - pestelBodyInset, CY: nativeRuleWidthEMU,
		}, ruleID, nativeRuleInkPct)))
		ruleID++
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

// generatePESTELShapeXML produces one text-bearing shape of a segment: a card
// in the segment's tint, or — with no tint — an unfilled text box over one.
func generatePESTELShapeXML(name string, rect pptx.RectEmu, shapeID uint32, tint *taxonomyTint, text pptx.TextBody) string {
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
		slog.Warn("generatePESTELShapeXML failed", "name", name, "error", err)
		return ""
	}
	return string(b)
}

// generatePESTELHeaderXML produces the header shape of a PESTEL segment.
func generatePESTELHeaderXML(title string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	return generatePESTELShapeXML("PESTEL "+title, pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy}, shapeID, &tint,
		pestelHeaderText(title, tint, pptx.ShapeTextInsetEMU, ""))
}

// generatePESTELBodyXML produces the body shape of a one-column PESTEL segment.
func generatePESTELBodyXML(body string, x, y, cx, cy int64, shapeID uint32, tint taxonomyTint) string {
	return generatePESTELShapeXML("PESTEL Body", pptx.RectEmu{X: x, Y: y, CX: cx, CY: cy}, shapeID, &tint,
		pestelColumnText(body, 0, 1, cx, pptx.ShapeTextInsetEMU, tint, ""))
}
