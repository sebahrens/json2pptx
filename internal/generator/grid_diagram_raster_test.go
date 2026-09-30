package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-4c9m7: a non-native strategy rasterizes grid diagrams at
// the frame's 96-DPI size times the configured scale, width-capped at
// MaxPNGWidth.
func TestGridDiagramRasterSizePx(t *testing.T) {
	px := int64(types.EMUPerPixel)
	cases := []struct {
		name     string
		scale    float64
		maxW     int
		cx, cy   int64
		wantSize int
	}{
		{"landscape default scale", 0, 0, 400 * px, 300 * px, 800},
		{"scale 3", 3, 0, 400 * px, 300 * px, 1200},
		{"landscape capped", 2, 500, 400 * px, 300 * px, 500},
		{"portrait width cap keeps aspect", 2, 200, 200 * px, 400 * px, 400},
		{"portrait under cap", 2, 1000, 200 * px, 400 * px, 800},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &SVGConverter{Scale: tc.scale, MaxPNGWidth: tc.maxW}
			if got := c.gridDiagramRasterSizePx(tc.cx, tc.cy); got != tc.wantSize {
				t.Errorf("size = %d, want %d", got, tc.wantSize)
			}
		})
	}
}
