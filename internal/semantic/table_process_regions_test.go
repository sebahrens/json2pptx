package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func processRegionFixture() map[string]any {
	return map[string]any{"kind": "process", "steps": []any{
		map[string]any{"label": "Mail arrives", "description": "Record the message"},
		map[string]any{"label": "System decides", "description": "Route to an owner"},
		map[string]any{"label": "Person handles", "description": "Resolve the case"},
		map[string]any{"label": "Staff judge", "description": "Review the result"},
		map[string]any{"label": "Count", "description": "Track completion"},
	}}
}

func TestProcessRegionsCompileEveryArrangement(t *testing.T) {
	for _, arrangement := range slides.RegionArrangements {
		t.Run(arrangement, func(t *testing.T) {
			body := map[string]any{"takeaway": "Every case reaches a named owner.", "arrangement": arrangement, "regions": []any{processRegionFixture(), map[string]any{"kind": "stat", "value": "32%", "label": "Resolved"}, map[string]any{"kind": "text", "body": "Every message has one accountable owner."}}}
			slide, _ := compileRegionsOK(t, body)
			var found int
			var walk func(*deckinput.ShapeGridInput)
			walk = func(g *deckinput.ShapeGridInput) {
				if g == nil {
					return
				}
				for _, row := range g.Rows {
					for _, cell := range row.Cells {
						if cell == nil {
							continue
						}
						walk(cell.Grid)
						if len(cell.Layers) == 0 {
							continue
						}
						found++
						if len(cell.Layers) != 5 {
							t.Fatalf("process has %d steps", len(cell.Layers))
						}
						encoded, _ := json.Marshal(cell.Layers)
						if !strings.Contains(string(encoded), "05  Count") || !strings.Contains(string(encoded), "Record the message") {
							t.Fatalf("process lost content: %s", encoded)
						}
						if cell.Layers[1].Frame.X <= cell.Layers[0].Frame.X {
							t.Fatal("step numbers do not run in reading order")
						}
						if cell.Layers[1].Frame.Y != cell.Layers[0].Frame.Y {
							t.Fatal("paired steps do not share their row top")
						}
						for _, layer := range cell.Layers {
							var text struct {
								VerticalAlign string `json:"vertical_align"`
							}
							if err := json.Unmarshal(layer.Shape.Text, &text); err != nil {
								t.Fatal(err)
							}
							if text.VerticalAlign != "t" {
								t.Fatalf("process step alignment = %q, want top", text.VerticalAlign)
							}
						}
						if arrangement == "rows" || arrangement == "main_top" || arrangement == "main_bottom" {
							if cell.Layers[4].Frame.Y != 0 {
								t.Fatal("horizontal process wrapped into rows")
							}
						} else if cell.Layers[4].Frame.Y <= cell.Layers[0].Frame.Y {
							t.Fatal("vertical process failed to use its height")
						}
					}
				}
			}
			walk(slide.ShapeGrid)
			if found != 1 {
				t.Fatalf("got %d process strips", found)
			}
		})
	}
}

func TestProcessRegionRejectsInvalidSteps(t *testing.T) {
	cases := map[string]func(map[string]any){
		"too few": func(r map[string]any) { r["steps"] = r["steps"].([]any)[:2] },
		"too many": func(r map[string]any) {
			r["steps"] = append(r["steps"].([]any), map[string]any{"label": "Six"}, map[string]any{"label": "Seven"})
		},
		"blank":            func(r map[string]any) { r["steps"].([]any)[0].(map[string]any)["label"] = " " },
		"wrong label type": func(r map[string]any) { r["steps"].([]any)[0].(map[string]any)["label"] = 3 },
		"long label":       func(r map[string]any) { r["steps"].([]any)[0].(map[string]any)["label"] = strings.Repeat("a", 41) },
		"long description": func(r map[string]any) {
			r["steps"].([]any)[0].(map[string]any)["description"] = strings.Repeat("a", 81)
		},
		"unknown field": func(r map[string]any) { r["steps"].([]any)[0].(map[string]any)["next"] = "loop" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			region := processRegionFixture()
			mutate(region)
			spec := regionsSpec(map[string]any{"takeaway": "Every case reaches a named owner.", "regions": []any{region, map[string]any{"kind": "text", "body": "Review"}}})
			_, _, err := Compile(spec, CompileOptions{Strict: StrictnessStrict})
			if err == nil {
				t.Fatal("invalid process accepted")
			}
		})
	}
}

