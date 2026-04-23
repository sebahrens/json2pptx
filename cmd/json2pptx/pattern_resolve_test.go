package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ensure patterns package init runs
var _ = patterns.Default()

func TestExpandPattern_KPI3Up(t *testing.T) {
	input := &PatternInput{
		Name: "kpi-3up",
		Values: json.RawMessage(`[
			{"big": "$4.2M", "small": "ARR"},
			{"big": "98%", "small": "Uptime"},
			{"big": "1.2K", "small": "Users"}
		]`),
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandPattern returned nil grid")
	}
	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(grid.Rows))
	}
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestExpandPattern_UnknownPattern(t *testing.T) {
	input := &PatternInput{
		Name:   "nonexistent-pattern",
		Values: json.RawMessage(`{}`),
	}

	ctx := patterns.ExpandContext{}
	_, _, err := expandPattern(input, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected error for unknown pattern")
	}
	if !strings.Contains(err.Error(), "unknown pattern") {
		t.Errorf("error should mention 'unknown pattern', got: %v", err)
	}
}

func TestExpandPattern_InvalidValues(t *testing.T) {
	input := &PatternInput{
		Name:   "kpi-3up",
		Values: json.RawMessage(`"not an array"`),
	}

	ctx := patterns.ExpandContext{}
	_, _, err := expandPattern(input, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected error for invalid values")
	}
}

func TestExpandPattern_ValidationFailure(t *testing.T) {
	// Only 2 cells instead of 3
	input := &PatternInput{
		Name: "kpi-3up",
		Values: json.RawMessage(`[
			{"big": "$4.2M", "small": "ARR"},
			{"big": "98%", "small": "Uptime"}
		]`),
	}

	ctx := patterns.ExpandContext{}
	_, _, err := expandPattern(input, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected validation error for wrong cell count")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation, got: %v", err)
	}
}

func TestConvertPresentationSlides_PatternAndShapeGridXOR(t *testing.T) {
	slide := SlideInput{
		LayoutID:  "content",
		Pattern:   &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`[]`)},
		ShapeGrid: &ShapeGridInput{},
	}

	_, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000, nil)
	if err == nil {
		t.Fatal("expected XOR error when both pattern and shape_grid set")
	}
	if !strings.Contains(err.Error(), "cannot set both pattern and shape_grid") {
		t.Errorf("error should mention XOR constraint, got: %v", err)
	}
}

func TestConvertPresentationSlides_PatternExpansion(t *testing.T) {
	slide := SlideInput{
		LayoutID: "content",
		Pattern: &PatternInput{
			Name: "kpi-3up",
			Values: json.RawMessage(`[
				{"big": "$4.2M", "small": "ARR"},
				{"big": "98%", "small": "Uptime"},
				{"big": "1.2K", "small": "Users"}
			]`),
		},
	}

	specs, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("convertPresentationSlides failed: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}

	// Pattern should have expanded into shape_grid XML
	if len(specs[0].RawShapeXML) == 0 {
		t.Error("expected RawShapeXML to be populated after pattern expansion")
	}
}

func TestExpandPattern_CalloutCardGrid(t *testing.T) {
	input := &PatternInput{
		Name: "card-grid",
		Values: json.RawMessage(`{
			"columns": 2,
			"rows": 1,
			"cells": [
				{"header": "Card 1", "body": "Description 1"},
				{"header": "Card 2", "body": "Description 2"}
			]
		}`),
		Callout: &patterns.PatternCallout{
			Text:     "Key takeaway: both cards matter",
			Emphasis: "bold",
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern with callout failed: %v", err)
	}

	// card-grid 1×2 = 1 row + 1 callout row = 2 rows
	if len(grid.Rows) != 2 {
		t.Fatalf("expected 2 rows (1 content + 1 callout), got %d", len(grid.Rows))
	}

	// Callout row should have AutoHeight and 1 cell spanning 2 columns
	calloutRow := grid.Rows[1]
	if !calloutRow.AutoHeight {
		t.Error("callout row should have AutoHeight=true")
	}
	if len(calloutRow.Cells) != 1 {
		t.Fatalf("callout row should have 1 cell, got %d", len(calloutRow.Cells))
	}
	if calloutRow.Cells[0].ColSpan != 2 {
		t.Errorf("callout cell ColSpan = %d, want 2", calloutRow.Cells[0].ColSpan)
	}
	if calloutRow.Cells[0].Shape == nil {
		t.Fatal("callout cell should have a Shape")
	}
}

func TestExpandPattern_CalloutComparison2col(t *testing.T) {
	input := &PatternInput{
		Name: "comparison-2col",
		Values: json.RawMessage(`{
			"header_left": "Pros",
			"header_right": "Cons",
			"rows": [
				{"left": "Fast", "right": "Expensive"}
			]
		}`),
		Callout: &patterns.PatternCallout{
			Text:   "Overall: choose wisely",
			Accent: "accent3",
		},
	}

	ctx := patterns.ExpandContext{}

	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern with callout failed: %v", err)
	}

	// comparison-2col with headers: 1 header row + 1 body row + 1 callout row = 3
	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}

	calloutRow := grid.Rows[2]
	if !calloutRow.AutoHeight {
		t.Error("callout row should have AutoHeight=true")
	}
	// Should use accent3 fill
	var fill string
	if err := json.Unmarshal(calloutRow.Cells[0].Shape.Fill, &fill); err != nil {
		t.Fatalf("fill unmarshal: %v", err)
	}
	if fill != "accent3" {
		t.Errorf("callout fill = %q, want %q", fill, "accent3")
	}
}

