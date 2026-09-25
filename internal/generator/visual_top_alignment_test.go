package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestContainedDiagramFrameSharesContentTop(t *testing.T) {
	bounds := types.BoundingBox{X: 100, Y: 200, Width: 800, Height: 600}
	for _, tc := range []struct {
		name       string
		contentW   float64
		contentH   float64
		wantX      int64
		wantWidth  int64
		wantHeight int64
	}{
		{"wide_chart", 2, 1, 100, 800, 400},
		{"tall_chart", 1, 2, 350, 300, 600},
		{"matched_ratio", 4, 3, 100, 800, 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := containedDiagramFrame(bounds, tc.contentW, tc.contentH)
			if got.X != tc.wantX || got.Y != bounds.Y || got.Width != tc.wantWidth || got.Height != tc.wantHeight {
				t.Errorf("frame = %+v, want x=%d y=%d width=%d height=%d", got, tc.wantX, bounds.Y, tc.wantWidth, tc.wantHeight)
			}
		})
	}
	if got := containedDiagramFrame(bounds, 0, 1); got != bounds {
		t.Errorf("invalid content size changed bounds: %+v", got)
	}
}

func TestTopAlignVisualPlaceholderPreservesNonBodyShapes(t *testing.T) {
	makeShape := func(kind string, withBody bool) *shapeXML {
		shape := &shapeXML{NonVisualProperties: nonVisualPropertiesXML{NvPr: nvPrXML{Placeholder: &placeholderXML{Type: kind}}}}
		if withBody {
			shape.TextBody = &textBodyXML{}
		}
		return shape
	}
	body := makeShape("body", true)
	topAlignVisualPlaceholder(body)
	if body.TextBody.BodyProperties == nil || body.TextBody.BodyProperties.Anchor != "t" {
		t.Errorf("body anchor = %+v, want top", body.TextBody.BodyProperties)
	}
	title := makeShape("title", true)
	title.TextBody.BodyProperties = &bodyPropertiesXML{Anchor: "ctr"}
	topAlignVisualPlaceholder(title)
	if title.TextBody.BodyProperties.Anchor != "ctr" {
		t.Error("title anchor changed")
	}
	topAlignVisualPlaceholder(makeShape("body", false))
}

func TestPopulatedSiblingBodyControlsTableTopAlignment(t *testing.T) {
	makeBody := func(name string, x int64) shapeXML {
		return shapeXML{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: name},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
			},
			ShapeProperties: shapePropertiesXML{Transform: &transformXML{
				Offset: offsetXML{X: x, Y: 1000000}, Extent: extentXML{CX: 3500000, CY: 4000000},
			}},
		}
	}
	slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{
		makeBody("body", 0), makeBody("body_2", 4000000),
	}}}}
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateSlideData[1] = slide
	ctx.slideContentMap[1] = SlideSpec{LayoutID: "slideLayout3", Content: []ContentItem{
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Point"}},
		{PlaceholderID: "body_2", Type: ContentTable},
	}}
	if !ctx.hasPopulatedSiblingBody(1, 1) {
		t.Fatal("populated side-by-side table should top-align")
	}
	tableItem := ContentItem{PlaceholderID: "body_2", Type: ContentTable, Value: &types.TableSpec{
		Headers: []string{"Team", "Status"},
		Rows:    [][]types.TableCell{{{Content: "Alpha"}, {Content: "Ready"}}},
		Style:   types.DefaultTableStyle,
	}}
	ctx.processTableContent(1, 1, tableItem, &slide.CommonSlideData.ShapeTree.Shapes[1], 1, nil)
	if len(ctx.tableInserts[1]) != 1 {
		t.Fatalf("paired table inserts = %d, want 1", len(ctx.tableInserts[1]))
	}
	if y, _ := frameGeometry(t, ctx.tableInserts[1][0].graphicFrameXML); y != 1000000 {
		t.Errorf("paired table starts at y=%d, want body top 1000000", y)
	}
	ctx.slideContentMap[1] = SlideSpec{LayoutID: "slideLayout3", Content: []ContentItem{{PlaceholderID: "body_2", Type: ContentTable}}}
	if ctx.hasPopulatedSiblingBody(1, 1) {
		t.Error("lone table should retain centered placement")
	}
	ctx.processTableContent(1, 0, tableItem, &slide.CommonSlideData.ShapeTree.Shapes[1], 1, nil)
	if len(ctx.tableInserts[1]) != 2 {
		t.Fatalf("lone table inserts = %d, want 2", len(ctx.tableInserts[1]))
	}
	if y, _ := frameGeometry(t, ctx.tableInserts[1][1].graphicFrameXML); y <= 1000000 {
		t.Errorf("lone short table starts at y=%d, want centered below body top", y)
	}
	slide.CommonSlideData.ShapeTree.Shapes[1].ShapeProperties.Transform.Offset.X = 0
	slide.CommonSlideData.ShapeTree.Shapes[1].ShapeProperties.Transform.Offset.Y = 6000000
	ctx.slideContentMap[1] = SlideSpec{LayoutID: "slideLayout3", Content: []ContentItem{
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Point"}},
		{PlaceholderID: "body_2", Type: ContentTable},
	}}
	if ctx.hasPopulatedSiblingBody(1, 1) {
		t.Error("stacked placeholders should not be treated as side by side")
	}
}
