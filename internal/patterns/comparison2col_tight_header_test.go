package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-0d9xy: a header that sits within a renderer's face slack of
// its box width needs a second line or a smaller size. When a source line or a
// takeaway leaves the rows no room for the second line, both headers step down
// to 14pt together instead of staying at 18pt for a renderer to shrink one of.
func TestComparison2col_TightHeaderStepsDown(t *testing.T) {
	vals := &Comparison2colValues{}
	if err := json.Unmarshal([]byte(`{"headers":["B. Model + 25 interviews (recommended)","C. B plus channel-partner survey"],"rows":[
		{"left":"EUR 320k, four weeks","right":"EUR 410k, five weeks"},
		{"left":"Two interview waves test price durability and share directly","right":"Survey of 40-60 distributors and OEM buyers adds volume confidence"},
		{"left":"Red-flag report and IC pack in week 4, inside the window","right":"IC pack slips one week past the exclusivity window"}]}`), vals); err != nil {
		t.Fatal(err)
	}
	headerPt := func(plan comparisonPlan) string {
		text := string(plan.rows[0].Cells[0].Shape.Text)
		for _, size := range []string{"18", "14"} {
			if strings.Contains(text, `"size":`+size) {
				return size
			}
		}
		return text
	}
	ctx := restraintCtx()
	ctx.LayoutBounds.Width = 500 * 12700 // the long header is within the face slack of its half

	// Find the header's own one-line and two-line needs at the default size.
	ctx.LayoutBounds.Height = 400 * 12700
	roomy := comparisonLayout(ctx, vals, &Comparison2colOverrides{}, nil)
	if !roomy.fits || roomy.tightHeader || headerPt(roomy) != "18" {
		t.Fatalf("with room: fits=%v tight=%v header=%spt, want an 18pt header on two lines", roomy.fits, roomy.tightHeader, headerPt(roomy))
	}

	steppedDown, keptAuthored := false, false
	for h := 400.0; h >= 120; h -= 2 {
		ctx.LayoutBounds.Height = int64(h * 12700)
		plan := comparisonLayout(ctx, vals, &Comparison2colOverrides{}, nil)
		if !plan.fits {
			break
		}
		if plan.tightHeader && headerPt(plan) == "18" {
			// Tight at 18pt is only acceptable when 14pt would not hold it either.
			lower := comparisonMeasure(ctx, vals, &Comparison2colOverrides{}, nil, comparisonMinHeaderPt, scaleSubheadPt, plan.rowGap)
			if lower.fits && !lower.tightHeader {
				t.Fatalf("at %.0fpt the header stays at 18pt on a one-line band although 14pt holds it with room", h)
			}
		}
		if headerPt(plan) == "14" {
			steppedDown = true
			// An authored header size is kept at the same height.
			authored := comparisonLayout(ctx, vals, &Comparison2colOverrides{TextOverrides: TextOverrides{HeaderSize: 18}}, nil)
			keptAuthored = keptAuthored || headerPt(authored) == "18"
		}
	}
	if !steppedDown {
		t.Error("no height made the headers step down to 14pt: the tight-header case was not exercised")
	}
	if steppedDown && !keptAuthored {
		t.Error("an authored header_size of 18 should be kept where the default steps down")
	}
}
