package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

func TestGridReadabilityAttributesSharedShrinkToCause(t *testing.T) {
	ctx := &singlePassContext{}
	ctx.viewingMode = tokens.ViewingModeReport
	affected := "/slides/0/shape_grid/rows/0/cells/0/shape/text"
	cause := "/slides/0/shape_grid/rows/0/cells/1/shape/text"
	ctx.reportGridReadability([][]byte{[]byte(slideWithAutofit("90000", "1200"))},
		[]RawShapeSource{{Path: affected, SharedAutofitSources: []string{cause}, SharedAutofitOriginalSizes: []int{1200}}},
		map[uint32][]tokens.TextRole{1: {tokens.TextRoleCardTitle}}, 0)
	if len(ctx.fitFindings) != 1 {
		t.Fatalf("expected one title refusal: %+v", ctx.fitFindings)
	}
	f := ctx.fitFindings[0]
	if f.Source != cause || !strings.Contains(f.Message, affected) || !strings.Contains(f.Message, "shared autofit") || f.Action != "refuse" {
		t.Fatalf("wrong shared-shrink attribution: %+v", f)
	}
}

func TestGridReadabilityBakedKPIAndAlreadySmallSource(t *testing.T) {
	for _, original := range []int{2400, 1600} {
		ctx := &singlePassContext{}
		ctx.viewingMode = tokens.ViewingModeReport
		ctx.reportGridReadability([][]byte{[]byte(slideWithAutofit("100000", "1700"))}, []RawShapeSource{{Path: "affected", SharedAutofitSources: []string{"cause"}, SharedAutofitOriginalSizes: []int{original}}}, map[uint32][]tokens.TextRole{1: {tokens.TextRoleKPIValue}}, 0)
		if len(ctx.fitFindings) != 1 {
			t.Fatalf("%d: %+v", original, ctx.fitFindings)
		}
		want := "affected"
		if original == 2400 {
			want = "cause"
		}
		if ctx.fitFindings[0].Source != want {
			t.Fatalf("original %d attribution: %+v", original, ctx.fitFindings[0])
		}
	}
}
