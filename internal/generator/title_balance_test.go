package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// widowedTitleWidth finds a title box width at which title ends in a one-word
// last line, measured the way balanceTitleLines measures.
func widowedTitleWidth(t *testing.T, title string, sizeHPt int) int64 {
	t.Helper()
	head := title[:len(title)-len(" exception")]
	for pt := 300.0; pt < 900; pt += 2 {
		w := int64(pt * 12700)
		full, _ := textfit.MeasureRun(title, "Liberation Sans", float64(sizeHPt)/100, w, 0)
		short, _ := textfit.MeasureRun(head, "Liberation Sans", float64(sizeHPt)/100, w, 0)
		if full.Lines == 2 && short.Lines == 1 {
			return w
		}
	}
	t.Fatal("no widowed width for fixture")
	return 0
}

func TestBalanceTitleLines(t *testing.T) {
	const title = "Production use is now the norm across the enterprise, not the exception"
	width := widowedTitleWidth(t, title, 2800)
	params := textfit.Params{WidthEMU: width, FontSizeHPt: 2800, FontName: "Liberation Sans", Paragraphs: []string{title}}
	for _, tc := range []struct {
		name      string
		align     string // paragraph algn
		inherited *template.InheritedTextStyle
		wantL     bool
		wantR     bool
	}{
		{"inherited left", "", &template.InheritedTextStyle{}, false, true},
		{"explicit left", "l", nil, false, true},
		{"inherited centre splits the margin", "", &template.InheritedTextStyle{Align: "ctr"}, true, true},
		{"unknown alignment untouched", "", nil, false, false},
		{"right-aligned untouched", "r", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: title}}}}}}
			if tc.align != "" {
				shape.TextBody.Paragraphs[0].Properties = &paragraphPropertiesXML{Algn: tc.align}
			}
			balanceTitleLines(shape, params, textfit.FitResult{}, &autofitConfig{inherited: tc.inherited})
			props := shape.TextBody.Paragraphs[0].Properties
			gotR := props != nil && props.MarR != nil && *props.MarR > 0
			gotL := props != nil && props.MarL != nil && *props.MarL > 0
			if gotR != tc.wantR || gotL != tc.wantL {
				t.Fatalf("marL set=%v marR set=%v, want %v/%v (%+v)", gotL, gotR, tc.wantL, tc.wantR, props)
			}
			if !gotR {
				return
			}
			margin := int64(*props.MarR)
			if props.MarL != nil {
				margin += int64(*props.MarL)
			}
			m, _ := textfit.MeasureRun(title, "Liberation Sans", 28, width-margin, 0)
			if m.Lines != 2 {
				t.Errorf("balanced title wraps to %d lines, want 2", m.Lines)
			}
		})
	}
}

func TestBalanceTitleLinesSkipsOverflowAndOneLine(t *testing.T) {
	const title = "Short title here"
	params := textfit.Params{WidthEMU: 9000000, FontSizeHPt: 2800, FontName: "Liberation Sans", Paragraphs: []string{title}}
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: title}}}}}}
	balanceTitleLines(shape, params, textfit.FitResult{}, &autofitConfig{inherited: &template.InheritedTextStyle{}})
	if shape.TextBody.Paragraphs[0].Properties != nil {
		t.Error("one-line title was changed")
	}
	balanceTitleLines(shape, params, textfit.FitResult{Overflow: true}, &autofitConfig{inherited: &template.InheritedTextStyle{}})
	if shape.TextBody.Paragraphs[0].Properties != nil {
		t.Error("overflowing title was changed")
	}
}
