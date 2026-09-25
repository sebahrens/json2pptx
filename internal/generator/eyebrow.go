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
	shapes := &slide.CommonSlideData.ShapeTree.Shapes
	fontSize := eyebrowFontSize(slide)
	paragraph := eyebrowParagraph(eyebrow, fontSize, "l")
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
			prependEyebrowParagraph(title, eyebrow, fontSize)
			return fmt.Errorf("title placeholder has no resolved bounds for a separate eyebrow textbox")
		}
		xfrm := title.ShapeProperties.Transform
		fontHeight := int64(fontSize) * 127 // hundredths of a point -> EMU
		reserve := fontHeight + eyebrowGapEMU
		if xfrm.Extent.CY < reserve+fontHeight {
			// Preserve authored copy on unusually short custom layouts, while
			// reporting that the independent band could not be made safely.
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
				ConnectionNonVisual: connectionNonVisualXML{ID: nextID, Name: "Eyebrow"},
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
