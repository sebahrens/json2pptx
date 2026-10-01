package generator

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

const nbsp = " "

const monthNames = `(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|June?|July?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)`

// dateTokenRE matches the date forms a dateline carries: "15 November 2026",
// "November 15, 2026", "November 2026", "Q3 2026" / "H1 FY26".
var dateTokenRE = regexp.MustCompile(`\b(?:\d{1,2} ` + monthNames + `\.? \d{4}|` + monthNames + `\.? \d{1,2}, \d{4}|` + monthNames + `\.? \d{4}|[QH][1-4] (?:FY)?\d{2,4})\b`)

// keepDatesTogether joins the words of each date token with non-breaking
// spaces so a wrap never strands the year on its own line
// (go-slide-creator-9bmaz).
func keepDatesTogether(text string) string {
	return dateTokenRE.ReplaceAllStringFunc(text, func(m string) string {
		return strings.ReplaceAll(m, " ", nbsp)
	})
}

// shapeTextWidth is the frame width available to text after the bodyPr insets
// (OOXML default 0.1in per side).
func shapeTextWidth(shape *shapeXML) int64 {
	w, _ := getShapeDimensions(shape)
	l, r := int64(91440), int64(91440)
	if tb := shape.TextBody; tb != nil && tb.BodyProperties != nil {
		if tb.BodyProperties.LIns != nil {
			l = *tb.BodyProperties.LIns
		}
		if tb.BodyProperties.RIns != nil {
			r = *tb.BodyProperties.RIns
		}
	}
	return w - l - r
}

// subtitleLines measures text in shape's current width at its font size.
func subtitleLines(shape *shapeXML, text, fontName string) int {
	size := extractFontSizeFromShape(shape)
	if size <= 0 {
		size = 2400
	}
	w := shapeTextWidth(shape)
	if w <= 0 {
		return 0
	}
	m, err := textfit.MeasureRun(text, fontName, float64(size)/100, w, 1)
	if err != nil {
		return 0
	}
	return m.Lines
}

// fitTitleSlideSubtitle prepares a title / closing subtitle before it is
// populated: date tokens are kept together, and a subtitle that would wrap in
// a placeholder narrower than the slide's title is widened to the title's
// span (its left edge and width). A subtitle that still wraps reports
// SUBTITLE_WRAPS. Abstract's 3.0in and p-style's 3.3in cover subtitles used
// to wrap "Steering committee read-out | 1 October 2026" to an orphaned
// "2026" (go-slide-creator-9bmaz). Returns content with the rewritten value.
func (ctx *singlePassContext) fitTitleSlideSubtitle(shapes []shapeXML, content []ContentItem, layoutID string, slideIndex int) []ContentItem {
	titleIdx := -1
	for i := range shapes {
		if isTitleShape(&shapes[i]) && shapes[i].ShapeProperties.Transform != nil {
			titleIdx = i
			break
		}
	}
	var out []ContentItem
	for j, item := range content {
		text, ok := item.Value.(string)
		if item.Type != ContentTitleSlideTitle || !ok || text == "" || isTitlePlaceholder(item.PlaceholderID) {
			continue
		}
		if out == nil {
			out = append([]ContentItem(nil), content...)
		}
		text = keepDatesTogether(text)
		out[j].Value = text
		idx, _, found := newPlaceholderResolver(shapes, layoutID).ResolveWithFallback(item.PlaceholderID)
		if !found || idx == titleIdx || shapes[idx].ShapeProperties.Transform == nil {
			continue
		}
		sub := &shapes[idx]
		lines := subtitleLines(sub, text, ctx.themeFontName)
		if lines <= 1 {
			continue
		}
		if titleIdx >= 0 {
			tx := shapes[titleIdx].ShapeProperties.Transform
			if tx.Extent.CX > sub.ShapeProperties.Transform.Extent.CX {
				// Copy: the transform may be shared with resolved layout data.
				widened := *sub.ShapeProperties.Transform
				widened.Offset.X, widened.Extent.CX = tx.Offset.X, tx.Extent.CX
				sub.ShapeProperties.Transform = &widened
				lines = subtitleLines(sub, text, ctx.themeFontName)
			}
		}
		if lines > 1 {
			ctx.emitFitFinding(patterns.SubtitleWraps(slidepath.ContentIndex(slideIndex, j), slideIndex+1, lines))
		}
	}
	if out == nil {
		return content
	}
	return out
}
