package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-vn35f: a columns panel sized its body box from an estimate
// that wrapped bullets at the full text width, so three two-line bullets were
// written into a box a line short and stored a shrink with the slide half
// empty. The box now holds the written body at its declared size.
func TestPanelColumnsBodyBoxHoldsItsBulletsUnshrunk(t *testing.T) {
	bodies := []string{
		"- Map the top twenty processes by volume\n- Interview the process owners\n- Baseline cost and cycle time",
		"- Define the target operating model\n- Select the first three use cases\n- Agree the control framework",
		"- Build on the shared platform\n- Pilot with two business units\n- Measure against the baseline",
	}
	for _, widthPt := range []float64{150, 180, 199, 213, 240} {
		width := int64(widthPt * float64(types.EMUPerPoint))
		for _, body := range bodies {
			h := panelBodyBoxHeight(body, "Arial", width)
			tb := panelBodyText(body, panelBodyFontSize)
			if s := pptx.AutofitScaleFor(tb, pptx.RectEmu{CX: width, CY: h}); s < 1 {
				t.Errorf("%.0fpt panel: body box of %.1fpt stores a %.0f%% shrink for %q", widthPt, float64(h)/float64(types.EMUPerPoint), s*100, body[:30])
			}
		}
	}
}
