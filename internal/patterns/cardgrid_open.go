package patterns

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// The open card grid (go-slide-creator-mot7a).
//
// The default card used to be a pale grey tile under an accent top rule with
// its heading and body hugging the top-left corner — the web card, six times
// a slide, six accent rules competing. The open card is the vocabulary the
// redesigned patterns share: a bold heading standing on ONE neutral rule, the
// body beneath it, and whitespace gutters instead of a box per item. A card
// is three grid rows (heading, rule, body), so the headings of a row share a
// baseline however many lines one of them wraps to.
const (
	cardGridStyleOpen   = "open"
	cardGridStyleFilled = "filled"

	// cardGridOpenRulePt is the rule a heading stands on.
	cardGridOpenRulePt = 1.0
	// cardGridOpenEmphasisRulePt is the rule of a card whose cell_overrides
	// asks for accent_bar: the accent, a little heavier.
	cardGridOpenEmphasisRulePt = 2.0
	// cardGridOpenRuleInkPct is the dk1 coverage of the rule.
	cardGridOpenRuleInkPct = NeutralTint60
	// cardGridOpenColGapPt is the whitespace gutter between two cards.
	cardGridOpenColGapPt = 24.0
	// cardGridOpenRowGapPt separates a heading, its rule and its body.
	cardGridOpenRowGapPt = 2.0
	// cardGridOpenBandGapPt is the whitespace between two rows of cards.
	cardGridOpenBandGapPt = 18.0
	// The text margins of an open card: the heading stands close on its
	// rule and the body starts close under it, so a card is one element and
	// a grid of twelve keeps the height the tiles needed.
	cardGridOpenHeadTopPt    = 2.0
	cardGridOpenHeadBottomPt = 4.0
	cardGridOpenBodyTopPt    = 5.0
	cardGridOpenBodyBottomPt = 2.0
	// cardGridOpenAirMaxPt is the most air a sparse grid adds under each
	// body and between two rows of cards.
	cardGridOpenAirMaxPt = 8.0
	// cardGridOpenIconPt is the icon zone above a heading that carries one.
	cardGridOpenIconPt = 36.0
)

// cardGridTileStyles are the explicit styles that draw a tile per card: their
// motif (motif.go).
var cardGridTileStyles = map[string]Motif{
	cardGridStyleFilled: MotifTiles, "accent-stripe": MotifTiles, "numbered-badge": MotifTiles,
	"icon-card": MotifTiles, "tinted": MotifTiles, "soft-card": MotifTiles,
}

// cardGridStyle is the effective overrides.style. The default is the open
// card; a deck that asks for a card surface (card_fill, a border) or attaches
// a secondary chart to a card is asking for a tile, so those keep the filled
// look the default used to be.
func cardGridStyle(vals *CardGridValues, ovr *CardGridOverrides) string {
	if ovr.Style != "" {
		return ovr.Style
	}
	if ovr.CardFill != "" || ovr.LineColor != "" || ovr.LineWidth > 0 || ovr.Border != "" {
		return cardGridStyleFilled
	}
	for _, cell := range vals.Cells {
		if cell.Secondary != nil {
			return cardGridStyleFilled
		}
	}
	return cardGridStyleOpen
}

// cardGridOpenText is one open text cell: unfilled, left-aligned, with the
// given top and bottom text margins.
func cardGridOpenText(content string, size float64, bold bool, ink, vAlign string, topPt, bottomPt float64) json.RawMessage {
	text := marshalTextObj(cardTextObj{
		Paragraphs:    []cardParagraph{{Content: content, Size: size, Bold: bold, Color: ink, Align: "l"}},
		Align:         "l",
		VerticalAlign: vAlign,
	})
	return withTextInsetSides(withTextInsetSides(text, topPt, "inset_top"), bottomPt, "inset_bottom")
}

