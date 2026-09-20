package template

import "testing"

// TestParseMasterBodyStyle pins the body-style parse the measured fit depends
// on: a master that declares spacing and line height without a size must still
// yield them, because the spacing alone changes the predicted font scale
// (go-slide-creator-nlrg).
func TestParseMasterBodyStyle(t *testing.T) {
	master := []byte(`<p:sldMaster xmlns:p="p" xmlns:a="a"><p:txStyles>
		<p:titleStyle><a:lvl1pPr><a:defRPr sz="4400"/></a:lvl1pPr></p:titleStyle>
		<p:bodyStyle><a:lvl1pPr>
			<a:lnSpc><a:spcPct val="94000"/></a:lnSpc>
			<a:spcBef><a:spcPts val="800"/></a:spcBef>
			<a:defRPr sz="2000"><a:latin typeface="+mn-lt"/></a:defRPr>
		</a:lvl1pPr></p:bodyStyle>
	</p:txStyles></p:sldMaster>`)

	st := ParseMasterBodyStyle(master)
	if st.SizeHPt != 2000 {
		t.Errorf("SizeHPt = %d, want 2000", st.SizeHPt)
	}
	if st.SpcBefPt != 8 {
		t.Errorf("SpcBefPt = %.1f, want 8", st.SpcBefPt)
	}
	if st.LineSpacingPct != 94 {
		t.Errorf("LineSpacingPct = %d, want 94", st.LineSpacingPct)
	}
	if st.Typeface != "+mn-lt" {
		t.Errorf("Typeface = %q, want +mn-lt", st.Typeface)
	}

	// It reads the BODY style, not the title's.
	if title := ParseMasterTitleStyle(master); title.SizeHPt != 4400 {
		t.Errorf("title SizeHPt = %d, want 4400 — the two parsers must not cross", title.SizeHPt)
	}

	if got := ParseMasterBodyStyle([]byte("not xml")); got != (InheritedTextStyle{}) {
		t.Errorf("unparseable master should yield the zero style, got %+v", got)
	}
}
