package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestExpandCompose_Vertical(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				SizePct: 40,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime SLA"}`),
				},
			},
			{
				SizePct: 60,
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "$4M", "small": "ARR"}, {"big": "98%", "small": "NRR"}, {"big": "1K", "small": "Customers"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}
	if len(grid.Rows) == 0 {
		t.Fatal("merged grid has no rows")
	}

	// Verify the merged grid has valid row heights
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	// Should sum to approximately 100% (40 + 60)
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestExpandCompose_Horizontal(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "3x", "label": "Growth"}`),
				},
			},
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}

	// Should have columns matching segment count
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns should be array of percentages: %v", err)
	}
	if len(cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(cols))
	}
}

func TestExpandCompose_EqualSplit(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "A", "label": "First"}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "B", "label": "Second"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}

	// Equal split: each segment gets 50%
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestValidateCompose_Errors(t *testing.T) {
	tests := []struct {
		name    string
		compose ComposeInput
		wantErr string
	}{
		{
			name: "invalid direction",
			compose: ComposeInput{
				Direction: "diagonal",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "direction must be",
		},
		{
			name: "too few segments",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "at least 2 segments",
		},
		{
			name: "too many segments",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "maximum 4 segments",
		},
		{
			name: "size exceeds 100",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{SizePct: 70, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "exceeds 100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCompose(&tt.compose)
			if err == nil {
				t.Fatal("expected error")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExpandCompose_ChildValidationBubbles(t *testing.T) {
	// stat-hero requires value and label — omit them to trigger child validation error
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "", "label": ""}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "X", "small": "Y"}, {"big": "A", "small": "B"}, {"big": "C", "small": "D"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	_, err := expandCompose(compose, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected validation error from child pattern")
	}
	if !contains(err.Error(), "segment[0]") {
		t.Errorf("error should reference segment index, got: %v", err)
	}
}

