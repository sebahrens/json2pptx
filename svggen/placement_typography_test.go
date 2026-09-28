package svggen

import (
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
)

func TestEmbeddedPlacementTypographyUsesPhysicalSize(t *testing.T) {
	for _, render := range []struct {
		name string
		fn   func(*RequestEnvelope, DrawFunc) (*SVGBuilder, *SVGDocument, error)
	}{
		{"standard", RenderWithHelper},
		{"custom dimensions", func(req *RequestEnvelope, draw DrawFunc) (*SVGBuilder, *SVGDocument, error) {
			return RenderWithHelperDimensions(req, 800, 600, draw)
		}},
	} {
		t.Run(render.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: 150, MinReadablePt: 12},
			}
			b, _, err := render.fn(req, func(b *SVGBuilder, _ *RequestEnvelope) error {
				b.SetFontSize(b.StyleGuide().Typography.SizeSmall).DrawText("Readable label", 100, 100, TextAlignLeft, TextBaselineTop)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			actual := b.MinDrawnFontSize() * placementScale(req.Style, b.Width(), b.Height())
			if math.Abs(actual-12) > 0.1 {
				t.Errorf("displayed label = %.2fpt, want 12pt at 200x150pt placement", actual)
			}
			for _, finding := range b.Findings() {
				if finding.Code == core.FindingTextBelowReadableMin {
					t.Errorf("readable label produced below-minimum finding: %+v", finding)
				}
			}
		})
	}
}

// go-slide-creator-h3x1i: embedded text is held to the native readability
// floor. The placed floor becomes the builder's fitter floor, so a dense
// fitter asking for its 7pt SVG minimum draws at the placed 12pt instead.
func TestEmbeddedPlacementClampsFitterFloor(t *testing.T) {
	req := &RequestEnvelope{
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: 150, MinReadablePt: 12},
	}
	b, _, err := RenderWithHelper(req, func(b *SVGBuilder, _ *RequestEnvelope) error {
		b.SetFontSize(7).DrawText("Dense label", 100, 100, TextAlignLeft, TextBaselineTop)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	actual := b.MinDrawnFontSize() * placementScale(req.Style, b.Width(), b.Height())
	if actual < 12-0.05 {
		t.Errorf("dense label drawn at %.2fpt placed, want >= 12pt", actual)
	}
	for _, finding := range b.Findings() {
		if finding.Code == core.FindingTextBelowReadableMin {
			t.Errorf("clamped label produced a below-minimum finding: %+v", finding)
		}
	}
}

// A diagram that lowers the builder floor itself and draws below the placed
// floor is refused, the same as measured native text below the floor.
func TestEmbeddedPlacementRefusesResidualUndersizedText(t *testing.T) {
	req := &RequestEnvelope{
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: 150, MinReadablePt: 12},
	}
	b, _, err := RenderWithHelper(req, func(b *SVGBuilder, _ *RequestEnvelope) error {
		b.SetMinFontSize(7)
		b.SetFontSize(7).DrawText("Dense label", 100, 100, TextAlignLeft, TextBaselineTop)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, finding := range b.Findings() {
		if finding.Code == core.FindingTextBelowReadableMin {
			found = true
			if finding.Severity != core.SeverityRefuse || finding.Fix == nil || finding.Fix.Kind != "simplify_or_enlarge_diagram" {
				t.Errorf("undersized label finding = %+v", finding)
			}
		}
	}
	if !found {
		t.Errorf("7pt SVG label in a quarter-size placement produced no readability finding: %+v", b.Findings())
	}
}

func TestStandaloneSVGDoesNotApplyPlacementPolicy(t *testing.T) {
	req := &RequestEnvelope{Output: OutputSpec{Width: 800, Height: 600}}
	b, _, err := RenderWithHelper(req, func(b *SVGBuilder, _ *RequestEnvelope) error {
		b.SetFontSize(7).DrawText("Standalone label", 100, 100, TextAlignLeft, TextBaselineTop)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.MinDrawnFontSize() != 7 || len(b.Findings()) != 0 {
		t.Errorf("standalone render changed by embedded-placement policy: size=%v findings=%+v", b.MinDrawnFontSize(), b.Findings())
	}
}

func TestPlacementScaleRejectsNonfiniteDimensions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		style StyleSpec
		w, h  float64
	}{
		{"nan placement", StyleSpec{PlacementWidthPt: math.NaN(), PlacementHeightPt: 150}, 800, 600},
		{"infinite placement", StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: math.Inf(1)}, 800, 600},
		{"nan canvas", StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: 150}, math.NaN(), 600},
		{"zero canvas", StyleSpec{PlacementWidthPt: 200, PlacementHeightPt: 150}, 800, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := placementScale(tc.style, tc.w, tc.h); got != 0 {
				t.Errorf("invalid placement scale = %v, want 0", got)
			}
		})
	}
}
