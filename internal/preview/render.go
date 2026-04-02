package preview

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"

	"github.com/ahrens/svggen/raster"
	"github.com/sebahrens/json2pptx/internal/types"
)

// RenderSlidePNG renders a single slide definition to PNG bytes.
// dpi controls the output resolution (default 192 = 2x).
func RenderSlidePNG(slide types.SlideDefinition, theme *types.ThemeInfo, dpi float64) ([]byte, error) {
	if dpi <= 0 {
		dpi = 192
	}

	c := canvas.New(slideWidthMM, slideHeightMM)
	ctx := canvas.NewContext(c)

	ff := loadFont(theme)
	renderSlide(ctx, slide, theme, ff)

	raster.Mu.Lock()
	img := rasterizer.Draw(c, canvas.DPI(dpi), nil)
	raster.Mu.Unlock()

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("preview: png encode: %w", err)
	}
	return buf.Bytes(), nil
}
