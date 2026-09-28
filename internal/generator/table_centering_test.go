package generator

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// frameOffsetRe captures the graphic frame's y offset and height.
var frameOffsetRe = regexp.MustCompile(`<a:off x="\d+" y="(\d+)"/><a:ext cx="\d+" cy="(\d+)"`)

func frameGeometry(t *testing.T, xml string) (y, cy int64) {
	t.Helper()
	m := frameOffsetRe.FindStringSubmatch(xml)
	if m == nil {
		t.Fatalf("no graphic frame geometry in %q", xml)
	}
	y, _ = strconv.ParseInt(m[1], 10, 64)
	cy, _ = strconv.ParseInt(m[2], 10, 64)
	return y, cy
}

func riskTable() *types.TableSpec {
	return &types.TableSpec{
		Headers: []string{"Risk", "Likelihood", "Impact", "Owner", "Mitigation"},
		Rows: [][]types.TableCell{
			{{Content: "Data migration slips"}, {Content: "Medium"}, {Content: "High"}, {Content: "Platform"}, {Content: "Dual-run the batch"}},
			{{Content: "Key supplier delay"}, {Content: "Low"}, {Content: "High"}, {Content: "Procurement"}, {Content: "Second source"}},
			{{Content: "Regulatory review"}, {Content: "Medium"}, {Content: "Medium"}, {Content: "Legal"}, {Content: "Pre-brief"}},
			{{Content: "Attrition in ops"}, {Content: "High"}, {Content: "Medium"}, {Content: "People"}, {Content: "Retention bonus"}},
		},
		Style: types.DefaultTableStyle,
	}
}

// A short table anchors at the top of its content area, directly under the
// title, instead of floating in the middle of the slide.
func TestTableAnchorsAtTopOfPlaceholder(t *testing.T) {
	bounds := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}
	result, err := GenerateTableXML(riskTable(), TableRenderConfig{Bounds: bounds})
	if err != nil {
		t.Fatalf("GenerateTableXML: %v", err)
	}
	y, cy := frameGeometry(t, result.XML)
	if cy >= bounds.Height {
		t.Fatalf("table fills the placeholder (cy=%d, height=%d); this case needs slack", cy, bounds.Height)
	}
	if y != bounds.Y {
		t.Errorf("table top y=%d, want the content-area top %d", y, bounds.Y)
	}
}

// A table that fills its placeholder does not move, and a table rendered
// without bounds is unaffected.
func TestTableCentringLeavesFullTablesAlone(t *testing.T) {
	table := riskTable()
	// Shorter than the rows need, so the height is capped to the bounds and
	// there is no slack to share.
	tight := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 300000}
	result, err := GenerateTableXML(table, TableRenderConfig{Bounds: tight})
	if err != nil {
		t.Fatalf("GenerateTableXML: %v", err)
	}
	y, _ := frameGeometry(t, result.XML)
	if y != tight.Y {
		t.Errorf("a table with no slack moved to y=%d, want %d", y, tight.Y)
	}

	unbounded, err := GenerateTableXML(table, TableRenderConfig{})
	if err != nil {
		t.Fatalf("GenerateTableXML: %v", err)
	}
	if y, _ := frameGeometry(t, unbounded.XML); y != 0 {
		t.Errorf("a table with no bounds moved to y=%d, want 0", y)
	}
}
