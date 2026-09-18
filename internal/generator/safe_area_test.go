package generator

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

func TestClampContentPlaceholdersToChrome(t *testing.T) {
	slide := &slideXML{}
	mk := func(phType string, y, cy int64) shapeXML {
		var s shapeXML
		s.NonVisualProperties.NvPr.Placeholder = &placeholderXML{Type: phType}
		s.ShapeProperties.Transform = &transformXML{Offset: offsetXML{X: 838200, Y: y}, Extent: extentXML{CX: 10515600, CY: cy}}
		return s
	}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{
		mk("title", 365125, 1325563),
		mk("body", 1825625, 4351338), // bottom 6176963 runs into the band
		mk("ftr", 6356350, 365125),
		mk("pic", 6000000, 500000), // starts inside the band: left alone
	}
	frame := template.ChromeFrame{Content: template.ChromeRect{X: 838200, Y: 1738694, CX: 10515600, CY: 3913340}, Fits: true}
	clampContentPlaceholdersToChrome(slide, frame)

	shapes := slide.CommonSlideData.ShapeTree.Shapes
	if got, want := shapes[1].ShapeProperties.Transform.Offset.Y+shapes[1].ShapeProperties.Transform.Extent.CY, frame.Content.Bottom(); got != want {
		t.Errorf("body bottom = %d, want clamped to %d", got, want)
	}
	if shapes[0].ShapeProperties.Transform.Extent.CY != 1325563 {
		t.Error("title must not be clamped")
	}
	if shapes[2].ShapeProperties.Transform.Extent.CY != 365125 {
		t.Error("footer must not be clamped")
	}
	if shapes[3].ShapeProperties.Transform.Extent.CY != 500000 {
		t.Error("placeholder starting below the content limit must not be clamped")
	}

	// A frame that does not fit is a no-op (the band is skipped at render).
	body := mk("body", 1825625, 4351338)
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{body}
	clampContentPlaceholdersToChrome(slide, template.ChromeFrame{Content: frame.Content, Fits: false})
	if slide.CommonSlideData.ShapeTree.Shapes[0].ShapeProperties.Transform.Extent.CY != 4351338 {
		t.Error("non-fitting frame must not clamp")
	}
}

// TestGeneratorChromeFrameUsesTemplateProfile verifies the generator resolves
// the takeaway/source band from the template profile (body column, above the
// master footer placeholders) rather than slide-size percentages.
func TestGeneratorChromeFrameUsesTemplateProfile(t *testing.T) {
	ctx := &singlePassContext{}
	ctx.loadTemplateProfile(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if ctx.profile == nil {
		t.Fatal("expected template profile to load")
	}
	frame := ctx.chromeFrameForLayout("slideLayout2", true, true)
	if frame.Basis != template.ChromeBasisLayout {
		t.Fatalf("basis = %q, want layout", frame.Basis)
	}
	if frame.Takeaway.X != 838200 || frame.Takeaway.X+frame.Takeaway.CX != 838200+10515600 {
		t.Errorf("takeaway x-range = [%d,%d], want body column [838200,11353800]", frame.Takeaway.X, frame.Takeaway.X+frame.Takeaway.CX)
	}
	if frame.Source.Bottom() > 6356350 {
		t.Errorf("source band bottom %d overlaps footer placeholders at 6356350", frame.Source.Bottom())
	}

	// Without a profile the frame falls back to slide-relative margins.
	bare := &singlePassContext{}
	if got := bare.chromeFrameForLayout("slideLayout2", true, false); got.Basis != template.ChromeBasisSlideFallback {
		t.Errorf("no-profile basis = %q, want slide_fallback", got.Basis)
	}
}
