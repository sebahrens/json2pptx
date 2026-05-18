package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSlideIndexFromPath(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/slides/0/content/body", 0},
		{"/slides/3/shape_grid/rows/1/cells/0", 3},
		{"/slides/12/content/1", 12},
		{"other_path", -1},
		{"/slides/", -1},
		{"/slides/abc", -1},
	}
	for _, tt := range tests {
		got := slideIndexFromPath(tt.path)
		if got != tt.want {
			t.Errorf("slideIndexFromPath(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestConvertTextFitFinding(t *testing.T) {
	tf := fitFinding{
		Code:        patterns.ErrCodeFitOverflow,
		Path:        "slides[0].content[1].rows[0][0]",
		Message:     "text needs 5 lines",
		Action:      "refuse",
		RequiredPt:  100,
		AllocatedPt: 50,
	}

	f := convertTextFitFinding(tf)

	if f.Code != patterns.ErrCodeFitOverflow {
		t.Errorf("Code = %q, want %q", f.Code, patterns.ErrCodeFitOverflow)
	}
	if f.Path != tf.Path {
		t.Errorf("Path = %q, want %q", f.Path, tf.Path)
	}
	if f.Action != "refuse" {
		t.Errorf("Action = %q, want %q", f.Action, "refuse")
	}
	if f.Measured == nil || f.Allowed == nil {
		t.Fatal("Measured and Allowed should be populated")
	}
	if f.OverflowRatio != 2.0 {
		t.Errorf("OverflowRatio = %f, want 2.0", f.OverflowRatio)
	}
}

func TestCollectFitFindingsSorting(t *testing.T) {
	// Create a presentation with two slides that will produce findings
	// at different severities. We test that the sorting works correctly.
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{{}, {}}, // Empty slides — no findings from here
	}

	findings := collectFitFindings(input, nil, 9144000, 6858000, nil)
	// With empty slides, expect no findings.
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty slides, got %d", len(findings))
	}
}

func TestExtractContentParagraphs(t *testing.T) {
	text := "Hello World"
	c := ContentInput{
		Type:      "text",
		TextValue: &text,
	}
	paras := extractContentParagraphs(&c)
	if len(paras) != 1 || paras[0] != "Hello World" {
		t.Errorf("text content: got %v, want [Hello World]", paras)
	}

	bullets := []string{"a", "b", "c"}
	c2 := ContentInput{
		Type:         "bullets",
		BulletsValue: &bullets,
	}
	paras2 := extractContentParagraphs(&c2)
	if len(paras2) != 3 {
		t.Errorf("bullets content: got %d paragraphs, want 3", len(paras2))
	}

	c3 := ContentInput{
		Type: "body_and_bullets",
		BodyAndBulletsValue: &BodyAndBulletsInput{
			Body:         "intro",
			Bullets:      []string{"x", "y"},
			TrailingBody: "end",
		},
	}
	paras3 := extractContentParagraphs(&c3)
	if len(paras3) != 4 {
		t.Errorf("body_and_bullets: got %d paragraphs, want 4", len(paras3))
	}
}

func TestContrastSwapsToFindings(t *testing.T) {
	swaps := []generator.ContrastSwap{
		{
			OriginalColor:   "#FD5108",
			ReplacedColor:   "#A03000",
			BackgroundColor: "#FFE8D4",
			RatioBefore:     2.1,
			RatioAfter:      4.6,
		},
		{
			OriginalColor:   "#FFFFFF",
			ReplacedColor:   "#1A1A1A",
			BackgroundColor: "#FFB6C1",
			RatioBefore:     1.65,
			RatioAfter:      8.2,
		},
	}

	findings := contrastSwapsToFindings(swaps)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	for i, f := range findings {
		if f.Code != "contrast_autofixed" {
			t.Errorf("finding[%d].Code = %q, want %q", i, f.Code, "contrast_autofixed")
		}
		if f.Action != "info" {
			t.Errorf("finding[%d].Action = %q, want %q", i, f.Action, "info")
		}
		if f.Fix == nil {
			t.Fatalf("finding[%d].Fix is nil", i)
		}
		if f.Fix.Kind != "replace_color" {
			t.Errorf("finding[%d].Fix.Kind = %q, want %q", i, f.Fix.Kind, "replace_color")
		}
		if !strings.Contains(f.Message, swaps[i].OriginalColor) {
			t.Errorf("finding[%d].Message should contain original color %q, got %q", i, swaps[i].OriginalColor, f.Message)
		}
	}
}

