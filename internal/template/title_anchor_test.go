package template

import "testing"

// go-slide-creator-pymy7: grid geometry tightens the title band only for
// top-anchored titles, so the anchor has to follow layout → master → "t".
func TestTitleAnchorResolution(t *testing.T) {
	cases := []struct {
		template, layout, want string
	}{
		{"blue-corporate.pptx", "slideLayout2", "t"},  // layout declares anchor="t"
		{"midnight-blue.pptx", "slideLayout2", "ctr"}, // inherited from the master
		{"modern-template.pptx", "slideLayout4", "b"}, // layout overrides a top-anchored master
	}
	for _, tc := range cases {
		layouts := parseBundledLayouts(t, tc.template)
		title := placeholderOn(t, layouts, tc.layout, "title")
		if title.Anchor != tc.want {
			t.Errorf("%s/%s title anchor = %q, want %q", tc.template, tc.layout, title.Anchor, tc.want)
		}
	}
}

func TestParseMasterTitleAnchor(t *testing.T) {
	master := []byte(`<p:sldMaster><p:cSld><p:spTree>` +
		`<p:sp><p:nvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr anchor="b"/></p:txBody></p:sp>` +
		`<p:sp><p:nvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:txBody><a:bodyPr vert="horz" anchor="ctr"/></p:txBody></p:sp>` +
		`</p:spTree></p:cSld></p:sldMaster>`)
	if got := ParseMasterTitleAnchor(master); got != "ctr" {
		t.Errorf("ParseMasterTitleAnchor = %q, want ctr", got)
	}
	if got := ParseMasterTitleAnchor([]byte(`<p:sldMaster/>`)); got != "" {
		t.Errorf("no title placeholder: got %q, want empty", got)
	}
}
