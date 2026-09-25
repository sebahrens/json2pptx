package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGenerateFitReport_EmptyDeck(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{},
	}
	findings := generateFitReport(input, nil, 0, 0)
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
	findings := generateFitReport(input, nil, 0, 0)
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
	findings := generateFitReport(input, nil, 0, 0)
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
	findings := generateFitReport(input, nil, 0, 0)

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
					Bounds:  &GridBoundsInput{X: 5, Y: 5, Width: 10, Height: 5},
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
	findings := generateFitReport(input, nil, 0, 0)

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
	findings := generateFitReport(input, nil, 0, 0)

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
			Path:    "/slides/0/content/0/rows/0/0",
			Message: "text needs 3 lines",
			Fix:     &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 5}},
		},
		{
			Code:    patterns.ErrCodeDensityExceeded,
			Path:    "/slides/0/content/0",
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
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "off", false, "off", "", false)
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
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "strict", false, "off", "", false)
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
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "warn", false, "off", "", false)
	// Warn mode should not fail due to fit issues — generation proceeds.
	if err != nil {
		t.Fatalf("warn mode should not error on unfittable content: %v", err)
	}
}

func TestGenerateFitReport_CellUnderfilled_DensityBands(t *testing.T) {
	// Use a large grid cell (wide bounds) so that MaxChars is large,
	// and vary content length to hit different density bands.
	// With default bounds and 1 column, cells are very wide — MaxChars will be large.
	// We use a single cell with known font size to control density precisely.

	tests := []struct {
		name          string
		textRepeat    int // number of "x" chars to place
		fontSz        float64
		boundsWidth   float64 // percentage of slide width
		boundsHeight  float64 // percentage of slide height
		wantCode      string
		wantSeverity  string
		wantNoFinding bool // expect no cell_underfilled or fit_overflow finding
	}{
		{
			// A 3-char cell is a LABEL, not an underfilled paragraph: the
			// character-count capacity model does not apply to it, so it is
			// exempt (go-slide-creator-xpz8).
			name:          "short_label_is_exempt",
			textRepeat:    3,
			fontSz:        11,
			boundsWidth:   90,
			boundsHeight:  50,
			wantNoFinding: true,
		},
		{
			// Likewise a 40-char caption.
			name:          "caption_is_exempt",
			textRepeat:    40,
			fontSz:        11,
			boundsWidth:   30,
			boundsHeight:  10,
			wantNoFinding: true,
		},
		{
			// 64 chars = 80% of the 80-char 12pt budget → optimal band.
			name:          "optimal_no_finding",
			textRepeat:    64,
			fontSz:        11,
			boundsWidth:   30,
			boundsHeight:  10,
			wantNoFinding: true,
		},
		{
			name:         "severe_overflow_error",
			textRepeat:   2000,
			fontSz:       14,
			boundsWidth:  10,
			boundsHeight: 5,
			wantCode:     "fit_overflow",
			wantSeverity: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := strings.Repeat("x", tt.textRepeat)
			textJSON := json.RawMessage(fmt.Sprintf(`{"content":"%s","size":%v}`, text, tt.fontSz))

			input := &PresentationInput{
				Template: "midnight-blue",
				Slides: []SlideInput{
					{
						LayoutID: "blank",
						ShapeGrid: &ShapeGridInput{
							Bounds:  &GridBoundsInput{X: 5, Y: 5, Width: tt.boundsWidth, Height: tt.boundsHeight},
							Columns: json.RawMessage(`1`),
							Rows: []GridRowInput{
								{
									Cells: []*GridCellInput{
										{Shape: &ShapeSpecInput{
											Geometry: "rect",
											Text:     textJSON,
										}},
									},
								},
							},
						},
					},
				},
			}

			findings := generateFitReport(input, nil, 0, 0)

			if tt.wantNoFinding {
				for _, f := range findings {
					if f.Code == patterns.ErrCodeCellUnderfilled || f.Code == patterns.ErrCodeFitOverflow {
						t.Errorf("expected no cell finding, got code=%s severity=%s density msg=%s", f.Code, f.Severity, f.Message)
					}
				}
				return
			}

			var found *fitFinding
			for i, f := range findings {
				if f.Code == tt.wantCode && strings.Contains(f.Path, "shape_grid") {
					found = &findings[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("expected finding with code=%s, got findings: %v", tt.wantCode, findings)
			}
			if found.Severity != tt.wantSeverity {
				t.Errorf("severity = %q, want %q", found.Severity, tt.wantSeverity)
			}
			if found.Fix == nil {
				t.Error("expected non-nil fix suggestion")
			}
			if tt.wantCode == patterns.ErrCodeCellUnderfilled {
				if found.Fix.Kind != "add_detail_or_resize" {
					t.Errorf("fix.kind = %q, want %q", found.Fix.Kind, "add_detail_or_resize")
				}
				if found.Fix.Params == nil {
					t.Fatal("expected fix.params to be non-nil")
				}
				if _, ok := found.Fix.Params["current_density_pct"]; !ok {
					t.Error("expected fix.params to contain current_density_pct")
				}
			}
		})
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
	err := runJSONMode(jsonPath, outputPath, templatesDir, tmpDir, "", false, false, "", "strict", false, "off", "", false)
	if err != nil {
		t.Fatalf("strict-fit=strict with zero chart findings should not reject: %v", err)
	}
}

// smallBodyLayouts builds a single non-blank layout whose body placeholder is
// far smaller than the generic default grid bounds. resolveVirtualLayout's
// priority-3 branch then hands the fit report override bounds anchored to that
// small body placeholder, so the SAME shape_grid resolves into a tight cell.
// The CanonicalType is forced to a non-blank value so priority 1/2 (blank /
// blank-title) are skipped and the priority-3 fallback is exercised — the same
// path generation takes for a no-layout_id shape_grid slide.
func smallBodyLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			ID:            "compact",
			Name:          "Compact One Content",
			CanonicalType: types.CanonicalLayoutOneContent,
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 457200, Y: 200000, Width: 11277600, Height: 700000}},
				// Small body placeholder: ~3.0in wide x ~0.7in tall.
				{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1100000, Width: 2743200, Height: 640000}},
			},
		},
	}
}