func TestContrastSwapsToFindings_Empty(t *testing.T) {
	findings := contrastSwapsToFindings(nil)
	if findings != nil {
		t.Errorf("expected nil for empty swaps, got %v", findings)
	}
}

// --- BudgetFitFindings tests ---

func makeFinding(slideIdx int, action string, code string, hasFix bool) patterns.FitFinding {
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    fmt.Sprintf("/slides/%d/content/body", slideIdx),
			Code:    code,
			Message: fmt.Sprintf("finding %s on slide %d", code, slideIdx),
		},
		Action: action,
	}
	if hasFix {
		f.Fix = &patterns.FixSuggestion{Kind: "reduce_text"}
	}
	return f
}

func TestBudgetFitFindings_Under(t *testing.T) {
	// 3 findings on one slide — should pass through unchanged.
	findings := []patterns.FitFinding{
		makeFinding(0, "refuse", "a", true),
		makeFinding(0, "review", "b", false),
		makeFinding(0, "info", "c", false),
	}
	result := BudgetFitFindings(findings, 5, false)
	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}
}

func TestBudgetFitFindings_Over(t *testing.T) {
	// 20 findings on slide 0 — should return 5 + 1 summary.
	var findings []patterns.FitFinding
	for i := 0; i < 20; i++ {
		action := "info"
		if i < 3 {
			action = "refuse"
		} else if i < 8 {
			action = "shrink_or_split"
		}
		findings = append(findings, makeFinding(0, action, fmt.Sprintf("code_%d", i), i%2 == 0))
	}
	result := BudgetFitFindings(findings, 5, false)

	if len(result) != 6 {
		t.Fatalf("expected 6 findings (5 + 1 summary), got %d", len(result))
	}

	// Top findings should be sorted by severity desc, fix-present first.
	for i := 0; i < 5; i++ {
		if i > 0 {
			ri := patterns.ActionRank(result[i].Action)
			rPrev := patterns.ActionRank(result[i-1].Action)
			if ri > rPrev {
				t.Errorf("finding[%d] has higher rank (%d) than finding[%d] (%d)", i, ri, i-1, rPrev)
			}
		}
	}

	// Last one should be the summary.
	summary := result[5]
	if summary.Code != "findings_truncated" {
		t.Errorf("summary code = %q, want %q", summary.Code, "findings_truncated")
	}
	if summary.Action != "info" {
		t.Errorf("summary action = %q, want %q", summary.Action, "info")
	}
	if !strings.Contains(summary.Message, "15 more findings suppressed") {
		t.Errorf("summary message = %q, want to contain '15 more findings suppressed'", summary.Message)
	}
	if !strings.Contains(summary.Message, "verbose_fit") {
		t.Errorf("summary message = %q, want to contain 'verbose_fit'", summary.Message)
	}

	// Verify truncation summary includes top_codes histogram.
	if summary.Fix == nil {
		t.Fatal("summary.Fix should be non-nil (truncation_summary)")
	}
	if summary.Fix.Kind != "truncation_summary" {
		t.Errorf("summary.Fix.Kind = %q, want %q", summary.Fix.Kind, "truncation_summary")
	}
	if summary.Fix.Params["suppressed_count"] != 15 {
		t.Errorf("suppressed_count = %v, want 15", summary.Fix.Params["suppressed_count"])
	}
	topCodes, ok := summary.Fix.Params["top_codes"].([]string)
	if !ok {
		t.Fatalf("top_codes should be []string, got %T", summary.Fix.Params["top_codes"])
	}
	if len(topCodes) == 0 {
		t.Error("top_codes should not be empty")
	}
}