func TestSemanticTableColumnWidths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		widths any
		valid  bool
	}{
		{"relative", []any{1, 3}, true}, {"fractional", []any{0.25, 0.75}, true},
		{"missing width", []any{1}, false}, {"extra width", []any{1, 2, 3}, false}, {"empty", []any{}, false},
		{"zero", []any{0, 3}, false}, {"negative", []any{-1, 3}, false}, {"string", []any{"1", 3}, false}, {"not array", "1,3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"title": "Terms have precise meanings", "takeaway": "Use these terms consistently.", "headers": []any{"Term", "Meaning"}, "rows": []any{[]any{"Owner", "Person accountable for the result"}}, "column_widths": tc.widths}
			spec := &DeckSpec{Meta: DeckMeta{Title: "Glossary", Template: "midnight-blue"}, Slides: []SlideSpec{{Kind: KindTable, Body: body}}}
			input, _, err := Compile(spec, CompileOptions{Strict: StrictnessStrict})
			if !tc.valid {
				if err == nil {
					t.Fatal("invalid widths accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			table := input.Slides[0].ShapeGrid.Rows[0].Cells[0].Table
			if len(table.ColumnWidths) != 2 || table.ColumnWidths[1]/table.ColumnWidths[0] != 3 {
				t.Fatalf("column widths lost: %v", table.ColumnWidths)
			}
		})
	}
}

func TestTableWidthsAndProcessRegionPublishedSchema(t *testing.T) {
	defs := Schema()["$defs"].(map[string]any)
	table := defs[kindDefName(KindTable)].(map[string]any)["properties"].(map[string]any)
	widths := table["column_widths"].(map[string]any)
	if widths["type"] != "array" || widths["items"].(map[string]any)["exclusiveMinimum"] != 0 {
		t.Fatalf("widths schema = %v", widths)
	}
	variants := regionsSchema()["items"].(map[string]any)["oneOf"].([]any)
	found := false
	for _, raw := range variants {
		v := raw.(map[string]any)
		props := v["properties"].(map[string]any)
		kind := props["kind"].(map[string]any)["const"]
		if kind == slides.RegionTable {
			if _, ok := props["column_widths"]; !ok {
				t.Fatal("table region hides column_widths")
			}
		}
		if kind != slides.RegionProcess {
			continue
		}
		found = true
		steps := props["steps"].(map[string]any)
		if steps["minItems"] != 3 || steps["maxItems"] != 6 {
			t.Fatalf("step bounds = %v", steps)
		}
		item := steps["items"].(map[string]any)
		if item["additionalProperties"] != false {
			t.Fatal("process steps accept dropped fields")
		}
		if properties := item["properties"].(map[string]any); len(properties) != 2 || properties["label"] == nil || properties["description"] == nil {
			t.Fatalf("step fields = %v", properties)
		}
	}
	if !found {
		t.Fatal("process region missing from schema")
	}
}

func TestTableRegionPreservesColumnWidths(t *testing.T) {
	body := map[string]any{"takeaway": "Each term has one meaning.", "regions": []any{
		map[string]any{"kind": "table", "headers": []any{"Term", "Meaning"}, "rows": []any{[]any{"Owner", "Accountable person"}}, "column_widths": []any{1, 3}},
		map[string]any{"kind": "text", "body": "Apply the glossary consistently."},
	}}
	slide, _ := compileRegionsOK(t, body)
	table := slide.ShapeGrid.Rows[0].Cells[0].Table
	if table == nil || len(table.ColumnWidths) != 2 || table.ColumnWidths[1] != 3 {
		t.Fatalf("table region lost widths: %+v", table)
	}
	body["regions"].([]any)[0].(map[string]any)["column_widths"] = []any{1, -1}
	if _, _, err := Compile(regionsSpec(body), CompileOptions{Strict: StrictnessStrict}); err == nil {
		t.Fatal("table region accepted negative width")
	}
}
