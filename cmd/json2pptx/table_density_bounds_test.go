package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func widthDensityGlossary() *jsonschema.TableInput {
	table := &jsonschema.TableInput{Headers: []string{"Term", "Meaning"}, ColumnWidths: []float64{1, 3}}
	for i := 0; i < 9; i++ {
		table.Rows = append(table.Rows, []jsonschema.TableCellInput{{Content: fmt.Sprintf("Term %d", i)}, {Content: strings.Repeat("meaning ", 13)}})
	}
	return table
}

func TestTableDensityUsesResolvedFrameWidths(t *testing.T) {
	table := widthDensityGlossary()
	if got := table.LogicalRowCountAtWidth(11000000); got != 10 {
		t.Fatalf("wide fixture has %d rows, want 10", got)
	}
	if got := table.LogicalRowCountAtWidth(8229600); got <= 10 {
		t.Fatalf("narrow fixture has %d rows, want >10", got)
	}
	encoded, _ := json.Marshal(table)
	for _, width := range []int64{11000000, 8229600} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			wantDense := width == 8229600
			bounds := types.BoundingBox{Width: width, Height: 4500000}
			analysis := &types.TemplateAnalysis{SlideWidth: width, SlideHeight: 5500000, Layouts: []types.LayoutMetadata{{ID: "content", Name: "Content", Tags: []string{"content"}, Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderContent, Bounds: bounds}}}}}
			grid := &ShapeGridInput{Columns: json.RawMessage(`[100]`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Table: table}}}}}
			for _, nested := range []bool{false, true} {
				candidate := grid
				if nested {
					candidate = &ShapeGridInput{Columns: json.RawMessage(`[100]`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Grid: grid}}}}}
				}
				slides := []SlideInput{{LayoutID: "content", ShapeGrid: candidate}, {LayoutID: "content", Content: []ContentInput{{PlaceholderID: "body", Type: "table", Value: encoded}}}}
				output := dryRunOutput{Valid: true}
				validateSlidesAgainstTemplate(&output, slides, analysis)
				counts := [2]int{}
				for _, d := range output.Diagnostics {
					if d.Code == patterns.ErrCodeDensityExceeded {
						if strings.Contains(d.Path, "/slides/0/") {
							counts[0]++
						}
						if strings.Contains(d.Path, "/slides/1/") {
							counts[1]++
						}
					}
				}
				for si, count := range counts {
					if (count > 0) != wantDense {
						t.Errorf("nested=%t slide%d density=%d want dense=%t", nested, si, count, wantDense)
					}
				}
				ctx := patterns.ExpandContext{SlideWidth: width, SlideHeight: 5500000, LayoutBounds: patterns.LayoutBounds{Width: width, Height: 4500000}}
				warnings := collectGridDensityWarnings(candidate, ctx)
				if (len(warnings) > 0) != wantDense {
					t.Errorf("nested=%t MCP density=%v want dense=%t", nested, warnings, wantDense)
				}
			}

			// A narrow region on a wide slide must use its own cell width.
			if width == 11000000 {
				region := &ShapeGridInput{Columns: json.RawMessage(`[50,50]`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Grid: grid}, {Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}}}}}}
				output := dryRunOutput{Valid: true}
				validateSlidesAgainstTemplate(&output, []SlideInput{{LayoutID: "content", ShapeGrid: region}}, analysis)
				dense := false
				for _, d := range output.Diagnostics {
					if d.Code == patterns.ErrCodeDensityExceeded && strings.Contains(d.Path, "/grid/") {
						dense = true
					}
				}
				if !dense {
					t.Error("half-width nested table region must report density")
				}
				ctx := patterns.ExpandContext{SlideWidth: width, SlideHeight: 5500000, LayoutBounds: patterns.LayoutBounds{Width: width, Height: 4500000}}
				if len(collectGridDensityWarnings(region, ctx)) == 0 {
					t.Error("MCP must measure nested table region at half width")
				}
			}
			fitDense := false
			for _, finding := range measureTable(table, "/table", "/table", 0, bounds) {
				if finding.Code == patterns.ErrCodeDensityExceeded && strings.Contains(finding.Message, "logical rows") {
					fitDense = true
				}
			}
			if fitDense != wantDense {
				t.Errorf("fit density=%t want %t", fitDense, wantDense)
			}
			if (len(pipeline.DetectTableDensity(table, "/table", width)) > 0) != wantDense {
				t.Fatal("pipeline density mismatch")
			}
		})
	}
}

func TestTableDensityUsesFittedBoundsOnceWithLayers(t *testing.T) {
	table := widthDensityGlossary()
	grid := &ShapeGridInput{Columns: json.RawMessage(`[100]`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Table: table, Fit: "contain", Layers: []jsonschema.LayerInput{{Name: "overlay", Frame: jsonschema.LayerFrameInput{W: 1, H: 1}, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}}}}}}}}
	frame := pptx.RectEmu{CX: 11000000, CY: 2000000}
	if resolveGridForStructural(grid, &frame, nil, 11000000, 5500000) != nil {
		t.Fatal("table plus layers must be refused by grid validation")
	}
	grid.Rows[0].Cells[0].Layers = nil
	resolved := resolveGridForStructural(grid, &frame, nil, 11000000, 5500000)
	if resolved == nil {
		t.Fatal("grid did not resolve")
	}
	var tableCell *shapegrid.ResolvedCell
	for i := range resolved.Cells {
		if resolved.Cells[i].Kind == shapegrid.CellKindTable {
			tableCell = &resolved.Cells[i]
		}
	}
	if tableCell == nil || tableCell.Bounds.CX != 2000000 || tableCell.CellBounds.CX != 11000000 {
		t.Fatalf("expected square fitted table inside wide frame: %+v", tableCell)
	}
	warnings := collectGridTableDensityResolved(grid, resolved, "/shape_grid", 0)
	if len(warnings) != 1 {
		t.Fatalf("table plus overlay emitted %d density findings, want one: %+v", len(warnings), warnings)
	}
	preflight := collectGridTablePreflightResolved(grid, resolved, "/shape_grid", 0)
	predictedHidden := -1
	for _, finding := range preflight {
		if finding.Code == patterns.ErrCodeTableRowsTruncated {
			if predictedHidden >= 0 {
				t.Fatal("overlay duplicated table truncation")
			}
			predictedHidden = finding.Fix.Params["hidden_rows"].(int)
		}
	}
	_, rendered, err := generateTableCell(*tableCell, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	actualHidden := -1
	for _, finding := range rendered {
		if finding.Code == patterns.ErrCodeTableRowsTruncated {
			actualHidden = finding.Fix.Params["hidden_rows"].(int)
		}
	}
	if predictedHidden <= 0 || predictedHidden != actualHidden {
		t.Fatalf("fitted table preflight hides %d rows, renderer hides %d", predictedHidden, actualHidden)
	}
}