func TestBudgetFitFindings_Verbose(t *testing.T) {
	var findings []patterns.FitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := BudgetFitFindings(findings, 5, true)
	if len(result) != 20 {
		t.Fatalf("verbose mode should return all 20 findings, got %d", len(result))
	}
}

func TestBudgetFitFindings_MultiSlide(t *testing.T) {
	// 3 findings on slide 0 (under budget), 7 on slide 1 (over budget).
	var findings []patterns.FitFinding
	for i := 0; i < 3; i++ {
		findings = append(findings, makeFinding(0, "review", fmt.Sprintf("a_%d", i), true))
	}
	for i := 0; i < 7; i++ {
		findings = append(findings, makeFinding(1, "info", fmt.Sprintf("b_%d", i), false))
	}
	result := BudgetFitFindings(findings, 5, false)

	// Slide 0: 3 findings. Slide 1: 5 + 1 summary = 6. Total = 9.
	if len(result) != 9 {
		t.Fatalf("expected 9 findings, got %d", len(result))
	}

	// Last should be summary for slide 1.
	summary := result[8]
	if summary.Code != "findings_truncated" {
		t.Errorf("last finding code = %q, want findings_truncated", summary.Code)
	}
	if !strings.Contains(summary.Message, "2 more") {
		t.Errorf("summary message = %q, want to contain '2 more'", summary.Message)
	}
}

func TestBudgetFitFindings_FixPriority(t *testing.T) {
	// Two findings with same severity — one with Fix, one without.
	// The one with Fix should come first.
	findings := []patterns.FitFinding{
		makeFinding(0, "review", "no_fix", false),
		makeFinding(0, "review", "has_fix", true),
	}
	result := BudgetFitFindings(findings, 5, false)
	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}
	if result[0].Code != "has_fix" {
		t.Errorf("expected fix-bearing finding first, got %q", result[0].Code)
	}
}

func TestBudgetFitFindings_Empty(t *testing.T) {
	result := BudgetFitFindings(nil, 5, false)
	if result != nil {
		t.Errorf("expected nil for empty findings, got %v", result)
	}
}

// --- budgetLocalFindings tests ---

func makeLocalFinding(slideIdx int, action, code string, hasFix bool) fitFinding {
	f := fitFinding{
		Path:    fmt.Sprintf("/slides/%d/content/body", slideIdx),
		Code:    code,
		Message: fmt.Sprintf("finding %s on slide %d", code, slideIdx),
		Action:  action,
	}
	if hasFix {
		f.Fix = &patterns.FixSuggestion{Kind: "reduce_text"}
	}
	return f
}

func TestBudgetLocalFindings_Over(t *testing.T) {
	var findings []fitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeLocalFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := budgetLocalFindings(findings, 5, false)

	if len(result) != 6 {
		t.Fatalf("expected 6 findings (5 + 1 summary), got %d", len(result))
	}
	if result[5].Code != "findings_truncated" {
		t.Errorf("summary code = %q, want findings_truncated", result[5].Code)
	}
	if !strings.Contains(result[5].Message, "--verbose-fit") {
		t.Errorf("local summary should reference --verbose-fit flag, got %q", result[5].Message)
	}
	if result[5].Fix == nil || result[5].Fix.Kind != "truncation_summary" {
		t.Error("local summary should have truncation_summary Fix")
	}
}

func TestBudgetLocalFindings_Verbose(t *testing.T) {
	var findings []fitFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, makeLocalFinding(0, "info", fmt.Sprintf("code_%d", i), false))
	}
	result := budgetLocalFindings(findings, 5, true)
	if len(result) != 20 {
		t.Fatalf("verbose mode should return all 20, got %d", len(result))
	}
}

