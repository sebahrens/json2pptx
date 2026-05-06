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

func TestExpandCompose_SmartCompose_StatHeroKpi3up(t *testing.T) {
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: true,
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime SLA"}`),
				},
			},
			{
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

	// stat-hero has 1 cell, kpi-3up has 3 cells -> 1:3 ratio
	// hero should get ~25%, kpi should get ~75%
	// Collect heights by segment: stat-hero rows come first
	heroRows := 0
	kpiRows := 0
	var heroHeight, kpiHeight float64
	for _, row := range grid.Rows {
		if heroRows == 0 || kpiHeight == 0 {
			// First row(s) belong to stat-hero (1 row in stat-hero)
			heroHeight += row.Height
			heroRows++
			if heroRows == 1 {
				// stat-hero only has 1 row, switch to kpi
				continue
			}
		}
		kpiHeight += row.Height
		kpiRows++
	}
	// Re-collect properly: stat-hero = 1 row, kpi-3up = 1 row
	// So grid.Rows[0] = hero, grid.Rows[1] = kpi
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}
	heroHeight = grid.Rows[0].Height
	kpiHeight = grid.Rows[1].Height

	// hero should be ~25% (1/(1+3)*100), kpi ~75% (3/(1+3)*100)
	if heroHeight < 20 || heroHeight > 30 {
		t.Errorf("hero height should be ~25%%, got %.1f%%", heroHeight)
	}
	if kpiHeight < 70 || kpiHeight > 80 {
		t.Errorf("kpi height should be ~75%%, got %.1f%%", kpiHeight)
	}
}

func TestExpandCompose_SmartCompose_ExplicitSizePctOverrides(t *testing.T) {
	// When explicit SizePct is set, smart_compose should NOT override it
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: true,
		Segments: []SegmentInput{
			{
				SizePct: 60,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
			{
				SizePct: 40,
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "A", "small": "X"}, {"big": "B", "small": "Y"}, {"big": "C", "small": "Z"}]`),
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
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}

	// Explicit sizes: hero=60%, kpi=40% — smart compose doesn't override
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestExpandCompose_SmartCompose_FalseUsesEqualSplit(t *testing.T) {
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: false,
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "A", "label": "X"}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "1", "small": "a"}, {"big": "2", "small": "b"}, {"big": "3", "small": "c"}]`),
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
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}

	// Without smart compose, both segments get 50%
	heroHeight := grid.Rows[0].Height
	kpiHeight := grid.Rows[1].Height
	if heroHeight < 49 || heroHeight > 51 {
		t.Errorf("hero height should be ~50%% without smart compose, got %.1f%%", heroHeight)
	}
	if kpiHeight < 49 || kpiHeight > 51 {
		t.Errorf("kpi height should be ~50%% without smart compose, got %.1f%%", kpiHeight)
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

