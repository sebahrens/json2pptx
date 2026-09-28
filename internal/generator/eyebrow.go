package generator

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// The title's own placeholder may inherit letter spacing, size and color from
// its layout. A separate textbox lets the eyebrow retain a distinct visual
// role even in heavily customized templates.
const eyebrowGapEMU int64 = 6 * 12700

var eyebrowListAlignmentRE = regexp.MustCompile(`<(?:(?:a:)?lvl1pPr)\b[^>]*\balgn="(l|ctr|r|just)"`)

func (ctx *singlePassContext) placeTitleEyebrow(slide *slideXML, eyebrow, layoutID string) error {
	fontSize := eyebrowFontSize(slide)
	return ctx.placeTitleBand(slide, eyebrow, layoutID, fontSize, eyebrowParagraph(eyebrow, fontSize, "l"), "Eyebrow", true)
}

// Section tracker typography (go-slide-creator-r3gsw): 9pt caps with +8%
// letter-spacing in accent1, set 6pt above the title.
const (
	trackerFontSizeHPt = 900
	// trackerSpacingHPt is +8% of the 9pt size, in hundredths of a point.
	trackerSpacingHPt = 72
)

// placeTitleTracker renders chrome.tracker's running section name in the band
// above the title — the same slot an eyebrow uses, with the tracker's smaller,
// letter-spaced type. A slide with an authored eyebrow never reaches here. A
// title too short to give up the band keeps its full height and the tracker is
// skipped: it is optional chrome, and folding it into the title would change
// the title's own measured fit.
func (ctx *singlePassContext) placeTitleTracker(slide *slideXML, section, layoutID string) error {
	return ctx.placeTitleBand(slide, section, layoutID, trackerFontSizeHPt, trackerParagraph(section, "l"), "Section Tracker", false)
}

func trackerParagraph(section string, alignment string) paragraphXML {
	return paragraphXML{
		Properties: &paragraphPropertiesXML{Algn: alignment, Inner: `<a:buNone/>`},
		Runs: []runXML{{
			Text: section,
			RunProperties: &runPropertiesXML{
				Lang:     "en-US",
				FontSize: fmt.Sprintf("%d", trackerFontSizeHPt),
				Caps:     "all",
				Spacing:  fmt.Sprintf("%d", trackerSpacingHPt),
				Inner:    `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:latin typeface="+mn-lt"/>`,
			},
		}},
	}
}

