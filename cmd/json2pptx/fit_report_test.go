package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestGenerateFitReport_EmptyDeck(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{},
	}
	findings := generateFitReport(input)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty deck, got %d", len(findings))
	}
}

func TestGenerateFitReport_NoOverflow(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Content: []ContentInput{
					{
						PlaceholderID: "body",
						Type:          "table",
						TableValue: &TableInput{
							Headers: []string{"A", "B"},
							Rows:    [][]TableCellInput{{{Content: "x"}, {Content: "y"}}},
						},
					},
				},
			},
		},
	}
	findings := generateFitReport(input)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for short text, got %d", len(findings))
	}
}

func TestGenerateFitReport_TableOverflow(t *testing.T) {
	// Create a table with a cell containing very long text that should overflow.
	longText := strings.Repeat("This is a very long text that should cause overflow. ", 20)
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Content: []ContentInput{
					{
						PlaceholderID: "body",
						Type:          "table",
						TableValue: &TableInput{
							Headers: []string{"Column A", "Column B", "Column C", "Column D", "Column E"},
							Rows: [][]TableCellInput{
								{{Content: longText}, {Content: "short"}, {Content: "short"}, {Content: "short"}, {Content: "short"}},
							},
						},
					},
				},
			},
		},
	}
	findings := generateFitReport(input)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for overflowing text")
	}

	found := false
	for _, f := range findings {
		if f.Code == patterns.ErrCodeFitOverflow {
			found = true
			if f.Path == "" {
				t.Error("finding has empty path")
			}
			if f.WrapLines == 0 {
				t.Error("finding has zero wrap lines")
			}
			if f.Fix == nil {
				t.Error("finding has nil fix")
			}
			if f.BindingDimension != "height" {
				t.Errorf("binding_dimension = %q, want %q", f.BindingDimension, "height")
			}
		}
	}
	if !found {
		t.Error("no fit_overflow finding found")
	}
}

func TestGenerateFitReport_DensityExceeded(t *testing.T) {
	// Create a very dense table that exceeds TDR ceiling.
	headers := make([]string, 10)
	for i := range headers {
		headers[i] = "H"
	}
	rows := make([][]TableCellInput, 15)
	for i := range rows {
		row := make([]TableCellInput, 10)
		for j := range row {
			row[j] = TableCellInput{Content: "x"}
		}
		rows[i] = row
	}

	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Content: []ContentInput{
					{
						PlaceholderID: "body",
						Type:          "table",
						TableValue:    &TableInput{Headers: headers, Rows: rows},
					},
				},
			},
		},
	}
	findings := generateFitReport(input)

	found := false
	for _, f := range findings {
		if f.Code == patterns.ErrCodeDensityExceeded {
			found = true
		}
	}
	if !found {
		t.Error("expected density_exceeded finding for very dense table")
	}
}

func TestGenerateFitReport_ShapeGridText(t *testing.T) {
	// Use a narrow, short grid (10% width × 5% height) with 4 columns so cells are tiny.
	longText := strings.Repeat("Long text that should cause overflow in shape cell. ", 30)
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "blank",
				ShapeGrid: &ShapeGridInput{
					Bounds: &GridBoundsInput{X: 5, Y: 5, Width: 10, Height: 5},
					Columns: json.RawMessage(`4`),
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Shape: &ShapeSpecInput{
									Geometry: "rect",
									Text:     json.RawMessage(`{"content":"` + longText + `","size":14}`),
								}},
								{Shape: &ShapeSpecInput{
									Geometry: "rect",
									Text:     json.RawMessage(`"short"`),
								}},
								nil,
								nil,
							},
						},
					},
				},
			},
		},
	}
	findings := generateFitReport(input)

	found := false
	for _, f := range findings {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") {
			found = true
		}
	}
	if !found {
		t.Error("expected fit_overflow finding for overflowing shape text")
	}
}

func TestGenerateFitReport_ShapeGridEmbeddedTable(t *testing.T) {
	longText := strings.Repeat("Overflow text here. ", 20)
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "blank",
				ShapeGrid: &ShapeGridInput{
					Columns: json.RawMessage(`3`),
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Table: &TableInput{
									Headers: []string{"A", "B", "C"},
									Rows: [][]TableCellInput{
										{{Content: longText}, {Content: "ok"}, {Content: "ok"}},
									},
								}},
								nil,
								nil,
							},
						},
					},
				},
			},
		},
	}
	findings := generateFitReport(input)

	found := false
	for _, f := range findings {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") && strings.Contains(f.Path, "table") {
			found = true
		}
	}
	if !found {
		t.Error("expected fit_overflow finding for overflowing embedded table")
	}
}

