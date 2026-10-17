package main

import (
	"encoding/json"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/types"
	"strings"
	"testing"
)

func TestRawTableGridErrorsNameAuthoredField(t *testing.T) {
	for _, tc := range []struct{ name, table, field string }{
		{"invalid ink after merged cell", `{"headers":["A","B","C"],"rows":[[{"content":"a","col_span":2},{"content":"b","text_color":"bad"}]]}`, "rows/0/1/text_color"},
		{"invalid cell ink", `{"headers":["A"],"rows":[[{"content":"a","text_color":"garbage"}]]}`, "rows/0/0/text_color"},
		{"negative widths", `{"headers":["A","B"],"rows":[["a","b"]],"column_widths":[1,-1]}`, "column_widths"},
		{"width count", `{"headers":["A","B"],"rows":[["a","b"]],"column_widths":[1]}`, "column_widths"},
		{"excess row cells", `{"headers":["A","B"],"rows":[["a","b","c"]]}`, "rows"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"table_value", "value"} {
				t.Run(key, func(t *testing.T) {
					content := fmt.Sprintf(`{"placeholder_id":"body","type":"table",%q:%s}`, key, tc.table)
					var typed ContentInput
					if err := json.Unmarshal([]byte(content), &typed); err != nil {
						t.Fatal(err)
					}
					_, err := convertPresentationContent([]ContentInput{typed}, 1, "")
					want := "/slides/0/content/0/" + key + "/" + tc.field
					if err == nil || cliErrorPath(err) != want {
						t.Fatalf("generation path=%q, want %q; err=%v", cliErrorPath(err), want, err)
					}
					analysis := &types.TemplateAnalysis{SlideWidth: 12192000, SlideHeight: 6858000, Layouts: []types.LayoutMetadata{{ID: "content", Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderContent, Bounds: types.BoundingBox{Width: 11000000, Height: 4500000}}}}}}
					result := dryRunOutput{Valid: true}
					validateSlidesAgainstTemplate(&result, []SlideInput{{LayoutID: "content", Content: []ContentInput{typed}}}, analysis)
					found := false
					for _, d := range result.Diagnostics {
						if d.Path == want {
							found = true
						}
					}
					if !found {
						t.Fatalf("validation omitted %q: %+v", want, result.Diagnostics)
					}
				})
			}
			var grid ShapeGridInput
			raw := `{"rows":[{"cells":[{"table":` + tc.table + `},{"grid":{"rows":[{"cells":[{"table":` + tc.table + `}]}]}}]}]}`
			if err := json.Unmarshal([]byte(raw), &grid); err != nil {
				t.Fatal(err)
			}
			ds := gridTableRowWidthDiagnostics(&grid, "/slides/0/shape_grid", 0)
			if len(ds) != 2 {
				t.Fatalf("got %d grid findings: %+v", len(ds), ds)
			}
			for _, d := range ds {
				if !strings.HasSuffix(d.Path, "/table/"+tc.field) {
					t.Fatalf("grid path=%q, want field %s", d.Path, tc.field)
				}
			}
		})
	}
}
