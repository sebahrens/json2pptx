package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGetOptimalRenderDimensions(t *testing.T) {
	tests := []struct {
		name              string
		diagramSpec       *types.DiagramSpec
		placeholderBounds types.BoundingBox
		wantWidth         int
		wantHeight        int
		wantSource        DiagramDimensionSource
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
			wantSource: DiagramDimsExplicit,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsUnresolved,
		},
		{
			// Width-only intent: the explicit width is preserved and the height
			// is derived from the target aspect (2.0) so the SVG fills the
			// bounds without letterboxing.
			name: "width-only preserves width and derives height from bounds aspect",
			diagramSpec: &types.DiagramSpec{
				Width: 600,
			},
			placeholderBounds: types.BoundingBox{
				Width:  8000000, // aspect 2.0
				Height: 4000000,
			},
			wantWidth:  600,
			wantHeight: 300, // round(600 / 2.0)
			wantSource: DiagramDimsWidthOnly,
		},
		{
			// Height-only intent: the explicit height is preserved and the width
			// is derived from the target aspect (2.0).
			name: "height-only preserves height and derives width from bounds aspect",
			diagramSpec: &types.DiagramSpec{
				Height: 400,
			},
			placeholderBounds: types.BoundingBox{
				Width:  8000000, // aspect 2.0
				Height: 4000000,
			},
			wantWidth:  800, // round(400 * 2.0)
			wantHeight: 400,
			wantSource: DiagramDimsHeightOnly,
		},
		{
			// Width-only with no usable bounds falls back to the default chart
			// aspect (800/600 = 1.333) for the derived dimension.
			name: "width-only with no bounds uses default chart aspect",
			diagramSpec: &types.DiagramSpec{
				Width: 600,
			},
			placeholderBounds: types.BoundingBox{Width: 0, Height: 0},
			wantWidth:         600,
			wantHeight:        450, // round(600 / (800/600))
			wantSource:        DiagramDimsWidthOnly,
		},
		{
			// Height-only with no usable bounds falls back to the default chart
			// aspect for the derived dimension.
			name: "height-only with no bounds uses default chart aspect",
			diagramSpec: &types.DiagramSpec{
				Height: 300,
			},
			placeholderBounds: types.BoundingBox{Width: 0, Height: 0},
			wantWidth:         400, // round(300 * (800/600))
			wantHeight:        300,
			wantSource:        DiagramDimsHeightOnly,
		},
		{
			// A derived dimension that would fall below the floor is clamped.
			name: "width-only clamps derived height to minimum",
			diagramSpec: &types.DiagramSpec{
				Width: 100,
			},
			placeholderBounds: types.BoundingBox{
				Width:  9000000, // aspect 90.0 -> derived height ~1pt
				Height: 100000,
			},
			wantWidth:  100,
			wantHeight: minRenderDimension,
			wantSource: DiagramDimsWidthOnly,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsBounds,
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
			wantSource: DiagramDimsBounds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight, gotSource := ResolveDiagramRenderDimensions(tt.diagramSpec, tt.placeholderBounds)
			if gotWidth != tt.wantWidth {
				t.Errorf("ResolveDiagramRenderDimensions() width = %d, want %d", gotWidth, tt.wantWidth)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("ResolveDiagramRenderDimensions() height = %d, want %d", gotHeight, tt.wantHeight)
			}
			if gotSource != tt.wantSource {
				t.Errorf("ResolveDiagramRenderDimensions() source = %q, want %q", gotSource, tt.wantSource)
			}

			// The thin wrapper must return the same width/height the resolver does.
			wrapW, wrapH := GetOptimalRenderDimensions(tt.diagramSpec, tt.placeholderBounds)
			if wrapW != tt.wantWidth || wrapH != tt.wantHeight {
				t.Errorf("GetOptimalRenderDimensions() = %dx%d, want %dx%d", wrapW, wrapH, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

