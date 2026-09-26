package generator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeImageOverlapsText(t *testing.T) {
	frame := types.BoundingBox{X: 10, Y: 20, Width: 100, Height: 100}
	opaque := types.PlaceholderFillStop{Ref: "lt1"}
	transparent := opaque
	transparent.Mods = types.BackgroundColorModifiers{HasAlpha: true, Alpha: 50000}
	for _, tc := range []struct {
		name  string
		ph    types.PlaceholderInfo
		image types.BoundingBox
		want  bool
	}{
		{"overlap", types.PlaceholderInfo{Bounds: frame}, frame, true},
		{"edge touch", types.PlaceholderInfo{Bounds: frame}, types.BoundingBox{X: 110, Y: 20, Width: 100, Height: 100}, false},
		{"separate vertical", types.PlaceholderInfo{Bounds: frame}, types.BoundingBox{X: 10, Y: 200, Width: 100, Height: 100}, false},
		{"empty text frame", types.PlaceholderInfo{}, frame, false},
		{"empty image frame", types.PlaceholderInfo{Bounds: frame}, types.BoundingBox{}, false},
		{"opaque solid", types.PlaceholderInfo{Bounds: frame, FillSolid: true, FillStops: []types.PlaceholderFillStop{opaque}}, frame, false},
		{"opaque gradient", types.PlaceholderInfo{Bounds: frame, FillGradient: true, FillStops: []types.PlaceholderFillStop{opaque, opaque}}, frame, false},
		{"transparent solid", types.PlaceholderInfo{Bounds: frame, FillSolid: true, FillStops: []types.PlaceholderFillStop{transparent}}, frame, true},
		{"transparent gradient stop", types.PlaceholderInfo{Bounds: frame, FillGradient: true, FillStops: []types.PlaceholderFillStop{opaque, transparent}}, frame, true},
		{"unknown fill", types.PlaceholderInfo{Bounds: frame, FillSolid: true}, frame, true},
		{"unknown color", types.PlaceholderInfo{Bounds: frame, FillSolid: true, FillStops: []types.PlaceholderFillStop{{}}}, frame, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NativeImageOverlapsText(tc.ph, []types.BoundingBox{tc.image}); got != tc.want {
				t.Fatalf("overlap=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestNativeImageContrastGenerationReportsOverlaidTitle(t *testing.T) {
	imagePath, err := filepath.Abs("../../tests/quality/evidence/connectors/midnight-blue/powerpoint-slide-4.png")
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := generateSinglePass(context.Background(), GenerationRequest{
		TemplatePath: "../../templates/modern.pptx", OutputPath: filepath.Join(t.TempDir(), "native.pptx"), ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout1", Content: []ContentItem{
			{PlaceholderID: "title", Type: ContentText, Value: "Service performance"},
			{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: imagePath, Fit: "contain"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range result.FitFindings {
		if finding.Code == patterns.ErrCodeTextOverImageUnverified && finding.Path == "/slides/0/content/0" {
			return
		}
	}
	t.Fatalf("native photo layout's populated title falsely treated as verified canvas text: %+v", result.FitFindings)
}

func TestNativeImageContrastRequiresAdvisoryReview(t *testing.T) {
	finding := NativeImageContrastFinding("slides[0].content[1]", "Title")
	if finding.Code != patterns.ErrCodeTextOverImageUnverified || finding.Action != "review" || finding.Fix == nil || !patterns.FixKindIsAdvisory(finding.Fix.Kind) {
		t.Fatalf("unknown image pixels must not claim a measurable contrast or executable color repair: %+v", finding)
	}
}

func TestNativeImageContrastExclusionPreservesTextStyle(t *testing.T) {
	slide := bodySlide("Visible")
	const style = `<a:lvl1pPr><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:defRPr></a:lvl1pPr>`
	slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner = style
	if swaps := enforceTextContrastInSlideExcept(slide, "#FFFFFF", modernLikeTheme(), 0, nil, true, map[int]bool{0: true}); len(swaps) != 0 {
		t.Fatalf("unverified picture text recolored using canvas: %+v", swaps)
	}
	if slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner != style {
		t.Fatal("excluded text style changed")
	}
	if swaps := enforceTextContrastInSlide(slide, "#FFFFFF", modernLikeTheme(), 0, nil, true); len(swaps) != 1 {
		t.Fatalf("ordinary canvas text must still be repaired: %+v", swaps)
	}
}

func TestNativeImageContrastExclusionPreservesInheritedColor(t *testing.T) {
	slide := bodySlide("Visible")
	swaps := enforceInheritedTextContrastExcept(slide, []byte(invertedSectionLayout), []byte(masterWithTx1Body), "#FFFFFF", modernLikeTheme(), 0, parseLayoutColorMapOverride([]byte(invertedSectionLayout)), map[int]bool{0: true})
	if len(swaps) != 0 || slideRunFills(slide)[0] != "" {
		t.Fatalf("image-dependent inherited text must not be recolored against the canvas: %+v %v", swaps, slideRunFills(slide))
	}
	if swaps := enforceInheritedTextContrast(slide, []byte(invertedSectionLayout), []byte(masterWithTx1Body), "#FFFFFF", modernLikeTheme(), 0, parseLayoutColorMapOverride([]byte(invertedSectionLayout))); len(swaps) != 1 {
		t.Fatalf("ordinary inherited canvas text still requires repair: %+v", swaps)
	}
}

func TestNativeImageContrastUsesResolvedFramesAndExplicitBounds(t *testing.T) {
	for _, tc := range []struct {
		name, textID string
		bounds       *types.BoundingBox
		want         int
	}{
		{"native frame", "body", nil, 1},
		{"index alias", "idx:1", nil, 1},
		{"explicit separate bounds", "body", &types.BoundingBox{X: 200, Width: 100, Height: 100}, 0},
		{"explicit overlapping bounds", "body", &types.BoundingBox{X: 50, Width: 100, Height: 100}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := bodySlide("Visible")
			shape := &slide.CommonSlideData.ShapeTree.Shapes[0]
			shape.ShapeProperties.Transform = &transformXML{Extent: extentXML{CX: 100, CY: 100}}
			picture := shapeXML{
				NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Picture"}},
				ShapeProperties:     shapePropertiesXML{Transform: &transformXML{Extent: extentXML{CX: 100, CY: 100}}},
			}
			slide.CommonSlideData.ShapeTree.Shapes = append(slide.CommonSlideData.ShapeTree.Shapes, picture)
			ctx := newSinglePassContext("", nil, nil, false, nil)
			spec := SlideSpec{LayoutID: "slideLayout1", Content: []ContentItem{
				{PlaceholderID: tc.textID, Type: ContentText, Value: "Visible"},
				{PlaceholderID: "Picture", Type: ContentImage, Value: ImageContent{Path: "source.png", Bounds: tc.bounds}},
			}}
			exclusions := ctx.nativeImageContrastExclusions(slide, spec, 0)
			if len(exclusions) != tc.want || len(ctx.fitFindings) != tc.want {
				t.Fatalf("exclusions=%v findings=%v, want %d", exclusions, ctx.fitFindings, tc.want)
			}
			if tc.want > 0 && (ctx.fitFindings[0].Path != "/slides/0/content/0" || !exclusions[0]) {
				t.Fatalf("wrong authored text target: %v %+v", exclusions, ctx.fitFindings)
			}
		})
	}
}
