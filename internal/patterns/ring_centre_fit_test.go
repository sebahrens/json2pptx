package patterns

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// The centre label keeps room for a renderer's wider face
// (go-slide-creator-t5ndl): at the size ringCentreFit returns, every word
// takes at most ringCentreWordShare of the hole's text width by the theme
// font's metrics. Sized to the full width, "Monthly" in the small ring of a
// 50% region was written "Monthl / y" where the template's face is
// substituted.
func TestRingCentreFitLeavesRoomForAWiderFace(t *testing.T) {
	ctx := cycleRingCtx(828, 349)
	spec := newRingSpec(4).withBand(1, ringBandFrac(ringDefaultThickness, 200))
	stepped := false
	for _, label := range []string{"Monthly", "Quarterly", "Continuous improvement", "Run"} {
		for side := 90.0; side <= 300; side += 5 {
			t.Run(fmt.Sprintf("%s/%.0f", label, side), func(t *testing.T) {
				w, h := ringCentreTextPt(spec, side)
				size, fits := ringCentreFit(ctx, spec, side, label, "")
				if size < shapegrid.MinTextSizePt || size > ringCentreMaxPt {
					t.Fatalf("size %.0fpt outside %.0f–%.0fpt", size, shapegrid.MinTextSizePt, ringCentreMaxPt)
				}
				if !fits {
					return
				}
				if ringBreaksWord(ctx, label, size, w*ringCentreWordShare) {
					t.Errorf("a word of %q at %.0fpt takes more than %.0f%% of the %.0fpt hole", label, size, 100*ringCentreWordShare, w)
				}
				if !ringCentreHolds(ctx, label, size, w, h) {
					t.Errorf("%q at %.0fpt does not hold in %.0f x %.0fpt", label, size, w, h)
				}
				// The model alone would have set it larger in a tight hole.
				if loose, ok := sshFitHubLabel(ctx, label, ringCentreMaxPt, w, h); ok && loose > size {
					stepped = true
				}
			})
		}
	}
	if !stepped {
		t.Error("no hole in the sweep made the label step down for the wider face; the sweep proves nothing")
	}
}
