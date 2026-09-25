package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func eyebrowTestTitle(height int64) shapeXML {
	return shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{ID: 7, Name: "Title"},
			NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "title"}},
		},
		ShapeProperties: shapePropertiesXML{Transform: &transformXML{
			Offset: offsetXML{X: 100000, Y: 200000},
			Extent: extentXML{CX: 4000000, CY: height},
		}},
		TextBody: &textBodyXML{
			ListStyle:  &listStyleXML{Inner: `<a:lvl1pPr algn="ctr"><a:defRPr sz="4400"/></a:lvl1pPr>`},
			Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "A clear decision", RunProperties: &runPropertiesXML{FontSize: "4400"}}}}},
		},
	}
}

func TestPlaceTitleEyebrowSeparateTextbox(t *testing.T) {
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrowTestTitle(1800000)}
	// A title placeholder may itself be named "Eyebrow Title" in a custom
	// layout; its name alone must not make it the dedicated eyebrow target.
	slide.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name = "Eyebrow Title"
	ctx := &singlePassContext{}
	if err := ctx.placeTitleEyebrow(slide, "STRATEGY", "title"); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 2 {
		t.Fatalf("got %d shapes, want title plus independent eyebrow", len(shapes))
	}
	title, eyebrow := shapes[0], shapes[1]
	if title.TextBody.Paragraphs[0].Runs[0].Text != "A clear decision" {
		t.Error("title copy was changed")
	}
	if eyebrow.NonVisualProperties.ConnectionNonVisual.ID != 8 || !eyebrow.NonVisualProperties.NonVisualShape.TxBox {
		t.Error("eyebrow is not a uniquely identified textbox")
	}
	if eyebrow.TextBody.Paragraphs[0].Properties.Algn != "ctr" {
		t.Error("eyebrow did not inherit the title's centered alignment")
	}
	if eyebrow.TextBody.Paragraphs[0].Runs[0].RunProperties.FontSize != "1200" {
		t.Error("eyebrow did not get its independent readable size")
	}
	if eyebrow.ShapeProperties.Transform.Offset.Y+eyebrow.ShapeProperties.Transform.Extent.CY >= title.ShapeProperties.Transform.Offset.Y {
		t.Error("eyebrow overlaps title")
	}
	if title.ShapeProperties.Transform.Offset.Y+title.ShapeProperties.Transform.Extent.CY != 200000+1800000 {
		t.Error("reserving the eyebrow band moved the title's bottom edge")
	}
}

func TestPlaceTitleEyebrowUsesDedicatedPlaceholder(t *testing.T) {
	slide := &slideXML{}
	title := eyebrowTestTitle(1800000)
	eyebrow := eyebrowTestTitle(250000)
	eyebrow.NonVisualProperties.ConnectionNonVisual.Name = "Eyebrow Placeholder"
	eyebrow.NonVisualProperties.NvPr.Placeholder.Type = "body"
	eyebrow.ShapeProperties.Transform.Offset.Y = 100000
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{title, eyebrow}
	ctx := &singlePassContext{}
	if err := ctx.placeTitleEyebrow(slide, "STRATEGY", "title"); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 2 || shapes[0].ShapeProperties.Transform.Offset.Y != 200000 {
		t.Error("dedicated eyebrow placeholder should not move or duplicate the title")
	}
	if got := shapes[1].TextBody.Paragraphs[0].Runs[0].Text; got != "STRATEGY" {
		t.Errorf("dedicated eyebrow copy = %q", got)
	}
}

