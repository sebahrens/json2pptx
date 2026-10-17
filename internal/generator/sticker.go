package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// placeSticker uses a separate top-right chrome band. Tracker text is kept to
// its left, and neither rectangle intrudes into the title placeholder.
func (ctx *singlePassContext) placeSticker(slide *slideXML, label, layoutID string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}
	if len([]rune(label)) > 20 {
		return fmt.Errorf("sticker holds at most 20 characters")
	}
	shapes := &slide.CommonSlideData.ShapeTree.Shapes
	titleIndex := -1
	for i := range *shapes {
		if isTitleShape(&(*shapes)[i]) && (*shapes)[i].ShapeProperties.Transform != nil {
			titleIndex = i
			break
		}
	}
	if titleIndex < 0 {
		return fmt.Errorf("sticker requires a title placeholder with resolved bounds")
	}
	title := (*shapes)[titleIndex].ShapeProperties.Transform
	const h int64 = 14 * 12700
	const gap int64 = 4 * 12700
	width, err := textfit.MeasureStyledLineWidth(label, ctx.themeFontName, 9, true)
	if err != nil {
		return fmt.Errorf("measure sticker: %w", err)
	}
	width += 12 * 12700
	right := title.Offset.X + title.Extent.CX
	frame := ctx.chromeFrameForLayout(layoutID, "", false)
	if frame.Content.CX > 0 {
		right = min(right, frame.Content.X+frame.Content.CX)
	}
	x := right - width
	y := title.Offset.Y - gap - h
	if y < 0 {
		reserve := -y
		title.Offset.Y += reserve
		title.Extent.CY -= reserve
		y = 0
	}
	if title.Extent.CY < 14*12700 || x < title.Offset.X {
		return fmt.Errorf("sticker does not fit above the title in the template safe area")
	}
	var id uint32 = 1
	for i := range *shapes {
		candidate := &(*shapes)[i]
		id = max(id, candidate.NonVisualProperties.ConnectionNonVisual.ID+1)
		if candidate.NonVisualProperties.ConnectionNonVisual.Name == "Section Tracker" && candidate.ShapeProperties.Transform != nil {
			box := candidate.ShapeProperties.Transform
			if box.Offset.Y < y+h && box.Offset.Y+box.Extent.CY > y {
				box.Extent.CX = max(int64(1), x-gap-box.Offset.X)
			}
		}
	}
	color := `<a:schemeClr val="tx1"/>`
	if hex := ctx.chromeTextColorForLayout(layoutID, 0); hex != "" {
		color = `<a:srgbClr val="` + strings.TrimPrefix(hex, "#") + `"/>`
	}
	shape := shapeXML{}
	shape.NonVisualProperties.ConnectionNonVisual = connectionNonVisualXML{ID: id, Name: "Sticker"}
	shape.NonVisualProperties.NonVisualShape.TxBox = true
	shape.ShapeProperties.Transform = &transformXML{Offset: offsetXML{X: x, Y: y}, Extent: extentXML{CX: width, CY: h}}
	zero := int64(0)

	run := runXML{Text: label, RunProperties: &runPropertiesXML{FontSize: "900", Bold: "1", Inner: `<a:solidFill>` + color + `</a:solidFill><a:latin typeface="+mn-lt"/>`}}
	para := paragraphXML{Properties: &paragraphPropertiesXML{Algn: "r", Inner: `<a:buNone/>`}, Runs: []runXML{run}}
	shape.TextBody = &textBodyXML{BodyProperties: &bodyPropertiesXML{Wrap: "none", Anchor: "ctr", TIns: &zero, BIns: &zero}, Paragraphs: []paragraphXML{para}}

	*shapes = append(*shapes, shape)
	return nil
}
