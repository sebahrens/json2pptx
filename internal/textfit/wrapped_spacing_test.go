package textfit

import "testing"

// go-slide-creator-vxnfu: WrappedLineSpacing replaces a tighter LineSpacing
// for a paragraph that wraps, and only for that paragraph.
func TestWrappedLineSpacing(t *testing.T) {
	base := Params{
		WidthEMU: 6629400, HeightEMU: 9144000, FontSizeHPt: 4000, FontName: "Arial",
		LineSpacing: 0.96,
	}
	oneLine, wrapped := base, base
	oneLine.Paragraphs = []string{"Thank you"}
	wrapped.Paragraphs = []string{"Approve the launch budget to start the paying-customer beta by the end of April"}

	for _, tc := range []struct {
		name       string
		p          Params
		wantRaised bool
	}{{"one line", oneLine, false}, {"wrapped", wrapped, true}} {
		tight, err := MeasureHeight(tc.p)
		if err != nil {
			t.Fatal(err)
		}
		withFloor := tc.p
		withFloor.WrappedLineSpacing = 1.08
		loose, _ := MeasureHeight(withFloor)
		res, _ := Calculate(withFloor)
		if res.LineSpacingRaised != tc.wantRaised {
			t.Errorf("%s: LineSpacingRaised = %v, want %v", tc.name, res.LineSpacingRaised, tc.wantRaised)
		}
		// Lines x size x spacing: the heights stand in the 1.08 / 0.96 ratio
		// (each is rounded up to a whole EMU).
		if diff := loose*96 - tight*108; tc.wantRaised && (diff > 108 || diff < -108) {
			t.Errorf("%s: height %d with the wrapped spacing, %d without; want the 1.08 / 0.96 ratio", tc.name, loose, tight)
		}
		if !tc.wantRaised && loose != tight {
			t.Errorf("%s: height changed from %d to %d though the paragraph does not wrap", tc.name, tight, loose)
		}
		// The bold / tracked measurement path honours it too.
		withFloor.Bold = true
		styledLoose, _ := MeasureHeight(withFloor)
		styledTight := tc.p
		styledTight.Bold = true
		st, _ := MeasureHeight(styledTight)
		if tc.wantRaised == (styledLoose == st) {
			t.Errorf("%s (bold): height %d with the wrapped spacing, %d without", tc.name, styledLoose, st)
		}
	}

	// A wrapped spacing at or under LineSpacing changes nothing.
	noop := wrapped
	noop.WrappedLineSpacing = 0.9
	a, _ := Calculate(noop)
	b, _ := Calculate(wrapped)
	if a != b {
		t.Errorf("a wrapped spacing under LineSpacing changed the fit: %+v vs %+v", a, b)
	}
}