func TestFindingCodeHistogram(t *testing.T) {
	items := []patterns.FitFinding{
		makeFinding(0, "info", "fit_overflow", false),
		makeFinding(0, "info", "density_exceeded", false),
		makeFinding(0, "info", "fit_overflow", false),
		makeFinding(0, "info", "fit_overflow", false),
		makeFinding(0, "info", "density_exceeded", false),
		makeFinding(0, "info", "bounds_overflow", false),
	}
	result := findingCodeHistogram(items)

	// Expect: fit_overflow:3, density_exceeded:2, bounds_overflow:1
	if len(result) != 3 {
		t.Fatalf("expected 3 histogram entries, got %d: %v", len(result), result)
	}
	if result[0] != "fit_overflow:3" {
		t.Errorf("result[0] = %q, want %q", result[0], "fit_overflow:3")
	}
	if result[1] != "density_exceeded:2" {
		t.Errorf("result[1] = %q, want %q", result[1], "density_exceeded:2")
	}
	if result[2] != "bounds_overflow:1" {
		t.Errorf("result[2] = %q, want %q", result[2], "bounds_overflow:1")
	}
}

func TestBudgetFitFindings_TopCodesHistogram(t *testing.T) {
	// 8 findings: 5 fit_overflow + 3 density_exceeded, budget=3.
	// Expect suppressed=5, top_codes reflects the 5 suppressed items.
	var findings []patterns.FitFinding
	for i := 0; i < 5; i++ {
		findings = append(findings, makeFinding(0, "info", "fit_overflow", false))
	}
	for i := 0; i < 3; i++ {
		findings = append(findings, makeFinding(0, "info", "density_exceeded", false))
	}
	result := BudgetFitFindings(findings, 3, false)

	// 3 kept + 1 summary = 4.
	if len(result) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(result))
	}
	summary := result[3]
	if summary.Fix == nil {
		t.Fatal("summary.Fix should be non-nil")
	}
	if summary.Fix.Params["suppressed_count"] != 5 {
		t.Errorf("suppressed_count = %v, want 5", summary.Fix.Params["suppressed_count"])
	}
	topCodes := summary.Fix.Params["top_codes"].([]string)
	if len(topCodes) == 0 {
		t.Fatal("top_codes should not be empty")
	}
	// The suppressed 5 items should produce a histogram.
	// Exact distribution depends on sort order but total should be 5.
	total := 0
	for _, entry := range topCodes {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			t.Errorf("unexpected format: %q", entry)
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Errorf("bad count in %q: %v", entry, err)
			continue
		}
		total += n
	}
	if total != 5 {
		t.Errorf("histogram total = %d, want 5", total)
	}
}

func TestDetectSparseRawGrid(t *testing.T) {
	slideHeight := int64(6858000) // standard 7.5 inch slide height

	t.Run("sparse raw grid emits finding", func(t *testing.T) {
		// Single row, short text, no explicit height → content is tiny
		// relative to the 70% default layout area.
		grid := &ShapeGridInput{
			Columns: json.RawMessage(`2`),
			Rows: []GridRowInput{
				{Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
					{Shape: &ShapeSpecInput{Geometry: "rect"}},
				}},
			},
		}
		f := detectSparseRawGrid(grid, 0, 0, slideHeight)
		if f == nil {
			t.Fatal("expected sparse_layout finding for raw grid with tiny content")
		}
		if f.Code != patterns.ErrCodeSparseLayout {
			t.Errorf("code = %q, want %q", f.Code, patterns.ErrCodeSparseLayout)
		}
		if f.Action != "review" {
			t.Errorf("action = %q, want %q", f.Action, "review")
		}
		if f.Fix == nil || f.Fix.Kind != "adopt_pattern" {
			t.Errorf("fix kind = %v, want adopt_pattern", f.Fix)
		}
	})

	t.Run("non-sparse raw grid no finding", func(t *testing.T) {
		// Many rows with multi-line text that fill the layout area.
		rows := make([]GridRowInput, 10)
		for i := range rows {
			rows[i] = GridRowInput{Cells: []*GridCellInput{
				{Shape: &ShapeSpecInput{Geometry: "rect", Text: json.RawMessage(`"Line1\nLine2\nLine3\nLine4\nLine5\nLine6\nLine7\nLine8"`)}},
			}}
		}
		grid := &ShapeGridInput{
			Columns: json.RawMessage(`1`),
			Rows:    rows,
		}
		f := detectSparseRawGrid(grid, 0, 0, slideHeight)
		if f != nil {
			t.Errorf("expected nil for dense grid, got finding: %s", f.Message)
		}
	})

	t.Run("explicit row height skips detection", func(t *testing.T) {
		grid := &ShapeGridInput{
			Columns: json.RawMessage(`2`),
			Rows: []GridRowInput{
				{
					Height: 100,
					Cells: []*GridCellInput{
						{Shape: &ShapeSpecInput{Geometry: "rect"}},
					},
				},
			},
		}
		if allGridRowHeightsZero(grid.Rows) {
			t.Error("allGridRowHeightsZero should be false for explicit height")
		}
	})
}

