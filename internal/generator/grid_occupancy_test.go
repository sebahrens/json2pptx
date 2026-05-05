package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestDetectPatternUnderfilled(t *testing.T) {
	tests := []struct {
		name     string
		input    GridOccupancyInput
		wantNil  bool
		wantCode string
	}{
		{
			name:    "nil when filled at 50%",
			input:   GridOccupancyInput{FilledSlots: 2, TotalSlots: 4, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
		{
			name:    "nil when fully filled",
			input:   GridOccupancyInput{FilledSlots: 4, TotalSlots: 4, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
		{
			name:     "fires when 2 of 8 filled (25%)",
			input:    GridOccupancyInput{FilledSlots: 2, TotalSlots: 8, PatternName: "matrix-2x2", Path: "slides[0].shape_grid"},
			wantCode: patterns.ErrCodePatternUnderfilled,
		},
		{
			name:     "fires when 1 of 4 filled (25%)",
			input:    GridOccupancyInput{FilledSlots: 1, TotalSlots: 4, PatternName: "card-grid", Path: "slides[1].shape_grid"},
			wantCode: patterns.ErrCodePatternUnderfilled,
		},
		{
			name:    "nil when zero total slots",
			input:   GridOccupancyInput{FilledSlots: 0, TotalSlots: 0, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
		{
			name:    "nil when zero filled slots",
			input:   GridOccupancyInput{FilledSlots: 0, TotalSlots: 4, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := DetectPatternUnderfilled(tt.input)
			if tt.wantNil {
				if f != nil {
					t.Fatalf("expected nil, got finding with code %q", f.Code)
				}
				return
			}
			if f == nil {
				t.Fatal("expected finding, got nil")
			}
			if f.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", f.Code, tt.wantCode)
			}
			if f.Fix == nil || f.Fix.Kind != "swap_pattern" {
				t.Errorf("fix kind = %v, want swap_pattern", f.Fix)
			}
			if f.Action != "review" {
				t.Errorf("action = %q, want review", f.Action)
			}
		})
	}
}

func TestDetectPatternOvercrowded(t *testing.T) {
	tests := []struct {
		name     string
		input    GridOccupancyInput
		wantNil  bool
		wantCode string
	}{
		{
			name:    "nil when no recommended max",
			input:   GridOccupancyInput{FilledSlots: 20, TotalSlots: 20, RecommendedMax: 0, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
		{
			name:    "nil when within limit",
			input:   GridOccupancyInput{FilledSlots: 9, TotalSlots: 12, RecommendedMax: 9, Path: "slides[0].shape_grid"},
			wantNil: true,
		},
		{
			name:     "fires when 12 cells exceed max 9",
			input:    GridOccupancyInput{FilledSlots: 12, TotalSlots: 16, RecommendedMax: 9, PatternName: "card-grid", Path: "slides[0].shape_grid"},
			wantCode: patterns.ErrCodePatternOvercrowded,
		},
		{
			name:     "fires when 6 cells exceed max 5",
			input:    GridOccupancyInput{FilledSlots: 6, TotalSlots: 6, RecommendedMax: 5, PatternName: "icon-row", Path: "slides[2].shape_grid"},
			wantCode: patterns.ErrCodePatternOvercrowded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := DetectPatternOvercrowded(tt.input)
			if tt.wantNil {
				if f != nil {
					t.Fatalf("expected nil, got finding with code %q", f.Code)
				}
				return
			}
			if f == nil {
				t.Fatal("expected finding, got nil")
			}
			if f.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", f.Code, tt.wantCode)
			}
			if f.Fix == nil || f.Fix.Kind != "split_pattern" {
				t.Errorf("fix kind = %v, want split_pattern", f.Fix)
			}
			if f.Action != "review" {
				t.Errorf("action = %q, want review", f.Action)
			}
		})
	}
}