func TestPlaceTitleEyebrowUsesDedicatedTitleTypedPlaceholder(t *testing.T) {
	slide := &slideXML{}
	mainTitle := eyebrowTestTitle(1800000)
	eyebrow := eyebrowTestTitle(250000)
	eyebrow.NonVisualProperties.ConnectionNonVisual = connectionNonVisualXML{ID: 6, Name: "Eyebrow"}
	eyebrow.ShapeProperties.Transform.Offset.Y = 100000
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrow, mainTitle}
	if err := (&singlePassContext{}).placeTitleEyebrow(slide, "STRATEGY", "title"); err != nil {
		t.Fatal(err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 2 || shapes[1].ShapeProperties.Transform.Offset.Y != 200000 {
		t.Fatal("title-typed eyebrow placeholder must not move the separate main title")
	}
	if got := shapes[0].TextBody.Paragraphs[0].Runs[0].Text; got != "STRATEGY" {
		t.Errorf("dedicated title-typed eyebrow copy = %q", got)
	}
	if shapes[0].NonVisualProperties.NvPr.Placeholder != nil || !shapes[0].NonVisualProperties.NonVisualShape.TxBox {
		t.Error("dedicated eyebrow slot must stop advertising itself as the main title")
	}
}

func TestTitleContentDoesNotOverwriteTitleTypedEyebrowSlot(t *testing.T) {
	slide := &slideXML{}
	mainTitle := eyebrowTestTitle(1800000)
	eyebrow := eyebrowTestTitle(250000)
	eyebrow.NonVisualProperties.ConnectionNonVisual = connectionNonVisualXML{ID: 6, Name: "Eyebrow"}
	eyebrow.ShapeProperties.Transform.Offset.Y = 100000
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrow, mainTitle}
	ctx := newSinglePassContext("", nil, nil, false, nil)
	warnings := ctx.populateTextInSlide(slide,
		[]ContentItem{{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "A clear decision"}},
		"slideLayout1", 0, "STRATEGY")
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if got := shapes[0].TextBody.Paragraphs[0].Runs[0].Text; got != "STRATEGY" {
		t.Errorf("eyebrow was overwritten by title: %q", got)
	}
	if got := shapes[1].TextBody.Paragraphs[0].Runs[0].Text; got != "A clear decision" {
		t.Errorf("main title = %q", got)
	}
}

func TestPlaceTitleEyebrowShortTitlePreservesCopyWithWarning(t *testing.T) {
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{eyebrowTestTitle(200000)}
	err := (&singlePassContext{}).placeTitleEyebrow(slide, "STRATEGY", "title")
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("expected short-title warning, got %v", err)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if len(shapes) != 1 || shapes[0].TextBody.Paragraphs[0].Runs[0].Text != "STRATEGY" {
		t.Error("short title should retain the eyebrow as a fallback paragraph")
	}
}

func TestPlaceTitleEyebrowUnresolvedTitlePreservesCopyWithWarning(t *testing.T) {
	slide := &slideXML{}
	title := eyebrowTestTitle(1800000)
	title.ShapeProperties.Transform = nil
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{title}
	err := (&singlePassContext{}).placeTitleEyebrow(slide, "STRATEGY", "title")
	if err == nil || !strings.Contains(err.Error(), "no resolved bounds") {
		t.Fatalf("expected unresolved-bounds warning, got %v", err)
	}
	if got := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.Paragraphs[0].Runs[0].Text; got != "STRATEGY" {
		t.Errorf("fallback eyebrow copy = %q", got)
	}
}

func TestEyebrowAlignmentFallsBackToMasterTitleStyle(t *testing.T) {
	shape := eyebrowTestTitle(1800000)
	shape.TextBody.ListStyle = nil
	master := []byte(`<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:txStyles><p:titleStyle><a:lvl1pPr algn="r"/></p:titleStyle></p:txStyles></p:sldMaster>`)
	if got := eyebrowAlignment(&shape, master); got != "r" {
		t.Errorf("master title alignment = %q, want r", got)
	}
}

func TestPlaceTitleEyebrowWithoutTitleReturnsWarning(t *testing.T) {
	err := (&singlePassContext{}).placeTitleEyebrow(&slideXML{}, "STRATEGY", "blank")
	if err == nil || !strings.Contains(err.Error(), "no title placeholder") {
		t.Fatalf("missing title warning = %v", err)
	}
}

func TestEyebrowReservationPrecedesTitleAutofit(t *testing.T) {
	makeSlide := func() *slideXML {
		slide := &slideXML{}
		title := eyebrowTestTitle(1050000)
		title.ShapeProperties.Transform.Extent.CX = 8000000
		slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{title}
		return slide
	}
	content := []ContentItem{{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "Decision\nEvidence"}}
	plain, withEyebrow := makeSlide(), makeSlide()
	plainCtx := newSinglePassContext("", nil, nil, false, nil)
	eyebrowCtx := newSinglePassContext("", nil, nil, false, nil)
	plainCtx.populateTextInSlide(plain, content, "slideLayout1", 0, "")
	eyebrowCtx.populateTextInSlide(withEyebrow, content, "slideLayout1", 0, "STRATEGY")
	if len(plainCtx.fitFindings) != 0 {
		t.Fatalf("title without an eyebrow unexpectedly overflows: %+v", plainCtx.fitFindings)
	}
	var foundOverflow bool
	for _, finding := range eyebrowCtx.fitFindings {
		if finding.Code == patterns.ErrCodeTitleOverflow {
			foundOverflow = true
		}
	}
	if !foundOverflow {
		t.Errorf("autofit did not measure the title after eyebrow reservation: %+v", eyebrowCtx.fitFindings)
	}
}