func TestAllGridRowHeightsZero(t *testing.T) {
	tests := []struct {
		name string
		rows []GridRowInput
		want bool
	}{
		{"nil rows", nil, true},
		{"empty rows", []GridRowInput{}, true},
		{"zero heights", []GridRowInput{{Height: 0}, {Height: 0}}, true},
		{"one explicit", []GridRowInput{{Height: 0}, {Height: 50}}, false},
		{"all explicit", []GridRowInput{{Height: 50}, {Height: 50}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allGridRowHeightsZero(tt.rows); got != tt.want {
				t.Errorf("allGridRowHeightsZero = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckShapeGridStructural_IncludesVisualCells(t *testing.T) {
	// Build a grid with icon, image, and diagram cells placed outside slide bounds.
	// Prior to the fix, these cells were skipped. After the fix, they should
	// produce slide_bounds_overflow findings.
	grid := &ShapeGridInput{
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Icon: &IconInput{Name: "check"}},
					{Image: &GridImageInput{Path: "https://example.com/img.png", Alt: "test"}},
					{Diagram: &types.DiagramSpec{Type: "cycle", Data: map[string]any{"steps": []any{"A", "B"}}}},
				},
			},
		},
		// Place grid far off-slide to trigger bounds overflow.
		Bounds: &GridBoundsInput{X: 110, Y: 0, Width: 20, Height: 50},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")

	// All three visual cells should produce bounds overflow findings.
	boundsCount := 0
	for _, f := range findings {
		if f.Code == "slide_bounds_overflow" {
			boundsCount++
		}
	}
	if boundsCount < 3 {
		t.Errorf("expected at least 3 slide_bounds_overflow findings for icon/image/diagram cells, got %d (total findings: %d)", boundsCount, len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.Code, f.Path)
		}
	}
}

func TestGenerateGridOutput_DiagramNarrowFinding(t *testing.T) {
	// Create a grid with a complex diagram in a narrow cell and verify that
	// generateGridOutput produces a structured finding (not just a warning string).
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type: "org_chart",
						Data: map[string]any{
							"root": map[string]any{
								"name": "CEO",
								"children": []any{
									map[string]any{"name": "VP1", "children": []any{
										map[string]any{"name": "M1"},
										map[string]any{"name": "M2"},
										map[string]any{"name": "M3"},
									}},
									map[string]any{"name": "VP2", "children": []any{
										map[string]any{"name": "M4"},
										map[string]any{"name": "M5"},
									}},
								},
							},
						},
					}},
					nil,
					nil,
				},
			},
		},
	}

	alloc := pptx.NewShapeIDAllocator(nil)
	diagCtx := &GridDiagramContext{
		SlideNum: 1,
	}

	result, err := resolveShapeGrid(grid, alloc, nil, nil, 12192000, 6858000, diagCtx)
	if err != nil {
		t.Fatalf("resolveShapeGrid failed: %v", err)
	}
	if result == nil {
		t.Fatal("resolveShapeGrid returned nil result")
	}

	// The diagram cell occupies ~33% of slide width (one of three columns),
	// which is well below the narrow threshold. Expect a structured finding.
	if len(result.FitFindings) == 0 {
		t.Fatal("expected at least one FitFinding for narrow diagram cell, got 0")
	}

	found := false
	for _, f := range result.FitFindings {
		if f.Code == "grid_diagram_narrow" {
			found = true
			if !strings.Contains(f.Path, "/diagram") {
				t.Errorf("finding path should target diagram field, got: %s", f.Path)
			}
			if f.Action != "review" {
				t.Errorf("expected action 'review', got %q", f.Action)
			}
			if f.Fix == nil || f.Fix.Kind != "reshape_grid" {
				t.Error("expected Fix with kind reshape_grid")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected grid_diagram_narrow finding, got: %v", result.FitFindings)
	}
}

// TestCheckShapeGridStructural_PreflightDiagramNarrow verifies that the
// preflight structural check approximates grid_diagram_narrow for a complex
// diagram in a narrow cell, matching the render-time finding.
func TestCheckShapeGridStructural_PreflightDiagramNarrow(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`3`),
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type: "org_chart",
						Data: map[string]any{
							"root": map[string]any{
								"name": "CEO",
								"children": []any{
									map[string]any{"name": "VP1", "children": []any{
										map[string]any{"name": "M1"},
										map[string]any{"name": "M2"},
										map[string]any{"name": "M3"},
									}},
									map[string]any{"name": "VP2", "children": []any{
										map[string]any{"name": "M4"},
										map[string]any{"name": "M5"},
									}},
								},
							},
						},
					}},
					nil,
					nil,
				},
			},
		},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")

	found := false
	for _, f := range findings {
		if f.Code == "grid_diagram_narrow" {
			found = true
			if !strings.Contains(f.Path, "/diagram") {
				t.Errorf("finding path should target diagram field, got: %s", f.Path)
			}
			if f.Action != "review" {
				t.Errorf("expected action 'review', got %q", f.Action)
			}
			if f.Fix == nil || f.Fix.Kind != "reshape_grid" {
				t.Error("expected Fix with kind reshape_grid")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected preflight grid_diagram_narrow finding; got findings: %v", findings)
	}
}

// TestCheckShapeGridStructural_PreflightDiagramAspectMismatch verifies that
// the preflight structural check emits diagram_aspect_mismatch when a grid
// cell's aspect differs from the diagram's rendered SVG aspect by more than
// 25% (default svggen aspect is 4:3 unless DiagramSpec.Width/Height override).
func TestCheckShapeGridStructural_PreflightDiagramAspectMismatch(t *testing.T) {
	// Grid has 4 narrow columns; the diagram cell occupies one column, giving
	// a tall narrow cell aspect that is well below the SVG's default 4:3.
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`4`),
		Rows: []GridRowInput{
			{
				Height: 100,
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type: "bar_chart",
						Data: map[string]any{
							"categories": []any{"A", "B"},
							"series":     []any{map[string]any{"name": "S", "values": []any{1.0, 2.0}}},
						},
					}},
					nil,
					nil,
					nil,
				},
			},
		},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")

	found := false
	for _, f := range findings {
		if f.Code == "diagram_aspect_mismatch" {
			found = true
			if !strings.Contains(f.Path, "/diagram") {
				t.Errorf("finding path should target diagram field, got: %s", f.Path)
			}
			if f.Action != "review" {
				t.Errorf("expected action 'review', got %q", f.Action)
			}
			if f.Fix == nil || f.Fix.Kind != "reshape_grid" {
				t.Error("expected Fix with kind reshape_grid")
			}
			break
		}
	}
	if !found {
		codes := make([]string, 0, len(findings))
		for _, f := range findings {
			codes = append(codes, f.Code)
		}
		t.Errorf("expected preflight diagram_aspect_mismatch finding; got codes: %v", codes)
	}
}