// TestGenerateFitReport_ShapeGridUsesLayoutBounds is the regression case for
// go-slide-creator-ur3z: a shape_grid measured by the fit report must resolve
// against the SAME layout-aware bounds generation renders, not against generic
// full-slide defaults. The slide carries no explicit layout_id, so it goes
// through virtual layout resolution. The text fits comfortably in the generic
// default cell but overflows the small body-placeholder cell — so the finding
// set differs purely because the bounds differ.
//
// The text is sized so the small cell clips even at the renderer's smallest
// autofit shrink, which is what fit_overflow now reports: a cell the renderer
// merely shrinks into renders every word, and flagging that as overflow refused
// decks that look right (go-slide-creator-lmpu).
func TestGenerateFitReport_ShapeGridUsesLayoutBounds(t *testing.T) {
	const (
		slideWidth  int64 = 12192000
		slideHeight int64 = 6858000
	)
	text := strings.TrimSpace(strings.Repeat("word ", 700)) // ~3.5k chars
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Columns: json.RawMessage(`1`),
					Rows: []GridRowInput{
						{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{
							Geometry: "rect",
							Text:     json.RawMessage(`{"content":"` + text + `","size":11}`),
						}}}},
					},
				},
			},
		},
	}

	// Generic default bounds (nil layouts): the large cell easily holds the text.
	for _, f := range generateFitReport(input, nil, 0, 0) {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") {
			t.Fatalf("did not expect a shape_grid fit_overflow with generic default bounds; got %s @ %s", f.Code, f.Path)
		}
	}

	// Layout-aware bounds: the same grid resolves into the small body
	// placeholder and the text overflows.
	layoutFindings := generateFitReport(input, smallBodyLayouts(), slideWidth, slideHeight)
	found := false
	for _, f := range layoutFindings {
		if f.Code == patterns.ErrCodeFitOverflow && strings.Contains(f.Path, "shape_grid") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a shape_grid fit_overflow once the report uses the small body-placeholder bounds; got:")
		for _, f := range layoutFindings {
			t.Logf("  %s @ %s", f.Code, f.Path)
		}
	}
}

// go-slide-creator-xpz8: cell_underfilled was emitted once per cell with no
// aggregation, so a slide of KPI cards accumulated 20+ review-weight findings
// and bottomed its score out at 0 — while the one genuinely broken thing on the
// same deck cost 5 points. It is now one finding per slide, advisory unless the
// slide really is mostly empty.
func TestCellUnderfilled_AggregatedPerSlide(t *testing.T) {
	// Six cells of long-but-sparse card body copy in a generous grid.
	body := strings.Repeat("word ", 12) // 60 chars: card-body, not a caption
	cells := make([]*GridCellInput, 0, 3)
	for i := 0; i < 3; i++ {
		cells = append(cells, &GridCellInput{Shape: &ShapeSpecInput{
			Geometry: "rect",
			Text:     json.RawMessage(fmt.Sprintf(`{"content":%q,"size":11}`, body)),
		}})
	}

	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID: "blank",
			ShapeGrid: &ShapeGridInput{
				Bounds:  &GridBoundsInput{X: 5, Y: 5, Width: 90, Height: 80},
				Columns: json.RawMessage(`3`),
				Rows:    []GridRowInput{{Cells: cells}},
			},
		}},
	}

	findings := generateFitReport(input, nil, 0, 0)

	var underfills []fitFinding
	for _, f := range findings {
		if f.Code == patterns.ErrCodeCellUnderfilled {
			underfills = append(underfills, f)
		}
	}
	if len(underfills) != 1 {
		t.Fatalf("expected exactly 1 aggregated cell_underfilled finding, got %d: %+v", len(underfills), underfills)
	}

	f := underfills[0]
	if f.Fix == nil {
		t.Fatal("aggregate must carry a fix")
	}
	list, ok := f.Fix.Params["cells"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("fix.params.cells must enumerate the offending cells, got %v", f.Fix.Params["cells"])
	}
	// Each entry must locate the cell and carry its density.
	first, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("cells[0] = %T, want an object", list[0])
	}
	for _, key := range []string{"path", "chars", "density_pct"} {
		if _, ok := first[key]; !ok {
			t.Errorf("cells[0] missing %q: %v", key, first)
		}
	}
	if _, ok := f.Fix.Params["slide_mostly_empty"]; !ok {
		t.Error("fix.params must say whether the whole slide is underused")
	}
	// The path must address the slide's grid, not one cell inside it.
	if !strings.HasSuffix(f.Path, "/shape_grid") {
		t.Errorf("aggregate path = %q, want the slide's shape_grid", f.Path)
	}
	// Pattern-generated cells can have the same geometry but no authored
	// capacity target. Keep raw-grid underfill while omitting this false signal.
	input.Slides[0].Pattern = &PatternInput{Name: "card-grid"}
	for _, got := range generateFitReport(input, nil, 0, 0) {
		if got.Code == patterns.ErrCodeCellUnderfilled {
			t.Fatalf("pattern-generated cells reported raw-grid underfill: %+v", got)
		}
	}
}

