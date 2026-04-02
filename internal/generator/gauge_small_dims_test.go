package generator

import (
	"os"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestGaugeSmallDimensions tests gauge rendering at small dimensions
// typical of real PPTX placeholders (300-500 points).
func TestGaugeSmallDimensions(t *testing.T) {
	dims := []struct {
		name   string
		width  int
		height int
	}{
		{"half_slide_300x300", 300, 300},
		{"half_slide_400x350", 400, 350},
		{"third_slide_250x300", 250, 300},
		{"full_width_720x400", 720, 400},
		{"tiny_100x100", 100, 100},
	}

	for _, d := range dims {
		t.Run(d.name, func(t *testing.T) {
			spec := &types.DiagramSpec{
				Type:   "gauge_chart",
				Title:  "Score",
				Data:   map[string]any{"value": float64(85), "min": 0.0, "max": 100.0},
				Width:  d.width,
				Height: d.height,
			}

			result, err := RenderDiagramSpecWithMetadata(spec, nil, 0, false)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if len(result.PNG) == 0 {
				t.Fatal("Empty PNG")
			}

			fname := "/tmp/gauge_dim_" + d.name + ".png"
			os.WriteFile(fname, result.PNG, 0644)
			t.Logf("Written %s (%d bytes), ContentW=%.0f ContentH=%.0f",
				fname, len(result.PNG), result.ContentWidth, result.ContentHeight)
		})
	}
}