// TestCheckShapeGridStructural_NoDiagramAspectMismatchWhenAligned verifies the
// preflight check does NOT emit diagram_aspect_mismatch when the cell aspect
// is within 25% of the rendered SVG aspect.
func TestCheckShapeGridStructural_NoDiagramAspectMismatchWhenAligned(t *testing.T) {
	// Explicit grid bounds yield a deterministic cell aspect. The 16:9 slide
	// (12192000 × 6858000 EMU) is wider than tall, so we scale width/height
	// percentages so that the resulting cell is ~2:1. wPct/hPct ≈ 1.125
	// cancels the 16:9 ratio to give cell aspect ≈ 2.0.
	grid := &ShapeGridInput{
		Bounds:  &GridBoundsInput{X: 5, Y: 10, Width: 90, Height: 80},
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type:   "bar_chart",
						Width:  1000,
						Height: 500, // 2:1, aligned with cell
						Data: map[string]any{
							"categories": []any{"A", "B"},
							"series":     []any{map[string]any{"name": "S", "values": []any{1.0, 2.0}}},
						},
					}},
				},
			},
		},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")
	for _, f := range findings {
		if f.Code == "diagram_aspect_mismatch" {
			t.Errorf("should not emit diagram_aspect_mismatch for aligned aspect; got finding: %s", f.Message)
		}
	}
}

