package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-s1uvj.26: generateDataRow wrote len(row) cells against a
// tblGrid of len(headers) columns, so a short row produced an invalid table.
// Short rows are padded; a row wider than the headers is refused.
func TestGenerateTableXML_RaggedRows(t *testing.T) {
	cell := func(s string) types.TableCell { return types.TableCell{Content: s, ColSpan: 1, RowSpan: 1} }
	config := TableRenderConfig{
		Bounds: types.BoundingBox{X: 914400, Y: 914400, Width: 8229600, Height: 4572000},
		Style:  types.DefaultTableStyle,
	}

	short := &types.TableSpec{
		Headers: []string{"Region", "Revenue", "Growth"},
		Rows:    [][]types.TableCell{{cell("North America"), cell("12.4")}, {cell("Europe"), cell("8.7"), cell("+5%")}},
		Style:   types.DefaultTableStyle,
	}
	result, err := GenerateTableXML(short, config)
	if err != nil {
		t.Fatalf("short row: %v", err)
	}
	for i, tr := range strings.Split(result.XML, "<a:tr ")[1:] {
		if n := strings.Count(tr, "<a:tc>") + strings.Count(tr, "<a:tc "); n != 3 {
			t.Errorf("row %d has %d cells, want 3 (one per grid column)", i, n)
		}
	}

	wide := &types.TableSpec{
		Headers: []string{"Region", "Revenue"},
		Rows:    [][]types.TableCell{{cell("North America"), cell("12.4"), cell("extra")}},
		Style:   types.DefaultTableStyle,
	}
	if _, err := GenerateTableXML(wide, config); err == nil || !strings.Contains(err.Error(), "headers define 2") {
		t.Errorf("wide row: err = %v, want a headers-define-2 error", err)
	}
}
