package patterns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Stacked-box and toc rows are sized against the zone left after the title,
// footer and any takeaway / source band (go-slide-creator-ni71s). A strip
// either writes every cell at or above the 12pt floor, or reports a measured
// BODY_TOO_LONG — never a silent shrink that generation then refuses. The
// areas are the shipped content zones with no band, a takeaway band, and a
// takeaway + source stack.
func TestNumberedStepRowsReadableOrReportedUnderChromeBands(t *testing.T) {
	areas := []struct {
		name string
		w, h float64
	}{
		{"forest-green", 828, 349},
		{"forest-green/takeaway", 828, 281},
		{"forest-green/takeaway+source", 828, 253},
		{"modern/takeaway+source", 851, 215},
		{"modern-template/takeaway+source", 824, 239},
		{"p-style/takeaway+source", 899, 264},
	}
	cases := []struct {
		style          string
		steps, bodyLen int
		// mustFit: inside the documented budget on every area here.
		mustFit bool
	}{
		{"stacked-box", 6, 0, true},
		{"stacked-box", 7, 0, true},
		{"toc", 6, 0, true},
		{"toc", 7, 0, true},
		{"stacked-box", 5, 117, false},
		{"stacked-box", 6, 60, false},
		{"toc", 4, 117, false},
		{"toc", 5, 40, false},
		{"toc", 6, 60, false},
	}
	p := &numberedStepStrip{}
	for _, tc := range cases {
		vals := &NumberedStepStripValues{Style: tc.style}
		for i := 0; i < tc.steps; i++ {
			step := NumberedStepStripStep{Label: fmt.Sprintf("Step %d", i+1)}
			if tc.bodyLen > 0 {
				step.Body = rowFitWords(tc.bodyLen, i)
			}
			vals.Steps = append(vals.Steps, step)
		}
		if err := p.Validate(vals, nil, nil); err != nil {
			t.Fatalf("payload is not legal: %v", err)
		}
		for _, a := range areas {
			t.Run(fmt.Sprintf("%s/%dx%d/%s", tc.style, tc.steps, tc.bodyLen, a.name), func(t *testing.T) {
				ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
				warned := ""
				for _, msg := range p.PostExpandWarnings(ctx, vals, nil) {
					if strings.HasPrefix(msg, ErrCodeBodyTooLong) {
						warned = msg
					}
				}
				grid, err := p.Expand(ctx, vals, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				ApplyGridDefaults(grid)
				shrunk := writtenBelowFloor(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
				if len(shrunk) > 0 && warned == "" {
					t.Errorf("written below the 12pt floor without BODY_TOO_LONG: %s", strings.Join(shrunk, "; "))
				}
				if tc.mustFit && (warned != "" || len(shrunk) > 0) {
					t.Errorf("label-only rows inside the budget do not fit: %q %v", warned, shrunk)
				}
			})
		}
	}
}

// The measured warning is what the over-budget chrome payload reports: the
// rows' need against the band-shortened area, not only a character count.
func TestNumberedStepRowsWarningNamesTheArea(t *testing.T) {
	vals := &NumberedStepStripValues{Style: "stacked-box"}
	for i := 0; i < 5; i++ {
		vals.Steps = append(vals.Steps, NumberedStepStripStep{Label: "Step", Body: rowFitWords(110, i)})
	}
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 828 * 12700, Height: 253 * 12700}}
	var got []string
	for _, w := range (&numberedStepStrip{}).PostExpandWarnings(ctx, vals, nil) {
		if strings.Contains(w, "content area holds about 253pt") {
			got = append(got, w)
		}
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") {
		t.Fatalf("want one measured BODY_TOO_LONG against the 253pt area, got %v", got)
	}
	// Without layout bounds the character budgets remain the contract.
	if w := (&numberedStepStrip{}).PostExpandWarnings(ExpandContext{}, vals, nil); len(w) != 0 {
		t.Fatalf("no layout bounds: want no measured warning, got %v", w)
	}
}