// placeTitleBand places a one-line label in its own textbox directly above the
// title placeholder, shortening the title from the top by the label height plus
// eyebrowGapEMU. A template-authored eyebrow slot takes precedence. When
// foldIntoTitle is set, a title without room keeps the label as a leading
// paragraph (the eyebrow's historical behaviour); otherwise the label is
// dropped.
func (ctx *singlePassContext) placeTitleBand(slide *slideXML, label, layoutID string, fontSize int, paragraph paragraphXML, shapeName string, foldIntoTitle bool) error {
	shapes := &slide.CommonSlideData.ShapeTree.Shapes
	eyebrow := label
	titleCount := 0
	for i := range *shapes {
		if isTitleShape(&(*shapes)[i]) {
			titleCount++
		}
	}

	// A template-authored eyebrow slot takes precedence over synthesized chrome.
	for i := range *shapes {
		shape := &(*shapes)[i]
		if !strings.Contains(strings.ToLower(shape.NonVisualProperties.ConnectionNonVisual.Name), "eyebrow") ||
			shape.ShapeProperties.Transform == nil || shape.NonVisualProperties.NvPr.Placeholder == nil ||
			(isTitleShape(shape) && titleCount == 1) {
			continue
		}
		if shape.TextBody == nil {
			shape.TextBody = &textBodyXML{}
		}
		paragraph.Properties.Algn = eyebrowAlignment(shape, ctx.masterXMLForLayout(layoutID))
		shape.TextBody.Paragraphs = []paragraphXML{paragraph}
		if isTitleShape(shape) {
			// Keep the dedicated slot's authored geometry but remove its title
			// placeholder identity so subsequent "title" content resolves to
			// the actual main title instead of overwriting the eyebrow.
			shape.NonVisualProperties.NvPr.Placeholder = nil
			shape.NonVisualProperties.NonVisualShape.TxBox = true
		}
		return nil
	}

	for i := range *shapes {
		title := &(*shapes)[i]
		if !isTitleShape(title) {
			continue
		}
		if title.ShapeProperties.Transform == nil {
			if !foldIntoTitle {
				return nil
			}
			prependEyebrowParagraph(title, eyebrow, fontSize)
			return fmt.Errorf("title placeholder has no resolved bounds for a separate eyebrow textbox")
		}
		xfrm := title.ShapeProperties.Transform
		fontHeight := int64(fontSize) * 127 // hundredths of a point -> EMU
		reserve := fontHeight + eyebrowGapEMU
		if xfrm.Extent.CY < reserve+fontHeight {
			// Preserve authored copy on unusually short custom layouts, while
			// reporting that the independent band could not be made safely.
			if !foldIntoTitle {
				return nil
			}
			prependEyebrowParagraph(title, eyebrow, fontSize)
			return fmt.Errorf("title placeholder is too short for a separate eyebrow textbox")
		}
		paragraph.Properties.Algn = eyebrowAlignment(title, ctx.masterXMLForLayout(layoutID))
		var nextID uint32
		for j := range *shapes {
			if id := (*shapes)[j].NonVisualProperties.ConnectionNonVisual.ID; id >= nextID {
				nextID = id + 1
			}
		}
		if nextID == 0 {
			nextID = 1
		}
		eyebrowShape := shapeXML{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{ID: nextID, Name: shapeName},
				NonVisualShape:      nonVisualShapeXML{TxBox: true},
			},
			ShapeProperties: shapePropertiesXML{Transform: &transformXML{
				Offset: xfrm.Offset,
				Extent: extentXML{CX: xfrm.Extent.CX, CY: fontHeight},
			}},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Wrap: "none", Anchor: "ctr"},
				Paragraphs:     []paragraphXML{paragraph},
			},
		}
		xfrm.Offset.Y += reserve
		xfrm.Extent.CY -= reserve
		*shapes = append(*shapes, eyebrowShape)
		return nil
	}
	if !foldIntoTitle {
		return nil
	}
	return fmt.Errorf("eyebrow %q has no title placeholder", eyebrow)
}

func eyebrowParagraph(eyebrow string, fontSize int, alignment string) paragraphXML {
	return paragraphXML{
		Properties: &paragraphPropertiesXML{Algn: alignment, Inner: `<a:buNone/>`},
		Runs: []runXML{{
			Text: eyebrow,
			RunProperties: &runPropertiesXML{
				Lang:     "en-US",
				FontSize: fmt.Sprintf("%d", fontSize),
				Bold:     "1",
				Caps:     "all",
				Inner:    `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:latin typeface="+mn-lt"/>`,
			},
		}},
	}
}

func eyebrowAlignment(title *shapeXML, master []byte) string {
	if title.TextBody != nil {
		for _, p := range title.TextBody.Paragraphs {
			if p.Properties != nil && p.Properties.Algn != "" {
				return p.Properties.Algn
			}
		}
		if title.TextBody.ListStyle != nil {
			if m := eyebrowListAlignmentRE.FindStringSubmatch(title.TextBody.ListStyle.Inner); len(m) == 2 {
				return m[1]
			}
		}
	}
	var doc struct {
		Styles struct {
			Title struct {
				Level struct {
					Alignment string `xml:"algn,attr"`
				} `xml:"lvl1pPr"`
			} `xml:"titleStyle"`
		} `xml:"txStyles"`
	}
	if err := xml.Unmarshal(master, &doc); err == nil && doc.Styles.Title.Level.Alignment != "" {
		return doc.Styles.Title.Level.Alignment
	}
	return "l"
}
