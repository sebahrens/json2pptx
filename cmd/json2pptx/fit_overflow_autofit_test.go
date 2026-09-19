package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-lmpu: fit_overflow is the dominant reason a deck fails the
// quality gate, and it fired on cells that render correctly — it compared a
// character count against a single-font-size capacity instead of measuring the
// wrapped block. The project's own showcase decks were refused:
// business-model-canvas scored 62 with 4 refuse findings while every bullet of
// those cells is fully visible in the render.
func TestBundledExamplesHaveNoShapeGridOverflowRefusals(t *testing.T) {
	for _, name := range []string{"business-model-canvas", "varied-pitch-deck"} {
		t.Run(name, func(t *testing.T) {
			input, layouts, w, h := loadExampleForFit(t, name)
			for _, f := range generateFitReport(input, layouts, w, h) {
				if f.Code != patterns.ErrCodeFitOverflow || !strings.Contains(f.Path, "shape_grid") {
					continue
				}
				if strings.Contains(f.Path, "/table/") {
					continue // table cells have their own line-based verdict
				}
				t.Errorf("%s: shape_grid text refused as overflow, but the renderer autofits it: %s @ %s", name, f.Message, f.Path)
			}
		})
	}
}

// A cell whose text cannot fit even at the smallest autofit shrink is still
// reported: the verdict moved from "over a character budget" to "clipped".
func TestShapeGridOverflowStillReportedWhenAutofitBottomsOut(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("word ", 700))
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			ShapeGrid: &ShapeGridInput{
				Columns: json.RawMessage(`1`),
				Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{
					Geometry: "rect",
					Text:     json.RawMessage(`{"content":"` + text + `","size":12}`),
				}}}}},
			},
		}},
	}
	var found *fitFinding
	for _, f := range generateFitReport(input, smallBodyLayouts(), 12192000, 6858000) {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") {
			f := f
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatal("text that clips even at the autofit floor must still be reported")
	}
	if found.Action != "refuse" {
		t.Errorf("action = %q, want refuse — this text is clipped, not merely small", found.Action)
	}
	if !strings.Contains(found.Message, "clipped") {
		t.Errorf("message should say the text is clipped: %q", found.Message)
	}
	if found.Fix == nil || found.Fix.Kind != "reduce_cell_text" {
		t.Errorf("fix = %+v, want reduce_cell_text", found.Fix)
	}
}

// loadExampleForFit parses a bundled example and resolves its template geometry
// the way the CLI and MCP paths do.
func loadExampleForFit(t *testing.T, name string) (*PresentationInput, []types.LayoutMetadata, int64, int64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	applyDefaults(&input)
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", input.Template+".pptx"))
	if err != nil {
		t.Fatalf("open template %s: %v", input.Template, err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	w, h := template.ParseSlideDimensions(reader)
	return &input, layouts, w, h
}
