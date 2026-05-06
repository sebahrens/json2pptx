package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestDetectSparseLayout_Sparse(t *testing.T) {
	// Content is 30% of bounds — should fire.
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:      0,
		Path:            "slides[0].shape_grid",
		BoundsHeightEMU: 1000000,
		ContentHeightEMU: 300000,
	})
	if f == nil {
		t.Fatal("expected sparse_layout finding, got nil")
	}
	if f.Code != patterns.ErrCodeSparseLayout {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeSparseLayout, f.Code)
	}
	if f.Action != "review" {
		t.Errorf("expected action %q, got %q", "review", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "grow_pattern" {
		t.Errorf("expected fix kind %q, got %v", "grow_pattern", f.Fix)
	}
}

func TestDetectSparseLayout_NotSparse(t *testing.T) {
	// Content is 60% of bounds — should not fire.
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:      0,
		Path:            "slides[0].shape_grid",
		BoundsHeightEMU: 1000000,
		ContentHeightEMU: 600000,
	})
	if f != nil {
		t.Errorf("expected nil, got finding with code %q", f.Code)
	}
}

func TestDetectSparseLayout_ExactThreshold(t *testing.T) {
	// Content is exactly 40% of bounds — should not fire (>= threshold).
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:      0,
		Path:            "slides[0].shape_grid",
		BoundsHeightEMU: 1000000,
		ContentHeightEMU: 400000,
	})
	if f != nil {
		t.Errorf("expected nil at threshold, got finding with code %q", f.Code)
	}
}

func TestDetectSparseLayout_ReshapeGrid(t *testing.T) {
	// 4 items in a 4×1 grid (4 rows, 1 col), content at 25% — should recommend reshape_grid.
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:       2,
		Path:             "slides[2].shape_grid",
		BoundsHeightEMU:  1000000,
		ContentHeightEMU: 250000,
		PatternName:      "card-grid",
		FilledSlots:      4,
		GridRows:         4,
		GridCols:         1,
	})
	if f == nil {
		t.Fatal("expected sparse_layout finding, got nil")
	}
	if f.Code != patterns.ErrCodeSparseLayout {
		t.Errorf("code = %q, want %q", f.Code, patterns.ErrCodeSparseLayout)
	}
	if f.Pattern != "card-grid" {
		t.Errorf("pattern = %q, want %q", f.Pattern, "card-grid")
	}
	if f.Fix == nil || f.Fix.Kind != "reshape_grid" {
		t.Fatalf("fix kind = %v, want reshape_grid", f.Fix)
	}
	rows, _ := f.Fix.Params["rows"].(int)
	cols, _ := f.Fix.Params["columns"].(int)
	if rows != 2 || cols != 2 {
		t.Errorf("reshape_grid rows=%d cols=%d, want rows=2 cols=2", rows, cols)
	}
}

func TestDetectSparseLayout_NoReshapeWhenDimensionsSame(t *testing.T) {
	// 4 items already in 2×2 — should fallback to grow_pattern.
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:       0,
		Path:             "slides[0].shape_grid",
		BoundsHeightEMU:  1000000,
		ContentHeightEMU: 300000,
		PatternName:      "card-grid",
		FilledSlots:      4,
		GridRows:         2,
		GridCols:         2,
	})
	if f == nil {
		t.Fatal("expected finding, got nil")
	}
	if f.Fix == nil || f.Fix.Kind != "grow_pattern" {
		t.Errorf("fix kind = %v, want grow_pattern (dimensions already optimal)", f.Fix)
	}
}

func TestDetectSparseLayout_NoPatternFallsBackToGrow(t *testing.T) {
	// No pattern name — should use grow_pattern.
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex:       0,
		Path:             "slides[0].shape_grid",
		BoundsHeightEMU:  1000000,
		ContentHeightEMU: 300000,
	})
	if f == nil {
		t.Fatal("expected finding, got nil")
	}
	if f.Fix == nil || f.Fix.Kind != "grow_pattern" {
		t.Errorf("fix kind = %v, want grow_pattern", f.Fix)
	}
}

func TestOptimalGridDimensions(t *testing.T) {
	tests := []struct {
		n        int
		wantRows int
		wantCols int
	}{
		{1, 1, 1},
		{2, 1, 2},
		{3, 1, 3},
		{4, 2, 2},
		{5, 2, 3},
		{6, 2, 3},
		{7, 2, 4},
		{8, 2, 4},
		{9, 3, 3},
	}
	for _, tt := range tests {
		rows, cols := optimalGridDimensions(tt.n)
		if rows != tt.wantRows || cols != tt.wantCols {
			t.Errorf("optimalGridDimensions(%d) = (%d, %d), want (%d, %d)", tt.n, rows, cols, tt.wantRows, tt.wantCols)
		}
	}
}

func TestDetectSparseLayout_InvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input SparseLayoutInput
	}{
		{"zero bounds", SparseLayoutInput{BoundsHeightEMU: 0, ContentHeightEMU: 100}},
		{"zero content", SparseLayoutInput{BoundsHeightEMU: 100, ContentHeightEMU: 0}},
		{"negative bounds", SparseLayoutInput{BoundsHeightEMU: -1, ContentHeightEMU: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if f := DetectSparseLayout(tt.input); f != nil {
				t.Errorf("expected nil for invalid input, got %v", f)
			}
		})
	}
}