func TestExpandPattern_CalloutUnsupportedPattern(t *testing.T) {
	input := &PatternInput{
		Name: "matrix-2x2",
		Values: json.RawMessage(`{
			"x_axis_label": "X",
			"y_axis_label": "Y",
			"top_left": {"header": "A"},
			"top_right": {"header": "B"},
			"bottom_left": {"header": "C"},
			"bottom_right": {"header": "D"}
		}`),
		Callout: &patterns.PatternCallout{
			Text: "This should fail",
		},
	}

	ctx := patterns.ExpandContext{}

	_, _, err := expandPattern(input, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected error for unsupported callout on matrix-2x2")
	}
	if !strings.Contains(err.Error(), "does not support callout") {
		t.Errorf("error should mention 'does not support callout', got: %v", err)
	}
}

func TestExpandPattern_NoCalloutNilDoesNothing(t *testing.T) {
	input := &PatternInput{
		Name: "card-grid",
		Values: json.RawMessage(`{
			"columns": 1,
			"rows": 1,
			"cells": [{"header": "A", "body": "B"}]
		}`),
		// No Callout
	}

	ctx := patterns.ExpandContext{}

	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern failed: %v", err)
	}
	// Should have only 1 row (no callout appended)
	if len(grid.Rows) != 1 {
		t.Errorf("expected 1 row (no callout), got %d", len(grid.Rows))
	}
}

// ---------------------------------------------------------------------------
// Cross-template callout rendering (all 4 templates)
// ---------------------------------------------------------------------------

func TestCalloutCrossTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-template callout test in short mode")
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join(projectRoot, "templates")
	templates := []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"}

	fixtures := []string{
		filepath.Join(projectRoot, "tests", "integration", "json_fixtures", "50_pattern_cardgrid_callout.json"),
		filepath.Join(projectRoot, "tests", "integration", "json_fixtures", "51_pattern_comparison2col_callout.json"),
	}

	outputDir := t.TempDir()

	for _, fixture := range fixtures {
		fixtureName := strings.TrimSuffix(filepath.Base(fixture), ".json")
		for _, tmpl := range templates {
			t.Run(fixtureName+"/"+tmpl, func(t *testing.T) {
				t.Parallel()

				data, err := os.ReadFile(fixture)
				if err != nil {
					t.Fatal(err)
				}

				var input map[string]interface{}
				if err := json.Unmarshal(data, &input); err != nil {
					t.Fatal(err)
				}
				input["template"] = tmpl
				outputFilename := fixtureName + "_" + tmpl + ".pptx"
				input["output_filename"] = outputFilename

				modifiedJSON, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}

				tmpJSON := filepath.Join(outputDir, fixtureName+"_"+tmpl+".json")
				if err := os.WriteFile(tmpJSON, modifiedJSON, 0644); err != nil {
					t.Fatal(err)
				}

				jsonResultPath := filepath.Join(outputDir, fixtureName+"_"+tmpl+".result.json")
				err = runJSONMode(tmpJSON, jsonResultPath, templatesDir, outputDir, "", false, false, "")
				if err != nil {
					t.Fatalf("runJSONMode failed: %v", err)
				}

				validatePPTX(t, filepath.Join(outputDir, outputFilename))
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Golden tests for callout expansion output
// ---------------------------------------------------------------------------

func TestExpandPattern_CalloutCardGrid_Golden(t *testing.T) {
	input := &PatternInput{
		Name: "card-grid",
		Values: json.RawMessage(`{
			"columns": 2,
			"rows": 1,
			"cells": [
				{"header": "Speed", "body": "Sub-second response times"},
				{"header": "Reliability", "body": "99.99% uptime SLA"}
			]
		}`),
		Callout: &patterns.PatternCallout{
			Text:     "Key takeaway: performance and reliability are non-negotiable",
			Emphasis: "bold",
		},
	}

	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern: %v", err)
	}

	got, err := json.MarshalIndent(grid, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "callout", "card-grid-callout.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandPattern_CalloutComparison2col_Golden(t *testing.T) {
	input := &PatternInput{
		Name: "comparison-2col",
		Values: json.RawMessage(`{
			"header_left": "Current State",
			"header_right": "Future State",
			"rows": [
				{"left": "Manual deployment", "right": "Automated CI/CD"},
				{"left": "Monolithic architecture", "right": "Microservices"}
			]
		}`),
		Callout: &patterns.PatternCallout{
			Text:   "Migration timeline: Q3 2026",
			Accent: "accent3",
		},
	}

	ctx := patterns.ExpandContext{}
	grid, _, err := expandPattern(input, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern: %v", err)
	}

	got, err := json.MarshalIndent(grid, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "callout", "comparison-2col-callout.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// ---------------------------------------------------------------------------
// cell_overrides regression: indices unchanged when callout present
// ---------------------------------------------------------------------------

func TestExpandPattern_CellOverridesUnchangedWithCallout(t *testing.T) {
	// Expand card-grid with cell_overrides[0] accent bar, WITHOUT callout
	baseInput := &PatternInput{
		Name: "card-grid",
		Values: json.RawMessage(`{
			"columns": 2,
			"rows": 1,
			"cells": [
				{"header": "Card 1", "body": "Description 1"},
				{"header": "Card 2", "body": "Description 2"}
			]
		}`),
		CellOverrides: map[string]json.RawMessage{
			"0": json.RawMessage(`{"accent_bar": true}`),
		},
	}

	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	gridWithout, _, err := expandPattern(baseInput, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern without callout: %v", err)
	}

	// Expand same card-grid with cell_overrides[0] accent bar, WITH callout
	calloutInput := &PatternInput{
		Name: "card-grid",
		Values: json.RawMessage(`{
			"columns": 2,
			"rows": 1,
			"cells": [
				{"header": "Card 1", "body": "Description 1"},
				{"header": "Card 2", "body": "Description 2"}
			]
		}`),
		CellOverrides: map[string]json.RawMessage{
			"0": json.RawMessage(`{"accent_bar": true}`),
		},
		Callout: &patterns.PatternCallout{
			Text:     "Summary note",
			Emphasis: "bold",
		},
	}

	gridWith, _, err := expandPattern(calloutInput, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandPattern with callout: %v", err)
	}

	// Content rows should be identical (cell_overrides indexing unchanged)
	if len(gridWithout.Rows) < 1 {
		t.Fatal("expected at least 1 row without callout")
	}
	if len(gridWith.Rows) < 2 {
		t.Fatal("expected at least 2 rows with callout (1 content + 1 callout)")
	}

	// Compare first row (content) — cell[0] should have accent bar in both
	abWithout := gridWithout.Rows[0].Cells[0].AccentBar
	abWith := gridWith.Rows[0].Cells[0].AccentBar

	if abWithout == nil {
		t.Fatal("cell[0] should have accent bar without callout")
	}
	if abWith == nil {
		t.Fatal("cell[0] should have accent bar with callout")
	}

	if abWithout.Color != abWith.Color {
		t.Errorf("accent bar color mismatch: without=%q, with=%q", abWithout.Color, abWith.Color)
	}
	if abWithout.Position != abWith.Position {
		t.Errorf("accent bar position mismatch: without=%q, with=%q", abWithout.Position, abWith.Position)
	}

	// Cell[1] should NOT have accent bar in either case
	if gridWithout.Rows[0].Cells[1].AccentBar != nil {
		t.Error("cell[1] should not have accent bar without callout")
	}
	if gridWith.Rows[0].Cells[1].AccentBar != nil {
		t.Error("cell[1] should not have accent bar with callout")
	}
}

func TestComputeQualityScore_ShapeGridNotEmpty(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID:  "content",
			ShapeGrid: &ShapeGridInput{},
			// No Content — should NOT be penalized as empty
		},
	}

	score := computeQualityScore(slides, nil)
	for _, sq := range score.SlideScores {
		for _, issue := range sq.Issues {
			if strings.Contains(issue, "empty slide") {
				t.Errorf("slide with shape_grid should not be scored as empty, got issue: %s", issue)
			}
		}
	}
}

func TestComputeQualityScore_PatternNotEmpty(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "content",
			Pattern:  &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`[]`)},
			// No Content — should NOT be penalized as empty
		},
	}

	score := computeQualityScore(slides, nil)
	for _, sq := range score.SlideScores {
		for _, issue := range sq.Issues {
			if strings.Contains(issue, "empty slide") {
				t.Errorf("slide with pattern should not be scored as empty, got issue: %s", issue)
			}
		}
	}
}

func TestComputeQualityScore_TrulyEmptyStillPenalized(t *testing.T) {
	slides := []SlideInput{
		{
			LayoutID: "content",
			// No Content, no ShapeGrid, no Pattern
		},
	}

	score := computeQualityScore(slides, nil)
	found := false
	for _, sq := range score.SlideScores {
		for _, issue := range sq.Issues {
			if strings.Contains(issue, "empty slide") {
				found = true
			}
		}
	}
	if !found {
		t.Error("truly empty slide should still be penalized")
	}
}