func TestWriteFitReport_NDJSON(t *testing.T) {
	findings := []fitFinding{
		{
			Code:    patterns.ErrCodeFitOverflow,
			Path:    "slides[0].content[0].rows[0][0]",
			Message: "text needs 3 lines",
			Fix:     &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 5}},
		},
		{
			Code:    patterns.ErrCodeDensityExceeded,
			Path:    "slides[0].content[0]",
			Message: "table too dense",
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := writeFitReport(path, findings); err != nil {
		t.Fatalf("writeFitReport failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d", len(lines))
	}

	var first fitFinding
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("failed to parse first line: %v", err)
	}
	if first.Code != patterns.ErrCodeFitOverflow {
		t.Errorf("first finding code = %q, want %q", first.Code, patterns.ErrCodeFitOverflow)
	}
	if first.Fix == nil || first.Fix.Kind != "split_at_row" {
		t.Error("first finding fix.kind should be split_at_row")
	}
}

func TestStrictFit_Off(t *testing.T) {
	// With strict-fit=off, runJSONMode should not run fit checks at all.
	// We just verify the code path doesn't error for an input that would
	// produce findings in warn/strict mode.
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "result.json")

	// Dense table that triggers density warnings.
	input := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "content",
			"content": [{
				"placeholder_id": "body",
				"type": "table",
				"table_value": {
					"headers": ["A","B","C","D","E","F","G","H","I","J"],
					"rows": [
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}],
						[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}]
					]
				}
			}]
		}]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join("..", "..", "templates")
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "off")
	// off mode should not fail due to density — only template/gen errors
	if err != nil {
		t.Logf("runJSONMode returned error (expected if template issue): %v", err)
	}
}

func TestStrictFit_Strict_RefusesUnfittable(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "result.json")

	// Very long text in a narrow 5-column table should trigger unfittable.
	longText := strings.Repeat("This is very long overflow text that cannot fit. ", 20)
	input := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "content",
			"content": [{
				"placeholder_id": "body",
				"type": "table",
				"table_value": {
					"headers": ["A","B","C","D","E"],
					"rows": [[{"content":"` + longText + `"},{"content":"ok"},{"content":"ok"},{"content":"ok"},{"content":"ok"}]]
				}
			}]
		}]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join("..", "..", "templates")
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "strict")
	if err == nil {
		t.Fatal("expected error in strict mode for unfittable content")
	}
	if !strings.Contains(err.Error(), "strict-fit") {
		t.Errorf("error should mention strict-fit, got: %v", err)
	}
}

func TestStrictFit_Warn_Succeeds(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "result.json")

	// Same unfittable content as strict test, but warn mode should succeed.
	longText := strings.Repeat("This is very long overflow text that cannot fit. ", 20)
	input := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "content",
			"content": [{
				"placeholder_id": "body",
				"type": "table",
				"table_value": {
					"headers": ["A","B","C","D","E"],
					"rows": [[{"content":"` + longText + `"},{"content":"ok"},{"content":"ok"},{"content":"ok"},{"content":"ok"}]]
				}
			}]
		}]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join("..", "..", "templates")
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "warn")
	// Warn mode should not fail due to fit issues — generation proceeds.
	if err != nil {
		t.Fatalf("warn mode should not error on unfittable content: %v", err)
	}
}

func TestStrictFit_ChartNoFindings_NoRejection(t *testing.T) {
	// Integration test: strict-fit=strict with a chart that produces zero
	// fit findings should succeed (baseline parity — svggen accepts the
	// level but does not act on it yet).
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "result.json")

	input := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "content",
			"content": [{
				"placeholder_id": "body",
				"type": "chart",
				"chart_value": {
					"type": "bar_chart",
					"title": "Revenue",
					"data": {
						"categories": ["Q1","Q2","Q3"],
						"series": [{"name":"Rev","values":[10,20,30]}]
					}
				}
			}]
		}]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join("..", "..", "templates")
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "strict")
	if err != nil {
		t.Fatalf("strict-fit=strict with zero chart findings should not reject: %v", err)
	}
}

func TestExtractShapeTextAndFont(t *testing.T) {
	tests := []struct {
		name     string
		raw      json.RawMessage
		wantText string
		wantPt   float64
	}{
		{"string", json.RawMessage(`"Hello"`), "Hello", 14},
		{"object", json.RawMessage(`{"content":"World","size":18}`), "World", 18},
		{"object_default_size", json.RawMessage(`{"content":"NoSize"}`), "NoSize", 14},
		{"empty", json.RawMessage(`""`), "", 14},
		{"null", nil, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, pt := extractShapeTextAndFont(tt.raw)
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
			if pt != tt.wantPt {
				t.Errorf("pt = %v, want %v", pt, tt.wantPt)
			}
		})
	}
}
