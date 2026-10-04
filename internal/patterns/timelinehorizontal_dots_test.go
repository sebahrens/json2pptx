package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-r684y: a max_height_pct / bounds cap stretches the rows of
// a timeline to fill the capped area, and a dot grew with its row — 16pt dots
// drawn 55pt across. Each dot cell carries its own height cap, which the grid
// honours whatever the row is stretched to.
func TestTimelineDotsKeepTheirSizeInAStretchedRow(t *testing.T) {
	th := &timelineHorizontal{}
	ctx := processFlowChevronCtx()
	vals := &TimelineHorizontalValues{{Label: "Plan"}, {Label: "Build"}, {Label: "Ship"}, {Label: "Scale"}}
	gridInput, err := th.Expand(ctx, vals, &TimelineHorizontalOverrides{}, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	// As a capped pattern is resolved: bounds of 60% of the content height,
	// rows stretched to fill them.
	gridInput.VerticalAlign = "stretch"
	contentW, contentH := contentAreaPt(ctx)
	res := resolveWrittenGrid(t, gridInput, pptx.RectEmu{CX: int64(contentW * sizingEMUPerPt), CY: int64(contentH * 0.6 * sizingEMUPerPt)})
	dots := 0
	for _, c := range res.Cells {
		if c.ShapeSpec == nil || c.ShapeSpec.Geometry != "ellipse" {
			continue
		}
		dots++
		if got := float64(c.Bounds.CY) / sizingEMUPerPt; got > timelineDotSizePt+0.5 {
			t.Errorf("dot is %.1fpt tall in a stretched row, want the %vpt it is designed at", got, float64(timelineDotSizePt))
		}
	}
	if dots != len(*vals) {
		t.Fatalf("found %d dots, want %d", dots, len(*vals))
	}
}
