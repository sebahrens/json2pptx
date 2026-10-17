package generator

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestConsultingMatricesWriteReadableNativeTables(t *testing.T) {
	for _, name := range []string{"raci-matrix", "status-board"} {
		t.Run(name, func(t *testing.T) {
			p, _ := patterns.Default().Get(name)
			var valuesJSON []byte
			if name == "raci-matrix" {
				var rows []any
				for i := 0; i < 10; i++ {
					rows = append(rows, map[string]any{"activity": fmt.Sprintf("Activity %d", i+1), "scores": []any{"A", "R", "C", "I", "-", "I"}})
				}
				valuesJSON, _ = json.Marshal(map[string]any{"roles": []any{"Lead", "PM", "Risk", "Ops", "Legal", "Board"}, "activities": rows})
			} else {
				var rows []any
				for i := 0; i < 10; i++ {
					rows = append(rows, map[string]any{"workstream": fmt.Sprintf("Stream %d", i+1), "statuses": []any{"red", "amber", "green", "amber", "green"}, "trend": "up", "comment": "On plan"})
				}
				valuesJSON, _ = json.Marshal(map[string]any{"columns": []any{"Scope", "Plan", "Cost", "Risk", "Team"}, "rows": rows})
			}
			values := p.NewValues()
			if err := json.Unmarshal(valuesJSON, values); err != nil {
				t.Fatal(err)
			}
			ctx := patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: 11000000, Height: 4500000}}
			grid, err := p.Expand(ctx, values, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			table := grid.Rows[0].Cells[0].Table
			if table == nil {
				t.Fatal("not a native table")
			}
			result, err := GenerateTableXML(table.ToTableSpec(), TableRenderConfig{Bounds: types.BoundingBox{Width: 11000000, Height: int64(grid.Rows[0].MinHeight * 12700)}, DefaultSize: 1200})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range result.Findings {
				if f.Code == patterns.ErrCodeTableRowsTruncated {
					t.Fatalf("matrix loses authored rows: %+v", f)
				}
			}
			if !strings.Contains(result.XML, `sz="1200"`) {
				t.Fatal("native12pt text missing")
			}
			if strings.Contains(result.XML, `sz="1000"`) {
				t.Fatal("matrix shrank below12pt")
			}
			if name == "status-board" && !strings.Contains(result.XML, `val="B71C1C"`) {
				t.Fatal("status dot ink missing")
			}
		})
	}
}
