package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// go-slide-creator-vxnfu: a mixed-case title that wraps under a display
// layout's tight leading is measured and written at the wrapped-title floor;
// a one-line title and an all-caps title keep the template's leading.
func TestWrappedMixedCaseTitleTakesLeadingFloor(t *testing.T) {
	const closing = "Approve the $4.2M launch budget to start the beta in April"
	// A closing title box as display layouts ship it: 66pt at 80% leading,
	// 7.25in x 1.62in.
	in := TitleFitInput{
		Title: closing, WidthEMU: 6629400, HeightEMU: 1482725, FontName: "Arial",
		Style: template.InheritedTextStyle{SizeHPt: 6600, LineSpacingPct: 80},
	}
	res, err := MeasureTitleFit(in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.LineSpacingRaised {
		t.Fatalf("wrapped mixed-case title at 80%% leading was not raised to the floor: %+v", res)
	}
	if res.Overflow || res.LnSpcReduction != 0 {
		t.Errorf("the title should shrink to its comfort size rather than overflow or compress its leading: %+v", res)
	}
	if got := scaleHPt(6600, res.FontScale); got < titleComfortMaxHPt {
		t.Errorf("fitted size %d hpt is under the %d hpt display comfort size", got, titleComfortMaxHPt)
	}

	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: closing}}}}}}
	bakeTitleFit(shape, titleFitParams(in), res)
	want := fmt.Sprintf(`<a:lnSpc><a:spcPct val="%d"/></a:lnSpc>`, minWrappedTitleLineSpacingPct*1000)
	if got := shape.TextBody.Paragraphs[0].Properties; got == nil || !strings.HasPrefix(got.Inner, want) {
		t.Errorf("baked paragraph properties = %+v, want leading %s", got, want)
	}

	oneLine := in
	oneLine.Title = "Thank you"
	if r, _ := MeasureTitleFit(oneLine); r.LineSpacingRaised || r.NeedsAutofit() {
		t.Errorf("a one-line title keeps the template's leading and size: %+v", r)
	}
	caps := in
	caps.Style.CapsAll = true
	if r, _ := MeasureTitleFit(caps); r.LineSpacingRaised {
		t.Errorf("an all-caps title keeps the template's leading: %+v", r)
	}
	// A template at or above the floor is measured exactly as before.
	loose := in
	loose.Style.LineSpacingPct = 90
	withFloor := titleFitParams(loose)
	without := withFloor
	without.WrappedLineSpacing = 0
	a, _ := textfit.Calculate(withFloor)
	b, _ := textfit.Calculate(without)
	if a != b {
		t.Errorf("90%% template leading: fit changed with the floor: %+v vs %+v", a, b)
	}
}

// A reduced mixed-case title is floored at the wrapped-title leading, not at
// the all-caps collision floor.
func TestBakeTitleFitMixedCaseReductionFloor(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "a"}}}}}}
	p := textfit.Params{FontSizeHPt: 4400, LineSpacing: baseLineSpacing * 0.9, WrappedLineSpacing: WrappedTitleLineSpacing(false)}
	bakeTitleFit(shape, p, textfit.FitResult{FontScale: 60000, LnSpcReduction: 20000})
	if inner := shape.TextBody.Paragraphs[0].Properties.Inner; !strings.HasPrefix(inner, `<a:lnSpc><a:spcPct val="90000"/></a:lnSpc>`) {
		t.Errorf("baked lnSpc inner = %s", inner)
	}
}
