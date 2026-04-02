package generator

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestGetOptimalRenderDimensions(t *testing.T) {
	tests := []struct {
		name              string
		diagramSpec       *types.DiagramSpec
		placeholderBounds types.BoundingBox
		wantWidth         int
		wantHeight        int
	}{
		{
			name: "explicit dimensions are preserved",
			diagramSpec: &types.DiagramSpec{
				Width:  1200,
				Height: 800,
			},
			placeholderBounds: types.BoundingBox{
				Width:  9144000,
				Height: 6858000,
			},
			wantWidth:  1200,
			wantHeight: 800,
		},
		{
			name:        "landscape placeholder converts EMU to points",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  8000000, // 8000000/12700 = 629pt
				Height: 5000000, // 5000000/12700 = 393pt
			},
			wantWidth:  629,
			wantHeight: 393,
		},
		{
			name:        "portrait placeholder converts EMU to points",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  4572000, // 4572000/12700 = 360pt
				Height: 6000000, // 6000000/12700 = 472pt
			},
			wantWidth:  360,
			wantHeight: 472,
		},
		{
			name:        "narrow portrait placeholder",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  2500000, // 2500000/12700 = 196pt
				Height: 5000000, // 5000000/12700 = 393pt
			},
			wantWidth:  196,
			wantHeight: 393,
		},
		{
			name:        "square placeholder",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  3000000, // 3000000/12700 = 236pt
				Height: 3000000,
			},
			wantWidth:  236,
			wantHeight: 236,
		},
		{
			name:        "zero placeholder bounds returns zeros",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  0,
				Height: 0,
			},
			wantWidth:  0,
			wantHeight: 0,
		},
		{
			name: "partial explicit dimensions use calculated for missing",
			diagramSpec: &types.DiagramSpec{
				Width: 0, // Not set
			},
			placeholderBounds: types.BoundingBox{
				Width:  8000000,
				Height: 5000000,
			},
			wantWidth:  629,
			wantHeight: 393,
		},
		{
			name:        "very small placeholder clamps to minimum",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  9000000, // 708pt
				Height: 100000,  // 7pt -> clamped to 100
			},
			wantWidth:  708,
			wantHeight: 100, // clamped to minimum
		},
		{
			name:        "typical full-width 16x9 placeholder",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  10515600, // ~828pt (typical full-width content)
				Height: 5905500,  // ~465pt
			},
			wantWidth:  828,
			wantHeight: 465,
		},
		{
			name:        "half-width placeholder",
			diagramSpec: &types.DiagramSpec{},
			placeholderBounds: types.BoundingBox{
				Width:  5257800, // ~414pt (half-width content)
				Height: 5905500, // ~465pt
			},
			wantWidth:  414,
			wantHeight: 465,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight := getOptimalRenderDimensions(tt.diagramSpec, tt.placeholderBounds)
			if gotWidth != tt.wantWidth {
				t.Errorf("getOptimalRenderDimensions() width = %d, want %d", gotWidth, tt.wantWidth)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("getOptimalRenderDimensions() height = %d, want %d", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestLayoutPresetSelection(t *testing.T) {
	// Test that the preset selection logic matches the documented thresholds
	slideWidth := types.StandardSlideWidth16x9

	tests := []struct {
		name               string
		placeholderWidthPct float64 // As percentage of slide width
		expectedPreset     types.LayoutPreset
	}{
		{
			name:               "100% width -> content_16x9",
			placeholderWidthPct: 1.0,
			expectedPreset:     types.PresetContent16x9,
		},
		{
			name:               "85% width -> content_16x9",
			placeholderWidthPct: 0.85,
			expectedPreset:     types.PresetContent16x9,
		},
		{
			name:               "79% width -> half_16x9",
			placeholderWidthPct: 0.79,
			expectedPreset:     types.PresetHalf16x9,
		},
		{
			name:               "50% width -> half_16x9",
			placeholderWidthPct: 0.50,
			expectedPreset:     types.PresetHalf16x9,
		},
		{
			name:               "36% width -> half_16x9",
			placeholderWidthPct: 0.36,
			expectedPreset:     types.PresetHalf16x9,
		},
		{
			name:               "34% width -> third_16x9",
			placeholderWidthPct: 0.34,
			expectedPreset:     types.PresetThird16x9,
		},
		{
			name:               "33% width -> third_16x9",
			placeholderWidthPct: 0.33,
			expectedPreset:     types.PresetThird16x9,
		},
		{
			name:               "20% width -> third_16x9",
			placeholderWidthPct: 0.20,
			expectedPreset:     types.PresetThird16x9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			placeholderWidth := types.EMU(float64(slideWidth) * tt.placeholderWidthPct)
			got := types.DetectLayoutPreset(placeholderWidth, slideWidth)
			if got != tt.expectedPreset {
				t.Errorf("DetectLayoutPreset(%v, %v) = %q, want %q",
					placeholderWidth, slideWidth, got, tt.expectedPreset)
			}
		})
	}
}
