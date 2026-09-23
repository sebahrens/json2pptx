package patterns

import (
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestFillCappedRows(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 100 * 12700, Height: 300 * 12700}}
	rows := []jsonschema.GridRowInput{
		{MaxHeight: 30},
		{MaxHeight: 1},
		{MaxHeight: 30},
	}
	fillCappedRows(ctx, rows, 8, 0.60, func(i int) bool { return i != 1 })
	if rows[1].MaxHeight != 1 || rows[1].MinHeight != 0 {
		t.Errorf("divider should remain fixed: %+v", rows[1])
	}
	used := rows[0].MaxHeight + rows[1].MaxHeight + rows[2].MaxHeight + 16
	if math.Abs(used-180) > 0.2 {
		t.Errorf("filled block is %.1fpt, want 180pt", used)
	}
	if rows[0].MinHeight != rows[0].MaxHeight || rows[2].MinHeight != rows[2].MaxHeight {
		t.Error("grown rows must be pinned so the resolver cannot redistribute the surplus")
	}
}

func TestFillCappedRowsPreservesDenseAndFlexRows(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 100 * 12700, Height: 300 * 12700}}
	for _, tc := range []struct {
		name string
		rows []jsonschema.GridRowInput
	}{
		{"dense", []jsonschema.GridRowInput{{MaxHeight: 190}}},
		{"flex", []jsonschema.GridRowInput{{MaxHeight: 30}, {}}},
		{"auto", []jsonschema.GridRowInput{{MaxHeight: 30}, {AutoHeight: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := make([]jsonschema.GridRowInput, len(tc.rows))
			copy(before, tc.rows)
			fillCappedRows(ctx, tc.rows, 8, 0.60, func(int) bool { return true })
			for i := range before {
				if tc.rows[i].MaxHeight != before[i].MaxHeight || tc.rows[i].MinHeight != before[i].MinHeight {
					t.Fatalf("row %d changed from %+v to %+v", i, before[i], tc.rows[i])
				}
			}
		})
	}
}