// cardGridOpenScales is the type an open grid is set in, largest first:
// heading, body. A tile's text was grown into its padding by the renderer
// (18 / 14pt on a sparse grid); an open card has no padding to grow into, so
// the pattern takes the larger step itself whenever the grid holds it.
var cardGridOpenScales = [][2]float64{{scaleLeadPt, scaleSubheadPt}, {sizeHeaderPt, scaleBodyPt}}

// expandOpen sets the open grid at the largest type step of
// cardGridOpenScales its content area holds; authored sizes stand.
func (c *cardGrid) expandOpen(ctx ExpandContext, vals *CardGridValues, ovr *CardGridOverrides, cellOverrides map[int]any, columns, gridRows, span int, centred bool, baseAccent string, headerSize, bodySize float64) *jsonschema.ShapeGridInput {
	scales := cardGridOpenScales
	if ovr.HeaderSize > 0 || ovr.BodySize > 0 {
		scales = [][2]float64{{headerSize, bodySize}}
	}
	_, areaH := sizingAreaPt(ctx)
	var (
		grid *jsonschema.ShapeGridInput
		need float64
		at   [2]float64
	)
	for _, sc := range scales {
		at = sc
		grid, need = c.expandOpenAt(ctx, vals, ovr, cellOverrides, columns, gridRows, span, centred, baseAccent, sc[0], sc[1], 0)
		if need <= areaH && !c.openLinesTight(ctx, vals, columns, sc[0], sc[1]) {
			break
		}
	}
	// A grid with height to spare takes air under each body and between its
	// rows of cards, up to the padding a tile had: the block keeps the
	// footprint of the tiles it replaces instead of shrinking to a strip.
	if air := math.Min((areaH-need)/float64(2*gridRows), cardGridOpenAirMaxPt); air >= 1 {
		grid, _ = c.expandOpenAt(ctx, vals, ovr, cellOverrides, columns, gridRows, span, centred, baseAccent, at[0], at[1], math.Floor(air))
	}
	return grid
}

// cardGridOpenWrapSlack is the share of its width a one-line heading or body
// must leave free: a renderer whose face runs wider wraps a line set closer
// than that and shrinks it, and one row of cards then reads as two sizes
// (the sibling-size finding).
const cardGridOpenWrapSlack = 0.12

// openLinesTight reports whether any card's heading or body would be a single
// line within cardGridOpenWrapSlack of its column at these sizes: the larger
// type step is not taken at that price.
func (c *cardGrid) openLinesTight(ctx ExpandContext, vals *CardGridValues, columns int, headerSize, bodySize float64) bool {
	font := ctx.Theme.BodyFont
	contentW, _ := contentAreaPt(ctx)
	textW := equalColumnWidthPt(contentW, columns, ctx.Gap(cardGridOpenColGapPt)) - 2*defaultShapeInsetLRPt
	tight := func(text string, bold bool, size float64) bool {
		return measuredLines(text, font, bold, size, textW) == 1 &&
			measuredLines(text, font, bold, size, textW*(1-cardGridOpenWrapSlack)) > 1
	}
	for _, cell := range vals.Cells {
		if tight(cell.Header, true, headerSize) || tight(inlineMarkupRe.ReplaceAllString(cell.Body, ""), false, bodySize) {
			return true
		}
	}
	return false
}

