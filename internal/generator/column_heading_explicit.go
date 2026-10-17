package generator

import (
	"fmt"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"strings"
)

// ColumnHeadingReserveEMU is a native heading line plus its six-point gap.
const ColumnHeadingReserveEMU int64 = 36 * 12700

// placeExplicitColumnHeadings reserves a shared heading band before either text
// or media population, so charts and images can have the same heading as lists.
func (ctx *singlePassContext) placeExplicitColumnHeadings(slide *slideXML, content []ContentItem, layoutID string) error {
	resolver := newPlaceholderResolver(slide.CommonSlideData.ShapeTree.Shapes, layoutID)
	seen := map[int]bool{}
	for _, item := range content {
		heading := strings.TrimSpace(item.Heading)
		if heading == "" {
			continue
		}
		if len([]rune(heading)) > 60 {
			return fmt.Errorf("column heading holds at most 60 characters")
		}
		idx, _, found := resolver.ResolveWithFallback(item.PlaceholderID)
		if !found || seen[idx] {
			continue
		}
		seen[idx] = true
		original := &slide.CommonSlideData.ShapeTree.Shapes[idx]
		if isTitleShape(original) || original.ShapeProperties.Transform == nil {
			return fmt.Errorf("heading requires a body/body_2 content placeholder")
		}
		bounds := *original.ShapeProperties.Transform
		reserve := NativeColumnHeadingReserve(heading, bounds.Extent.CX, ctx.themeFontName)
		if bounds.Extent.CY <= reserve {
			return fmt.Errorf("column heading leaves no content height")
		}
		original.ShapeProperties.Transform.Offset.Y += reserve
		original.ShapeProperties.Transform.Extent.CY -= reserve
		bounds.Extent.CY = reserve - 6*12700
		shape := shapeXML{}
		var id uint32 = 1
		for _, candidate := range slide.CommonSlideData.ShapeTree.Shapes {
			id = max(id, candidate.NonVisualProperties.ConnectionNonVisual.ID+1)
		}
		shape.NonVisualProperties.ConnectionNonVisual.ID = id
		shape.NonVisualProperties.ConnectionNonVisual.Name = "Column heading " + item.PlaceholderID
		shape.NonVisualProperties.NonVisualShape.TxBox = true
		shape.ShapeProperties.Transform = &bounds

		run := runXML{Text: heading, RunProperties: &runPropertiesXML{FontSize: "1400", Bold: "1", Inner: `<a:solidFill><a:schemeClr val="dk1"/></a:solidFill><a:latin typeface="+mn-lt"/>`}}
		paragraph := paragraphXML{Properties: &paragraphPropertiesXML{Algn: "l", Inner: `<a:buNone/>`}, Runs: []runXML{run}}
		shape.TextBody = &textBodyXML{BodyProperties: &bodyPropertiesXML{Anchor: "t"}, ListStyle: &listStyleXML{}, Paragraphs: []paragraphXML{paragraph}}

		slide.CommonSlideData.ShapeTree.Shapes = append(slide.CommonSlideData.ShapeTree.Shapes, shape)
	}
	return nil
}

// NativeColumnHeadingReserve measures multi-line headings before content gets
// its final rectangle, retaining the ordinary one-line band for short labels.
func NativeColumnHeadingReserve(heading string, width int64, fonts ...string) int64 {
	measured, err := textfit.MeasureRun(heading, tableMeasureFont(fonts), 14, max(int64(1), width-2*91440), 0)
	if err != nil {
		return ColumnHeadingReserveEMU
	}
	return max(ColumnHeadingReserveEMU, measured.RequiredEMU+20*12700)
}
