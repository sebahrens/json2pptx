package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
)

// insetFrame is midnight-blue's content frame: the body column pushed from
// 838200 to 1234200 by the left-edge accent bar and stripe.
func insetFrame() template.ChromeFrame {
	return template.ChromeFrame{
		Canvas:         template.ChromeRect{CX: 12192000, CY: 6858000},
		Content:        template.ChromeRect{X: 1234200, Y: 1825625, CX: 11353800 - 1234200, CY: 4000000},
		SideDecorInset: true,
	}
}

// TestTitleAndFooterFollowEdgeArtInset pins go-slide-creator-oa0ru: when edge
// art moves the content column in, the title and the left footer text move
// with it, so title, content, source and footer share one left edge. The
// title keeps its layout width (title fit is measured against it).
func TestTitleAndFooterFollowEdgeArtInset(t *testing.T) {
	frame := insetFrame()
	slide := &slideXML{}
	var title shapeXML
	title.NonVisualProperties.NvPr.Placeholder = &placeholderXML{Type: "title"}
	title.ShapeProperties.Transform = &transformXML{Offset: offsetXML{X: 838200, Y: 365125}, Extent: extentXML{CX: 10515600, CY: 1325563}}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{title}
	clampContentPlaceholdersToChrome(slide, frame)
	got := slide.CommonSlideData.ShapeTree.Shapes[0].ShapeProperties.Transform
	if got.Offset.X != 1234200 || got.Extent.CX != 10515600 {
		t.Errorf("title at x=%d cx=%d, want x=1234200 with its 10515600 width kept", got.Offset.X, got.Extent.CX)
	}

	dt := &transformXML{Offset: offsetXML{X: 838200, Y: 6356350}, Extent: extentXML{CX: 2743200, CY: 365125}}
	positions := map[string]*transformXML{"type:dt": dt}
	moved := alignFooterWithInsetContent(positions, frame)
	if moved["type:dt"].Offset.X != 1234200 {
		t.Errorf("footer date slot at x=%d, want the content edge 1234200", moved["type:dt"].Offset.X)
	}
	if dt.Offset.X != 838200 {
		t.Error("alignFooterWithInsetContent mutated the cached footer positions")
	}

	// No inset: nothing moves.
	frame.SideDecorInset = false
	if same := alignFooterWithInsetContent(positions, frame); same["type:dt"].Offset.X != 838200 {
		t.Error("footer moved without a side-decor inset")
	}
}

// TestSourceNoteZoneStyle pins the source zone's one style
// (go-slide-creator-cuszt): 9pt italic, the text colour muted to ~60%, left
// aligned with the title and footer text on the content grid.
func TestSourceNoteZoneStyle(t *testing.T) {
	xml := generateSourceNoteShapeInBounds("Company filings", 7, pptx.RectEmu{X: 1234200, Y: 6000000, CX: 10000000, CY: 200000})
	for _, want := range []string{
		`sz="900"`,
		`<a:schemeClr val="` + patterns.SourceNoteScheme + `">`,
		`<a:lumMod val="60000"`,
		`<a:lumOff val="40000"`,
		`algn="l"`,
		`lIns="91440"`,
		`Source: Company filings`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("source note missing %q:\n%s", want, xml)
		}
	}
}
