package main

import (
	"encoding/json"
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

	_, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000)
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

	specs, err := convertPresentationSlides([]SlideInput{slide}, nil, 12192000, 6858000)
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
