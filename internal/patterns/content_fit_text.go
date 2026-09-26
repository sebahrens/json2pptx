package patterns

import (
	"encoding/json"
	"math"
	"regexp"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// inlineMarkupRe strips inline emphasis tags (<b>…</b>) before measuring.
var inlineMarkupRe = regexp.MustCompile(`<[^>]+>`)

// shapeTextHeightPt estimates the rendered height (points) of a pattern text
// object ({"paragraphs":[{content,size,bold}]} or a plain string) wrapped at
// widthPt (insets already removed).
func shapeTextHeightPt(font string, text json.RawMessage, widthPt float64) float64 {
	var s string
	if json.Unmarshal(text, &s) == nil {
		return textBlockHeightPt(font, widthPt, textParagraph{text: inlineMarkupRe.ReplaceAllString(s, ""), size: 14})
	}
	var obj struct {
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
			Bold    bool    `json:"bold"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(text, &obj) != nil {
		return 0
	}
	paras := make([]textParagraph, 0, len(obj.Paragraphs))
	for _, p := range obj.Paragraphs {
		size := p.Size
		if size <= 0 {
			size = 14
		}
		paras = append(paras, textParagraph{text: inlineMarkupRe.ReplaceAllString(p.Content, ""), size: size, bold: p.Bold})
	}
	return textBlockHeightPt(font, widthPt, paras...)
}

// withVerticalAlign returns text with its top-level vertical_align replaced.
func withVerticalAlign(text json.RawMessage, vAlign string) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(text, &obj) != nil {
		return text
	}
	v, _ := json.Marshal(vAlign)
	obj["vertical_align"] = v
	out, err := json.Marshal(obj)
	if err != nil {
		return text
	}
	return out
}

// sparseTextCentreFrac: text filling less than this share of its box is
// vertically centred instead of hanging top-left in an empty card.
const sparseTextCentreFrac = 0.55

// anchorSparseText centres the text of a card whose estimated text height is
// small relative to the card height; denser cards keep their top anchor.
// A bottom anchor is never a pattern default, only an author's
// cell_overrides vertical_align, so it is kept (go-slide-creator-s1uvj.36).
func anchorSparseText(text json.RawMessage, textHPt, boxHPt float64) json.RawMessage {
	if boxHPt <= 0 || textHPt >= boxHPt*sparseTextCentreFrac {
		return text
	}
	var anchor struct {
		VerticalAlign string `json:"vertical_align"`
	}
	if json.Unmarshal(text, &anchor) == nil && anchor.VerticalAlign == "b" {
		return text
	}
	return withVerticalAlign(text, "ctr")
}

// ContentSizedRowGapPt is the column gap the content-sized row helper assumes
// when it derives the card width from the content area.
const contentSizedRowGapPt = 10.0

// contentSizedRow sizes a row of shape cells to the tallest card's own text
// and vertically centres the text in every card that does not fill its share.
//
// A grid row's height is the row's, not the cell's: every card in a row is as
// tall as the tallest one. Without a max_height the row also stretches to fill
// the content area, so a one-line quote next to a four-line quote sat in the
// top fifth of a tall tinted box (go-slide-creator-pr3g). Sizing the row to its
// content shrinks the box, and centring the sparse cards' text fixes what is
// left.
//
// Returns the row untouched when any cell is not a plain shape (a composite
// cell carries a chart that needs the flex height).
func contentSizedRow(ctx ExpandContext, cells []*jsonschema.GridCellInput, cols int) jsonschema.GridRowInput {
	row := jsonschema.GridRowInput{Cells: cells}
	for _, c := range cells {
		if c == nil || c.Shape == nil || c.Composite != nil {
			return row
		}
	}

	font := ctx.Theme.BodyFont
	contentW, _ := contentAreaPt(ctx)
	cardW := equalColumnWidthPt(contentW, cols, contentSizedRowGapPt)
	textW := cardW - 2*defaultShapeInsetLRPt

	textHs := make([]float64, len(cells))
	cardH := 0.0
	for i, c := range cells {
		textHs[i] = shapeTextHeightPt(font, c.Shape.Text, textW)
		cardH = math.Max(cardH, contentCardHeightPt(textHs[i], cardW, c.Shape.Icon != nil))
	}
	for i, c := range cells {
		if c.Shape.Icon == nil {
			c.Shape.Text = anchorSparseText(c.Shape.Text, textHs[i], cardH-2*defaultShapeInsetTBPt)
		}
	}
	row.MaxHeight = cardH
	return row
}

// headerBandPadPt is extra breathing room inside a header band.
const headerBandPadPt = 12.0

// cardPadPt is the vertical breathing room added to content-sized cards.
const cardPadPt = 20.0

// headerRowPt returns a fixed header-band height: the tallest header's text
// (at sizePt, wrapped at widthPt) plus padding — about 1.2x the line height
// plus insets for a one-line header, instead of a percentage of the slide.
func headerRowPt(font string, headers []string, sizePt, widthPt float64) float64 {
	h := 0.0
	for _, t := range headers {
		h = math.Max(h, textBlockHeightPt(font, widthPt, textParagraph{text: t, size: sizePt, bold: true}))
	}
	if h == 0 {
		h = sizePt * contentLineHeight
	}
	return math.Round(h + 2*defaultShapeInsetTBPt + headerBandPadPt)
}

// contentCardHeightPt returns the height (points) of a content-sized card
// whose text is textHPt tall, adding insets and padding, and — when the card
// carries a top overlay icon — the icon zone. shapegrid caps a default-scale
// top icon on a landscape card at 40% of the card height, so the card is
// solved for that share; square/portrait cards (rare for content-sized
// cards) reserve a width-derived zone instead.
func contentCardHeightPt(textHPt, cardWPt float64, hasTopIcon bool) float64 {
	h := textHPt + 2*defaultShapeInsetTBPt + cardPadPt
	if !hasTopIcon {
		return math.Round(h)
	}
	withIcon := (h + 6) / (1 - 0.4)
	if cardWPt > withIcon*1.2 {
		return math.Round(withIcon)
	}
	return math.Round(h + 0.6*cardWPt + 6)
}
