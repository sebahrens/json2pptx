package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeColumnHeadingReservesTextAndMediaBounds(t *testing.T) {
	ctx := &singlePassContext{}
	for _, kind := range []ContentType{ContentText, ContentBullets, ContentDiagram, ContentTable, ContentImage} {
		t.Run(string(kind), func(t *testing.T) {
			body := columnShape("body", 1000000)
			body.NonVisualProperties.NvPr.Placeholder = &placeholderXML{Type: "body"}
			slide := &slideXML{}
			slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{body}
			before := *body.ShapeProperties.Transform
			if err := ctx.placeExplicitColumnHeadings(slide, []ContentItem{{PlaceholderID: "body", Type: kind, Heading: "Only this column"}}, ""); err != nil {
				t.Fatal(err)
			}
			shapes := slide.CommonSlideData.ShapeTree.Shapes
			if len(shapes) != 2 || shapes[0].ShapeProperties.Transform.Offset.Y != before.Offset.Y+NativeColumnHeadingReserve("Only this column", before.Extent.CX) || shapes[0].ShapeProperties.Transform.Extent.CY != before.Extent.CY-NativeColumnHeadingReserve("Only this column", before.Extent.CX) {
				t.Fatalf("heading did not reserve media/text area: %+v", shapes)
			}
			if shapes[1].TextBody.Paragraphs[0].Runs[0].Text != "Only this column" || shapes[1].TextBody.Paragraphs[0].Runs[0].RunProperties.Bold != "1" {
				t.Fatal("explicit heading lost its style")
			}
		})
	}
}

func TestNativeStickerClearsTitleAndTracker(t *testing.T) {
	ctx := &singlePassContext{SlideContext: SlideContext{slideWidth: 12192000, slideHeight: 6858000}}
	title := columnShape("title", 600000)
	title.NonVisualProperties.NvPr.Placeholder = &placeholderXML{Type: "title"}
	title.ShapeProperties.Transform.Offset.Y = 500000
	tracker := columnShape("Section Tracker", 600000)
	tracker.ShapeProperties.Transform.Offset.Y = 300000
	tracker.ShapeProperties.Transform.Extent.CY = 120000
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{title, tracker}
	if err := ctx.placeSticker(slide, "For discussion", ""); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	tag := shapes[len(shapes)-1].ShapeProperties.Transform
	titleBox := shapes[0].ShapeProperties.Transform
	if tag.Offset.Y+tag.Extent.CY > titleBox.Offset.Y {
		t.Fatal("sticker overlaps title")
	}
	trackerBox := shapes[1].ShapeProperties.Transform
	if trackerBox.Offset.Y < tag.Offset.Y+tag.Extent.CY && trackerBox.Offset.Y+trackerBox.Extent.CY > tag.Offset.Y && trackerBox.Offset.X+trackerBox.Extent.CX > tag.Offset.X {
		t.Fatal("sticker overlaps tracker")
	}
	if err := ctx.placeSticker(slide, strings.Repeat("x", 21), ""); err == nil {
		t.Fatal("oversized sticker accepted")
	}
}

func TestNarrowDiagramUsesActualFourThreeSlideWidth(t *testing.T) {
	children := []any{}
	for i := 0; i < 8; i++ {
		children = append(children, map[string]any{"name": "Team"})
	}
	diagram := &types.DiagramSpec{Type: "org_chart", Data: map[string]any{"root": map[string]any{"name": "Owner", "children": children}}}
	const width43 int64 = 9144000
	if finding := CheckDiagramInNarrowBoundsFinding(diagram, 5000000, "/diagram", width43); finding != nil {
		t.Fatalf("55%% of 4:3 slide is not narrow: %+v", finding)
	}
	finding := CheckDiagramInNarrowBoundsFinding(diagram, 4000000, "/diagram", width43)
	if finding == nil || !strings.Contains(finding.Message, "44%") || finding.Allowed.WidthEMU != width43/2 {
		t.Fatalf("wrong 4:3 threshold: %+v", finding)
	}
	if warning := CheckDiagramInNarrowBounds(diagram, 5000000, 1, "body", width43); warning != "" {
		t.Fatalf("string warning disagrees: %s", warning)
	}
}