// expandOpenAt lays the cards out as open items: per row of cards a heading
// row, a rule row and a body row, and a band of whitespace between two rows
// of cards, with airPt of extra room under each body and between two rows of
// cards. It also returns the height the grid needs at these sizes.
func (c *cardGrid) expandOpenAt(ctx ExpandContext, vals *CardGridValues, ovr *CardGridOverrides, cellOverrides map[int]any, columns, gridRows, span int, centred bool, baseAccent string, headerSize, bodySize, airPt float64) (*jsonschema.ShapeGridInput, float64) {
	none := json.RawMessage(`"none"`)
	colGap := ctx.Gap(cardGridOpenColGapPt)
	font := ctx.Theme.BodyFont
	fonts := ctx.themeFonts()
	contentW, _ := contentAreaPt(ctx)
	cardW := equalColumnWidthPt(contentW, columns, colGap)
	textW := cardW - 2*defaultShapeInsetLRPt
	ruleFill := neutralFillJSON(cardGridOpenRuleInkPct)

	var rows []jsonschema.GridRowInput
	idx := 0
	for r := 0; r < gridRows; r++ {
		inRow := min(columns, len(vals.Cells)-idx)
		heads := make([]*jsonschema.GridCellInput, inRow)
		rules := make([]*jsonschema.GridCellInput, inRow)
		bodies := make([]*jsonschema.GridCellInput, inRow)
		headH, bodyH, anyIcon := 0.0, 0.0, false
		for col := 0; col < inRow; col++ {
			cell := vals.Cells[idx]
			accent := ctx.ResolveCellAccent(baseAccent, idx, ovr.CellAccentMode)
			head := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect", Fill: none, Line: noLine,
				Text: cardGridOpenText(cell.Header, headerSize, true, "dk1", "b", cardGridOpenHeadTopPt, cardGridOpenHeadBottomPt),
			}}
			if cell.Icon != nil {
				head.Shape.Icon = cell.Icon.Resolve(accentInkOnLight(ctx, accent, 3.0), "top")
				anyIcon = true
			}
			rule := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill, Line: noLine}}
			body := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect", Fill: none, Line: noLine,
				Text: cardGridOpenText(pptx.ConvertMarkdownEmphasis(cell.Body), bodySize, false, "dk1", "t", cardGridOpenBodyTopPt, cardGridOpenBodyBottomPt),
			}}
			if co, ok := cellOverrides[idx].(*CardGridCellOverride); ok {
				applyCellTextOverride(head, co)
				applyCellTextOverride(body, co)
				// The heading stays on its rule whatever anchor the body takes.
				head.Shape.Text = withVerticalAlign(head.Shape.Text, "b")
				if co.AccentBar {
					rule.Shape.Fill = accentFillJSON(accent)
					rule.BleedTop = cardGridOpenEmphasisRulePt - cardGridOpenRulePt
				}
			}
			// Never under the written fit: the theme-font model alone can
			// hand the writer a row it stores with an autofit shrink.
			headH = math.Max(headH, math.Max(shapeTextHeightPt(font, head.Shape.Text, textW)+cardGridOpenHeadTopPt+cardGridOpenHeadBottomPt,
				writtenFitHeightPt(fonts, head.Shape.Text, cardW, 0)))
			bodyH = math.Max(bodyH, math.Max(shapeTextHeightPt(font, body.Shape.Text, textW)+cardGridOpenBodyTopPt+cardGridOpenBodyBottomPt,
				writtenFitHeightPt(fonts, body.Shape.Text, cardW, 0)))
			heads[col], rules[col], bodies[col] = head, rule, body
			idx++
		}
		headPt := math.Ceil(headH)
		if anyIcon {
			headPt += cardGridOpenIconPt
		}
		bodyPt := math.Ceil(bodyH) + airPt
		if r > 0 {
			band := ctx.Gap(cardGridOpenBandGapPt) + airPt
			rows = append(rows, jsonschema.GridRowInput{
				Cells:     []*jsonschema.GridCellInput{{ColSpan: columns * span}},
				MinHeight: band, MaxHeight: band,
			})
		}
		rows = append(rows,
			jsonschema.GridRowInput{Cells: cardGridPlaceRow(heads, columns, span, centred), MinHeight: headPt, MaxHeight: headPt},
			jsonschema.GridRowInput{Cells: cardGridPlaceRow(rules, columns, span, centred), MinHeight: cardGridOpenRulePt, MaxHeight: cardGridOpenRulePt},
			jsonschema.GridRowInput{Cells: cardGridPlaceRow(bodies, columns, span, centred), MaxHeight: bodyPt},
		)
	}
	need := float64(len(rows)-1) * cardGridOpenRowGapPt
	for _, row := range rows {
		need += row.MaxHeight
	}
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, columns*span)),
		ColGap:        colGap,
		RowGap:        cardGridOpenRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, need
}
