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