// TestCheckShapeGridStructural_PreflightDiagramAspectConflict verifies that
// the preflight structural check emits diagram_aspect_conflict when a grid
// cell aspect deviates from a non-chart diagram's natural svggen viewBox
// aspect by more than 30%. Uses a timeline (natural 2:1) in a tall narrow
// cell — explicit grid bounds force the cell to roughly 1:1, well below the
// 2:1 natural aspect.
func TestCheckShapeGridStructural_PreflightDiagramAspectConflict(t *testing.T) {
	grid := &ShapeGridInput{
		// Tall narrow grid: width 25% × height 85% of a 16:9 slide → cell
		// aspect ≈ (0.25*12192000)/(0.85*6858000) ≈ 0.52, vs timeline 2.0
		// → deviation ~74% > 30% threshold.
		Bounds:  &GridBoundsInput{X: 5, Y: 5, Width: 25, Height: 85},
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type: "timeline",
						Data: map[string]any{
							"values": []any{"Q1", "Q2", "Q3", "Q4"},
						},
					}},
				},
			},
		},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")

	found := false
	for _, f := range findings {
		if f.Code == "diagram_aspect_conflict" {
			found = true
			if !strings.Contains(f.Path, "/diagram") {
				t.Errorf("finding path should target diagram field, got: %s", f.Path)
			}
			if f.Action != "review" {
				t.Errorf("expected action 'review', got %q", f.Action)
			}
			if f.Fix == nil || f.Fix.Kind != "reshape_grid" {
				t.Error("expected Fix with kind reshape_grid")
			}
			break
		}
	}
	if !found {
		codes := make([]string, 0, len(findings))
		for _, f := range findings {
			codes = append(codes, f.Code)
		}
		t.Errorf("expected preflight diagram_aspect_conflict finding; got codes: %v", codes)
	}
}

// TestCheckShapeGridStructural_NoDiagramNarrowForWideCell verifies that the
// preflight check does NOT emit grid_diagram_narrow when the diagram cell
// is wide enough (render-time-only findings are not false-positived).
func TestCheckShapeGridStructural_NoDiagramNarrowForWideCell(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`), // single column = full width
		Rows: []GridRowInput{
			{
				Cells: []*GridCellInput{
					{Diagram: &types.DiagramSpec{
						Type: "org_chart",
						Data: map[string]any{
							"root": map[string]any{
								"name": "CEO",
								"children": []any{
									map[string]any{"name": "VP1"},
									map[string]any{"name": "VP2"},
									map[string]any{"name": "VP3"},
									map[string]any{"name": "VP4"},
									map[string]any{"name": "VP5"},
								},
							},
						},
					}},
				},
			},
		},
	}

	slideWidth := int64(12192000)
	slideHeight := int64(6858000)
	layout := &types.LayoutMetadata{
		ID: "blank",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 11277600, Height: 4800600}},
		},
	}

	findings := checkShapeGridStructural(grid, 0, slideWidth, slideHeight, layout, false, "")

	for _, f := range findings {
		if f.Code == "grid_diagram_narrow" {
			t.Errorf("should not emit grid_diagram_narrow for wide cell; got finding at path %s", f.Path)
		}
	}
}
