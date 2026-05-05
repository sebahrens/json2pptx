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
