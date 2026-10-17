package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestMeasuredMatricesSurviveSemanticPipeline(t *testing.T) {
	for _, template := range []string{"midnight-blue", "p-style"} {
		for _, kind := range []string{"raci", "status_board"} {
			t.Run(template+"/"+kind, func(t *testing.T) {
				if template == "p-style" {
					if os.Getenv("JSON2PPTX_SKIP_LOCAL_TEMPLATES") == "1" {
						t.Skip("optional local templates disabled")
					}
					info, err := os.Stat(filepath.Join(testutil.TemplatesDir(), "p-style.pptx"))
					if os.IsNotExist(err) {
						t.Skip("optional local p-style template is absent")
					}
					if err != nil {
						t.Fatal(err)
					}
					if !info.Mode().IsRegular() {
						t.Fatal("local p-style template is not a regular file")
					}
				}
				item := map[string]any{"kind": kind, "title": "Every workstream has an accountable owner", "takeaway": "Resolve the red workstreams before the next gate."}
				rows := []any{}
				for i := 0; i < 10; i++ {
					if kind == "raci" {
						rows = append(rows, map[string]any{"activity": fmt.Sprintf("Activity %d", i+1), "scores": []any{"A", "R", "C", "I", "-", "I"}})
					} else {
						rows = append(rows, map[string]any{"workstream": fmt.Sprintf("Stream %d", i+1), "statuses": []any{"red", "amber", "green", "amber", "green"}, "trend": "up", "comment": "On plan"})
					}
				}
				if kind == "raci" {
					item["roles"] = []any{"Lead", "PM", "Risk", "Ops", "Legal", "Board"}
					item["activities"] = rows
				} else {
					item["columns"] = []any{"Scope", "Plan", "Cost", "Risk", "Team"}
					item["rows"] = rows
				}
				out := renderDeckSpecCall(t, semanticTestConfig(t), map[string]any{"spec": map[string]any{"meta": map[string]any{"title": "Matrix review", "template": template}, "slides": []any{item}}})
				if !out.Success {
					t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
				}
				for _, d := range out.Diagnostics {
					for _, bad := range []string{"DENSITY_EXCEEDED", "CONTENT_DROPPED", "TEXT_BELOW_READABLE_MIN", "TABLE_ROWS_TRUNCATED"} {
						if strings.Contains(strings.ToUpper(d.Code), bad) {
							t.Errorf("matrix guard failure: %+v", d)
						}
					}
				}
				xml := string(pptxParts(t, out.PptxPath)["ppt/slides/slide1.xml"])
				table := regexp.MustCompile(`(?s)<a:tbl>.*?</a:tbl>`).FindString(xml)
				if table == "" {
					t.Fatal("native table missing")
				}
				for i := 1; i <= 10; i++ {
					label := fmt.Sprintf("Activity %d", i)
					if kind == "status_board" {
						label = fmt.Sprintf("Stream %d", i)
					}
					if !strings.Contains(table, ">"+label+"<") {
						t.Errorf("row lost: %s", label)
					}
				}
				sizes := regexp.MustCompile(`\bsz="([0-9]+)"`).FindAllStringSubmatch(table, -1)
				if len(sizes) == 0 {
					t.Fatal("no explicit native table font sizes")
				}
				for _, m := range sizes {
					size, _ := strconv.Atoi(m[1])
					if size < 1200 {
						t.Errorf("native table text below12pt: %d", size)
					}
				}
			})
		}
	}
}

func TestJSONCannotForgeMeasuredMatrixFloor(t *testing.T) {
	for _, key := range []string{"MinimumFontSize", "minimum_font_size", "minimumFontSize"} {
		var table jsonschema.TableInput
		raw := fmt.Sprintf(`{"headers":["A","B","C","D","E","F","G"],"rows":[["a"]],%q:1200}`, key)
		if err := json.Unmarshal([]byte(raw), &table); err != nil {
			t.Fatal(err)
		}
		if table.MinimumFontSize != 0 {
			t.Fatalf("JSON forged private floor via %s", key)
		}
		if len(pipeline.DetectTableDensity(&table, "/table")) == 0 {
			t.Fatalf("JSON bypassed ordinary table density via %s", key)
		}
	}
}

func TestTableCellInkRespectsConstrainedDesign(t *testing.T) {
	for _, tc := range []struct {
		color string
		want  int
	}{{"#FF0000", 1}, {"FF0000", 1}, {"accent2", 0}, {"", 0}} {
		table := &TableInput{Headers: []string{"Owner"}, Rows: [][]jsonschema.TableCellInput{{{Content: "Finance", TextColor: tc.color}}}}
		fs := checkTableInput(table, 1, "/slides/0/content/0/table_value")
		if len(fs) != tc.want {
			t.Errorf("ink %q: got %+v", tc.color, fs)
		}
		if len(fs) > 0 && fs[0].Path != "/slides/0/content/0/table_value/rows/0/0/text_color" {
			t.Errorf("wrong ink address: %s", fs[0].Path)
		}
	}
}

func TestComposeNativeTableRejectsMissingHeadersAndInvalidInk(t *testing.T) {
	for _, raw := range []string{`{}`, `{"headers":[],"rows":[]}`, `{"headers":["A"],"rows":[[{"content":"a","text_color":"bad"}]]}`} {
		var table TableInput
		if err := json.Unmarshal([]byte(raw), &table); err != nil {
			t.Fatal(err)
		}
		c := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{Table: &table}, composeTextLeaf("Context")}}
		if err := validateCompose(c); err == nil {
			t.Errorf("invalid native table accepted: %s", raw)
		}
	}
}
