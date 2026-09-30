package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-b7qqg.13: blue-corporate's title inherits b="1" from the
// master titleStyle and spc="300" from the Blank + Title layout lstStyle. Both
// widen every line, so the measured title band needs them.
func TestTitlePlaceholderInheritsBoldAndLetterSpacing(t *testing.T) {
	reader, err := OpenTemplate("../../templates/blue-corporate.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range layouts {
		if l.Name != "Blank + Title" {
			continue
		}
		for _, ph := range l.Placeholders {
			if ph.Type != types.PlaceholderTitle {
				continue
			}
			if !ph.TextBold || ph.CharSpacingHPt != 300 || !ph.TextCaps {
				t.Fatalf("title style = bold %v spc %d caps %v, want bold, spc 300, caps", ph.TextBold, ph.CharSpacingHPt, ph.TextCaps)
			}
			return
		}
	}
	t.Fatal("blue-corporate Blank + Title title placeholder not found")
}

func TestInheritedFromLevelPropsReadsBoldAndSpc(t *testing.T) {
	st := ParseMasterTitleStyle([]byte(`<p:sldMaster xmlns:p="p" xmlns:a="a"><p:txStyles><p:titleStyle><a:lvl1pPr><a:defRPr sz="3200" b="1" spc="-150"/></a:lvl1pPr></p:titleStyle></p:txStyles></p:sldMaster>`))
	if !st.Bold || st.SpcHPt != -150 || st.SizeHPt != 3200 {
		t.Fatalf("parsed %+v", st)
	}
}
