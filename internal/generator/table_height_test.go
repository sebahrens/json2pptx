package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestMeasureTableHeightUsesWrappedColumnPlan(t *testing.T) {
	table := &types.TableSpec{Headers: []string{"Term", "Meaning"}, ColumnWidths: []float64{1, 3}, Rows: [][]types.TableCell{{{Content: "SLA"}, {Content: strings.Repeat("Service level agreement requires a recorded owner. ", 4)}}}}
	wide := MeasureTableHeight(table, 700*12700, 0)
	narrow := MeasureTableHeight(table, 300*12700, 0)
	if wide <= 0 || narrow <= wide {
		t.Fatalf("wrapped native rows unmeasured wide%d narrow%d", wide, narrow)
	}
	if MeasureTableHeight(nil, 300*12700, 0) != 0 {
		t.Fatal("nil table acquired height")
	}
}