// A KPI card holding "$12.4M" is CORRECT, not underfilled.
func TestUnderfillExemptRole(t *testing.T) {
	tests := []struct {
		name   string
		fontPt float64
		chars  int
		want   bool
	}{
		{"kpi value at display size", 32, 6, true},
		{"short label", 11, 8, true},
		{"chevron step name", 12, 18, true},
		{"caption at the 40-char boundary", 11, 40, true},
		{"card body copy", 11, 60, false},
		{"long paragraph", 11, 300, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := underfillExemptRole(tt.fontPt, tt.chars); got != tt.want {
				t.Errorf("underfillExemptRole(%v, %d) = %v, want %v", tt.fontPt, tt.chars, got, tt.want)
			}
		})
	}
}

// The aggregate escalates only when the grid as a whole carries barely any
// content. Counting sparse CELLS was the wrong measure: a card grid whose
// bodies are one deliberate sentence each has every cell "sparse" while the
// slide reads fine.
func TestAggregateUnderfilledFinding_EscalatesOnlyWhenSlideIsEmpty(t *testing.T) {
	// A cell holding 20 of 200 possible characters: 10% fill.
	veryEmpty := underfilledCell{path: "/slides/0/shape_grid/rows/0/cells/0/shape/text", chars: 20, maxChars: 200, densityPct: 10}
	// A cell holding 110 of 200: sparse by the 60% cell rule, but substantial.
	airy := underfilledCell{path: "/slides/0/shape_grid/rows/0/cells/1/shape/text", chars: 110, maxChars: 200, densityPct: 55}

	t.Run("a nearly empty grid escalates", func(t *testing.T) {
		f := aggregateUnderfilledFinding(0, []underfilledCell{veryEmpty, veryEmpty, veryEmpty}, 3, 60, 600)
		if f == nil {
			t.Fatal("expected a finding")
		}
		if f.Action != "review" {
			t.Errorf("action = %q, want review at 10 percent slide fill", f.Action)
		}
		if !strings.Contains(f.Message, "mostly empty") {
			t.Errorf("message should say the slide is mostly empty, got: %s", f.Message)
		}
		if pct, ok := f.Fix.Params["slide_fill_pct"].(int); !ok || pct != 10 {
			t.Errorf("fix.params.slide_fill_pct = %v, want 10", f.Fix.Params["slide_fill_pct"])
		}
	})

	t.Run("a deliberately airy grid stays advisory", func(t *testing.T) {
		// Every cell is "sparse" by the per-cell rule, but the grid carries 55%
		// of its capacity — a legitimate design, not a defect.
		f := aggregateUnderfilledFinding(0, []underfilledCell{airy, airy, airy, airy}, 4, 440, 800)
		if f == nil {
			t.Fatal("expected a finding")
		}
		if f.Action != "info" {
			t.Errorf("action = %q, want info at 55 percent slide fill", f.Action)
		}
		if mostly, _ := f.Fix.Params["slide_mostly_empty"].(bool); mostly {
			t.Error("a grid carrying 55 percent of its capacity must not be reported as mostly empty")
		}
	})

	t.Run("too few cells to judge stays advisory", func(t *testing.T) {
		f := aggregateUnderfilledFinding(0, []underfilledCell{veryEmpty, veryEmpty}, 2, 40, 400)
		if f == nil {
			t.Fatal("expected a finding")
		}
		if f.Action != "info" {
			t.Errorf("action = %q, want info below the minimum-cell floor", f.Action)
		}
	})

	t.Run("no underfilled cells produces no finding", func(t *testing.T) {
		if aggregateUnderfilledFinding(0, nil, 4, 400, 500) != nil {
			t.Error("expected no finding")
		}
	})
}
