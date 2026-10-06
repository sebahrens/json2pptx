package template

import (
	"encoding/xml"
	"testing"
)

func TestFirstBulletLevel(t *testing.T) {
	tests := []struct {
		name   string
		master string
		want   int
	}{
		{"first level bulleted", `<p:bodyStyle><a:lvl1pPr><a:buChar char="x"/></a:lvl1pPr><a:lvl2pPr><a:buChar char="x"/></a:lvl2pPr></p:bodyStyle>`, 0},
		{"first level unmarked", `<p:bodyStyle><a:lvl1pPr marL="0"><a:buNone/></a:lvl1pPr><a:lvl2pPr><a:buChar char="x"/></a:lvl2pPr></p:bodyStyle>`, 1},
		{"self-closing level counts as bulleted", `<p:bodyStyle><a:lvl1pPr><a:buNone/></a:lvl1pPr><a:lvl2pPr marL="1"/></p:bodyStyle>`, 1},
		{"every level unmarked", `<p:bodyStyle><a:lvl1pPr><a:buNone/></a:lvl1pPr></p:bodyStyle>`, -1},
		{"no body style", `<p:titleStyle><a:lvl1pPr/></p:titleStyle>`, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstBulletLevel([]byte(tt.master)); got != tt.want {
				t.Errorf("FirstBulletLevel = %d, want %d", got, tt.want)
			}
		})
	}
}

// A deeper list level takes the colour the layout's own list style states for
// it, else the master's for that level, each mapped through the layout's
// colour map; the size recorded is the one the render-time pass reads its
// threshold from.
func TestPlaceholderLevelText(t *testing.T) {
	var shape shapeXML
	layout := `<p:sp xmlns:p="p" xmlns:a="a"><p:txBody><a:lstStyle>` +
		`<a:lvl2pPr><a:defRPr sz="1600" b="1"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl2pPr>` +
		`<a:lvl3pPr><a:defRPr sz="1100"/></a:lvl3pPr>` +
		`</a:lstStyle></p:txBody></p:sp>`
	if err := xml.Unmarshal([]byte(layout), &shape); err != nil {
		t.Fatal(err)
	}
	master := &MasterFontStyles{BodyStyle: map[int]*FontStyle{
		1: {FontColor: "accent1", FontSize: 1800},
		2: {FontColor: "accent2", FontSize: 1400},
		3: {FontColor: "#112233", FontSize: 2400, Bold: true},
		4: {FontSize: 1200},
	}}
	got := placeholderLevelText(&shape, "body", master, map[string]string{"tx1": "lt1"})
	want := map[int]struct {
		color      string
		size       int
		bold       bool
		fromMaster bool
	}{
		1: {"lt1", 1600, true, false},     // the layout states it; mapped through the override
		2: {"accent2", 1100, false, true}, // master colour, judged at the layout level's size
		3: {"#112233", 2400, true, true},  // the layout does not define the level
	}
	if len(got) != len(want) {
		t.Fatalf("got %d levels (%+v), want %d", len(got), got, len(want))
	}
	for _, entry := range got {
		w, ok := want[entry.Level]
		if !ok {
			t.Errorf("unexpected level %d: %+v", entry.Level, entry)
			continue
		}
		if entry.Color != w.color || entry.FontSize != w.size || entry.Bold != w.bold || entry.FromMaster != w.fromMaster {
			t.Errorf("level %d = %+v, want %+v", entry.Level, entry, w)
		}
	}

	// A title placeholder has no deeper master levels to inherit from.
	if levels := placeholderLevelText(&shapeXML{}, "title", master, nil); len(levels) != 0 {
		t.Errorf("title placeholder resolved deeper levels: %+v", levels)
	}
}
